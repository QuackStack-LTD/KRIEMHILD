package httpapi

import (
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/terrain/tiles"
	"kriemhild/internal/world"
	"math"
	"net/http"
	"strconv"
)

func (s *Server) continuousMesh(w http.ResponseWriter, r *http.Request) {
	b := [4]float64{-180, -90, 180, 90}
	for i, k := range []string{"west", "south", "east", "north"} {
		if raw := r.URL.Query().Get(k); raw != "" {
			v, err := strconv.ParseFloat(raw, 64)
			if err != nil || !model.Finite(v) {
				fail(w, 400, "Invalid mesh bounds")
				return
			}
			b[i] = v
		}
		b[i] *= math.Pi / 180
	}
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	if v.planet == nil {
		v.mu.Unlock()
		fail(w, 409, "Legacy terrain has no continuous mesh")
		return
	}
	p := v.planet
	ops := append([]world.Operation(nil), v.authored().Ops(world.Bounds{Width: float64(v.solver.W - 1), Height: float64(v.solver.H - 1)})...)
	transform := continuousTransform(ops, v.solver.W, v.solver.H)
	v.mu.Unlock()
	select {
	case s.detailSlots <- struct{}{}:
		defer func() { <-s.detailSlots }()
	case <-r.Context().Done():
		return
	}
	m, err := tiles.SampleMesh(r.Context(), p, b, 160, 96, transform)
	if err != nil {
		if r.Context().Err() == nil {
			fail(w, 400, err.Error())
		}
		return
	}
	respond(w, 200, m)
}
