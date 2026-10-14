package generate

import (
	"container/heap"
	"context"
	"fmt"
	"kriemhild/internal/terrain/model"
	"math"
	"sort"
)

type floodNode struct {
	i int
	z float64
}
type floodQueue []floodNode

func (q floodQueue) Len() int { return len(q) }
func (q floodQueue) Less(i, j int) bool {
	if q[i].z == q[j].z {
		return q[i].i < q[j].i
	}
	return q[i].z < q[j].z
}
func (q floodQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *floodQueue) Push(v any)   { *q = append(*q, v.(floodNode)) }
func (q *floodQueue) Pop() any     { a := *q; v := a[len(a)-1]; *q = a[:len(a)-1]; return v }

func drainage(ctx context.Context, p *model.Planet) error {
	g := &p.Grid
	bed, flow, level := g.Fields["bed"], g.Fields["flow"], g.Fields["drainage"]
	n := len(bed)
	seen := make([]bool, n)
	q := &floodQueue{}
	order := make([]int, 0, n)
	ocean := func(i int) bool {
		if p.Spec.Version == "continuous-1" {
			return bed[i] <= 0
		}
		return g.Fields["coast"][i] <= 0
	}
	for i := range bed {
		if ocean(i) {
			seen[i] = true
			level[i] = 0
			heap.Push(q, floodNode{i, 0})
		}
	}
	if q.Len() == 0 {
		return fmt.Errorf("world has no ocean outlets")
	}
	for q.Len() > 0 {
		if len(order)%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		a := heap.Pop(q).(floodNode)
		order = append(order, a.i)
		adjacent := neighbors(a.i, g.Width, g.Height)
		if p.Surface != nil {
			adjacent = neighbors8(a.i, g.Width, g.Height)
		}
		for _, j := range adjacent {
			if seen[j] {
				continue
			}
			seen[j] = true
			level[j] = math.Max(bed[j], a.z+.001)
			flow[j] = float64(a.i)
			heap.Push(q, floodNode{j, level[j]})
		}
	}
	// Priority flood establishes spill levels; choose the steepest descending
	// D8 edge afterward, rather than the first heap visitor's cardinal edge.
	if p.Surface != nil {
		for i := range bed {
			if ocean(i) {
				continue
			}
			pt := g.Point(i%g.Width, i/g.Width)
			best, slope := int(flow[i]), -1.
			for _, j := range neighbors8(i, g.Width, g.Height) {
				if level[j] >= level[i] {
					continue
				}
				other := g.Point(j%g.Width, j/g.Width)
				d := math.Hypot(model.Wrap(pt[0]-other[0])*math.Cos(pt[1]), pt[1]-other[1])
				gradient := (level[i] - level[j]) / math.Max(1e-12, d)
				if gradient > slope {
					best, slope = j, gradient
				}
			}
			flow[i] = float64(best)
		}
	}
	discharge := g.Fields["discharge"]
	catchment := make([]float64, n)
	for i, b := range bed {
		pt := g.Point(i%g.Width, i/g.Width)
		area := model.Tau / float64(g.Width) * math.Pi / float64(g.Height) * math.Cos(pt[1]) * p.Spec.RadiusKM * p.Spec.RadiusKM * 1e6
		runoff := math.Max(0, g.Fields["precipitation"][i]-g.Fields["pet"][i]*.55)
		if p.Surface != nil {
			// Soil recharge supplies perennial base flow away from mountain tops;
			// arid runoff remains seasonal and is classified separately below.
			rain := g.Fields["precipitation"][i]
			runoff = math.Max(rain*.025, runoff)
		}
		if !ocean(i) {
			discharge[i] = runoff / 1000 * area / (365.25 * 86400)
			catchment[i] = area / 1e6
		}
		if !ocean(i) && level[i]-b > 20 && g.Fields["moisture"][i] > .35 {
			g.Fields["lake"][i] = level[i]
		}
		if p.Surface != nil && !ocean(i) && level[i]-b > 5 && g.Fields["moisture"][i] > .3 {
			g.Fields["lake"][i] = level[i]
		}
	}
	for k := len(order) - 1; k >= 0; k-- {
		i := order[k]
		j := int(flow[i])
		if j >= 0 {
			discharge[j] += discharge[i]
			catchment[j] += catchment[i]
		}
	}
	// Persist basin water planes. A lake boundary is evaluated against the
	// continuous bed, never an interpolated zero-valued lake cell border.
	lakeSeen := make([]bool, n)
	for i, b := range bed {
		if lakeSeen[i] || g.Fields["lake"][i] <= b || g.Fields["lake"][i] == 0 {
			continue
		}
		cells := []int{i}
		lakeSeen[i] = true
		low := i
		for head := 0; head < len(cells); head++ {
			j := cells[head]
			if bed[j] < bed[low] {
				low = j
			}
			for _, k := range neighbors(j, g.Width, g.Height) {
				if !lakeSeen[k] && g.Fields["lake"][k] > bed[k] && g.Fields["lake"][k] > 0 {
					lakeSeen[k] = true
					cells = append(cells, k)
				}
			}
		}
		if level[low]-bed[low] < 40 && p.Surface == nil {
			continue
		}
		center := g.Point(low%g.Width, low/g.Width)
		kind := "basin"
		if p.Surface != nil {
			if g.Fields["temperature"][low] < 4 {
				kind = "glacial"
			} else if bed[low] < 250 {
				kind = "floodplain"
			} else if g.Fields["roughness"][low] > .6 {
				kind = "tectonic"
			}
			if len(cells) < 4 && g.Fields["soil"][low] > 130 && bed[low] < 700 {
				kind = "spring-fed"
			}
			if len(cells) == 1 && bed[low] > 900 && g.Fields["temperature"][low] > 3 {
				rim := math.Inf(1)
				for _, j := range neighbors8(low, g.Width, g.Height) {
					rim = math.Min(rim, bed[j])
				}
				if rim-bed[low] > 120 && g.Fields["roughness"][low] > .65 {
					kind = "volcanic"
				}
			}
		}
		radius := math.Min(.035, .009*math.Sqrt(float64(len(cells))))
		if p.Surface != nil {
			radius = math.Min(.035, math.Pi/float64(g.Height)*.55*math.Sqrt(float64(len(cells))))
		}
		p.Lakes = append(p.Lakes, model.Lake{Center: center, Radius: radius, Level: level[low], Kind: kind})
	}
	threshold := 1800.
	if p.Spec.Version != "continuous-1" {
		threshold = 300
	}
	if p.Surface != nil {
		// An explicit, resolution-independent network coverage target. Selecting
		// by accumulated runoff retains downstream connectivity automatically.
		values := []float64{}
		for i, q := range discharge {
			if !ocean(i) && g.Fields["precipitation"][i] > 80 {
				values = append(values, q)
			}
		}
		sort.Float64s(values)
		if len(values) > 0 {
			threshold = math.Max(.01, values[int(float64(len(values))*.78)])
		}
		return basinRivers(ctx, p, order, catchment, threshold)
	}
	upstream := make([]int, n)
	streamOrder, maximumUpstream, sameOrder := make([]int, n), make([]int, n), make([]int, n)
	for k := len(order) - 1; k >= 0; k-- {
		i := order[k]
		if ocean(i) || discharge[i] < threshold {
			continue
		}
		streamOrder[i] = max(1, maximumUpstream[i])
		if sameOrder[i] >= 2 {
			streamOrder[i]++
		}
		if j := int(flow[i]); j >= 0 {
			if streamOrder[i] > maximumUpstream[j] {
				maximumUpstream[j] = streamOrder[i]
				sameOrder[j] = 1
			} else if streamOrder[i] == maximumUpstream[j] {
				sameOrder[j]++
			}
		}
	}
	for i, f := range flow {
		j := int(f)
		if j >= 0 && discharge[i] >= threshold && !ocean(i) {
			upstream[j]++
		}
	}
	for i := range bed {
		if ocean(i) || discharge[i] < threshold || upstream[i] == 1 {
			continue
		}
		river := model.River{ID: fmt.Sprintf("river-%d", i), Destination: "ocean"}
		if p.Spec.Version != "continuous-1" {
			river.Order = streamOrder[i]
		}
		for j, steps := i, 0; steps < n; steps++ {
			pt := g.Point(j%g.Width, j/g.Width)
			river.Path = append(river.Path, pt)
			river.Surface = append(river.Surface, level[j])
			river.Bed = append(river.Bed, level[j]-math.Max(1, math.Pow(discharge[j], .3)*.3))
			river.Discharge = append(river.Discharge, discharge[j])
			river.Width = append(river.Width, math.Max(4, math.Sqrt(discharge[j])*2.4))
			if ocean(j) {
				break
			}
			next := int(flow[j])
			if next < 0 {
				river.Destination = "basin"
				break
			}
			if j != i && upstream[j] > 1 {
				river.Destination = fmt.Sprintf("river-%d", j)
				break
			}
			j = next
		}
		if len(river.Path) >= 2 {
			if p.Spec.Version != "continuous-1" {
				river = refineRiver(p, river)
			}
			p.Rivers = append(p.Rivers, river)
		}
	}
	return nil
}

func neighbors8(i, w, h int) []int {
	x, y := i%w, i/w
	a := make([]int, 0, 8)
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 || y+dy < 0 || y+dy >= h {
				continue
			}
			a = append(a, (y+dy)*w+(x+dx+w)%w)
		}
	}
	return a
}
