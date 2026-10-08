// Package terrain ports TerrainGenOnSteroids' algorithms to native Go.
package terrain

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
)

//go:embed tiles.json
var Defaults []byte

type Kind struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Color string  `json:"color"`
	Odds  float64 `json:"odds"`
}
type Type struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Color           string             `json:"color"`
	Weight          float64            `json:"weight"`
	Height          float64            `json:"height"`
	Continent       string             `json:"continent"`
	Pattern         string             `json:"pattern"`
	EnvironmentType string             `json:"environmentType,omitempty"`
	Neighbors       []json.RawMessage  `json:"neighbors"`
	WeightNear      map[string]float64 `json:"weightNear,omitempty"`
}
type Zone struct {
	ID    string             `json:"id"`
	Name  string             `json:"name"`
	Color string             `json:"color"`
	Kinds map[string]float64 `json:"kinds,omitempty"`
	Types map[string]float64 `json:"types,omitempty"`
}
type Config struct {
	Version    int    `json:"version"`
	Types      []Type `json:"types"`
	Continents []Kind `json:"continents"`
	Climates   []Zone `json:"climates"`
}
type Rules struct {
	Config             Config
	Allowed            []uint32
	Weight, Near       []float64
	Continent          []int
	TypeDist           []int
	KindMult, TypeMult []float64
}

