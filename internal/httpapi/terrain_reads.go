package httpapi

import (
	"net/http"
	"strings"
)

func isTerrainRead(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(p) >= 6 && p[0] == "api" && p[1] == "worlds" && p[3] == "settlements" && (p[5] == "resources" || p[5] == "resource-archives") {
		return true
	}
	if len(p) >= 4 && p[0] == "api" && p[1] == "worlds" && (p[3] == "resource-surveys" || len(p) == 5 && p[3] == "library") {
		return true
	}
	if len(p) < 5 || p[0] != "api" || p[1] != "sessions" || p[3] != "terrain" {
		return false
	}
	switch p[4] {
	case "sample", "mesh", "features", "resources", "resource-archives":
		return len(p) == 5
	case "tiles":
		return len(p) == 8
	}
	return false
}

// Terrain handlers capture immutable terrain and authored operations under their
// session lock. Sampling/rendering then runs without blocking unrelated maps or
// inspections. Reauthorize the buffered response before disclosing its data.
func (s *Server) serveTerrainRead(w http.ResponseWriter, r *http.Request) {
	s.accessMu.Lock()
	_, _, err := s.auth.User(r)
	allowed := false
	if err != nil {
		fail(w, 401, "Sign in to continue")
	} else {
		allowed = s.authorize(w, r)
	}
	s.accessMu.Unlock()
	if !allowed {
		return
	}
	response := &bufferedResponse{header: make(http.Header)}
	s.mux.ServeHTTP(response, r)
	if r.Context().Err() != nil {
		return
	}
	s.accessMu.Lock()
	if _, _, err = s.auth.User(r); err != nil {
		s.accessMu.Unlock()
		fail(w, 401, "Sign in to continue")
		return
	}
	allowed = s.authorize(w, r)
	s.accessMu.Unlock()
	if !allowed {
		return
	}
	for key, values := range response.Header() {
		w.Header()[key] = values
	}
	status := response.Code
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(response.Body.Bytes())
}
