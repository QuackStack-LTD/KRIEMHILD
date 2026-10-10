package terrain

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type SeasonalDescription struct {
	ClimateSeason
	StartFraction  float64 `json:"startFraction"`
	EndFraction    float64 `json:"endFraction"`
	DurationDays   float64 `json:"durationDays"`
	MinTemperature float64 `json:"minTemperature"`
	MaxTemperature float64 `json:"maxTemperature"`
	Rain           float64 `json:"precipitation"`
	Humidity       float64 `json:"humidity"`
	WindSpeed      float64 `json:"windSpeed"`
	WindDirection  string  `json:"windDirection"`
	Description    string  `json:"description"`
}
type ClimateDescription struct {
	Available      bool                  `json:"available"`
	Approximate    bool                  `json:"approximate"`
	ZoneID         string                `json:"zoneId"`
	Zone           string                `json:"zone"`
	Transition     []string              `json:"transitionZones"`
	Summary        string                `json:"summary"`
	Influences     []string              `json:"influences"`
	Mode           string                `json:"mode"`
	RequestedCount int                   `json:"requestedCount"`
	LocalCount     int                   `json:"localCount"`
	CalendarCount  int                   `json:"calendarCount"`
	YearDays       float64               `json:"yearDays"`
	AxialTilt      float64               `json:"axialTilt"`
	Eccentricity   float64               `json:"eccentricity"`
	ResolutionKM   [2]float64            `json:"resolutionKm"`
	Seasons        []SeasonalDescription `json:"seasons"`
	Months         [12]ClimateMonth      `json:"months"`
}

