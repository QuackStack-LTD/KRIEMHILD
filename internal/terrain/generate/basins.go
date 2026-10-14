package generate

import (
	"context"
	"fmt"
	"kriemhild/internal/terrain/model"
	"math"
	"sort"
)

// Select a dominant upstream branch at each confluence. Its identity continues
// to the outlet; tributaries end on that saved trunk, not at an arbitrary biome.
func basinRivers(ctx context.Context, p *model.Planet, order []int, area []float64, threshold float64) error {
	g := &p.Grid
	n := len(area)
	flow, q, level, bed := g.Fields["flow"], g.Fields["discharge"], g.Fields["drainage"], g.Fields["bed"]
	main, basin, strahler, maxOrder, ties := make([]int, n), make([]int, n), make([]int, n), make([]int, n), make([]int, n)
	owners := make([]int, n)
	landArea := 0.
	for i := range main {
		main[i] = -1
		owners[i] = -1
	}
	for k := len(order) - 1; k >= 0; k-- {
		i := order[k]
		if !p.Emerged(i) {
			continue
		}
		strahler[i] = max(1, maxOrder[i])
		if ties[i] > 1 {
			strahler[i]++
		}
		j := int(flow[i])
		if j < 0 {
			continue
		}
		if main[j] < 0 || area[i] > area[main[j]] {
			main[j] = i
		}
		if strahler[i] > maxOrder[j] {
			maxOrder[j], ties[j] = strahler[i], 1
		} else if strahler[i] == maxOrder[j] {
			ties[j]++
		}
	}
	g.Fields["catchment"] = area
	for _, key := range []string{"basin", "wetland", "spring", "network"} {
		g.Fields[key] = make([]float64, n)
	}
	d := &model.HydrologyDiagnostics{Classes: map[string]model.RiverStatistics{}, LakeKinds: map[string]int{}, TargetCoverage: .22}
	p.Diagnostics.Hydrology = d
	for _, i := range order {
		if !p.Emerged(i) {
			continue
		}
		j := int(flow[i])
		basin[i] = i
		if j >= 0 && p.Emerged(j) {
			basin[i] = basin[j]
		} else {
			d.BasinAreasKM2 = append(d.BasinAreasKM2, area[i])
			landArea += area[i]
		}
		g.Fields["basin"][i] = float64(basin[i] + 1)
		if q[i] < threshold {
			continue
		}
		owners[i] = i
		if j >= 0 && p.Emerged(j) && main[j] == i {
			owners[i] = owners[j]
		}
		g.Fields["network"][i] = 1
	}
	d.Basins = len(d.BasinAreasKM2)
	sort.Sort(sort.Reverse(sort.Float64Slice(d.BasinAreasKM2)))
	cellArea := func(i int) float64 {
		return model.Tau / float64(g.Width) * math.Pi / float64(g.Height) * math.Cos(g.Point(i%g.Width, i/g.Width)[1]) * p.Spec.RadiusKM * p.Spec.RadiusKM
	}
	networkArea := 0.
	major := math.Max(landArea*.002, 4*model.Tau/float64(g.Width)*math.Pi/float64(g.Height)*p.Spec.RadiusKM*p.Spec.RadiusKM)
	for i := range area {
		if i%512 == 0 && ctx.Err() != nil {
			return ctx.Err()
		}
		if !p.Emerged(i) {
			continue
		}
		if q[i] >= threshold {
			networkArea += cellArea(i)
		}
		j := int(flow[i])
		slope := 1.
		if j >= 0 {
			slope = (level[i] - level[j]) / math.Max(1, distanceKM(g.Point(i%g.Width, i/g.Width), g.Point(j%g.Width, j/g.Width), p.Spec.RadiusKM)*1000)
		}
		if bed[i] < 600 && slope < .001 && g.Fields["moisture"][i] > .5 && (q[i] > threshold || g.Fields["soil"][i] > 140) {
			g.Fields["wetland"][i] = 1
		}
		if owners[i] < 0 || main[i] >= 0 && owners[main[i]] == owners[i] {
			continue
		}
		r := model.River{ID: fmt.Sprintf("river-%d", owners[i]), Destination: "ocean", Basin: basin[i] + 1, Order: strahler[owners[i]]}
		if bed[i] < 800 {
			d.LowlandHeadwaters++
		}
		if g.Fields["soil"][i] > 110 && q[i] > threshold && g.Fields["roughness"][i] > .15 {
			g.Fields["spring"][i] = 1
			d.Springs++
		}
		lastLand := i
		for current, steps := i, 0; steps < n; steps++ {
			pt := g.Point(current%g.Width, current/g.Width)
			discharge := q[current]
			terminal := !p.Emerged(current) || owners[current] != owners[i]
			if terminal {
				discharge = q[lastLand]
			}
			r.Path = append(r.Path, pt)
			r.Surface = append(r.Surface, level[current])
			r.Bed = append(r.Bed, level[current]-math.Max(1, math.Pow(discharge, .3)*.3))
			r.Discharge = append(r.Discharge, discharge)
			r.Width = append(r.Width, math.Max(3, math.Sqrt(discharge)*2.4))
			if terminal {
				if p.Emerged(current) {
					r.Destination = fmt.Sprintf("river-%d", owners[current])
				}
				break
			}
			lastLand = current
			next := int(flow[current])
			if next < 0 {
				r.Destination = "basin"
				break
			}
			current = next
		}
		r.CatchmentKM2 = area[lastLand]
		r.Class = "stream"
		if r.CatchmentKM2 >= major {
			r.Class = "major"
		} else if r.CatchmentKM2 >= major*.12 {
			r.Class = "regional"
		}
		if g.Fields["precipitation"][lastLand] < g.Fields["pet"][lastLand]*.35 && q[lastLand]/math.Max(1, area[lastLand]) < .002 {
			r.Class = "ephemeral"
		}
		if len(r.Path) < 2 {
			continue
		}
		if r.Destination == "ocean" {
			trimOcean(p, &r)
		}
		r = refineRiver(p, r)
		for k := 1; k < len(r.Path); k++ {
			r.LengthKM += distanceKM(r.Path[k-1], r.Path[k], p.Spec.RadiusKM)
		}
		p.Rivers = append(p.Rivers, r)
	}
	d.NetworkCoverage = networkArea / math.Max(1, landArea)
	// Connected wetland complexes, not a count of individual raster samples.
	seen := make([]bool, n)
	for i, v := range g.Fields["wetland"] {
		if v == 0 || seen[i] {
			continue
		}
		d.Wetlands++
		stack := []int{i}
		seen[i] = true
		for len(stack) > 0 {
			j := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, k := range neighbors8(j, g.Width, g.Height) {
				if !seen[k] && g.Fields["wetland"][k] > 0 {
					seen[k] = true
					stack = append(stack, k)
				}
			}
		}
	}
	addOxbows(p)
	d.Lakes = len(p.Lakes)
	for _, l := range p.Lakes {
		d.LakeKinds[l.Kind]++
	}
	addDistributaries(p)
	byID := map[string]model.River{}
	for _, r := range p.Rivers {
		byID[r.ID] = r
		st := d.Classes[r.Class]
		st.Count++
		st.LengthKM += r.LengthKM
		d.Classes[r.Class] = st
		d.LengthsKM = append(d.LengthsKM, r.LengthKM)
		d.CatchmentsKM2 = append(d.CatchmentsKM2, r.CatchmentKM2)
	}
	for _, r := range p.Rivers {
		visited := map[string]bool{}
		id := r.ID
		for {
			if id == "ocean" || id == "basin" {
				break
			}
			a, ok := byID[id]
			if !ok || visited[id] {
				d.InvalidTerminations++
				break
			}
			visited[id] = true
			id = a.Destination
		}
	}
	sort.Float64s(d.LengthsKM)
	sort.Float64s(d.CatchmentsKM2)
	if d.NetworkCoverage < d.TargetCoverage*.5 {
		p.Diagnostics.Warnings = append(p.Diagnostics.Warnings, "Climate permits only sparse drainage coverage; inspect hydrology diagnostics.")
	}
	return nil
}

