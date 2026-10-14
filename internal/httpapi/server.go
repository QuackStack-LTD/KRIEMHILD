package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"kriemhild/internal/auth"
	"kriemhild/internal/history"
	"kriemhild/internal/legacyterrain"
	"kriemhild/internal/storage"
	terrainmodel "kriemhild/internal/terrain/model"
	terraintiles "kriemhild/internal/terrain/tiles"
	"kriemhild/internal/world"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type session struct {
	tileIdentity    string
	planet          *terrainmodel.Planet
	settlement      *settlementDraft
	creatorID       string
	actorID         string
	collabRevision  int
	historyRevision int
	biosphereHover  *biosphereHoverCache
	climate         authoredClimate
	natural         *terrain.NaturalIndex
	mu              sync.Mutex
	solver          *terrain.Solver
	last            time.Time
	undo            map[string]terrain.Snapshot
	order           []string
	detail          *terrain.DetailModel
	dir             string
	cacheRoot       string
	tiles           map[string]bool
	generation      json.RawMessage
	projectUI       json.RawMessage
	worldID         string
	base            *terrain.Snapshot
	edits           []json.RawMessage
	persisted       bool
	unsaved         bool
	dirty           bool
	detailPersisted bool
	savedTiles      map[string]bool
	viewClient      string
	viewSequence    uint64
	retired         bool
	historical      *history.Map
	historyWorld    string
	world           *world.State
	composed        map[string][]byte
}
type Server struct {
	terrainTileCache *terraintiles.Cache
	terrainJobs      map[string]*terrainJob
	limits           requestLimits
	auth             *auth.Service
	accessMu         sync.Mutex
	liveMu           sync.Mutex
	live             map[*liveClient]bool
	mu               sync.Mutex
	sessions         map[string]*session
	mux              *http.ServeMux
	slots            chan struct{}
	openMu           sync.Mutex
	detailSlots      chan struct{}
	projects         *storage.Store
	cacheRoot        string
}

type Options struct {
	Auth     *auth.Service
	Projects *storage.Store
	CacheDir string
}

func New(dist string) *Server {
	return NewWithOptions(dist, Options{})
}
func NewWithOptions(dist string, options Options) *Server {
	s := &Server{sessions: map[string]*session{}, mux: http.NewServeMux(), slots: make(chan struct{}, 4), detailSlots: make(chan struct{}, 3)}
	s.projects = options.Projects
	s.auth = options.Auth
	s.live = map[*liveClient]bool{}
	s.cacheRoot = options.CacheDir
	if s.cacheRoot == "" {
		s.cacheRoot = filepath.Join(".tools", "projects")
	}
	s.mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]string{"name": "KRIEMHILD", "status": "ok", "engine": "native-go"})
	})
	s.mux.HandleFunc("GET /api/ready", func(w http.ResponseWriter, r *http.Request) {
		if s.projects != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if s.projects.Ping(ctx) != nil {
				fail(w, 503, "Project database is unavailable")
				return
			}
		}
		respond(w, 200, map[string]string{"status": "ready"})
	})
	s.mux.HandleFunc("GET /api/defaults", retiredTerrainEndpoint)
	s.mux.HandleFunc("POST /api/sessions", retiredTerrainEndpoint)
	s.continuousRoutes()
	s.mux.HandleFunc("DELETE /api/sessions/{id}", s.remove)
	s.mux.HandleFunc("POST /api/sessions/{id}/{action}", retiredTerrainEndpoint)
	s.mux.HandleFunc("GET /api/sessions/{id}/detail/{level}/{x}/{y}", s.detailTile)
	s.mux.HandleFunc("POST /api/sessions/{id}/project", s.saveProject)
	s.mux.HandleFunc("POST /api/sessions/{id}/autosave", s.saveView) // Legacy endpoint stages view only.
	s.mux.HandleFunc("POST /api/sessions/{id}/view", s.saveView)
	s.mux.HandleFunc("POST /api/sessions/{id}/save", s.commitWorld)
	s.mux.HandleFunc("DELETE /api/projects/{id}", s.deleteProject)
	s.mux.HandleFunc("GET /api/sessions/{id}/world", s.worldInfo)
	s.mux.HandleFunc("GET /api/sessions/{id}/world/state", s.worldState)
	s.mux.HandleFunc("GET /api/sessions/{id}/world/entities", s.worldEntities)
	s.mux.HandleFunc("POST /api/sessions/{id}/world/illustration", s.illustrationData)
	s.mux.HandleFunc("GET /api/sessions/{id}/world/artworks", s.listArtworks)
	s.mux.HandleFunc("POST /api/sessions/{id}/world/artworks", s.saveArtwork)
	s.mux.HandleFunc("GET /api/sessions/{id}/world/natural", s.worldNatural)
	s.mux.HandleFunc("GET /api/sessions/{id}/world/climate", s.worldClimateOverlay)
	s.mux.HandleFunc("POST /api/sessions/{id}/world", s.worldCommand)
	s.mux.HandleFunc("POST /api/projects/import", s.importProject)
	s.mux.HandleFunc("GET /api/projects", s.listProjects)
	s.mux.HandleFunc("POST /api/projects/{id}/open", s.openStoredProject)
	s.historyRoutes()
	s.accessRoutes()
	s.accountRoutes()
	s.collaborationRoutes()
	s.mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { fail(w, 404, "API route not found") })
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		file := filepath.Join(dist, filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/")))
		if info, err := os.Stat(file); err == nil && !info.IsDir() {
			http.ServeFile(w, r, file)
			return
		}
		if r.URL.Path != "/" && r.URL.Path != "/world/new" && !validWorldRoute(r.URL.Path) && !validHistoryRoute(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		if _, err := os.Stat(filepath.Join(dist, "index.html")); err != nil {
			http.Error(w, "Build the React frontend with npm run build, or use http://localhost:5173 during development.", 503)
			return
		}
		http.ServeFile(w, r, filepath.Join(dist, "index.html"))
	})
	return s
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.auth != nil && s.auth.Config.Mode == "oidc" {
		observed := &observedResponse{ResponseWriter: w}
		w = observed
		defer observeRequest(observed, r)
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Frame-Options", "DENY")
	if s.auth != nil && s.auth.Config.Mode == "oidc" {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https:; connect-src 'self'; worker-src 'self' blob:; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Cache-Control", "no-store")
		if origin := r.Header.Get("Origin"); origin != "" {
			allowed := origin == "http://"+r.Host || origin == "https://"+r.Host || origin == "http://127.0.0.1:5173" || origin == "http://localhost:5173"
			if s.auth != nil && s.auth.Config.Mode == "oidc" {
				allowed = origin == strings.TrimRight(s.auth.Config.PublicURL, "/")
			}
			if !allowed {
				fail(w, 403, "Origin not allowed")
				return
			}
		}
	}
	if s.auth != nil && (strings.HasPrefix(r.URL.Path, "/auth/") || strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/health" && r.URL.Path != "/api/ready") {
		s.secureServe(w, r)
		return
	}
	s.mux.ServeHTTP(w, r)
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	d := json.NewDecoder(r.Body)
	if err := d.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return fmt.Errorf("expected one JSON object")
	}
	return nil
}
func token() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

