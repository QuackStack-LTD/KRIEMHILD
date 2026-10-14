package world

import (
	"fmt"
	"sort"
)

// validateBatch checks the final graph without mutating the live state. This
// permits parent and child creation/deletion in the same atomic history entry.
func (s *State) validateBatch(entities []Entity) error {
	if s.Header.Version != 2 || len(entities) == 0 || len(entities) > 5000 {
		return fmt.Errorf("invalid entity batch")
	}
	next := *s
	next.Entities = make(map[string]Entity, len(s.Entities)+len(entities))
	for id, e := range s.Entities {
		next.Entities[id] = e
	}
	seen := map[string]bool{}
	for _, e := range entities {
		if seen[e.ID] {
			return fmt.Errorf("duplicate batch identity")
		}
		seen[e.ID] = true
		if err := s.editable(e.Layer); err != nil {
			return err
		}
		if old, ok := s.Entities[e.ID]; ok {
			if err := s.editable(old.Layer); err != nil {
				return err
			}
		}
		if !AllowedGeometry(e.Layer, e.Kind, e.Geometry) || e.River != nil {
			return fmt.Errorf("unsupported batch geometry")
		}
		next.Entities[e.ID] = e
	}
	for _, e := range next.Entities {
		if e.Deleted {
			e.ParentID = ""
		}
		if err := next.ValidateEntity(e); err != nil {
			return err
		}
	}
	return nil
}

func (s *State) PutEntities(entities []Entity) error {
	if err := s.validateBatch(entities); err != nil {
		return err
	}
	before := make([]Entity, 0, len(entities))
	for _, e := range entities {
		old, ok := s.Entities[e.ID]
		if !ok {
			old = e
			old.Deleted = true
		}
		before = append(before, old)
	}
	s.record(Change{Key: "entities", Before: raw(before), After: raw(entities)})
	return nil
}

func (s *State) MoveFamily(id string, dx, dy float64) error {
	root, ok := s.Entities[id]
	if !ok || root.Deleted || !finite(dx) || !finite(dy) {
		return fmt.Errorf("invalid move")
	}
	ids := []string{id}
	for head := 0; head < len(ids); head++ {
		children := []string{}
		for _, e := range s.Entities {
			if !e.Deleted && e.ParentID == ids[head] {
				children = append(children, e.ID)
			}
		}
		sort.Strings(children)
		ids = append(ids, children...)
	}
	entities := make([]Entity, 0, len(ids))
	for _, id := range ids {
		e := s.Entities[id]
		e.Points = append([][2]float64(nil), e.Points...)
		for i := range e.Points {
			e.Points[i][0] += dx
			e.Points[i][1] += dy
		}
		e.Cells = nil
		entities = append(entities, e)
	}
	return s.PutEntities(entities)
}