func (c *ClimateState) Describe(x, y float64) ClimateDescription {
	if c == nil {
		return ClimateDescription{Summary: "This saved terrain has no seasonal climate layer. Its stored geography has been preserved."}
	}
	w, h := c.Scale.GridWidth, c.Scale.GridHeight
	i := int(clamp(math.Round(y), 0, float64(h-1)))*w + int(clamp(math.Round(x), 0, float64(w-1)))
	cell := c.Cells[i]
	z := c.Zones[cell.Zone]
	m := cell.Modifiers
	q := ClimateDescription{Available: true, Approximate: c.Scale.Approximate, ZoneID: z.ID, Zone: z.Name, Mode: c.Parameters.Mode, RequestedCount: c.Parameters.RequestedCount, CalendarCount: c.FinalCount, LocalCount: len(cell.Seasons), YearDays: c.Parameters.OrbitalDays, AxialTilt: c.Parameters.AxialTilt, Eccentricity: c.Parameters.Eccentricity, Months: cell.Months, Influences: []string{}, Transition: []string{}, Seasons: []SeasonalDescription{}, ResolutionKM: [2]float64{rounded(c.Scale.EquatorialEastWestKM * math.Cos(m.Latitude*math.Pi/180)), rounded(c.Scale.NorthSouthKM)}}
	adjacent := map[string]bool{}
	for _, j := range nb(i, w, h) {
		other := c.Zones[c.Cells[j].Zone]
		if other.Kind != z.Kind {
			adjacent[other.Name] = true
		}
	}
	for name := range adjacent {
		q.Transition = append(q.Transition, name)
	}
	sort.Strings(q.Transition)
	if m.Maritime > .5 {
		q.Influences = append(q.Influences, "Nearby marine water moderates temperature swings.")
	}
	if m.Maritime < .2 {
		q.Influences = append(q.Influences, "The inland setting permits stronger seasonal temperature swings.")
	}
	if m.WaterProximity > .4 && m.Maritime < .5 {
		q.Influences = append(q.Influences, "Nearby inland water increases humidity and moderates local temperatures.")
	}
	if m.Elevation > 1200 {
		q.Influences = append(q.Influences, "High elevation lowers temperature through the atmospheric lapse rate.")
	}
	if m.RainShadow > .15 {
		q.Influences = append(q.Influences, "An upwind mountain barrier reduces the available moisture.")
	}
	if math.Abs(m.Current) > .5 {
		direction := "warm"
		if m.Current < 0 {
			direction = "cold"
		}
		q.Influences = append(q.Influences, "A "+direction+" ocean current modifies coastal temperatures.")
	}
	if m.Wetland > .5 {
		q.Influences = append(q.Influences, "Saturated wetlands sustain a humid surface environment.")
	}
	if m.Forest > .5 {
		q.Influences = append(q.Influences, "Vegetation moderates evaporation and supports soil-water storage.")
	}
	if m.Glacier > .1 {
		q.Influences = append(q.Influences, "Glacial ice maintains snow cover and supplies meltwater during warmer periods.")
	}
	if m.Geothermal > .4 {
		q.Influences = append(q.Influences, "Geothermal heating has a small ground-level influence; regional air remains governed by the climate.")
	}
	mean, rain, lo, hi := 0., 0., math.Inf(1), math.Inf(-1)
	for _, v := range cell.Months {
		mean += v.Temperature / 12
		rain += v.Rain
		lo = math.Min(lo, v.Temperature)
		hi = math.Max(hi, v.Temperature)
	}
	q.Summary = fmt.Sprintf("%s climate at %.0f m, %.1f° latitude. Modeled annual mean %.1f °C; monthly means %.1f–%.1f °C; %.0f mm precipitation per %.0f-day local year. %d local seasonal periods.", z.Name, m.Elevation, m.Latitude, mean, lo, hi, rain, q.YearDays, q.LocalCount)
	if q.Mode == "custom" {
		q.Summary += fmt.Sprintf(" The %d periods are user-defined subdivisions; similar periods do not imply different weather.", q.RequestedCount)
	}
	for index, s := range cell.Seasons {
		d := SeasonalDescription{ClimateSeason: s, StartFraction: float64(s.Start) / 12, EndFraction: float64((s.Start+s.Months)%12) / 12, DurationDays: rounded(float64(s.Months) * q.YearDays / 12), MinTemperature: math.Inf(1), MaxTemperature: math.Inf(-1)}
		avg, snow, drought, flood, light, windX, windY, soil := 0., 0., 0., 0., 0., 0., 0., 0.
		for n := 0; n < s.Months; n++ {
			v := cell.Months[(s.Start+n)%12]
			d.MinTemperature = math.Min(d.MinTemperature, v.Temperature)
			d.MaxTemperature = math.Max(d.MaxTemperature, v.Temperature)
			d.Rain += v.Rain
			d.Humidity += v.Humidity / float64(s.Months)
			d.WindSpeed += v.WindSpeed / float64(s.Months)
			avg += v.Temperature / float64(s.Months)
			snow += v.Snow / float64(s.Months)
			drought += v.Drought / float64(s.Months)
			flood += v.FloodPotential / float64(s.Months)
			light += v.Daylight / float64(s.Months)
			windX += v.WindX
			windY += v.WindY
			soil += v.SoilMoisture / float64(s.Months)
		}
		prev := cell.Seasons[(index+len(cell.Seasons)-1)%len(cell.Seasons)]
		previousTemp, previousRain := 0., 0.
		for n := 0; n < prev.Months; n++ {
			v := cell.Months[(prev.Start+n)%12]
			previousTemp += v.Temperature / float64(prev.Months)
			previousRain += v.Rain / float64(prev.Months)
		}
		change := "Temperatures remain similar to the previous period."
		if avg > previousTemp+1.5 {
			change = "Temperatures rise from the previous period."
		} else if avg < previousTemp-1.5 {
			change = "Temperatures fall from the previous period."
		}
		rainChange := ""
		if d.Rain/float64(s.Months) > previousRain*1.4+10 {
			rainChange = " Rainfall increases."
		} else if d.Rain/float64(s.Months)+10 < previousRain*.7 {
			rainChange = " Rainfall decreases."
		}
		conditions := []string{}
		if snow > .4 {
			conditions = append(conditions, "persistent snow or ice is modeled")
		}
		if d.MinTemperature < 0 {
			conditions = append(conditions, "freezing conditions occur in the monthly averages")
		}
		if drought > .6 {
			conditions = append(conditions, "soil-water deficit favors drought")
		}
		if flood > .4 {
			conditions = append(conditions, "water surplus raises local flooding potential")
		}
		if light < 2 {
			conditions = append(conditions, "little or no daylight limits solar heating")
		}
		if m.Monsoon > .45 && d.Rain/float64(s.Months) > rain/12*1.2 {
			conditions = append(conditions, "seasonally shifting winds deliver the wet circulation phase")
		}
		if avg > 5 && snow < .2 && soil > .4 && light > 8 {
			conditions = append(conditions, "warmth, moisture and daylight support vegetation growth")
		}
		directions := []string{"east", "southeast", "south", "southwest", "west", "northwest", "north", "northeast"}
		d.WindDirection = directions[(int(math.Round(math.Atan2(windY, windX)*4/math.Pi))+8)%8]
		if math.Hypot(windX, windY) < float64(s.Months)*.25 {
			d.WindDirection = "variable directions"
		}
		d.Description = fmt.Sprintf("%s%s Typical monthly means %.1f–%.1f °C; %.0f mm over %.0f days, %.0f%% relative humidity and %.1f m/s prevailing flow toward %s.", change, rainChange, d.MinTemperature, d.MaxTemperature, d.Rain, d.DurationDays, d.Humidity*100, d.WindSpeed, d.WindDirection)
		if len(conditions) > 0 {
			d.Description += " Modeled conditions: " + strings.Join(conditions, "; ") + "."
		}
		if len(q.Influences) > 0 {
			d.Description += " " + q.Influences[0]
		}
		q.Seasons = append(q.Seasons, d)
	}
	return q
}

