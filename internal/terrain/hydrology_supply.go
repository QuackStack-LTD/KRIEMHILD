package terrain

import "math"

// Establish geology, snow storage and the annual water budget before routing.
func (e *Environment) prepareHydrology() {
	n, w := len(e.Mask), e.Options.Columns
	for _, name := range HydrologyFields {
		e.Fields[name] = make([]float64, n)
	}
	for _, name := range []string{"drainageDistance", "riverClass", "riverSystem", "catchmentKM2", "dischargeM3s"} {
		e.Fields[name] = make([]float64, n)
	}
	e.Hydrology = &HydrologyState{Version: 1, NetworkVersion: 2, Units: "1000 mm per base cell per year"}
	if e.Geography != nil {
		e.Hydrology.NetworkVersion = 3
		e.Hydrology.Units = "1000 mm over mean cell area per year; SI discharge and catchment fields stored separately"
	}
	sample := noise(Seed(e.Options.Seed))
	f, set := e.get, e.set
	for i := range e.Mask {
		rock := 0
		g := sample(float64(i%w)/float64(w)*6+40, float64(i/w)/float64(e.Options.Rows)*6+50)
		if g > .42 {
			rock = 1
		}
		if g > .52 {
			rock = 2
		}
		if g > .65 {
			rock = 3
		}
		if g > .8 && f("aridity", i) > .6 {
			rock = 5
		}
		if f("volcano", i) > .55 {
			rock = 4
		}
		perm := []float64{.18, .12, .72, .85, .65, .3}[rock]
		perm = clamp(perm+f("tectonicStress", i)*.12, .05, .95)
		if e.Geology != nil && len(e.Geology.Cells) == n {
			c := e.Geology.Cells[i]
			formation := c.Basement
			if c.Cover >= 0 {
				formation = c.Cover
			}
			if formation >= 0 {
				strata := e.Geology.Formations[formation].Strata
				if len(strata) > 0 {
					perm = clamp(strata[0].Permeability+f("tectonicStress", i)*.12, .05, .95)
				}
			}
		}
		set("lithology", i, float64(rock))
		set("permeability", i, perm)
		rain := math.Max(0, f("precipitation", i))
		cold := clamp((2-f("winter", i))/15, 0, 1)
		snow := rain * cold
		melt := math.Min(snow, math.Max(0, f("summer", i))*120)
		set("snow", i, clamp((snow-melt)/700, 0, 1))
		set("snowmelt", i, melt/1000)
		if f("elevation", i) > 1800 || math.Abs(f("latitude", i)) > 60 {
			set("glacier", i, f("snow", i))
		}
		// Melt draws from current snowfall and a bounded old ice reserve proxy.
		glacierMelt := math.Min(rain*.12, f("glacier", i)*math.Max(0, f("summer", i))*14)
		set("glacierMelt", i, glacierMelt/1000)
		available := math.Max(0, rain-(snow-melt)*.65) + glacierMelt
		cover := clamp(f("moisture", i)*(f("temperature", i)+10)/30, 0, 1)
		et := math.Min(available*.8, math.Max(40, (f("temperature", i)+20)*12)*(.6+.4*cover))
		liquid := math.Max(0, available-et)
		recharge := liquid * perm * .5
		baseflow := recharge * .65
		runoff := liquid - recharge + baseflow
		set("evapotranspiration", i, et/1000)
		set("recharge", i, recharge/1000)
		set("baseflow", i, baseflow/1000*e.areaWeight(i))
		set("runoff", i, runoff/1000*e.areaWeight(i))
		lat := math.Abs(f("latitude", i))
		share := .5 + .24*math.Exp(-math.Pow((lat-13)/13, 2)) - .28*math.Exp(-math.Pow((lat-36)/8, 2))*math.Exp(-f("oceanDistance", i)/8)
		seasonal := clamp(2*math.Min(share, 1-share)*(1-f("aridity", i)*.85), .02, 1)
		set("dryRunoff", i, (baseflow+(runoff-baseflow)*seasonal*.45)/1000*e.areaWeight(i))
		set("geothermal", i, 25+90*f("tectonicStress", i)+140*f("volcano", i))
		if f("ocean", i) > 0 {
			set("runoff", i, 0)
			set("dryRunoff", i, 0)
			set("recharge", i, 0)
			set("baseflow", i, 0)
		}
	}
}

