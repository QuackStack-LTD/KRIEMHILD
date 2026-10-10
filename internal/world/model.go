// Package world owns authored world data independently of the generator and renderer.
package world

import (
	"bytes"
	"encoding/json"
	"fmt"
	"kriemhild/internal/terrain"
	"math"
	"sort"
	"strings"
	"time"
)

type Layer struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Visible bool   `json:"visible"`
	Locked  bool   `json:"locked"`
	Order   int    `json:"order"`
}
type Entity struct {
	Cells     []int        `json:"cells,omitempty"`
	SourceID  string       `json:"sourceId,omitempty"`
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Kind      string       `json:"kind"`
	Layer     string       `json:"layer"`
	ParentID  string       `json:"parentId,omitempty"`
	Geometry  string       `json:"geometry"`
	Points    [][2]float64 `json:"points"`
	Color     string       `json:"color"`
	MinDetail float64      `json:"minDetail"`
	Created   *int         `json:"created,omitempty"`
	Destroyed *int         `json:"destroyed,omitempty"`
	Notes     string       `json:"notes,omitempty"`
	Deleted   bool         `json:"deleted,omitempty"`
}
type Operation struct {
	River    *terrain.DetailFeature `json:"river,omitempty"`
	ID       string                 `json:"id"`
	Kind     string                 `json:"kind"`
	Layer    string                 `json:"layer"`
	Center   [2]float64             `json:"center"`
	Radius   float64                `json:"radius"`
	Amount   float64                `json:"amount"`
	Target   float64                `json:"target"`
	Path     [][2]float64           `json:"path,omitempty"`
	Samples  []float64              `json:"samples,omitempty"`
	Year     *int                   `json:"year,omitempty"`
	Sequence int                    `json:"sequence"`
	Deleted  bool                   `json:"deleted,omitempty"`
}
type Change struct {
	Key    string          `json:"key"`
	Before json.RawMessage `json:"before"`
	After  json.RawMessage `json:"after"`
	Bounds *Bounds         `json:"bounds,omitempty"`
}
type Header struct {
	MapID        string          `json:"mapId,omitempty"`
	Version      int             `json:"version"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	CreatedAt    int64           `json:"createdAt"`
	Revision     int             `json:"revision"`
	Layers       []Layer         `json:"layers"`
	History      []Change        `json:"history"`
	Cursor       int             `json:"cursor"`
	NextSequence int             `json:"nextSequence"`
	View         json.RawMessage `json:"view"`
}
type State struct {
	Header                      Header               `json:"metadata"`
	Entities                    map[string]Entity    `json:"entities"`
	Operations                  map[string]Operation `json:"modifications"`
	entityIndex, operationIndex *Index
	dirty                       map[string]bool
	W, H                        float64 `json:"-"`
}

func New(w, h int) *State {
	layers := []Layer{}
	for i, name := range []string{"Terrain", "Elevation", "Water", "Vegetation", "Natural features", "Roads", "Settlements", "Buildings", "Borders", "Countries", "Labels"} {
		layers = append(layers, Layer{strings.ReplaceAll(strings.ToLower(name), " ", "-"), name, true, false, i})
	}
	s := &State{Header: Header{Version: 1, Name: "Unnamed World", CreatedAt: time.Now().UnixMilli(), Layers: layers, History: []Change{}, View: json.RawMessage(`{}`)}, Entities: map[string]Entity{}, Operations: map[string]Operation{}}
	s.Reindex(w, h)
	return s
}
func (s *State) Reindex(w, h int) {
	s.W = float64(w - 1)
	s.H = float64(h - 1)
	s.entityIndex = NewIndex(s.W, s.H)
	s.operationIndex = NewIndex(s.W, s.H)
	s.dirty = map[string]bool{}
	if s.Entities == nil {
		s.Entities = map[string]Entity{}
	}
	if s.Operations == nil {
		s.Operations = map[string]Operation{}
	}
	for id, e := range s.Entities {
		if !e.Deleted {
			s.entityIndex.Put(id, Box(e.Points), e.MinDetail)
		}
	}
	for id, o := range s.Operations {
		if !o.Deleted {
			s.operationIndex.Put(id, o.Bounds(), 0)
		}
	}
}
func Box(points [][2]float64) Bounds {
	if len(points) == 0 {
		return Bounds{}
	}
	b := Bounds{X: points[0][0], Y: points[0][1]}
	mx, my := b.X, b.Y
	for _, p := range points {
		b.X = math.Min(b.X, p[0])
		b.Y = math.Min(b.Y, p[1])
		mx = math.Max(mx, p[0])
		my = math.Max(my, p[1])
	}
	b.Width = mx - b.X
	b.Height = my - b.Y
	return b
}
func (o Operation) Bounds() Bounds {
	if len(o.Path) > 0 {
		return Box(o.Path).Expand(o.influenceRadius())
	}
	return Bounds{o.Center[0] - o.Radius, o.Center[1] - o.Radius, o.Radius * 2, o.Radius * 2}
}
func (s *State) Layer(id string) (Layer, bool) {
	for _, l := range s.Header.Layers {
		if l.ID == id {
			return l, true
		}
	}
	return Layer{}, false
}
func (s *State) visible(id string) bool { l, ok := s.Layer(id); return ok && l.Visible }
func (s *State) editable(id string) error {
	l, ok := s.Layer(id)
	if !ok {
		return fmt.Errorf("unknown layer")
	}
	if l.Locked {
		return fmt.Errorf("layer is locked")
	}
	return nil
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func (s *State) validPoint(p [2]float64) bool {
	return finite(p[0]) && finite(p[1]) && p[0] >= 0 && p[1] >= 0 && p[0] <= s.W && p[1] <= s.H
}
func validID(id string) bool {
	if len(id) < 1 || len(id) > 80 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
func (s *State) ValidateEntity(e Entity) error {
	if !validID(e.ID) || len(e.Name) > 160 || strings.TrimSpace(e.Name) == "" || len(e.Kind) > 80 || e.Kind == "" || len(e.Notes) > 8192 || !finite(e.MinDetail) || e.MinDetail < 0 || e.MinDetail > 8 {
		return fmt.Errorf("invalid entity properties")
	}
	if _, ok := s.Layer(e.Layer); !ok {
		return fmt.Errorf("unknown layer")
	}
	if len(e.Cells) > int((s.W+1)*(s.H+1)) {
		return fmt.Errorf("geographical cell set too large")
	}
	for _, cell := range e.Cells {
		if cell < 0 || cell >= int((s.W+1)*(s.H+1)) {
			return fmt.Errorf("geographical cell outside terrain")
		}
	}
	n := len(e.Points)
	if n > 2048 || !(e.Geometry == "point" && n == 1 || e.Geometry == "line" && n >= 2 || e.Geometry == "polygon" && n >= 3) {
		return fmt.Errorf("invalid geometry")
	}
	for _, p := range e.Points {
		if !s.validPoint(p) {
			return fmt.Errorf("geometry outside world")
		}
	}
	if len(e.Color) != 7 || e.Color[0] != '#' {
		return fmt.Errorf("color must be #rrggbb")
	}
	for _, c := range e.Color[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return fmt.Errorf("invalid color")
		}
	}
	if e.Created != nil && e.Destroyed != nil && *e.Destroyed < *e.Created {
		return fmt.Errorf("destruction precedes creation")
	}
	seen := map[string]bool{e.ID: true}
	for p := e.ParentID; p != ""; {
		if seen[p] {
			return fmt.Errorf("entity hierarchy contains a cycle")
		}
		seen[p] = true
		parent, ok := s.Entities[p]
		if !ok || parent.Deleted {
			return fmt.Errorf("parent entity does not exist")
		}
		p = parent.ParentID
	}
	return nil
}
func (s *State) ValidateOperation(o Operation) error {
	if !validID(o.ID) || !s.validPoint(o.Center) || !finite(o.Radius) || o.Radius < .01 || o.Radius > math.Max(s.W, s.H) || !finite(o.Amount) || math.Abs(o.Amount) > 12000 || !finite(o.Target) || math.Abs(o.Target) > 20000 || len(o.Path) > 2048 {
		return fmt.Errorf("invalid landscape operation")
	}
	layer := "elevation"
	switch o.Kind {
	case "water", "drain", "river":
		layer = "water"
	case "plant", "clear":
		layer = "vegetation"
	}
	if o.Layer != layer {
		return fmt.Errorf("incorrect operation layer")
	}
	if err := s.validateRiver(o); err != nil {
		return err
	}
	if len(o.Samples) > 81 || (len(o.Path) > 0 && len(o.Path) < 2) || (o.Kind == "river" && len(o.Path) < 2) {
		return fmt.Errorf("invalid operation samples or path")
	}
	for _, p := range o.Path {
		if !s.validPoint(p) {
			return fmt.Errorf("path outside world")
		}
	}
	for _, v := range o.Samples {
		if !finite(v) {
			return fmt.Errorf("invalid smoothing samples")
		}
	}
	if o.Kind == "smooth" && len(o.Samples) != 81 {
		return fmt.Errorf("smoothing requires saved local samples")
	}
	switch o.Kind {
	case "raise", "lower", "flatten", "smooth", "crater", "water", "drain", "plant", "clear", "valley", "island", "river":
		return nil
	}
	return fmt.Errorf("unsupported landscape operation")
}
func raw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
func (s *State) record(c Change) {
	if !strings.HasPrefix(c.Key, "entity/") {
		s.apply(c.Key, c.After)
		s.Header.Revision++
		return
	}
	s.Header.History = append(s.Header.History[:s.Header.Cursor], c)
	if len(s.Header.History) > 100 {
		s.Header.History = s.Header.History[1:]
	}
	s.Header.Cursor = len(s.Header.History)
	s.apply(c.Key, c.After)
	s.Header.Revision++
}
func (s *State) apply(key string, data json.RawMessage) {
	if strings.HasPrefix(key, "entity/") {
		var e Entity
		json.Unmarshal(data, &e)
		s.Entities[e.ID] = e
		s.entityIndex.Remove(e.ID)
		if !e.Deleted {
			s.entityIndex.Put(e.ID, Box(e.Points), e.MinDetail)
		}
	} else if strings.HasPrefix(key, "operation/") {
		var o Operation
		json.Unmarshal(data, &o)
		s.Operations[o.ID] = o
		s.operationIndex.Remove(o.ID)
		if !o.Deleted {
			s.operationIndex.Put(o.ID, o.Bounds(), 0)
		}
	} else if key == "layers" {
		json.Unmarshal(data, &s.Header.Layers)
	}
	s.dirty[key] = true
}
func (s *State) PutEntity(e Entity) (*Bounds, error) {
	if !AllowedGeometry(e.Layer, e.Kind, e.Geometry) {
		return nil, fmt.Errorf("geometry is not allowed for this object type")
	}
	if e.Color == "" {
		e.Color = "#000000"
	}
	if err := s.editable(e.Layer); err != nil {
		return nil, err
	}
	if err := s.ValidateEntity(e); err != nil {
		return nil, err
	}
	old, exists := s.Entities[e.ID]
	if exists && !bytes.Equal(raw(old.Points), raw(e.Points)) {
		e.Cells = nil
	}
	if exists {
		if err := s.editable(old.Layer); err != nil {
			return nil, err
		}
		if bytes.Equal(raw(old), raw(e)) {
			return nil, nil
		}
	} else {
		old = e
		old.Deleted = true
	}
	if e.Deleted {
		for _, child := range s.Entities {
			if !child.Deleted && child.ParentID == e.ID {
				return nil, fmt.Errorf("reparent or remove child entities first")
			}
		}
	}
	s.record(Change{Key: "entity/" + e.ID, Before: raw(old), After: raw(e)})
	return nil, nil
}
func (s *State) AddOperation(o Operation) (*Bounds, error) {
	if err := s.editable(o.Layer); err != nil {
		return nil, err
	}
	if err := s.ValidateOperation(o); err != nil {
		return nil, err
	}
	if _, ok := s.Operations[o.ID]; ok {
		return nil, fmt.Errorf("operation identity already exists")
	}
	s.Header.NextSequence++
	o.Sequence = s.Header.NextSequence
	old := o
	old.Deleted = true
	b := o.Bounds()
	s.record(Change{"operation/" + o.ID, raw(old), raw(o), &b})
	return &b, nil
}
func (s *State) SetLayers(layers []Layer) (*Bounds, error) {
	if err := s.validateLayers(layers); err != nil {
		return nil, err
	}
	var dirty *Bounds
	for _, l := range layers {
		old, _ := s.Layer(l.ID)
		if old.Visible != l.Visible && (l.ID == "elevation" || l.ID == "water" || l.ID == "vegetation") {
			b := Bounds{0, 0, s.W, s.H}
			dirty = &b
		}
	}
	s.record(Change{"layers", raw(s.Header.Layers), raw(layers), dirty})
	return dirty, nil
}
func (s *State) validateLayers(layers []Layer) error {
	if len(layers) != len(s.Header.Layers) {
		return fmt.Errorf("layer inventory must be preserved")
	}
	seen := map[string]bool{}
	orders := map[int]bool{}
	for _, l := range layers {
		if _, ok := s.Layer(l.ID); !ok || seen[l.ID] || orders[l.Order] || len(l.Name) > 80 || l.Order < 0 || l.Order >= len(layers) {
			return fmt.Errorf("invalid layers")
		}
		seen[l.ID] = true
		orders[l.Order] = true
	}
	return nil
}
func (s *State) Undo(redo bool) (*Bounds, error) {
	i := s.Header.Cursor - 1
	if redo {
		i = s.Header.Cursor
	}
	if i < 0 || i >= len(s.Header.History) {
		return nil, fmt.Errorf("no history available")
	}
	c := s.Header.History[i]
	data := c.Before
	if redo {
		data = c.After
	}
	if strings.HasPrefix(c.Key, "entity/") {
		var e Entity
		json.Unmarshal(data, &e)
		if err := s.editable(e.Layer); err != nil {
			return nil, err
		}
		if e.Deleted {
			for _, child := range s.Entities {
				if !child.Deleted && child.ParentID == e.ID {
					return nil, fmt.Errorf("reparent or remove child entities first")
				}
			}
		}
		if !e.Deleted {
			if err := s.ValidateEntity(e); err != nil {
				return nil, err
			}
		}
	}
	if redo {
		s.Header.Cursor++
	} else {
		s.Header.Cursor--
	}
	s.apply(c.Key, data)
	s.Header.Revision++
	return c.Bounds, nil
}
func (s *State) Query(b Bounds, detail float64) ([]Entity, bool) {
	ids, more := s.entityIndex.QueryWhere(b, detail, 2000, func(id string) bool { return s.visible(s.Entities[id].Layer) })
	out := []Entity{}
	for _, id := range ids {
		e := s.Entities[id]
		if s.visible(e.Layer) {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, _ := s.Layer(out[i].Layer)
		b, _ := s.Layer(out[j].Layer)
		if a.Order != b.Order {
			return a.Order < b.Order
		}
		return out[i].ID < out[j].ID
	})
	return out, more
}
func (s *State) Ops(b Bounds) []Operation {
	ids, _ := s.operationIndex.Query(b, 8, len(s.Operations)+1)
	out := []Operation{}
	for _, id := range ids {
		o := s.Operations[id]
		if s.visible(o.Layer) {
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Sequence < out[j].Sequence })
	return out
}
func (s *State) Parts(all bool) map[string]any {
	p := map[string]any{"builder/header": s.Header}
	if !all {
		for key := range s.dirty {
			if strings.HasPrefix(key, "entity/") {
				p["builder/"+key] = s.Entities[strings.TrimPrefix(key, "entity/")]
			} else if strings.HasPrefix(key, "operation/") {
				p["builder/"+key] = s.Operations[strings.TrimPrefix(key, "operation/")]
			}
		}
		return p
	}
	for id, e := range s.Entities {
		if all || s.dirty["entity/"+id] {
			p["builder/entity/"+id] = e
		}
	}
	for id, o := range s.Operations {
		if all || s.dirty["operation/"+id] {
			p["builder/operation/"+id] = o
		}
	}
	return p
}
func (s *State) Saved() { s.dirty = map[string]bool{} }
func (s *State) Validate(w, h int) error {
	if s.Header.Version != 1 || len(s.Header.Name) > 160 || strings.TrimSpace(s.Header.Name) == "" || len(s.Header.Description) > 8192 || s.Header.Revision < 0 || s.Header.NextSequence < 0 || s.Header.Cursor < 0 || s.Header.Cursor > len(s.Header.History) || len(s.Header.History) > 100 {
		return fmt.Errorf("invalid world metadata")
	}
	s.Reindex(w, h)
	defaults := New(w, h)
	if len(s.Header.Layers) != len(defaults.Header.Layers) {
		return fmt.Errorf("invalid layer inventory")
	}
	seen := map[string]bool{}
	orders := map[int]bool{}
	for _, l := range s.Header.Layers {
		if _, ok := defaults.Layer(l.ID); !ok || seen[l.ID] || orders[l.Order] || l.Order < 0 || l.Order >= len(s.Header.Layers) {
			return fmt.Errorf("invalid layers")
		}
		seen[l.ID] = true
		orders[l.Order] = true
	}
	for id, e := range s.Entities {
		if id != e.ID {
			return fmt.Errorf("entity identity mismatch")
		}
	}
	for id, o := range s.Operations {
		if id != o.ID {
			return fmt.Errorf("operation identity mismatch")
		}
	}
	for _, c := range s.Header.History {
		if c.Bounds != nil && (!finite(c.Bounds.X) || !finite(c.Bounds.Y) || !finite(c.Bounds.Width) || !finite(c.Bounds.Height) || c.Bounds.Width < 0 || c.Bounds.Height < 0) {
			return fmt.Errorf("invalid history bounds")
		}
		for _, data := range []json.RawMessage{c.Before, c.After} {
			if strings.HasPrefix(c.Key, "entity/") {
				var e Entity
				if json.Unmarshal(data, &e) != nil || c.Key != "entity/"+e.ID || !validID(e.ID) {
					return fmt.Errorf("invalid entity history")
				}
				e.ParentID = "" // Historical parents may themselves have been undone.
				if s.ValidateEntity(e) != nil {
					return fmt.Errorf("invalid entity history geometry")
				}
				if _, ok := s.Entities[e.ID]; !ok {
					return fmt.Errorf("history references missing entity")
				}
			} else if strings.HasPrefix(c.Key, "operation/") {
				var o Operation
				if json.Unmarshal(data, &o) != nil || c.Key != "operation/"+o.ID || s.ValidateOperation(o) != nil {
					return fmt.Errorf("invalid operation history")
				}
				current, ok := s.Operations[o.ID]
				if !ok || current.Sequence != o.Sequence || c.Bounds == nil || *c.Bounds != o.Bounds() {
					return fmt.Errorf("operation history disagrees with world")
				}
			} else if c.Key == "layers" {
				var l []Layer
				if json.Unmarshal(data, &l) != nil || s.validateLayers(l) != nil {
					return fmt.Errorf("invalid layer history")
				}
			} else {
				return fmt.Errorf("unsupported history record")
			}
		}
	}
	for _, e := range s.Entities {
		if e.Deleted {
			e.ParentID = ""
		}
		if err := s.ValidateEntity(e); err != nil {
			return err
		}
	}
	sequences := map[int]bool{}
	for _, o := range s.Operations {
		if o.Sequence < 1 || o.Sequence > s.Header.NextSequence || sequences[o.Sequence] {
			return fmt.Errorf("invalid operation sequence")
		}
		sequences[o.Sequence] = true
		if err := s.ValidateOperation(o); err != nil {
			return err
		}
	}
	// Older archives included landscape/layer actions in the shared history.
	// Preserve their current effects, but only objects participate in undo/redo.
	filtered := []Change{}
	cursor := 0
	for i, c := range s.Header.History {
		if strings.HasPrefix(c.Key, "entity/") {
			filtered = append(filtered, c)
			if i < s.Header.Cursor {
				cursor++
			}
		}
	}
	s.Header.History, s.Header.Cursor = filtered, cursor
	return nil
}

func AllowedGeometry(layer, kind, geometry string) bool {
	if kind == "custom-structure" {
		return geometry == "point" || geometry == "line" || geometry == "polygon"
	}
	if kind == "factory" || kind == "landmark" {
		return geometry == "point"
	}
	switch kind {
	case "village", "town", "city", "capital", "custom-settlement":
		return geometry == "polygon"
	case "road", "highway", "bridge", "tunnel", "railway", "port", "dam", "canal":
		return geometry == "line"
	case "house", "castle", "fortification", "temple", "administrative-building":
		return geometry == "point" || geometry == "polygon"
	}
	switch layer {
	case "settlements":
		return geometry == "polygon"
	case "roads":
		return geometry == "line"
	case "buildings":
		return geometry == "point" || geometry == "polygon"
	}
	return geometry == "point" || geometry == "line" || geometry == "polygon"
}

// Refresh shared footprints from historical state without losing this editor's
// pending commands or marking unrelated inherited entities as local edits.
func (s *State) RefreshRepresentations(shapes map[string]Entity) {
	dirty := s.dirty
	for key := range dirty {
		if strings.HasPrefix(key, "entity/") {
			id := strings.TrimPrefix(key, "entity/")
			shapes[id] = s.Entities[id]
		}
	}
	s.Entities = shapes
	s.Reindex(int(s.W)+1, int(s.H)+1)
	s.dirty = dirty
}
