// Package resourcesurvey stores immutable, content-addressed observations on a
// spherical grid. Circle boundaries are coverage, never cell identity.
package resourcesurvey

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

const Version = 1

type Circle struct {
	Longitude      float64 `json:"longitude"`
	Latitude       float64 `json:"latitude"`
	RadiusKM       float64 `json:"radiusKm"`
	PlanetRadiusKM float64 `json:"planetRadiusKm"`
}

func (c Circle) Valid() bool {
	return finite(c.Longitude) && finite(c.Latitude) && finite(c.RadiusKM) && finite(c.PlanetRadiusKM) && math.Abs(c.Longitude) <= 180 && math.Abs(c.Latitude) <= 90 && c.PlanetRadiusKM > 0 && c.RadiusKM > 0 && c.RadiusKM <= math.Pi*c.PlanetRadiusKM
}
func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
func (c Circle) Area() float64 {
	return 4 * math.Pi * c.PlanetRadiusKM * c.PlanetRadiusKM * math.Pow(math.Sin(c.RadiusKM/c.PlanetRadiusKM/2), 2)
}
func Distance(lon, lat, x, y, r float64) float64 {
	const rad = math.Pi / 180
	a, b := (y-lat)*rad, (x-lon)*rad
	h := math.Pow(math.Sin(a/2), 2) + math.Cos(lat*rad)*math.Cos(y*rad)*math.Pow(math.Sin(b/2), 2)
	return 2 * r * math.Asin(math.Sqrt(math.Max(0, math.Min(1, h))))
}
func (c Circle) Contains(lon, lat float64) bool {
	return Distance(c.Longitude, c.Latitude, lon, lat, c.PlanetRadiusKM) <= c.RadiusKM+1e-8
}

type State struct {
	Biosphere      string             `json:"biosphere,omitempty"`
	Fields         map[string]float64 `json:"fields"`
	Biome          string             `json:"biome"`
	FreshWater     bool               `json:"freshWater"`
	RechargeMMYear float64            `json:"rechargeMmYear"`
	Resources      map[string]string  `json:"resources"` // occurrence identity -> immutable value hash
}
type Manifest struct {
	ReportBiosphere string            `json:"reportBiosphere,omitempty"`
	Communities     []string          `json:"communities,omitempty"`
	Version         int               `json:"version"`
	Level           int               `json:"level"`
	Cells           map[string]string `json:"cells"`
	Report          string            `json:"report"`
	Resources       map[string]string `json:"resources"`
}

func Hash(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }
func Put(blobs map[string][]byte, v any) string {
	raw, _ := json.Marshal(v)
	h := Hash(raw)
	blobs[h] = raw
	return h
}
func (m Manifest) References() []string {
	refs := map[string]bool{m.Report: true}
	if m.ReportBiosphere != "" {
		refs[m.ReportBiosphere] = true
	}
	for _, h := range m.Communities {
		refs[h] = true
	}
	for _, h := range m.Cells {
		refs[h] = true
	}
	for _, h := range m.Resources {
		refs[h] = true
	}
	out := []string{}
	for h := range refs {
		out = append(out, h)
	}
	sort.Strings(out)
	return out
}
func (m Manifest) Validate(blobs map[string][]byte) error {
	if m.Version != Version || m.Level < 1 || m.Level > 22 || len(m.Cells) == 0 || len(m.Cells) > 32768 {
		return fmt.Errorf("invalid resource sample manifest")
	}
	for _, h := range m.References() {
		if raw, ok := blobs[h]; !ok || Hash(raw) != h {
			return fmt.Errorf("missing or corrupt resource observation")
		}
	}
	if !json.Valid(blobs[m.Report]) {
		return fmt.Errorf("invalid saved resource report")
	}
	if m.ReportBiosphere != "" && !json.Valid(blobs[m.ReportBiosphere]) {
		return fmt.Errorf("invalid saved biological report")
	}
	communities := map[string]bool{}
	for _, h := range m.Communities {
		communities[h] = true
		if !json.Valid(blobs[h]) {
			return fmt.Errorf("invalid biological observation")
		}
	}
	for k, h := range m.Cells {
		var l, face, x, y int
		if n, _ := fmt.Sscanf(k, "%d/%d/%d/%d", &l, &face, &x, &y); n != 4 || l != m.Level || face < 0 || face > 5 || x < 0 || y < 0 || x >= 1<<l || y >= 1<<l || (Cell{l, face, x, y}).Key() != k {
			return fmt.Errorf("invalid resource cell")
		}
		var s State
		if json.Unmarshal(blobs[h], &s) != nil {
			return fmt.Errorf("invalid resource sample")
		}
		if s.Biosphere != "" && !communities[s.Biosphere] {
			return fmt.Errorf("missing biological observation")
		}
		for id, h := range s.Resources {
			if m.Resources[id] != h {
				return fmt.Errorf("invalid resource identity")
			}
		}
	}
	return nil
}

type Observation struct {
	ID         string
	Circle     Circle
	Year       *int64
	RecordedAt int64
	Manifest   Manifest
}

func Earlier(a, b Observation) bool { // Undated observations are usable now, never assigned a historical year.
	if a.Year != nil && b.Year != nil && *a.Year != *b.Year {
		return *a.Year < *b.Year
	}
	if (a.Year == nil) != (b.Year == nil) {
		return a.Year != nil
	}
	if a.RecordedAt != b.RecordedAt {
		return a.RecordedAt < b.RecordedAt
	}
	return a.ID < b.ID
}

type Resolved struct {
	Cell     Cell
	Hash     string
	ReportID string
}

// Resolve uses only retained observations in ONE Age and ONE terrain. Newer
// overlapping coverage wins; older knowledge outside that coverage survives.
func Resolve(circle Circle, observations []Observation, year *int64) []Resolved {
	obs := append([]Observation(nil), observations...)
	sort.Slice(obs, func(i, j int) bool { return Earlier(obs[i], obs[j]) })
	level := Level(circle)
	out := []Resolved{}
	cells := Cells(circle, level)
	interior := 0
	for _, cell := range cells {
		lon, lat := cell.Center()
		if !circle.Contains(lon, lat) {
			continue
		}
		interior++
		for i := len(obs) - 1; i >= 0; i-- {
			o := obs[i]
			if math.Abs(o.Circle.PlanetRadiusKM-circle.PlanetRadiusKM) > 1e-8 {
				continue
			}
			if year != nil && (o.Year == nil || *o.Year > *year) {
				continue
			}
			if !o.Circle.Contains(lon, lat) {
				continue
			}
			h := o.Manifest.Cells[At(o.Manifest.Level, lon, lat).Key()]
			if h != "" {
				out = append(out, Resolved{cell, h, o.ID})
				break
			}
		}
	}
	if interior == 0 {
		for i := len(obs) - 1; i >= 0; i-- {
			o := obs[i]
			if year != nil && (o.Year == nil || *o.Year > *year) {
				continue
			}
			if o.Circle.Contains(circle.Longitude, circle.Latitude) {
				cell := At(level, circle.Longitude, circle.Latitude)
				if h := o.Manifest.Cells[At(o.Manifest.Level, circle.Longitude, circle.Latitude).Key()]; h != "" {
					out = append(out, Resolved{cell, h, o.ID})
					break
				}
			}
		}
	}
	return out
}
