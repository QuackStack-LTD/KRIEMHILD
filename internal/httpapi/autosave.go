package httpapi

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"kriemhild/internal/storage"
	"kriemhild/internal/terrain"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Internal checkpoints retain the entire running solver, including its RNG,
// work queues and backtracking state. Portable ZIPs still represent completed worlds.
type checkpoint struct {
	Version    int                         `json:"version"`
	Engine     string                      `json:"engine"`
	Solver     terrain.Solver              `json:"solver"`
	Config     terrain.Config              `json:"config"`
	Generation json.RawMessage             `json:"generation"`
	Base       *terrain.Snapshot           `json:"base"`
	Edits      []json.RawMessage           `json:"edits"`
	Undo       map[string]terrain.Snapshot `json:"undo"`
	Order      []string                    `json:"order"`
}

func compressJSON(value any) ([]byte, error) {
	var b bytes.Buffer
	z, err := gzip.NewWriterLevel(&b, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}
	if err = json.NewEncoder(z).Encode(value); err != nil {
		return nil, err
	}
	if err = z.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func expandJSON(data []byte, value any) error {
	z, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer z.Close()
	b, err := io.ReadAll(io.LimitReader(z, maxExpandedBytes+1))
	if err != nil {
		return err
	}
	if len(b) > maxExpandedBytes {
		return fmt.Errorf("checkpoint exceeds supported size")
	}
	return json.Unmarshal(b, value)
}

// Caller holds the session lock. A completed response means this transaction
// committed; client disconnects do not cancel a mutation that already ran.
func (s *Server) autosave(v *session, changed bool) error {
	if v.retired {
		return fmt.Errorf("world was replaced by an imported project")
	}
	if s.projects == nil {
		return nil
	}
	v.dirty = v.dirty || changed
	if v.worldID == "" {
		v.worldID = token()
	}
	parts := map[string][]byte{}
	add := func(name string, value any) error {
		b, err := compressJSON(value)
		if err == nil {
			parts[name] = b
		}
		return err
	}
	if !v.persisted || v.dirty {
		solver := *v.solver
		solver.Rules = nil
		solver.Environment = nil
		if err := add("checkpoint", checkpoint{1, terrain.DetailEngineVersion, solver, v.solver.Rules.Config, v.generation, v.base, v.edits, v.undo, v.order}); err != nil {
			return err
		}
	}
	if !v.persisted {
		if err := add("environment", v.solver.Environment); err != nil {
			return err
		}
	}
	if v.detail != nil && (!v.persisted || !v.detailPersisted) {
		if err := add("detail-model", v.detail.ProjectState()); err != nil {
			return err
		}
	}
	if len(v.projectUI) == 0 {
		v.projectUI = json.RawMessage(`{}`)
	}
	if err := add("view", v.projectUI); err != nil {
		return err
	}
	for key := range v.tiles {
		if v.persisted && v.savedTiles[key] {
			continue
		}
		b, err := os.ReadFile(filepath.Join(v.dir, filepath.FromSlash(key+".json")))
		if err != nil {
			return err
		}
		if err = add("tile/"+key, json.RawMessage(b)); err != nil {
			return err
		}
	}
	seed := ""
	if v.solver.Environment != nil {
		seed = v.solver.Environment.Options.Seed
	} else {
		var gen createRequest
		json.Unmarshal(v.generation, &gen)
		seed = fmt.Sprint(gen.Options.Seed)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.projects.SaveParts(ctx, storage.Project{ID: v.worldID, Seed: seed, Width: v.solver.W, Height: v.solver.H, DetailTiles: len(v.tiles)}, parts); err != nil {
		return err
	}
	v.persisted = true
	v.dirty = false
	v.detailPersisted = v.detail != nil
	v.savedTiles = make(map[string]bool, len(v.tiles))
	for key := range v.tiles {
		v.savedTiles[key] = true
	}
	return nil
}

func restoreCheckpoint(id string, parts map[string][]byte, cacheRoot string) (v *session, err error) {
	var c checkpoint
	if err = expandJSON(parts["checkpoint"], &c); err != nil {
		return nil, err
	}
	if c.Version != 1 || c.Engine != terrain.DetailEngineVersion {
		return nil, fmt.Errorf("unsupported autosave version")
	}
	solver := &c.Solver
	if solver.W < 16 || solver.W > 256 || solver.H < 16 || solver.H > 256 || solver.N != solver.W*solver.H || len(solver.Dom) != solver.N || len(solver.Pinned) != solver.N || len(solver.Locked) != solver.N || len(solver.InQueue) != solver.N || len(solver.Pos) != solver.N || len(solver.Buckets) != solver.T+1 || (solver.Status != "done" && solver.Status != "running" && solver.Status != "failed") {
		return nil, fmt.Errorf("invalid solver checkpoint")
	}
	solver.Rules, err = terrain.Compile(c.Config)
	if err != nil {
		return nil, err
	}
	if solver.T != len(c.Config.Types) {
		return nil, fmt.Errorf("checkpoint palette mismatch")
	}
	if err = expandJSON(parts["environment"], &solver.Environment); err != nil {
		return nil, err
	}
	v = &session{solver: solver, generation: c.Generation, base: c.Base, edits: c.Edits, undo: c.Undo, order: c.Order, worldID: id, cacheRoot: cacheRoot, last: time.Now(), persisted: true, savedTiles: map[string]bool{}}
	if v.undo == nil {
		v.undo = map[string]terrain.Snapshot{}
	}
	restored := v
	defer func() {
		if err != nil && restored.dir != "" {
			os.RemoveAll(restored.dir)
		}
	}()
	if err = expandJSON(parts["view"], &v.projectUI); err != nil {
		return nil, err
	}
	if b := parts["detail-model"]; b != nil {
		var state terrain.DetailState
		if err = expandJSON(b, &state); err != nil {
			return nil, err
		}
		v.detail, err = terrain.RestoreDetailModel(solver.Environment, state)
		if err != nil {
			return nil, err
		}
		v.detailPersisted = true
	}
	for name, b := range parts {
		if !strings.HasPrefix(name, "tile/") {
			continue
		}
		key := strings.TrimPrefix(name, "tile/")
		var tile terrain.DetailTile
		if err = expandJSON(b, &tile); err != nil {
			return nil, err
		}
		if key != detailKey(tile.Level, tile.X, tile.Y) {
			return nil, fmt.Errorf("invalid tile identity")
		}
		if err = terrain.ValidateDetailTile(tile, solver.Environment); err != nil {
			return nil, err
		}
		var raw json.RawMessage
		if err = expandJSON(b, &raw); err != nil {
			return nil, err
		}
		if err = v.putDetail(key, raw); err != nil {
			return nil, err
		}
		v.savedTiles[key] = true
	}
	for key := range v.tiles {
		var l, x, y int
		fmt.Sscanf(key, "%d/%d/%d", &l, &x, &y)
		if l > 0 && !v.tiles[detailKey(l-1, x/2, y/2)] {
			return nil, fmt.Errorf("missing stored parent tile")
		}
	}
	return v, nil
}

func (s *Server) saveView(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UI       json.RawMessage `json:"ui"`
		Merge    bool            `json:"merge"`
		Client   string          `json:"client"`
		Sequence uint64          `json:"sequence"`
	}
	if err := decode(w, r, &req); err != nil || len(req.UI) == 0 {
		fail(w, 400, "Builder state is required")
		return
	}
	s.mu.Lock()
	v := s.sessions[r.PathValue("id")]
	s.mu.Unlock()
	if v == nil {
		fail(w, 404, "Map session not found")
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.last = time.Now()
	if req.Client != "" && req.Client == v.viewClient && req.Sequence <= v.viewSequence {
		respond(w, 200, map[string]any{"worldId": v.worldID, "saved": s.projects != nil})
		return
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(req.UI, &fields) != nil || fields == nil {
		fail(w, 400, "Builder state must be an object")
		return
	}
	if req.Merge {
		merged := map[string]json.RawMessage{}
		json.Unmarshal(v.projectUI, &merged)
		for key, value := range fields {
			merged[key] = value
		}
		req.UI, _ = json.Marshal(merged)
	}
	v.projectUI = req.UI
	if err := s.autosave(v, false); err != nil {
		fail(w, 503, "Autosave failed. Changes are still in this session; retry before closing.")
		return
	}
	v.viewClient = req.Client
	v.viewSequence = req.Sequence
	respond(w, 200, map[string]any{"worldId": v.worldID, "saved": s.projects != nil})
}