type createRequest struct {
	Config      json.RawMessage `json:"config"`
	Options     terrain.Options `json:"options"`
	Environment json.RawMessage `json:"environment"`
	WrapX       bool            `json:"wrapX"`
}

func (s *Server) remove(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	v := s.sessions[r.PathValue("id")]
	delete(s.sessions, r.PathValue("id"))
	s.mu.Unlock()
	if v != nil {
		v.mu.Lock()
		v.retired = true
		if v.dir != "" {
			_ = os.RemoveAll(v.dir)
		}
		v.mu.Unlock()
	}
	w.WriteHeader(204)
}

func (s *Server) detailTile(w http.ResponseWriter, r *http.Request) {
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	var a [3]int
	for i, k := range []string{"level", "x", "y"} {
		n, err := strconv.Atoi(r.PathValue(k))
		if err != nil {
			fail(w, 400, "Invalid detail address")
			return
		}
		a[i] = n
	}
	if !v.tiles[detailKey(a[0], a[1], a[2])] {
		fail(w, 404, "This detail was not saved in the archive. Legacy detail generation has been retired.")
		return
	}
	data, err := v.storedDetail(a[0], a[1], a[2])
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
func retiredTerrainEndpoint(w http.ResponseWriter, r *http.Request) {
	fail(w, http.StatusGone, "The legacy terrain generator has been retired. Use /api/terrain/generation-jobs and the continuous workspace.")
}
func view(s *terrain.Solver, initial bool) map[string]any {
	out := map[string]any{"dom": s.Dom, "locked": s.Locked, "pinned": s.Pinned, "status": s.Status, "message": s.Message, "steps": s.Steps, "backtracks": s.Backtracks, "repairs": s.Repairs, "cleaned": s.Cleaned, "settled": s.N - s.Open, "prefer": s.Prefer}
	if initial {
		out["W"] = s.W
		out["H"] = s.H
		out["N"] = s.N
		out["T"] = s.T
		out["K"] = s.K
		out["Z"] = s.Z
		out["wrapX"] = s.WrapX
		out["stability"] = s.Stability
		out["contShare"] = s.ContShare
		out["climShare"] = s.ClimShare
		out["contStrength"] = s.ContStrength
		out["contNeutral"] = s.ContNeutral
		out["climStrength"] = s.ClimStrength
		out["contPoints"] = s.ContPoints
		out["environment"] = renderEnvironment(s.Environment)
		out["config"] = s.Rules.Config
		dx, dy := []int{}, []int{}
		for _, d := range s.Offsets {
			dx = append(dx, d[0])
			dy = append(dy, d[1])
		}
		out["DX"] = dx
		out["DY"] = dy
		out["D"] = len(dx)
	}
	return out
}
