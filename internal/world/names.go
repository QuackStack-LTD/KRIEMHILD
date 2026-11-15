package world

import (
	"encoding/json"
	"strings"
)

// RefreshNames leaves drawable geometry, local edits, the spatial index and
// raster revision intact. Canonical labels also follow geometric undo/redo.
func (s *State) RefreshNames(names map[string]Entity) {
	if len(names) == 0 {
		return
	}
	s.rebaseNames(names)
	for id, named := range names {
		if e, ok := s.Entities[id]; ok && e.RecordStatus != "draft" {
			e.Name, e.LabelName = named.Name, named.LabelName
			s.Entities[id] = e
		}
	}
}

// Rebase names without replacing pending geometry or terrain operations. Undo
// of an older geometric edit must not resurrect a superseded canonical name.
func (s *State) rebaseNames(canonical map[string]Entity) {
	project := func(e Entity) Entity {
		if named, ok := canonical[e.ID]; ok && e.RecordStatus != "draft" && s.Entities[e.ID].Name != named.Name {
			e.Name, e.LabelName = named.Name, named.LabelName
		}
		return e
	}
	for i := range s.Header.History {
		c := &s.Header.History[i]
		for _, raw := range []*json.RawMessage{&c.Before, &c.After} {
			if strings.HasPrefix(c.Key, "entity/") {
				var e Entity
				if json.Unmarshal(*raw, &e) == nil && e.ID != "" {
					*raw, _ = json.Marshal(project(e))
				}
			} else if c.Key == "entities" {
				var entities []Entity
				if json.Unmarshal(*raw, &entities) == nil {
					for j := range entities {
						entities[j] = project(entities[j])
					}
					*raw, _ = json.Marshal(entities)
				}
			}
		}
	}
	for id, art := range s.Header.Illustrations {
		for key, o := range art.Objects {
			for _, e := range canonical {
				old := s.Entities[e.ID]
				if old.Name != e.Name && o.Text == old.Name && o.Source != "" && (o.Source == e.ID || o.Source == e.SourceID || o.Source == e.LogicalID) && o.Kind == "text" {
					o.Text = e.Name
					art.Objects[key] = o
					break
				}
			}
		}
		s.Header.Illustrations[id] = art
	}
}
