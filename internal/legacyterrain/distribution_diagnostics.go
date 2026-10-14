package terrain

import (
	"math"
	"sort"
)

func (e *Environment) measureContinents(r *LandShapeReport, land, water [][]int) {
	w, n := e.Options.Columns, len(e.Mask)
	r.Layout = e.Layout
	r.MajorThreshold = max(16, int(math.Ceil(float64(n)*.01)))
	sort.Slice(land, func(i, j int) bool { return len(land[i]) > len(land[j]) })
	shores := [][]int{}
	for _, g := range land {
		shore := []int{}
		for _, i := range g {
			for _, j := range e.neighbors(i) {
				if e.Mask[j] == 0 {
					shore = append(shore, i)
					break
				}
			}
		}
		if len(g) >= r.MajorThreshold {
			r.MajorAreas = append(r.MajorAreas, len(g))
			shores = append(shores, shore)
		}
	}
	r.MajorCount = len(shores)
	distance := func(a, b int) float64 {
		dx := math.Abs(float64(a%w - b%w))
		if e.Geography != nil && e.Geography.WrapX {
			dx = math.Min(dx, float64(w)-dx)
		}
		return math.Hypot(dx, float64(a/w-b/w))
	}
	r.MinimumGap = math.Inf(1)
	for a := range shores {
		for b := a + 1; b < len(shores); b++ {
			gap := math.Inf(1)
			for _, i := range shores[a] {
				for _, j := range shores[b] {
					gap = math.Min(gap, math.Max(0, distance(i, j)-1))
				}
			}
			r.OceanGaps = append(r.OceanGaps, OceanGapMetric{a, b, gap})
			r.MinimumGap = math.Min(r.MinimumGap, gap)
		}
	}
	if len(shores) < 2 {
		r.MinimumGap = 0
	}
	sea, open, largest := 0, 0, 0
	landMask := make([]bool, n)
	for i, v := range e.Mask {
		landMask[i] = v != 0
	}
	shoreDistance := euclideanDistance(landMask, e.Options.Columns, e.Options.Rows, e.Geography != nil && e.Geography.WrapX)
	for _, g := range water {
		largest = max(largest, len(g))
	}
	for i, v := range e.Mask {
		if e.climateField("oceanReserve", i) > 0 && v != 0 {
			r.ReservedLandCells++
		}
		if v != 0 {
			continue
		}
		sea++
		d := shoreDistance[i]
		if d >= e.Options.oceanGap()*.5 {
			open++
		}
	}
	r.OpenOceanFraction = float64(open) / float64(max(1, sea))
	r.LargestOceanFraction = float64(largest) / float64(max(1, sea))
}

type LatitudeBandMetric struct {
	LargestDryRegion int     `json:"largestDrylandRegionCells"`
	South            float64 `json:"absoluteLatitudeFrom"`
	North            float64 `json:"absoluteLatitudeTo"`
	Land             int     `json:"landCells"`
	Dry              int     `json:"desertOrSteppeCells"`
	DryPercent       float64 `json:"drylandPercent"`
	Rain             float64 `json:"meanAnnualRainMM"`
	AI               float64 `json:"meanAridityIndex"`
}
type ClimateDistribution struct {
	LandCells           int                  `json:"landCells"`
	BiomeShares         map[string]float64   `json:"biomeGroupPercent"`
	LargestBiomeRegions map[string]int       `json:"largestBiomeGroupRegions"`
	Bands               []LatitudeBandMetric `json:"latitudeBands"`
	Biomes              map[string]int       `json:"biomeAreas"`
	Kinds               map[string]int       `json:"climateAreas"`
	DryComponents       []int                `json:"drylandComponentAreas"`
	SmallDryComponents  int                  `json:"drylandComponentsAtMostTwoCells"`
	SmallDryCells       int                  `json:"cellsInSmallDrylandComponents"`
}

