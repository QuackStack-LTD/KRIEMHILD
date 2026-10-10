package terrain

import (
	"math"
	"sort"
)

// Lengths and areas are measured in base-grid cells, not assumed kilometres.
// Every reach belongs to one river; tributaries end at their receiving river.
type RiverSystem struct {
	ID            string   `json:"id"`
	Class         string   `json:"class"`
	Reaches       []string `json:"reaches"`
	Source        int      `json:"source"`
	Mouth         int      `json:"mouth"`
	Downstream    string   `json:"downstream"`
	Tributaries   []string `json:"tributaries"`
	Watershed     string   `json:"watershed"`
	Length        float64  `json:"length"`
	CatchmentArea float64  `json:"catchmentArea"`
	Discharge     float64  `json:"discharge"`
}
type DrainageBasin struct {
	ID          string  `json:"id"`
	Outlet      int     `json:"outlet"`
	Destination string  `json:"destination"`
	Area        int     `json:"area"`
	LongestPath float64 `json:"longestPath"`
	Discharge   float64 `json:"discharge"`
}
type DrainageTarget struct {
	Landmass           int `json:"landmass"`
	LandCells          int `json:"landCells"`
	SupportedCells     int `json:"supportedCells"`
	TargetCells        int `json:"targetCells"`
	ActualCells        int `json:"actualCells"`
	TargetLakeCells    int `json:"targetLakeCells"`
	LakeCells          int `json:"lakeCells"`
	TargetWetlandCells int `json:"targetWetlandCells"`
	WetlandCells       int `json:"wetlandCells"`
}
type RiverClassStats struct {
	Class  string  `json:"class"`
	Count  int     `json:"count"`
	Length float64 `json:"length"`
}
type HydroDistribution struct {
	Min    float64 `json:"min"`
	Median float64 `json:"median"`
	P90    float64 `json:"p90"`
	Max    float64 `json:"max"`
}
type HydroStatistics struct {
	LengthUnits           string            `json:"lengthUnits"`
	AreaUnits             string            `json:"areaUnits"`
	Classes               []RiverClassStats `json:"classes"`
	LandCells             int               `json:"landCells"`
	DrainageCells         int               `json:"drainageCells"`
	CoveragePercent       float64           `json:"coveragePercent"`
	TargetCoveragePercent float64           `json:"targetCoveragePercent"`
	BasinCount            int               `json:"basinCount"`
	Lakes                 int               `json:"lakes"`
	Wetlands              int               `json:"wetlands"`
	Springs               int               `json:"springs"`
	InvalidDestinations   int               `json:"invalidDestinations"`
	LowlandHeadwaters     int               `json:"lowlandHeadwaters"`
	Lengths               HydroDistribution `json:"lengths"`
	Catchments            HydroDistribution `json:"catchments"`
	BasinSizes            HydroDistribution `json:"basinSizes"`
	Notes                 []string          `json:"notes"`
}

func (e *Environment) hydroLandmasses() ([]int, []int) {
	labels := make([]int, len(e.Mask))
	sizes := []int{0}
	for i := range labels {
		if labels[i] > 0 || e.get("ocean", i) > 0 {
			continue
		}
		id := len(sizes)
		q := []int{i}
		labels[i] = id
		for head := 0; head < len(q); head++ {
			for _, j := range nb(q[head], e.Options.Columns, e.Options.Rows) {
				if labels[j] == 0 && e.get("ocean", j) == 0 {
					labels[j] = id
					q = append(q, j)
				}
			}
		}
		sizes = append(sizes, len(q))
	}
	return labels, sizes
}

func (e *Environment) hydroStep(i, j int) float64 {
	w := e.Options.Columns
	return math.Hypot(float64(i%w-j%w), float64(i/w-j/w))
}

