package biosphere

import (
	"encoding/json"
	"fmt"
	"kriemhild/internal/terrain"
	"math"
	"slices"
	"sort"
	"strings"
)

// Habitat cells use the terrain's existing parent-grid coordinates, never pixels.
// Connected components respect the planetary east/west seam.
func Habitats(e *terrain.Environment, categories []string, terrainID string) ([]Habitat, string, error) {
	if e == nil {
		return nil, "", fmt.Errorf("a saved physical terrain is required")
	}
	w, h := e.Options.Columns, e.Options.Rows
	n := w * h
	if n == 0 || len(e.Fields["elevation"]) != n {
		return nil, "", fmt.Errorf("terrain elevation grid is incomplete")
	}
	raw, _ := json.Marshal(struct {
		E *terrain.Environment
		C []string
	}{e, categories})
	signature := Stable(string(raw))
	get := func(k string, i int, def float64) float64 {
		if i < len(e.Fields[k]) {
			return e.Fields[k][i]
		}
		return def
	}
	keys := make([]string, n)
	kinds := make([]string, n)
	profiles := make([]Conditions, n)
	hotCells := map[int]bool{}
	if e.Hydrology != nil {
		for _, spring := range e.Hydrology.Springs {
			if spring.Temperature >= 35 && spring.Discharge > 0 {
				hotCells[spring.Cell] = true
			}
		}
	}
	for i := 0; i < n; i++ {
		z := get("elevation", i, 0)
		t := get("temperature", i, 12)
		m := get("moisture", i, .5)
		depth := get("waterDepth", i, 0)
		sal := get("salinity", i, 0)
		medium := "terrestrial"
		hasWaterBodies := len(e.Fields["waterBody"]) == n
		marine := get("ocean", i, 0) > .5 || i < len(e.Mask) && e.Mask[i] == 0
		inland := get("lake", i, 0) > 0 || depth > 0
		if hasWaterBodies {
			marine = get("waterBody", i, 0) == 1 && depth > 0
			inland = get("waterBody", i, 0) > 1 && depth > 0
		}
		if marine {
			medium = "marine"
			depth = math.Max(depth, get("bathymetry", i, 0))
			if sal < 5 {
				sal = 35
			}
		} else if inland || get("river", i, 0) > .08 {
			medium = "freshwater"
			depth = math.Max(depth, 1)
		}
		c := Conditions{Temperature: Range{get("winter", i, t-5), get("summer", i, t+5)}, Moisture: Range{m, m}, Elevation: Range{z, z}, Depth: Range{depth, depth}, Salinity: Range{sal, sal}, Rain: get("precipitation", i, 800), Fertility: get("fertility", i, .4), Light: 1, Medium: medium, Substrates: []string{"soil"}, ClimateZones: []string{}, Seasons: []string{}}
		if e.Climate != nil && i < len(e.Climate.Cells) {
			cc := e.Climate.Cells[i]
			c.Temperature = Range{math.Inf(1), math.Inf(-1)}
			c.Moisture = Range{1, 0}
			c.Rain = 0
			for _, month := range cc.Months {
				c.Temperature.Min = math.Min(c.Temperature.Min, month.Temperature)
				c.Temperature.Max = math.Max(c.Temperature.Max, month.Temperature)
				c.Moisture.Min = math.Min(c.Moisture.Min, month.SoilMoisture)
				c.Moisture.Max = math.Max(c.Moisture.Max, month.SoilMoisture)
				c.Rain += month.Rain
			}
			if cc.Zone >= 0 && cc.Zone < len(e.Climate.Zones) {
				c.ClimateZones = append(c.ClimateZones, e.Climate.Zones[cc.Zone].Kind)
			}
			for _, s := range cc.Seasons {
				c.Seasons = append(c.Seasons, s.Name)
			}
		}
		if medium != "terrestrial" {
			c.Moisture = Range{1, 1}
			c.Substrates = []string{"sediment"}
			// Climate stores air temperature. Aquatic organisms inhabit the
			// liquid layer under seasonal ice, not subzero air above it.
			freezing := -.054 * sal
			c.Temperature.Min = math.Max(freezing, c.Temperature.Min*.65)
			c.Temperature.Max = math.Max(c.Temperature.Min, c.Temperature.Max*.8+2)
			c.Light = math.Exp(-math.Min(depth, 500) / 60)
		}
		if e.Geology != nil && i < len(e.Geology.Cells) {
			g := e.Geology.Cells[i]
			if g.Province >= 0 && g.Province < len(e.Geology.Provinces) {
				c.Substrates = append(c.Substrates, e.Geology.Provinces[g.Province].Rock)
			}
			c.Fertility = clamp((c.Fertility+g.Soil)/2, 0, 1)
		}
		if get("volcano", i, 0) > .3 || get("ash", i, 0) > .2 {
			c.Substrates = append(c.Substrates, "volcanic")
		}
		if get("dune", i, 0) > .15 {
			c.Substrates = append(c.Substrates, "sand")
		}
		tile := ""
		if i < len(categories) {
			tile = categories[i]
		}
		kind := "grassland"
		switch {
		case medium == "marine":
			kind = "open-ocean"
			if depth < 180 {
				kind = "shallow-marine"
			}
			if depth < 12 {
				kind = "intertidal"
			}
			if get("reef", i, 0) > .1 || get("reefType", i, 0) > 0 {
				kind = "reef"
			}
			if sal > 0 && sal < 28 {
				kind = "estuary"
			}
		case medium == "freshwater":
			kind = "lake"
			if get("river", i, 0) > .08 {
				kind = "river"
			}
		case hotCells[i] || get("geothermal", i, 0) > 65 && get("spring", i, 0) > 0:
			kind = "geothermal"
			c.Substrates = append(c.Substrates, "geothermal")
		case get("wetland", i, 0) > .45 || tile == "swamp":
			kind = "wetland"
		case get("oasis", i, 0) > .3:
			kind = "oasis"
		case get("glacier", i, 0) > .4 || tile == "glacier":
			kind = "polar"
		case z > 2200:
			kind = "alpine"
		case c.Temperature.Max < 12 || tile == "tundra":
			kind = "tundra"
		case tile == "desert" || tile == "dunes" || get("aridity", i, 0) > .7:
			kind = "desert"
			if get("dune", i, 0) > .15 {
				kind = "desert-dunes"
			}
		case tile == "jungle" || tile == "rainforest" || (t > 20 && c.Rain > 1600):
			kind = "tropical-forest"
		case tile == "forest" || tile == "taiga" || tile == "woodland" || (m > .65 && c.Rain > 400):
			kind = "temperate-forest"
			if t < 5 {
				kind = "boreal-forest"
			}
		case get("floodplain", i, 0) > .3:
			kind = "floodplain"
		}
		c.SeasonalAmplitude = c.Temperature.Max - c.Temperature.Min
		keys[i] = kind + "/" + fmt.Sprint(math.Floor(c.Temperature.Min/15)) + "/" + fmt.Sprint(math.Floor(c.Temperature.Max/15))
		kinds[i] = kind
		profiles[i] = c
	}
	visited := make([]bool, n)
	out := []Habitat{}
	for start := 0; start < n; start++ {
		if visited[start] {
			continue
		}
		kind := kinds[start]
		cells := []int{start}
		visited[start] = true
		for j := 0; j < len(cells); j++ {
			i := cells[j]
			x, y := i%w, i/w
			ns := []int{}
			if x > 0 {
				ns = append(ns, i-1)
			} else if e.Geography != nil && e.Geography.WrapX {
				ns = append(ns, i+w-1)
			}
			if x+1 < w {
				ns = append(ns, i+1)
			} else if e.Geography != nil && e.Geography.WrapX {
				ns = append(ns, i-w+1)
			}
			if y > 0 {
				ns = append(ns, i-w)
			}
			if y+1 < h {
				ns = append(ns, i+w)
			}
			for _, k := range ns {
				if !visited[k] && keys[k] == keys[start] {
					visited[k] = true
					cells = append(cells, k)
				}
			}
		}
		sort.Ints(cells)
		c := profiles[cells[0]]
		c.Rain = 0
		c.Fertility = 0
		c.Light = 0
		c.ClimateZones = nil
		c.Seasons = nil
		c.Substrates = nil
		for _, i := range cells {
			p := profiles[i]
			mergeRange := func(a *Range, b Range) { a.Min = math.Min(a.Min, b.Min); a.Max = math.Max(a.Max, b.Max) }
			mergeRange(&c.Temperature, p.Temperature)
			mergeRange(&c.Moisture, p.Moisture)
			mergeRange(&c.Elevation, p.Elevation)
			mergeRange(&c.Depth, p.Depth)
			mergeRange(&c.Salinity, p.Salinity)
			c.Rain += p.Rain
			c.Fertility += p.Fertility
			c.Light += p.Light
			for _, pair := range []struct {
				dst *[]string
				src []string
			}{{&c.ClimateZones, p.ClimateZones}, {&c.Substrates, p.Substrates}, {&c.Seasons, p.Seasons}} {
				for _, s := range pair.src {
					if !slices.Contains(*pair.dst, s) {
						*pair.dst = append(*pair.dst, s)
					}
				}
			}
		}
		count := float64(len(cells))
		c.Rain /= count
		c.Fertility /= count
		c.Light /= count
		c.SeasonalAmplitude = c.Temperature.Max - c.Temperature.Min
		strata := []string{"surface"}
		if strings.Contains(kind, "forest") {
			strata = []string{"canopy", "understory", "floor"}
		}
		for _, stratum := range strata {
			local := c
			if stratum == "understory" {
				local.Light *= .4
			}
			if stratum == "floor" {
				local.Light *= .15
			}
			// Stable within an environmental revision. Changed cell sets or conditions
			// receive a new revision ID so saved occurrences keep their old extent.
			region := Stable(terrainID, kind, stratum, fmt.Sprint(start))
			revision, _ := json.Marshal(struct {
				Cells      []int
				Conditions Conditions
			}{cells, local})
			id := Stable(region, string(revision))
			out = append(out, Habitat{ID: id, RegionID: region, Name: strings.ReplaceAll(kind, "-", " ") + " · " + stratum + " " + fmt.Sprint(start), Kind: kind, Stratum: stratum, Cells: cells, Conditions: local, Description: fmt.Sprintf("Connected %s habitat; seasonal temperature %.1f–%.1f °C, annual precipitation %.0f mm, elevation %.0f–%.0f m. %s.", kind, c.Temperature.Min, c.Temperature.Max, c.Rain, c.Elevation.Min, c.Elevation.Max, strings.Join(c.Substrates, ", "))})
		}
	}
	return out, signature, nil
}

