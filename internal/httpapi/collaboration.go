package httpapi

import (
	"context"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"kriemhild/internal/auth"
	"kriemhild/internal/storage"
	"net/http"
	"time"
)

type liveClient struct {
	invalid                            bool
	conn                               *websocket.Conn
	world, node, user, name, selection string
	send                               chan any
}

func (s *Server) disconnectWorld(world, user string) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()
	for c := range s.live {
		if c.world == world && (user == "" || c.user == user) {
			c.conn.CloseNow()
		}
	}
}
func (s *Server) broadcast(world, node string, message any) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()
	for c := range s.live {
		if c.world == world && (node == "" || c.node == node) {
			if m, ok := message.(map[string]string); ok && m["type"] == "reset" {
				c.invalid = true
			}
			select {
			case c.send <- message:
			default:
				c.conn.CloseNow()
			}
		}
	}
}
func (s *Server) presence(world string) {
	s.liveMu.Lock()
	members := []map[string]string{}
	for c := range s.live {
		if c.world == world {
			members = append(members, map[string]string{"userId": c.user, "name": c.name, "nodeId": c.node, "selection": c.selection})
		}
	}
	s.liveMu.Unlock()
	s.broadcast(world, "", map[string]any{"type": "presence", "members": members})
}
func (s *Server) collaborationRoutes() {
	s.mux.HandleFunc("POST /api/writer/projects/{project}/recovery/{node}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Version  int `json:"version"`
			Revision int `json:"revision"`
		}
		if decode(w, r, &req) != nil {
			fail(w, 400, "Select a revision")
			return
		}
		n, e := s.projects.RestoreManuscript(r.Context(), r.PathValue("project"), r.PathValue("node"), req.Version, req.Revision)
		if e != nil {
			fail(w, 409, e.Error())
			return
		}
		world, _ := s.projects.WriterWorld(r.Context(), r.PathValue("project"))
		s.broadcast(world, n.ID, map[string]string{"type": "reset"})
		respond(w, 200, n)
	})
	s.mux.HandleFunc("GET /api/collaboration/worlds/{world}", s.collaborate)
	s.mux.HandleFunc("GET /api/writer/projects/{project}/comments/{node}", func(w http.ResponseWriter, r *http.Request) {
		if _, e := s.projects.WriterNode(r.Context(), r.PathValue("project"), r.PathValue("node")); e != nil {
			fail(w, 404, "Scene not found")
			return
		}
		v, e := s.projects.Comments(r.Context(), r.PathValue("node"))
		if e != nil {
			fail(w, 503, "Comments unavailable")
			return
		}
		respond(w, 200, v)
	})
	s.mux.HandleFunc("POST /api/writer/projects/{project}/comments/{node}", func(w http.ResponseWriter, r *http.Request) {
		if _, e := s.projects.WriterNode(r.Context(), r.PathValue("project"), r.PathValue("node")); e != nil {
			fail(w, 404, "Scene not found")
			return
		}
		world, e := s.projects.WriterWorld(r.Context(), r.PathValue("project"))
		if e != nil {
			fail(w, 404, "Project not found")
			return
		}
		var c storage.Comment
		if decode(w, r, &c) != nil {
			fail(w, 400, "Invalid comment")
			return
		}
		if e = s.projects.SaveComment(r.Context(), world, r.PathValue("node"), &c); e != nil {
			fail(w, 403, e.Error())
			return
		}
		s.broadcast(world, r.PathValue("node"), map[string]string{"type": "comments"})
		respond(w, 200, c)
	})
}
func (s *Server) collaborate(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		fail(w, 503, "Configure application identity to enable collaboration")
		return
	}
	world, project, node := r.PathValue("world"), r.URL.Query().Get("project"), r.URL.Query().Get("node")
	s.accessMu.Lock()
	if !s.allowed(w, r, world, "view") {
		s.accessMu.Unlock()
		return
	}
	if node != "" {
		parent, e := s.projects.WriterWorld(r.Context(), project)
		if e != nil || parent != world {
			fail(w, 404, "Scene not found")
			s.accessMu.Unlock()
			return
		}
		if _, e = s.projects.WriterNode(r.Context(), project, node); e != nil {
			fail(w, 404, "Scene not found")
			s.accessMu.Unlock()
			return
		}
	}
	u := auth.Identity(r.Context())
	s.liveMu.Lock()
	count := 0
	for c := range s.live {
		if c.user == u.ID {
			count++
		}
	}
	s.liveMu.Unlock()
	if count >= 12 {
		fail(w, 429, "Too many live connections")
		s.accessMu.Unlock()
		return
	}
	conn, e := websocket.Accept(w, r, nil)
	if e != nil {
		s.accessMu.Unlock()
		return
	}
	conn.SetReadLimit(3 << 20)
	c := &liveClient{conn: conn, world: world, node: node, user: u.ID, name: u.Name, send: make(chan any, 64)}
	s.liveMu.Lock()
	s.live[c] = true
	s.liveMu.Unlock()
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	defer conn.CloseNow()
	defer func() { s.liveMu.Lock(); delete(s.live, c); s.liveMu.Unlock(); s.presence(world) }()
	if node != "" {
		updates, e := s.projects.DocumentUpdates(ctx, node)
		if e != nil {
			s.accessMu.Unlock()
			return
		}
		n, e := s.projects.WriterNode(ctx, project, node)
		if e != nil {
			s.accessMu.Unlock()
			return
		}
		c.send <- map[string]any{"type": "sync", "updates": updates, "node": n, "role": s.role(r, world)}
	}
	s.accessMu.Unlock()
	s.presence(world)
	go func() {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case message := <-c.send:
				writeCtx, stop := context.WithTimeout(ctx, 10*time.Second)
				e := wsjson.Write(writeCtx, conn, message)
				stop()
				if e != nil {
					return
				}
			}
		}
	}()
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		last := -1
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.accessMu.Lock()
				_, _, err := s.auth.User(r)
				role := s.role(r, world)
				if err != nil || role == "" {
					s.accessMu.Unlock()
					conn.CloseNow()
					cancel()
					return
				}
				revision, err := s.projects.CurrentRevision(ctx, world)
				s.accessMu.Unlock()
				if err != nil {
					conn.CloseNow()
					cancel()
					return
				}
				if revision != last {
					last = revision
					select {
					case c.send <- map[string]any{"type": "revision", "revision": last}:
					default:
						conn.CloseNow()
						return
					}
				}
			}
		}
	}()
	for {
		var message struct {
			Type      string `json:"type"`
			Data      []byte `json:"data"`
			ID        string `json:"id"`
			Revision  int    `json:"revision"`
			Selection string `json:"selection"`
		}
		if e = wsjson.Read(ctx, conn, &message); e != nil {
			return
		}
		s.accessMu.Lock()
		s.liveMu.Lock()
		invalid := c.invalid
		s.liveMu.Unlock()
		if invalid {
			s.accessMu.Unlock()
			return
		}
		_, _, err := s.auth.User(r)
		role := s.role(r, world)
		if err != nil || role == "" {
			s.accessMu.Unlock()
			return
		}
		if len(message.ID) > 128 || !s.limits.allow("live:"+u.ID, 2400, time.Minute, time.Now()) {
			s.accessMu.Unlock()
			conn.Close(websocket.StatusPolicyViolation, "Collaboration rate or message limit exceeded")
			return
		}
		switch message.Type {
		case "selection":
			if len(message.Selection) < 500 {
				s.liveMu.Lock()
				c.selection = message.Selection
				s.liveMu.Unlock()
				s.presence(world)
			}
		case "update", "seed":
			if !storage.CanEdit(role) || node == "" {
				s.accessMu.Unlock()
				conn.Close(websocket.StatusPolicyViolation, "Editing permission required")
				return
			}
			if _, e = s.projects.WriterNode(ctx, project, node); e != nil {
				s.accessMu.Unlock()
				return
			}
			seed := 0
			if message.Type == "seed" {
				seed = message.Revision
				if seed < 1 {
					s.accessMu.Unlock()
					return
				}
			}
			id, err := s.projects.AppendDocumentUpdate(ctx, node, message.Data, seed)
			if err != nil {
				select {
				case c.send <- map[string]string{"type": "error", "message": err.Error()}:
				default:
				}
				s.accessMu.Unlock()
				continue
			}
			s.broadcast(world, node, map[string]any{"type": "update", "id": id, "data": message.Data, "authorId": u.ID})
			select {
			case c.send <- map[string]string{"type": "ack", "id": message.ID}:
			default:
			}
		default:
			s.accessMu.Unlock()
			conn.Close(websocket.StatusPolicyViolation, "Unknown message")
			return
		}
		s.accessMu.Unlock()
	}
}

// Keep serialization expectations explicit: document updates are opaque Yjs
// binary deltas, never arbitrary commands or client-supplied identities.
