package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"kriemhild/internal/terrain"
	"kriemhild/internal/world"
	"math"
	"net/http"
	"strings"
)

func (s *Server) nameGeography(w http.ResponseWriter, r *http.Request) {
	var req struct {
		X        float64 `json:"x"`
		Y        float64 `json:"y"`
		Name     string  `json:"name"`
		Kind     string  `json:"kind"`
		Revision int     `json:"revision"`
	}
	if err := decode(w, r, &req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	e := v.solver.Environment
	if e == nil || v.solver.Status != "done" {
		fail(w, 409, "Finish a physical terrain first")
		return
	}
	if req.Revision != v.authored().Header.Revision {
		fail(w, 409, "Map changed; refresh and retry")
		return
	}
	if req.X < 0 || req.Y < 0 || req.X > float64(v.solver.W-1) || req.Y > float64(v.solver.H-1) || math.IsNaN(req.X+req.Y) || math.IsInf(req.X+req.Y, 0) || strings.TrimSpace(req.Name) == "" || len(req.Name) > 160 {
		fail(w, 400, "Invalid geographical name or location")
		return
	}
	shape, err := geographyShape(v.solver, req.Kind, req.X, req.Y)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	shape.SourceID = shape.ID
	scope := v.worldID
	if v.historical != nil {
		scope = v.historical.TerrainID
	}
	digest := sha256.Sum256([]byte(scope + "/" + shape.ID))
	shape.ID = "geo-" + hex.EncodeToString(digest[:16])
	for _, existing := range v.authored().Entities {
		if !existing.Deleted && existing.SourceID == shape.SourceID && existing.Kind == shape.Kind {
			shape = existing
			break
		}
	}
	shape.Name = strings.TrimSpace(req.Name)
	shape.Color = "#000000"
	shape.Layer = "natural-features"
	shape.MinDetail = 0
	if _, err = v.authored().PutEntity(shape); err != nil {
		fail(w, 400, err.Error())
		return
	}
	v.unsaved = true
	respond(w, 200, map[string]any{"project": v.world.Header, "unsaved": true, "entity": shape})
}
func geographyShape(solver *terrain.Solver, kind string, x, y float64) (world.Entity, error) {
	e := solver.Environment
	w, h := e.Options.Columns, e.Options.Rows
	n := w * h
	cell := int(math.Round(y))*w + int(math.Round(x))
	f := func(key string, i int) float64 {
		if a := e.Fields[key]; len(a) == n {
			return a[i]
		}
		return 0
	}
	result := world.Entity{Kind: kind, Geometry: "polygon"}
	source := cell
	if kind == "river" {
		if e.Hydrology == nil {
			return result, fmt.Errorf("no river network here")
		}
		best := 3.
		chosen := ""
		for _, r := range e.Hydrology.Reaches {
			dx, dy := float64(r.From%w)-x, float64(r.From/w)-y
			distance := math.Hypot(dx, dy)
			if distance < best {
				best = distance
				chosen = r.RiverID
			}
		}
		for _, river := range e.Hydrology.Rivers {
			if river.ID != chosen {
				continue
			}
			result.ID = river.ID
			result.Geometry = "line"
			for at, k := river.Source, 0; at >= 0 && k < n; k++ {
				result.Points = append(result.Points, [2]float64{float64(at % w), float64(at / w)})
				if at == river.Mouth {
					break
				}
				at = int(f("flow", at))
			}
			if len(result.Points) >= 2 {
				return result, nil
			}
		}
		return result, fmt.Errorf("pin a generated river or a point within three map cells of one")
	}
	matches := func(i int) bool {
		biome := ""
		if t := solver.TypeAt(i); t >= 0 {
			tile := solver.Rules.Config.Types[t]
			biome = tile.EnvironmentType
			if biome == "" {
				biome = tile.ID
			}
		}
		switch kind {
		case "island", "continent":
			return f("ocean", i) == 0
		case "mountain":
			return f("waterBody", i) == 0 && (f("mountainCore", i) > .05 || f("elevation", i) > 900)
		case "glacier":
			return f("glacier", i) > .1
		case "lake":
			return f("lake", i) > 0
		case "sea", "ocean":
			return f("ocean", i) > 0
		case "forest":
			return f("waterBody", i) == 0 && (biome == "forest" || biome == "jungle" || biome == "taiga")
		case "grassland":
			return f("waterBody", i) == 0 && (biome == "grass" || biome == "grassland" || biome == "meadow")
		case "desert":
			return f("waterBody", i) == 0 && (biome == "desert" || biome == "dunes")
		}
		return false
	}
	if !matches(source) {
		best := math.Inf(1)
		source = -1
		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				cx, cy := cell%w+dx, cell/w+dy
				if cx < 0 || cy < 0 || cx >= w || cy >= h {
					continue
				}
				i := cy*w + cx
				distance := math.Hypot(float64(cx)-x, float64(cy)-y)
				if matches(i) && distance < best {
					source = i
					best = distance
				}
			}
		}
	}
	if source < 0 {
		return result, fmt.Errorf("no matching %s at the pin; choose its terrain or draw a geographical region", kind)
	}
	cells := []int{source}
	seen := map[int]bool{source: true}
	minimum := source
	for head := 0; head < len(cells); head++ {
		i := cells[head]
		for _, delta := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			cx, cy := i%w+delta[0], i/w+delta[1]
			if cx < 0 || cy < 0 || cx >= w || cy >= h {
				continue
			}
			j := cy*w + cx
			if seen[j] || !matches(j) || (kind == "lake" && f("waterBody", j) != f("waterBody", source)) {
				continue
			}
			seen[j] = true
			cells = append(cells, j)
			minimum = min(minimum, j)
		}
	}
	type point struct{ X, Y int }
	edges := map[point][]point{}
	add := func(a, b point) { edges[a] = append(edges[a], b) }
	for _, i := range cells {
		cx, cy := i%w, i/w
		l, r, t, b := cx*2-1, cx*2+1, cy*2-1, cy*2+1
		if cy == 0 || !seen[i-w] {
			add(point{l, t}, point{r, t})
		}
		if cx == w-1 || !seen[i+1] {
			add(point{r, t}, point{r, b})
		}
		if cy == h-1 || !seen[i+w] {
			add(point{r, b}, point{l, b})
		}
		if cx == 0 || !seen[i-1] {
			add(point{l, b}, point{l, t})
		}
	}
	largest := 0.
	for len(edges) > 0 {
		var start point
		first := true
		for p := range edges {
			if first || p.Y < start.Y || p.Y == start.Y && p.X < start.X {
				start = p
				first = false
			}
		}
		loop := []point{start}
		at := start
		for k := 0; k < n*4+1; k++ {
			next := edges[at]
			if len(next) == 0 {
				break
			}
			to := next[0]
			if len(next) == 1 {
				delete(edges, at)
			} else {
				edges[at] = next[1:]
			}
			loop = append(loop, to)
			at = to
			if at == start {
				break
			}
		}
		area := 0.
		for i := 1; i < len(loop); i++ {
			area += float64(loop[i-1].X*loop[i].Y - loop[i].X*loop[i-1].Y)
		}
		if math.Abs(area) <= largest {
			continue
		}
		largest = math.Abs(area)
		points := [][2]float64{}
		for i, p := range loop[:len(loop)-1] {
			prev, next := loop[(i+len(loop)-2)%(len(loop)-1)], loop[(i+1)%(len(loop)-1)]
			if (p.X-prev.X)*(next.Y-p.Y) == (p.Y-prev.Y)*(next.X-p.X) {
				continue
			}
			points = append(points, [2]float64{math.Max(0, math.Min(float64(w-1), float64(p.X)/2)), math.Max(0, math.Min(float64(h-1), float64(p.Y)/2))})
		}
		result.Points = points
	}
	if len(result.Points) > 2048 {
		return result, fmt.Errorf("this geographical boundary is too complex; draw a smaller named region")
	}
	if len(result.Points) < 3 {
		return result, fmt.Errorf("feature has no usable boundary")
	}
	result.Cells = cells
	result.ID = fmt.Sprintf("%s-%d", kind, minimum)
	return result, nil
}