func DecodeConfig(data []byte) (Config, error) {
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	// Defaults must distinguish an omitted numeric property from an explicit zero.
	var raw struct {
		Types      []map[string]json.RawMessage
		Continents []map[string]json.RawMessage
		Climates   []map[string]json.RawMessage
	}
	_ = json.Unmarshal(data, &raw)
	for i := range c.Types {
		t := &c.Types[i]
		r := raw.Types[i]
		if _, ok := r["id"]; !ok {
			t.ID = strconv.Itoa(i)
		}
		if _, ok := r["weight"]; !ok {
			t.Weight = 1
		}
		if _, ok := r["height"]; !ok {
			t.Height = .5
		}
		if _, ok := r["name"]; !ok {
			t.Name = t.ID
		}
		if _, ok := r["color"]; !ok {
			t.Color = "#ff00ff"
		}
	}
	if c.Continents == nil {
		c.Continents = []Kind{{"water", "Water", "#3a7bd5", .4}, {"land", "Land", "#7cb342", .4}, {"mountain", "Mountain", "#8a8580", .2}}
	} else {
		for i := range c.Continents {
			k := &c.Continents[i]
			r := raw.Continents[i]
			if _, ok := r["odds"]; !ok {
				k.Odds = 1
			}
			if _, ok := r["name"]; !ok {
				k.Name = k.ID
			}
			if _, ok := r["color"]; !ok {
				k.Color = "#888888"
			}
		}
	}
	for i := range c.Climates {
		z := &c.Climates[i]
		r := raw.Climates[i]
		if _, ok := r["name"]; !ok {
			z.Name = z.ID
		}
		if _, ok := r["color"]; !ok {
			z.Color = "#888888"
		}
	}
	return c, nil
}
func Compile(c Config) (*Rules, error) {
	T, K, Z := len(c.Types), len(c.Continents), len(c.Climates)
	if T < 1 || T > 32 {
		return nil, fmt.Errorf("the config needs 1–32 terrain types")
	}
	r := &Rules{Config: c, Allowed: make([]uint32, T), Weight: make([]float64, T), Near: make([]float64, T*T), Continent: make([]int, T), TypeDist: make([]int, T*T), KindMult: make([]float64, Z*K), TypeMult: make([]float64, Z*T)}
	for i := range r.Near {
		r.Near[i] = math.NaN()
	}
	for i := range r.KindMult {
		r.KindMult[i] = 1
	}
	for i := range r.TypeMult {
		r.TypeMult[i] = 1
	}
	ids, kinds, zones := map[string]int{}, map[string]int{}, map[string]bool{}
	for i, t := range c.Types {
		if _, ok := ids[t.ID]; ok {
			return nil, fmt.Errorf("duplicate type id %q", t.ID)
		}
		ids[t.ID] = i
	}
	for i, k := range c.Continents {
		if _, ok := kinds[k.ID]; ok {
			return nil, fmt.Errorf("duplicate continent id %q", k.ID)
		}
		if k.Odds < 0 || !finite(k.Odds) {
			return nil, fmt.Errorf("invalid odds for %s", k.ID)
		}
		kinds[k.ID] = i
	}
	for i, t := range c.Types {
		if t.Weight < 0 || !finite(t.Weight) || !finite(t.Height) {
			return nil, fmt.Errorf("invalid weight or height for %s", t.ID)
		}
		r.Weight[i] = t.Weight
		r.Continent[i] = -1
		if t.Continent != "" && t.Continent != "none" {
			k, ok := kinds[t.Continent]
			if !ok {
				return nil, fmt.Errorf("unknown continent %q", t.Continent)
			}
			r.Continent[i] = k
		}
		if t.Neighbors == nil {
			return nil, fmt.Errorf("%s needs a neighbors array", t.ID)
		}
		for _, ref := range t.Neighbors {
			var id string
			var j int
			if json.Unmarshal(ref, &id) == nil {
				v, ok := ids[id]
				if !ok {
					return nil, fmt.Errorf("unknown neighbor %q", id)
				}
				j = v
			} else {
				var n int
				if json.Unmarshal(ref, &n) != nil {
					return nil, fmt.Errorf("invalid neighbor")
				}
				if v, ok := ids[strconv.Itoa(n)]; ok {
					j = v
				} else {
					j = n
				}
				if j < 0 || j >= T {
					return nil, fmt.Errorf("invalid neighbor index")
				}
			}
			r.Allowed[i] |= bit(j)
			r.Allowed[j] |= bit(i)
		}
		for id, w := range t.WeightNear {
			j, ok := ids[id]
			if !ok {
				return nil, fmt.Errorf("unknown weightNear type %q", id)
			}
			if w < 0 || !finite(w) {
				return nil, fmt.Errorf("invalid near weight")
			}
			r.Near[i*T+j] = w
		}
	}
	for z, v := range c.Climates {
		if zones[v.ID] {
			return nil, fmt.Errorf("duplicate climate %q", v.ID)
		}
		zones[v.ID] = true
		for id, m := range v.Kinds {
			k, ok := kinds[id]
			if !ok || m < 0 || !finite(m) {
				return nil, fmt.Errorf("invalid climate kind %q", id)
			}
			r.KindMult[z*K+k] = m
		}
		for id, m := range v.Types {
			t, ok := ids[id]
			if !ok || m < 0 || !finite(m) {
				return nil, fmt.Errorf("invalid climate type %q", id)
			}
			r.TypeMult[z*T+t] = m
		}
	}
	for i := range r.TypeDist {
		r.TypeDist[i] = 255
	}
	for a := 0; a < T; a++ {
		r.TypeDist[a*T+a] = 0
		q := []int{a}
		for qi := 0; qi < len(q); qi++ {
			u := q[qi]
			for v := 0; v < T; v++ {
				if r.Allowed[u]&bit(v) != 0 && r.TypeDist[a*T+v] == 255 {
					r.TypeDist[a*T+v] = r.TypeDist[a*T+u] + 1
					q = append(q, v)
				}
			}
		}
	}
	return r, nil
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func bit(t int) uint32      { return uint32(1) << t }
func Seed(str string) uint32 {
	s := strings.TrimSpace(str)
	numeric := len(s) > 0
	for _, c := range s {
		if c < '0' || c > '9' {
			numeric = false
			break
		}
	}
	if numeric {
		v, _ := strconv.ParseFloat(s, 64)
		if !finite(v) {
			return 0
		}
		return uint32(uint64(math.Mod(v, 4294967296)))
	}
	h := uint32(0x811c9dc5)
	for _, c := range utf16.Encode([]rune(s)) {
		h ^= uint32(c)
		h *= 0x01000193
	}
	return h
}

type RNG uint32

func (r *RNG) Next() float64 {
	*r += 0x6d2b79f5
	t := uint32(*r)
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return float64(t^(t>>14)) / 4294967296
}
func clamp(v, lo, hi float64) float64 { return math.Min(hi, math.Max(lo, v)) }
func smooth(v float64) float64        { v = clamp(v, 0, 1); return v * v * (3 - 2*v) }
func round(v float64) float64         { return math.Floor(v + .5) }
