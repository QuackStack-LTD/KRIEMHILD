package terrain

import (
	"fmt"
	"math"
)

// Resolve braids narrower than a simulation cell as a persistent channel graph.
// Node indices reference ChannelPath on the owning reach. Each side channel
// splits at one node and rejoins at a later node; water is partitioned only
// between those junctions. Geometry follows a least-cost route through sampled
// floodplain terrain, not a translated copy of the main line.
func (e *Environment) buildSubcellChannels() {
	if e.Geography == nil {
		return
	}
	m := &DetailModel{World: e, Seed: Seed(e.Options.Seed) ^ 0x91ab6731}
	m.buildDrainage()
	byID := map[string]DetailFeature{}
	for _, f := range m.Features {
		byID[f.ID] = f
	}
	for k := range e.Hydrology.Reaches {
		r := &e.Hydrology.Reaches[k]
		f, ok := byID[r.ID]
		grade := e.channelMorphologyGrade(r.From, r.To, f.Grade)
		if !ok || r.Source == "lake-outlet" || len(f.Path) < 20 || len(r.Branches) > 0 || e.get("waterBody", r.To) > 0 || r.Discharge < 8 || e.get("sediment", r.From) < .5 || grade > 90 || r.ValleyWidthMetres < r.WidthMetres*2 {
			continue
		}
		metresPerCell := r.WidthMetres / math.Max(1e-12, r.Width)
		corridor := math.Min(r.ValleyWidthMetres*.4, r.WidthMetres*4) / metresPerCell
		if corridor <= r.Width*1.8 {
			continue
		}
		loops := 1
		if e.get("sediment", r.From) > .68 && r.Order > 1 {
			loops = 2
		}
		for loop := 0; loop < loops; loop++ {
			start, end := 2, len(f.Path)-3
			if loops == 2 {
				start = 2 + loop*11
				end = min(len(f.Path)-3, start+9)
			}
			path := m.routeFloodplainChannel(f.Path, start, end, corridor, math.Max(6, r.IncisionMetres+8))
			if len(path) == 0 {
				continue
			}
			fraction := .18 + .12*e.get("sediment", r.From)
			if loop == 1 {
				fraction *= .7
			}
			kind := "braid"
			if grade < 5 && r.DryDischarge > r.Discharge*.3 {
				kind = "anabranch"
			}
			r.Branches = append(r.Branches, HydroBranch{ID: fmt.Sprintf("%s/corridor/%d", r.ID, loop), Kind: kind, To: r.To, Downstream: r.ID, Fraction: fraction, Discharge: r.Discharge * fraction, DischargeM3s: r.DischargeM3s * fraction, Path: path, SplitNode: start, JoinNode: end})
		}
		if len(r.Branches) > 0 {
			r.ChannelPath = append([][3]float64(nil), f.Path...)
			r.Morphology = "braided"
			if r.Branches[0].Kind == "anabranch" {
				r.Morphology = "anabranching"
			}
		}
	}
}

func (m *DetailModel) routeFloodplainChannel(main [][3]float64, start, end int, corridor, incision float64) [][3]float64 {
	const lanes = 9
	n := end - start + 1
	cost := make([][lanes]float64, n)
	previous := make([][lanes]int, n)
	points := make([][lanes][3]float64, n)
	for row := 0; row < n; row++ {
		at := start + row
		a, b := main[max(0, at-1)], main[min(len(main)-1, at+1)]
		length := math.Hypot(b[0]-a[0], b[1]-a[1])
		if length == 0 {
			return nil
		}
		for lane := 0; lane < lanes; lane++ {
			cost[row][lane] = math.Inf(1)
			offset := float64(lane-4) / 4 * corridor
			p := main[at]
			p[0] -= (b[1] - a[1]) / length * offset
			p[1] += (b[0] - a[0]) / length * offset
			points[row][lane] = p
			if p[0] < 0 || p[1] < 0 || p[0] > float64(m.World.Options.Columns-1) || p[1] > float64(m.World.Options.Rows-1) {
				continue
			}
			ground := m.base(p[0], p[1])
			// A floodplain branch cannot cut a ridge or sit above an unrelated
			// deep valley. Its incision is bounded by the reach's erosion history.
			if ground.WaterBody > 0 || math.Abs(ground.Elevation-p[2]) > incision {
				continue
			}
			if row == 0 {
				if lane == 4 {
					cost[row][lane] = 0
				}
				continue
			}
			if row == n-1 && lane != 4 {
				continue
			}
			// Sediment bars require a distinct route, with dry ground between
			// channels; a path that simply overlaps the trunk is not a braid.
			if row >= n/3 && row <= 2*n/3 && abs(lane-4) < 2 {
				continue
			}
			local := math.Max(0, ground.Elevation-p[2]) + .2*math.Abs(offset)/corridor
			for before := max(0, lane-1); before <= min(lanes-1, lane+1); before++ {
				value := cost[row-1][before] + local + .05*float64(abs(lane-before))
				if value < cost[row][lane] {
					cost[row][lane] = value
					previous[row][lane] = before
				}
			}
		}
	}
	if math.IsInf(cost[n-1][4], 1) {
		return nil
	}
	path := make([][3]float64, n)
	lane := 4
	for row := n - 1; row >= 0; row-- {
		path[row] = points[row][lane]
		lane = previous[row][lane]
	}
	return path
}
