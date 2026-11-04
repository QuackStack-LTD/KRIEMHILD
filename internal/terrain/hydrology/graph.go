// Package hydrology derives explicit reach topology from persisted river paths.
// It never moves vertices, invents drainage edges, or modifies saved worlds.
package hydrology

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"kriemhild/internal/terrain/model"
	"math"
	"sort"
)

type Node struct {
	ID       string      `json:"id"`
	Point    model.Point `json:"point"`
	Kind     string      `json:"kind"`
	Incoming []string    `json:"incoming"`
	Outgoing []string    `json:"outgoing"`
	Lake     string      `json:"lake,omitempty"`
}
type Reach struct {
	ID        string        `json:"id"`
	River     string        `json:"river"`
	From      string        `json:"from"`
	To        string        `json:"to"`
	Basin     int           `json:"basin"`
	Path      []model.Point `json:"path"`
	Discharge float64       `json:"discharge"`
	Source    string        `json:"source,omitempty"`
	Terminal  string        `json:"terminal,omitempty"`
}
type Issue struct {
	Code   string `json:"code"`
	River  string `json:"river"`
	Vertex int    `json:"vertex"`
}
type Graph struct {
	Revision string         `json:"revision"`
	Nodes    []Node         `json:"nodes"`
	Reaches  []Reach        `json:"reaches"`
	Issues   []Issue        `json:"issues"`
	Counts   map[string]int `json:"counts"`
}

