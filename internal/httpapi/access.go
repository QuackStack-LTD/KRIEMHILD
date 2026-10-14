package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"kriemhild/internal/auth"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"net/http"
	"strconv"
	"strings"
)

type bufferedResponse struct {
	header http.Header
	Body   bytes.Buffer
	Code   int
}

func (b *bufferedResponse) Header() http.Header { return b.header }
func (b *bufferedResponse) WriteHeader(code int) {
	if b.Code == 0 {
		b.Code = code
	}
}
func (b *bufferedResponse) Write(p []byte) (int, error) {
	if b.Code == 0 {
		b.Code = 200
	}
	return b.Body.Write(p)
}

func (s *Server) role(r *http.Request, id string) string {
	if s.auth == nil {
		return "owner"
	}
	role, _ := s.projects.Role(r.Context(), id, storage.Actor(r.Context()))
	return role
}
func (s *Server) allowed(w http.ResponseWriter, r *http.Request, id, permission string) bool {
	role := s.role(r, id)
	if role == "" {
		fail(w, 404, "Resource not found")
		return false
	}
	if permission == "edit" && !storage.CanEdit(role) || permission == "manage" && !storage.CanManage(role) || permission == "owner" && role != "owner" || permission == "comment" && role == "viewer" {
		fail(w, 403, "Your world role does not permit this action")
		return false
	}
	return true
}
func mutating(r *http.Request) bool {
	return r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS"
}
func peekJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 4<<20))
	if e != nil {
		fail(w, 413, "Request is too large")
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(b))
	if json.Unmarshal(b, v) != nil {
		fail(w, 400, "Invalid JSON")
		return false
	}
	return true
}

