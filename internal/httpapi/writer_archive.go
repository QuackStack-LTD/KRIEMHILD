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

func (s *Server) writerArchiveRoutes() {
	s.mux.HandleFunc("GET /api/worlds/{world}/writer-archive/files/{hash}", func(w http.ResponseWriter, r *http.Request) {
		if !s.historyStore(w) {
			return
		}
		data, err := s.projects.WriterArchiveFile(r.Context(), r.PathValue("world"), r.PathValue("hash"))
		if err != nil {
			fail(w, 404, "Archive image not found")
			return
		}
		kind := http.DetectContentType(data)
		if kind == "text/xml; charset=utf-8" || kind == "text/plain; charset=utf-8" {
			head := string(data[:min(len(data), 1024)])
			if strings.Contains(head, "<svg") {
				kind = "image/svg+xml"
			}
		}
		w.Header().Set("Content-Type", kind)
		w.Header().Set("Content-Disposition", "attachment")
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
		w.Header().Set("Cache-Control", "private, max-age=3600")
		w.Write(data)
	})
	s.mux.HandleFunc("GET /api/worlds/{world}/writer-archive", func(w http.ResponseWriter, r *http.Request) {
		if !s.historyStore(w) {
			return
		}
		v, e := s.projects.WriterArchive(r.Context(), r.PathValue("world"))
		if e != nil {
			fail(w, 503, e.Error())
			return
		}
		respond(w, 200, v)
	})
	s.mux.HandleFunc("POST /api/worlds/{world}/writer-archive", func(w http.ResponseWriter, r *http.Request) {
		if !s.historyStore(w) {
			return
		}
		v, e := s.projects.EnsureWriterArchive(r.Context(), r.PathValue("world"))
		if e != nil {
			fail(w, 409, e.Error())
			return
		}
		respond(w, 200, v)
	})
	s.mux.HandleFunc("GET /api/worlds/{world}/writer-archive/inworld", func(w http.ResponseWriter, r *http.Request) {
		if !s.historyStore(w) {
			return
		}
		v, e := s.projects.InworldArchive(r.Context(), r.PathValue("world"))
		if e != nil {
			fail(w, 503, e.Error())
			return
		}
		respond(w, 200, v)
	})
	s.mux.HandleFunc("POST /api/worlds/{world}/writer-archive/actions", func(w http.ResponseWriter, r *http.Request) {
		if !s.historyStore(w) {
			return
		}
		var a storage.WriterArchiveAction
		if e := decode(w, r, &a); e != nil {
			fail(w, 400, e.Error())
			return
		}
		if e := s.projects.WriterArchiveAction(r.Context(), r.PathValue("world"), a); e != nil {
			fail(w, 409, e.Error())
			return
		}
		respond(w, 200, map[string]bool{"saved": true})
	})
	s.mux.HandleFunc("POST /api/writer/projects/{project}/files", func(w http.ResponseWriter, r *http.Request) {
		if !s.historyStore(w) {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, history.MaxLibraryFileSize+(4<<20))
		if e := r.ParseMultipartForm(2 << 20); e != nil {
			fail(w, 413, "Invalid upload or file exceeds 64 MiB")
			return
		}
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		var n storage.WritingNode
		if e := json.Unmarshal([]byte(r.FormValue("metadata")), &n); e != nil {
			fail(w, 400, "Invalid file metadata")
			return
		}
		file, h, e := r.FormFile("file")
		if e != nil {
			fail(w, 400, "A file is required")
			return
		}
		defer file.Close()
		data, e := io.ReadAll(io.LimitReader(file, history.MaxLibraryFileSize+1))
		if e != nil || len(data) > history.MaxLibraryFileSize {
			fail(w, 413, "File exceeds 64 MiB")
			return
		}
		name := filepath.Base(strings.ReplaceAll(h.Filename, "\\", "/"))
		kind := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
		if kind == "" {
			kind = "application/octet-stream"
		}
		if e = s.projects.SaveWriterFile(r.Context(), r.PathValue("project"), &n, name, kind, data); e != nil {
			fail(w, 409, e.Error())
			return
		}
		respond(w, 200, n)
	})
	s.mux.HandleFunc("GET /api/writer/projects/{project}/files/{node}", func(w http.ResponseWriter, r *http.Request) {
		if !s.historyStore(w) {
			return
		}
		data, n, e := s.projects.WriterFile(r.Context(), r.PathValue("project"), r.PathValue("node"), r.URL.Query().Get("version"))
		if e != nil {
			fail(w, 404, "File version not found")
			return
		}
		w.Header().Set("Content-Type", n.MIME)
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": n.FileName}))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
		w.Header().Set("Cache-Control", "private, no-store")
		w.Write(data)
	})
}
