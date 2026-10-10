package history

import (
	"encoding/json"
	"fmt"
	"kriemhild/internal/world"
	"math"
	"sort"
)

func (d *Document) AddTerrain(age, name, project string, w, h int) (Map, error) {
	if d.Age(age) == nil {
		return Map{}, fmt.Errorf("Age not found")
	}
	t := Terrain{ID: ID(), AgeID: age, Name: name, Setting: "planet", Width: w, Height: h}
	d.Terrains = append(d.Terrains, t)
	m := Map{ID: ID(), AgeID: age, TerrainID: t.ID, ProjectID: project, Name: name, Kind: "terrain", Bounds: world.Bounds{Width: float64(w - 1), Height: float64(h - 1)}}
	d.Maps = append(d.Maps, m)
	return m, nil
}
func (d *Document) Child(age, parentID, entityID, name string, b world.Bounds) (Map, error) {
	parent := d.Map(age, parentID)
	if parent == nil {
		return Map{}, fmt.Errorf("parent map not found")
	}
	p := *parent
	if entityID != "" {
		found := false
		for _, r := range d.Representations {
			if r.AgeID == age && r.MapID == parentID && r.EntityID == entityID {
				b = world.Box(r.Shape.Points).Expand(.5)
				found = true
				break
			}
		}
		if !found {
			return Map{}, fmt.Errorf("settlement not found on parent map")
		}
		for _, m := range d.Maps {
			if m.AgeID == age && m.ParentID == parentID && m.EntityID == entityID {
				return m, nil
			}
		}
	}
	right, bottom := math.Min(p.Bounds.X+p.Bounds.Width, b.X+b.Width), math.Min(p.Bounds.Y+p.Bounds.Height, b.Y+b.Height)
	b.X = math.Max(p.Bounds.X, b.X)
	b.Y = math.Max(p.Bounds.Y, b.Y)
	b.Width = right - b.X
	b.Height = bottom - b.Y
	if b.Width <= 0 || b.Height <= 0 {
		return Map{}, fmt.Errorf("select an area inside the parent map")
	}
	m := Map{ID: ID(), AgeID: age, TerrainID: p.TerrainID, ProjectID: ID(), Snapshot: p.Snapshot, Name: name, ParentID: p.ID, EntityID: entityID, Kind: "settlement", Bounds: b}
	d.Maps = append(d.Maps, m)
	for _, r := range append([]Representation(nil), d.Representations...) {
		if r.AgeID == age && r.MapID == parentID && b.Intersects(world.Box(r.Shape.Points)) {
			r.MapID = m.ID
			r.ID = m.ID + "/" + r.EntityID
			d.Representations = append(d.Representations, r)
		}
	}
	return m, nil
}

