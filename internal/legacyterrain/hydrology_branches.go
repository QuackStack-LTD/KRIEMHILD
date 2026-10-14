package terrain

import (
	"fmt"
	"math"
	"sort"
)

// Side channels are paths in the saved hydraulic graph, not offset renderings.
// A braid rejoins the same receiving node; a delta reaches a separate marine
// outlet. Interiors cannot cross an existing channel or another side channel.
func (e *Environment) buildHydroBranches() {
	f := e.get
	occupied := map[int]bool{}
	for _, r := range e.Hydrology.Reaches {
		occupied[r.From] = true
	}
	surface := func(i int) float64 {
		if f("waterBody", i) > 0 {
			return f("waterLevel", i)
		}
		return f("elevation", i)
	}
	for k := range e.Hydrology.Reaches {
		r := &e.Hydrology.Reaches[k]
		grade := e.channelMorphologyGrade(r.From, r.To, math.Max(0, surface(r.From)-surface(r.To)))
		if f("sediment", r.From) < .35 || r.Discharge < 8 || grade > 90 {
			continue
		}
		delta := f("ocean", r.To) > 0 && grade < 20
		inland := f("lake", r.To) > 0 && grade < 12 && f("aridity", r.From) > .35
		braid := !delta && !inland && f("waterBody", r.To) == 0 && f("sediment", r.From) > .45
		// A coarse cell must fit inside the physical corridor. Narrow braids
		// are routed on a saved sub-cell graph instead of becoming 200-km loops.
		if braid && e.Geography != nil && e.stepKM(r.From, r.To)*1000 > r.ValleyWidthMetres {
			braid = false
		}
		if !delta && !inland && !braid {
			continue
		}
		type candidate struct {
			cells []int
			cost  float64
		}
		candidates := []candidate{}
		// Each alternate first facet must reach a valid sink or rejoin. Search
		// only downhill in a bounded corridor, preferring low depositional ground.
		for _, start := range e.drainageNeighbors(r.From) {
			if math.Abs(float64(start%e.Options.Columns-r.From%e.Options.Columns)) > 3 || start == r.To || occupied[start] || surface(start) > surface(r.From) || !e.drainageCornerOpen(r.From, start, surface) {
				continue
			}
			paths := [][]int{{r.From, start}}
			seen := map[int]bool{r.From: true, start: true}
			for head := 0; head < len(paths) && head < 64; head++ {
				path := paths[head]
				at := path[len(path)-1]
				end := braid && at == r.To || delta && f("ocean", at) > 0 && at != r.To || inland && f("lake", at) > 0 && f("waterBody", at) == f("waterBody", r.To) && at != r.To
				if end {
					cost := 0.
					for p := 1; p < len(path); p++ {
						cost += e.hydroStep(path[p-1], path[p]) * (1 + surface(path[p])*.001)
					}
					candidates = append(candidates, candidate{path, cost})
					break
				}
				if len(path) > 6 || f("waterBody", at) > 0 {
					continue
				}
				next := e.drainageNeighbors(at)
				sort.SliceStable(next, func(a, b int) bool { return surface(next[a]) < surface(next[b]) })
				for _, j := range next {
					if seen[j] || occupied[j] && j != r.To || j == r.To && !braid || surface(j) > surface(at) || !e.drainageCornerOpen(at, j, surface) {
						continue
					}
					if e.hydroStep(r.From, j) > 3 || math.Abs(float64(j%e.Options.Columns-r.From%e.Options.Columns)) > 3 {
						continue
					}
					seen[j] = true
					paths = append(paths, append(append([]int(nil), path...), j))
				}
			}
		}
		sort.SliceStable(candidates, func(a, b int) bool { return candidates[a].cost < candidates[b].cost })
		used := map[int]bool{}
		for _, p := range candidates {
			conflict := false
			for _, i := range p.cells[1 : len(p.cells)-1] {
				conflict = conflict || used[i]
			}
			end := p.cells[len(p.cells)-1]
			if !braid && used[end] {
				conflict = true
			}
			if conflict {
				continue
			}
			for _, i := range p.cells[1:] {
				used[i] = true
			}
			fraction := .28 / float64(1+len(r.Branches))
			kind := "braid"
			if delta {
				kind = "delta"
			} else if inland {
				kind = "inland-delta"
			} else if grade < 5 && f("dryDischarge", r.From) > r.Discharge*.3 {
				kind = "anabranch"
			}
			r.Branches = append(r.Branches, HydroBranch{ID: fmt.Sprintf("%s/channel/%d", r.ID, len(r.Branches)), To: end, Downstream: e.waterRef(end), Fraction: fraction, Cells: p.cells, Kind: kind, Discharge: r.Discharge * fraction, DischargeM3s: r.DischargeM3s * fraction})
			for _, i := range p.cells[1 : len(p.cells)-1] {
				occupied[i] = true
			}
			if len(r.Branches) == 3 {
				break
			}
		}
		if len(r.Branches) > 0 {
			r.Morphology = r.Branches[0].Kind
			if r.Morphology == "braid" {
				r.Morphology = "braided"
			}
			if r.Morphology == "anabranch" {
				r.Morphology = "anabranching"
			}
		} else if r.Morphology == "braided" {
			r.Morphology = "meandering"
		}
	}
}
