package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"kriemhild/internal/history"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/world"
	"math"
	"net/http"
	"strings"
)

type waterBody struct {
	SourceID string     `json:"sourceId"`
	Kind     string     `json:"kind"`
	Name     string     `json:"name"`
	EntityID string     `json:"entityId,omitempty"`
	Position [2]float64 `json:"position"` // normalized map coordinates
}

func waterPosition(p model.Point) [2]float64 {
	return [2]float64{(model.Wrap(p[0]) + math.Pi) / math.Pi, .5 - p[1]/math.Pi}
}

// Identities are derived from immutable saved source geometry, never names,
// drawing order, zoom, or random tokens. Seas/bays can be authored separately
// because circulation does not define a unique boundary within one ocean.
func generatedWaterBodies(p *model.Planet) []waterBody {
	out := make([]waterBody, 0, len(p.Rivers)+len(p.Lakes)+4)
	for _, r := range p.Rivers {
		if len(r.Path) > 0 {
			out = append(out, waterBody{SourceID: r.ID, Kind: "river", Name: "River " + r.ID, Position: waterPosition(r.Path[len(r.Path)/2])})
		}
	}
	for i, l := range p.Lakes {
		out = append(out, waterBody{SourceID: fmt.Sprintf("lake-%d", i), Kind: "lake", Name: fmt.Sprintf("Lake %d", i+1), Position: waterPosition(l.Center)})
	}
	w, h := p.Grid.Width, p.Grid.Height
	seen := make([]bool, w*h)
	oceans := 0
	for start := range seen {
		if seen[start] || p.Emerged(start) {
			continue
		}
		queue := []int{start}
		seen[start] = true
		anchor := start
		for at := 0; at < len(queue); at++ {
			i := queue[at]
			if p.Grid.Fields["coast"][i] < p.Grid.Fields["coast"][anchor] {
				anchor = i
			}
			x, y := i%w, i/w
			for _, j := range []int{y*w + (x+w-1)%w, y*w + (x+1)%w, i - w, i + w} {
				if j >= 0 && j < len(seen) && !seen[j] && !p.Emerged(j) {
					seen[j] = true
					queue = append(queue, j)
				}
			}
		}
		oceans++
		out = append(out, waterBody{SourceID: fmt.Sprintf("ocean-%d", start), Kind: "ocean", Name: fmt.Sprintf("Ocean %d", oceans), Position: waterPosition(p.Grid.Point(anchor%w, anchor/w))})
	}
	return out
}

func (v *session) waterCatalog() []waterBody {
	if v.waterBodies == nil {
		v.waterBodies = generatedWaterBodies(v.planet)
	}
	return append([]waterBody(nil), v.waterBodies...)
}

func waterNames(d *history.Document, m history.Map) map[string]history.Entity {
	names := map[string]history.Entity{}
	for _, e := range d.Entities {
		if !e.Deleted && e.AgeID == m.AgeID && e.TerrainID == m.TerrainID && e.SourceID != "" {
			names[e.SourceID] = e
		}
	}
	return names
}

func (s *Server) listWaterBodies(w http.ResponseWriter, r *http.Request) {
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	if v.planet == nil {
		fail(w, 409, "Open continuous terrain to name generated water bodies")
		return
	}
	bodies := v.waterCatalog()
	revision := 0
	if v.historical != nil {
		latest, err := s.projects.CurrentRevision(r.Context(), v.historyWorld)
		if err == nil && latest != v.historyRevision {
			err = s.applyHistoricalMap(r.Context(), v)
		}
		if err != nil {
			fail(w, 409, err.Error())
			return
		}
		revision = v.historyRevision
		names := map[string]history.Entity{}
		for _, e := range v.authored().Entities {
			if !e.Deleted && e.SourceID != "" {
				names[e.SourceID] = history.Entity{ID: e.LogicalID, Name: e.Name}
				if e.LogicalID == "" {
					entry := names[e.SourceID]
					entry.ID = e.ID
					names[e.SourceID] = entry
				}
			}
		}

		for i := range bodies {
			if e, ok := names[bodies[i].SourceID]; ok {
				bodies[i].Name, bodies[i].EntityID = e.Name, e.ID
			}
		}
	}
	respond(w, 200, map[string]any{"bodies": bodies, "revision": revision})
}

