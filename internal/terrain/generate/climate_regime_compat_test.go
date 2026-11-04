package generate

import "kriemhild/internal/terrain/model"

// Frozen version-1 vegetation oracle from commit 2e4f5e4.
func seasonalBiomeBeforeRegimes(p *model.Planet, lon, lat, bed float64, wet bool) string {
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
