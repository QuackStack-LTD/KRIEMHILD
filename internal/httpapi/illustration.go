package httpapi

import (
	"encoding/json"
	"fmt"
	"kriemhild/internal/history"
	"kriemhild/internal/terrain"
	"kriemhild/internal/world"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

func artworkBounds(v *session) world.Bounds {
	if v.historical != nil {
		return v.historical.Bounds
	}
	return world.Bounds{X: 0, Y: 0, Width: float64(v.solver.W - 1), Height: float64(v.solver.H - 1)}
}

// Explicit generation flushes a complete source snapshot before rendering.
// No viewport query, visibility switch, or object-count cap determines content.
func (s *Server) illustrationData(w http.ResponseWriter, r *http.Request) {
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	if v.solver.Status != "done" {
		fail(w, 409, "Finish terrain generation first")
		return
	}
	model := v.authored()
	// Illustration includes source content regardless of editor layer visibility.
	exportState := *model
	exportState.Header = model.Header
	exportState.Header.Layers = append([]world.Layer(nil), model.Header.Layers...)
	for i := range exportState.Header.Layers {
		exportState.Header.Layers[i].Visible = true
	}
	b := artworkBounds(v)
	entities := []world.Entity{}
	for _, e := range model.Entities {
		if !e.Deleted {
			entities = append(entities, e)
		}
	}
	sort.Slice(entities, func(i, j int) bool { return entities[i].ID < entities[j].ID })
	features := []terrain.DetailFeature{}
	env := v.solver.Environment
	cols := min(385, max(65, int(math.Ceil(b.Width*2))+1))
	rows := max(33, int(math.Round(float64(cols-1)*b.Height/b.Width))+1)
	if rows > 385 {
		cols = max(33, int(math.Round(384*b.Width/b.Height))+1)
		rows = 385
	}
	// Samples are compact: water occupancy, elevation, forest cover, mountain relief.
	samples := make([][4]float64, cols*rows)
	level := max(0, min(terrain.DetailLevels, int(math.Floor(math.Log2(float64(cols-1)/b.Width)))))
	tiles := map[string]*terrain.DetailTile{}
	if env != nil && v.detail == nil {
		v.detail = terrain.NewDetailModel(env)
	}
	if v.detail != nil {
		features = append(features, v.detail.Features...)
	}
	for _, op := range exportState.Ops(b) {
		if op.River != nil {
			features = append(features, *op.River)
		}
	}
	sample := func(x, y float64) (terrain.DetailPoint, error) {
		scale := math.Exp2(float64(level))
		tx, ty := int(x*scale/32), int(y*scale/32)
		if float64(tx)*32/scale >= float64(v.solver.W-1) {
			tx = max(0, tx-1)
		}
		if float64(ty)*32/scale >= float64(v.solver.H-1) {
			ty = max(0, ty-1)
		}
		key := detailKey(level, tx, ty)
		tile := tiles[key]
		if tile == nil {
			raw, err := v.storedDetail(level, tx, ty)
			if err != nil {
				return terrain.DetailPoint{}, err
			}
			tile = &terrain.DetailTile{}
			if err = json.Unmarshal(raw, tile); err != nil {
				return terrain.DetailPoint{}, err
			}
			exportState.ApplyTile(tile, nil)
			tiles[key] = tile
		}
		u, z := math.Min(32, math.Max(0, x*scale-float64(tx*32))), math.Min(32, math.Max(0, y*scale-float64(ty*32)))
		ix, iy := min(31, int(u)), min(31, int(z))
		a, c := u-float64(ix), z-float64(iy)
		p := terrain.DetailPoint{}
		for k, xy := range [][2]int{{ix, iy}, {ix + 1, iy}, {ix, iy + 1}, {ix + 1, iy + 1}} {
			q := tile.Points[xy[1]*33+xy[0]]
			weight := []float64{(1 - a) * (1 - c), a * (1 - c), (1 - a) * c, a * c}[k]
			p.Elevation += q.Elevation * weight
			p.Vegetation += q.Vegetation * weight
			if q.Authored && weight > .01 {
				p.Authored = true
			}
			p.Rock += q.Rock * weight
			if !q.Authored {
				if q.Shore == 0 && q.WaterBody > 0 {
					p.Shore += weight
				} else {
					p.Shore += q.Shore * weight
				}
			} else if q.WaterBody > 0 && q.WaterDepth >= 0 {
				p.Shore += weight
			}
		}
		return p, nil
	}
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			wx, wy := b.X+float64(x)*b.Width/float64(cols-1), b.Y+float64(y)*b.Height/float64(rows-1)
			cell := min(v.solver.H-1, int(math.Round(wy)))*v.solver.W + min(v.solver.W-1, int(math.Round(wx)))
			typ := ""
			if t := v.solver.TypeAt(cell); t >= 0 {
				typ = v.solver.Rules.Config.Types[t].ID
			}
			forest := strings.Contains(typ, "forest") || typ == "taiga" || typ == "jungle"
			mountain := strings.Contains(typ, "mountain") || typ == "volcano" || typ == "glacier"
			q := [4]float64{}
			if env != nil {
				p, err := sample(wx, wy)
				if err != nil {
					fail(w, 500, err.Error())
					return
				}
				q[0] = p.Shore
				q[1] = p.Elevation
				if forest || (p.Authored && p.Vegetation > .65) {
					q[2] = p.Vegetation
				}
				if mountain || p.Elevation > 1300 {
					q[3] = math.Min(1, math.Max(.25, (p.Elevation-700)/3500))
				}
			}
			if env == nil {
				if strings.Contains(typ, "water") || typ == "ocean" || typ == "sea" || typ == "lake" {
					q[0] = 1
				}
				if forest {
					q[2] = .8
				}
				if mountain {
					q[1] = 2000
					q[3] = .7
				}
			}
			samples[y*cols+x] = q
		}
	}
	if err := s.saveWorld(v, false); err != nil {
		fail(w, 503, "Could not save the illustration's source map: "+err.Error())
		return
	}
	out := map[string]any{"name": model.Header.Name, "revision": model.Header.Revision, "mapId": model.Header.MapID, "worldId": v.worldID, "historicalMap": v.historical, "bounds": b, "columns": cols, "rows": rows, "samples": samples, "entities": entities, "features": features, "saved": true, "unsaved": false, "terrainWidth": v.solver.W, "terrainHeight": v.solver.H}
	if v.historical != nil && s.projects != nil {
		d, err := s.projects.History(r.Context(), v.historyWorld)
		if err != nil {
			fail(w, 503, "Could not load the illustration's geographical relationships: "+err.Error())
			return
		}
		shapes, logical, relations := illustrationHierarchy(d, *v.historical, entities)
		out["entities"], out["logicalEntities"], out["relations"] = shapes, logical, relations
	}
	if env != nil {
		out["geography"] = env.Geography
		out["hydrology"] = env.Hydrology
	}
	respond(w, 200, out)
}

