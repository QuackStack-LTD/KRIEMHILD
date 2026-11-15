package history

import (
	"encoding/json"
	"kriemhild/internal/biosphere"
	"kriemhild/internal/languages"
	"kriemhild/internal/world"
)

// Index the destination once, and clone only source records that will be used.
// Iterating the original slice length also prevents newly appended records from
// being inherited again. Conflicting identities retain the merge policy.
func inheritRecords[T any](records []T, from, to string, overwrite bool, identity func(T) (string, string), clone func(T) T) []T {
	destination := map[string]int{}
	for i, record := range records {
		age, id := identity(record)
		if age == to {
			destination[id] = i
		}
	}
	for _, record := range records {
		age, id := identity(record)
		if age != from {
			continue
		}
		index, exists := destination[id]
		if exists && !overwrite {
			continue
		}
		copied := clone(record)
		if exists {
			records[index] = copied
		} else {
			destination[id] = len(records)
			records = append(records, copied)
		}
	}
	return records
}

func (d *Document) inherit(from, to string, overwrite bool) {
	// Archive inheritance depends on the destination entities already existing.
	defer d.inheritLibrary(from, to, overwrite)
	d.World.Networks = inheritRecords(d.World.Networks, from, to, overwrite, func(v NetworkView) (string, string) { return v.AgeID, v.ID }, func(v NetworkView) NetworkView {
		v.AgeID = to
		v.RelationTypes = append([]string(nil), v.RelationTypes...)
		v.MemberIDs = append([]string(nil), v.MemberIDs...)
		v.LegacyData = append(json.RawMessage(nil), v.LegacyData...)
		return v
	})
	d.Biospheres = inheritRecords(d.Biospheres, from, to, overwrite, func(v biosphere.State) (string, string) { return v.AgeID, v.ID }, func(v biosphere.State) biosphere.State {
		v = biosphere.CloneState(v)
		v.AgeID = to
		return v
	})
	d.Terrains = inheritRecords(d.Terrains, from, to, overwrite, func(v Terrain) (string, string) { return v.AgeID, v.ID }, func(v Terrain) Terrain {
		v.AgeID = to
		return v
	})
	d.Maps = inheritRecords(d.Maps, from, to, overwrite, func(v Map) (string, string) { return v.AgeID, v.ID }, func(v Map) Map {
		v.AgeID, v.ProjectID = to, ID()
		return v
	})
	d.Entities = inheritRecords(d.Entities, from, to, overwrite, func(v Entity) (string, string) { return v.AgeID, v.ID }, func(v Entity) Entity {
		if v.Language != nil {
			v.Language = languages.Clone(v.Language)
		}
		if v.Script != nil {
			v.Script = languages.Clone(v.Script)
		}
		v.Properties = append(json.RawMessage(nil), v.Properties...)
		v.Content = cloneContent(v.Content)
		v.System = cloneSystem(v.System)
		v.Archive = cloneArchive(v.Archive)
		v.Biology = biosphere.CloneLifeform(v.Biology)
		v.Names = append([]EntityName(nil), v.Names...)
		if v.ContinuityID == "" {
			v.ContinuityID = v.ID
		}
		v.AgeID = to
		return v
	})
	d.Representations = inheritRecords(d.Representations, from, to, overwrite, func(v Representation) (string, string) { return v.AgeID, v.ID }, func(v Representation) Representation {
		shapeData, _ := json.Marshal(v.Shape)
		var shape world.Entity
		_ = json.Unmarshal(shapeData, &shape)
		v.Shape, v.AgeID = shape, to
		return v
	})
	d.Relations = inheritRecords(d.Relations, from, to, overwrite, func(v Relation) (string, string) { return v.AgeID, v.ID }, func(v Relation) Relation {
		v.AgeID = to
		return v
	})
}
