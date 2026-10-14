package generate

import "kriemhild/internal/terrain/model"

// BiomeAt keeps the original aridity thresholds (annual mm / annual mm), then
// distinguishes seasonal and maritime vegetation. No target biome shares enter it.
func BiomeAt(p *model.Planet, lon, lat, bed float64, wet bool) string {
	g := p.Grid
	t := g.At("temperature", lon, lat) - (bed-g.At("bed", lon, lat))*.0065
	rain, pet := g.At("precipitation", lon, lat), g.At("pet", lon, lat)
	base := Biome(t, rain, pet, bed, wet)
	if p.Spec.Version == "continuous-1" || wet || base == "ice" || base == "tundra" || base == "alpine" || base == "desert" || base == "steppe" {
		return base
	}
	ratio := rain / max(100, pet)
	dry := g.At("dryMonths", lon, lat)
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
