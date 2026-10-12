package history

import (
	"embed"
	"encoding/json"
	"fmt"
	"kriemhild/internal/biosphere"
	"reflect"
	"slices"
	"strings"
	"time"
)

// Registry is shared with the browser. Adding a type does not require a table migration.
//
//go:embed registry.json
var registryFiles embed.FS

type TypeDefinition struct {
	Category string `json:"category"`
	Spatial  bool   `json:"spatial"`
	Label    string `json:"label"`
}
type RelationDefinition struct {
	Family       string   `json:"family"`
	Sources      []string `json:"sources"`
	Destinations []string `json:"destinations"`
	Reciprocal   bool     `json:"reciprocal"`
	Label        string   `json:"label"`
}

var Registry struct {
	Types     map[string]TypeDefinition     `json:"types"`
	Relations map[string]RelationDefinition `json:"relations"`
}

func init() {
	b, _ := registryFiles.ReadFile("registry.json")
	if err := json.Unmarshal(b, &Registry); err != nil {
		panic(err)
	}
}

type EntityName struct {
	Name      string `json:"name"`
	Language  string `json:"language,omitempty"`
	Script    string `json:"script,omitempty"`
	Type      string `json:"type"`
	ContextID string `json:"contextId,omitempty"`
	ValidFrom string `json:"validFrom,omitempty"`
	ValidTo   string `json:"validTo,omitempty"`
}

// NetworkView stores a query, never copies its members or relationships.
type NetworkView struct {
	Archived      bool            `json:"archived,omitempty"`
	MemberIDs     []string        `json:"memberIds,omitempty"`
	LegacyData    json.RawMessage `json:"legacyData,omitempty"`
	Genealogy     bool            `json:"genealogy,omitempty"`
	ID            string          `json:"id"`
	AgeID         string          `json:"ageId"`
	Name          string          `json:"name"`
	CenterID      string          `json:"centerId,omitempty"`
	FamilyID      string          `json:"familyId,omitempty"`
	RelationTypes []string        `json:"relationTypes,omitempty"`
}

func (d *Document) Entity(age, id string) *Entity {
	for i := range d.Entities {
		if d.Entities[i].AgeID == age && d.Entities[i].ID == id {
			return &d.Entities[i]
		}
	}
	return nil
}

