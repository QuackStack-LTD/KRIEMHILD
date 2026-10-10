package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"kriemhild/internal/world"
	"net/http"
	"os"
	"strings"
)

func validHistoryRoute(path string) bool {
	p := strings.Split(strings.Trim(path, "/"), "/")
	if len(p) < 2 || p[0] != "worlds" || len(p[1]) != 32 {
		return false
	}
	for _, c := range p[1] {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return len(p) == 2 || (len(p) == 3 && (p[2] == "graph" || p[2] == "terrains"))
}
func (s *Server) historyRoutes() {
	s.mux.HandleFunc("POST /api/sessions/{id}/name-geography", s.nameGeography)
	s.mux.HandleFunc("GET /api/worlds", s.listHistories)
	s.mux.HandleFunc("POST /api/worlds", s.createHistory)
	s.mux.HandleFunc("POST /api/worlds/import", s.importHistory)
	s.mux.HandleFunc("GET /api/worlds/{world}", s.getHistory)
	s.mux.HandleFunc("POST /api/worlds/{world}", s.commandHistory)
	s.mux.HandleFunc("DELETE /api/worlds/{world}", s.deleteHistory)
	s.mux.HandleFunc("POST /api/worlds/{world}/accept", s.acceptTerrain)
	s.mux.HandleFunc("POST /api/worlds/{world}/child", s.childMap)
	s.mux.HandleFunc("GET /api/worlds/{world}/export", s.exportHistory)
	s.mux.HandleFunc("GET /api/map-context/{project}", s.mapHistoryContext)
}
func (s *Server) historyStore(w http.ResponseWriter) bool {
	if s.projects == nil {
		fail(w, 503, "World database is not configured")
		return false
	}
	return true
}
func (s *Server) listHistories(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	list, err := s.projects.Histories(r.Context())
	if err != nil {
		fail(w, 503, err.Error())
		return
	}
	respond(w, 200, list)
}
func (s *Server) createHistory(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := decode(w, r, &req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	d := history.New(req.Name)
	if err := s.projects.CreateHistory(r.Context(), d); err != nil {
		fail(w, 400, err.Error())
		return
	}
	respond(w, 201, d)
}
func (s *Server) getHistory(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	d, err := s.projects.History(r.Context(), r.PathValue("world"))
	if err != nil {
		fail(w, 404, "World not found")
		return
	}
	respond(w, 200, d)
}
func (s *Server) commandHistory(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	var req history.Command
	if err := decode(w, r, &req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	d, err := s.projects.UpdateHistory(r.Context(), r.PathValue("world"), req.Revision, func(d *history.Document) error { return d.Apply(req) })
	if err != nil {
		fail(w, 409, err.Error())
		return
	}
	respond(w, 200, d)
}
func (s *Server) deleteHistory(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	s.openMu.Lock()
	defer s.openMu.Unlock()
	d, err := s.projects.History(r.Context(), r.PathValue("world"))
	if err != nil {
		fail(w, 404, "World not found")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.sessions {
		v.mu.Lock()
	}
	defer func() {
		for _, v := range s.sessions {
			v.mu.Unlock()
		}
	}()
	if err = s.projects.DeleteHistory(r.Context(), d.World.ID); err != nil {
		fail(w, 503, err.Error())
		return
	}
	for _, v := range s.sessions {
		if d.Project(v.worldID) != nil {
			v.retired = true
		}
	}
	w.WriteHeader(204)
}
func (s *Server) acceptTerrain(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	var req struct {
		Session string `json:"session"`
		AgeID   string `json:"ageId"`
		Name    string `json:"name"`
	}
	if err := decode(w, r, &req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	r.SetPathValue("id", req.Session)
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	if v.solver.Status != "done" {
		fail(w, 409, "Finish terrain generation before accepting it")
		return
	}
	if v.historical != nil {
		fail(w, 409, "Terrain is already accepted")
		return
	}
	if strings.TrimSpace(req.Name) == "" || len(req.Name) > 160 {
		fail(w, 400, "Provide a terrain name")
		return
	}
	target, err := s.projects.History(r.Context(), r.PathValue("world"))
	if err != nil || target.Age(req.AgeID) == nil {
		fail(w, 404, "Choose an existing World and Age before accepting terrain")
		return
	}
	v.authored().Header.Name = req.Name
	if err := s.saveWorld(v, true); err != nil {
		fail(w, 503, err.Error())
		return
	}
	parts, err := s.projects.LoadParts(r.Context(), v.worldID)
	if err != nil {
		fail(w, 503, err.Error())
		return
	}
	d, m, err := s.projects.AttachTerrain(r.Context(), r.PathValue("world"), req.AgeID, v.worldID, req.Name, v.solver.W, v.solver.H, parts, v.authored())
	if err != nil {
		fail(w, 409, err.Error())
		return
	}
	v.historical = &m
	v.historyWorld = d.World.ID
	respond(w, 201, map[string]any{"document": d, "map": m})
}
func (s *Server) childMap(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	var req struct {
		AgeID    string       `json:"ageId"`
		MapID    string       `json:"mapId"`
		EntityID string       `json:"entityId"`
		Name     string       `json:"name"`
		Bounds   world.Bounds `json:"bounds"`
		Revision int          `json:"revision"`
	}
	if err := decode(w, r, &req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if strings.TrimSpace(req.Name) == "" || len(req.Name) > 160 {
		fail(w, 400, "Name the detailed map")
		return
	}
	var m history.Map
	d, err := s.projects.UpdateHistory(r.Context(), r.PathValue("world"), req.Revision, func(d *history.Document) error {
		var err error
		m, err = d.Child(req.AgeID, req.MapID, req.EntityID, req.Name, req.Bounds)
		return err
	})
	if err != nil {
		fail(w, 409, err.Error())
		return
	}
	respond(w, 201, map[string]any{"document": d, "map": m})
}
func (s *Server) mapHistoryContext(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	d, m, err := s.projects.MapContext(r.Context(), r.PathValue("project"))
	if errors.Is(err, sql.ErrNoRows) {
		respond(w, 200, map[string]any{"map": nil})
		return
	}
	if err != nil {
		fail(w, 503, err.Error())
		return
	}
	respond(w, 200, map[string]any{"world": d.World, "map": m})
}
func (s *Server) applyHistoricalMap(ctx context.Context, v *session) error {
	if s.projects == nil {
		return nil
	}
	d, m, err := s.projects.MapContext(ctx, v.worldID)
	if errors.Is(err, sql.ErrNoRows) {
		if v.historical != nil {
			return fmt.Errorf("this historical map was deleted")
		}
		return nil
	}
	if err != nil {
		return err
	}
	v.historical = m
	v.historyWorld = d.World.ID
	model := v.authored()
	if model.Header.MapID != m.ID {
		model.Header.History = []world.Change{}
		model.Header.Cursor = 0
		model.Header.View = json.RawMessage(`{}`)
	}
	model.Header.MapID = m.ID
	model.Header.Name = m.Name
	model.RefreshRepresentations(d.Shapes(*m))
	v.composed = nil
	return nil
}
func (s *Server) exportHistory(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	b, err := s.projects.HistoryBundle(r.Context(), r.PathValue("world"))
	if err != nil {
		fail(w, 422, err.Error())
		return
	}
	var output bytes.Buffer
	z := zip.NewWriter(&output)
	add := func(name string, data []byte) error {
		f, err := z.Create(name)
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		return err
	}
	manifest, _ := json.Marshal(b)
	if err = add("manifest.json", manifest); err != nil {
		fail(w, 500, err.Error())
		return
	}
	for h, data := range b.Blobs {
		if err = add("blobs/"+h+".bin", data); err != nil {
			fail(w, 500, err.Error())
			return
		}
	}
	if err = z.Close(); err != nil {
		fail(w, 500, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="KRIEMHILD.world.zip"`)
	w.Write(output.Bytes())
}
func (s *Server) importHistory(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxProjectBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		fail(w, 413, "World archive too large")
		return
	}
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		fail(w, 400, "Invalid ZIP archive")
		return
	}
	files := map[string][]byte{}
	var total int64
	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !safeProjectPath(f.Name) || files[f.Name] != nil {
			fail(w, 400, "Invalid or duplicate archive path")
			return
		}
		reader, err := f.Open()
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		data, err := io.ReadAll(io.LimitReader(reader, maxExpandedBytes-total+1))
		reader.Close()
		total += int64(len(data))
		if err != nil || total > maxExpandedBytes {
			fail(w, 413, "Expanded archive too large")
			return
		}
		files[f.Name] = data
	}
	var b storage.Bundle
	if err = json.Unmarshal(files["manifest.json"], &b); err != nil || b.Format != "KRIEMHILD-WORLD" {
		fail(w, 400, "This is a terrain archive. Import it using Import legacy terrain instead.")
		return
	}
	b.Blobs = map[string][]byte{}
	for path, data := range files {
		if strings.HasPrefix(path, "blobs/") && strings.HasSuffix(path, ".bin") {
			b.Blobs[strings.TrimSuffix(strings.TrimPrefix(path, "blobs/"), ".bin")] = data
		}
	}
	if err = b.Validate(); err != nil {
		fail(w, 422, err.Error())
		return
	}
	// Validate every saved terrain and explored tile before committing anything.
	for id, manifest := range b.Snapshots {
		parts := map[string][]byte{}
		for path, h := range manifest {
			parts[path] = b.Blobs[h]
		}
		v, err := restoreCheckpoint(id, parts, s.cacheRoot)
		if err != nil {
			fail(w, 422, fmt.Sprintf("Invalid terrain snapshot: %v", err))
			return
		}
		if v.dir != "" {
			os.RemoveAll(v.dir)
		}
	}
	if _, lookupErr := s.projects.History(r.Context(), b.Document.World.ID); lookupErr == nil {
		// Import alongside an existing World as a copy, without replacing its edits.
		b.Document.World.ID = history.ID()
	} else if !errors.Is(lookupErr, sql.ErrNoRows) {
		fail(w, 503, lookupErr.Error())
		return
	}
	for i := range b.Document.Maps {
		if _, _, lookupErr := s.projects.MapContext(r.Context(), b.Document.Maps[i].ProjectID); lookupErr == nil {
			b.Document.Maps[i].ProjectID = history.ID()
		} else if !errors.Is(lookupErr, sql.ErrNoRows) {
			fail(w, 503, lookupErr.Error())
			return
		}
	}
	if err = s.projects.ImportHistory(r.Context(), b); err != nil {
		fail(w, 422, err.Error())
		return
	}
	respond(w, 201, b.Document)
}