// Abandoned meanders are small saved channels beside mature lowland rivers.
// The bed test rejects hills/ridges, and the offset keeps them off the live arm.
func addOxbows(p *model.Planet) {
	for _, r := range p.Rivers {
		if r.Class != "major" && r.Class != "regional" {
			continue
		}
		count := 0
		for i := 8; i+8 < len(r.Path) && count < 2; i += 16 {
			if r.Surface[i] > 200 || r.Surface[i-4]-r.Surface[i+4] > 3 {
				continue
			}
			center := r.Path[i]
			if p.Grid.At("moisture", center[0], center[1]) < .5 {
				continue
			}
			width := math.Min(1000, math.Max(30, r.Width[i]*.6))
			scale := p.Spec.RadiusKM * 1000
			radius := math.Max(500, r.Width[i]*5) / scale
			c := math.Max(.05, math.Cos(center[1]))
			center[0] = model.Wrap(center[0] + radius*2/c)
			lake := model.Lake{Kind: "oxbow", Center: center, Radius: radius * 2, Level: r.Surface[i], Width: width}
			valid := true
			for k := 0; k <= 16; k++ {
				a := -math.Pi*.75 + float64(k)/16*math.Pi*1.5
				pt := model.Point{model.Wrap(center[0] + radius*math.Cos(a)/c), center[1] + radius*math.Sin(a)}
				if math.Abs(pt[1]) > math.Pi/2 || p.Surface.At(p.Surface.Coast, pt[0], pt[1]) < 0 || p.Surface.At(p.Surface.Bed, pt[0], pt[1]) > lake.Level+3 {
					valid = false
					break
				}
				lake.Path = append(lake.Path, pt)
			}
			if valid {
				p.Lakes = append(p.Lakes, lake)
				count++
			}
		}
	}
}

