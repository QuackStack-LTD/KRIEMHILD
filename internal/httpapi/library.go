package httpapi

import (
	"encoding/json"
	"io"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

func (s *Server) libraryRoutes() {
	s.mux.HandleFunc("POST /api/worlds/{world}/archives/gather", func(w http.ResponseWriter, r *http.Request) {
		if !s.historyStore(w) {
			return
		}
		var req history.GatherRequest
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			fail(w, 400, "Invalid gathering request")
			return
		}
		d, result, err := s.projects.GatherArchive(r.Context(), r.PathValue("world"), req)
		if err != nil {
			fail(w, 409, err.Error())
			return
		}
		respond(w, 200, map[string]any{"document": d, "result": result})
	})
	s.mux.HandleFunc("POST /api/worlds/{world}/archives/custody", func(w http.ResponseWriter, r *http.Request) {
		if !s.historyStore(w) {
			return
		}
		var req storage.ArchiveCustodyRequest
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			fail(w, 400, "Invalid custody request")
			return
		}
		d, err := s.projects.ArchiveCustody(r.Context(), r.PathValue("world"), req)
		if err != nil {
			fail(w, 409, err.Error())
			return
		}
		respond(w, 200, d)
	})
	s.mux.HandleFunc("POST /api/worlds/{world}/library", func(w http.ResponseWriter, r *http.Request) {
		if !s.historyStore(w) {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, history.MaxLibraryFileSize+(2<<20))
		if err := r.ParseMultipartForm(2 << 20); err != nil {
			fail(w, 400, "Invalid upload or file exceeds 64 MiB")
			return
		}
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		var meta struct {
			Revision  int            `json:"revision"`
			Entity    history.Entity `json:"entity"`
			ArchiveID string         `json:"archiveId"`
		}
		if err := json.Unmarshal([]byte(r.FormValue("metadata")), &meta); err != nil {
			fail(w, 400, "Invalid document metadata")
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			fail(w, 400, "A document file is required")
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, history.MaxLibraryFileSize+1))
		if err != nil || len(data) > history.MaxLibraryFileSize {
			fail(w, 413, "File exceeds 64 MiB")
			return
		}
		name := filepath.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
		kind := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
		if kind == "" {
			kind = "application/octet-stream"
		}
		if meta.Entity.ID == "" {
			meta.Entity.ID = history.ID()
		}
		d, err := s.projects.SaveLibraryFile(r.Context(), r.PathValue("world"), meta.Revision, meta.Entity, meta.ArchiveID, name, kind, data)
		if err != nil {
			fail(w, 409, err.Error())
			return
		}
		respond(w, 200, d)
	})
	s.mux.HandleFunc("GET /api/worlds/{world}/library/{document}", func(w http.ResponseWriter, r *http.Request) {
		if !s.historyStore(w) {
			return
		}
		data, v, err := s.projects.LibraryContent(r.Context(), r.PathValue("world"), r.URL.Query().Get("age"), r.PathValue("document"), r.URL.Query().Get("version"))
		if err != nil {
			fail(w, 404, "Document version not found")
			return
		}
		w.Header().Set("Content-Type", v.MIME)
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": v.Name}))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
		w.Header().Set("Cache-Control", "private, no-store")
		w.Write(data)
	})
}
