package project

import (
	"encoding/json"
	"fmt"
	"math"
)

// Local map coordinates are normalized fictional XY, never geographic degrees.
// The whole authored drawing belongs to this Age snapshot.
type LocalDrawing struct {
	Parent  string       `json:"parent,omitempty"`
	Anchor  Point        `json:"anchor"`
	Span    float64      `json:"span"`
	Unit    string       `json:"unit"`
	Grid    string       `json:"grid"`
	Columns int          `json:"columns"`
	Shapes  []LocalShape `json:"shapes"`
	Tokens  []LocalToken `json:"tokens"`
}
type LocalShape struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Kind   string  `json:"kind"`
	Level  int     `json:"level"`
	Points []Point `json:"points"`
}
type LocalToken struct {
	ID     string  `json:"id"`
	Entity string  `json:"entity"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Level  int     `json:"level"`
	Facing int     `json:"facing"`
}

func localDrawing(r Record) (*LocalDrawing, error) {
	value, ok := r.Properties["_localMap"]
	if !ok {
		return nil, nil
	}
	b, e := json.Marshal(value)
	if e != nil {
		return nil, e
	}
	var d LocalDrawing
	if e = json.Unmarshal(b, &d); e != nil {
		return nil, fmt.Errorf("invalid local map drawing")
	}
	return &d, nil
}
func normalized(p Point) bool {
	return !math.IsNaN(p.X) && !math.IsNaN(p.Y) && !math.IsInf(p.X, 0) && !math.IsInf(p.Y, 0) && p.X >= 0 && p.X <= 1 && p.Y >= 0 && p.Y <= 1
}
func validateLocalMap(r Record, records map[string]Record) error {
	d, e := localDrawing(r)
	if e != nil || d == nil {
		return e
	}
	if r.Kind != "map" {
		return fmt.Errorf("local drawings belong to maps")
	}
	if !normalized(d.Anchor) || d.Span <= 0 || d.Span > 1e12 || math.IsNaN(d.Span) || math.IsInf(d.Span, 0) || !nameOK(d.Unit) || d.Columns < 2 || d.Columns > 100 {
		return fmt.Errorf("local maps need a positive scale, units, an anchor and 2–100 grid columns")
	}
	if d.Grid != "none" && d.Grid != "square" && d.Grid != "hex" {
		return fmt.Errorf("unknown local map grid")
	}
	seen := map[string]bool{r.ID: true}
	for parent := d.Parent; parent != ""; {
		if seen[parent] || len(seen) > 100 {
			return fmt.Errorf("local map parent hierarchy is cyclic or too deep")
		}
		seen[parent] = true
		p, ok := records[parent]
		if !ok || p.Kind != "map" {
			return fmt.Errorf("local map parent must exist in this Age")
		}
		next, e := localDrawing(p)
		if e != nil {
			return e
		}
		if next == nil {
			break
		}
		parent = next.Parent
	}
	if len(d.Shapes) > 1000 || len(d.Tokens) > 1000 {
		return fmt.Errorf("local map exceeds 1000 shapes or tokens")
	}
	seen = map[string]bool{}
	for _, shape := range d.Shapes {
		if !validID(shape.ID) || seen[shape.ID] || !nameOK(shape.Name) || shape.Level < -100 || shape.Level > 100 {
			return fmt.Errorf("invalid local map shape")
		}
		seen[shape.ID] = true
		if shape.Kind != "room" && shape.Kind != "wall" && shape.Kind != "path" {
			return fmt.Errorf("invalid drawing tool")
		}
		if len(shape.Points) < 2 || len(shape.Points) > 200 || (shape.Kind == "room" && len(shape.Points) < 3) {
			return fmt.Errorf("drawing needs 2–200 points, or at least 3 for a room")
		}
		for _, p := range shape.Points {
			if !normalized(p) {
				return fmt.Errorf("drawing coordinates must be between 0 and 1")
			}
		}
	}
	for _, token := range d.Tokens {
		if !validID(token.ID) || seen[token.ID] || records[token.Entity].Kind != "entity" || !normalized(Point{token.X, token.Y}) || token.Level < -100 || token.Level > 100 || token.Facing < 0 || token.Facing >= 360 {
			return fmt.Errorf("invalid tactical token or entity")
		}
		seen[token.ID] = true
	}
	return nil
}