func distanceKM(a, b model.Point, radius float64) float64 {
	h := math.Pow(math.Sin((b[1]-a[1])/2), 2) + math.Cos(a[1])*math.Cos(b[1])*math.Pow(math.Sin(model.Wrap(b[0]-a[0])/2), 2)
	return 2 * math.Asin(math.Sqrt(model.Clamp(h, 0, 1))) * radius
}

// Terminate at the physical shoreline, not at a coarse ocean cell centre.
func trimOcean(p *model.Planet, r *model.River) {
	s := p.Surface
	for i := 1; i < len(r.Path); i++ {
		b := r.Path[i]
		if s.At(s.Coast, b[0], b[1]) > 0 {
			continue
		}
		a := r.Path[i-1]
		lo, hi := 0., 1.
		for k := 0; k < 30; k++ {
			t := (lo + hi) * .5
			lon, lat := a[0]+model.Wrap(b[0]-a[0])*t, a[1]+(b[1]-a[1])*t
			if s.At(s.Coast, lon, lat) > 0 {
				lo = t
			} else {
				hi = t
			}
		}
		r.Path = r.Path[:i+1]
		r.Bed = r.Bed[:i+1]
		r.Surface = r.Surface[:i+1]
		r.Discharge = r.Discharge[:i+1]
		r.Width = r.Width[:i+1]
		r.Path[i] = model.Point{model.Wrap(a[0] + model.Wrap(b[0]-a[0])*hi), a[1] + (b[1]-a[1])*hi}
		r.Surface[i] = 0
		r.Bed[i] = -math.Max(1, math.Pow(r.Discharge[i], .3)*.3)
		return
	}
}

// A distributary is accepted only on a low, descending coastal fan. Both arms
// share the same junction and divide the parent's discharge without creating water.
func addDistributaries(p *model.Planet) {
	s := p.Surface
	original := len(p.Rivers)
	for ri := 0; ri < original; ri++ {
		r := &p.Rivers[ri]
		if r.Class != "major" || r.Destination != "ocean" || len(r.Path) < 12 {
			continue
		}
		start := len(r.Path) - 9
		if r.Surface[start] > 30 {
			continue
		}
		a, b := r.Path[start], r.Path[len(r.Path)-1]
		dx, dy := model.Wrap(b[0]-a[0]), b[1]-a[1]
		for _, sign := range []float64{-1, 1} {
			branch := model.River{ID: r.ID + "-delta", Parent: r.ID, Destination: "ocean", Class: "distributary", Basin: r.Basin, Order: r.Order, CatchmentKM2: r.CatchmentKM2 * .3}
			valid := true
			for k := 0; k <= 16; k++ {
				t := float64(k) / 16
				pt := model.Point{model.Wrap(a[0] + dx*t - dy*.65*sign*t*t), a[1] + dy*t + dx*.65*sign*t*t}
				z := r.Surface[start] * (1 - t)
				if math.Abs(pt[1]) > math.Pi/2 || s.At(s.Bed, pt[0], pt[1]) > z+2 {
					valid = false
					break
				}
				branch.Path = append(branch.Path, pt)
				branch.Surface = append(branch.Surface, z)
				branch.Bed = append(branch.Bed, z-2)
				q := r.Discharge[start] * .3
				branch.Discharge = append(branch.Discharge, q)
				branch.Width = append(branch.Width, math.Max(3, math.Sqrt(q)*2.4))
			}
			if !valid || s.At(s.Coast, branch.Path[len(branch.Path)-1][0], branch.Path[len(branch.Path)-1][1]) > 0 {
				continue
			}
			trimOcean(p, &branch)
			for k := 1; k < len(branch.Path); k++ {
				branch.LengthKM += distanceKM(branch.Path[k-1], branch.Path[k], p.Spec.RadiusKM)
			}
			// Store the explicit diversion on a new downstream reach. Each reach
			// remains non-decreasing; the split is a graph junction with two arms.
			arm := *r
			arm.ID = r.ID + "-mouth"
			arm.Parent = r.ID
			arm.Path = append([]model.Point(nil), r.Path[start:]...)
			arm.Bed = append([]float64(nil), r.Bed[start:]...)
			arm.Surface = append([]float64(nil), r.Surface[start:]...)
			arm.Discharge = append([]float64(nil), r.Discharge[start:]...)
			arm.Width = append([]float64(nil), r.Width[start:]...)
			for k := range arm.Discharge {
				arm.Discharge[k] -= r.Discharge[start] * .3
				arm.Width[k] = math.Max(3, math.Sqrt(arm.Discharge[k])*2.4)
			}
			arm.LengthKM = 0
			for k := 1; k < len(arm.Path); k++ {
				arm.LengthKM += distanceKM(arm.Path[k-1], arm.Path[k], p.Spec.RadiusKM)
			}
			r.Destination = arm.ID
			r.Path = r.Path[:start+1]
			r.Bed = r.Bed[:start+1]
			r.Surface = r.Surface[:start+1]
			r.Discharge = r.Discharge[:start+1]
			r.Width = r.Width[:start+1]
			r.LengthKM -= arm.LengthKM
			p.Rivers = append(p.Rivers, arm, branch)
			p.Diagnostics.Hydrology.Distributaries++
			break
		}
	}
}
