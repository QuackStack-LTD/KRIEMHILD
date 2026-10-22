package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/terrain/tiles"
	"kriemhild/internal/world"
	"math"
	"net/http"
	"strconv"
)

func (s *Server) continuousMesh(w http.ResponseWriter, r *http.Request) {
	b := [4]float64{-180, -90, 180, 90}
	columns, rows := 160, 96
	if r.URL.Query().Has("z") {
		z, e1 := strconv.Atoi(r.URL.Query().Get("z"))
		x, e2 := strconv.Atoi(r.URL.Query().Get("x"))
		y, e3 := strconv.Atoi(r.URL.Query().Get("y"))
		if e1 != nil || e2 != nil || e3 != nil || !tiles.Valid(z, x, y, 256) {
			fail(w, 400, "Invalid mesh tile")
			return
		}
		span := 180 / math.Exp2(float64(z))
		b = [4]float64{-180 + float64(x)*span, 90 - float64(y+1)*span, -180 + float64(x+1)*span, 90 - float64(y)*span}
		columns, rows = 65, 65
	}
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
	box := world.Bounds{X: (b[0] + math.Pi) / model.Tau * float64(v.solver.W-1), Y: (.5 - b[3]/math.Pi) * float64(v.solver.H-1), Width: (b[2] - b[0]) / model.Tau * float64(v.solver.W-1), Height: (b[3] - b[1]) / math.Pi * float64(v.solver.H-1)}
	ops := append([]world.Operation(nil), v.authored().Ops(box.Expand(.1))...)
	transform := continuousTransform(ops, v.solver.W, v.solver.H, p)
	if v.tileIdentity == "" {
		meta, fields, err := p.Encode()
		if err != nil {
			v.mu.Unlock()
			fail(w, 500, err.Error())
			return
		}
		hash := sha256.New()
		hash.Write(meta)
		hash.Write(fields)
		v.tileIdentity = fmt.Sprintf("%x", hash.Sum(nil))
	}
	opData, _ := json.Marshal(ops)
	key := fmt.Sprintf("mesh/%s/%v/%d/%d/%x", v.tileIdentity, b, columns, rows, sha256.Sum256(opData))
	v.mu.Unlock()
	if data, ok := s.terrainTileCache.Get(key); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Terrain-Cache", "hit")
		w.Write(data)
		return
	}
	select {
	case s.detailSlots <- struct{}{}:
		defer func() { <-s.detailSlots }()
	case <-r.Context().Done():
		return
	}
	m, err := tiles.SampleMesh(r.Context(), p, b, columns, rows, transform)
	if err != nil {
		if r.Context().Err() == nil {
			fail(w, 400, err.Error())
		}
		return
	}
	data, err := json.Marshal(m)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	s.terrainTileCache.Put(key, data)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Terrain-Cache", "miss")
	w.Write(data)
}
