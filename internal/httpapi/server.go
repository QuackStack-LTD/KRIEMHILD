package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"kriemhild/internal/terrain"
	"kriemhild/internal/world"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type session struct {
	climate         authoredClimate
	natural         *terrain.NaturalIndex
	mu              sync.Mutex
	solver          *terrain.Solver
	last            time.Time
	undo            map[string]terrain.Snapshot
	order           []string
	detail          *terrain.DetailModel
	dir             string
	cacheRoot       string
	tiles           map[string]bool
	generation      json.RawMessage
	projectUI       json.RawMessage
	worldID         string
	base            *terrain.Snapshot
	edits           []json.RawMessage
	persisted       bool
	unsaved         bool
	dirty           bool
	detailPersisted bool
	savedTiles      map[string]bool
	viewClient      string
	viewSequence    uint64
	retired         bool
	historical      *history.Map
	historyWorld    string
	world           *world.State
	composed        map[string][]byte
}
type Server struct {
	mu          sync.Mutex
	sessions    map[string]*session
	mux         *http.ServeMux
	slots       chan struct{}
	openMu      sync.Mutex
	detailSlots chan struct{}
	projects    *storage.Store
	cacheRoot   string
}

type Options struct {
	Projects *storage.Store
	CacheDir string
}

func New(dist string) *Server {
	return NewWithOptions(dist, Options{})
}
func NewWithOptions(dist string, options Options) *Server {
	s := &Server{sessions: map[string]*session{}, mux: http.NewServeMux(), slots: make(chan struct{}, 4), detailSlots: make(chan struct{}, 3)}
	s.projects = options.Projects
	s.cacheRoot = options.CacheDir
	if s.cacheRoot == "" {
		s.cacheRoot = filepath.Join(".tools", "projects")
	}
	s.mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]string{"name": "KRIEMHILD", "status": "ok", "engine": "native-go"})
	})
	s.mux.HandleFunc("GET /api/ready", func(w http.ResponseWriter, r *http.Request) {
		if s.projects != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if s.projects.Ping(ctx) != nil {
				fail(w, 503, "Project database is unavailable")
				return
			}
		}
		respond(w, 200, map[string]string{"status": "ready"})
	})
	s.mux.HandleFunc("GET /api/defaults", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(terrain.Defaults)
	})
	s.mux.HandleFunc("POST /api/sessions", s.create)
	s.mux.HandleFunc("DELETE /api/sessions/{id}", s.remove)
	s.mux.HandleFunc("POST /api/sessions/{id}/{action}", s.action)
	s.mux.HandleFunc("GET /api/sessions/{id}/detail/{level}/{x}/{y}", s.detailTile)
	s.mux.HandleFunc("POST /api/sessions/{id}/project", s.saveProject)
	s.mux.HandleFunc("POST /api/sessions/{id}/autosave", s.saveView) // Legacy endpoint stages view only.
	s.mux.HandleFunc("POST /api/sessions/{id}/view", s.saveView)
	s.mux.HandleFunc("POST /api/sessions/{id}/save", s.commitWorld)
	s.mux.HandleFunc("DELETE /api/projects/{id}", s.deleteProject)
	s.mux.HandleFunc("GET /api/sessions/{id}/world", s.worldInfo)
	s.mux.HandleFunc("GET /api/sessions/{id}/world/state", s.worldState)
	s.mux.HandleFunc("GET /api/sessions/{id}/world/entities", s.worldEntities)
	s.mux.HandleFunc("GET /api/sessions/{id}/world/natural", s.worldNatural)
	s.mux.HandleFunc("GET /api/sessions/{id}/world/climate", s.worldClimateOverlay)
	s.mux.HandleFunc("POST /api/sessions/{id}/world", s.worldCommand)
	s.mux.HandleFunc("POST /api/projects/import", s.importProject)
	s.mux.HandleFunc("GET /api/projects", s.listProjects)
	s.mux.HandleFunc("POST /api/projects/{id}/open", s.openStoredProject)
	s.historyRoutes()
	s.mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { fail(w, 404, "API route not found") })
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		file := filepath.Join(dist, filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/")))
		if info, err := os.Stat(file); err == nil && !info.IsDir() {
			http.ServeFile(w, r, file)
			return
		}
		if r.URL.Path != "/" && r.URL.Path != "/world/new" && !validWorldRoute(r.URL.Path) && !validHistoryRoute(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		if _, err := os.Stat(filepath.Join(dist, "index.html")); err != nil {
			http.Error(w, "Build the React frontend with npm run build, or use http://localhost:5173 during development.", 503)
			return
		}
		http.ServeFile(w, r, filepath.Join(dist, "index.html"))
	})
	return s
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Cache-Control", "no-store")
		if origin := r.Header.Get("Origin"); origin != "" {
			allowed := origin == "http://"+r.Host || origin == "https://"+r.Host || origin == "http://127.0.0.1:5173" || origin == "http://localhost:5173"
			if !allowed {
				fail(w, 403, "Origin not allowed")
				return
			}
		}
	}
	s.mux.ServeHTTP(w, r)
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	d := json.NewDecoder(r.Body)
	if err := d.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return fmt.Errorf("expected one JSON object")
	}
	return nil
}
func token() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

