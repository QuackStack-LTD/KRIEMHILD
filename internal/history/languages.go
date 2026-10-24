package history

import "fmt"

func (d *Document) validateLanguageLinks(candidate Entity) error {
	state := candidate.Language
	if state == nil {
		state = candidate.Script
	}
	if state == nil {
		return nil
	}
	for _, l := range state.Links {
		target := d.Entity(candidate.AgeID, l.ID)
		if target == nil || target.ID == candidate.ID {
			return fmt.Errorf("language relationship target must exist in this Age")
		}
		switch l.Kind {
		case "parent", "variety", "cognate", "contact":
			if target.Kind != "language" {
				return fmt.Errorf("language relationship requires another language")
			}
		case "script":
			if target.Kind != "script" {
				return fmt.Errorf("script association requires a script")
			}
		case "speaker", "region":
		default:
			return fmt.Errorf("unknown language relationship")
		}
		if l.Revision != "" {
			s := target.Language
			if s == nil {
				s = target.Script
			}
			if s.Ref(l.Revision) == nil {
				return fmt.Errorf("related definition revision is not retained")
			}
		}
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var walk func(Entity) error
	walk = func(e Entity) error {
		if visiting[e.ID] {
			return fmt.Errorf("language ancestry cannot contain cycles")
		}
		if done[e.ID] || e.Language == nil {
			return nil
		}
		visiting[e.ID] = true
		for _, l := range e.Language.Links {
			if l.Kind != "parent" {
				continue
			}
			if l.ID == candidate.ID {
				return fmt.Errorf("language ancestry cannot contain cycles")
			}
			parent := d.Entity(candidate.AgeID, l.ID)
			if parent != nil {
				if err := walk(*parent); err != nil {
					return err
				}
			}
		}
		delete(visiting, e.ID)
		done[e.ID] = true
		return nil
	}
	return walk(candidate)
}