// Sync logical identity separately from a map's drawables. Inherited footprints
// and geological features share identity; internal buildings remain map-local.
func (d *Document) Sync(m Map, state *world.State) {
	for id, shape := range state.Entities {
		ei := -1
		for i, e := range d.Entities {
			if e.AgeID == m.AgeID && e.ID == id {
				ei = i
				break
			}
		}
		if shape.Deleted {
			if ei >= 0 {
				d.Entities[ei].Deleted = true
			}
			out := d.Representations[:0]
			for _, r := range d.Representations {
				if r.AgeID != m.AgeID || r.EntityID != id {
					out = append(out, r)
				}
			}
			d.Representations = out
			continue
		}
		natural := shape.Layer == "natural-features"
		props, _ := json.Marshal(map[string]any{"notes": shape.Notes, "created": shape.Created, "destroyed": shape.Destroyed, "category": shape.Layer})
		e := Entity{ID: id, AgeID: m.AgeID, TerrainID: m.TerrainID, HomeMap: m.ID, Name: shape.Name, Kind: shape.Kind, Natural: natural, SourceID: shape.SourceID, Properties: props}
		if natural {
			for p := d.Map(m.AgeID, e.HomeMap); p != nil && p.ParentID != ""; p = d.Map(m.AgeID, e.HomeMap) {
				e.HomeMap = p.ParentID
			}
		}
		if ei >= 0 {
			e.HomeMap = d.Entities[ei].HomeMap
			e.SourceID = d.Entities[ei].SourceID
			d.Entities[ei] = e
		} else {
			d.Entities = append(d.Entities, e)
		}
		found := false
		for i := range d.Representations {
			r := &d.Representations[i]
			if r.AgeID == m.AgeID && r.EntityID == id {
				r.Shape = shape
				if r.MapID == m.ID {
					found = true
				}
			}
		}
		if !found {
			d.Representations = append(d.Representations, Representation{ID: m.ID + "/" + id, AgeID: m.AgeID, MapID: m.ID, EntityID: id, Shape: shape})
		}
		if natural {
			for _, other := range d.Maps {
				if other.AgeID != m.AgeID || other.TerrainID != m.TerrainID || other.ID == m.ID {
					continue
				}
				if !other.Bounds.Intersects(world.Box(shape.Points)) {
					continue
				}
				exists := false
				for _, r := range d.Representations {
					if r.AgeID == m.AgeID && r.MapID == other.ID && r.EntityID == id {
						exists = true
						break
					}
				}
				if !exists {
					d.Representations = append(d.Representations, Representation{ID: other.ID + "/" + id, AgeID: m.AgeID, MapID: other.ID, EntityID: id, Shape: shape})
				}
			}
		}
	}
	d.Resolve(m.AgeID)
}
func contains(p [2]float64, polygon [][2]float64) bool {
	inside := false
	for i, j := 0, len(polygon)-1; i < len(polygon); j, i = i, i+1 {
		a, b := polygon[i], polygon[j]
		cross := (b[0]-a[0])*(p[1]-a[1]) - (b[1]-a[1])*(p[0]-a[0])
		if math.Abs(cross) < 1e-8 && p[0] >= math.Min(a[0], b[0])-1e-8 && p[0] <= math.Max(a[0], b[0])+1e-8 && p[1] >= math.Min(a[1], b[1])-1e-8 && p[1] <= math.Max(a[1], b[1])+1e-8 {
			return true
		}
		if (a[1] > p[1]) != (b[1] > p[1]) && p[0] < (b[0]-a[0])*(p[1]-a[1])/(b[1]-a[1])+a[0] {
			inside = !inside
		}
	}
	return inside
}

// Recompute from geometry, not naming order. Test every segment as well as
// vertices so concave regions do not falsely contain crossing roads/rivers.
func enclosed(shape, area world.Entity, inside func([2]float64) bool) bool {
	if area.Geometry != "polygon" || len(area.Points) < 3 {
		return false
	}
	for _, p := range shape.Points {
		if !inside(p) {
			return false
		}
	}
	path := append([][2]float64(nil), shape.Points...)
	if shape.Geometry == "polygon" {
		path = append(path, path[0])
	}
	for i := 1; i < len(path); i++ {
		a, b := path[i-1], path[i]
		dx, dy := b[0]-a[0], b[1]-a[1]
		cuts := []float64{0, 1}
		for j, c := range area.Points {
			e := area.Points[(j+1)%len(area.Points)]
			ex, ey := e[0]-c[0], e[1]-c[1]
			den := dx*ey - dy*ex
			if math.Abs(den) < 1e-12 {
				continue
			}
			t := ((c[0]-a[0])*ey - (c[1]-a[1])*ex) / den
			u := ((c[0]-a[0])*dy - (c[1]-a[1])*dx) / den
			if t > 0 && t < 1 && u >= 0 && u <= 1 {
				cuts = append(cuts, t)
			}
		}
		// Cell sets retain interior holes (islands inside an ocean, for example).
		if len(area.Cells) > 0 {
			steps := int(math.Ceil(math.Max(math.Abs(dx), math.Abs(dy)) * 2))
			for k := 1; k < steps; k++ {
				cuts = append(cuts, float64(k)/float64(steps))
			}
		}
		sort.Float64s(cuts)
		for k := 1; k < len(cuts); k++ {
			t := (cuts[k-1] + cuts[k]) / 2
			if !inside([2]float64{a[0] + dx*t, a[1] + dy*t}) {
				return false
			}
		}
	}
	return true
}

