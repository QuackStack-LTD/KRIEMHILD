package httpapi

import (
	"encoding/json"
	"fmt"
	"kriemhild/internal/terrain"
	"kriemhild/internal/world"
	"math"
	"net/http"
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
	}
	v.world.PrepareLegacyRivers(v.solver.Environment)
	return v.world
}
func sessionState(v *session) map[string]any {
	out := view(v.solver, true)
	e := v.authored().Environment(v.solver.Environment)
	if e != nil {
		composed := *e
		composed.Climate = v.climateLayer()
		e = &composed
	}
	out["environment"] = renderEnvironment(e)
	out["worldId"] = v.worldID
	out["historicalMap"] = v.historical
	out["historyWorldId"] = v.historyWorld
	out["unsaved"] = v.unsaved
	out["project"] = v.authored().Header
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
	respond(w, 200, map[string]any{"entities": entities, "truncated": more, "revision": v.world.Header.Revision})
}
func (s *Server) worldCommand(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind        string          `json:"kind"`
		Revision    int             `json:"revision"`
		Entity      world.Entity    `json:"entity"`
		Operation   world.Operation `json:"operation"`
		Layers      []world.Layer   `json:"layers"`
		Name        string          `json:"name"`
		Description string          `json:"description"`
		View        json.RawMessage `json:"view"`
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
	previousView := append(json.RawMessage(nil), model.Header.View...)
	switch req.Kind {
	case "entity":
		if req.Entity.ID == "" {
			req.Entity.ID = token()
		}
		dirty, err = model.PutEntity(req.Entity)
	case "operation":
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
	respond(w, 200, map[string]any{"unsaved": v.unsaved, "project": model.Header, "dirty": dirty, "entityId": req.Entity.ID, "operationId": req.Operation.ID})
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
	if v.world == nil || len(v.world.Operations) == 0 {
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
