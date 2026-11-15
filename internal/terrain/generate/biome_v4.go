package generate

import "kriemhild/internal/terrain/model"

// BiomeFromClimate lets sampling share one monthly regime between vegetation
// and the displayed water balance. Physical water always takes precedence.
func BiomeFromClimate(p *model.Planet, c ClimateRegime, lon, lat, bed float64, wet bool) string {
	if wet {
		return "water"
	}
	return vegetationAt(p, c, lon, lat, bed)
}

func TropicalRainforestEligible(c ClimateRegime) bool {
	return c.Thermal == "tropical" && c.Coldest >= 18 && c.Perhumid && c.Rain >= 1800 && c.Aridity >= 1.15
}

func TemperateRainforestEligible(c ClimateRegime) bool {
	return c.Thermal == "temperate" && c.Coldest > 0 && c.Warmest < 24 && c.Perhumid && c.Rain >= 2400 && c.Aridity >= 1.8 && c.Continentality < .3
}

func SavannaEligible(c ClimateRegime) bool {
	return c.Thermal == "tropical" && c.Aridity >= .5 &&
		(c.Aridity < .8 || (c.LongestDrySeason >= 6 && c.Aridity < 1))
}

// Steppe types share a semi-arid water balance, not a fixed rainfall band:
// tropical evaporative demand permits more annual rainfall than cold steppe.
func steppeVegetation(c ClimateRegime) string {
	if c.Thermal == "tropical" {
		return "tropical-steppe"
	}
	if c.Coldest >= 6 && c.Warmest >= 18 {
		return "subtropical-steppe"
	}
	return "temperate-steppe"
}

func SteppeShare(shares map[string]float64) float64 {
	return shares["steppe"] + shares["temperate-steppe"] + shares["subtropical-steppe"] + shares["tropical-steppe"]
}

func vegetationV4(p *model.Planet, c ClimateRegime, lon, lat, bed float64) string {
	g := p.Grid
	switch c.Thermal {
	case "ice":
		return "ice"
	case "tundra":
		if bed >= 1800 {
			return "alpine"
		}
		return "tundra"
	}
	if bed >= 1800 && (c.GrowingMonths < 3 || (c.Warmest < 12 && (g.At("slope", lon, lat) > .12 || g.At("exposure", lon, lat) > .25))) {
		return "alpine"
	}
	if c.Moisture == "arid" {
		return "desert"
	}
	if c.Moisture == "semi-arid" {
		return steppeVegetation(c)
	}
	if c.Rainfall == "summer-dry" {
		return "shrubland"
	}
	if g.At("wetland", lon, lat) > .5 || (bed < 180 && g.At("soil", lon, lat) > 130 && g.At("slope", lon, lat) < .015) {
		return "wetland"
	}
	if c.Thermal == "tropical" {
		if TropicalRainforestEligible(c) {
			return "rainforest"
		}
		// Seasonal forest retains wet-season water availability. A failure of
		// the perennial predicate is not sufficient evidence for savanna.
		if SavannaEligible(c) {
			return "savanna"
		}
		return "tropical-woodland"
	}
	if c.Moisture == "subhumid" {
		return "grassland"
	}
	if c.Thermal == "boreal" {
		return "taiga"
	}
	if TemperateRainforestEligible(c) {
		return "temperate-rainforest"
	}
	if c.Moisture == "moist" || c.GrowingDryMonths >= 4 {
		return "woodland"
	}
	return "forest"
}
