// These handlers construct historical fixtures only. They are not compiled into
// the application and must never be registered on production servers.
package httpapi

import (
	"bytes"
	"encoding/json"
	"kriemhild/internal/legacyterrain"
	"kriemhild/internal/storage"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func (s *Server) createLegacyFixture(w http.ResponseWriter, r *http.Request) {
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
	v.creatorID, v.actorID = storage.Actor(r.Context()), storage.Actor(r.Context())
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
func (s *Server) mutateLegacyFixture(w http.ResponseWriter, r *http.Request) {
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
	if v.planet != nil {
		fail(w, 409, "The generated base is immutable; use authored world operations.")
		return
	}
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
func (s *Server) legacyDetailFixture(w http.ResponseWriter, r *http.Request) {
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
	if v.planet != nil {
		fail(w, 409, "Use the continuous terrain tile API for this world.")
		return
	}
	v.last = time.Now()
	// Abandoned view requests must not create extra detail after an explicit Save.
	if r.Context().Err() != nil {
		return
	}
	data, err := v.fixtureDetail(args[0], args[1], args[2])
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

func legacyFixtureRequest(t *testing.T, s *Server, method, path, body string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if method == "POST" && path == "/api/sessions" {
		s.createLegacyFixture(rec, req)
	} else if len(parts) == 4 && parts[0] == "api" && parts[1] == "sessions" && method == "POST" && strings.Contains("|step|cleanup|paint|snapshot|restore|", "|"+parts[3]+"|") {
		req.SetPathValue("id", parts[2])
		req.SetPathValue("action", parts[3])
		s.mutateLegacyFixture(rec, req)
	} else if len(parts) == 7 && parts[3] == "detail" && method == "GET" {
		req.SetPathValue("id", parts[2])
		req.SetPathValue("level", parts[4])
		req.SetPathValue("x", parts[5])
		req.SetPathValue("y", parts[6])
		s.legacyDetailFixture(rec, req)
	} else {
		return autoRequest(t, s, method, path, body)
	}
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("historical fixture %s: %d %s", path, rec.Code, rec.Body.String())
	}
	var out map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
	}
	return out
}