func (s *Server) nameWaterBody(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Revision int    `json:"revision"`
	}
	if err := decode(w, r, &req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 160 {
		fail(w, 400, "Provide a water body name of at most 160 characters")
		return
	}
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	if v.planet == nil || v.historical == nil || s.projects == nil {
		fail(w, 409, "Save terrain to its World before naming water bodies")
		return
	}
	var body *waterBody
	for _, b := range v.waterCatalog() {
		if b.SourceID == r.PathValue("source") {
			copy := b
			body = &copy
			break
		}
	}
	if body == nil {
		fail(w, 404, "Water body not found in this terrain")
		return
	}
	m := *v.historical
	for _, e := range v.authored().Entities {
		if e.Deleted || e.RecordStatus == "draft" || e.SourceID != body.SourceID {
			continue
		}
		id := e.LogicalID
		if id == "" {
			id = e.ID
		}
		change, err := s.projects.RenameHistory(r.Context(), v.historyWorld, history.Command{Kind: "rename-entity", AgeID: m.AgeID, EntityID: id, Name: req.Name, Revision: req.Revision})
		if err != nil {
			fail(w, 409, err.Error())
			return
		}
		applySessionNameChange(v, change)
		d, err := s.readHistoryView(r.Context(), v.historyWorld, r.URL.Query().Get("view"))
		if err != nil {
			fail(w, 503, err.Error())
			return
		}
		respondHistory(w, r, 200, map[string]any{"document": d, "project": v.authored().Header})
		return
	}
	d, err := s.projects.UpdateMapRecords(r.Context(), v.worldID, req.Revision, func(d *history.Document, _ *history.Map) error {
		if e, ok := waterNames(d, m)[body.SourceID]; ok {
			return d.RenameEntity(m.AgeID, e.ID, req.Name)
		}
		hash := sha256.Sum256([]byte(m.TerrainID + "/water/" + body.SourceID))
		id := fmt.Sprintf("%x", hash[:16])
		root := m
		for root.ParentID != "" {
			root = *d.Map(m.AgeID, root.ParentID)
		}
		points := [][2]float64{{body.Position[0] / 2 * float64(v.solver.W-1), body.Position[1] * float64(v.solver.H-1)}}
		geometry := "point"
		if body.Kind == "river" {
			for _, river := range v.planet.Rivers {
				if river.ID != body.SourceID || len(river.Path) < 2 {
					continue
				}
				geometry, points = "line", nil
				step := max(1, int(math.Ceil(float64(len(river.Path)-1)/2047)))
				for i := 0; i < len(river.Path)-1; i += step {
					q := waterPosition(river.Path[i])
					points = append(points, [2]float64{q[0] / 2 * float64(v.solver.W-1), q[1] * float64(v.solver.H-1)})
				}
				last := waterPosition(river.Path[len(river.Path)-1])
				points = append(points, [2]float64{last[0] / 2 * float64(v.solver.W-1), last[1] * float64(v.solver.H-1)})
			}
		}
		shape := world.Entity{ID: id, LogicalID: id, Name: req.Name, Kind: body.Kind, SourceID: body.SourceID, Layer: "natural-features", Geometry: geometry, Points: points, Color: "#5aa4bc", RecordStatus: "published"}
		props, _ := json.Marshal(map[string]any{"generated": true})
		d.Entities = append(d.Entities, history.Entity{ID: id, ContinuityID: id, AgeID: m.AgeID, TerrainID: m.TerrainID, HomeMap: root.ID, Kind: body.Kind, Name: req.Name, Natural: true, SourceID: body.SourceID, Properties: props})
		for _, view := range d.Maps {
			if view.AgeID == m.AgeID && view.TerrainID == m.TerrainID && (view.ID == root.ID || view.Bounds.Intersects(world.Box(points))) {
				d.Representations = append(d.Representations, history.Representation{ID: view.ID + "/" + id, AgeID: m.AgeID, MapID: view.ID, EntityID: id, Shape: shape})
			}
		}
		return nil
	})
	if err != nil {
		fail(w, 409, err.Error())
		return
	}
	if err := s.applyHistoricalMap(r.Context(), v); err != nil {
		fail(w, 409, err.Error())
		return
	}
	v.authored().Header.Revision++
	if d, err = s.readHistoryView(r.Context(), v.historyWorld, r.URL.Query().Get("view")); err != nil {
		fail(w, 503, err.Error())
		return
	}
	respondHistory(w, r, 200, map[string]any{"document": historyResponseView(d, r), "project": v.authored().Header})
}
