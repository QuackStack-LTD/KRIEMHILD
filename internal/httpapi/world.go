package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/history"
	"kriemhild/internal/legacyterrain"
	"kriemhild/internal/storage"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/world"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func validWorldRoute(path string) bool {
	p := strings.Split(strings.Trim(path, "/"), "/")
	if len(p) != 3 || p[0] != "world" || (p[2] != "generate" && p[2] != "build") || len(p[1]) != 32 {
		return false
	}
	for _, c := range p[1] {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
func (v *session) authored() *world.State {
	if v.world == nil {
		v.world = world.New(v.solver.W, v.solver.H)
		if v.planet != nil {
			v.world.Header.Version = 2
		}
	}
	v.world.PrepareLegacyRivers(v.solver.Environment)
	return v.world
}
func (v *session) terrainAnalysisStale() bool {
	for _, operation := range v.authored().Operations {
		if !operation.Deleted {
			return true
		}
	}
	return false
}

func sessionState(v *session) map[string]any {
	out := view(v.solver, true)
	if v.planet != nil {
		out["continuous"] = map[string]any{"schema": v.planet.Schema, "spec": v.planet.Spec, "diagnostics": v.planet.Diagnostics, "analysisStale": v.terrainAnalysisStale()}
	}
	e := v.authored().Environment(v.solver.Environment)
	if e != nil {
		composed := *e
		composed.Climate = v.climateLayer()
		e = &composed
	}
	out["environment"] = renderEnvironment(e)
	out["worldId"] = v.worldID
	out["mapRevision"] = v.collabRevision
	out["historicalMap"] = v.historical
	out["historyWorldId"] = v.historyWorld
	out["unsaved"] = v.unsaved
	out["project"] = v.authored().Header
	if count := v.authored().RecoveredParts(); count > 0 {
		out["recoveredEntityParts"] = count
	}
	return out
}
func (s *Server) worldSession(w http.ResponseWriter, r *http.Request) *session {
	s.mu.Lock()
	v := s.sessions[r.PathValue("id")]
	s.mu.Unlock()
	if v == nil {
		fail(w, 404, "World session expired; reopen it from Projects.")
		return nil
	}
	v.mu.Lock()
	if v.retired {
		v.mu.Unlock()
		fail(w, 409, "World was replaced by an import; reopen it from Projects.")
		return nil
	}
	v.last = time.Now()
	return v
}
func (s *Server) worldInfo(w http.ResponseWriter, r *http.Request) {
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	respond(w, 200, v.authored().Header)
}
func (s *Server) worldState(w http.ResponseWriter, r *http.Request) {
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	out := sessionState(v)
	out["id"] = r.PathValue("id")
	respond(w, 200, out)
}
func (s *Server) worldEntities(w http.ResponseWriter, r *http.Request) {
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	box := world.Bounds{Width: float64(v.solver.W - 1), Height: float64(v.solver.H - 1)}
	detail := 8.
	for name, dst := range map[string]*float64{"x": &box.X, "y": &box.Y, "width": &box.Width, "height": &box.Height, "detail": &detail} {
		if raw := r.URL.Query().Get(name); raw != "" {
			n, err := strconv.ParseFloat(raw, 64)
			if err != nil || math.IsInf(n, 0) || math.IsNaN(n) {
				fail(w, 400, "Invalid viewport")
				return
			}
			*dst = n
		}
	}
	if box.Width < 0 || box.Height < 0 || detail < 0 || detail > 8 {
		fail(w, 400, "Invalid viewport")
		return
	}
	entities, more := v.authored().Query(box, detail)
	if r.URL.Query().Get("all") == "1" {
		entities, more = v.authored().QueryLimit(box, detail, len(v.authored().Entities)+1)
	}
	respond(w, 200, map[string]any{"entities": entities, "truncated": more, "revision": v.world.Header.Revision})
}
func (s *Server) worldCommand(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind         string             `json:"kind"`
		Publish      bool               `json:"publish"`
		Revision     int                `json:"revision"`
		Entity       world.Entity       `json:"entity"`
		Operation    world.Operation    `json:"operation"`
		Layers       []world.Layer      `json:"layers"`
		Name         string             `json:"name"`
		Description  string             `json:"description"`
		View         json.RawMessage    `json:"view"`
		Delta        [2]float64         `json:"delta"`
		Illustration world.Illustration `json:"illustration"`
	}
	if err := decode(w, r, &req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	model := v.authored()
	if req.Revision != model.Header.Revision && req.Kind != "save" {
		respond(w, 409, map[string]any{"error": "World changed; refresh and retry.", "project": model.Header})
		return
	}
	var dirty *world.Bounds
	var err error
	if v.solver.Status != "done" && req.Kind != "metadata" && req.Kind != "view" && req.Kind != "save" {
		fail(w, 409, "Finish generation before building.")
		return
	}
	publishEntities := req.Kind == "entity" || req.Kind == "move-family" || req.Kind == "undo" || req.Kind == "redo"
	var previous *world.State
	if publishEntities && v.historical != nil {
		previous, err = model.Clone()
		if err != nil {
			fail(w, 500, "Could not prepare entity transaction")
			return
		}
	}
	previousView := append(json.RawMessage(nil), model.Header.View...)
	switch req.Kind {
	case "illustration":
		err = model.PutIllustration(req.Illustration)
	case "move-family":
		if v.planet == nil {
			err = fmt.Errorf("family moves require continuous terrain")
		} else {
			err = model.MoveFamily(req.Entity.ID, req.Delta[0], req.Delta[1])
		}
	case "entity":
		if req.Entity.ID == "" {
			req.Entity.ID = token()
		}
		old, exists := model.Entities[req.Entity.ID]
		if !exists {
			req.Entity.PlaceholderName = ""
			req.Entity.RecordStatus = "draft"
			if req.Entity.LogicalID != "" {
				req.Entity.RecordStatus = "published"
			}
		} else {
			req.Entity.RecordStatus = old.RecordStatus
			req.Entity.PlaceholderName = old.PlaceholderName
		}
		if !exists && !world.MeaningfulName(req.Entity) {
			req.Entity.PlaceholderName = req.Entity.Name
		}
		if req.Publish {
			if !world.MeaningfulName(req.Entity) {
				fail(w, 400, "Give this place or object its own name before applying changes")
				return
			}
			req.Entity.RecordStatus = "published"
		}
		if v.planet != nil && v.planet.Surface != nil && samePlaceFamily(req.Entity.Kind, "city") && (!exists || !reflect.DeepEqual(old.Points, req.Entity.Points)) {
			sampler := detail.New(v.planet)
			transform := continuousTransform(model.Ops(world.Box(req.Entity.Points)), v.solver.W, v.solver.H, v.planet)
			dry := false
			for _, point := range req.Entity.Points {
				lon, lat := (point[0]/float64(v.solver.W-1)-.5)*2*math.Pi, (.5-point[1]/float64(v.solver.H-1))*math.Pi
				sample := sampler.Sample(lon, lat)
				if transform != nil {
					sample = transform(sample, lon, lat)
				}
				if sample.Water == "dry" {
					dry = true
					break
				}
			}
			if !dry {
				fail(w, 400, "Place the settlement boundary on dry land; use inspection to check the site.")
				return
			}
		}
		if v.historical != nil && v.historical.Kind == "settlement" && req.Entity.Layer == "natural-features" {
			err = fmt.Errorf("geographical features are edited in the Map Builder")
			break
		}
		if req.Entity.ID == "" {
			req.Entity.ID = token()
		}
		if strings.TrimSpace(req.Entity.Name) == "" {
			req.Entity.Name = world.RecordName(req.Entity)
		}
		req.Entity.River = nil // Capture authoritative terrain-following geometry.
		if req.Entity.Color == "" {
			req.Entity.Color = "#000000"
		}
		if err = model.ValidateEntity(req.Entity); err == nil {
			model.PrepareEntityRiver(&req.Entity, v.solver.Environment)
			dirty, err = model.PutEntity(req.Entity)
		}
	case "operation":
		if v.historical != nil && v.historical.Kind == "settlement" {
			err = fmt.Errorf("terrain is edited in the Map Builder")
			break
		}
		if v.solver.Environment == nil {
			fail(w, 400, "Landscape editing requires Physical environment.")
			return
		}
		if req.Operation.ID == "" {
			req.Operation.ID = token()
		}
		req.Operation.River = nil // The server captures the authoritative channel profile.
		if req.Operation.Kind == "smooth" {
			req.Operation.Samples = make([]float64, 81)
		}
		if err = model.ValidateOperation(req.Operation); err == nil {
			model.Prepare(&req.Operation, v.solver.Environment)
			if v.planet != nil {
				v.prepareContinuousOperation(&req.Operation)
			}
			dirty, err = model.AddOperation(req.Operation)
		}
	case "layers":
		dirty, err = model.SetLayers(req.Layers)
	case "undo":
		dirty, err = model.Undo(false)
	case "redo":
		dirty, err = model.Undo(true)
	case "metadata":
		if strings.TrimSpace(req.Name) == "" || len(req.Name) > 160 || len(req.Description) > 8192 {
			err = fmt.Errorf("provide a name up to 160 characters and description up to 8192 characters")
		} else {
			model.Header.Name = strings.TrimSpace(req.Name)
			model.Header.Description = req.Description
			model.Header.Revision++
		}
	case "view":
		var obj map[string]json.RawMessage
		if len(req.View) > 64<<10 || json.Unmarshal(req.View, &obj) != nil || obj == nil {
			err = fmt.Errorf("invalid Builder view state")
		} else {
			model.Header.View = req.View
		}
	case "save":
	default:
		err = fmt.Errorf("unknown Builder action")
	}
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	// Publish entity identity and representation immediately. Terrain, explored
	// tiles and artwork still use the explicit Save action.
	if publishEntities && v.historical != nil {
		changes := world.New(v.solver.W, v.solver.H)
		for id, shape := range model.Entities {
			if old, exists := previous.Entities[id]; !exists || !reflect.DeepEqual(old, shape) {
				changes.Entities[id] = shape
			}
		}
		if len(changes.Entities) > 0 {
			err = s.publishBuilderEntities(v, changes)
		}
		if err != nil {
			*model = *previous
			fail(w, 503, "Could not persist entities: "+err.Error())
			return
		}
	}

	if dirty != nil {
		expanded := dirty.Expand(2)
		dirty = &expanded
		for key := range v.composed {
			var l, x, y int
			fmt.Sscanf(key, "%d/%d/%d", &l, &x, &y)
			step := math.Exp2(-float64(l))
			if (world.Bounds{X: float64(x) * 32 * step, Y: float64(y) * 32 * step, Width: 32 * step, Height: 32 * step}).Intersects(*dirty) {
				delete(v.composed, key)
			}
		}
	}
	if req.Kind == "save" {
		if err = s.saveWorld(v, false); err != nil {
			fail(w, 503, "Could not save world; click Save to retry.")
			return
		}
	} else if req.Kind != "view" || !sameJSON(previousView, req.View) {
		v.unsaved = true
	}
	respond(w, 200, map[string]any{"unsaved": v.unsaved, "project": model.Header, "dirty": dirty, "entityId": req.Entity.ID, "operationId": req.Operation.ID, "analysisStale": v.terrainAnalysisStale()})
}

// One publication path for individual drawings, generated settlements, grouped
// moves and undo/redo. All members of a batch reach the World/Age atomically.
func (s *Server) publishBuilderEntities(v *session, state *world.State) error {
	if v.historical == nil {
		return nil
	}
	revision := -1
	if s.auth != nil {
		revision = v.historyRevision
	}
	updated, err := s.projects.UpdateHistory(storage.WithActor(context.Background(), v.actorID), v.historyWorld, revision, func(d *history.Document) error {
		m := d.Map(v.historical.AgeID, v.historical.ID)
		if m == nil {
			return fmt.Errorf("map no longer exists in this Age")
		}
		for _, shape := range state.Entities {
			if shape.LogicalID != "" && !shape.Deleted {
				e := d.Entity(m.AgeID, shape.LogicalID)
				if e == nil || e.Deleted || (e.Kind != shape.Kind && !samePlaceFamily(e.Kind, shape.Kind)) {
					return fmt.Errorf("invalid logical entity reference")
				}
			}
		}
		d.Sync(*m, state)
		return nil
	})
	if err == nil {
		v.historyRevision = updated.World.Revision
	}
	return err
}

func samePlaceFamily(a, b string) bool {
	family := func(k string) int {
		switch k {
		case "country", "state", "kingdom", "empire":
			return 1
		case "capital", "city", "town", "village", "hamlet", "custom-settlement":
			return 2
		}
		return 0
	}
	return family(a) > 0 && family(a) == family(b)
}

// The generated tile store stays immutable; only intersecting composed tiles
// are invalidated after edits. Child morph targets use edited parent samples.
func (v *session) composedDetail(l, x, y int) ([]byte, error) {
	v.authored()
	key := detailKey(l, x, y)
	if data := v.composed[key]; data != nil {
		return data, nil
	}
	base, err := v.storedDetail(l, x, y)
	if err != nil {
		return nil, err
	}
	if v.world == nil || (len(v.world.Operations) == 0 && len(v.world.Entities) == 0) {
		return base, nil
	}
	var tile terrain.DetailTile
	if err = json.Unmarshal(base, &tile); err != nil {
		return nil, err
	}
	var parent *terrain.DetailTile
	if l > 0 {
		b, e := v.composedDetail(l-1, x/2, y/2)
		if e != nil {
			return nil, e
		}
		parent = &terrain.DetailTile{}
		if e = json.Unmarshal(b, parent); e != nil {
			return nil, e
		}
	}
	v.world.ApplyTile(&tile, parent)
	data, err := json.Marshal(tile)
	if err == nil {
		if v.composed == nil || len(v.composed) >= 192 {
			v.composed = map[string][]byte{}
		}
		v.composed[key] = data
	}
	return data, err
}