// Selection ranks whole catchments, anchored to supported upstream sources.
// Coverage targets only rank supported channels; they never manufacture water.
func (e *Environment) selectBasinChannels() []bool {
	n := len(e.Mask)
	visible := make([]bool, n)
	f := e.get
	labels, sizes := e.hydroLandmasses()
	candidates := make([][]int, len(sizes))
	humidity := make([]float64, len(sizes))
	land := make([]int, len(sizes))
	// Reverse drainage order gives route length, including lake transit.
	order := e.accumulateWater()
	for k := len(order) - 1; k >= 0; k-- {
		i := order[k]
		d := 0.
		if j := int(f("flow", i)); j >= 0 {
			d = e.hydroStep(i, j) + f("drainageDistance", j)
		}
		e.set("drainageDistance", i, d)
	}

	// Propagate a real headwater through the DAG. Downstream accumulated flow
	// selects a whole river, never an isolated lowland/coastal fragment.
	origin := make([]int, n)
	for i := range origin {
		origin[i] = -1
	}
	for _, i := range order {
		if origin[i] < 0 && e.supportedRiverHeadwater(i) {
			origin[i] = i
		}
		if j := int(f("flow", i)); j >= 0 && origin[i] >= 0 && (f("lake", i) == 0 || f("accumulation", i) > f("hydroLoss", i)) {
			previous := origin[j]
			if previous < 0 || f("drainageDistance", origin[i]) > f("drainageDistance", previous) {
				origin[j] = origin[i]
			}
		}
	}
	resolution := math.Max(1, float64(n)/16000)
	for i, id := range labels {
		if id == 0 || f("waterBody", i) > 0 {
			continue
		}
		land[id]++
		humidity[id] += clamp(f("runoff", i)/.6, 0, 1)
		// Even an ephemeral channel needs a concentrated contributing area and
		// actual water. Local aridity does not erase a wet upstream catchment.
		if origin[i] < 0 || f("flow", i) < 0 || f("catchmentArea", i) < math.Max(3, 3*resolution) || f("accumulation", i) < .25*resolution {
			continue
		}
		candidates[id] = append(candidates[id], i)
	}
	trace := func(start int) {
		start = origin[start]
		for at, k := start, 0; at >= 0 && k < n; k++ {
			if visible[at] {
				break
			}
			if f("waterBody", at) == 0 {
				visible[at] = true
			}
			if f("lake", at) > 0 && f("accumulation", at) <= f("hydroLoss", at) {
				break
			}
			at = int(f("flow", at))
		}
	}
	e.Hydrology.Targets = nil
	for id := 1; id < len(sizes); id++ {
		wet := humidity[id] / math.Max(1, float64(land[id]))
		// Dense maps need fewer channel cells per unit area at the same physical
		// scale. Humid land supports a richer hierarchy than dry land.
		target := int(math.Ceil(float64(land[id]) * (.055 + .18*wet) / math.Sqrt(resolution)))
		list := candidates[id]
		sort.SliceStable(list, func(a, b int) bool {
			score := func(i int) float64 {
				return math.Log1p(f("accumulation", i)) * math.Sqrt(f("catchmentArea", i)) * (1 + math.Log1p(f("drainageDistance", i)))
			}
			sa, sb := score(list[a]), score(list[b])
			if sa == sb {
				return list[a] < list[b]
			}
			return sa > sb
		})
		// Keep every substantial basin even after the local-stream target is met.
		majorArea := math.Max(20, float64(sizes[id])*.025)
		for _, i := range list {
			if f("catchmentArea", i) >= majorArea && f("accumulation", i) >= 2*resolution {
				trace(i)
			}
		}
		count := 0
		for i, label := range labels {
			if label == id && visible[i] && f("flow", i) >= 0 {
				count++
			}
		}
		for _, i := range list {
			if count >= target {
				break
			}
			if visible[i] {
				continue
			}
			before := 0
			for at, k := origin[i], 0; at >= 0 && k < n && !visible[at]; k++ {
				if labels[at] == id && f("waterBody", at) == 0 && f("flow", at) >= 0 {
					before++
				}
				at = int(f("flow", at))
			}
			trace(i)
			count += before
		}
		e.Hydrology.Targets = append(e.Hydrology.Targets, DrainageTarget{Landmass: id, LandCells: land[id], SupportedCells: len(list), TargetCells: target, ActualCells: count, TargetLakeCells: int(float64(land[id]) * .008 * wet), TargetWetlandCells: int(float64(land[id]) * .025 * wet)})
	}
	return visible
}