func (d *Document) pruneReferences(age string) {
	active := func(id string) bool { e := d.Entity(age, id); return e != nil && !e.Deleted }
	for i := range d.Entities {
		e := &d.Entities[i]
		if e.AgeID != age {
			continue
		}
		for j := range e.Names {
			if e.Names[j].ContextID != "" && !active(e.Names[j].ContextID) {
				e.Names[j].ContextID = ""
			}
		}
	}
	d.Relations = slices.DeleteFunc(d.Relations, func(r Relation) bool {
		return r.AgeID == age && (!active(r.Source) || !active(r.Destination) || (r.ContextID != "" && !active(r.ContextID)))
	})
	for i := range d.World.Networks {
		v := &d.World.Networks[i]
		if v.AgeID != age {
			continue
		}
		v.MemberIDs = slices.DeleteFunc(append([]string(nil), v.MemberIDs...), func(id string) bool { return !active(id) })
		if !active(v.CenterID) {
			v.CenterID = ""
		}
		if !active(v.FamilyID) {
			v.FamilyID = ""
		}
	}
}
func (d *Document) validateEntity(e Entity) error {
	if err := biosphere.ValidateLifeform(e.Biology); err != nil {
		return err
	}
	if e.Biology != nil && e.Kind != "lifeform" {
		return fmt.Errorf("biological profiles belong to lifeforms")
	}
	if e.Kind == "lifeform" && e.Biology == nil {
		return fmt.Errorf("lifeform requires biological data")
	}
	if !e.Deleted && e.Biology != nil && e.Biology.ParentID != "" {
		parent := d.Entity(e.AgeID, e.Biology.ParentID)
		if parent == nil || parent.Deleted || parent.ID == e.ID || parent.Biology == nil {
			return fmt.Errorf("invalid parent taxon")
		}
		seen := map[string]bool{e.ID: true}
		for parent != nil && parent.Biology != nil {
			if seen[parent.ID] {
				return fmt.Errorf("taxonomic lineage cannot contain cycles")
			}
			seen[parent.ID] = true
			parent = d.Entity(e.AgeID, parent.Biology.ParentID)
		}
	}
	if err := d.validateSystem(e); err != nil {
		return err
	}
	if err := d.validateContent(e); err != nil {
		return err
	}
	if err := d.validateInstitution(e); err != nil {
		return err
	}
	if (strings.TrimSpace(e.Name) == "" && e.Kind != "house") || len(e.Name) > 200 {
		return fmt.Errorf("entity needs a name of at most 200 characters")
	}
	var props map[string]any
	if json.Unmarshal(e.Properties, &props) != nil || props == nil {
		return fmt.Errorf("entity properties must be an object")
	}
	for _, n := range e.Names {
		if strings.TrimSpace(n.Name) == "" || !slices.Contains([]string{"canonical", "historical", "local", "alternate", "exonym"}, n.Type) {
			return fmt.Errorf("invalid name variant")
		}
		if n.ContextID != "" {
			x := d.Entity(e.AgeID, n.ContextID)
			if x == nil || x.Deleted {
				return fmt.Errorf("name context must exist in this Age")
			}
		}
	}
	return nil
}
func matchesType(e *Entity, allowed []string) bool {
	return len(allowed) == 0 || slices.Contains(allowed, e.Kind) || slices.Contains(allowed, Registry.Types[e.Kind].Category)
}
func (d *Document) validateRelation(r Relation) error {
	for _, text := range []string{r.Stance, r.Authority, r.Prevalence, r.Interpretation, r.Provenance} {
		if len(text) > 30000 {
			return fmt.Errorf("relationship metadata exceeds 30000 characters")
		}
	}
	if err := validateSystemRelation(r); err != nil {
		return err
	}
	if err := d.validateInstitutionRelation(r); err != nil {
		return err
	}
	a, b := d.Entity(r.AgeID, r.Source), d.Entity(r.AgeID, r.Destination)
	if a == nil || b == nil || a.Deleted || b.Deleted || a.ID == b.ID {
		return fmt.Errorf("relationship endpoints must be different active entities in this Age")
	}
	rule, ok := Registry.Relations[r.Kind]
	if !ok {
		return fmt.Errorf("unknown relationship type %q", r.Kind)
	}
	if !matchesType(a, rule.Sources) || !matchesType(b, rule.Destinations) {
		return fmt.Errorf("%s cannot connect %s to %s", r.Kind, a.Kind, b.Kind)
	}
	if r.ContextID != "" {
		c := d.Entity(r.AgeID, r.ContextID)
		if c == nil || c.Deleted {
			return fmt.Errorf("relationship context must exist in this Age")
		}
	}
	for _, port := range []string{r.SourcePort, r.DestinationPort} {
		if port != "" && !slices.Contains([]string{"n", "ne", "e", "se", "s", "sw", "w", "nw"}, port) {
			return fmt.Errorf("invalid character connection port")
		}
	}
	if len(r.Description) > 10000 || len(r.Notes) > 50000 {
		return fmt.Errorf("relationship description is too long")
	}
	return nil
}
func (d *Document) applyEntities(c Command) (bool, error) {
	if !slices.Contains([]string{"entity", "delete-entities", "relation", "relations", "delete-relation", "network", "delete-network"}, c.Kind) {
		return false, nil
	}
	if d.Age(c.AgeID) == nil {
		return true, fmt.Errorf("Age not found")
	}
	switch c.Kind {
	case "relations":
		if len(c.Relations) == 0 && len(c.RemoveRelationIDs) == 0 {
			return true, fmt.Errorf("provide at least one relationship")
		}
		copy := *d
		copy.Relations = append([]Relation(nil), d.Relations...)
		for _, id := range c.RemoveRelationIDs {
			if _, err := copy.applyEntities(Command{Kind: "delete-relation", AgeID: c.AgeID, EdgeID: id}); err != nil {
				return true, err
			}
		}
		for i := range c.Relations {
			if _, err := copy.applyEntities(Command{Kind: "relation", AgeID: c.AgeID, Relation: &c.Relations[i]}); err != nil {
				return true, err
			}
		}
		d.Relations = copy.Relations
	case "entity":
		if c.Entity == nil {
			return true, fmt.Errorf("entity is required")
		}
		e := *c.Entity
		e.System = cloneSystem(e.System)
		e.Archive = cloneArchive(e.Archive)
		e.AgeID = c.AgeID
		e.Content = cloneContent(e.Content)
		e.Biology = biosphere.Clone(e.Biology)
		if e.Biology != nil {
			e.Biology.Locked = true
		}
		e.UpdatedAt = time.Now().UnixMilli()
		e.Name = strings.TrimSpace(e.Name)
		if e.ID == "" {
			e.ID = ID()
		}
		old := d.Entity(c.AgeID, e.ID)
		if old != nil && e.Archive == nil {
			e.Archive = cloneArchive(old.Archive)
		}
		if old == nil {
			e.CreatedAt = e.UpdatedAt
			if _, ok := Registry.Types[e.Kind]; !ok {
				return true, fmt.Errorf("unknown entity type")
			}
			e.TerrainID = ""
			e.HomeMap = ""
			e.ContinuityID = e.ID
		} else {
			e.CreatedAt = old.CreatedAt
			if e.Biology == nil {
				e.Biology = biosphere.Clone(old.Biology)
			}
			if e.System == nil {
				e.System = cloneSystem(old.System)
			}
			if e.Content == nil {
				e.Content = cloneContent(old.Content)
			}
			if e.Kind != old.Kind {
				return true, fmt.Errorf("entity type cannot be changed by editing its profile")
			}
			e.TerrainID = old.TerrainID
			e.HomeMap = old.HomeMap
			e.ContinuityID = old.ContinuityID
			if e.ContinuityID == "" {
				e.ContinuityID = e.ID
			}
			e.Natural = old.Natural
			e.SourceID = old.SourceID
		}
		if len(e.Properties) == 0 {
			e.Properties = json.RawMessage(`{}`)
		}
		normalizeEntityYears(&e, old)
		if err := d.validateEntity(e); err != nil {
			return true, err
		}
		if err := validateYearEdit(e, old); err != nil {
			return true, err
		}
		biologyChanged := false
		if old != nil && old.Biology != nil && e.Biology != nil {
			previous, current := *old.Biology, *e.Biology
			previous.Locked = current.Locked
			biologyChanged = !reflect.DeepEqual(previous, current)
		}
		if old == nil {
			d.Entities = append(d.Entities, e)
		} else {
			*old = e
		}
		if biologyChanged {
			for i := range d.Biospheres {
				state := &d.Biospheres[i]
				if state.AgeID == e.AgeID {
					for _, o := range state.Occurrences {
						if o.SpeciesID == e.ID {
							state.Stale = true
							state.Diagnostics = append(state.Diagnostics, "Lifeform edited; review its habitat compatibility and ecological dependencies.")
							break
						}
					}
				}
			}
		}
		for i := range d.Representations {
			r := &d.Representations[i]
			if r.AgeID == c.AgeID && r.EntityID == e.ID {
				r.Shape.Name = e.Name
			}
		}
	case "delete-entities":
		if err := d.validateArchiveDeletion(c.AgeID, c.EntityIDs); err != nil {
			return true, err
		}
		if err := d.validateSystemDeletion(c.AgeID, c.EntityIDs); err != nil {
			return true, err
		}
		if err := d.validateBiologicalDeletion(c.AgeID, c.EntityIDs); err != nil {
			return true, err
		}
		if err := d.validateConceptDeletion(c.AgeID, c.EntityIDs); err != nil {
			return true, err
		}
		if err := d.validateInstitutionDeletion(c.AgeID, c.EntityIDs); err != nil {
			return true, err
		}
		for _, id := range c.EntityIDs {
			e := d.Entity(c.AgeID, id)
			if e == nil {
				return true, fmt.Errorf("entity not found")
			}
		}
		for _, id := range c.EntityIDs {
			d.Entity(c.AgeID, id).Deleted = true
		}
		d.pruneSystems(c.AgeID)
		d.PruneDeleted(c.AgeID)
		d.Resolve(c.AgeID)
		for i := range d.Entities {
			e := &d.Entities[i]
			if e.AgeID != c.AgeID {
				continue
			}
			for j := range e.Names {
				if slices.Contains(c.EntityIDs, e.Names[j].ContextID) {
					e.Names[j].ContextID = ""
				}
			}
		}
		d.Relations = slices.DeleteFunc(d.Relations, func(r Relation) bool { return r.AgeID == c.AgeID && slices.Contains(c.EntityIDs, r.ContextID) })
	case "relation":
		if c.Relation == nil {
			return true, fmt.Errorf("relationship is required")
		}
		r := *c.Relation
		r.AgeID = c.AgeID
		r.Automatic = false
		if err := d.validateRelation(r); err != nil {
			return true, err
		}
		if err := d.validateAppointmentChange(r); err != nil {
			return true, err
		}
		if err := d.validateFamilyChange(r); err != nil {
			return true, err
		}
		if Registry.Relations[r.Kind].Reciprocal && r.Source > r.Destination {
			r.SourcePort, r.DestinationPort = r.DestinationPort, r.SourcePort
			r.Source, r.Destination = r.Destination, r.Source
		}
		if r.ID == "" {
			r.ID = ID()
		}
		found := false
		for i, x := range d.Relations {
			if x.AgeID == r.AgeID && x.ID == r.ID {
				if x.Automatic {
					return true, fmt.Errorf("derived relationships are controlled by geography")
				}
				d.Relations[i] = r
				found = true
				break
			}
		}
		if !found {
			d.Relations = append(d.Relations, r)
		}
	case "delete-relation":
		for _, r := range d.Relations {
			if r.AgeID == c.AgeID && r.ID == c.EdgeID && r.Automatic {
				return true, fmt.Errorf("derived relationships are controlled by geography")
			}
		}
		d.Relations = slices.DeleteFunc(d.Relations, func(r Relation) bool { return r.AgeID == c.AgeID && r.ID == c.EdgeID })
	case "network":
		if c.Network == nil || strings.TrimSpace(c.Network.Name) == "" {
			return true, fmt.Errorf("network name is required")
		}
		v := *c.Network
		v.AgeID = c.AgeID
		v.MemberIDs = append([]string(nil), v.MemberIDs...)
		v.RelationTypes = append([]string(nil), v.RelationTypes...)
		for _, id := range append([]string{v.CenterID, v.FamilyID}, v.MemberIDs...) {
			if id != "" && (d.Entity(c.AgeID, id) == nil || d.Entity(c.AgeID, id).Deleted || d.Entity(c.AgeID, id).Kind != "character") {
				return true, fmt.Errorf("network subject not found")
			}
		}
		if v.ID == "" {
			v.ID = ID()
		}
		d.World.Networks = slices.DeleteFunc(d.World.Networks, func(x NetworkView) bool { return x.AgeID == v.AgeID && x.ID == v.ID })
		d.World.Networks = append(d.World.Networks, v)
	case "delete-network":
		d.World.Networks = slices.DeleteFunc(d.World.Networks, func(x NetworkView) bool { return x.AgeID == c.AgeID && x.ID == c.EdgeID })
	}
	return true, nil
}
