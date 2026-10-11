package history

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// Institution records use the same Age-scoped identity and snapshot machinery
// as people. Units and vacant offices exist independently of their occupants.
type institutionProperties struct {
	OrganizationID string `json:"organizationId"`
	ParentUnitID   string `json:"parentUnitId"`
	UnitID         string `json:"unitId"`
	Shared         bool   `json:"shared"`
}

func institution(e *Entity) institutionProperties {
	var p institutionProperties
	if e != nil {
		_ = json.Unmarshal(e.Properties, &p)
	}
	return p
}
func isOrganization(e *Entity) bool {
	return e != nil && !e.Deleted && Registry.Types[e.Kind].Category == "organizations"
}
func (d *Document) validateInstitution(e Entity) error {
	if e.Deleted || (e.Kind != "organization-unit" && e.Kind != "organization-position") {
		return nil
	}
	p := institution(&e)
	if e.Kind == "organization-position" && !p.Shared {
		for _, r := range d.Relations {
			if r.AgeID == e.AgeID && r.Kind == "appointed-to" && r.Destination == e.ID {
				if err := d.validateOfficeAppointment(r, false); err != nil {
					return err
				}
			}
		}
	}
	if !isOrganization(d.Entity(e.AgeID, p.OrganizationID)) {
		return fmt.Errorf("unit or office needs an active organization in this Age")
	}
	parent := p.ParentUnitID
	if e.Kind == "organization-position" {
		parent = p.UnitID
	}
	seen := map[string]bool{e.ID: true}
	for parent != "" {
		if seen[parent] {
			return fmt.Errorf("organizational units cannot contain themselves or their ancestors")
		}
		seen[parent] = true
		u := d.Entity(e.AgeID, parent)
		if u == nil || u.Deleted || u.Kind != "organization-unit" || institution(u).OrganizationID != p.OrganizationID {
			return fmt.Errorf("unit must belong to the same organization")
		}
		parent = institution(u).ParentUnitID
	}
	if old := d.Entity(e.AgeID, e.ID); old != nil && institution(old).OrganizationID != p.OrganizationID {
		return fmt.Errorf("a unit or office cannot silently change its organization")
	}
	return nil
}
func (d *Document) validateInstitutionRelation(r Relation) error {
	if r.Kind == "appointed-to" {
		p := institution(d.Entity(r.AgeID, r.Destination))
		if r.UnitID != "" && p.UnitID != "" && r.UnitID != p.UnitID {
			return fmt.Errorf("appointment must use the office's unit")
		}
	}
	if r.UnitID == "" {
		return nil
	}
	u := d.Entity(r.AgeID, r.UnitID)
	org := r.Destination
	if r.Kind == "appointed-to" {
		org = institution(d.Entity(r.AgeID, r.Destination)).OrganizationID
	}
	if u == nil || u.Deleted || u.Kind != "organization-unit" || institution(u).OrganizationID != org {
		return fmt.Errorf("membership unit must belong to its organization")
	}
	return nil
}

func (d *Document) validateAppointmentChange(r Relation) error {
	return d.validateOfficeAppointment(r, institution(d.Entity(r.AgeID, r.Destination)).Shared)
}
func (d *Document) validateOfficeAppointment(r Relation, shared bool) error {
	if r.Kind != "appointed-to" || shared {
		return nil
	}
	active := func(status string) bool {
		return status == "" || status == "active" || status == "temporary" || status == "acting"
	}
	bound := func(v string, fallback float64) float64 {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fallback
		}
		return n
	}
	if !active(r.Status) {
		return nil
	}
	for _, old := range d.Relations {
		if old.AgeID != r.AgeID || old.ID == r.ID || old.Kind != r.Kind || old.Destination != r.Destination || old.Source == r.Source || !active(old.Status) {
			continue
		}
		if math.Max(bound(old.ValidFrom, math.Inf(-1)), bound(r.ValidFrom, math.Inf(-1))) <= math.Min(bound(old.ValidTo, math.Inf(1)), bound(r.ValidTo, math.Inf(1))) {
			return fmt.Errorf("office already has an overlapping appointment; end the earlier appointment or mark the office as shared")
		}
	}
	return nil
}
func (d *Document) validateInstitutionDeletion(age string, ids []string) error {
	removing := map[string]bool{}
	for _, id := range ids {
		removing[id] = true
	}
	for _, e := range d.Entities {
		if e.AgeID != age || e.Deleted || removing[e.ID] {
			continue
		}
		p := institution(&e)
		if removing[p.OrganizationID] || removing[p.ParentUnitID] || removing[p.UnitID] {
			return fmt.Errorf("move or remove dependent units and offices first; dissolve an organization to retain its history")
		}
	}
	for _, r := range d.Relations {
		if r.AgeID == age && removing[r.UnitID] && !removing[r.Source] && !removing[r.Destination] {
			return fmt.Errorf("transfer unit members before removing the unit")
		}
	}
	return nil
}

// A batch is committed only after every member succeeds. This prevents orphan
// event records when their institutional relationship fails validation.
func (d *Document) applyBatch(c Command) error {
	if len(c.Commands) == 0 || len(c.Commands) > 100 {
		return fmt.Errorf("batch must contain 1–100 commands")
	}
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	var next Document
	if err = json.Unmarshal(b, &next); err != nil {
		return err
	}
	for _, child := range c.Commands {
		if child.Kind != "entity" && child.Kind != "relation" && child.Kind != "relations" && child.Kind != "delete-relation" && child.Kind != "delete-entities" {
			return fmt.Errorf("unsupported batch command")
		}
		child.AgeID = c.AgeID
		if err = next.Apply(child); err != nil {
			return err
		}
	}
	*d = next
	return nil
}
