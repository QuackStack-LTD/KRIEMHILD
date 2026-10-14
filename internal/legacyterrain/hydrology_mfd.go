package terrain

import (
	"math"
	"sort"
)

// Hillslope routing is diffuse; channel routing is convergent. The first pass
// conservatively partitions area and runoff among downhill facets. The second
// extracts a primary drainage tree using that concentration and valley relief.
// Its conservative catchment budget is computed by accumulateWater; MFD values
// remain separate, queryable fields rather than being counted a second time.
func (e *Environment) routeWatersheds(rank []int, surface func(int) float64) {
	n := len(e.Mask)
	area, runoff := make([]float64, n), make([]float64, n)
	order := make([]int, n)
	for i := range order {
		order[i], area[i], runoff[i] = i, e.cellArea(i), e.get("runoff", i)
	}
	sort.Slice(order, func(a, b int) bool {
		i, j := order[a], order[b]
		if surface(i) == surface(j) {
			return rank[i] > rank[j]
		}
		return surface(i) > surface(j)
	})
	type facet struct {
		to            int
		weight, slope float64
	}
	paths := make([][]facet, n)
	for _, i := range order {
		if e.get("waterBody", i) > 0 {
			continue
		}
		z, sum := surface(i), 0.
		for _, j := range e.drainageNeighbors(i) {
			drop := z - surface(j)
			if drop < 0 || drop == 0 && rank[j] >= rank[i] || !e.drainageCornerOpen(i, j, surface) {
				continue
			}
			length := e.hydroStep(i, j)
			if e.Geography != nil {
				length = math.Max(.001, e.stepKM(i, j))
			}
			slope := math.Max(1e-9, drop/length)
			weight := math.Pow(slope, 1.1)
			paths[i] = append(paths[i], facet{j, weight, slope})
			sum += weight
		}
		for _, p := range paths[i] {
			fraction := p.weight / sum
			area[p.to] += area[i] * fraction
			runoff[p.to] += runoff[i] * fraction
		}
	}
	e.Fields["hillslopeAreaKM2"], e.Fields["hillslopeRunoff"] = area, runoff
	// A valley receiver can win over a marginally steeper planar facet, but
	// never over a ridge. No seed noise, uphill flow or coordinate jitter.
	for i, candidates := range paths {
		if e.get("waterBody", i) > 0 {
			continue
		}
		best, score := -1, -1.
		maxSlope := 0.
		for _, p := range candidates {
			maxSlope = math.Max(maxSlope, p.slope)
		}
		for _, p := range candidates {
			if p.slope < maxSlope*.03 {
				continue
			}
			concentration := area[p.to] / math.Max(1e-9, e.cellArea(p.to))
			valley := 0.
			for _, k := range e.neighbors(p.to) {
				valley += math.Max(0, surface(k)-surface(p.to))
			}
			value := math.Pow(p.slope/maxSlope, .35) * math.Pow(1+concentration, .8) * (1 + .25*valley/(1+valley))
			if value > score {
				best, score = p.to, value
			}
		}
		e.set("flow", i, float64(best))
	}
	// Incipient channels attract nearby downslope runoff as their contributing
	// area grows. Three bounded capture passes break independent planar-slope
	// tracks using existing concentration, without changing any elevation or
	// admitting a new uphill facet. Erosion reinforces the selected valleys.
	for pass := 0; pass < 3; pass++ {
		e.accumulateWater()
		contributing := append([]float64(nil), e.Fields["catchmentArea"]...)
		concentrated := append([]float64(nil), e.Fields["accumulation"]...)
		for i, candidates := range paths {
			if e.get("waterBody", i) > 0 || len(candidates) < 2 {
				continue
			}
			maxSlope := 0.
			for _, p := range candidates {
				maxSlope = math.Max(maxSlope, p.slope)
			}
			best, score := int(e.get("flow", i)), -1.
			for _, p := range candidates {
				if p.slope < maxSlope*.03 {
					continue
				}
				// Removing this cell's own existing upstream contribution prevents
				// its present receiver from winning merely through circular scoring.
				a, q := contributing[p.to], concentrated[p.to]
				if p.to == int(e.get("flow", i)) {
					a -= contributing[i]
					q -= concentrated[i]
				}
				focus := math.Log1p(math.Max(0, a)) + math.Log1p(math.Max(0, q))
				value := math.Pow(p.slope/maxSlope, .2) * (1 + focus)
				if value > score {
					best, score = p.to, value
				}
			}
			e.set("flow", i, float64(best))
		}
	}
	e.joinCrossingFacets(rank, surface)
	for i := range area {
		area[i] = f32(area[i])
		runoff[i] = f32(runoff[i])
	}
}

// Crossing diagonals are a confluence, not two rivers passing through each
// other. Redirect one diagonal to the lower corner using a cardinal downhill
// edge. This cannot introduce another crossing or a directed flow cycle.
func (e *Environment) joinCrossingFacets(rank []int, surface func(int) float64) {
	w := e.Options.Columns
	for i := range e.Mask {
		j := int(e.get("flow", i))
		if j < 0 || i%w == j%w || i/w == j/w {
			continue
		}
		a, b := i/w*w+j%w, j/w*w+i%w
		from, to := a, b
		if int(e.get("flow", a)) != b {
			from, to = b, a
			if int(e.get("flow", b)) != a {
				continue
			}
		}
		if surface(to) < surface(i) || surface(to) == surface(i) && rank[to] < rank[i] {
			e.set("flow", i, float64(to))
		} else if surface(j) < surface(from) || surface(j) == surface(from) && rank[j] < rank[from] {
			e.set("flow", from, float64(j))
		}
	}
}

// A diagonal may not cut across a ridge or shortcut through a water corner.
func (e *Environment) drainageCornerOpen(i, j int, surface func(int) float64) bool {
	w := e.Options.Columns
	dx, dy := j%w-i%w, j/w-i/w
	if e.Geography != nil && e.Geography.WrapX {
		if dx > w/2 {
			dx -= w
		}
		if dx < -w/2 {
			dx += w
		}
	}
	if dx == 0 || dy == 0 {
		return true
	}
	x := (i%w + dx + w) % w
	a, b := i/w*w+x, i+dy*w
	if surface(a) > surface(i) && surface(b) > surface(i) {
		return false
	}
	return !(e.get("waterBody", i) == 0 && e.get("waterBody", j) == 0 && (e.get("waterBody", a) > 0 || e.get("waterBody", b) > 0))
}
