package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"kriemhild/internal/storage"
	"net/http"
	"os"
	"time"
)

func (s *Server) persistProject(ctx context.Context, v *session, archive []byte) error {
	if v.retired {
		return errors.New("world was replaced by an imported project")
	}
	if s.projects == nil {
		return nil
	}
	seed := ""
	if v.solver.Environment != nil {
		seed = v.solver.Environment.Options.Seed
	}
	err := s.projects.Save(ctx, storage.Project{ID: v.worldID, Seed: seed, Width: v.solver.W, Height: v.solver.H, DetailTiles: len(v.tiles)}, archive)
	if err == nil {
		v.persisted = false
	}
	return err
}

// Import replaces a world atomically, then retires old in-flight writers. The
// caller holds openMu so no concurrent open can install a second live copy.
func (s *Server) replaceWorld(id string, persist func() error) error {
	old := map[string]*session{}
	s.mu.Lock()
	for key, v := range s.sessions {
		v.mu.Lock()
		if v.worldID == id {
			old[key] = v
		} else {
			v.mu.Unlock()
		}
	}
	s.mu.Unlock()
	defer func() {
		for _, v := range old {
			v.mu.Unlock()
		}
	}()
	if err := persist(); err != nil {
		return err
	}
	s.mu.Lock()
	for key, v := range old {
		v.retired = true
		delete(s.sessions, key)
	}
	s.mu.Unlock()
	for _, v := range old {
		if v.dir != "" {
			os.RemoveAll(v.dir)
		}
	}
	return nil
}
func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		respond(w, 200, map[string]any{"database": "disabled", "projects": []storage.Project{}})
		return
	}
	projects, err := s.projects.List(r.Context())
	if err != nil {
		fail(w, 503, "Project database is unavailable")
		return
	}
	respond(w, 200, map[string]any{"database": s.projects.Driver(), "projects": projects})
}
func (s *Server) openStoredProject(w http.ResponseWriter, r *http.Request) {
	s.openMu.Lock()
	defer s.openMu.Unlock()
	if s.projects == nil {
		fail(w, 503, "Project database is not configured")
		return
	}
	// Reuse the authoritative live world rather than creating competing writers.
	s.mu.Lock()
	for id, v := range s.sessions {
		v.mu.Lock()
		if v.worldID == r.PathValue("id") {
			s.mu.Unlock()
			defer v.mu.Unlock()
			v.last = time.Now()
			state := view(v.solver, true)
			state["id"] = id
			state["worldId"] = v.worldID
			state["projectUI"] = v.projectUI
			state["storedDetailCount"] = len(v.tiles)
			respond(w, 201, state)
			return
		}
		v.mu.Unlock()
	}
	s.mu.Unlock()
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		fail(w, 429, "Project loading is busy; retry shortly.")
		return
	}
	parts, err := s.projects.LoadParts(r.Context(), r.PathValue("id"))
	if err != nil {
		fail(w, 503, "Project database is unavailable")
		return
	}
	var v *session
	if len(parts) > 0 {
		v, err = restoreCheckpoint(r.PathValue("id"), parts, s.cacheRoot)
		if err != nil {
			fail(w, 422, "Stored autosave failed validation")
			return
		}
	} else {
		archive, err := s.projects.Load(r.Context(), r.PathValue("id"))
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, 404, "Saved world not found")
			return
		}
		if err != nil {
			fail(w, 503, "Project database is unavailable")
			return
		}
		request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "http://project.local/", bytes.NewReader(archive))
		if err != nil {
			fail(w, 500, "Cannot read saved world")
			return
		}
		files, err := readProjectFiles(request)
		if err != nil {
			fail(w, 422, "Stored world archive is invalid")
			return
		}
		v, err = decodeProject(files, s.cacheRoot)
		if err != nil {
			fail(w, 422, "Stored world failed validation")
			return
		}
	}
	s.mu.Lock()
	if len(s.sessions) >= 32 {
		s.mu.Unlock()
		if v.dir != "" {
			os.RemoveAll(v.dir)
		}
		fail(w, 429, "Too many active maps")
		return
	}
	id := token()
	v.last = time.Now()
	s.sessions[id] = v
	s.mu.Unlock()
	state := view(v.solver, true)
	state["id"] = id
	state["worldId"] = v.worldID
	state["projectUI"] = v.projectUI
	state["storedDetailCount"] = len(v.tiles)
	respond(w, 201, state)
}
