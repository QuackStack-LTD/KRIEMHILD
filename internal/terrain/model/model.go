// Package model defines the persisted continuous planet. Coordinates are radians,
// elevations and water surfaces are metres, precipitation is mm/year, and radius is km.
// It deliberately has no dependency on the legacy terrain solver or HTTP/rendering.
package model

import (
	"fmt"
	"math"
)

const Version = "continuous-1"
const Schema = 1
const Tau = 2 * math.Pi

type Spec struct {
	Version       string  `json:"version"`
	Seed          string  `json:"seed"`
	Preset        string  `json:"preset"`
	RadiusKM      float64 `json:"radiusKm"`
	LandCoverage  float64 `json:"landCoverage"`
	Continents    int     `json:"continents"`
	Complexity    float64 `json:"complexity"`
	Islands       float64 `json:"islands"`
	Relief        float64 `json:"relief"`
	Rainfall      float64 `json:"rainfall"`
	Tilt          float64 `json:"tilt"`
	LayoutVariant int     `json:"layoutVariant"`
	TileSize      int     `json:"tileSize"`
}

func DefaultSpec() Spec {
	return Spec{Version: Version, Seed: "Kriemhild", Preset: "earthlike", RadiusKM: 6371, LandCoverage: .32, Continents: 5, Complexity: 1, Islands: 1, Relief: 1, Rainfall: 1, Tilt: 23.4, TileSize: 256}
}
func (s Spec) Validate() error {
	if s.Version != Version {
		return fmt.Errorf("unsupported terrain generator %q", s.Version)
	}
	if len(s.Seed) == 0 || len(s.Seed) > 1200 || s.Continents < 1 || s.Continents > 12 || s.LayoutVariant < 0 || s.LayoutVariant > 1000000 {
		return fmt.Errorf("invalid seed or continent settings")
	}
	for _, v := range []struct{ v, lo, hi float64 }{{s.RadiusKM, 100, 100000}, {s.LandCoverage, .03, .8}, {s.Complexity, 0, 2}, {s.Islands, 0, 3}, {s.Relief, .1, 2}, {s.Rainfall, .1, 3}, {s.Tilt, 0, 60}} {
		if !Finite(v.v) || v.v < v.lo || v.v > v.hi {
			return fmt.Errorf("terrain setting outside supported range")
		}
	}
	switch s.Preset {
	case "earthlike", "archipelago", "rugged", "supercontinent", "ocean", "arid":
	default:
		return fmt.Errorf("unknown preset")
	}
	if s.TileSize != 256 && s.TileSize != 512 {
		return fmt.Errorf("tile size must be 256 or 512")
	}
	return nil
}

type Point [2]float64

// Knot is a local tangent-plane point with a radius, all in radians.
type Knot [3]float64
type Stroke struct {
	Knots []Knot `json:"knots"`
}
type Mass struct {
	ID       string   `json:"id"`
	Family   string   `json:"family"`
	Center   Point    `json:"center"`
	Rotation float64  `json:"rotation"`
	Extent   float64  `json:"extent"`
	Strokes  []Stroke `json:"strokes"`
	Cuts     []Stroke `json:"cuts"`
}
type River struct {
	ID          string    `json:"id"`
	Destination string    `json:"destination"`
	Path        []Point   `json:"path"`
	Bed         []float64 `json:"bed"`
	Surface     []float64 `json:"surface"`
	Discharge   []float64 `json:"discharge"`
	Width       []float64 `json:"width"`
}
type Grid struct {
	Width  int                  `json:"width"`
	Height int                  `json:"height"`
	Fields map[string][]float64 `json:"-"`
}
type Diagnostics struct {
	LandCoverage    float64            `json:"landCoverage"`
	MajorContinents int                `json:"majorContinents"`
	LargestShare    float64            `json:"largestShare"`
	IslandAreas     []float64          `json:"islandAreasKm2"`
	BiomeShares     map[string]float64 `json:"biomeShares"`
	Warnings        []string           `json:"warnings"`
	Timings         map[string]float64 `json:"timingsMs"`
}
type Planet struct {
	Schema      int         `json:"schema"`
	Spec        Spec        `json:"spec"`
	Seed        uint64      `json:"noiseSeed"`
	Masses      []Mass      `json:"masses"`
	SeaOffset   float64     `json:"seaOffset"`
	Grid        Grid        `json:"grid"`
	Rivers      []River     `json:"rivers"`
	Diagnostics Diagnostics `json:"diagnostics"`
	Lakes       []Lake      `json:"lakes"`
	Plates      []Plate     `json:"plates"`
}
type Plate struct {
	Center      [3]float64 `json:"center"`
	Motion      [3]float64 `json:"motion"`
	Continental bool       `json:"continental"`
}
type Lake struct {
	Center Point   `json:"center"`
	Radius float64 `json:"radius"`
	Level  float64 `json:"level"`
}
type Sample struct {
	Bed           float64 `json:"bed"`
	WaterSurface  float64 `json:"waterSurface"`
	WaterDepth    float64 `json:"waterDepth"`
	Water         string  `json:"water"`
	CoastDistance float64 `json:"coastDistanceKm"`
	Temperature   float64 `json:"temperature"`
	Precipitation float64 `json:"precipitation"`
	Moisture      float64 `json:"moisture"`
	Roughness     float64 `json:"roughness"`
	Biome         string  `json:"biome"`
}

func Finite(v float64) bool         { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func Clamp(v, a, b float64) float64 { return math.Max(a, math.Min(b, v)) }
func Wrap(lon float64) float64      { return lon - Tau*math.Floor((lon+math.Pi)/Tau) }
func (g Grid) Point(x, y int) Point {
	return Point{-math.Pi + Tau*float64(x)/float64(g.Width), math.Pi/2 - math.Pi*(float64(y)+.5)/float64(g.Height)}
}

// Bilinear interpolation wraps longitude and uses constant polar caps. This is a
// climate/drainage approximation only; it never determines coastline topology.
func (g Grid) At(name string, lon, lat float64) float64 {
	a := g.Fields[name]
	if len(a) != g.Width*g.Height {
		return 0
	}
	x := (Wrap(lon) + math.Pi) / Tau * float64(g.Width)
	y := Clamp((.5-lat/math.Pi)*float64(g.Height)-.5, 0, float64(g.Height-1))
	i, j := int(x)%g.Width, int(y)
	u, v := x-math.Floor(x), y-float64(j)
	i1, j1 := (i+1)%g.Width, min(j+1, g.Height-1)
	return (a[j*g.Width+i]*(1-u)+a[j*g.Width+i1]*u)*(1-v) + (a[j1*g.Width+i]*(1-u)+a[j1*g.Width+i1]*u)*v
}
