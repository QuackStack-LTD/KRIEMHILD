package httpapi

import (
	"kriemhild/internal/settlements"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/world"
	"math"
	"net/http"
)

type settlementDraft struct {
	ID       string              `json:"id"`
	Revision int                 `json:"revision"`
	Preview  settlements.Preview `json:"preview"`
	Entities []world.Entity      `json:"entities"`
}

func (s *Server) previewSettlement(w http.ResponseWriter, r *http.Request) {
	var spec settlements.Spec
	if err := decode(w, r, &spec); err != nil {
		fail(w, 400, err.Error())
		return
	}
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	if v.planet == nil {
		fail(w, 409, "Open a continuous terrain first")
		return
	}
	preview, err := settlements.Generate(v.planet, spec)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	draft := &settlementDraft{ID: token(), Revision: v.authored().Header.Revision, Preview: preview, Entities: []world.Entity{}}
	ids := map[string]string{}
	for _, f := range preview.Features {
		ids[f.ID] = token()
	}
	for _, f := range preview.Features {
		layer := "buildings"
		if f.Kind == spec.Kind || f.Kind == "district" {
			layer = "settlements"
		}
		if f.Kind == "road" {
			layer = "roads"
		}
		e := world.Entity{ID: ids[f.ID], ParentID: ids[f.Parent], Kind: f.Kind, Name: f.Name, Geometry: f.Geometry, Layer: layer, MinDetail: f.MinDetail, Color: "#c8a976"}
		e.RecordStatus = "draft"
		e.Name = world.RecordName(e)
		if f.ID != "settlement" || !world.MeaningfulName(e) {
			e.PlaceholderName = e.Name
		}
		for _, pt := range f.Points {
			e.Points = append(e.Points, [2]float64{(model.Wrap(pt[0]) + math.Pi) / model.Tau * float64(v.solver.W-1), (.5 - pt[1]/math.Pi) * float64(v.solver.H-1)})
		}
		draft.Entities = append(draft.Entities, e)
	}
	v.settlement = draft
	respond(w, 200, draft)
}
func (s *Server) acceptSettlement(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string `json:"id"`
		Revision int    `json:"revision"`
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
	draft := v.settlement
	if draft == nil || draft.ID != req.ID || draft.Revision != req.Revision || v.authored().Header.Revision != req.Revision {
		fail(w, 409, "Settlement preview is stale; preview the site again.")
		return
	}
	next, err := v.authored().Clone()
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	for i := range draft.Entities {
		draft.Entities[i].RecordStatus = "draft"
	}
	if err := next.PutEntities(draft.Entities); err != nil {
		fail(w, 400, err.Error())
		return
	}
	changes := world.New(v.solver.W, v.solver.H)
	for _, shape := range draft.Entities {
		changes.Entities[shape.ID] = shape
	}
	if err := s.publishBuilderEntities(v, changes); err != nil {
		fail(w, 503, "Could not persist settlement: "+err.Error())
		return
	}
	v.world = next
	v.settlement = nil
	v.unsaved = true
	respond(w, 201, map[string]any{"project": v.world.Header, "entities": draft.Entities, "unsaved": true})
}