type createRequest struct {
	Config      json.RawMessage `json:"config"`
	Options     terrain.Options `json:"options"`
	Environment json.RawMessage `json:"environment"`
	WrapX       bool            `json:"wrapX"`
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := decode(w, r, &req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		fail(w, 429, "Generation is busy; try again shortly.")
		return
	}
	raw := req.Config
	if len(raw) == 0 {
		raw = terrain.Defaults
	}
	config, err := terrain.DecodeConfig(raw)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	rules, err := terrain.Compile(config)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	var solver *terrain.Solver
	if len(req.Environment) > 0 && string(req.Environment) != "null" {
		opts, e := terrain.DecodeEnvironment(req.Environment)
		if e != nil {
			fail(w, 400, e.Error())
			return
		}
		solver, err = terrain.PrepareEnvironment(opts, config)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		solver.WrapX = req.WrapX || solver.Environment != nil && solver.Environment.Geography != nil && solver.Environment.Geography.WrapX
	} else {
		o := req.Options
		if o.Width < 16 || o.Width > 256 || o.Height < 16 || o.Height > 256 {
			fail(w, 400, "Width and height must be between 16 and 256.")
			return
		}
		if o.Radius2 < 1 || o.Radius2 > 25 || o.Stability < 0 || o.Stability > 300 {
			fail(w, 400, "Invalid radius or stability.")
			return
		}
		if o.Continents != nil && (o.Continents.Count < 1 || o.Continents.Count > 40 || o.Continents.Strength < 0 || o.Continents.Strength > 20) {
			fail(w, 400, "Invalid continental settings.")
			return
		}
		if o.Climate != nil && (o.Climate.Strength < 0 || o.Climate.Strength > 5) {
			fail(w, 400, "Invalid climate strength.")
			return
		}
		solver = terrain.NewSolver(rules, o, nil)
	}
	s.mu.Lock()
	for id, v := range s.sessions {
		if v.mu.TryLock() {
			expired := time.Since(v.last) > 30*time.Minute
			v.mu.Unlock()
			if expired {
				if v.dir != "" {
					_ = os.RemoveAll(v.dir)
				}
				delete(s.sessions, id)
			}
		}
	}
	if len(s.sessions) >= 32 {
		s.mu.Unlock()
		fail(w, 429, "Too many active maps. Close an old map or try again later.")
		return
	}
	id := token()
	gen, _ := json.Marshal(req)
	v := &session{solver: solver, last: time.Now(), undo: map[string]terrain.Snapshot{}, generation: gen, cacheRoot: s.cacheRoot, worldID: token()}
	s.sessions[id] = v
	v.mu.Lock()
	s.mu.Unlock()
	defer v.mu.Unlock()
	if v.retired {
		fail(w, 409, "World was deleted or replaced; reopen it from Projects.")
		return
	}
	v.dirty, v.unsaved = true, true
	state := sessionState(v)
	state["id"] = id
	state["worldId"] = v.worldID
	respond(w, 201, state)
}
func (s *Server) remove(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	v := s.sessions[r.PathValue("id")]
	delete(s.sessions, r.PathValue("id"))
	s.mu.Unlock()
	if v != nil {
		v.mu.Lock()
		v.retired = true
		if v.dir != "" {
			_ = os.RemoveAll(v.dir)
		}
		v.mu.Unlock()
	}
	w.WriteHeader(204)
}