func (e *Environment) ClimateDiagnostics() ClimateDistribution {
	r := ClimateDistribution{Biomes: map[string]int{}, Kinds: map[string]int{}}
	r.BiomeShares = map[string]float64{}
	r.LargestBiomeRegions = map[string]int{}
	bounds := []float64{0, 10, 20, 35, 50, 65, 90}
	for k := 1; k < len(bounds); k++ {
		r.Bands = append(r.Bands, LatitudeBandMetric{South: bounds[k-1], North: bounds[k]})
	}
	dry := make([]bool, len(e.Mask))
	for i, land := range e.Mask {
		if land == 0 {
			continue
		}
		kind := e.ClimateKind(i)
		r.Kinds[kind]++
		r.Biomes[e.BiomeAt(i)]++
		r.LandCells++
		dry[i] = kind == "desert" || kind == "steppe"
		lat := math.Abs(e.climateField("latitude", i))
		for k := range r.Bands {
			b := &r.Bands[k]
			if lat >= b.South && lat <= b.North {
				b.Land++
				if dry[i] {
					b.Dry++
				}
				b.Rain += e.climateField("precipitation", i)
				b.AI += e.climateField("aridityIndex", i)
				break
			}
		}
	}
	for k := range r.Bands {
		b := &r.Bands[k]
		b.DryPercent = 100 * float64(b.Dry) / float64(max(1, b.Land))
		b.Rain /= float64(max(1, b.Land))
		b.AI /= float64(max(1, b.Land))
	}
	for i, v := range dry {
		if !v {
			continue
		}
		q := []int{i}
		dry[i] = false
		for head := 0; head < len(q); head++ {
			for _, j := range e.neighbors(q[head]) {
				if dry[j] {
					dry[j] = false
					q = append(q, j)
				}
			}
		}
		r.DryComponents = append(r.DryComponents, len(q))
		if len(q) <= 2 {
			r.SmallDryComponents++
			r.SmallDryCells += len(q)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(r.DryComponents)))
	groups := make([]string, len(e.Mask))
	for i, v := range e.Mask {
		if v == 0 {
			continue
		}
		b := e.BiomeAt(i)
		group := "other"
		switch b {
		case "desert", "cold_desert", "dunes":
			group = "desert"
		case "steppe", "scrubland":
			group = "semiarid"
		case "grass", "savanna", "mediterranean":
			group = "grassland-woodland"
		case "forest", "jungle", "taiga":
			group = "forest"
		case "tundra", "snow", "alpine":
			group = "cold"
		}
		groups[i] = group
		r.BiomeShares[group] += 100 / float64(max(1, r.LandCells))
	}
	seen := make([]bool, len(groups))
	for i, g := range groups {
		if g == "" || seen[i] {
			continue
		}
		q := []int{i}
		seen[i] = true
		for head := 0; head < len(q); head++ {
			for _, j := range e.neighbors(q[head]) {
				if !seen[j] && groups[j] == g {
					seen[j] = true
					q = append(q, j)
				}
			}
		}
		r.LargestBiomeRegions[g] = max(r.LargestBiomeRegions[g], len(q))
	}
	for k := range r.Bands {
		b := &r.Bands[k]
		clear(seen)
		eligible := func(i int) bool {
			lat := math.Abs(e.climateField("latitude", i))
			kind := e.ClimateKind(i)
			return e.Mask[i] != 0 && lat >= b.South && lat <= b.North && (kind == "desert" || kind == "steppe")
		}
		for i := range e.Mask {
			if seen[i] || !eligible(i) {
				continue
			}
			q := []int{i}
			seen[i] = true
			for head := 0; head < len(q); head++ {
				for _, j := range e.neighbors(q[head]) {
					if !seen[j] && eligible(j) {
						seen[j] = true
						q = append(q, j)
					}
				}
			}
			b.LargestDryRegion = max(b.LargestDryRegion, len(q))
		}
	}
	return r
}

func (e *Environment) measureEmergedIslands(r *LandShapeReport) {
	r.IslandSizeTiers = map[string]int{"rocks": 0, "small": 0, "medium": 0, "large": 0}
	f := e.Fields["islandGroup"]
	if len(f) != len(e.Mask) {
		return
	}
	seen := make([]bool, len(f))
	groups := map[int]bool{}
	for i, v := range f {
		if v == 0 || e.Mask[i] == 0 {
			continue
		}
		r.EmergedIslandCells++
		groups[int(v)] = true
		if seen[i] {
			continue
		}
		q := []int{i}
		seen[i] = true
		for head := 0; head < len(q); head++ {
			for _, j := range e.neighbors(q[head]) {
				if !seen[j] && f[j] > 0 && e.Mask[j] != 0 {
					seen[j] = true
					q = append(q, j)
				}
			}
		}
		r.EmergedIslandSizes = append(r.EmergedIslandSizes, len(q))
		tier := "large"
		if len(q) <= 2 {
			tier = "rocks"
		} else if len(q) <= 8 {
			tier = "small"
		} else if len(q) <= 31 {
			tier = "medium"
		}
		r.IslandSizeTiers[tier]++
	}
	r.VisibleArchipelagos = len(groups)
	sort.Sort(sort.Reverse(sort.IntSlice(r.EmergedIslandSizes)))
}
