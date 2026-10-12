package httpapi

import (
	"kriemhild/internal/storage"
	"net/http"
)

func (s *Server) writingRoutes() {
	s.writerRoutes()
	s.mux.HandleFunc("GET /api/writing", func(w http.ResponseWriter, r *http.Request) {
		if !s.historyStore(w) {
			return
		}
		list, err := s.projects.Writing(r.Context())
		if err != nil {
			fail(w, 503, err.Error())
			return
		}
		respond(w, 200, list)
	})
	s.mux.HandleFunc("POST /api/writing", func(w http.ResponseWriter, r *http.Request) {
		if !s.historyStore(w) {
			return
		}
		var d storage.WritingDocument
		if err := decode(w, r, &d); err != nil {
			fail(w, 400, err.Error())
			return
		}
		if err := s.projects.SaveWriting(r.Context(), &d); err != nil {
			fail(w, 409, err.Error())
			return
		}
		respond(w, 200, d)
	})
	s.mux.HandleFunc("DELETE /api/writing/{document}", func(w http.ResponseWriter, r *http.Request) {
		if !s.historyStore(w) {
			return
		}
		if err := s.projects.DeleteWriting(r.Context(), r.PathValue("document")); err != nil {
			fail(w, 503, err.Error())
			return
		}
		respond(w, 200, map[string]bool{"deleted": true})
	})
}