// Include settlement footprints from detailed maps, without exporting their
// internal buildings onto a country map. Coordinates and identities are shared.
func illustrationHierarchy(d *history.Document, m history.Map, shapes []world.Entity) ([]world.Entity, []history.Entity, []history.Relation) {
	logical := []history.Entity{}
	relations := []history.Relation{}
	active := map[string]history.Entity{}
	for _, e := range d.Entities {
		if e.AgeID == m.AgeID && !e.Deleted {
			active[e.ID] = e
			logical = append(logical, e)
		}
	}
	for _, r := range d.Relations {
		if r.AgeID == m.AgeID {
			relations = append(relations, r)
		}
	}
	seen := map[string]bool{}
	for i := range shapes {
		e := &shapes[i]
		id := e.LogicalID
		if id == "" {
			id = e.ID
		}
		if entity, ok := active[id]; ok {
			e.Name = entity.Name
		}
		seen[e.ID] = true
	}
	for _, r := range d.Representations {
		e, ok := active[r.EntityID]
		if !ok || r.AgeID != m.AgeID || e.TerrainID != m.TerrainID || seen[r.Shape.ID] || r.Shape.Deleted || !m.Bounds.Intersects(world.Box(r.Shape.Points)) {
			continue
		}
		switch e.Kind {
		case "city", "town", "village", "capital", "country", "state", "kingdom", "empire":
		default:
			continue
		}
		shape := r.Shape
		shape.Name, shape.LogicalID = e.Name, e.ID
		shapes = append(shapes, shape)
		seen[shape.ID] = true
	}
	sort.Slice(shapes, func(i, j int) bool { return shapes[i].ID < shapes[j].ID })
	return shapes, logical, relations
}

func (s *Server) listArtworks(w http.ResponseWriter, r *http.Request) {
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	list := []world.Artwork{}
	for _, a := range v.authored().Artworks {
		list = append(list, a)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt == list[j].CreatedAt {
			return list[i].ID < list[j].ID
		}
		return list[i].CreatedAt > list[j].CreatedAt
	})
	respond(w, 200, map[string]any{"artworks": list})
}

func (s *Server) saveArtwork(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 14<<20)
	var a world.Artwork
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		fail(w, 400, "Invalid artwork request")
		return
	}
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	model := v.authored()
	if a.SourceRevision != model.Header.Revision {
		fail(w, 409, "The map changed since this illustration was generated. Regenerate the preview.")
		return
	}
	a.ID = token()
	a.MapID = v.worldID
	a.WorldID = v.historyWorld
	a.Bounds = artworkBounds(v)
	a.CreatedAt = time.Now().UnixMilli()
	if v.historical != nil {
		a.MapID = v.historical.ID
		a.AgeID = v.historical.AgeID
		a.ParentMapID = v.historical.ParentID
	}
	if err := model.AddArtwork(a); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if err := s.saveWorld(v, false); err != nil {
		model.DiscardArtwork(a.ID)
		fail(w, 503, fmt.Sprintf("Could not save artwork: %v", err))
		return
	}
	respond(w, 201, map[string]any{"artwork": a, "saved": true, "unsaved": false})
}
