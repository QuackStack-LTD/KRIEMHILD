// Package model defines the persisted continuous planet. Coordinates are radians,
// elevations and water surfaces are metres, precipitation is mm/year, and radius is km.
// It deliberately has no dependency on the legacy terrain solver or HTTP/rendering.
package model

import (
	"fmt"
	"math"
)

const Version = "continuous-3"
const Schema = 1
const ClimateVersion = 1

// Versions 2 and 3 remain explicit experimental new-world options. Saved worlds
// retain their original version; the default is still version 1.
const MaxClimateVersion = 3
const Tau = 2 * math.Pi

type Spec struct {
	ReliefVersion  int     `json:"reliefVersion,omitempty"`
	ClimateVersion int     `json:"climateVersion,omitempty"`
	Version        string  `json:"version"`
	Seed           string  `json:"seed"`
	Preset         string  `json:"preset"`
	RadiusKM       float64 `json:"radiusKm"`
	LandCoverage   float64 `json:"landCoverage"`
	Continents     int     `json:"continents"`
	Complexity     float64 `json:"complexity"`
	Islands        float64 `json:"islands"`
	Relief         float64 `json:"relief"`
	Rainfall       float64 `json:"rainfall"`
	Tilt           float64 `json:"tilt"`
	LayoutVariant  int     `json:"layoutVariant"`
	TileSize       int     `json:"tileSize"`
	SurfaceWidth   int     `json:"surfaceWidth,omitempty"`
}

func DefaultSpec() Spec {
	return Spec{Version: Version, ReliefVersion: 2, ClimateVersion: ClimateVersion, Seed: "Kriemhild", Preset: "earthlike", RadiusKM: 6371, LandCoverage: .32, Continents: 5, Complexity: 1, Islands: 1, Relief: .8, Rainfall: 1.65, Tilt: 23.4, TileSize: 256, SurfaceWidth: 4096}
}
func (s Spec) Validate() error {
	if s.ReliefVersion < 0 || s.ReliefVersion > 2 {
		return fmt.Errorf("unsupported relief version")
	}
	if s.ClimateVersion < 0 || s.ClimateVersion > MaxClimateVersion {
		return fmt.Errorf("unsupported climate version")
	}
	if s.Version != Version && s.Version != "continuous-1" && s.Version != "continuous-2" {
		return fmt.Errorf("unsupported terrain generator %q", s.Version)
	}
	if s.ClimateVersion >= 2 && s.Version != Version {
		return fmt.Errorf("climate version %d requires the continuous-3 terrain generator", s.ClimateVersion)
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
	if s.SurfaceWidth != 0 && s.SurfaceWidth != 1024 && s.SurfaceWidth != 2048 && s.SurfaceWidth != 4096 {
		return fmt.Errorf("surface width must be 1024, 2048 or 4096")
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
	Class        string    `json:"class,omitempty"`
	CatchmentKM2 float64   `json:"catchmentKm2,omitempty"`
	LengthKM     float64   `json:"lengthKm,omitempty"`
	Basin        int       `json:"basin,omitempty"`
	Parent       string    `json:"parent,omitempty"`
	Order        int       `json:"order,omitempty"`
	ID           string    `json:"id"`
	Destination  string    `json:"destination"`
	Path         []Point   `json:"path"`
	Bed          []float64 `json:"bed"`
	Surface      []float64 `json:"surface"`
	Discharge    []float64 `json:"discharge"`
	Width        []float64 `json:"width"`
}
type Grid struct {
	Width  int                  `json:"width"`
	Height int                  `json:"height"`
	Fields map[string][]float64 `json:"-"`
}
type Diagnostics struct {
	ClimateBudget       []ClimateWaterBudget          `json:"climateWaterBudget,omitempty"`
	Hydrology           *HydrologyDiagnostics         `json:"hydrology,omitempty"`
	CoastLengthKM       float64                       `json:"macroCoastLengthKm"`
	Compactness         float64                       `json:"macroSilhouetteCompactness"`
	CoverageDeviation   float64                       `json:"coverageDeviation"`
	LakeCoverage        float64                       `json:"lakeCoverage"`
	LongestRiverKM      float64                       `json:"longestRiverKm"`
	RiverDestinations   map[string]int                `json:"riverDestinations"`
	CatchmentAreas      []float64                     `json:"catchmentAreasKm2"`
	LatitudeBiomes      map[string]map[string]float64 `json:"latitudeBiomeShares"`
	MoistureBudgetError float64                       `json:"moistureBudgetRelativeError"`
	LandCoverage        float64                       `json:"landCoverage"`
	MajorContinents     int                           `json:"majorContinents"`
	LargestShare        float64                       `json:"largestShare"`
	IslandAreas         []float64                     `json:"islandAreasKm2"`
	BiomeShares         map[string]float64            `json:"biomeShares"`
	Warnings            []string                      `json:"warnings"`
	Timings             map[string]float64            `json:"timingsMs"`
}

// Monthly water quantities are mm over the WHOLE planet, area weighted. They
// expose the supply terms separately; mass conservation alone cannot validate
// the magnitude of free-water evaporation.
type ClimateWaterBudget struct {
	Month            int     `json:"month"`
	OceanEvaporation float64 `json:"oceanEvaporation"`
	LandEvaporation  float64 `json:"landEvaporation"`
	OceanPotential   float64 `json:"oceanPotential"`
	Precipitation    float64 `json:"precipitation"`
	StorageChange    float64 `json:"storageChange"`
}
type Planet struct {
	Surface     *Surface    `json:"surface,omitempty"`
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

// Emerged distinguishes inland depressions from the ocean. Version 1 used bed sign.
func (p *Planet) Emerged(i int) bool {
	if p.Spec.Version == "continuous-1" {
		return p.Grid.Fields["bed"][i] > 0
	}
	return p.Grid.Fields["coast"][i] > 0
}

type Plate struct {
	Center      [3]float64 `json:"center"`
	Motion      [3]float64 `json:"motion"`
	Continental bool       `json:"continental"`
}
type Lake struct {
	Path   []Point `json:"path,omitempty"`
	Width  float64 `json:"width,omitempty"`
	Kind   string  `json:"kind,omitempty"`
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
	PET           float64 `json:"pet"`
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
