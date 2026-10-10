package terrain

import (
	"fmt"
	"math"
)

// Grid coordinates locate cell centres; bounds describe cell edges. Equirectangular
// coordinates are a display projection, never an equal-area hydrology model.
type GeographicOptions struct {
	WidthPX       float64  `json:"width_px"`
	HeightPX      float64  `json:"height_px"`
	PixelsPerCell float64  `json:"pixels_per_cell"`
	WorldScale    float64  `json:"world_scale,omitempty"` // north/south km per cell
	Radius        float64  `json:"planetary_radius"`
	Coverage      string   `json:"map_coverage"`
	Projection    string   `json:"projection"`
	West          *float64 `json:"longitude_west,omitempty"`
	East          *float64 `json:"longitude_east,omitempty"`
}
type GeographicScale struct {
	Version       int       `json:"version"`
	Projection    string    `json:"projection"`
	Coverage      string    `json:"map_coverage"`
	Radius        float64   `json:"planetary_radius"`
	WidthPX       float64   `json:"width_px"`
	HeightPX      float64   `json:"height_px"`
	PixelsPerCell float64   `json:"pixels_per_cell"`
	WorldScale    float64   `json:"world_scale"`
	North         float64   `json:"latitude_north"`
	South         float64   `json:"latitude_south"`
	West          float64   `json:"longitude_west"`
	East          float64   `json:"longitude_east"`
	WrapX         bool      `json:"wrap_x"`
	CellAreaKM2   []float64 `json:"cell_area_km2"` // one value per row
}

