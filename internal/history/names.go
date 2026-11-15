package history

import (
	"fmt"
	"strings"
	"time"
)

// Names belong to logical identities in one Age. Drawables and derived map
// titles are projections of those identities, never independent rename targets.
func (d *Document) RenameEntity(age, id, name string) error {
	e := d.Entity(age, id)
	if e == nil || e.Deleted {
		return fmt.Errorf("active entity not found in this Age")
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 200 {
		return fmt.Errorf("provide a name of at most 200 characters")
	}
	for _, r := range d.Representations {
		if r.AgeID == age && r.EntityID == id && len(name) > 160 {
			return fmt.Errorf("mapped names must fit within 160 characters")
		}
	}
	old := e.Name
	e.Name = name
	e.UpdatedAt = time.Now().UnixMilli()
	d.projectEntityName(age, id, old, name)
	return nil
}

func (d *Document) projectEntityName(age, id, old, name string) {
	if e := d.Entity(age, id); e != nil {
		for i := range e.Names {
			if e.Names[i].Type == "canonical" && e.Names[i].Name == old {
				e.Names[i].Name = name
			}
		}
	}
	for i := range d.Representations {
		r := &d.Representations[i]
		if r.AgeID == age && r.EntityID == id {
			r.Shape.Name = name
			if r.Shape.LabelName == old {
				r.Shape.LabelName = name
			}
		}
	}
	for i := range d.Maps {
		m := &d.Maps[i]
		if m.AgeID == age && m.EntityID == id {
			for _, suffix := range []string{"", " - settlement map", " atlas"} {
				if m.Name == old+suffix {
					m.Name = name + suffix
					break
				}
			}
		}
	}
}

func (d *Document) RenameTerrain(age, id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 160 {
		return fmt.Errorf("provide a terrain name of at most 160 characters")
	}
	for i := range d.Terrains {
		t := &d.Terrains[i]
		if t.AgeID == age && t.ID == id {
			t.Name = name
			for j := range d.Maps {
				m := &d.Maps[j]
				if m.AgeID == age && m.TerrainID == id && m.ParentID == "" {
					m.Name = name
				}
			}
			return nil
		}
	}
	return fmt.Errorf("terrain not found in this Age")
}
