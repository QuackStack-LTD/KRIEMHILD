package terrain

import (
	"math"
	"sort"
)

// Resolve storage landforms missing from the coarse continental surface.
// Differential weathering of low-permeability lowlands, glacial overdeepening
// and fault subsidence create depressions; the water budget decides whether
// they hold lakes, wetlands or remain dry. Mountain cores and authored heights
// are preserved, and no coast is moved to manufacture an inland water body.
func (e *Environment) carveStorageBasins() bool {
	if e.Options.Heights != nil {
		return false
	}
	f := e.get
	w, h := e.Options.Columns, e.Options.Rows
	labels, sizes := e.hydroLandmasses()
	d := distance(e.Mask, w, h, 0)
	type site struct {
		cell         int
		score, depth float64
	}
	candidates := make([]site, 0)
	for i := range e.Mask {
		if f("ocean", i) > 0 || d[i] < 3 || f("mountainCore", i) > .22 || f("slope", i) > .065 || f("elevation", i) < 32 {
			continue
		}
		weathering := (1 - f("permeability", i)) * (1 - smooth(f("slope", i)/.065))
		glacial := math.Max(f("glacier", i), f("snowmelt", i))
		support := math.Max(weathering, math.Max(glacial, f("tectonicStress", i)))
		if support < .35 {
			continue
		}
		low, mean := f("elevation", i), 0.
		neighbors := e.drainageNeighbors(i)
		for _, j := range neighbors {
			low = math.Min(low, f("elevation", j))
			mean += f("elevation", j) / float64(len(neighbors))
		}
		// A shallow, broad depression can close a gentle valley, but must not
		// excavate an arbitrary hole through a steep hillside.
		depth := f("elevation", i) - low + 12 + 28*support
		if depth > 80 || f("elevation", i)-depth < 4 {
			continue
		}
		variation := detailHash(Seed(e.Options.Seed), i%w, i/w, 73)
		score := support + clamp((mean-f("elevation", i))/40, -.5, .5) + variation*.3
		candidates = append(candidates, site{i, score, depth})
	}
	sort.SliceStable(candidates, func(a, b int) bool { return candidates[a].score > candidates[b].score })
	counts := make([]int, len(sizes))
	used := make([]bool, len(e.Mask))
	changed := false
	for _, candidate := range candidates {
		i := candidate.cell
		id := labels[i]
		if used[i] || counts[id] >= max(1, sizes[id]/220) {
			continue
		}
		counts[id]++
		radius := 1.6 + .8*clamp(candidate.score, 0, 1)
		for dy := -5; dy <= 5; dy++ {
			for dx := -5; dx <= 5; dx++ {
				x, y := i%w+dx, i/w+dy
				if x < 0 || x >= w || y < 0 || y >= h {
					continue
				}
				j := y*w + x
				used[j] = true
				r := math.Hypot(float64(dx), float64(dy)) / radius
				if r >= 1 || d[j] < 2 || f("mountainCore", j) > .22 {
					continue
				}
				old := f("elevation", j)
				z := math.Max(4, old-candidate.depth*math.Pow(1-r*r, 2))
				// The basin apron can touch a steeper neighbouring formation.
				// Retain the existing gradient bound without moving its mountain.
				for _, neighbor := range nb(j, w, h) {
					z = math.Max(z, math.Min(old, f("elevation", neighbor)-650))
				}
				e.Heights[j] = int(math.Ceil(z / 4))
				e.set("elevation", j, float64(e.Heights[j]*4))
				for _, field := range []string{"temperature", "summer", "winter"} {
					e.set(field, j, f(field, j)+(old-f("elevation", j))*.0065)
				}
				changed = true
			}
		}
	}
	if changed {
		for i := range e.Mask {
			slope := 0.
			for _, j := range nb(i, w, h) {
				slope = math.Max(slope, math.Abs(f("elevation", i)-f("elevation", j))/2000)
			}
			e.set("slope", i, clamp(slope, 0, 1))
		}
	}
	return changed
}
