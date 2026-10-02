package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"kriemhild/internal/project"
)

type Server struct {
	access       *Access
	importOwners map[string]string
	Data         string
	Web          string
	mu           sync.Mutex
	stores       map[string]*project.Store
	token        string
	imports      map[string]project.ImportPreview
}

func New(data, web string) (*Server, error) {
	abs, e := filepath.Abs(data)
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(abs, 0700); e != nil {
		return nil, e
	}
	abs, e = filepath.EvalSymlinks(abs)
	if e != nil {
		return nil, e
	}
	return &Server{Data: abs, Web: web, stores: map[string]*project.Store{}, imports: map[string]project.ImportPreview{}, importOwners: map[string]string{}, token: project.NewID() + project.NewID()}, nil
}
func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, store := range s.stores {
		store.Close()
	}
	if s.access != nil && s.access.lock != nil {
		s.access.lock.Unlock()
	}
}
func send(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func failure(w http.ResponseWriter, e error) {
	status := 400
	if errors.Is(e, project.ErrConflict) {
		status = 409
	}
	send(w, status, map[string]string{"error": e.Error()})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if e := d.Decode(&struct{}{}); e != io.EOF {
		return fmt.Errorf("expected one JSON value")
	}
	return nil
}
func localHost(host string) bool {
	h, _, e := net.SplitHostPort(host)
	if e != nil {
		h = host
	}
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	if (s.access == nil && !localHost(r.Host)) || (s.access != nil && r.Host != s.access.origin.Host) {
		http.Error(w, "Local connections only", 403)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, e := url.Parse(origin)
		if e != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") || (s.access != nil && origin != s.access.origin.String()) {
			http.Error(w, "Origin rejected", 403)
			return
		}
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "Cross-site request rejected", 403)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		http.FileServer(http.Dir(s.Web)).ServeHTTP(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	_, port, _ := net.SplitHostPort(r.Host)
	cookieName := "kriemhild-p2-session-" + port
	user, handled := s.authentication(w, r, cookieName)
	if handled {
		return
	}
	if r.URL.Path == "/api/v1/session" && r.Method == "GET" {
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: s.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		send(w, 200, map[string]string{"library": s.Data, "version": "0.2.0"})
		return
	}
	cookie, e := r.Cookie(cookieName)
	if s.access == nil && (e != nil || cookie.Value != s.token) {
		send(w, 401, map[string]string{"error": "Open KRIEMHILD again to start a local session."})
		return
	}
	if strings.HasPrefix(r.Method, "P") && r.Method != "PUT" && r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	if r.URL.Path == "/api/v1/projects" {
		switch r.Method {
		case "GET":
			items := []project.Marker{}
			entries, e := os.ReadDir(s.Data)
			if e != nil {
				failure(w, e)
				return
			}
			for _, entry := range entries {
				if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
					continue
				}
				b, e := os.ReadFile(filepath.Join(s.Data, entry.Name(), "kriemhild.json"))
				if e != nil {
					continue
				}
				var m project.Marker
				if json.Unmarshal(b, &m) == nil && m.ID == entry.Name() && (s.access == nil || s.access.role(user, m.ID) != "") {
					items = append(items, m)
				}
			}
			sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
			send(w, 200, items)
		case "POST":
			var input struct {
				Name string `json:"name"`
				Age  string `json:"age"`
			}
			if e := decode(w, r, &input); e != nil {
				failure(w, e)
				return
			}
			id := project.NewID()
			if s.access != nil {
				if e := s.access.grant(id, user, "owner"); e != nil {
					failure(w, e)
					return
				}
			}
			store, age, e := project.Create(filepath.Join(s.Data, id), input.Name, input.Age)
			if e != nil {
				failure(w, e)
				return
			}
			s.mu.Lock()
			s.stores[id] = store
			s.mu.Unlock()
			state, e := store.State(age)
			if e != nil {
				failure(w, e)
				return
			}
			state.Role = "owner"
			send(w, 201, state)
		default:
			http.Error(w, "Method not allowed", 405)
		}
		return
	}
	if r.URL.Path == "/api/v1/clone-preview" && r.Method == "POST" {
		if s.access != nil && user != "admin" {
			send(w, 403, map[string]string{"error": "Administrator access required to clone a repository into this hosted library"})
			return
		}
		var input struct {
			Remote string `json:"remote"`
		}
		if e := decode(w, r, &input); e != nil {
			failure(w, e)
			return
		}
		preview, e := project.PrepareClone(s.Data, input.Remote)
		if e != nil {
			failure(w, e)
			return
		}
		s.mu.Lock()
		s.imports[preview.Token] = preview
		s.importOwners[preview.Token] = user
		s.mu.Unlock()
		send(w, 200, preview)
		return
	}
	if r.URL.Path == "/api/v1/import-preview" && r.Method == "POST" {
		r.Body = http.MaxBytesReader(w, r.Body, project.ArchiveLimit)
		file, e := os.CreateTemp("", "kriemhild-import-*.zip")
		if e != nil {
			failure(w, e)
			return
		}
		defer func() { file.Close(); os.Remove(file.Name()) }()
		size, e := io.Copy(file, r.Body)
		if e != nil {
			failure(w, e)
			return
		}
		preview, e := project.PrepareImportReader(s.Data, file, size)
		if e != nil {
			failure(w, e)
			return
		}
		s.mu.Lock()
		s.imports[preview.Token] = preview
		s.importOwners[preview.Token] = user
		s.mu.Unlock()
		send(w, 200, preview)
		return
	}
	if (r.URL.Path == "/api/v1/import-accept" || r.URL.Path == "/api/v1/import-discard") && r.Method == "POST" {
		var c struct {
			Token string `json:"token"`
		}
		if e := decode(w, r, &c); e != nil {
			failure(w, e)
			return
		}
		s.mu.Lock()
		p, ok := s.imports[c.Token]
		if s.importOwners[c.Token] != user {
			ok = false
		}
		defer s.mu.Unlock()
		if !ok {
			failure(w, fmt.Errorf("import preview expired; preview again"))
			return
		}
		if r.URL.Path == "/api/v1/import-discard" {
			if e := project.DiscardImport(s.Data, p); e != nil {
				failure(w, e)
				return
			}
			delete(s.imports, c.Token)
			delete(s.importOwners, c.Token)
			send(w, 200, map[string]bool{"discarded": true})
			return
		}
		if e := project.AcceptImport(s.Data, p); e != nil {
			failure(w, e)
			return
		}
		if s.access != nil {
			if e := s.access.grant(p.World.ID, user, "owner"); e != nil {
				failure(w, e)
				return
			}
		}
		delete(s.imports, c.Token)
		delete(s.importOwners, c.Token)
		// The accepted world now lives outside the staging directory.
		_ = project.DiscardImport(s.Data, p)
		send(w, 201, p.World)
		return
	}
	if r.URL.Path == "/api/v1/domains" && r.Method == "GET" {
		send(w, 200, project.DomainCatalog())
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "projects" {
		http.NotFound(w, r)
		return
	}
	role := "owner"
	if s.access != nil {
		role = s.access.role(user, parts[3])
		if role == "" {
			send(w, 404, map[string]string{"error": "World not found"})
			return
		}
		ownerOnly := parts[4] == "repository" || parts[4] == "merge" || parts[4] == "members" || parts[4] == "restore" || parts[4] == "revisions"
		readPost := parts[4] == "export" || parts[4] == "experiment" || parts[4] == "terrain-preview" || parts[4] == "content-preview" || parts[4] == "graph"
		if ownerOnly && role != "owner" || r.Method != "GET" && !readPost && role == "viewer" {
			send(w, 403, map[string]string{"error": "Your role does not permit this action"})
			return
		}
	}
	store, e := s.store(parts[3])
	if e != nil {
		failure(w, e)
		return
	}
	age := r.URL.Query().Get("age")
	switch parts[4] {
	case "revisions":
		if r.Method != "GET" {
			break
		}
		out, e := store.Revisions(r.URL.Query().Get("cursor"))
		if e != nil {
			failure(w, e)
			return
		}
		send(w, 200, out)
		return
	case "restore":
		if r.Method != "POST" {
			break
		}
		var input project.RestoreRequest
		if e := decode(w, r, &input); e != nil {
			failure(w, e)
			return
		}
		out, e := store.RestoreRevision(input)
		if e != nil {
			failure(w, e)
			return
		}
		send(w, 200, out)
		return
	case "content-preview":
		if r.Method != "POST" {
			break
		}
		var c project.ContentImportRequest
		if e := decode(w, r, &c); e != nil {
			failure(w, e)
			return
		}
		out, e := store.PreviewContent(c)
		if e != nil {
			failure(w, e)
			return
		}
		send(w, 200, out)
		return
	case "graph":
		if r.Method != "POST" {
			break
		}
		var c project.GraphRequest
		if e := decode(w, r, &c); e != nil {
			failure(w, e)
			return
		}
		out, e := store.RelationshipGraph(c)
		if e != nil {
			failure(w, e)
			return
		}
		send(w, 200, out)
		return
	case "live":
		var c project.LiveRequest
		if r.Method == "GET" {
			c.Age = age
			c.Scene = r.URL.Query().Get("scene")
			c.Client = r.URL.Query().Get("client")
			fmt.Sscan(r.URL.Query().Get("after"), &c.After)
		} else if r.Method == "POST" {
			if e := decode(w, r, &c); e != nil {
				failure(w, e)
				return
			}
		} else {
			break
		}
		out, e := store.Live(c, user)
		if e != nil {
			failure(w, e)
			return
		}
		send(w, 200, out)
		return
	case "members":
		if s.access == nil {
			send(w, 200, map[string]string{"Local author": "owner"})
			return
		}
		if r.Method == "GET" {
			s.access.mu.Lock()
			members := map[string]string{}
			for name, role := range s.access.data.Roles[parts[3]] {
				members[name] = role
			}
			s.access.mu.Unlock()
			send(w, 200, members)
			return
		}
		if r.Method == "POST" {
			var input struct {
				User string `json:"user"`
				Role string `json:"role"`
			}
			if e := decode(w, r, &input); e != nil {
				failure(w, e)
				return
			}
			if e := s.access.grant(parts[3], input.User, input.Role); e != nil {
				failure(w, e)
				return
			}
			send(w, 200, map[string]bool{"ok": true})
			return
		}
		break
	case "experiment":
		if r.Method != "POST" {
			break
		}
		var c project.ExperimentRequest
		if e := decode(w, r, &c); e != nil {
			failure(w, e)
			return
		}
		out, e := store.Experiment(c)
		if e != nil {
			failure(w, e)
			return
		}
		send(w, 200, out)
		return
	case "repository":
		if r.Method == "GET" {
			out, e := store.GitStatus()
			if e != nil {
				failure(w, e)
				return
			}
			send(w, 200, out)
			return
		}
		if r.Method == "POST" {
			var c project.GitCommand
			if e := decode(w, r, &c); e != nil {
				failure(w, e)
				return
			}
			if e := store.GitAction(c); e != nil {
				failure(w, e)
				return
			}
			send(w, 200, map[string]bool{"ok": true})
			return
		}
		break
	case "merge":
		if r.Method != "POST" {
			break
		}
		var c project.MergeRequest
		if e := decode(w, r, &c); e != nil {
			failure(w, e)
			return
		}
		out, e := store.GitMerge(c)
		if e != nil {
			failure(w, e)
			return
		}
		send(w, 200, out)
		return
	case "checks":
		if r.Method != "GET" {
			break
		}
		out, e := store.Checks(age)
		if e != nil {
			failure(w, e)
			return
		}
		send(w, 200, out)
		return
	case "archive":
		if r.Method != "GET" {
			break
		}
		file, e := os.CreateTemp("", "kriemhild-backup-*.zip")
		if e != nil {
			failure(w, e)
			return
		}
		defer func() { file.Close(); os.Remove(file.Name()) }()
		if e = store.WriteArchive(file); e != nil {
			failure(w, e)
			return
		}
		info, e := file.Stat()
		if e != nil {
			failure(w, e)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", "attachment; filename=world.kriemhild.zip")
		http.ServeContent(w, r, "world.kriemhild.zip", info.ModTime(), file)
		return
	case "export":
		if r.Method != "POST" {
			break
		}
		var c project.ExportRequest
		if e := decode(w, r, &c); e != nil {
			failure(w, e)
			return
		}
		out, e := store.Export(c)
		if e != nil {
			failure(w, e)
			return
		}
		w.Header().Set("Content-Type", out.MIME)
		w.Header().Set("Content-Disposition", "attachment; filename=kriemhild."+out.Extension)
		w.Write(out.Data)
		return
	case "terrain-preview":
		if r.Method != "POST" {
			break
		}
		var input project.TerrainRequest
		if e := decode(w, r, &input); e != nil {
			failure(w, e)
			return
		}
		out, e := store.PreviewTerrain(input)
		if e != nil {
			failure(w, e)
			return
		}
		send(w, 200, out)
		return
	case "historical":
		if r.Method != "GET" {
			break
		}
		out, e := store.Historical(r.URL.Query().Get("snapshot"), r.URL.Query().Get("tick"))
		if e != nil {
			failure(w, e)
			return
		}
		send(w, 200, out)
		return
	case "state":
		if r.Method != "GET" {
			break
		}
		state, e := store.State(age)
		if e != nil {
			failure(w, e)
			return
		}
		state.ReadOnly = role == "viewer"
		state.Role = role
		send(w, 200, state)
		return
	case "commands":
		if r.Method != "POST" {
			break
		}
		var c project.Command
		if e := decode(w, r, &c); e != nil {
			failure(w, e)
			return
		}
		state, e := store.Apply(c)
		if e != nil {
			failure(w, e)
			return
		}
		state.Role = role
		send(w, 200, state)
		return
	case "search":
		if r.Method != "GET" {
			break
		}
		out, e := store.Search(age, r.URL.Query().Get("q"))
		if e != nil {
			failure(w, e)
			return
		}
		send(w, 200, out)
		return
	case "search-page":
		if r.Method != "GET" {
			break
		}
		out, e := store.SearchPage(age, r.URL.Query().Get("q"), r.URL.Query().Get("cursor"), 100)
		if e != nil {
			failure(w, e)
			return
		}
		send(w, 200, out)
		return
	case "search-ages":
		if r.Method != "GET" {
			break
		}
		out, e := store.SearchAcrossAges(r.URL.Query().Get("q"), r.URL.Query().Get("cursor"), 100)
		if e != nil {
			failure(w, e)
			return
		}
		send(w, 200, out)
		return
	case "compare":
		if r.Method != "GET" {
			break
		}
		out, e := store.Differences(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
		if e != nil {
			failure(w, e)
			return
		}
		send(w, 200, out)
		return
	case "corrections":
		if r.Method != "GET" {
			break
		}
		out, incoming, e := store.Corrections(age)
		if e != nil {
			failure(w, e)
			return
		}
		send(w, 200, map[string]any{"changes": out, "incoming": incoming})
		return
	case "snapshots":
		if r.Method != "GET" || len(parts) != 6 {
			break
		}
		out, e := store.SnapshotRecords(parts[5])
		if e != nil {
			failure(w, e)
			return
		}
		send(w, 200, out)
		return
	case "assets":
		if r.Method == "GET" && len(parts) == 6 {
			b, e := store.Asset(parts[5])
			if e != nil {
				failure(w, e)
				return
			}
			w.Header().Set("Content-Type", http.DetectContentType(b))
			w.Write(b)
			return
		}
		if r.Method != "POST" || len(parts) != 5 {
			break
		}
		r.Body = http.MaxBytesReader(w, r.Body, 15<<20)
		b, e := io.ReadAll(r.Body)
		if e != nil {
			failure(w, fmt.Errorf("map images must be at most 15 MB"))
			return
		}
		cfg, format, e := image.DecodeConfig(bytes.NewReader(b))
		if e != nil || (format != "png" && format != "jpeg" && format != "gif") || cfg.Width > 20000 || cfg.Height > 20000 || int64(cfg.Width)*int64(cfg.Height) > 40000000 {
			failure(w, fmt.Errorf("use a PNG, JPEG or GIF image with at most 40 million pixels"))
			return
		}
		id, e := store.AddAsset(b)
		if e != nil {
			failure(w, e)
			return
		}
		send(w, 201, map[string]any{"asset": id, "width": cfg.Width, "height": cfg.Height})
		return
	}
	http.Error(w, "Route or method not supported", 405)
}
func (s *Server) store(id string) (*project.Store, error) {
	if len(id) != 36 || strings.ContainsAny(id, "/\\.") {
		return nil, fmt.Errorf("invalid project ID")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if store, ok := s.stores[id]; ok {
		return store, nil
	}
	path := filepath.Join(s.Data, id)
	actual, e := filepath.EvalSymlinks(path)
	if e != nil {
		return nil, e
	}
	if filepath.Dir(actual) != s.Data {
		return nil, fmt.Errorf("project is outside the library")
	}
	store, e := project.Open(actual)
	if e != nil {
		return nil, e
	}
	_, root, e := store.Root()
	if e != nil || root.World.ID != id {
		store.Close()
		return nil, fmt.Errorf("project identity mismatch")
	}
	s.stores[id] = store
	return store, nil
}
