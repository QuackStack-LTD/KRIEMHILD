package world

import "sort"

// A loose-free quadtree stores each object once, in the smallest containing
// node. Large roads/borders stay in ancestors; buildings live in leaf buckets.
type Bounds struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

func (a Bounds) Intersects(b Bounds) bool {
	return a.X <= b.X+b.Width && a.X+a.Width >= b.X && a.Y <= b.Y+b.Height && a.Y+a.Height >= b.Y
}
func (a Bounds) contains(b Bounds) bool {
	return b.X >= a.X && b.Y >= a.Y && b.X+b.Width <= a.X+a.Width && b.Y+b.Height <= a.Y+a.Height
}
func (a Bounds) Expand(n float64) Bounds {
	return Bounds{a.X - n, a.Y - n, a.Width + 2*n, a.Height + 2*n}
}

type indexItem struct {
	ID    string
	Box   Bounds
	Level float64
}
type node struct {
	box      Bounds
	depth    int
	minLevel float64
	items    map[string]indexItem
	children [4]*node
}
type Index struct {
	root      *node
	locations map[string]*node
}

func NewIndex(w, h float64) *Index {
	return &Index{&node{box: Bounds{0, 0, w, h}, minLevel: 9, items: map[string]indexItem{}}, map[string]*node{}}
}
func (q *Index) Remove(id string) {
	if n := q.locations[id]; n != nil {
		delete(n.items, id)
		delete(q.locations, id)
	}
}
func (q *Index) Put(id string, b Bounds, level float64) {
	q.Remove(id)
	q.put(q.root, indexItem{id, b, level})
}
func (q *Index) put(n *node, v indexItem) {
	n.minLevel = min(n.minLevel, v.Level)
	if n.depth < 10 {
		if n.children[0] == nil && len(n.items) >= 16 {
			for k := 0; k < 4; k++ {
				n.children[k] = &node{box: Bounds{n.box.X + float64(k%2)*n.box.Width/2, n.box.Y + float64(k/2)*n.box.Height/2, n.box.Width / 2, n.box.Height / 2}, depth: n.depth + 1, minLevel: 9, items: map[string]indexItem{}}
			}
			for id, old := range n.items {
				for _, c := range n.children {
					if c.box.contains(old.Box) {
						delete(n.items, id)
						q.put(c, old)
						break
					}
				}
			}
		}
		for _, c := range n.children {
			if c != nil && c.box.contains(v.Box) {
				q.put(c, v)
				return
			}
		}
	}
	n.items[v.ID] = v
	q.locations[v.ID] = n
}
func (q *Index) Query(b Bounds, level float64, limit int) ([]string, bool) {
	return q.QueryWhere(b, level, limit, nil)
}
func (q *Index) QueryWhere(b Bounds, level float64, limit int, include func(string) bool) ([]string, bool) {
	out := []string{}
	truncated := false
	var visit func(*node)
	visit = func(n *node) {
		if n == nil || n.minLevel > level || !n.box.Intersects(b) || truncated {
			return
		}
		keys := make([]string, 0, len(n.items))
		for id := range n.items {
			keys = append(keys, id)
		}
		sort.Strings(keys)
		for _, id := range keys {
			v := n.items[id]
			if v.Level <= level && v.Box.Intersects(b) && (include == nil || include(id)) {
				if len(out) >= limit {
					truncated = true
					return
				}
				out = append(out, v.ID)
			}
		}
		for _, c := range n.children {
			visit(c)
		}
	}
	visit(q.root)
	return out, truncated
}
