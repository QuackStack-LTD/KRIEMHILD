package project

import "fmt"

// recordEvent stores actual before/after object references and updates the
// overview and dated assertions in the same crash-safe commit and undo step.
func (s *Store) recordEvent(age Age, old string, command Command) (string, error) {
	if command.Event == nil || command.Event.Kind != "event" || command.Event.Event == nil {
		return "", fmt.Errorf("event is required")
	}
	snap, err := s.snapshot(old)
	if err != nil {
		return "", err
	}
	records, err := s.records(old)
	if err != nil {
		return "", err
	}
	// Exact event assertions remain usable even when the opening state is unknown.
	if chronology(records) == nil {
		metadata := Record{ID: NewID(), Kind: "chronology", Name: "Age chronology", Chronology: &Chronology{Age: age.ID, Baseline: old, Branch: "canon"}}
		hash, err := s.put("objects", metadata)
		if err != nil {
			return "", err
		}
		snap.Records[metadata.ID] = hash
	}
	r := *command.Event
	event := *r.Event
	r.Event = &event
	event.Age = age.ID
	event.Changes = nil
	event.Track = cleanTrack(event.Track)
	if _, ok := snap.Records[r.ID]; ok {
		return "", fmt.Errorf("event already exists")
	}
	if err = validateDate(event.Date); err != nil {
		return "", err
	}
	if event.Until != "" {
		if _, err := tick(event.Until); err != nil {
			return "", err
		}
		if event.Date.Precision != "exact" || cmp(event.Date.Tick, event.Until) >= 0 {
			return "", fmt.Errorf("validity end must follow exact event date")
		}
	}
	if len(command.Records) > 0 && event.Date.Precision != "exact" {
		return "", fmt.Errorf("dated state changes require an exact event date")
	}
	if len(command.Records) > 100 {
		return "", fmt.Errorf("at most 100 records per event")
	}
	seen := map[string]bool{}
	for _, after := range command.Records {
		if !safeNarrative(after) || seen[after.ID] || after.ID == r.ID {
			return "", fmt.Errorf("event changes must be unique entities, relations, maps or notes")
		}
		seen[after.ID] = true
		before := snap.Records[after.ID]
		if before != "" && records[after.ID].Kind != after.Kind {
			return "", fmt.Errorf("record kind cannot change")
		}
		hash, err := s.put("objects", after)
		if err != nil {
			return "", err
		}
		if hash == before {
			continue
		}
		// A new exact assertion closes the previous open assertion for that target.
		// Compound assertions require an explicit end to avoid ending unrelated facts.
		for id, prior := range records {
			if prior.Event == nil || prior.Event.Age != age.ID || prior.Event.Date.Precision != "exact" {
				continue
			}
			ev := *prior.Event
			ev.Changes = append([]Change{}, prior.Event.Changes...)
			for _, ch := range ev.Changes {
				if ch.Target != after.ID {
					continue
				}
				if ev.Until == "" && cmp(ev.Date.Tick, event.Date.Tick) < 0 {
					if len(ev.Changes) > 1 {
						return "", fmt.Errorf("%s has a compound open interval; give it an end date in Timeline before superseding part of it", prior.Name)
					}
					ev.Until = event.Date.Tick
					prior.Event = &ev
					h, err := s.put("objects", prior)
					if err != nil {
						return "", err
					}
					snap.Records[id] = h
					records[id] = prior
				} else if intervalsOverlap(&ev, &event) {
					return "", fmt.Errorf("event overlaps an existing validity interval")
				}
			}
		}
		event.Changes = append(event.Changes, Change{Target: after.ID, Before: before, After: hash})
		snap.Records[after.ID] = hash
	}
	if len(command.Records) > 0 && len(event.Changes) == 0 {
		return "", fmt.Errorf("the proposed event changes no records")
	}
	hash, err := s.put("objects", r)
	if err != nil {
		return "", err
	}
	snap.Records[r.ID] = hash
	return s.put("snapshots", snap)
}
