package project

import (
	"fmt"
	"sort"
)

type RevisionSummary struct {
	Revision string `json:"revision"`
	Message  string `json:"message"`
	SavedAt  string `json:"savedAt"`
	Ages     int    `json:"ages"`
}
type RevisionPage struct {
	Items []RevisionSummary `json:"items"`
	Next  string            `json:"next"`
}
type RestoreRequest struct {
	Expected string `json:"expected"`
	Revision string `json:"revision"`
	Apply    bool   `json:"apply"`
}
type AgeRestoreChange struct {
	ID      string `json:"id"`
	Before  string `json:"before"`
	After   string `json:"after"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
	Changed int    `json:"changed"`
}
type RestoreResult struct {
	Changes  []AgeRestoreChange `json:"changes"`
	Applied  bool               `json:"applied"`
	Revision string             `json:"revision"`
}

func (s *Store) Revisions(cursor string) (RevisionPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := RevisionPage{Items: []RevisionSummary{}}
	head, root, e := s.root()
	if e != nil {
		return out, e
	}
	if cursor != "" {
		nodes, e := s.revisionAncestry(head, root.World.ID)
		if e != nil {
			return out, e
		}
		if _, ok := nodes[cursor]; !ok {
			return out, fmt.Errorf("revision cursor is outside this world's history")
		}
		head = cursor
	}
	seen := map[string]bool{}
	for id := head; id != ""; {
		if seen[id] {
			return out, fmt.Errorf("revision cycle")
		}
		seen[id] = true
		if len(out.Items) == 50 {
			out.Next = id
			break
		}
		var r Root
		if e = s.read("revisions", id, &r); e != nil {
			return out, e
		}
		if r.World.ID != root.World.ID {
			return out, fmt.Errorf("revision belongs to another world")
		}
		out.Items = append(out.Items, RevisionSummary{id, r.Message, r.SavedAt, len(r.Ages)})
		id = r.Parent
	}
	return out, nil
}

func (s *Store) RestoreRevision(c RestoreRequest) (RestoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := RestoreResult{Changes: []AgeRestoreChange{}}
	head, current, e := s.root()
	if e != nil {
		return out, e
	}
	if c.Expected != head {
		return out, ErrConflict
	}
	nodes, e := s.revisionAncestry(head, current.World.ID)
	if e != nil {
		return out, e
	}
	if _, ok := nodes[c.Revision]; !ok {
		return out, fmt.Errorf("revision is outside this world's history")
	}
	var target Root
	if e = s.read("revisions", c.Revision, &target); e != nil {
		return out, e
	}
	ids := map[string]bool{}
	for id := range current.Ages {
		ids[id] = true
	}
	for id := range target.Ages {
		ids[id] = true
	}
	for id := range ids {
		before, after := current.Ages[id], target.Ages[id]
		if equalJSON(before, after) {
			continue
		}
		change := AgeRestoreChange{ID: id, Before: before.Name, After: after.Name}
		old, newer := Snapshot{Records: map[string]string{}}, Snapshot{Records: map[string]string{}}
		if before.Snapshot != "" {
			old, e = s.snapshot(before.Snapshot)
			if e != nil {
				return out, e
			}
		}
		if after.Snapshot != "" {
			newer, e = s.snapshot(after.Snapshot)
			if e != nil {
				return out, e
			}
		}
		for id, hash := range old.Records {
			if next, ok := newer.Records[id]; !ok {
				change.Removed++
			} else if hash != next {
				change.Changed++
			}
		}
		for id := range newer.Records {
			if _, ok := old.Records[id]; !ok {
				change.Added++
			}
		}
		out.Changes = append(out.Changes, change)
	}
	sort.Slice(out.Changes, func(i, j int) bool { return out.Changes[i].ID < out.Changes[j].ID })
	out.Revision = head
	if !c.Apply {
		return out, nil
	}
	if c.Revision == head {
		return out, fmt.Errorf("this revision is already current")
	}
	// A restoration is a new save. The current head remains its parent, keeping
	// all intervening work reachable for a later restoration or native backup.
	target.World = current.World
	target.Message = "Restored save from " + target.SavedAt + " (" + c.Revision[:12] + ")"
	out.Revision, e = s.commit(head, target)
	if e != nil {
		return out, e
	}
	out.Applied = true
	return out, nil
}