func distribution(values []float64) HydroDistribution {
	if len(values) == 0 {
		return HydroDistribution{}
	}
	sort.Float64s(values)
	return HydroDistribution{Min: values[0], Median: values[(len(values)-1)/2], P90: values[int(float64(len(values)-1)*.9)], Max: values[len(values)-1]}
}

func (e *Environment) summarizeRiverSystems() {
	h, f := e.Hydrology, e.get
	n := len(e.Mask)
	labels, sizes := e.hydroLandmasses()
	byCell := map[int]int{}
	for k, r := range h.Reaches {
		byCell[r.From] = k
	}
	// Follow downstream through a lake's stored outlet, retaining the same
	// continental system on both sides without drawing a line across the lake.
	nextReach := func(i int) int {
		at := int(f("flow", i))
		for k := 0; at >= 0 && k < n; k++ {
			if _, ok := byCell[at]; ok {
				return at
			}
			at = int(f("flow", at))
		}
		return -1
	}
	next := map[int]int{}
	incoming := map[int][]int{}
	for _, r := range h.Reaches {
		j := nextReach(r.From)
		next[r.From] = j
		if j >= 0 {
			incoming[j] = append(incoming[j], r.From)
		}
	}
	heads := []int{}
	for _, r := range h.Reaches {
		if len(incoming[r.From]) == 0 {
			heads = append(heads, r.From)
		}
	}
	sort.SliceStable(heads, func(a, b int) bool {
		x, y := heads[a], heads[b]
		if f("drainageDistance", x) == f("drainageDistance", y) {
			return x < y
		}
		return f("drainageDistance", x) > f("drainageDistance", y)
	})
	owner := map[int]int{}
	h.Rivers = nil
	for _, head := range heads {
		river := RiverSystem{ID: e.hydroID("river-system", head), Source: head, Watershed: e.hydroID("watershed", int(f("watershed", head))), Class: "stream"}
		index := len(h.Rivers)
		for at, k := head, 0; at >= 0 && k < n; k++ {
			if parent, ok := owner[at]; ok {
				river.Downstream = h.Rivers[parent].ID
				break
			}
			r := h.Reaches[byCell[at]]
			owner[at] = index
			river.Reaches = append(river.Reaches, r.ID)
			river.Mouth = r.To
			river.Downstream = r.Downstream
			river.Length += e.hydroStep(r.From, r.To)
			river.CatchmentArea = math.Max(river.CatchmentArea, f("catchmentArea", r.From))
			river.Discharge = r.Discharge
			at = next[at]
		}
		area := float64(sizes[labels[head]])
		if river.CatchmentArea >= math.Max(20, area*.025) && river.Length >= math.Max(6, math.Sqrt(area)*.22) && river.Discharge >= 2 {
			river.Class = "major"
		} else if river.CatchmentArea >= math.Max(8, area*.006) && river.Length >= 3 && river.Discharge >= .8 {
			river.Class = "regional"
		}
		h.Rivers = append(h.Rivers, river)
	}
	riverIndex := map[string]int{}
	for k, r := range h.Rivers {
		riverIndex[r.ID] = k
	}
	for _, r := range h.Rivers {
		if parent, ok := riverIndex[r.Downstream]; ok {
			h.Rivers[parent].Tributaries = append(h.Rivers[parent].Tributaries, r.ID)
		}
	}
	for cell, index := range owner {
		r := &h.Reaches[byCell[cell]]
		r.RiverID = h.Rivers[index].ID
		r.Class = h.Rivers[index].Class
		class := 1.
		if r.Class == "regional" {
			class = 2
		}
		if r.Class == "major" {
			class = 3
		}
		e.set("riverClass", cell, class)
		e.set("riverSystem", cell, float64(index+1))
	}
	basins := map[int]*DrainageBasin{}
	for i := range e.Mask {
		if f("ocean", i) > 0 {
			continue
		}
		root := int(f("watershed", i))
		b := basins[root]
		if b == nil {
			b = &DrainageBasin{ID: e.hydroID("watershed", root), Outlet: root, Destination: e.waterRef(root), Discharge: f("accumulation", root)}
			basins[root] = b
		}
		b.Area++
		b.LongestPath = math.Max(b.LongestPath, f("drainageDistance", i))
	}
	roots := []int{}
	for root := range basins {
		roots = append(roots, root)
	}
	sort.Ints(roots)
	h.Watersheds = nil
	for _, root := range roots {
		h.Watersheds = append(h.Watersheds, *basins[root])
	}
	stats := &HydroStatistics{LengthUnits: "base-grid cells", AreaUnits: "base-grid cells squared", BasinCount: len(h.Watersheds), Lakes: len(e.WaterBodies) - 1, Wetlands: len(h.Wetlands), Springs: len(h.Springs), Classes: []RiverClassStats{{Class: "major"}, {Class: "regional"}, {Class: "stream"}}}
	lengths, areas, basinAreas := []float64{}, []float64{}, []float64{}
	valid := map[string]bool{}
	for _, r := range h.Rivers {
		valid[r.ID] = true
	}
	for _, b := range e.WaterBodies {
		valid[e.hydroID("water", b.ID)] = true
	}
	for i := range e.Mask {
		if f("terminalBasin", i) > 0 {
			valid[e.hydroID("terminal-basin", i)] = true
		}
	}
	for _, r := range h.Rivers {
		for k := range stats.Classes {
			if stats.Classes[k].Class == r.Class {
				stats.Classes[k].Count++
				stats.Classes[k].Length += r.Length
			}
		}
		if !valid[r.Downstream] {
			stats.InvalidDestinations++
		}
		if f("mountainCore", r.Source) < .2 {
			stats.LowlandHeadwaters++
		}
		lengths = append(lengths, r.Length)
		areas = append(areas, r.CatchmentArea)
	}
	for _, b := range h.Watersheds {
		basinAreas = append(basinAreas, float64(b.Area))
	}
	for k := range h.Targets {
		target := &h.Targets[k]
		stats.LandCells += target.LandCells
		stats.TargetCoveragePercent += float64(target.TargetCells)
	}
	for i, id := range labels {
		if id == 0 {
			continue
		}
		if f("lake", i) > 0 {
			h.Targets[id-1].LakeCells++
		}
		if f("wetland", i) >= .6 {
			h.Targets[id-1].WetlandCells++
		}
	}
	stats.DrainageCells = len(h.Reaches)
	stats.CoveragePercent = 100 * float64(stats.DrainageCells) / math.Max(1, float64(stats.LandCells))
	stats.TargetCoveragePercent *= 100 / math.Max(1, float64(stats.LandCells))
	stats.Lengths = distribution(lengths)
	stats.Catchments = distribution(areas)
	stats.BasinSizes = distribution(basinAreas)
	if stats.CoveragePercent < stats.TargetCoveragePercent*.85 {
		stats.Notes = append(stats.Notes, "Drainage coverage is below target: remaining catchments lack sufficient contributing area or water.")
	}
	if len(h.Rivers) > 0 && stats.Classes[0].Count == 0 {
		stats.Notes = append(stats.Notes, "No watershed supports a continental trunk at this landmass size, drainage length and discharge.")
	}
	h.Statistics = stats
}

// Mountain/glacier runoff needs water and a contributing catchment, not just
// a steep cell. Lowland tributary runoff still feeds the accumulated discharge.
func (e *Environment) supportedRiverHeadwater(i int) bool {
	f := e.get
	return f("waterBody", i) == 0 && f("flow", i) >= 0 &&
		f("catchmentArea", i) >= 2 && f("accumulation", i) >= .18*math.Max(1, float64(len(e.Mask))/16000) &&
		(f("glacier", i) > .1 || f("mountainCore", i) > .09 || (f("elevation", i) > 250 && f("mountainCore", i) > .035))
}
