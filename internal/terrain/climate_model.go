package terrain

import (
	"fmt"
	"math"
	"strings"
)

const ClimateModelVersion = "kriemhild-climate-1"

// Optional pointers distinguish an explicitly requested zero tilt/eccentricity
// from an unspecified planetary property. Defaults are generated from the seed.
type SeasonalClimateOptions struct {
	Names         []string      `json:"names,omitempty"`
	Mode          string        `json:"mode"`
	Count         int           `json:"count,omitempty"`
	MinSeasons    int           `json:"minSeasons,omitempty"`
	MaxSeasons    int           `json:"maxSeasons,omitempty"`
	AxialTilt     *float64      `json:"axialTilt,omitempty"`
	OrbitalDays   *float64      `json:"orbitalDays,omitempty"`
	Eccentricity  *float64      `json:"eccentricity,omitempty"`
	Perihelion    *float64      `json:"perihelion,omitempty"`
	Circulation   *float64      `json:"circulation,omitempty"`
	RadiusKM      *float64      `json:"radiusKm,omitempty"`
	LongitudeWest *float64      `json:"longitudeWest,omitempty"`
	LongitudeEast *float64      `json:"longitudeEast,omitempty"`
	Coverage      string        `json:"coverage,omitempty"`
	WidthPX       float64       `json:"width_px,omitempty"`
	HeightPX      float64       `json:"height_px,omitempty"`
	PixelsPerCell float64       `json:"pixels_per_cell,omitempty"`
	Rules         []ClimateRule `json:"rules,omitempty"`
}

type ClimateRule struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	MinTemperature float64 `json:"minTemperature"`
	MaxTemperature float64 `json:"maxTemperature"`
	MinRain        float64 `json:"minRain"`
	MaxRain        float64 `json:"maxRain"`
}

type ClimateParameters struct {
	Names          []string      `json:"names,omitempty"`
	Mode           string        `json:"mode"`
	RequestedCount int           `json:"requestedCount"`
	MinSeasons     int           `json:"minSeasons"`
	MaxSeasons     int           `json:"maxSeasons"`
	AxialTilt      float64       `json:"axialTilt"`
	OrbitalDays    float64       `json:"orbitalDays"`
	Eccentricity   float64       `json:"eccentricity"`
	Perihelion     float64       `json:"perihelion"`
	Circulation    float64       `json:"circulation"`
	Rules          []ClimateRule `json:"rules,omitempty"`
	Generated      []string      `json:"generated"`
}
type ClimateScale struct {
	WidthPX              float64 `json:"width_px"`
	HeightPX             float64 `json:"height_px"`
	PixelsPerCell        float64 `json:"pixels_per_cell"`
	GridWidth            int     `json:"grid_width"`
	GridHeight           int     `json:"grid_height"`
	CellCount            int     `json:"cell_count"`
	RadiusKM             float64 `json:"radiusKm"`
	LatitudeNorth        float64 `json:"latitudeNorth"`
	LatitudeSouth        float64 `json:"latitudeSouth"`
	LongitudeWest        float64 `json:"longitudeWest"`
	LongitudeEast        float64 `json:"longitudeEast"`
	NorthSouthKM         float64 `json:"northSouthKmPerCell"`
	EquatorialEastWestKM float64 `json:"equatorialEastWestKmPerCell"`
	Coverage             string  `json:"coverage"`
	Approximate          bool    `json:"approximate"`
}
type ClimateMonth struct {
	Temperature    float64 `json:"temperature"`
	Rain           float64 `json:"precipitation"`
	Humidity       float64 `json:"humidity"`
	Evaporation    float64 `json:"evaporation"`
	WindX          float64 `json:"windX"`
	WindY          float64 `json:"windY"`
	WindSpeed      float64 `json:"windSpeed"`
	Snow           float64 `json:"snow"`
	SoilMoisture   float64 `json:"soilMoisture"`
	Drought        float64 `json:"drought"`
	FloodPotential float64 `json:"floodPotential"`
	Daylight       float64 `json:"daylightHours"`
}
type ClimateSeason struct {
	Name        string `json:"name"`
	Start       int    `json:"startMonth"` // 0..11; end wraps through the local year.
	Months      int    `json:"months"`
	UserDefined bool   `json:"userDefined"`
}
type ClimateModifiers struct {
	Elevation      float64 `json:"elevation"`
	Latitude       float64 `json:"latitude"`
	Maritime       float64 `json:"maritime"`
	WaterProximity float64 `json:"waterProximity"`
	RainShadow     float64 `json:"rainShadow"`
	Current        float64 `json:"current"`
	Geothermal     float64 `json:"geothermal"`
	Wetland        float64 `json:"wetland"`
	Glacier        float64 `json:"glacier"`
	Forest         float64 `json:"forest"`
	River          float64 `json:"river"`
	Monsoon        float64 `json:"monsoon"`
}
type ClimateCell struct {
	Zone      int              `json:"zone"`
	Modifiers ClimateModifiers `json:"modifiers"`
	Months    [12]ClimateMonth `json:"months"`
	Seasons   []ClimateSeason  `json:"seasons"`
	Estimate  int              `json:"estimatedCount"`
}
type ClimateRegion struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Name  string `json:"name"`
	Cells []int  `json:"cells"`
}
type ClimateState struct {
	Version    string            `json:"version"`
	Parameters ClimateParameters `json:"parameters"`
	Scale      ClimateScale      `json:"scale"`
	Calendar   []ClimateSeason   `json:"calendar"`
	FinalCount int               `json:"finalCount"`
	Zones      []ClimateRegion   `json:"zones"`
	Cells      []ClimateCell     `json:"cells,omitempty"`
	Source     string            `json:"source"`
}