func (d *Document) Resolve(age string) {
	valid := map[string]bool{}
	entities := map[string]Entity{}
	for _, e := range d.Entities {
		if e.AgeID == age && !e.Deleted {
			valid[e.ID] = true
			entities[e.ID] = e
		}
	}
	out := d.Relations[:0]
	for _, r := range d.Relations {
		if r.AgeID != age || (!r.Automatic && valid[r.Source] && valid[r.Destination]) {
			out = append(out, r)
		}
	}
	d.Relations = out
	shapes := map[string]world.Entity{}
	for _, r := range d.Representations {
		if r.AgeID == age {
			shapes[r.EntityID] = r.Shape
		}
	}
	add := func(from, to, kind string) {
		id := from + "/" + kind + "/" + to
		for _, r := range d.Relations {
			if r.AgeID == age && r.ID == id {
				return
			}
		}
		d.Relations = append(d.Relations, Relation{id, age, from, to, kind, true})
	}
	widths := map[string]int{}
	for _, t := range d.Terrains {
		if t.AgeID == age {
			widths[t.ID] = t.Width
		}
	}
	regions := map[string]map[int]bool{}
	for id, s := range shapes {
		if len(s.Cells) > 0 {
			regions[id] = map[int]bool{}
			for _, cell := range s.Cells {
				regions[id][cell] = true
			}
		}
	}
	for id, a := range shapes {
		if a.ParentID != "" && valid[a.ParentID] {
			add(id, a.ParentID, "part-of")
		}
		for other, b := range shapes {
			if id == other || entities[id].TerrainID != entities[other].TerrainID {
				continue
			}
			if b.Geometry == "polygon" {
				inside := func(p [2]float64) bool {
					if cells := regions[other]; cells != nil {
						w := widths[entities[other].TerrainID]
						return cells[int(math.Round(p[1]))*w+int(math.Round(p[0]))]
					}
					return contains(p, b.Points)
				}
				ab, bb := world.Box(a.Points), world.Box(b.Points)
				if a.Geometry != "polygon" || ab.Width*ab.Height < bb.Width*bb.Height-1e-8 {
					if enclosed(a, b, inside) {
						add(id, other, "contained-by")
					}
				}
				if a.Kind == "river" && len(a.Points) > 1 {
					if inside(a.Points[0]) && (b.Kind == "mountain" || b.Kind == "glacier" || b.Kind == "lake") {
						add(id, other, "originates-in")
					}
					if inside(a.Points[len(a.Points)-1]) && (b.Kind == "lake" || b.Kind == "sea" || b.Kind == "ocean") {
						add(id, other, "flows-into")
					}
				}
			}
		}
	}
}
func (d *Document) Shapes(m Map) map[string]world.Entity {
	out := map[string]world.Entity{}
	for _, r := range d.Representations {
		if r.AgeID == m.AgeID && r.MapID == m.ID {
			out[r.EntityID] = r.Shape
		}
	}
	for id, s := range out {
		if _, ok := out[s.ParentID]; !ok {
			s.ParentID = ""
			out[id] = s
		}
	}
	return out
}

// Tombstones distinguish a deletion from content that never existed on a branch.
func (d *Document) PruneDeleted(age string) {
	deleted := map[string]bool{}
	for _, e := range d.Entities {
		if e.AgeID == age && e.Deleted {
			deleted[e.ID] = true
		}
	}
	reps := d.Representations[:0]
	for _, r := range d.Representations {
		if r.AgeID != age || !deleted[r.EntityID] {
			reps = append(reps, r)
		}
	}
	d.Representations = reps
}
