package project

import "fmt"

// Native revisions retain the second parent independently of Git metadata, so
// repeated synchronization and archive migration see the same authored history.
func (s *Store) revisionAncestry(head, world string) (map[string][]string, error) {
	nodes := map[string][]string{}
	queue := []string{head}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if _, ok := nodes[id]; ok {
			continue
		}
		if len(nodes) >= 10000 {
			return nil, fmt.Errorf("revision ancestry exceeds merge budget")
		}
		if !validHash(id) {
			return nil, fmt.Errorf("invalid revision ancestor")
		}
		var r Root
		if e := s.read("revisions", id, &r); e != nil {
			return nil, e
		}
		if r.World.ID != world || r.Version != Format || len(r.MergeParents) > 8 {
			return nil, fmt.Errorf("invalid revision ancestry")
		}
		parents := []string{}
		if r.Parent != "" {
			parents = append(parents, r.Parent)
		}
		parents = append(parents, r.MergeParents...)
		nodes[id] = parents
		queue = append(queue, parents...)
	}
	// Content addressing makes a cycle impractical, but reject corrupt graphs
	// explicitly rather than silently accepting a repeated node as an ancestor.
	colors := map[string]uint8{}
	var visit func(string) error
	visit = func(id string) error {
		if colors[id] == 1 {
			return fmt.Errorf("revision cycle")
		}
		if colors[id] == 2 {
			return nil
		}
		colors[id] = 1
		for _, p := range nodes[id] {
			if e := visit(p); e != nil {
				return e
			}
		}
		colors[id] = 2
		return nil
	}
	if e := visit(head); e != nil {
		return nil, e
	}
	return nodes, nil
}

func mergeBase(local, remote map[string][]string) (string, error) {
	common := map[string]bool{}
	for id := range local {
		if _, ok := remote[id]; ok {
			common[id] = true
		}
	}
	candidates := map[string]bool{}
	for id := range common {
		candidates[id] = true
	}
	for id := range common {
		for _, parent := range local[id] {
			delete(candidates, parent)
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no shared project revision")
	}
	if len(candidates) > 1 {
		return "", fmt.Errorf("multiple common revision bases; reconcile this criss-cross history before merging")
	}
	for id := range candidates {
		return id, nil
	}
	panic("unreachable merge base")
}
