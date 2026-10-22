package generate

import (
	"fmt"
	"kriemhild/internal/terrain/model"
	"math"
)

// BiomeAt keeps the original aridity thresholds (annual mm / annual mm), then
// distinguishes seasonal and maritime vegetation. No target biome shares enter it.
func BiomeAt(p *model.Planet, lon, lat, bed float64, wet bool) string {
	if p.Spec.ClimateVersion >= 1 && p.Spec.Version == model.Version {
		return seasonalBiome(p, lon, lat, bed, wet)
	}
	g := p.Grid
	t := g.At("temperature", lon, lat) - (bed-g.At("bed", lon, lat))*.0065
	rain, pet := g.At("precipitation", lon, lat), g.At("pet", lon, lat)
	base := Biome(t, rain, pet, bed, wet)
	if p.Spec.Version == "continuous-1" || wet || base == "ice" || base == "tundra" || base == "alpine" || base == "desert" || base == "steppe" {
		return base
	}
	ratio := rain / max(100, pet)
	dry := g.At("dryMonths", lon, lat)
	if p.Surface != nil && g.At("wetland", lon, lat) > .5 {
		return "wetland"
	}
	if bed < 180 && g.At("soil", lon, lat) > 130 && g.At("slope", lon, lat) < .015 {
		return "wetland"
	}
	if t > 20 {
		if rain > 1600 && dry < 3 {
			return "rainforest"
		}
		if ratio < .8 || dry > 5 {
			return "savanna"
		}
		return "tropical-woodland"
	}
	if t < 8 {
		return "taiga"
	}
	if g.At("warmDry", lon, lat) >= 2 && g.At("coldRain", lon, lat) > rain*.55 {
		return "shrubland"
	}
	if ratio < .8 {
		return "grassland"
	}
	if ratio > 1.5 && dry < 2 {
		return "temperate-rainforest"
	}
	if ratio < 1 {
		return "woodland"
	}
	return "forest"
}

var temperatureMonths = [12]string{}
var rainfallMonths = [12]string{}

func init() {
	for m := 0; m < 12; m++ {
		temperatureMonths[m] = fmt.Sprintf("temperature-%02d", m)
		rainfallMonths[m] = fmt.Sprintf("rainfall-%02d", m)
	}
}

// These summaries are derived from persisted monthly fields, never biome quotas.
// Precomputation keeps per-pixel rendering independent of a 12-month scan.
var seasonalKeys = []string{"coldest", "warmest", "rainDriest", "summerDriest", "winterWettest", "summerRain", "winterRain"}

func seasonAt(g model.Grid, lon, lat float64) [7]float64 {
	if len(g.Fields["coldest"]) == g.Width*g.Height {
		var out [7]float64
		for i, k := range seasonalKeys {
			out[i] = g.At(k, lon, lat)
		}
		return out
	}
	out := [7]float64{math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(1), 0, 0, 0}
	for m := 0; m < 12; m++ {
		temp, rain := g.At(temperatureMonths[m], lon, lat), g.At(rainfallMonths[m], lon, lat)
		out[0] = math.Min(out[0], temp)
		out[1] = math.Max(out[1], temp)
		out[2] = math.Min(out[2], rain)
		summer := m >= 3 && m <= 8
		if lat < 0 {
			summer = !summer
		}
		if summer {
			out[3] = math.Min(out[3], rain)
			out[5] += rain
		} else {
			out[4] = math.Max(out[4], rain)
			out[6] += rain
		}
	}
	return out
}
func summarizeSeasons(g *model.Grid) {
	// Remove old summaries before rebuilding after elevation resampling.
	for _, k := range seasonalKeys {
		delete(g.Fields, k)
	}
	fields := make([][]float64, len(seasonalKeys))
	for k := range fields {
		fields[k] = make([]float64, g.Width*g.Height)
	}
	for i := 0; i < g.Width*g.Height; i++ {
		pt := g.Point(i%g.Width, i/g.Width)
		v := seasonAt(*g, pt[0], pt[1])
		for k := range fields {
			fields[k][i] = v[k]
		}
	}
	for k, key := range seasonalKeys {
		g.Fields[key] = fields[k]
	}
}

func seasonalBiome(p *model.Planet, lon, lat, bed float64, wet bool) string {
	if wet {
		return "water"
	}
	g := p.Grid
	delta := (bed - g.At("bed", lon, lat)) * .0065
	var fallback [7]float64
	cached := len(g.Fields["coldest"]) == g.Width*g.Height
	if !cached {
		fallback = seasonAt(g, lon, lat)
	}
	season := func(i int) float64 {
		if cached {
			return g.At(seasonalKeys[i], lon, lat)
		}
		return fallback[i]
	}
	coldest, warmest := season(0)-delta, season(1)-delta
	// Frozen growing seasons constrain vegetation before moisture does.
	if warmest < 0 {
		return "ice"
	}
	if warmest < 10 {
		if bed >= 1800 {
			return "alpine"
		}
		return "tundra"
	}
	if bed >= 1800 && warmest < 12 && (g.At("slope", lon, lat) > .12 || g.At("exposure", lon, lat) > .25) {
		return "alpine"
	}
	rain, pet := g.At("precipitation", lon, lat), g.At("pet", lon, lat)
	ratio := rain / max(100, pet)
	if ratio < .22 {
		return "desert"
	}
	// A demonstrable summer drought with a wet winter is not continental steppe.
	// Monthly precipitation is mm/month; annual rain/PET is dimensionless.
	if coldest > -3 && coldest < 18 && warmest > 18 {
		summerDriest := season(3)
		if summerDriest < 40 && summerDriest*3 < season(4) && season(6) > rain*.55 {
			return "shrubland"
		}
	}
	if ratio < .5 {
		return "steppe"
	}
	if g.At("wetland", lon, lat) > .5 || (bed < 180 && g.At("soil", lon, lat) > 130 && g.At("slope", lon, lat) < .015) {
		return "wetland"
	}
	dry := g.At("dryMonths", lon, lat)
	if coldest >= 18 {
		if rain >= 1800 && ratio >= 1 && dry < 2 && season(2) >= 60 {
			return "rainforest"
		}
		if ratio < .8 || dry > 5 {
			return "savanna"
		}
		return "tropical-woodland"
	}
	if ratio < .8 {
		return "grassland"
	}
	if coldest < -3 && warmest < 18 {
		return "taiga"
	}
	// Humidity alone does not make rainforest: require abundant absolute rain,
	// mild winters, cool summers and no pronounced dry season.
	if rain >= 1800 && ratio >= 1.5 && coldest > 0 && warmest < 24 && dry < 2 && season(2) >= 60 {
		return "temperate-rainforest"
	}
	if ratio < 1 {
		return "woodland"
	}
	return "forest"
}
