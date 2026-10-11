package httpapi

import (
	"fmt"
	"kriemhild/internal/biosphere"
	"kriemhild/internal/history"
	"kriemhild/internal/terrain"
	"kriemhild/internal/world"
	"math/bits"
	"net/http"
	"strings"
)

func (s *Server) biosphereRoutes() {
	s.mux.HandleFunc("GET /api/biosphere/catalogue", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]any{"version": biosphere.Version, "species": biosphere.Catalogue(), "groups": biosphere.Groups, "roles": biosphere.Roles, "interactions": biosphere.InteractionKinds})
	})
	s.mux.HandleFunc("GET /api/worlds/{world}/biosphere/{terrain}", s.getBiosphere)
	s.mux.HandleFunc("POST /api/worlds/{world}/biosphere/{terrain}", s.generateBiosphere)
}
func (s *Server) biosphereTerrain(r *http.Request, d *history.Document, age, id string) (biosphere.State, error) {
	var root *history.Map
	for i := range d.Maps {
		m := &d.Maps[i]
		if m.AgeID == age && m.TerrainID == id && m.ParentID == "" {
			root = m
			break
		}
	}
	if root == nil {
		return biosphere.State{}, fmt.Errorf("save and accept a terrain map in this Age first")
	}
	raw, err := s.projects.SnapshotPart(r.Context(), root.Snapshot, "environment")
	if err != nil {
		return biosphere.State{}, err
	}
	var e *terrain.Environment
	if err = expandJSON(raw, &e); err != nil || e == nil {
		return biosphere.State{}, fmt.Errorf("this terrain has no physical environment; generate terrain with environment layers enabled")
	}
	categories := []string{}
	raw, err = s.projects.SnapshotPart(r.Context(), root.Snapshot, "checkpoint")
	if err == nil {
		var c checkpoint
		if expandJSON(raw, &c) == nil {
			for _, mask := range c.Solver.Dom {
				kind := ""
				j := bits.TrailingZeros32(mask)
				if j < len(c.Config.Types) {
					kind = c.Config.Types[j].EnvironmentType
					if kind == "" {
						kind = c.Config.Types[j].ID
					}
				}
				categories = append(categories, kind)
			}
		}
	}
	// Restore only authored operations, without loading render tiles or spawning a session.
	parts, err := s.projects.SnapshotPartsMatching(r.Context(), root.Snapshot, []string{"builder/operation/", "builder/entity/", "authored-climate"})
	if err != nil {
		return biosphere.State{}, err
	}
	authored := world.New(e.Options.Columns, e.Options.Rows)
	for key, raw := range parts {
		if strings.HasPrefix(key, "builder/operation/") {
			var o world.Operation
			if expandJSON(raw, &o) == nil {
				authored.Operations[o.ID] = o
			}
		}
	}
	for key, raw := range parts {
		if strings.HasPrefix(key, "builder/entity/") {
			var entity world.Entity
			if expandJSON(raw, &entity) == nil {
				authored.Entities[entity.ID] = entity
			}
		}
	}
	authored.Reindex(e.Options.Columns, e.Options.Rows)
	if len(authored.Operations) > 0 || len(authored.Entities) > 0 {
		base := e
		e = authored.Environment(base)
		var overlay authoredClimate
		if raw := parts["authored-climate"]; len(raw) > 0 && expandJSON(raw, &overlay) == nil && overlay.State != nil {
			e.Climate = overlay.State
		} else {
			e.RebuildClimateForSurface(base)
		}
	}
	habitats, source, err := biosphere.Habitats(e, categories, id)
	if err != nil {
		return biosphere.State{}, err
	}
	st := biosphere.State{ID: id, AgeID: age, TerrainID: id, Version: biosphere.Version, Source: source, Width: e.Options.Columns, Height: e.Options.Rows, Habitats: habitats, Occurrences: []biosphere.Occurrence{}, Interactions: []biosphere.Interaction{}, Diagnostics: []string{}, Settings: biosphere.Settings{Mode: "guided", Preset: "earth-like", Familiarity: .5, Richness: "moderate", Coherence: "realistic", Scope: "entire", Seed: e.Options.Seed, Rules: []biosphere.Rule{}}}
	return st, nil
}
func (s *Server) getBiosphere(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	d, err := s.projects.History(r.Context(), r.PathValue("world"))
	if err != nil {
		fail(w, 404, "World not found")
		return
	}
	a := r.URL.Query().Get("age")
	fresh, err := s.biosphereTerrain(r, d, a, r.PathValue("terrain"))
	if err != nil {
		fail(w, 422, err.Error())
		return
	}
	saved := d.Biosphere(a, fresh.TerrainID)
	stale := saved != nil && (saved.Stale || saved.Source != fresh.Source)
	respond(w, 200, map[string]any{"environment": fresh, "saved": saved, "stale": stale})
}
func (s *Server) generateBiosphere(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	var req struct {
		AgeID      string             `json:"ageId"`
		Revision   int                `json:"revision"`
		Source     string             `json:"source"`
		Settings   biosphere.Settings `json:"settings"`
		Accept     bool               `json:"accept"`
		AcceptAll  bool               `json:"acceptAll"`
		ExcludeIDs []string           `json:"excludeIds"`
		AcceptIDs  []string           `json:"acceptIds"`
		LockIDs    []string           `json:"lockIds"`
		Names      map[string]string  `json:"names"`
	}
	if err := decode(w, r, &req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	d, err := s.projects.History(r.Context(), r.PathValue("world"))
	if err != nil {
		fail(w, 404, "World not found")
		return
	}
	if d.World.Revision != req.Revision {
		fail(w, 409, "World changed; reload before generating")
		return
	}
	base, err := s.biosphereTerrain(r, d, req.AgeID, r.PathValue("terrain"))
	if err != nil {
		fail(w, 422, err.Error())
		return
	}
	if req.Accept && base.Source != req.Source {
		fail(w, 409, "Terrain changed since preview; generate a new preview")
		return
	}
	if old := d.Biosphere(req.AgeID, base.TerrainID); old != nil {
		base.Occurrences = old.Occurrences
	}
	p, err := biosphere.Generate(base, req.Settings, d.Lifeforms(req.AgeID))
	if err != nil {
		fail(w, 422, err.Error())
		return
	}
	if !req.Accept {
		respond(w, 200, p)
		return
	}
	if len(req.AcceptIDs) > 500000 || len(req.ExcludeIDs) > 500000 || len(req.LockIDs) > 500000 {
		fail(w, 400, "too many selected entries")
		return
	}
	valid := map[string]bool{}
	for _, o := range p.State.Occurrences {
		valid[o.ID] = true
	}
	if req.AcceptAll {
		exclude := map[string]bool{}
		for _, id := range req.ExcludeIDs {
			if !valid[id] {
				fail(w, 400, "excluded occurrence is not in this preview")
				return
			}
			exclude[id] = true
		}
		req.AcceptIDs = nil
		for _, o := range p.State.Occurrences {
			if !exclude[o.ID] {
				req.AcceptIDs = append(req.AcceptIDs, o.ID)
			}
		}
	}
	for _, id := range req.AcceptIDs {
		if !valid[id] {
			fail(w, 400, "selected occurrence is not in this preview")
			return
		}
	}
	result, err := s.projects.UpdateHistory(r.Context(), d.World.ID, req.Revision, func(next *history.Document) error {
		return next.AcceptBiosphere(p, req.AcceptIDs, req.LockIDs, req.Names)
	})
	if err != nil {
		fail(w, 409, err.Error())
		return
	}
	respond(w, 200, result)
}

// Overlay is read-only, using habitat geometry in the existing Atlas preview.
func habitatCells(d *history.Document, age, terrain, id string) map[int]bool {
	cells := map[int]bool{}
	if state := d.Biosphere(age, terrain); state != nil {
		for _, h := range state.Habitats {
			if h.ID == id {
				for _, cell := range h.Cells {
					cells[cell] = true
				}
			}
		}
	}
	return cells
}