// Kahn order keeps every upstream contribution, including distant wet mountains
// feeding desert rivers. Losses are applied at lake outlets, never by biome.
func (e *Environment) accumulateWater() []int {
	n := len(e.Mask)
	degree := make([]int, n)
	maxOrder := make([]int, n)
	ties := make([]int, n)
	q := []int{}
	f, set := e.get, e.set
	for i := 0; i < n; i++ {
		set("accumulation", i, f("runoff", i))
		set("dryDischarge", i, f("dryRunoff", i))
		set("groundFlow", i, f("baseflow", i))
		set("catchmentArea", i, 1)
		if e.Geography != nil {
			set("catchmentKM2", i, e.cellArea(i))
			set("dischargeM3s", i, f("runoff", i)/e.areaWeight(i)*e.cellArea(i)*1e6/(365.25*86400))
		}
		set("streamOrder", i, 1)
		if j := int(f("flow", i)); j >= 0 {
			degree[j]++
		}
	}
	for i, d := range degree {
		if d == 0 {
			q = append(q, i)
		}
	}
	for head := 0; head < len(q); head++ {
		i := q[head]
		order := max(1, maxOrder[i])
		if ties[i] > 1 {
			order++
		}
		set("streamOrder", i, float64(order))
		if j := int(f("flow", i)); j >= 0 {
			loss := math.Min(f("accumulation", i), f("hydroLoss", i))
			ratio := 1 - loss/math.Max(1e-12, f("accumulation", i))
			set("accumulation", j, f("accumulation", j)+f("accumulation", i)-loss)
			set("dryDischarge", j, f("dryDischarge", j)+f("dryDischarge", i)*ratio)
			set("groundFlow", j, f("groundFlow", j)+f("groundFlow", i)*ratio)
			set("catchmentArea", j, f("catchmentArea", j)+f("catchmentArea", i))
			if e.Geography != nil {
				set("catchmentKM2", j, f("catchmentKM2", j)+f("catchmentKM2", i))
				set("dischargeM3s", j, f("dischargeM3s", j)+f("dischargeM3s", i)*ratio)
			}
			if order > maxOrder[j] {
				maxOrder[j] = order
				ties[j] = 1
			} else if order == maxOrder[j] {
				ties[j]++
			}
			degree[j]--
			if degree[j] == 0 {
				q = append(q, j)
			}
		}
	}
	for k := len(q) - 1; k >= 0; k-- {
		i := q[k]
		j := int(f("flow", i))
		root := float64(i)
		if j >= 0 {
			root = f("watershed", j)
		}
		set("watershed", i, root)
	}
	return q
}

func (e *Environment) drainageNeighbors(i int) []int {
	w, h := e.Options.Columns, e.Options.Rows
	out := e.neighbors(i)
	for _, d := range [][2]int{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		x, y := i%w+d[0], i/w+d[1]
		if e.Geography != nil && e.Geography.WrapX {
			x = (x + w) % w
		}
		if x < 0 || x >= w || y < 0 || y >= h {
			continue
		}
		out = append(out, y*w+x)
	}
	return out
}
func (e *Environment) steepestDrain(i, parent int, rank []int, surface func(int) float64) int {
	z := surface(i)
	best := -1
	slope := 0.
	w := e.Options.Columns
	for _, j := range e.drainageNeighbors(i) {
		dx, dy := j%w-i%w, j/w-i/w
		if e.Geography != nil && e.Geography.WrapX {
			if dx > w/2 {
				dx -= w
			}
			if dx < -w/2 {
				dx += w
			}
		}
		length := math.Hypot(float64(dx), float64(dy))
		if e.Geography != nil {
			length = e.stepKM(i, j)
		}
		horizontal := i + dx
		if i%w+dx < 0 {
			horizontal = i + w - 1
		}
		if i%w+dx >= w {
			horizontal = i - w + 1
		}
		v := surface(j)
		if dx != 0 && dy != 0 && surface(horizontal) > z && surface(i+dy*w) > z {
			continue
		} // Do not cut across a ridge corner.
		// A land channel cannot shortcut diagonally through a marine/lake corner.
		if dx != 0 && dy != 0 && e.get("waterBody", i) == 0 && e.get("waterBody", j) == 0 && (e.get("waterBody", horizontal) > 0 || e.get("waterBody", i+dy*w) > 0) {
			continue
		}
		drop := (z - v) / length
		if drop > slope || drop == slope && drop >= 0 && rank[j] < rank[i] && (best < 0 || rank[j] < rank[best]) {
			best, slope = j, drop
		}
	}
	return best
}