func (o EnvironmentOptions) geographicScale() (*GeographicScale, error) {
	g := o.Geography
	if g == nil {
		return nil, nil
	} // Legacy saved worlds retain their original geometry.
	c := o.SeasonalClimate
	s := &GeographicScale{Version: 1, Projection: g.Projection, Coverage: g.Coverage, Radius: g.Radius, North: o.LatitudeNorth, South: o.LatitudeSouth, West: -180, East: 180, WidthPX: g.WidthPX, HeightPX: g.HeightPX, PixelsPerCell: g.PixelsPerCell}
	if s.Projection == "" {
		s.Projection = "equirectangular"
	}
	if s.Projection != "equirectangular" {
		return nil, fmt.Errorf("unsupported planetary projection")
	}
	if s.Coverage == "" {
		s.Coverage = "planet"
	}
	if s.Radius == 0 {
		s.Radius = 6371
	}
	if c != nil {
		if c.RadiusKM != nil && g.Radius == 0 {
			s.Radius = *c.RadiusKM
		}
		if g.Coverage == "" && c.Coverage != "" {
			s.Coverage = c.Coverage
		}
		if c.LongitudeWest != nil {
			s.West = *c.LongitudeWest
		}
		if c.LongitudeEast != nil {
			s.East = *c.LongitudeEast
		}
	}
	if g.West != nil {
		s.West = *g.West
	}
	if g.East != nil {
		s.East = *g.East
	}
	switch s.Coverage {
	case "planet", "hemisphere", "continent", "island", "local":
	default:
		return nil, fmt.Errorf("invalid map coverage")
	}
	if s.Coverage != "planet" && g.West == nil && g.East == nil && (c == nil || c.LongitudeWest == nil && c.LongitudeEast == nil) {
		span := map[string]float64{"hemisphere": 180, "continent": 80, "island": 15, "local": 2}[s.Coverage]
		s.West = -span / 2
		s.East = span / 2
	}
	if !finite(s.Radius) || s.Radius < 500 || s.Radius > 50000 {
		return nil, fmt.Errorf("planetary radius must be 500?50000 km")
	}
	if !finite(g.WorldScale) || g.WorldScale < 0 {
		return nil, fmt.Errorf("world_scale must be a positive distance")
	}
	if g.WorldScale > 0 {
		span := g.WorldScale * float64(o.Rows) / s.Radius * 180 / math.Pi
		if s.Coverage == "planet" && math.Abs(span-180) > 1e-6 {
			return nil, fmt.Errorf("full-planet world_scale must equal pi * radius / rows")
		}
		centre := (s.North + s.South) / 2
		s.North = centre + span/2
		s.South = centre - span/2
		if s.Coverage != "planet" && g.West == nil && g.East == nil {
			lonspan := g.WorldScale * float64(o.Columns) / s.Radius * 180 / math.Pi / math.Max(.05, math.Cos(centre*math.Pi/180))
			s.West = -lonspan / 2
			s.East = lonspan / 2
		}
	}
	if !finite(s.North) || !finite(s.South) || s.North > 90 || s.South < -90 || s.North <= s.South || !finite(s.West) || !finite(s.East) || s.West < -180 || s.East > 180 || s.East <= s.West {
		return nil, fmt.Errorf("invalid geographic extent")
	}
	if s.Coverage == "planet" && (math.Abs(s.North-90) > 1e-6 || math.Abs(s.South+90) > 1e-6 || math.Abs(s.East-s.West-360) > 1e-6) {
		s.Coverage = "regional"
	} // Explicit latitude bounds are authoritative.
	s.WrapX = math.Abs(s.East-s.West-360) < 1e-6
	if s.PixelsPerCell == 0 {
		s.PixelsPerCell = 1
	}
	if s.WidthPX == 0 {
		s.WidthPX = float64(o.Columns) * s.PixelsPerCell
	}
	if s.HeightPX == 0 {
		s.HeightPX = float64(o.Rows) * s.PixelsPerCell
	}
	if !finite(s.PixelsPerCell) || s.PixelsPerCell <= 0 || !finite(s.WidthPX) || !finite(s.HeightPX) || math.Abs(s.WidthPX/s.PixelsPerCell-float64(o.Columns)) > 1e-6 || math.Abs(s.HeightPX/s.PixelsPerCell-float64(o.Rows)) > 1e-6 {
		return nil, fmt.Errorf("geographic pixel dimensions do not match grid")
	}
	s.WorldScale = s.Radius * (s.North - s.South) * math.Pi / 180 / float64(o.Rows)
	dl := (s.East - s.West) * math.Pi / 180 / float64(o.Columns)
	for y := 0; y < o.Rows; y++ {
		north := (s.North - (s.North-s.South)*float64(y)/float64(o.Rows)) * math.Pi / 180
		south := (s.North - (s.North-s.South)*float64(y+1)/float64(o.Rows)) * math.Pi / 180
		s.CellAreaKM2 = append(s.CellAreaKM2, s.Radius*s.Radius*dl*(math.Sin(north)-math.Sin(south)))
	}
	return s, nil
}
func (e *Environment) cellArea(i int) float64 {
	if e.Geography == nil {
		return 1
	}
	return e.Geography.CellAreaKM2[i/e.Options.Columns]
}
func (e *Environment) areaWeight(i int) float64 {
	if e.Geography == nil {
		return 1
	}
	s := e.Geography
	mean := s.Radius * s.Radius * (s.East - s.West) * math.Pi / 180 * (math.Sin(s.North*math.Pi/180) - math.Sin(s.South*math.Pi/180)) / float64(e.Options.Columns*e.Options.Rows)
	return e.cellArea(i) / mean
}
func (e *Environment) neighbors(i int) []int {
	w, h := e.Options.Columns, e.Options.Rows
	out := nb(i, w, h)
	if e.Geography != nil && e.Geography.WrapX {
		if i%w == 0 {
			out = append(out, i+w-1)
		}
		if i%w == w-1 {
			out = append(out, i-w+1)
		}
	}
	return out
}
func (e *Environment) cellLatitude(i int) float64 {
	if e.Geography == nil {
		return e.Options.LatitudeNorth + (e.Options.LatitudeSouth-e.Options.LatitudeNorth)*float64(i/e.Options.Columns)/float64(e.Options.Rows-1)
	}
	s := e.Geography
	return s.North + (s.South-s.North)*(float64(i/e.Options.Columns)+.5)/float64(e.Options.Rows)
}
func (e *Environment) stepKM(i, j int) float64 {
	if e.Geography == nil {
		return e.hydroStep(i, j)
	}
	s := e.Geography
	w := e.Options.Columns
	a, b := e.cellLatitude(i)*math.Pi/180, e.cellLatitude(j)*math.Pi/180
	dl := float64(j%w-i%w) * (s.East - s.West) / float64(w) * math.Pi / 180
	v := math.Pow(math.Sin((b-a)/2), 2) + math.Cos(a)*math.Cos(b)*math.Pow(math.Sin(dl/2), 2)
	return 2 * s.Radius * math.Asin(math.Sqrt(clamp(v, 0, 1)))
}
func (e *Environment) edgeOutlet(i int) bool {
	if e.Geography == nil {
		return false
	}
	w, h := e.Options.Columns, e.Options.Rows
	return (!e.Geography.WrapX && (i%w == 0 || i%w == w-1)) || (i < w && e.Geography.North < 90) || (i >= w*(h-1) && e.Geography.South > -90)
}
func (e *Environment) gridDistance(mask []int, value int) []float64 {
	d := make([]float64, len(mask))
	q := []int{}
	for i := range d {
		d[i] = float64(e.Options.Columns + e.Options.Rows)
		if mask[i] == value {
			d[i] = 0
			q = append(q, i)
		}
	}
	for head := 0; head < len(q); head++ {
		i := q[head]
		for _, j := range e.neighbors(i) {
			if d[j] > d[i]+1 {
				d[j] = d[i] + 1
				q = append(q, j)
			}
		}
	}
	return d
}

