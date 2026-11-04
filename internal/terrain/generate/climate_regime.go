package generate

import "kriemhild/internal/terrain/model"

// ClimateRegime describes temperature, moisture and rainfall timing, not a
// vegetation assignment. Version 1's existing thresholds are deliberately
// retained; terrain/wetlands and alpine exposure belong to vegetation selection.
type ClimateRegime struct {
	Thermal   string  `json:"thermal"`
	Moisture  string  `json:"moisture"`
	Rainfall  string  `json:"rainfall"`
	Coldest   float64 `json:"coldest"`
	Warmest   float64 `json:"warmest"`
	Rain      float64 `json:"annualRain"`
	PET       float64 `json:"annualPET"`
	Aridity   float64 `json:"aridity"`
	DryMonths float64 `json:"dryMonths"`
	Perhumid  bool    `json:"perhumid"`
}

func ClimateRegimeAt(p *model.Planet, lon, lat, bed float64) ClimateRegime {
	return climateRegimeAt(p, lon, lat, bed, false)
}

func climateRegimeAt(p *model.Planet, lon, lat, bed float64, vegetationOnly bool) ClimateRegime {
	g := p.Grid
	delta := p.AtmosphericLapse(lon, lat, bed)
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
	c := ClimateRegime{Coldest: season(0) - delta, Warmest: season(1) - delta, Thermal: "temperate", Rainfall: "seasonal"}
	switch {
	case c.Warmest < 0:
		c.Thermal = "ice"
	case c.Warmest < 10:
		c.Thermal = "tundra"
	case c.Coldest >= 18:
		c.Thermal = "tropical"
	case c.Coldest < -3 && c.Warmest < 18:
		c.Thermal = "boreal"
	}
	// Vegetation is already determined below the growing-season cold limit.
	// Keep the original cheap cold path while diagnostics still obtain all axes.
	if vegetationOnly && (c.Thermal == "ice" || c.Thermal == "tundra") {
		return c
	}
	c.Rain, c.PET = g.At("precipitation", lon, lat), g.At("pet", lon, lat)
	c.Aridity = c.Rain / max(100, c.PET)
	switch {
	case c.Aridity < .22:
		c.Moisture = "arid"
	case c.Aridity < .5:
		c.Moisture = "semi-arid"
	case c.Aridity < .8:
		c.Moisture = "subhumid"
	case c.Aridity < 1:
		c.Moisture = "moist"
	default:
		c.Moisture = "humid"
	}
	if c.Coldest > -3 && c.Coldest < 18 && c.Warmest > 18 {
		driest := season(3)
		if driest < 40 && driest*3 < season(4) && season(6) > c.Rain*.55 {
			c.Rainfall = "summer-dry"
		}
	}
	c.DryMonths = g.At("dryMonths", lon, lat)
	// Avoid a monthly-minimum lookup where no rainforest outcome is possible.
	if c.Rain >= 1800 && c.Aridity >= 1 && c.DryMonths < 2 {
		c.Perhumid = season(2) >= 60
		if c.Perhumid {
			c.Rainfall = "perennial-wet"
		}
	}
	return c
}

func vegetationAt(p *model.Planet, c ClimateRegime, lon, lat, bed float64) string {
	g := p.Grid
	if c.Thermal == "ice" {
		return "ice"
	}
	if c.Thermal == "tundra" {
		if bed >= 1800 {
			return "alpine"
		}
		return "tundra"
	}
	if bed >= 1800 && c.Warmest < 12 && (g.At("slope", lon, lat) > .12 || g.At("exposure", lon, lat) > .25) {
		return "alpine"
	}
	if c.Moisture == "arid" {
		return "desert"
	}
	if c.Rainfall == "summer-dry" {
		return "shrubland"
	}
	if c.Moisture == "semi-arid" {
		return "steppe"
	}
	if g.At("wetland", lon, lat) > .5 || (bed < 180 && g.At("soil", lon, lat) > 130 && g.At("slope", lon, lat) < .015) {
		return "wetland"
	}
	if c.Thermal == "tropical" {
		if c.Perhumid {
			return "rainforest"
		}
		if c.Aridity < .8 || c.DryMonths > 5 {
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
	if c.Perhumid && c.Aridity >= 1.5 && c.Coldest > 0 && c.Warmest < 24 {
		return "temperate-rainforest"
	}
	if c.Moisture == "moist" {
		return "woodland"
	}
	return "forest"
}