func (o EnvironmentOptions) validateClimateOptions() error {
	c := o.SeasonalClimate
	if c == nil {
		return nil
	}
	if c.Mode != "" && c.Mode != "automatic" && c.Mode != "custom" {
		return fmt.Errorf("invalid season mode")
	}
	if len(c.Names) > 0 {
		if c.Mode != "custom" || len(c.Names) != c.Count {
			return fmt.Errorf("season names must match the custom season count")
		}
		for _, name := range c.Names {
			if strings.TrimSpace(name) == "" || len(name) > 80 {
				return fmt.Errorf("invalid season name")
			}
		}
	}
	lo, hi := c.MinSeasons, c.MaxSeasons
	if lo == 0 {
		lo = 1
	}
	if hi == 0 {
		hi = 6
	}
	if lo < 1 || hi > 6 || hi < lo || (c.Mode == "custom" && (c.Count < lo || c.Count > hi)) {
		return fmt.Errorf("season count must be within the configured range 1..6")
	}
	for _, v := range []struct {
		p      *float64
		lo, hi float64
	}{{c.AxialTilt, 0, 90}, {c.OrbitalDays, 30, 3000}, {c.Eccentricity, 0, .6}, {c.Perihelion, 0, 1}, {c.Circulation, 0, 2}, {c.RadiusKM, 500, 50000}, {c.LongitudeWest, -180, 180}, {c.LongitudeEast, -180, 180}} {
		if v.p != nil && (!finite(*v.p) || *v.p < v.lo || *v.p > v.hi) {
			return fmt.Errorf("invalid planetary climate parameter")
		}
	}
	west, east := -180., 180.
	if c.LongitudeWest != nil {
		west = *c.LongitudeWest
	}
	if c.LongitudeEast != nil {
		east = *c.LongitudeEast
	}
	if east <= west {
		return fmt.Errorf("east longitude must exceed west longitude")
	}
	if c.Coverage != "" && c.Coverage != "planet" && c.Coverage != "hemisphere" && c.Coverage != "continent" && c.Coverage != "island" && c.Coverage != "local" {
		return fmt.Errorf("invalid geographic coverage")
	}
	if c.WidthPX != 0 || c.HeightPX != 0 || c.PixelsPerCell != 0 {
		if !finite(c.WidthPX) || !finite(c.HeightPX) || !finite(c.PixelsPerCell) || c.PixelsPerCell <= 0 || c.WidthPX <= 0 || c.HeightPX <= 0 || math.Abs(c.WidthPX/c.PixelsPerCell-float64(o.Columns)) > 1e-6 || math.Abs(c.HeightPX/c.PixelsPerCell-float64(o.Rows)) > 1e-6 {
			return fmt.Errorf("pixel dimensions / pixels_per_cell must match the terrain grid")
		}
	}
	if len(c.Rules) > 16 {
		return fmt.Errorf("at most 16 fictional climate rules")
	}
	seen := map[string]bool{}
	for _, r := range c.Rules {
		if r.ID == "" || len(r.ID) > 80 || seen[r.ID] || r.Name == "" || len(r.Name) > 120 || !finite(r.MinTemperature) || !finite(r.MaxTemperature) || !finite(r.MinRain) || !finite(r.MaxRain) || r.MinTemperature > r.MaxTemperature || r.MinRain < 0 || r.MinRain > r.MaxRain {
			return fmt.Errorf("invalid fictional climate classification rule")
		}
		seen[r.ID] = true
	}
	return nil
}

