package project

import (
	"fmt"
	"hash/fnv"
	"math"
	"sort"
)

// Terrain v1 is a bounded regional grid. Accepted samples, not only a seed,
// are stored so future generator changes cannot rewrite a world's geography.
type Terrain struct {
	Algorithm string `json:"algorithm"`
	Seed      string `json:"seed"`
	Columns   int    `json:"columns"`
	Rows      int    `json:"rows"`
	Sea       int    `json:"sea"`
	Heights   []int  `json:"heights"`
	Water     []int  `json:"water"` // 0 land, 1 ocean, 2 enclosed water
	Flow      []int  `json:"flow"`  // downstream cell or -1; strict descent, no cycles
	Wrap      bool   `json:"wrap"`
	Unit      string `json:"unit"`
}
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type Feature struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Kind   string  `json:"kind"`
	Points []Point `json:"points"`
	Entity string  `json:"entity,omitempty"`
	Notes  string  `json:"notes,omitempty"`
}
type TerrainRequest struct {
	Expected  string  `json:"expected"`
	Age       string  `json:"age"`
	Map       string  `json:"map"`
	Operation string  `json:"operation"`
	Seed      string  `json:"seed"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Radius    float64 `json:"radius"`
	Strength  int     `json:"strength"`
	Sea       int     `json:"sea"`
	Wrap      bool    `json:"wrap"`
}
type Impact struct {
	Entity string `json:"entity"`
	Name   string `json:"name"`
	Change string `json:"change"`
}
type TerrainPreview struct {
	Record   Record   `json:"record"`
	Impacts  []Impact `json:"impacts"`
	Changed  int      `json:"changed"`
	Revision string   `json:"revision"`
}

func neighbors(t *Terrain, i int) []int {
	x, y := i%t.Columns, i/t.Columns
	out := []int{}
	if y > 0 {
		out = append(out, i-t.Columns)
	}
	if y < t.Rows-1 {
		out = append(out, i+t.Columns)
	}
	if x > 0 {
		out = append(out, i-1)
	} else if t.Wrap {
		out = append(out, i+t.Columns-1)
	}
	if x < t.Columns-1 {
		out = append(out, i+1)
	} else if t.Wrap {
		out = append(out, i-t.Columns+1)
	}
	return out
}
func derive(t *Terrain) {
	n := len(t.Heights)
	t.Water = make([]int, n)
	t.Flow = make([]int, n)
	queue := []int{}
	for i, h := range t.Heights {
		t.Flow[i] = -1
		if h <= t.Sea {
			t.Water[i] = 2
			if i/t.Columns == 0 || i/t.Columns == t.Rows-1 || (!t.Wrap && (i%t.Columns == 0 || i%t.Columns == t.Columns-1)) {
				t.Water[i] = 1
				queue = append(queue, i)
			}
		}
	}
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		for _, j := range neighbors(t, i) {
			if t.Water[j] == 2 {
				t.Water[j] = 1
				queue = append(queue, j)
			}
		}
	}
	for i := range t.Heights {
		if t.Water[i] != 0 {
			continue
		}
		lowest := i
		for _, j := range neighbors(t, i) {
			if t.Heights[j] < t.Heights[lowest] {
				lowest = j
			}
		}
		if lowest != i {
			t.Flow[i] = lowest
		}
	}
}
func generated(seed string, wrap bool) *Terrain {
	t := &Terrain{Algorithm: "regional-grid-v1", Seed: seed, Columns: 64, Rows: 48, Sea: 0, Wrap: wrap, Unit: "relative elevation"}
	h := fnv.New32a()
	h.Write([]byte(seed))
	state := h.Sum32()
	if state == 0 {
		state = 1
	}
	random := func() float64 {
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5
		return float64(state) / 4294967295
	}
	coarse := make([]float64, 9*7)
	for i := range coarse {
		coarse[i] = random()
	}
	if wrap {
		for y := 0; y < 7; y++ {
			coarse[y*9+8] = coarse[y*9]
		}
	}
	for y := 0; y < t.Rows; y++ {
		for x := 0; x < t.Columns; x++ {
			fx, fy := float64(x)*8/float64(t.Columns-1), float64(y)*6/float64(t.Rows-1)
			ix, iy := int(fx), int(fy)
			if ix == 8 {
				ix = 7
			}
			if iy == 6 {
				iy = 5
			}
			dx, dy := fx-float64(ix), fy-float64(iy)
			dx = dx * dx * (3 - 2*dx)
			dy = dy * dy * (3 - 2*dy)
			a := coarse[iy*9+ix]*(1-dx) + coarse[iy*9+ix+1]*dx
			b := coarse[(iy+1)*9+ix]*(1-dx) + coarse[(iy+1)*9+ix+1]*dx
			nx, ny := float64(x)/float64(t.Columns-1)*2-1, float64(y)/float64(t.Rows-1)*2-1
			edge := ny * ny
			if !wrap {
				edge = math.Max(nx*nx, edge)
			}
			t.Heights = append(t.Heights, int(math.Round(((a*(1-dy)+b*dy)*1.5-.45-edge*.65)*900)))
		}
	}
	derive(t)
	return t
}
func validateTerrain(t *Terrain) error {
	if t == nil {
		return nil
	}
	n := t.Columns * t.Rows
	if t.Algorithm != "regional-grid-v1" || len(t.Seed) > 300 || t.Columns < 2 || t.Rows < 2 || t.Columns > 128 || t.Rows > 128 || len(t.Heights) != n || len(t.Water) != n || len(t.Flow) != n || t.Sea < -2000 || t.Sea > 2000 || !nameOK(t.Unit) {
		return fmt.Errorf("invalid terrain grid")
	}
	for _, v := range t.Heights {
		if v < -2000 || v > 2000 {
			return fmt.Errorf("elevation outside supported range")
		}
	}
	check := *t
	derive(&check)
	for i := range t.Heights {
		if t.Water[i] != check.Water[i] || t.Flow[i] != check.Flow[i] {
			return fmt.Errorf("terrain water and drainage must be recomputed")
		}
	}
	return nil
}
func (s *Store) PreviewTerrain(c TerrainRequest) (TerrainPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.state(c.Age)
	if err != nil {
		return TerrainPreview{}, err
	}
	if state.Revision != c.Expected {
		return TerrainPreview{}, ErrConflict
	}
	r, ok := state.Records[c.Map]
	if !ok || r.Kind != "map" {
		return TerrainPreview{}, fmt.Errorf("map not found")
	}
	before := r.Terrain
	if c.Operation == "generate" {
		if !nameOK(c.Seed) {
			return TerrainPreview{}, fmt.Errorf("seed is required")
		}
		r.Terrain = generated(c.Seed, c.Wrap)
	} else {
		if before == nil {
			return TerrainPreview{}, fmt.Errorf("generate terrain first")
		}
		t := *before
		t.Heights = append([]int{}, before.Heights...)
		r.Terrain = &t
		if c.Operation == "sea" {
			if c.Sea < -2000 || c.Sea > 2000 {
				return TerrainPreview{}, fmt.Errorf("sea level must be between -2000 and 2000")
			}
			t.Sea = c.Sea
		} else {
			if math.IsNaN(c.X) || math.IsNaN(c.Y) || math.IsNaN(c.Radius) || c.X < 0 || c.X > 1 || c.Y < 0 || c.Y > 1 || c.Radius <= 0 || c.Radius > 1 || c.Strength < 1 || c.Strength > 2000 {
				return TerrainPreview{}, fmt.Errorf("invalid terrain brush")
			}
			for i, h := range before.Heights {
				dx := math.Abs(float64(i%t.Columns)/float64(t.Columns-1) - c.X)
				if t.Wrap {
					dx = math.Min(dx, 1-dx)
				}
				dy := float64(i/t.Columns)/float64(t.Rows-1) - c.Y
				d := math.Hypot(dx, dy) / c.Radius
				if d > 1 {
					continue
				}
				amount := int(math.Round(float64(c.Strength) * (1 - d)))
				switch c.Operation {
				case "raise":
					t.Heights[i] = h + amount
				case "lower":
					t.Heights[i] = h - amount
				case "flatten":
					t.Heights[i] = int(math.Round(float64(h)*d + float64(c.Sea)*(1-d)))
				case "smooth":
					sum := h
					ns := neighbors(&t, i)
					for _, j := range ns {
						sum += before.Heights[j]
					}
					t.Heights[i] = int(math.Round(float64(h)*d + float64(sum)/float64(len(ns)+1)*(1-d)))
				case "flood":
					t.Heights[i] = min(h, t.Sea-amount-1)
				case "drain":
					t.Heights[i] = max(h, t.Sea+amount+1)
				default:
					return TerrainPreview{}, fmt.Errorf("unknown terrain operation")
				}
				t.Heights[i] = max(-2000, min(2000, t.Heights[i]))
			}
		}
		derive(&t)
	}
	out := TerrainPreview{Record: r, Impacts: []Impact{}, Revision: state.Revision}
	for i, h := range r.Terrain.Heights {
		if before == nil || len(before.Heights) != len(r.Terrain.Heights) || h != before.Heights[i] || r.Terrain.Water[i] != before.Water[i] {
			out.Changed++
		}
	}
	wet := func(t *Terrain, p Pin) bool {
		if t == nil {
			return false
		}
		i := min(t.Rows-1, int(p.Y*float64(t.Rows)))*t.Columns + min(t.Columns-1, int(p.X*float64(t.Columns)))
		return t.Water[i] != 0
	}
	for _, p := range r.Pins {
		was, now := wet(before, p), wet(r.Terrain, p)
		if was != now {
			change := "newly exposed"
			if now {
				change = "newly submerged"
			}
			out.Impacts = append(out.Impacts, Impact{p.Entity, state.Records[p.Entity].Name, change})
		}
	}
	sort.Slice(out.Impacts, func(i, j int) bool { return out.Impacts[i].Entity < out.Impacts[j].Entity })
	return out, nil
}
