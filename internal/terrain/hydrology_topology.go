package terrain

import "math"

type HydroTopology struct {
	Confluences                 int                          `json:"confluences"`
	RiversWithTributaries       int                          `json:"riversWithTributaries"`
	MajorConnectedCatchments    int                          `json:"majorConnectedCatchments"`
	Splits                      int                          `json:"splits"`
	RejoiningBranches           int                          `json:"rejoiningBranches"`
	DistributaryOutlets         int                          `json:"distributaryOutlets"`
	ParallelIndependentFraction float64                      `json:"parallelIndependentFraction"`
	WidthsMetres                HydroDistribution            `json:"widthsMetres"`
	WidthsByClass               map[string]HydroDistribution `json:"widthsByClass"`
}

func (e *Environment) summarizeHydroTopology() {
	t := &HydroTopology{WidthsByClass: map[string]HydroDistribution{}}
	widths := []float64{}
	classes := map[string][]float64{}
	for _, r := range e.Hydrology.Reaches {
		if len(r.Upstream) > 1 {
			t.Confluences++
		}
		if len(r.Branches) > 0 {
			t.Splits++
		}
		for _, b := range r.Branches {
			if b.To == r.To && e.get("waterBody", b.To) == 0 {
				t.RejoiningBranches++
			} else {
				t.DistributaryOutlets++
			}
		}
		if r.WidthMetres > 0 {
			widths = append(widths, r.WidthMetres)
			classes[r.Class] = append(classes[r.Class], r.WidthMetres)
		}
	}
	for class, v := range classes {
		t.WidthsByClass[class] = distribution(v)
	}
	t.WidthsMetres = distribution(widths)
	for _, r := range e.Hydrology.Rivers {
		if len(r.Tributaries) > 0 {
			t.RiversWithTributaries++
			if r.Class == "major" {
				t.MajorConnectedCatchments++
			}
		}
	}
	// Nearby, independent basins with nearly parallel overall courses. Parallel
	// tributaries in one watershed are not penalized as disconnected artifacts.
	pairs, parallel := 0, 0
	w := e.Options.Columns
	for a, r := range e.Hydrology.Rivers {
		for _, s := range e.Hydrology.Rivers[a+1:] {
			if r.Watershed == s.Watershed || e.hydroStep(r.Source, s.Source) > 8 {
				continue
			}
			ax, ay := float64(r.Mouth%w-r.Source%w), float64(r.Mouth/w-r.Source/w)
			bx, by := float64(s.Mouth%w-s.Source%w), float64(s.Mouth/w-s.Source/w)
			length := math.Hypot(ax, ay) * math.Hypot(bx, by)
			if length < 4 {
				continue
			}
			pairs++
			if (ax*bx+ay*by)/length > .97 {
				parallel++
			}
		}
	}
	if pairs > 0 {
		t.ParallelIndependentFraction = float64(parallel) / float64(pairs)
	}
	e.Hydrology.Topology = t
}

func (e *Environment) validateHydroBranch(r HydroReach, b HydroBranch, add func(string, string)) {
	if len(b.Path) > 0 {
		e.validateCorridorBranch(r, b, add)
		return
	}
	f := e.get
	n := len(e.Mask)
	if len(b.Cells) < 2 || b.Cells[0] != r.From || b.Cells[len(b.Cells)-1] != b.To {
		add(b.ID, "branch endpoints disagree with its split and receiving node")
		return
	}
	if b.To == r.To && len(b.Cells) < 3 {
		add(b.ID, "rejoining branch duplicates the primary channel")
	}
	if b.Downstream != e.waterRef(b.To) || b.To != r.To && f("waterBody", b.To) == 0 {
		add(b.ID, "branch neither rejoins its trunk nor reaches receiving water")
	}
	seen := map[int]bool{}
	surface := func(i int) float64 {
		if f("waterBody", i) > 0 {
			return f("waterLevel", i)
		}
		return f("elevation", i)
	}
	for k, i := range b.Cells {
		if i < 0 || i >= n {
			add(b.ID, "branch leaves terrain")
			return
		}
		if seen[i] {
			add(b.ID, "branch contains a cycle")
		}
		seen[i] = true
		if k == 0 {
			continue
		}
		prev := b.Cells[k-1]
		if e.hydroStep(prev, i) > 1.415 || surface(i) > surface(prev)+.02 || !e.drainageCornerOpen(prev, i, surface) {
			add(b.ID, "branch crosses a ridge, climbs, or contains a disconnected segment")
		}
		if k < len(b.Cells)-1 && f("waterBody", i) > 0 {
			add(b.ID, "branch passes through open water before its mouth")
		}
	}
	if math.Abs(b.Discharge-r.Discharge*b.Fraction) > math.Max(1e-6, r.Discharge*1e-6) || math.Abs(b.DischargeM3s-r.DischargeM3s*b.Fraction) > math.Max(1e-6, r.DischargeM3s*1e-6) {
		add(b.ID, "branch does not conserve its allocated discharge")
	}
}

