package httpapi

import (
	"errors"
	"kriemhild/internal/auth"
	"kriemhild/internal/storage"
	"net/http"
	"strconv"
)

func (s *Server) disconnectUser(user string) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()
	for c := range s.live {
		if c.user == user {
			c.conn.CloseNow()
		}
	}
}
func (s *Server) accountRoutes() {
	s.mux.HandleFunc("GET /api/account", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.projects.Account(r.Context())
		if e != nil {
			fail(w, 503, "Account unavailable")
			return
		}
		respond(w, 200, v)
	})
	s.mux.HandleFunc("POST /api/account/sessions/revoke", func(w http.ResponseWriter, r *http.Request) {
		if s.auth.Config.Mode == "local" {
			fail(w, 400, "Local mode does not use login sessions")
			return
		}
		if e := s.projects.RevokeUserSessions(r.Context()); e != nil {
			fail(w, 503, "Unable to revoke sessions")
			return
		}
		s.disconnectUser(storage.Actor(r.Context()))
		w.WriteHeader(204)
	})
	s.mux.HandleFunc("DELETE /api/account", func(w http.ResponseWriter, r *http.Request) {
		if s.auth.Config.Mode == "local" {
			fail(w, 403, "The local workspace cannot be closed")
			return
		}
		var req struct {
			Confirmation string `json:"confirmation"`
		}
		if decode(w, r, &req) != nil || req.Confirmation != "CLOSE" {
			fail(w, 400, "Type CLOSE to confirm account closure")
			return
		}
		if e := s.projects.CloseAccount(r.Context()); e != nil {
			if errors.Is(e, storage.ErrOwnsWorlds) {
				fail(w, 409, e.Error())
			} else {
				fail(w, 503, "Unable to close account")
			}
			return
		}
		s.disconnectUser(storage.Actor(r.Context()))
		w.WriteHeader(204)
	})
	s.mux.HandleFunc("GET /api/worlds/{world}/invitations", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.projects.WorldInvitations(r.Context(), r.PathValue("world"))
		if e != nil {
			fail(w, 503, "Invitations unavailable")
			return
		}
		respond(w, 200, v)
	})
	s.mux.HandleFunc("DELETE /api/worlds/{world}/invitations/{invitation}", func(w http.ResponseWriter, r *http.Request) {
		if e := s.projects.RevokeInvitation(r.Context(), r.PathValue("world"), r.PathValue("invitation")); e != nil {
			fail(w, 403, "Cannot cancel this invitation")
			return
		}
		w.WriteHeader(204)
	})
	s.mux.HandleFunc("DELETE /api/invitations/{invitation}", func(w http.ResponseWriter, r *http.Request) {
		if e := s.projects.DeclineInvitation(r.Context(), auth.Identity(r.Context()), r.PathValue("invitation")); e != nil {
			fail(w, 404, "Invitation unavailable")
			return
		}
		w.WriteHeader(204)
	})
	s.mux.HandleFunc("GET /api/worlds/{world}/activity", func(w http.ResponseWriter, r *http.Request) {
		before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
		v, e := s.projects.AuditEvents(r.Context(), r.PathValue("world"), before)
		if e != nil {
			fail(w, 503, "Activity unavailable")
			return
		}
		respond(w, 200, v)
	})
	s.mux.HandleFunc("POST /api/worlds/{world}/visit", func(w http.ResponseWriter, r *http.Request) {
		if e := s.projects.VisitWorld(r.Context(), r.PathValue("world")); e != nil {
			fail(w, 503, "Unable to remember this world")
			return
		}
		w.WriteHeader(204)
	})
}
