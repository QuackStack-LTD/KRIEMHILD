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
	allShore := []int{}
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
		allShore = append(allShore, shore...)
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
		d := math.Inf(1)
		for _, j := range allShore {
			d = math.Min(d, distance(i, j))
		}
		if d >= e.Options.oceanGap()*.5 {
			open++
		}
	}
	r.OpenOceanFraction = float64(open) / float64(max(1, sea))
	r.LargestOceanFraction = float64(largest) / float64(max(1, sea))
}

type LatitudeBandMetric struct {
	South      float64 `json:"absoluteLatitudeFrom"`
	North      float64 `json:"absoluteLatitudeTo"`
	Land       int     `json:"landCells"`
	Dry        int     `json:"desertOrSteppeCells"`
	DryPercent float64 `json:"drylandPercent"`
	Rain       float64 `json:"meanAnnualRainMM"`
	AI         float64 `json:"meanAridityIndex"`
}
type ClimateDistribution struct {
	Bands              []LatitudeBandMetric `json:"latitudeBands"`
	Biomes             map[string]int       `json:"biomeAreas"`
	Kinds              map[string]int       `json:"climateAreas"`
	DryComponents      []int                `json:"drylandComponentAreas"`
	SmallDryComponents int                  `json:"drylandComponentsAtMostTwoCells"`
	SmallDryCells      int                  `json:"cellsInSmallDrylandComponents"`
}

func (e *Environment) ClimateDiagnostics() ClimateDistribution {
	r := ClimateDistribution{Biomes: map[string]int{}, Kinds: map[string]int{}}
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
	return r
}