func (o EnvironmentOptions) climateConfiguration() (ClimateParameters, ClimateScale) {
	c := o.SeasonalClimate
	if c == nil {
		c = &SeasonalClimateOptions{}
	}
	rng := RNG(Seed(o.Seed) ^ 0x434c494d)
	p := ClimateParameters{Names: c.Names, Mode: c.Mode, RequestedCount: c.Count, MinSeasons: c.MinSeasons, MaxSeasons: c.MaxSeasons, Rules: c.Rules, Generated: []string{}}
	if p.Mode == "" {
		p.Mode = "automatic"
	}
	if p.MinSeasons == 0 {
		p.MinSeasons = 1
	}
	if p.MaxSeasons == 0 {
		p.MaxSeasons = 6
	}
	value := func(name string, v *float64, def float64) float64 {
		if v != nil {
			return *v
		}
		p.Generated = append(p.Generated, name)
		return def
	}
	p.AxialTilt = value("axialTilt", c.AxialTilt, 18+rng.Next()*12)
	p.OrbitalDays = value("orbitalDays", c.OrbitalDays, 310+rng.Next()*110)
	p.Eccentricity = value("eccentricity", c.Eccentricity, rng.Next()*.075)
	p.Perihelion = value("perihelion", c.Perihelion, rng.Next())
	p.Circulation = value("circulation", c.Circulation, .8+rng.Next()*.4)
	s := ClimateScale{WidthPX: c.WidthPX, HeightPX: c.HeightPX, PixelsPerCell: c.PixelsPerCell, GridWidth: o.Columns, GridHeight: o.Rows, CellCount: o.Columns * o.Rows, RadiusKM: 6371, LatitudeNorth: o.LatitudeNorth, LatitudeSouth: o.LatitudeSouth, LongitudeWest: -180, LongitudeEast: 180, Coverage: c.Coverage}
	if c.RadiusKM != nil {
		s.RadiusKM = *c.RadiusKM
	}
	if c.LongitudeWest != nil {
		s.LongitudeWest = *c.LongitudeWest
	}
	if c.LongitudeEast != nil {
		s.LongitudeEast = *c.LongitudeEast
	}
	if s.PixelsPerCell == 0 {
		s.PixelsPerCell = 1
		s.WidthPX = float64(o.Columns)
		s.HeightPX = float64(o.Rows)
	}
	if s.Coverage == "" {
		s.Coverage = "planet"
	}
	s.Approximate = c.RadiusKM == nil || c.Coverage == "" || c.LongitudeWest == nil || c.LongitudeEast == nil || len(p.Generated) > 0
	s.NorthSouthKM = s.RadiusKM * math.Pi / 180 * (o.LatitudeNorth - o.LatitudeSouth) / float64(max(1, o.Rows-1))
	s.EquatorialEastWestKM = s.RadiusKM * math.Pi / 180 * (s.LongitudeEast - s.LongitudeWest) / float64(max(1, o.Columns-1))
	if g, err := o.geographicScale(); g != nil && err == nil {
		s.WidthPX = g.WidthPX
		s.HeightPX = g.HeightPX
		s.PixelsPerCell = g.PixelsPerCell
		s.RadiusKM = g.Radius
		s.LatitudeNorth = g.North
		s.LatitudeSouth = g.South
		s.LongitudeWest = g.West
		s.LongitudeEast = g.East
		s.Coverage = g.Coverage
		s.NorthSouthKM = g.WorldScale
		s.EquatorialEastWestKM = g.Radius * (g.East - g.West) * math.Pi / 180 / float64(o.Columns)
		s.Approximate = s.Approximate || len(p.Generated) > 0
	}
	return p, s
}
