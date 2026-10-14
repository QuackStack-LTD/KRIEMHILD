package world

import (
	"encoding/json"
	"fmt"
)

// Illustration coordinates are normalized map coordinates [0,2] x [0,1].
// Overrides link to stable source IDs; styles never mutate source geometry.
type IllustrationLayer struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Order   int     `json:"order"`
	Visible bool    `json:"visible"`
	Locked  bool    `json:"locked"`
	Opacity float64 `json:"opacity"`
}
type IllustrationObject struct {
	ID       string       `json:"id"`
	Source   string       `json:"source,omitempty"`
	Layer    string       `json:"layer"`
	Kind     string       `json:"kind"`
	Text     string       `json:"text,omitempty"`
	Position *[2]float64  `json:"position,omitempty"`
	Points   [][2]float64 `json:"points,omitempty"`
	Color    string       `json:"color,omitempty"`
	Symbol   string       `json:"symbol,omitempty"`
	Dash     string       `json:"dash,omitempty"`
	Hidden   bool         `json:"hidden,omitempty"`
}
type Illustration struct {
	Version int                           `json:"version"`
	ID      string                        `json:"id"`
	Title   string                        `json:"title"`
	Scope   string                        `json:"scope"`
	Focus   string                        `json:"focus,omitempty"`
	Style   string                        `json:"style"`
	Bounds  [4]float64                    `json:"bounds"`
	Layers  []IllustrationLayer           `json:"layers"`
	Objects map[string]IllustrationObject `json:"objects"`
}

func (a Illustration) Validate() error {
	if a.Version != 1 || !validID(a.ID) || len(a.Title) > 180 || len(a.Focus) > 160 || len(a.Layers) > 32 || len(a.Objects) > 3000 {
		return fmt.Errorf("invalid illustration")
	}
	if a.Scope != "world" && a.Scope != "continent" && a.Scope != "country" && a.Scope != "city" {
		return fmt.Errorf("invalid illustration scope")
	}
	if a.Style != "medieval" && a.Style != "historical" {
		return fmt.Errorf("invalid illustration style")
	}
	for _, v := range a.Bounds {
		if !finite(v) || v < -4 || v > 4 {
			return fmt.Errorf("invalid illustration extent")
		}
	}
	if a.Bounds[2] <= 0 || a.Bounds[3] <= 0 {
		return fmt.Errorf("empty illustration extent")
	}
	layers := map[string]bool{}
	orders := map[int]bool{}
	for _, l := range a.Layers {
		if !validID(l.ID) || layers[l.ID] || orders[l.Order] || len(l.Name) > 80 || l.Order < 0 || l.Order >= len(a.Layers) || !finite(l.Opacity) || l.Opacity < 0 || l.Opacity > 1 {
			return fmt.Errorf("invalid illustration layer")
		}
		layers[l.ID] = true
		orders[l.Order] = true
	}
	for id, o := range a.Objects {
		if o.Kind != "text" && o.Kind != "symbol" && o.Kind != "line" && o.Kind != "polygon" {
			return fmt.Errorf("invalid illustration object kind")
		}
		if id != o.ID || !validID(id) || len(o.Source) > 200 || len(o.Text) > 2000 || len(o.Color) > 32 || len(o.Symbol) > 32 || len(o.Dash) > 32 || !layers[o.Layer] || len(o.Points) > 4096 {
			return fmt.Errorf("invalid illustration object")
		}
		points := append([][2]float64{}, o.Points...)
		if o.Position != nil {
			points = append(points, *o.Position)
		}
		for _, p := range points {
			if !finite(p[0]) || !finite(p[1]) || p[0] < -4 || p[0] > 4 || p[1] < -4 || p[1] > 4 {
				return fmt.Errorf("invalid illustration coordinates")
			}
		}
	}
	return nil
}
func (s *State) PutIllustration(a Illustration) error {
	if err := a.Validate(); err != nil {
		return err
	}
	if len(s.Header.Illustrations) >= 32 {
		if _, ok := s.Header.Illustrations[a.ID]; !ok {
			return fmt.Errorf("too many illustrations")
		}
	}
	before := json.RawMessage("null")
	if old, ok := s.Header.Illustrations[a.ID]; ok {
		before = raw(old)
	}
	s.record(Change{Key: "illustration/" + a.ID, Before: before, After: raw(a)})
	return nil
}