// Central default-deny route policy. Payload-selected resources are checked in
// addition to route-selected parents, so changing an ID cannot escape a world.
func (s *Server) authorize(w http.ResponseWriter, r *http.Request) bool {
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	permission := "view"
	if mutating(r) {
		permission = "edit"
	}
	if len(p) < 2 {
		return false
	}
	switch p[1] {
	case "me", "account", "workspaces", "invitations", "defaults", "biosphere", "terrain":
		return true
	case "worlds":
		if len(p) == 2 || len(p) == 3 && p[2] == "import" {
			return true
		}
		if len(p) == 3 && r.Method == "DELETE" {
			permission = "owner"
		}
		if len(p) > 3 && (p[3] == "invitations" || p[3] == "activity") {
			permission = "manage"
		}
		if len(p) > 3 && p[3] == "visit" {
			permission = "view"
		}
		if len(p) > 3 && p[3] == "members" {
			if mutating(r) {
				permission = "manage"
			}
		}
		if len(p) > 3 && p[3] == "ownership" {
			permission = "owner"
		}
		if !s.allowed(w, r, p[2], permission) {
			return false
		}
		if len(p) > 3 && p[3] == "accept" {
			var body struct {
				Session string `json:"session"`
			}
			if !peekJSON(w, r, &body) {
				return false
			}
			return s.authorizeSession(w, r, body.Session, false)
		}
		return true
	case "projects":
		if len(p) == 2 || len(p) == 3 && p[2] == "import" {
			return true
		}
		if r.Method == "DELETE" {
			permission = "owner"
		}
		if len(p) > 3 && p[3] == "open" {
			permission = "view"
		}
		return s.allowed(w, r, p[2], permission)
	case "map-context":
		if len(p) == 3 {
			return s.allowed(w, r, p[2], "view")
		}
	case "sessions":
		if len(p) == 2 {
			return true
		}
		return s.authorizeSession(w, r, p[2], true)
	case "writer":
		if len(p) >= 3 && p[2] == "legacy" {
			u := auth.Identity(r.Context())
			if s.auth.Config.Mode != "local" && (s.auth.Config.LegacySubject == "" || u.Subject != s.auth.Config.LegacySubject) {
				fail(w, 404, "Resource not found")
				return false
			}
			if mutating(r) {
				var req struct {
					WorldID string `json:"worldId"`
				}
				if !peekJSON(w, r, &req) {
					return false
				}
				return s.allowed(w, r, req.WorldID, "edit")
			}
			return true
		}
		if len(p) == 3 && p[2] == "import" {
			id := r.URL.Query().Get("world")
			if id == "" {
				fail(w, 400, "Choose a destination world")
				return false
			}
			return s.allowed(w, r, id, "edit")
		}
		if len(p) >= 3 && p[2] == "projects" {
			if len(p) == 3 {
				if r.Method == "GET" {
					id := r.URL.Query().Get("world")
					return id == "" || s.allowed(w, r, id, "view")
				}
				var body storage.WritingProject
				if !peekJSON(w, r, &body) {
					return false
				}
				if body.WorldID == "" {
					fail(w, 400, "Writing projects must belong to a world")
					return false
				}
				if old, e := s.projects.WriterWorld(r.Context(), body.ID); e == nil && !s.allowed(w, r, old, "edit") {
					return false
				}
				return s.allowed(w, r, body.WorldID, "edit")
			}
			id, e := s.projects.WriterWorld(r.Context(), p[3])
			if e != nil || id == "" {
				fail(w, 404, "Writing project not found; legacy drafts require reassignment")
				return false
			}
			if len(p) > 5 && p[4] == "comments" && mutating(r) {
				permission = "comment"
			}
			return s.allowed(w, r, id, permission)
		}
	case "writing": // Legacy unscoped drafts are retained for migration, never exposed in hosted mode.
		if s.auth.Config.Mode == "local" {
			return true
		}
		fail(w, 410, "Use world-scoped writing projects")
		return false
	case "collaboration":
		if len(p) == 4 && p[2] == "worlds" {
			return s.allowed(w, r, p[3], "view")
		}
	}
	fail(w, 404, "API route not found")
	return false
}
func (s *Server) authorizeSession(w http.ResponseWriter, r *http.Request, id string, route bool) bool {
	s.mu.Lock()
	v := s.sessions[id]
	s.mu.Unlock()
	if v == nil {
		fail(w, 404, "Map session not found")
		return false
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	permission := "view"
	if mutating(r) {
		permission = "edit"
	}
	if v.historyWorld != "" || v.persisted {
		if !s.allowed(w, r, v.worldID, permission) {
			return false
		}
	} else if v.creatorID != storage.Actor(r.Context()) {
		fail(w, 404, "Map session not found")
		return false
	}
	if mutating(r) {
		v.actorID = storage.Actor(r.Context())
	}
	if v.historyWorld != "" {
		revision, e := s.projects.CurrentRevision(r.Context(), v.historyWorld)
		if e != nil {
			fail(w, 404, "World not found")
			return false
		}
		if revision != v.historyRevision {
			if mutating(r) {
				fail(w, 409, "World history changed; reopen the map before editing")
				return false
			}
			if r.URL.Path == "/api/sessions/"+id+"/world/state" {
				if e = s.applyHistoricalMap(r.Context(), v); e != nil {
					fail(w, 409, e.Error())
					return false
				}
				v.collabRevision++
			}
		}
	}
	if route && mutating(r) && r.Method != "DELETE" && (s.auth.Config.Mode == "oidc" || r.Header.Get("If-Match") != "") {
		expected, e := strconv.Atoi(r.Header.Get("If-Match"))
		if e != nil {
			fail(w, 428, "A map revision is required; reopen the map")
			return false
		}
		if expected != v.collabRevision {
			w.Header().Set("X-Map-Revision", strconv.Itoa(v.collabRevision))
			fail(w, 409, "Map changed in another session; reload before retrying")
			return false
		}
	}
	w.Header().Set("X-Map-Revision", strconv.Itoa(v.collabRevision))
	return true
}
func (s *Server) secureServe(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/auth/") {
		if !s.limitRequest(w, r, "") {
			return
		}
		s.auth.ServeHTTP(w, r)
		return
	}
	u, csrf, e := s.auth.User(r)
	if e != nil {
		fail(w, 401, "Sign in to continue")
		return
	}
	if !s.limitRequest(w, r, u.ID) {
		return
	}
	r = r.WithContext(auth.Context(r.Context(), u))
	if mutating(r) && !s.auth.ValidCSRF(r, csrf) {
		fail(w, 403, "Invalid CSRF token")
		return
	}
	if r.URL.Path == "/api/me" && r.Method == "GET" {
		spaces, e := s.projects.Workspaces(r.Context(), u.ID)
		if e != nil {
			fail(w, 503, "Workspace unavailable")
			return
		}
		respond(w, 200, map[string]any{"user": u, "workspaces": spaces, "csrfToken": csrf, "mode": s.auth.Config.Mode})
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/collaboration/") {
		if s.authorize(w, r) {
			s.mux.ServeHTTP(w, r)
		}
		return
	}
	// The map engine has one authoritative cache. One server replica is enforced
	// operationally; SQL revisions additionally protect durable historical writes.
	s.accessMu.Lock()
	defer s.accessMu.Unlock()
	// A queued request may outlive session revocation or account closure.
	if _, _, err := s.auth.User(r); err != nil {
		fail(w, 401, "Sign in to continue")
		return
	}
	if !s.authorize(w, r) {
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/sessions/") && mutating(r) && r.Method != "DELETE" {
		recorder := &bufferedResponse{header: make(http.Header)}
		s.mux.ServeHTTP(recorder, r)
		if recorder.Code < 300 {
			p := strings.Split(r.URL.Path, "/")
			s.mu.Lock()
			v := s.sessions[p[3]]
			s.mu.Unlock()
			if v != nil {
				v.mu.Lock()
				v.collabRevision++
				if s.auth.Config.Mode == "oidc" && v.unsaved {
					e = s.saveWorld(v, true)
				}
				w.Header().Set("X-Map-Revision", strconv.Itoa(v.collabRevision))
				v.mu.Unlock()
				if e != nil {
					fail(w, 503, "Map persistence failed; reload before editing")
					return
				}
			}
		}
		for k, values := range recorder.Header() {
			w.Header()[k] = values
		}
		w.WriteHeader(recorder.Code)
		_, _ = w.Write(recorder.Body.Bytes())
		return
	}
	s.mux.ServeHTTP(w, r)
}
func (s *Server) accessRoutes() {
	s.mux.HandleFunc("GET /api/writer/legacy", func(w http.ResponseWriter, r *http.Request) {
		projects, e := s.projects.WriterProjects(r.Context(), "")
		if e != nil {
			fail(w, 503, "Legacy projects unavailable")
			return
		}
		out := []storage.WritingProject{}
		for _, p := range projects {
			if p.WorldID == "" {
				out = append(out, p)
			}
		}
		respond(w, 200, out)
	})
	s.mux.HandleFunc("POST /api/writer/legacy/{project}/assign", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			WorldID string `json:"worldId"`
		}
		if decode(w, r, &req) != nil {
			fail(w, 400, "Choose a world")
			return
		}
		b, e := s.projects.WriterCurrent(r.Context(), r.PathValue("project"))
		if e != nil || b.Project.WorldID != "" {
			fail(w, 404, "Unassigned project not found")
			return
		}
		b.Project.WorldID = req.WorldID
		if e = s.projects.SaveWriterProject(r.Context(), &b.Project); e != nil {
			fail(w, 409, e.Error())
			return
		}
		respond(w, 200, b.Project)
	})
	s.mux.HandleFunc("GET /api/workspaces", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.projects.Workspaces(r.Context(), storage.Actor(r.Context()))
		if e != nil {
			fail(w, 503, "Workspace unavailable")
			return
		}
		respond(w, 200, v)
	})
	s.mux.HandleFunc("GET /api/worlds/{world}/members", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.projects.Members(r.Context(), r.PathValue("world"))
		if e != nil {
			fail(w, 503, "Members unavailable")
			return
		}
		respond(w, 200, map[string]any{"members": v, "role": s.role(r, r.PathValue("world"))})
	})
	s.mux.HandleFunc("POST /api/worlds/{world}/invitations", func(w http.ResponseWriter, r *http.Request) {
		var v struct{ Email, Role string }
		if decode(w, r, &v) != nil {
			fail(w, 400, "Invalid invitation")
			return
		}
		secret, e := s.projects.Invite(r.Context(), r.PathValue("world"), v.Email, v.Role)
		if e != nil {
			fail(w, 400, e.Error())
			return
		}
		respond(w, 201, map[string]string{"token": secret})
	})
	s.mux.HandleFunc("GET /api/invitations", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.projects.Invitations(r.Context(), auth.Identity(r.Context()))
		if e != nil {
			fail(w, 503, "Invitations unavailable")
			return
		}
		respond(w, 200, v)
	})
	s.mux.HandleFunc("POST /api/invitations/accept", func(w http.ResponseWriter, r *http.Request) {
		var v struct{ Token string }
		if decode(w, r, &v) != nil {
			fail(w, 400, "Invalid invitation")
			return
		}
		id, e := s.projects.AcceptInvitation(r.Context(), auth.Identity(r.Context()), v.Token)
		if e != nil {
			fail(w, 400, e.Error())
			return
		}
		respond(w, 200, map[string]string{"worldId": id})
	})
	member := func(w http.ResponseWriter, r *http.Request) {
		var v struct{ Role string }
		if r.Method == "PATCH" && decode(w, r, &v) != nil {
			fail(w, 400, "Invalid role")
			return
		}
		if e := s.projects.SetMember(r.Context(), r.PathValue("world"), r.PathValue("user"), v.Role); e != nil {
			fail(w, 403, e.Error())
			return
		}
		s.disconnectWorld(r.PathValue("world"), r.PathValue("user"))
		w.WriteHeader(204)
	}
	s.mux.HandleFunc("PATCH /api/worlds/{world}/members/{user}", member)
	s.mux.HandleFunc("DELETE /api/worlds/{world}/members/{user}", member)
	s.mux.HandleFunc("POST /api/worlds/{world}/ownership", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			UserID string `json:"userId"`
		}
		if decode(w, r, &v) != nil {
			fail(w, 400, "Invalid member")
			return
		}
		if e := s.projects.TransferOwner(r.Context(), r.PathValue("world"), v.UserID); e != nil {
			fail(w, 403, e.Error())
			return
		}
		s.disconnectWorld(r.PathValue("world"), "")
		w.WriteHeader(204)
	})
	revisions := func(w http.ResponseWriter, r *http.Request) {
		after, _ := strconv.Atoi(r.URL.Query().Get("after"))
		before, _ := strconv.Atoi(r.URL.Query().Get("before"))
		v, e := s.projects.RevisionPage(r.Context(), r.PathValue("world"), after, before)
		if e != nil {
			fail(w, 503, "History unavailable")
			return
		}
		respond(w, 200, v)
	}
	s.mux.HandleFunc("GET /api/worlds/{world}/revisions", revisions)
	s.mux.HandleFunc("GET /api/worlds/{world}/events", revisions)
	s.mux.HandleFunc("POST /api/worlds/{world}/revisions/{revision}/restore", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Revision int `json:"revision"`
		}
		if decode(w, r, &req) != nil {
			fail(w, 400, "Expected revision required")
			return
		}
		rev, _ := strconv.Atoi(r.PathValue("revision"))
		raw, e := s.projects.RevisionSnapshot(r.Context(), r.PathValue("world"), rev)
		if e != nil {
			fail(w, 404, "Revision not found")
			return
		}
		var snapshot history.Document
		if json.Unmarshal([]byte(raw), &snapshot) != nil {
			fail(w, 422, "Invalid revision")
			return
		}
		d, e := s.projects.UpdateHistory(storage.WithOperation(r.Context(), "world-restored"), r.PathValue("world"), req.Revision, func(d *history.Document) error {
			if snapshot.World.ID != d.World.ID {
				return fmt.Errorf("wrong world")
			}
			*d = snapshot
			return nil
		})
		if e != nil {
			fail(w, 409, e.Error())
			return
		}
		s.retireHistorySessions(r.PathValue("world"))
		respond(w, 200, d)
	})
}
func (s *Server) retireHistorySessions(world string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.sessions {
		v.mu.Lock()
		if v.historyWorld == world {
			v.retired = true
		}
		v.mu.Unlock()
	}
}
