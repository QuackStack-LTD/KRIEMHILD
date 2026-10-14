package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"kriemhild/internal/legacyterrain"
	"kriemhild/internal/storage"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/terrain/generate"
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/terrain/tiles"
	"kriemhild/internal/world"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type terrainJob struct {
	mu       sync.Mutex
	actor    string
	cancel   context.CancelFunc
	created  time.Time
	Stage    string  `json:"stage"`
	Progress float64 `json:"progress"`
	Status   string  `json:"status"`
	Error    string  `json:"error,omitempty"`
	Session  string  `json:"session,omitempty"`
}

func (s *Server) continuousRoutes() {
	s.terrainJobs = map[string]*terrainJob{}
	s.terrainTileCache = tiles.NewPersistentCache(64<<20, filepath.Join(s.cacheRoot, "continuous-tiles-v2"), 512<<20)
	s.mux.HandleFunc("POST /api/terrain/generation-jobs", s.startTerrainJob)
	s.mux.HandleFunc("GET /api/terrain/generation-jobs/{job}", s.terrainJobState)
	s.mux.HandleFunc("DELETE /api/terrain/generation-jobs/{job}", s.terrainJobState)
	s.mux.HandleFunc("GET /api/sessions/{id}/terrain/tiles/{z}/{x}/{y}", s.continuousTile)
	s.mux.HandleFunc("GET /api/sessions/{id}/terrain/sample", s.continuousSample)
	s.mux.HandleFunc("GET /api/sessions/{id}/terrain/mesh", s.continuousMesh)
	s.mux.HandleFunc("GET /api/sessions/{id}/terrain/features", s.continuousFeatures)
	s.mux.HandleFunc("POST /api/sessions/{id}/settlements/preview", s.previewSettlement)
	s.mux.HandleFunc("POST /api/sessions/{id}/settlements", s.acceptSettlement)
}
func (s *Server) startTerrainJob(w http.ResponseWriter, r *http.Request) {
	defaults := model.DefaultSpec()
	defaults.Seed = ""
	req := struct {
		Spec model.Spec `json:"spec"`
	}{Spec: defaults}
	if err := decode(w, r, &req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if req.Spec.Seed == "" {
		req.Spec.Seed = token()
	}
	if err := req.Spec.Validate(); err != nil {
		fail(w, 400, err.Error())
		return
	}
	select {
	case s.slots <- struct{}{}:
	default:
		fail(w, 429, "Generation is busy; try again shortly.")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	job := &terrainJob{actor: storage.Actor(r.Context()), cancel: cancel, created: time.Now(), Stage: "queued", Status: "running"}
	id := token()
	s.mu.Lock()
	for key, j := range s.terrainJobs {
		if time.Since(j.created) > 30*time.Minute {
			j.cancel()
			delete(s.terrainJobs, key)
		}
	}
	if len(s.terrainJobs) >= 32 || len(s.sessions) >= 32 {
		s.mu.Unlock()
		cancel()
		<-s.slots
		fail(w, 429, "Close an old terrain preview before generating another.")
		return
	}
	s.terrainJobs[id] = job
	s.mu.Unlock()
	go func() {
		defer func() { <-s.slots; cancel() }()
		p, err := generate.Generate(ctx, req.Spec, func(stage string, f float64) { job.mu.Lock(); job.Stage, job.Progress = stage, f*.9; job.mu.Unlock() })
		var solver *terrain.Solver
		if err == nil {
			job.mu.Lock()
			job.Stage, job.Progress = "project integration", .95
			job.mu.Unlock()
			start := time.Now()
			solver, err = legacyProjection(p)
			p.Diagnostics.Timings["platform projection"] = float64(time.Since(start).Microseconds()) / 1000
		}
		job.mu.Lock()
		defer job.mu.Unlock()
		if ctx.Err() != nil {
			job.Status = "cancelled"
			return
		}
		if err != nil {
			job.Status = "failed"
			job.Error = err.Error()
			return
		}
		sid := token()
		gen, _ := json.Marshal(req)
		v := &session{planet: p, solver: solver, last: time.Now(), undo: map[string]terrain.Snapshot{}, generation: gen, cacheRoot: s.cacheRoot, worldID: token(), creatorID: job.actor, actorID: job.actor, dirty: true, unsaved: true}
		s.mu.Lock()
		s.sessions[sid] = v
		s.mu.Unlock()
		job.Status, job.Stage, job.Progress, job.Session = "ready", "ready", 1, sid
	}()
	respond(w, 202, map[string]string{"id": id, "status": "running"})
}
func (s *Server) terrainJobState(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	job := s.terrainJobs[r.PathValue("job")]
	s.mu.Unlock()
	if job == nil || job.actor != storage.Actor(r.Context()) {
		fail(w, 404, "Generation job not found")
		return
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	if r.Method == "DELETE" {
		job.cancel()
		if job.Status == "running" {
			job.Status = "cancelled"
		}
		w.WriteHeader(204)
		return
	}
	respond(w, 200, map[string]any{"stage": job.Stage, "progress": job.Progress, "status": job.Status, "error": job.Error, "session": job.Session})
}
func continuousTransform(ops []world.Operation, w, h int) tiles.Transform {
	if len(ops) == 0 {
		return nil
	}
	return func(s model.Sample, lon, lat float64) model.Sample {
		x, y := (model.Wrap(lon)+math.Pi)/model.Tau*float64(w-1), (.5-lat/math.Pi)*float64(h-1)
		wb := 0
		if s.Water != "dry" {
			wb = 1
		}
		a := world.Apply(terrain.DetailPoint{Elevation: s.Bed, WaterLevel: s.WaterSurface, WaterDepth: s.WaterDepth, WaterBody: wb, Vegetation: s.Moisture, Biome: s.Biome}, x, y, ops)
		s.Temperature -= (a.Elevation - s.Bed) * .0065
		s.Bed, s.WaterSurface, s.WaterDepth, s.Moisture, s.Biome = a.Elevation, a.WaterLevel, a.WaterDepth, a.Vegetation, a.Biome
		if a.WaterBody == 0 {
			s.Water = "dry"
			if s.Biome == "water" {
				s.Biome = generate.Biome(s.Temperature, s.Precipitation, s.PET, s.Bed, false)
			}
		} else if wb == 0 {
			s.Water = "lake"
		}
		return s
	}
}

func (v *session) prepareContinuousOperation(op *world.Operation) {
	if op.Kind != "smooth" {
		return
	}
	sampler := detail.New(v.planet)
	op.Samples = make([]float64, 81)
	for j := 0; j < 9; j++ {
		for i := 0; i < 9; i++ {
			x, y := op.Center[0]-op.Radius+float64(i)*op.Radius/4, op.Center[1]-op.Radius+float64(j)*op.Radius/4
			for _, offset := range [][2]float64{{0, 0}, {-.25, 0}, {.25, 0}, {0, -.25}, {0, .25}} {
				px, py := model.Clamp(x+offset[0]*op.Radius, 0, float64(v.solver.W-1)), model.Clamp(y+offset[1]*op.Radius, 0, float64(v.solver.H-1))
				lon, lat := -math.Pi+model.Tau*px/float64(v.solver.W-1), math.Pi/2-math.Pi*py/float64(v.solver.H-1)
				s := sampler.Sample(lon, lat)
				if f := continuousTransform(v.authored().Ops(world.Bounds{X: px, Y: py}), v.solver.W, v.solver.H); f != nil {
					s = f(s, lon, lat)
				}
				op.Samples[j*9+i] += s.Bed / 5
			}
		}
	}
}
func (s *Server) continuousTile(w http.ResponseWriter, r *http.Request) {
	var a [3]int
	for i, k := range []string{"z", "x", "y"} {
		v, err := strconv.Atoi(r.PathValue(k))
		if err != nil {
			fail(w, 400, "Invalid tile address")
			return
		}
		a[i] = v
	}
	size := 256
	if r.URL.Query().Get("size") == "512" {
		size = 512
	}
	style := r.URL.Query().Get("style")
	if style == "" {
		style = "natural"
	}
	if !tiles.Valid(a[0], a[1], a[2], size) {
		fail(w, 400, "Invalid tile address")
		return
	}
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	if v.planet == nil {
		v.mu.Unlock()
		fail(w, 409, "This archive uses the legacy terrain reader.")
		return
	}
	p := v.planet
	revision := v.authored().Header.Revision
	if r.URL.Query().Get("strict") == "1" && r.URL.Query().Get("revision") != strconv.Itoa(revision) {
		v.mu.Unlock()
		fail(w, 409, "Terrain changed during export")
		return
	}
	lon0, lat0 := tiles.Coordinate(a[0], a[1], a[2], size, -1, -1)
	lon1, lat1 := tiles.Coordinate(a[0], a[1], a[2], size, size, size)
	box := world.Bounds{X: (lon0 + math.Pi) / model.Tau * float64(v.solver.W-1), Y: (.5 - lat0/math.Pi) * float64(v.solver.H-1), Width: (lon1 - lon0) / model.Tau * float64(v.solver.W-1), Height: (lat0 - lat1) / math.Pi * float64(v.solver.H-1)}
	ops := append([]world.Operation(nil), v.authored().Ops(box.Expand(.1))...)
	if v.tileIdentity == "" {
		meta, fields, err := p.Encode()
		if err != nil {
			v.mu.Unlock()
			fail(w, 500, err.Error())
			return
		}
		h := sha256.New()
		h.Write(meta)
		h.Write(fields)
		v.tileIdentity = fmt.Sprintf("%x", h.Sum(nil))
	}
	opBytes, _ := json.Marshal(ops)
	key := fmt.Sprintf("%s/%d/%d/%d/%d/%s/%x", v.tileIdentity, a[0], a[1], a[2], size, style, sha256.Sum256(opBytes))
	transform := continuousTransform(ops, v.solver.W, v.solver.H)
	v.mu.Unlock()
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("X-Terrain-Version", p.Spec.Version)
	w.Header().Set("X-Terrain-Revision", fmt.Sprint(revision))
	if data, ok := s.terrainTileCache.Get(key); ok {
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
	if data, ok := s.terrainTileCache.Get(key); ok {
		w.Header().Set("X-Terrain-Cache", "hit")
		w.Write(data)
		return
	}
	start := time.Now()
	data, err := tiles.Render(r.Context(), p, a[0], a[1], a[2], size, style, transform)
	if err != nil {
		if r.Context().Err() == nil {
			fail(w, 400, err.Error())
		}
		return
	}
	s.terrainTileCache.Put(key, data)
	w.Header().Set("Server-Timing", fmt.Sprintf("terrain;dur=%.2f", float64(time.Since(start).Microseconds())/1000))
	w.Header().Set("X-Terrain-Cache", "miss")
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("X-Terrain-Version", p.Spec.Version)
	w.Header().Set("X-Terrain-Revision", fmt.Sprint(revision))
	w.Write(data)
}
func (s *Server) continuousSample(w http.ResponseWriter, r *http.Request) {
	lon, e1 := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	lat, e2 := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	if e1 != nil || e2 != nil || !model.Finite(lon) || !model.Finite(lat) || math.Abs(lat) > 90 {
		fail(w, 400, "Provide longitude and latitude in degrees")
		return
	}
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	if v.planet == nil {
		fail(w, 409, "Legacy terrain has no continuous sampler")
		return
	}
	lon *= math.Pi / 180
	lat *= math.Pi / 180
	a := detail.New(v.planet).Sample(lon, lat)
	x, y := (model.Wrap(lon)+math.Pi)/model.Tau*float64(v.solver.W-1), (.5-lat/math.Pi)*float64(v.solver.H-1)
	if f := continuousTransform(v.authored().Ops(world.Bounds{X: x, Y: y}), v.solver.W, v.solver.H); f != nil {
		a = f(a, lon, lat)
	}
	var mass any
	if a.Water == "dry" {
		if m := landmass.Identity(v.planet, lon, lat); m != nil {
			mass = map[string]any{"id": m.ID, "family": m.Family, "center": m.Center, "extent": m.Extent}
		}
	}
	respond(w, 200, struct {
		model.Sample
		Landmass any                `json:"landmass,omitempty"`
		Climate  map[string]float64 `json:"climate"`
	}{a, mass, sampleClimate(v.planet, lon, lat)})
}
func sampleClimate(p *model.Planet, lon, lat float64) map[string]float64 {
	fields := map[string]float64{}
	for _, key := range []string{"soil", "aridity", "seasonality", "dryMonths", "rainShadow", "exposure", "slope", "windX", "windY"} {
		if len(p.Grid.Fields[key]) > 0 {
			fields[key] = p.Grid.At(key, lon, lat)
		}
	}
	return fields
}
func (s *Server) continuousFeatures(w http.ResponseWriter, r *http.Request) {
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	if v.planet == nil {
		fail(w, 409, "Legacy terrain")
		return
	}
	respond(w, 200, map[string]any{"rivers": v.planet.Rivers, "legend": tiles.BiomeColors, "masses": v.planet.Masses, "revision": v.authored().Header.Revision, "units": "radians; elevations and widths in metres; discharge in m3/s"})
}