func (s *Server) detailTile(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	v := s.sessions[r.PathValue("id")]
	s.mu.Unlock()
	if v == nil {
		fail(w, 404, "This map session has expired. Generate a new map.")
		return
	}
	var args [3]int
	for k, name := range []string{"level", "x", "y"} {
		n, err := strconv.Atoi(r.PathValue(name))
		if err != nil || n < 0 || n > 65536 {
			fail(w, 400, "Invalid detail coordinates")
			return
		}
		args[k] = n
	}
	select {
	case s.detailSlots <- struct{}{}:
		defer func() { <-s.detailSlots }()
	default:
		fail(w, 429, "Detail generation is busy; retry shortly.")
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.retired {
		fail(w, 409, "World was deleted or replaced; reopen it from Projects.")
		return
	}
	if v.solver.Environment == nil {
		fail(w, 400, "Continuous detail requires Physical environment")
		return
	}
	v.last = time.Now()
	// Abandoned view requests must not create extra detail after an explicit Save.
	if r.Context().Err() != nil {
		return
	}
	data, err := v.composedDetail(args[0], args[1], args[2])
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	if !v.savedTiles[detailKey(args[0], args[1], args[2])] {
		v.unsaved = true
	}
	w.Header().Set("X-World-Unsaved", strconv.FormatBool(v.unsaved))
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
func (s *Server) action(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	v := s.sessions[r.PathValue("id")]
	s.mu.Unlock()
	if v == nil {
		fail(w, 404, "This map session has expired. Generate a new map.")
		return
	}
	var req struct {
		Count int    `json:"count"`
		Cells []int  `json:"cells"`
		Type  int    `json:"type"`
		Token string `json:"token"`
	}
	if err := decode(w, r, &req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.retired {
		fail(w, 409, "World was deleted or replaced; reopen it from Projects.")
		return
	}
	v.last = time.Now()
	solver := v.solver
	out := map[string]any{}
	start := time.Now()
	switch r.PathValue("action") {
	case "step":
		if req.Count < 1 || req.Count > 10000 {
			fail(w, 400, "Step count must be between 1 and 10,000.")
			return
		}
		for i := 0; i < req.Count && solver.Status == "running"; i++ {
			solver.Step()
			if time.Since(start) > 30*time.Millisecond {
				break
			}
		}
	case "cleanup":
		if req.Count < 1 || req.Count > 50 {
			fail(w, 400, "Cleanup count must be between 1 and 50.")
			return
		}
		changed := 0
		for i := 0; i < req.Count; i++ {
			n := solver.Cleanup()
			changed += n
			if n == 0 {
				break
			}
		}
		out["changed"] = changed
	case "snapshot":
		id := token()
		v.undo[id] = solver.Snapshot()
		v.order = append(v.order, id)
		if len(v.order) > 30 {
			delete(v.undo, v.order[0])
			v.order = v.order[1:]
		}
		v.dirty, v.unsaved = true, true
		respond(w, 200, map[string]string{"token": id})
		return
	case "restore":
		snap, ok := v.undo[req.Token]
		if !ok {
			fail(w, 400, "Undo snapshot is no longer available.")
			return
		}
		solver.Restore(snap)
		delete(v.undo, req.Token)
	case "paint":
		if req.Type < 0 || req.Type >= solver.T || len(req.Cells) > solver.N {
			fail(w, 400, "Invalid brush type or cells.")
			return
		}
		for _, c := range req.Cells {
			if c < 0 || c >= solver.N {
				fail(w, 400, "Brush cell outside map.")
				return
			}
		}
		if v.base == nil && solver.Status == "done" {
			snap := solver.Snapshot()
			v.base = &snap
		}
		out["painted"] = solver.Paint(req.Cells, req.Type)
		if out["painted"] == true {
			entry, _ := json.Marshal(map[string]any{"action": "paint", "cells": req.Cells, "type": req.Type})
			v.edits = append(v.edits, entry)
		}
		for solver.Status == "running" {
			solver.Step()
			if time.Since(start) > 100*time.Millisecond {
				break
			}
		}
	default:
		fail(w, 404, "Unknown map action.")
		return
	}
	v.dirty, v.unsaved = true, true
	state := view(solver, false)
	state["unsaved"] = v.unsaved
	out["state"] = state
	out["solveMs"] = float64(time.Since(start).Microseconds()) / 1000
	respond(w, 200, out)
}
func view(s *terrain.Solver, initial bool) map[string]any {
	out := map[string]any{"dom": s.Dom, "locked": s.Locked, "pinned": s.Pinned, "status": s.Status, "message": s.Message, "steps": s.Steps, "backtracks": s.Backtracks, "repairs": s.Repairs, "cleaned": s.Cleaned, "settled": s.N - s.Open, "prefer": s.Prefer}
	if initial {
		out["W"] = s.W
		out["H"] = s.H
		out["N"] = s.N
		out["T"] = s.T
		out["K"] = s.K
		out["Z"] = s.Z
		out["wrapX"] = s.WrapX
		out["stability"] = s.Stability
		out["contShare"] = s.ContShare
		out["climShare"] = s.ClimShare
		out["contStrength"] = s.ContStrength
		out["contNeutral"] = s.ContNeutral
		out["climStrength"] = s.ClimStrength
		out["contPoints"] = s.ContPoints
		out["environment"] = renderEnvironment(s.Environment)
		out["config"] = s.Rules.Config
		dx, dy := []int{}, []int{}
		for _, d := range s.Offsets {
			dx = append(dx, d[0])
			dy = append(dy, d[1])
		}
		out["DX"] = dx
		out["DY"] = dy
		out["D"] = len(dx)
	}
	return out
}
