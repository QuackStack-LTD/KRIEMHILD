package project

import (
	"fmt"
	"sort"
	"strings"
)

type GraphRequest struct {
	Snapshot  string   `json:"snapshot"`
	Tick      string   `json:"tick"`
	Start     string   `json:"start"`
	Direction string   `json:"direction"`
	Roles     []string `json:"roles"`
	Depth     int      `json:"depth"`
}
type GraphNode struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Depth int    `json:"depth"`
}
type RelationshipGraph struct {
	Nodes      []GraphNode `json:"nodes"`
	Edges      []Record    `json:"edges"`
	Truncated  bool        `json:"truncated"`
	Unresolved []string    `json:"unresolved"`
}

func (s *Store) RelationshipGraph(c GraphRequest) (RelationshipGraph, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := RelationshipGraph{Nodes: []GraphNode{}, Edges: []Record{}, Unresolved: []string{}}
	if c.Depth < 1 || c.Depth > 8 || len(c.Roles) > 100 || (c.Direction != "ancestors" && c.Direction != "descendants" && c.Direction != "connected") {
		return out, fmt.Errorf("choose a direction and depth of 1–8")
	}
	records, e := s.records(c.Snapshot)
	if e != nil {
		return out, e
	}
	if c.Tick != "" {
		historical, e := s.historical(c.Snapshot, c.Tick)
		if e != nil {
			return out, e
		}
		records = historical.Records
		out.Unresolved = historical.Unresolved
	}
	if records[c.Start].Kind != "entity" {
		return out, fmt.Errorf("choose an entity present in this context")
	}
	roles := map[string]bool{}
	for _, r := range c.Roles {
		roles[strings.TrimSpace(r)] = true
	}
	edges := []Record{}
	for _, r := range records {
		if r.Kind == "relation" && (len(roles) == 0 || roles[r.Name]) {
			edges = append(edges, r)
		}
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	depth := map[string]int{c.Start: 0}
	queue := []string{c.Start}
	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]
		if depth[at] >= c.Depth {
			continue
		}
		for _, r := range edges {
			to := ""
			if r.From == at && (c.Direction == "descendants" || c.Direction == "connected") {
				to = r.To
			}
			if r.To == at && (c.Direction == "ancestors" || c.Direction == "connected") {
				to = r.From
			}
			if to == "" {
				continue
			}
			if _, seen := depth[to]; seen {
				continue
			}
			if len(depth) >= 200 {
				out.Truncated = true
				continue
			}
			depth[to] = depth[at] + 1
			queue = append(queue, to)
		}
	}
	for id, d := range depth {
		r := records[id]
		out.Nodes = append(out.Nodes, GraphNode{id, r.Name, r.Type, d})
	}
	sort.Slice(out.Nodes, func(i, j int) bool {
		a, b := out.Nodes[i], out.Nodes[j]
		if a.Depth != b.Depth {
			return a.Depth < b.Depth
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.ID < b.ID
	})
	for _, r := range edges {
		_, from := depth[r.From]
		_, to := depth[r.To]
		if from && to {
			if len(out.Edges) >= 500 {
				out.Truncated = true
				break
			}
			out.Edges = append(out.Edges, r)
		}
	}
	return out, nil
}