func pointKey(p model.Point) string {
	return fmt.Sprintf("%.12f:%.12f", model.Wrap(p[0]), p[1])
}
func Build(p *model.Planet) Graph {
	b, _ := json.Marshal(struct {
		Rivers []model.River `json:"rivers"`
		Lakes  []model.Lake  `json:"lakes"`
	}{p.Rivers, p.Lakes})
	g := Graph{Revision: fmt.Sprintf("%x", sha256.Sum256(b)), Counts: map[string]int{}, Issues: []Issue{}, Nodes: []Node{}, Reaches: []Reach{}}
	issue := func(code, id string, vertex int) {
		g.Counts[code]++
		if len(g.Issues) < 200 {
			g.Issues = append(g.Issues, Issue{code, id, vertex})
		}
	}
	anchors := map[string]bool{}
	byID := map[string]model.River{}
	vertices := map[string]map[string]bool{}
	valid := map[string]bool{}
	for _, r := range p.Rivers {
		if _, exists := byID[r.ID]; exists {
			issue("duplicate-id", r.ID, 0)
		}
		byID[r.ID] = r
		if len(r.Path) < 2 || len(r.Surface) != len(r.Path) || len(r.Bed) != len(r.Path) || len(r.Discharge) != len(r.Path) || len(r.Width) != len(r.Path) {
			issue("invalid-geometry", r.ID, 0)
			continue
		}
		valid[r.ID] = true
		vertices[r.ID] = map[string]bool{}
		for i, pt := range r.Path {
			if !model.Finite(pt[0]) || !model.Finite(pt[1]) || math.Abs(pt[1]) > math.Pi/2 || !model.Finite(r.Surface[i]) || !model.Finite(r.Discharge[i]) || r.Discharge[i] < 0 {
				issue("invalid-sample", r.ID, i)
				valid[r.ID] = false
			}
			vertices[r.ID][pointKey(pt)] = true
			if i > 0 && r.Surface[i] > r.Surface[i-1]+1e-6 {
				issue("uphill-flow", r.ID, i)
			}
			if r.Bed[i] > r.Surface[i]+1e-6 {
				issue("invalid-water-depth", r.ID, i)
			}
			if p.Surface != nil && len(p.Surface.Bed) == p.Surface.Width*p.Surface.Height && p.Surface.At(p.Surface.Bed, pt[0], pt[1]) > r.Surface[i]+100 {
				// Carving lowers the bed; expose deep cuts for review, do not
				// automatically reroute a reach across an unvalidated corridor.
				issue("deep-terrain-cut", r.ID, i)
			}
		}
		anchors[pointKey(r.Path[0])] = true
		anchors[pointKey(r.Path[len(r.Path)-1])] = true
	}
	nodes := map[string]*Node{}
	node := func(pt model.Point) *Node {
		key := pointKey(pt)
		if nodes[key] == nil {
			nodes[key] = &Node{ID: key, Point: pt, Incoming: []string{}, Outgoing: []string{}, Lake: lakeAt(p, pt)}
		}
		return nodes[key]
	}
	for _, r := range p.Rivers {
		if !valid[r.ID] {
			continue
		}
		last := len(r.Path) - 1
		if r.Parent != "" && !vertices[r.Parent][pointKey(r.Path[0])] {
			issue("unexplained-source", r.ID, 0)
		}
		if r.Destination != "ocean" && r.Destination != "basin" {
			if !vertices[r.Destination][pointKey(r.Path[last])] {
				issue("mismatched-confluence", r.ID, last)
			}
			if other, ok := byID[r.Destination]; ok && r.Basin != 0 && other.Basin != 0 && r.Basin != other.Basin {
				issue("basin-mismatch", r.ID, last)
			}
		} else if r.Destination == "ocean" && p.Surface != nil && math.Abs(p.Surface.At(p.Surface.Coast, r.Path[last][0], r.Path[last][1])) > .001 {
			issue("invalid-mouth", r.ID, last)
		}
		seen := map[string]bool{}
		for id := r.ID; id != "ocean" && id != "basin"; {
			if seen[id] {
				issue("cycle", r.ID, last)
				break
			}
			seen[id] = true
			next, ok := byID[id]
			if !ok {
				issue("disconnected-reach", r.ID, last)
				break
			}
			id = next.Destination
		}
		start := 0
		previousLake := lakeAt(p, r.Path[0])
		for i := 1; i <= last; i++ {
			lake := lakeAt(p, r.Path[i])
			if i != last && !anchors[pointKey(r.Path[i])] && lake == previousLake {
				continue
			}
			a, z := node(r.Path[start]), node(r.Path[i])
			reach := Reach{ID: fmt.Sprintf("%s@%d", r.ID, start), River: r.ID, From: a.ID, To: z.ID, Basin: r.Basin, Path: r.Path[start : i+1], Discharge: r.Discharge[i]}
			if start == 0 && r.Parent == "" {
				reach.Source = "headwater"
			}
			if i == last && (r.Destination == "ocean" || r.Destination == "basin") {
				reach.Terminal = r.Destination
			}
			g.Reaches = append(g.Reaches, reach)
			a.Outgoing = append(a.Outgoing, reach.ID)
			z.Incoming = append(z.Incoming, reach.ID)
			start = i
			previousLake = lake
		}
	}
	for i := range g.Reaches {
		r := &g.Reaches[i]
		if len(nodes[r.From].Incoming) > 0 {
			r.Source = ""
		}
		if r.Terminal != "" && len(nodes[r.To].Outgoing) > 0 {
			issue("terminal-has-downstream", r.River, len(r.Path)-1)
		}
	}
	for _, n := range nodes {
		n.Kind = "junction"
		switch {
		case len(n.Incoming) == 0:
			n.Kind = "headwater"
			g.Counts["sources"]++
		case len(n.Outgoing) == 0:
			n.Kind = "terminal"
			g.Counts["terminals"]++
		case len(n.Outgoing) > 1:
			n.Kind = "divergence"
			g.Counts["divergences"]++
		case len(n.Incoming) > 1:
			n.Kind = "confluence"
			g.Counts["confluences"]++
		}
		if n.Lake != "" {
			g.Counts["lake-nodes"]++
		}
		g.Nodes = append(g.Nodes, *n)
	}
	sort.Slice(g.Nodes, func(i, j int) bool { return g.Nodes[i].ID < g.Nodes[j].ID })
	return g
}

func lakeAt(p *model.Planet, pt model.Point) string {
	if s := p.Surface; s != nil {
		if len(s.LakeLevel) != s.Width*s.Height || s.At(s.LakeLevel, pt[0], pt[1]) <= s.At(s.Bed, pt[0], pt[1]) || s.At(s.LakeLevel, pt[0], pt[1]) <= 0 {
			return ""
		}
	}
	best, distance := -1, math.Inf(1)
	for i, l := range p.Lakes {
		d := math.Hypot(model.Wrap(pt[0]-l.Center[0])*math.Cos(pt[1]), pt[1]-l.Center[1])
		if d <= l.Radius && d < distance {
			best, distance = i, d
		}
	}
	if best >= 0 {
		return fmt.Sprintf("lake-%d", best)
	}
	return ""
}