func (e *Environment) ValidateGeographicData() error {
	s := e.Geography
	if s == nil {
		return nil
	}
	w, h := e.Options.Columns, e.Options.Rows
	if s.Version != 1 || s.Projection != "equirectangular" || !finite(s.Radius) || s.Radius < 500 || s.Radius > 50000 || !finite(s.North) || !finite(s.South) || s.North > 90 || s.South < -90 || s.North <= s.South || !finite(s.East) || !finite(s.West) || s.East > 180 || s.West < -180 || s.East <= s.West || len(s.CellAreaKM2) != h {
		return fmt.Errorf("invalid stored geographic reference system")
	}
	if !finite(s.PixelsPerCell) || s.PixelsPerCell <= 0 || !finite(s.WidthPX) || !finite(s.HeightPX) || math.Abs(s.WidthPX/s.PixelsPerCell-float64(w)) > 1e-6 || math.Abs(s.HeightPX/s.PixelsPerCell-float64(h)) > 1e-6 || !finite(s.WorldScale) || math.Abs(s.WorldScale-s.Radius*(s.North-s.South)*math.Pi/180/float64(h)) > 1e-6 || s.WrapX != (math.Abs(s.East-s.West-360) < 1e-6) {
		return fmt.Errorf("geographic grid dimensions or wrapping disagree with extent")
	}
	for y, a := range s.CellAreaKM2 {
		north := (s.North - (s.North-s.South)*float64(y)/float64(h)) * math.Pi / 180
		south := (s.North - (s.North-s.South)*float64(y+1)/float64(h)) * math.Pi / 180
		expected := s.Radius * s.Radius * (s.East - s.West) * math.Pi / 180 / float64(w) * (math.Sin(north) - math.Sin(south))
		if !finite(a) || a <= 0 || math.Abs(a-expected) > expected*1e-9 {
			return fmt.Errorf("invalid spherical cell area in row %d", y)
		}
	}
	if e.Hydrology != nil && e.Hydrology.NetworkVersion >= 3 {
		for _, name := range []string{"catchmentKM2", "dischargeM3s", "drainageDistanceKM"} {
			if len(e.Fields[name]) != len(e.Mask) {
				return fmt.Errorf("missing physical drainage field %s", name)
			}
		}
		for _, r := range e.Hydrology.Rivers {
			if !finite(r.LengthKM) || r.LengthKM <= 0 || !finite(r.CatchmentKM2) || r.CatchmentKM2 <= 0 || !finite(r.DischargeM3s) || r.DischargeM3s < 0 {
				return fmt.Errorf("invalid physical river measurements %s", r.ID)
			}
		}
	}
	if g := e.Geomorphology; g != nil {
		if err := (EnvironmentOptions{Erosion: &g.Parameters}).validateRiverOptions(); err != nil {
			return err
		}
		if g.Version != 1 || g.Iterations < 0 || g.Iterations > 6 || !finite(g.ErodedM3) || !finite(g.DepositedM3) || !finite(g.ExportedM3) || g.ErodedM3 < 0 || g.DepositedM3 < 0 || g.ExportedM3 < 0 || math.Abs(g.ErodedM3-g.DepositedM3-g.ExportedM3) > math.Max(1, g.ErodedM3)*1e-8 {
			return fmt.Errorf("invalid fluvial sediment budget")
		}
		formations := map[string]bool{}
		if e.Geology != nil {
			for _, f := range e.Geology.Formations {
				formations[f.ID] = true
			}
		}
		ids := map[string]bool{}
		for _, v := range g.Landforms {
			if v.ID == "" || ids[v.ID] || v.Cell < 0 || v.Cell >= len(e.Mask) || v.FormerTo < -1 || v.FormerTo >= len(e.Mask) || !formations[v.Formation] || !finite(v.AgeMa) || v.AgeMa < 0 || !finite(v.Incision) || v.Incision < 0 || !finite(v.Deposition) || v.Deposition < 0 || !finite(v.WidthKM) || v.WidthKM < 0 || v.Process == "" {
				return fmt.Errorf("invalid fluvial history %s", v.ID)
			}
			ids[v.ID] = true
		}
	}
	return nil
}
