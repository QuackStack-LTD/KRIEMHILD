package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/world"
	"net/http"
)

func (s *Server) landSelection(w http.ResponseWriter, r *http.Request) {
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	if v.planet == nil || v.planet.Surface == nil {
		v.mu.Unlock()
		fail(w, 409, "This terrain has no saved land surface")
		return
	}
	p := v.planet
	surface := p.Surface
	state := v.authored()
	ops := state.Ops(world.Bounds{Width: float64(v.solver.W - 1), Height: float64(v.solver.H - 1)})
	raw, _ := json.Marshal(ops)
	key := fmt.Sprintf("connected-land/1/%s/%d/%x", r.PathValue("id"), state.Header.Revision, sha256.Sum256(raw))
	width, height, revision := v.solver.W, v.solver.H, state.Header.Revision
	transform := continuousTransform(ops, width, height, p)
	v.mu.Unlock()
	if r.URL.Query().Get("key") == "1" {
		respond(w, 200, map[string]any{"revision": revision, "sourceKey": fmt.Sprintf("%x", sha256.Sum256(raw))})
		return
	}
	if data, ok := s.terrainTileCache.Get(key); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return
	}
	values := append([]float32(nil), surface.Coast...)
	for i := range values {
		if i%surface.Width == 0 && r.Context().Err() != nil {
			return
		}
		if surface.Coast[i] > 0 && surface.LakeLevel[i] > 0 {
			values[i] = surface.Bed[i] - surface.LakeLevel[i]
		}
		if transform == nil {
			continue
		}
		x, y := float64(i%surface.Width)/float64(surface.Width)*float64(width-1), (float64(i/surface.Width)+.5)/float64(surface.Height)*float64(height-1)
		affected := false
		for _, op := range ops {
			if op.Bounds().Intersects(world.Bounds{X: x, Y: y}) {
				affected = true
				break
			}
		}
		if !affected {
			continue
		}
		lon, lat := (x/float64(width-1)-.5)*model.Tau, (.5-y/float64(height-1))*model.Tau/2
		sample := model.Sample{Bed: float64(surface.Bed[i]), Water: "dry"}
		if values[i] <= 0 {
			sample.Water = "ocean"
			sample.WaterDepth = -sample.Bed
		}
		sample = transform(sample, lon, lat)
		if sample.Water == "dry" {
			values[i] = float32(max(.001, sample.Bed-sample.WaterSurface))
		} else {
			values[i] = float32(-max(.001, sample.WaterDepth))
		}
	}
	components, err := landmass.Components(r.Context(), surface.Width, surface.Height, values)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	data, err := json.Marshal(map[string]any{"revision": revision, "sourceWidth": surface.Width, "components": components, "sourceKey": fmt.Sprintf("%x", sha256.Sum256(raw))})
	if err != nil {
		fail(w, 500, "Could not prepare land selection")
		return
	}
	s.terrainTileCache.Put(key, data)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func landKey(v *session) string {
	ops := v.authored().Ops(world.Bounds{Width: float64(v.solver.W - 1), Height: float64(v.solver.H - 1)})
	raw, _ := json.Marshal(ops)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}
