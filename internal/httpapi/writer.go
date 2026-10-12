package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"kriemhild/internal/storage"
	"net/http"
)

func (s *Server) writerRoutes() {
	handle := func(pattern string, fn func(http.ResponseWriter, *http.Request) error) {
		s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			if !s.historyStore(w) {
				return
			}
			if err := fn(w, r); err != nil {
				code := 400
				if errors.Is(err, storage.ErrWritingConflict) {
					code = 409
				}
				if errors.Is(err, sql.ErrNoRows) {
					code = 404
				}
				fail(w, code, err.Error())
			}
		})
	}
	handle("GET /api/writer/projects", func(w http.ResponseWriter, r *http.Request) error {
		p, e := s.projects.WriterProjects(r.Context(), r.URL.Query().Get("world"))
		if s.auth != nil {
			visible := []storage.WritingProject{}
			for _, v := range p {
				if v.WorldID != "" && s.role(r, v.WorldID) != "" {
					visible = append(visible, v)
				}
			}
			p = visible
		}
		if e == nil {
			respond(w, 200, p)
		}
		return e
	})
	handle("POST /api/writer/projects", func(w http.ResponseWriter, r *http.Request) error {
		var p storage.WritingProject
		if e := decode(w, r, &p); e != nil {
			return e
		}
		e := s.projects.SaveWriterProject(r.Context(), &p)
		if e == nil {
			respond(w, 200, p)
		}
		return e
	})
	handle("GET /api/writer/projects/{project}", func(w http.ResponseWriter, r *http.Request) error {
		var b storage.WriterBundle
		var e error
		if r.URL.Query().Get("backup") != "" {
			b, e = s.projects.WriterBundle(r.Context(), r.PathValue("project"))
		} else {
			b, e = s.projects.WriterCurrent(r.Context(), r.PathValue("project"))
		}
		if e == nil {
			if r.URL.Query().Get("backup") == "" {
				b.Versions = nil
			}
			respond(w, 200, b)
		}
		return e
	})
	handle("POST /api/writer/projects/{project}/nodes", func(w http.ResponseWriter, r *http.Request) error {
		var body struct {
			Node  storage.WritingNode `json:"node"`
			Label string              `json:"label"`
		}
		if e := decode(w, r, &body); e != nil {
			return e
		}
		e := s.projects.SaveWriterNode(r.Context(), r.PathValue("project"), &body.Node, body.Label)
		if e == nil {
			respond(w, 200, body.Node)
		}
		return e
	})
	handle("GET /api/writer/projects/{project}/versions/{node}", func(w http.ResponseWriter, r *http.Request) error {
		b, e := s.projects.WriterBundle(r.Context(), r.PathValue("project"))
		if e != nil {
			return e
		}
		out := []storage.WritingVersion{}
		for _, v := range b.Versions {
			if v.Node.ID == r.PathValue("node") {
				out = append(out, v)
			}
		}
		respond(w, 200, out)
		return nil
	})
	handle("POST /api/writer/import", func(w http.ResponseWriter, r *http.Request) error {
		var b storage.WriterBundle
		r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
		if e := json.NewDecoder(r.Body).Decode(&b); e != nil {
			return e
		}
		p, e := s.projects.ImportWriter(r.Context(), b, r.URL.Query().Get("world"))
		if e == nil {
			respond(w, 201, p)
		}
		return e
	})
}