func (c *ClimateState) Validate(w, h int) error {
	if c == nil {
		return nil
	}
	if c.Version != ClimateModelVersion || c.Scale.GridWidth != w || c.Scale.GridHeight != h || c.Scale.CellCount != w*h || len(c.Cells) != w*h || len(c.Zones) == 0 || len(c.Zones) > w*h || c.FinalCount != len(c.Calendar) {
		return fmt.Errorf("invalid climate grid, version or calendar")
	}
	p := c.Parameters
	if p.Mode != "automatic" && p.Mode != "custom" {
		return fmt.Errorf("invalid stored season mode")
	}
	o := EnvironmentOptions{Columns: w, Rows: h, SeasonalClimate: &SeasonalClimateOptions{Names: p.Names, Mode: p.Mode, Count: p.RequestedCount, MinSeasons: p.MinSeasons, MaxSeasons: p.MaxSeasons, AxialTilt: &p.AxialTilt, OrbitalDays: &p.OrbitalDays, Eccentricity: &p.Eccentricity, Perihelion: &p.Perihelion, Circulation: &p.Circulation, Rules: p.Rules, RadiusKM: &c.Scale.RadiusKM, LongitudeWest: &c.Scale.LongitudeWest, LongitudeEast: &c.Scale.LongitudeEast, Coverage: c.Scale.Coverage, WidthPX: c.Scale.WidthPX, HeightPX: c.Scale.HeightPX, PixelsPerCell: c.Scale.PixelsPerCell}}
	if err := o.validateClimateOptions(); err != nil {
		return err
	}
	if p.MinSeasons < 1 || p.MaxSeasons > 6 || p.MinSeasons > p.MaxSeasons || !finite(c.Scale.LatitudeNorth) || !finite(c.Scale.LatitudeSouth) || c.Scale.LatitudeNorth > 90 || c.Scale.LatitudeSouth < -90 || c.Scale.LatitudeNorth <= c.Scale.LatitudeSouth || !finite(c.Scale.NorthSouthKM) || !finite(c.Scale.EquatorialEastWestKM) || c.Scale.NorthSouthKM <= 0 || c.Scale.EquatorialEastWestKM <= 0 {
		return fmt.Errorf("invalid stored climate scale")
	}
	phases := func(seasons []ClimateSeason) bool {
		if len(seasons) < p.MinSeasons || len(seasons) > p.MaxSeasons || p.Mode == "custom" && len(seasons) != p.RequestedCount {
			return false
		}
		var used [12]bool
		total := 0
		for _, s := range seasons {
			if s.Start < 0 || s.Start >= 12 || s.Months < 1 || s.Months > 12 || s.Name == "" || len(s.Name) > 120 || s.UserDefined != (p.Mode == "custom") {
				return false
			}
			for n := 0; n < s.Months; n++ {
				at := (s.Start + n) % 12
				if used[at] {
					return false
				}
				used[at] = true
				total++
			}
		}
		return total == 12
	}
	if !phases(c.Calendar) {
		return fmt.Errorf("invalid climate calendar")
	}
	seen := make([]bool, w*h)
	ids := map[string]bool{}
	for zone, z := range c.Zones {
		if z.ID == "" || ids[z.ID] || len(z.ID) > 160 || len(z.Name) > 120 || z.Name == "" || len(z.Cells) == 0 {
			return fmt.Errorf("invalid climate zone identity")
		}
		ids[z.ID] = true
		for _, i := range z.Cells {
			if i < 0 || i >= len(seen) || seen[i] || c.Cells[i].Zone != zone {
				return fmt.Errorf("invalid climate zone cell relationship")
			}
			seen[i] = true
		}
	}
	for i, cell := range c.Cells {
		if !seen[i] || !phases(cell.Seasons) || cell.Estimate < 1 || cell.Estimate > 6 {
			return fmt.Errorf("invalid climate seasons at cell %d", i)
		}
		m := cell.Modifiers
		for _, v := range []float64{m.Elevation, m.Latitude, m.Maritime, m.WaterProximity, m.RainShadow, m.Current, m.Geothermal, m.Wetland, m.Glacier, m.Forest, m.River, m.Monsoon} {
			if !finite(v) {
				return fmt.Errorf("non-finite climate modifier at cell %d", i)
			}
		}
		for _, v := range cell.Months {
			for _, n := range []float64{v.Temperature, v.Rain, v.Humidity, v.Evaporation, v.WindX, v.WindY, v.WindSpeed, v.Snow, v.SoilMoisture, v.Drought, v.FloodPotential, v.Daylight} {
				if !finite(n) {
					return fmt.Errorf("non-finite monthly climate at cell %d", i)
				}
			}
			if v.Rain < 0 || v.Evaporation < 0 || v.WindSpeed < 0 || v.Daylight < 0 || v.Daylight > 24 || math.Abs(v.WindX) > 1.001 || math.Abs(v.WindY) > 1.001 {
				return fmt.Errorf("invalid climate measurement at cell %d", i)
			}
			for _, n := range []float64{v.Humidity, v.Snow, v.SoilMoisture, v.Drought, v.FloodPotential} {
				if n < 0 || n > 1 {
					return fmt.Errorf("invalid climate fraction at cell %d", i)
				}
			}
		}
	}
	return nil
}
