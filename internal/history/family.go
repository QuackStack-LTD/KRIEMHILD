package history

import (
	"encoding/json"
	"fmt"
	"slices"
)

func parentPair(r Relation) (string, string, bool) {
	switch r.Kind {
	case "biological-parent", "adoptive-parent", "parent-of":
		return r.Source, r.Destination, true
	case "child-of":
		return r.Destination, r.Source, true
	}
	return "", "", false
}

// Validate changes rather than discarding ambiguous historical records. Old
// contradictions can still be inspected and deleted, but cannot be extended.
func (d *Document) validateFamilyChange(r Relation) error {
	p, c, parent := parentPair(r)
	if !parent && Registry.Relations[r.Kind].Family != "genealogical" {
		return nil
	}
	children := map[string][]string{}
	parents := map[string]bool{}
	for _, old := range d.Relations {
		if old.AgeID != r.AgeID || (r.ID != "" && old.ID == r.ID) {
			continue
		}
		op, oc, ok := parentPair(old)
		samePair := (old.Source == r.Source && old.Destination == r.Destination) || (old.Source == r.Destination && old.Destination == r.Source)
		if samePair && (r.Kind == old.Kind && (Registry.Relations[r.Kind].Reciprocal || old.Source == r.Source) || parent && ok && p == op && c == oc) {
			return fmt.Errorf("this family relationship already exists; edit its existing connection")
		}
		if samePair && ((parent && slices.Contains([]string{"sibling", "spouse", "partner"}, old.Kind)) || (ok && slices.Contains([]string{"sibling", "spouse", "partner"}, r.Kind))) {
			return fmt.Errorf("parent and child cannot also be siblings or partners in the same Age")
		}
		if ok {
			children[op] = append(children[op], oc)
			if oc == c {
				parents[op] = true
			}
		}
	}
	if !parent {
		return nil
	}
	if !parents[p] && len(parents) >= 2 {
		return fmt.Errorf("a child already has two recorded parents; edit its parentage")
	}
	seen := map[string]bool{}
	queue := []string{c}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if n == p {
			return fmt.Errorf("parentage would make a character their own ancestor (or both parent and child of another character)")
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		queue = append(queue, children[n]...)
	}
	return nil
}

// Old family/house/dynasty records become named views over the same characters.
// Preserve their original metadata and links as migration provenance, not active
// duplicate entities. Conversion is idempotent and is saved on the next write.
func (d *Document) MigrateFamilyViews() {
	legacy := map[string]bool{}
	for _, e := range d.Entities {
		if !slices.Contains([]string{"family", "house-family", "dynasty"}, e.Kind) {
			continue
		}
		key := e.AgeID + "/" + e.ID
		legacy[key] = true
		members := []string{}
		links := []Relation{}
		for _, r := range d.Relations {
			if r.AgeID != e.AgeID {
				continue
			}
			if r.Source == e.ID || r.Destination == e.ID || r.ContextID == e.ID {
				links = append(links, r)
			}
			if r.Kind == "member-of-family" && r.Destination == e.ID {
				if member := d.Entity(e.AgeID, r.Source); member != nil && !member.Deleted && member.Kind == "character" {
					members = append(members, r.Source)
				}
			}
		}
		for i := range d.World.Networks {
			v := &d.World.Networks[i]
			if v.AgeID == e.AgeID && v.FamilyID == e.ID {
				v.MemberIDs = append([]string(nil), members...)
				v.FamilyID = ""
				v.Genealogy = true
			}
		}
		exists := slices.ContainsFunc(d.World.Networks, func(v NetworkView) bool { return v.AgeID == e.AgeID && v.ID == e.ID })
		if !exists {
			data, _ := json.Marshal(map[string]any{"entity": e, "relations": links})
			d.World.Networks = append(d.World.Networks, NetworkView{ID: e.ID, AgeID: e.AgeID, Name: e.Name, Genealogy: true, Archived: e.Deleted, MemberIDs: members, LegacyData: data})
		}
	}
	if len(legacy) == 0 {
		return
	}
	d.Entities = slices.DeleteFunc(d.Entities, func(e Entity) bool { return legacy[e.AgeID+"/"+e.ID] })
	d.Relations = slices.DeleteFunc(d.Relations, func(r Relation) bool { return legacy[r.AgeID+"/"+r.Source] || legacy[r.AgeID+"/"+r.Destination] })
	for _, a := range d.Ages {
		d.pruneReferences(a.ID)
	}
}