// Hard compatibility is never relaxed implicitly. Speculative tolerance changes
// must be explicit in a species or rule, rather than disabling water/host checks.
func Suitability(b Lifeform, h Habitat) (float64, string) {
	r, c := b.Requirements, h.Conditions
	if r.Medium != "any" && r.Medium != c.Medium && !(r.Medium == "amphibious" && (c.Medium == "terrestrial" || c.Medium == "freshwater")) {
		return 0, "incompatible terrestrial / freshwater / marine requirements"
	}
	for _, v := range []struct {
		name           string
		actual, wanted Range
	}{{"seasonal temperature", c.Temperature, r.Temperature}, {"elevation", c.Elevation, r.Elevation}, {"depth", c.Depth, r.Depth}, {"salinity", c.Salinity, r.Salinity}} {
		if v.actual.Min < v.wanted.Min || v.actual.Max > v.wanted.Max {
			return 0, "outside " + v.name + " survival limits"
		}
	}
	if c.Moisture.Max < r.Moisture.Min || c.Moisture.Min > r.Moisture.Max {
		return 0, "insufficient moisture compatibility"
	}
	if c.Light < r.MinLight {
		return 0, "insufficient sunlight"
	}
	if len(r.Habitats) > 0 && !slices.Contains(r.Habitats, h.Kind) {
		return 0, "outside native habitat types"
	}
	if len(r.Substrates) > 0 {
		ok := false
		for _, s := range r.Substrates {
			ok = ok || slices.Contains(c.Substrates, s)
		}
		if !ok {
			return 0, "essential substrate absent"
		}
	}
	thermal := 1 - math.Abs((c.Temperature.Min+c.Temperature.Max)/2-(r.Temperature.Min+r.Temperature.Max)/2)/math.Max(1, r.Temperature.Max-r.Temperature.Min)
	return clamp(.4+.35*thermal+.15*c.Fertility+.1*(c.Moisture.Min+c.Moisture.Max)/2, .1, 1), fmt.Sprintf("Compatible %s habitat with %.0f–%.0f °C seasons, %.0f mm annual rain and %s substrate", h.Kind, c.Temperature.Min, c.Temperature.Max, c.Rain, strings.Join(c.Substrates, ", "))
}