func (e *Environment) validateHydroConservation(add func(string, string)) {
	if e.Hydrology.NetworkVersion < 4 {
		return
	}
	f := e.get
	for _, name := range []string{"hillslopeAreaKM2", "hillslopeRunoff"} {
		values := e.Fields[name]
		if len(values) != len(e.Mask) {
			add("hydrology", "missing distributed hillslope field "+name)
			continue
		}
		for i, v := range values {
			if !finite(v) || v < 0 {
				add(e.hydroID("cell", i), "invalid distributed hillslope budget")
			}
		}
	}
	if e.Hydrology.Topology == nil {
		add("hydrology", "missing channel topology diagnostics")
	}
	for _, r := range e.Hydrology.Reaches {
		if r.From < 0 || r.From >= len(e.Mask) {
			continue
		}
		if math.Abs(r.Discharge-f("accumulation", r.From)) > math.Max(1e-6, r.Discharge*1e-6) || math.Abs(r.DischargeM3s-f("dischargeM3s", r.From)) > math.Max(1e-6, r.DischargeM3s*1e-6) {
			add(r.ID, "saved reach discharge disagrees with contributing catchment")
		}
	}
	expected := append([]float64(nil), e.Fields["runoff"]...)
	for i := range e.Mask {
		if j := int(f("flow", i)); j >= 0 && j < len(expected) {
			expected[j] += math.Max(0, f("accumulation", i)-f("hydroLoss", i))
		}
	}
	for i, q := range expected {
		if math.Abs(q-f("accumulation", i)) > math.Max(1e-5, q*2e-6) {
			add(e.hydroID("cell", i), "runoff budget does not conserve water at drainage junction")
		}
	}
}

// Stream order belongs to channel topology, not every unrendered hillslope cell.
func (e *Environment) orderVisibleReaches() {
	indices := map[string]int{}
	for i, r := range e.Hydrology.Reaches {
		indices[r.ID] = i
	}
	pending := make([]int, len(indices))
	orders := make([]int, len(indices))
	ties := make([]int, len(indices))
	queue := []int{}
	for i, r := range e.Hydrology.Reaches {
		pending[i] = len(r.Upstream)
		if pending[i] == 0 {
			queue = append(queue, i)
		}
	}
	for head := 0; head < len(queue); head++ {
		i := queue[head]
		r := &e.Hydrology.Reaches[i]
		r.Order = max(1, orders[i])
		if ties[i] > 1 {
			r.Order++
		}
		if j, ok := indices[r.Downstream]; ok {
			if r.Order > orders[j] {
				orders[j] = r.Order
				ties[j] = 1
			} else if r.Order == orders[j] {
				ties[j]++
			}
			pending[j]--
			if pending[j] == 0 {
				queue = append(queue, j)
			}
		}
	}
}

func (e *Environment) validateCorridorBranch(r HydroReach, b HydroBranch, add func(string, string)) {
	if b.SplitNode < 0 || b.JoinNode <= b.SplitNode || b.JoinNode >= len(r.ChannelPath) || len(b.Path) != b.JoinNode-b.SplitNode+1 || b.Downstream != r.ID {
		add(b.ID, "invalid branch split/rejoin topology")
		return
	}
	if b.Path[0] != r.ChannelPath[b.SplitNode] || b.Path[len(b.Path)-1] != r.ChannelPath[b.JoinNode] {
		add(b.ID, "branch junction does not coincide with trunk")
	}
	distinct := false
	model := &DetailModel{World: e, Seed: Seed(e.Options.Seed) ^ 0x91ab6731}
	corridor := math.Min(r.ValleyWidthMetres*.4, r.WidthMetres*4) / math.Max(1e-12, r.WidthMetres) * r.Width
	for k, p := range b.Path {
		if !finite(p[0]) || !finite(p[1]) || !finite(p[2]) || p[0] < 0 || p[1] < 0 || p[0] > float64(e.Options.Columns-1) || p[1] > float64(e.Options.Rows-1) {
			add(b.ID, "branch geometry outside terrain")
			return
		}
		if k > 0 && p[2] > b.Path[k-1][2]+.02 {
			add(b.ID, "branch hydraulic grade climbs")
		}
		anchor := r.ChannelPath[b.SplitNode+k]
		ground := model.base(p[0], p[1])
		if math.Hypot(p[0]-anchor[0], p[1]-anchor[1]) > corridor+1e-8 || ground.WaterBody > 0 || math.Abs(ground.Elevation-p[2]) > math.Max(6, r.IncisionMetres+8)+1e-5 {
			add(b.ID, "branch leaves its supplied erosion corridor")
			return
		}
		distinct = distinct || math.Hypot(p[0]-anchor[0], p[1]-anchor[1]) > r.Width*.8
	}
	if !distinct {
		add(b.ID, "branch overlaps trunk instead of splitting")
	}
	if math.Abs(b.Discharge-r.Discharge*b.Fraction) > math.Max(1e-6, r.Discharge*1e-6) || math.Abs(b.DischargeM3s-r.DischargeM3s*b.Fraction) > math.Max(1e-6, r.DischargeM3s*1e-6) {
		add(b.ID, "side channel discharge is not conserved")
	}
}
