package terrain

import (
	"math"
	"sort"
)

// Cell-scale diagnostics separate structural concavity from pixel roughness.
// Hull bays count substantial connected indentations, not every coast pixel.
type LandmassMetric struct {
	Area            int     `json:"areaCells"`
	Perimeter       int     `json:"perimeterCellEdges"`
	Aspect          float64 `json:"principalAspectRatio"`
	Solidity        float64 `json:"convexHullSolidity"`
	CoastComplexity float64 `json:"coastComplexity"`
	Bays            int     `json:"majorBays"`
	BayDepth        float64 `json:"deepestBayCells"`
}
type LandShapeReport struct {
	LandCells      int              `json:"landCells"`
	LandFraction   float64          `json:"landFraction"`
	IslandSizes    []int            `json:"islandSizes"`
	Landmasses     []LandmassMetric `json:"landmasses"`
	EnclosedWaters int              `json:"enclosedWaterRegions"`
	MarineRegions  int              `json:"marineRegions"`
	Formations     map[string]int   `json:"formations"`
}
type shapeXY struct{ x, y float64 }

func shapeCross(a, b, c shapeXY) float64 { return (b.x-a.x)*(c.y-a.y) - (b.y-a.y)*(c.x-a.x) }
func shapeHull(p []shapeXY) []shapeXY {
	sort.Slice(p, func(i, j int) bool {
		if p[i].x == p[j].x {
			return p[i].y < p[j].y
		}
		return p[i].x < p[j].x
	})
	h := []shapeXY{}
	for _, v := range p {
		for len(h) > 1 && shapeCross(h[len(h)-2], h[len(h)-1], v) <= 0 {
			h = h[:len(h)-1]
		}
		h = append(h, v)
	}
	start := len(h)
	for i := len(p) - 2; i >= 0; i-- {
		v := p[i]
		for len(h) > start && shapeCross(h[len(h)-2], h[len(h)-1], v) <= 0 {
			h = h[:len(h)-1]
		}
		h = append(h, v)
	}
	if len(h) > 1 {
		h = h[:len(h)-1]
	}
	return h
}
func (e *Environment) LandShapeDiagnostics() LandShapeReport {
	w, h := e.Options.Columns, e.Options.Rows
	n := w * h
	r := LandShapeReport{Formations: map[string]int{}}
	for _, s := range e.ContinentalStructures {
		r.Formations[s.Formation]++
	}
	// Components use diagonals for land and cardinal connections for water.
	seen := make([]bool, n)
	landGroups := [][]int{}
	waterGroups := [][]int{}
	for i := range e.Mask {
		if seen[i] {
			continue
		}
		land := e.Mask[i] != 0
		q := []int{i}
		seen[i] = true
		for head := 0; head < len(q); head++ {
			at := q[head]
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 || !land && dx*dy != 0 {
						continue
					}
					x, y := at%w+dx, at/w+dy
					if e.Geography != nil && e.Geography.WrapX {
						x = (x + w) % w
					}
					if x < 0 || x >= w || y < 0 || y >= h {
						continue
					}
					j := y*w + x
					if !seen[j] && (e.Mask[j] != 0) == land {
						seen[j] = true
						q = append(q, j)
					}
				}
			}
		}
		if land {
			landGroups = append(landGroups, q)
			r.LandCells += len(q)
			r.IslandSizes = append(r.IslandSizes, len(q))
		} else {
			waterGroups = append(waterGroups, q)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(r.IslandSizes)))
	r.LandFraction = float64(r.LandCells) / float64(n)
	ocean := make([]bool, n)
	largest := -1
	for k, g := range waterGroups {
		if largest < 0 || len(g) > len(waterGroups[largest]) {
			largest = k
		}
	}
	for k, g := range waterGroups {
		marine := false
		if values := e.Fields["ocean"]; len(values) == n {
			for _, i := range g {
				if values[i] > 0 {
					marine = true
					break
				}
			}
		} else {
			marine = k == largest
		}
		if marine {
			r.MarineRegions++
			for _, i := range g {
				ocean[i] = true
			}
		} else {
			r.EnclosedWaters++
		}
	}
	for _, g := range landGroups {
		if len(g) < 16 {
			continue
		}
		m := LandmassMetric{Area: len(g)}
		points := []shapeXY{}
		sx, sy := 0., 0.
		minX, minY, maxX, maxY := w, h, 0, 0
		for _, i := range g {
			x, y := i%w, i/w
			sx += float64(x)
			sy += float64(y)
			minX = min(minX, x)
			maxX = max(maxX, x)
			minY = min(minY, y)
			maxY = max(maxY, y)
			edge := false
			for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				xx, yy := x+d[0], y+d[1]
				if xx < 0 || xx >= w || yy < 0 || yy >= h || e.Mask[yy*w+xx] == 0 {
					m.Perimeter++
					edge = true
				}
			}
			if edge {
				for _, d := range [][2]float64{{-.5, -.5}, {.5, -.5}, {.5, .5}, {-.5, .5}} {
					points = append(points, shapeXY{float64(x) + d[0], float64(y) + d[1]})
				}
			}
		}
		sx /= float64(len(g))
		sy /= float64(len(g))
		xx, yy, xy := 0., 0., 0.
		for _, i := range g {
			x, y := float64(i%w)-sx, float64(i/w)-sy
			xx += x * x
			yy += y * y
			xy += x * y
		}
		d := math.Sqrt((xx-yy)*(xx-yy) + 4*xy*xy)
		m.Aspect = math.Sqrt((xx + yy + d) / math.Max(1, xx+yy-d))
		hull := shapeHull(points)
		area := 0.
		for k, p := range hull {
			q := hull[(k+1)%len(hull)]
			area += p.x*q.y - p.y*q.x
		}
		area = math.Abs(area) / 2
		m.Solidity = math.Min(1, float64(m.Area)/math.Max(1, area))
		m.CoastComplexity = float64(m.Perimeter*m.Perimeter) / (4 * math.Pi * float64(m.Area))
		bays := make([]bool, n)
		for y := minY; y <= maxY; y++ {
			for x := minX; x <= maxX; x++ {
				i := y*w + x
				if !ocean[i] {
					continue
				}
				p := shapeXY{float64(x), float64(y)}
				depth := math.Inf(1)
				inside := true
				for k, a := range hull {
					b := hull[(k+1)%len(hull)]
					cross := shapeCross(a, b, p)
					if cross < 0 {
						inside = false
						break
					}
					depth = math.Min(depth, cross/math.Max(.001, math.Hypot(b.x-a.x, b.y-a.y)))
				}
				if inside {
					m.BayDepth = math.Max(m.BayDepth, depth)
					bays[i] = depth >= 1.5
				}
			}
		}
		for i, v := range bays {
			if !v {
				continue
			}
			bays[i] = false
			q := []int{i}
			for head := 0; head < len(q); head++ {
				for _, j := range nb(q[head], w, h) {
					if bays[j] {
						bays[j] = false
						q = append(q, j)
					}
				}
			}
			if len(q) >= 4 {
				m.Bays++
			}
		}
		r.Landmasses = append(r.Landmasses, m)
	}
	sort.Slice(r.Landmasses, func(i, j int) bool { return r.Landmasses[i].Area > r.Landmasses[j].Area })
	return r
}
