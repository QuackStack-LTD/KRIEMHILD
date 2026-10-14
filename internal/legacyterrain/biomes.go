package terrain

import (
	_ "embed"
	"encoding/json"
	"math"
)

// Biome IDs are append-only persisted enum values. The renderer imports the
// same catalog; ecological thresholds live only in this server classifier.
//
//go:embed biomes.json
var biomeJSON []byte

const (
	desertAI      = .25
	steppeAI      = .50
	drySubhumidAI = .65
)

type SurfaceBiome struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Color string   `json:"color"`
	Types []string `json:"types"`
}

var Biomes = func() []SurfaceBiome {
	var v []SurfaceBiome
	if err := json.Unmarshal(biomeJSON, &v); err != nil {
		panic(err)
	}
	return v
}()

func biomeIndex(id string) int {
	for i, b := range Biomes {
		if b.ID == id {
			return i
		}
	}
	return 1
}
func (e *Environment) ClimateKind(i int) string {
	if e.Climate == nil || i < 0 || i >= len(e.Climate.Cells) {
		return ""
	}
	zone := e.Climate.Cells[i].Zone
	if zone < 0 || zone >= len(e.Climate.Zones) {
		return ""
	}
	return e.Climate.Zones[zone].Kind
}
func (e *Environment) BiomeAt(i int) string {
	values := e.Fields["biome"]
	if i < 0 || i >= len(values) {
		return ""
	}
	index := int(values[i])
	if index < 0 || index >= len(Biomes) {
		return ""
	}
	return Biomes[index].ID
}

func (e *Environment) surfaceBiome(i int, kind string, c ClimateCell, ai float64) string {
	f := e.climateField
	if f("waterBody", i) > 0 {
		return "water"
	}
	if cover := int(f("authoredCover", i)) - 1; cover >= 0 && cover < len(Biomes) {
		return Biomes[cover].ID
	}
	if f("lava", i) > .2 {
		return "lava"
	}
	if f("ash", i) > .2 {
		return "ash"
	}
	lo, hi, mean, growingMoisture, growing := math.Inf(1), math.Inf(-1), 0., 0., 0.
	for _, m := range c.Months {
		lo = math.Min(lo, m.Temperature)
		hi = math.Max(hi, m.Temperature)
		mean += m.Temperature / 12
		if m.Temperature > 5 {
			growing++
			growingMoisture += m.SoilMoisture
		}
	}
	if growing > 0 {
		growingMoisture /= growing
	}
	if kind == "polar" || f("glacier", i) > .45 && hi < 5 {
		return "snow"
	}
	if kind == "alpine" {
		return "alpine"
	}
	// Bare summits, not ordinary wooded slopes, replace the cover biome.
	if f("elevation", i) > 2800 && f("mountainCore", i) > .45 && f("slope", i) > .35 {
		return "mountain"
	}
	if f("beach", i) > .8 {
		return "sand"
	}
	if f("wetland", i) > .6 && hi > 5 {
		return "swamp"
	}
	if kind == "desert" && f("oasis", i) > .5 {
		return "oasis"
	}
	if kind == "desert" && f("dune", i) > .13 && f("slope", i) < .25 {
		return "dunes"
	}
	if kind == "desert" && f("mesa", i) > .65 && f("elevation", i) > 400 {
		return "mesa"
	}
	switch kind {
	case "tundra":
		return "tundra"
	case "desert":
		if mean < 8 {
			return "cold_desert"
		}
		return "desert"
	case "steppe":
		return "steppe"
	case "boreal":
		if ai >= drySubhumidAI {
			return "taiga"
		}
		return "grass"
	case "mediterranean":
		return "mediterranean"
	case "rainforest":
		return "jungle"
	case "monsoon", "tropical-seasonal":
		if growingMoisture > .65 && ai > 1.2 {
			return "jungle"
		}
		return "savanna"
	}
	if ai < drySubhumidAI && mean > 10 && hi > 18 {
		return "scrubland"
	}
	if lo > 12 && hi > 24 && growingMoisture < .5 {
		return "savanna"
	}
	if ai > .85 && growingMoisture > .35 {
		return "forest"
	}
	return "grass"
}

func (e *Environment) biomeCandidates(i int) []string {
	if id := e.BiomeAt(i); id != "" {
		return Biomes[biomeIndex(id)].Types
	}
	// Transient old worlds may have a climate layer without a stored surface
	// classification. Derive it only for explicit terrain selection, not on load.
	if kind := e.ClimateKind(i); kind != "" {
		c := e.Climate.Cells[i]
		rain, pet := 0., 0.
		for _, m := range c.Months {
			rain += m.Rain
			pet += m.Evaporation
		}
		return Biomes[biomeIndex(e.surfaceBiome(i, kind, c, rain/math.Max(1, pet)))].Types
	}
	return nil
}
