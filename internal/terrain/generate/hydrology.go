package generate

import (
	"container/heap"
	"context"
	"fmt"
	"kriemhild/internal/terrain/model"
	"math"
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
	for i, b := range bed {
		if b <= 0 {
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
		for _, j := range neighbors(a.i, g.Width, g.Height) {
			if seen[j] {
				continue
			}
			seen[j] = true
			level[j] = math.Max(bed[j], a.z+.001)
			flow[j] = float64(a.i)
			heap.Push(q, floodNode{j, level[j]})
		}
	}
	discharge := g.Fields["discharge"]
	for i, b := range bed {
		pt := g.Point(i%g.Width, i/g.Width)
		area := model.Tau / float64(g.Width) * math.Pi / float64(g.Height) * math.Cos(pt[1]) * p.Spec.RadiusKM * p.Spec.RadiusKM * 1e6
		runoff := math.Max(0, g.Fields["precipitation"][i]-g.Fields["pet"][i]*.55)
		if b > 0 {
			discharge[i] = runoff / 1000 * area / (365.25 * 86400)
		}
		if b > 0 && level[i]-b > 20 && g.Fields["moisture"][i] > .35 {
			g.Fields["lake"][i] = level[i]
		}
	}
	for k := len(order) - 1; k >= 0; k-- {
		i := order[k]
		j := int(flow[i])
		if j >= 0 {
			discharge[j] += discharge[i]
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
		if level[low]-bed[low] < 40 {
			continue
		}
		center := g.Point(low%g.Width, low/g.Width)
		p.Lakes = append(p.Lakes, model.Lake{Center: center, Radius: math.Min(.035, .009*math.Sqrt(float64(len(cells)))), Level: level[low]})
	}
	threshold := 1800.
	upstream := make([]int, n)
	for i, f := range flow {
		j := int(f)
		if j >= 0 && discharge[i] >= threshold && bed[i] > 0 {
			upstream[j]++
		}
	}
	for i, b := range bed {
		if b <= 0 || discharge[i] < threshold || upstream[i] == 1 {
			continue
		}
		river := model.River{ID: fmt.Sprintf("river-%d", i), Destination: "ocean"}
		for j, steps := i, 0; steps < n; steps++ {
			pt := g.Point(j%g.Width, j/g.Width)
			river.Path = append(river.Path, pt)
			river.Surface = append(river.Surface, level[j])
			river.Bed = append(river.Bed, level[j]-math.Max(1, math.Pow(discharge[j], .3)*.3))
			river.Discharge = append(river.Discharge, discharge[j])
			river.Width = append(river.Width, math.Max(4, math.Sqrt(discharge[j])*2.4))
			if bed[j] <= 0 {
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
			p.Rivers = append(p.Rivers, river)
		}
	}
	return nil
}
