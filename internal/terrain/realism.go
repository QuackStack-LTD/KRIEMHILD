package terrain

// Lightweight causal models, not a numerical Earth simulation. All quantities
// are derived from the seeded environmental state; rendering never places them.
import (
	"math"
	"sort"
)

var ClimateZones = []string{"Ocean", "Rainforest", "Monsoon", "Savanna", "Hot desert", "Cold desert", "Mediterranean", "Oceanic", "Continental", "Boreal", "Tundra", "Ice cap", "Alpine"}
var RealismFields = []string{"climate", "summerRain", "winterRain", "growingSeason", "drainageElevation", "river", "freshwaterDistance", "rockType", "layering", "caprock", "erosion", "sandSupply", "sandTransport", "deposition", "vegetation", "substrate", "snowBalance", "duneField", "hotspot", "vent", "cultivated", "village"}

type River struct {
	ID        int     `json:"id"`
	Source    int     `json:"source"`
	Mouth     int     `json:"mouth"`
	Path      []int   `json:"path"`
	Discharge float64 `json:"discharge"`
}
type Volcano struct {
	ID     int    `json:"id"`
	Cell   int    `json:"cell"`
	Origin string `json:"origin"`
	Active bool   `json:"active"`
	Lava   []int  `json:"lava"`
	Ash    []int  `json:"ash"`
}
type Region struct {
	ID          int     `json:"id"`
	Cells       []int   `json:"cells"`
	Orientation float64 `json:"orientation"`
}
type Settlement struct {
	Cell  int   `json:"cell"`
	Farms []int `json:"farms"`
}
type WorldEntities struct {
	Rivers      []River      `json:"rivers"`
	Volcanoes   []Volcano    `json:"volcanoes"`
	DuneFields  []Region     `json:"duneFields"`
	Settlements []Settlement `json:"settlements"`
}

func (e *Environment) realisticWind() {
	for i := range e.Mask {
		lat := e.get("latitude", i)
		a := math.Abs(lat)
		// Smooth transitions at the edges of the circulation cells.
		westerly := smooth((a-25)/10) * (1 - smooth((a-55)/10))
		x := -1 + 2*westerly
		if e.Options.WindDirection != 0 {
			x = e.Options.WindDirection
		}
		y := (.38 - .60*westerly) * math.Tanh(lat/6)
		length := math.Hypot(x, y)
		e.set("windX", i, x/length)
		e.set("windY", i, y/length)
	}
}

func (e *Environment) classifyClimate(i int) int {
	f := e.get
	if e.Mask[i] == 0 && f("lake", i) == 0 {
		return 0
	}
	if f("summer", i) < 0 {
		return 11
	}
	if f("elevation", i) > 1800 && f("summer", i) < 10 {
		return 12
	}
	if f("summer", i) < 10 {
		return 10
	}
	if f("moisture", i) < .28 {
		if f("temperature", i) >= 18 {
			return 4
		}
		return 5
	}
	if f("winter", i) > 18 {
		if f("precipitation", i) > 1800 && math.Min(f("summerRain", i), f("winterRain", i)) > 600 {
			return 1
		}
		if f("precipitation", i) > 1200 {
			return 2
		}
		return 3
	}
	if f("temperature", i) < 5 && f("winter", i) < -3 {
		return 9
	}
	if f("summerRain", i) < f("winterRain", i)*.5 && f("summer", i) > 20 && f("winter", i) > 0 {
		return 6
	}
	if f("winter", i) > -3 && f("summer", i)-f("winter", i) < 28 {
		return 7
	}
	return 8
}

func (e *Environment) realisticEntities(sample func(float64, float64) float64) {
	w, h := e.Options.Columns, e.Options.Rows
	f, set := e.get, e.set
	for i := range e.Mask {
		lat := math.Abs(f("latitude", i))
		// Seasonal circulation proxy: tropical wet summers and subtropical
		// coastal dry summers. Seasons refer to the local hemisphere.
		share := .5 + .24*math.Exp(-math.Pow((lat-13)/13, 2)) - .28*math.Exp(-math.Pow((lat-36)/8, 2))*math.Exp(-f("oceanDistance", i)/8)
		set("summerRain", i, f("precipitation", i)*share)
		set("winterRain", i, f("precipitation", i)*(1-share))
		set("growingSeason", i, clamp((f("summer", i)-5)/20, 0, 1)*clamp((f("temperature", i)+10)/20, 0, 1))
		set("climate", i, float64(e.classifyClimate(i)))
		geology := sample(float64(i%w)/float64(w)*6+40, float64(i/w)/float64(h)*6+50)
		rock := 0. // crystalline, sedimentary, volcanic
		if geology > .42 {
			rock = 1
		}
		if f("volcano", i) > .55 {
			rock = 2
		}
		set("rockType", i, rock)
		if rock == 1 {
			set("layering", i, geology)
			set("caprock", i, clamp((sample(float64(i%w)*.09+91, float64(i/w)*.09)-.35)*2, 0, 1))
		}
		set("erosion", i, clamp(f("slope", i)*(.3+f("precipitation", i)/1600)+math.Log1p(f("accumulation", i))*.08, 0, 1))
		set("vegetation", i, clamp(f("moisture", i)*f("growingSeason", i), 0, 1))
		// Exposed rock provides attachment; mobile sediment inhibits coral.
		set("substrate", i, clamp(.9-f("sediment", i)*.7-f("slope", i)*.6, 0, 1))
		set("mesa", i, 0)
		if e.Mask[i] != 0 && f("elevation", i) > 400 && f("slope", i) > .06 && f("slope", i) < .5 {
			set("mesa", i, f("layering", i)*f("caprock", i)*f("aridity", i)*clamp(f("erosion", i)*3, 0, 1))
		}
		set("snowBalance", i, f("precipitation", i)*clamp((2-f("winter", i))/15, 0, 1)-math.Max(0, f("summer", i))*120)
		set("reef", i, f("reef", i)*f("substrate", i)*clamp(1-f("slope", i)*2, 0, 1))
		set("lava", i, 0)
		set("ash", i, 0)
	}
	e.buildRivers()
	e.transportSand()
	e.buildVolcanoes()
	e.placeSettlements()
}

func (e *Environment) buildRivers() {
	w, h := e.Options.Columns, e.Options.Rows
	f, set := e.get, e.set
	visible := e.VisibleChannels()
	upstream := make([]int, len(e.Mask))
	fresh := make([]int, len(e.Mask))
	for i := range fresh {
		if f("lake", i) > 0 {
			fresh[i] = 1
		}
		if e.Mask[i] != 0 && f("lake", i) == 0 && visible[i] {
			set("river", i, clamp(math.Log1p(f("accumulation", i))/8, 0, 1))
			fresh[i] = 1
		}
	}
	for i := range fresh {
		if f("river", i) > 0 {
			j := int(f("flow", i))
			if j >= 0 {
				upstream[j]++
			}
		}
	}
	for i := range fresh {
		if f("river", i) == 0 || upstream[i] == 1 || f("flow", i) < 0 {
			continue
		}
		r := River{ID: len(e.Entities.Rivers) + 1, Source: i, Path: []int{i}}
		at := i
		for steps := 0; steps < len(e.Mask); steps++ {
			j := int(f("flow", at))
			if j < 0 {
				break
			}
			r.Path = append(r.Path, j)
			at = j
			if e.Mask[j] == 0 || f("lake", j) > 0 || upstream[j] > 1 {
				break
			}
		}
		r.Mouth = at
		r.Discharge = f("accumulation", at)
		e.Entities.Rivers = append(e.Entities.Rivers, r)
	}
	e.Fields["freshwaterDistance"] = distance(fresh, w, h, 1)
	for i := range fresh {
		if e.Mask[i] == 0 {
			continue
		}
		water := math.Exp(-f("freshwaterDistance", i) / 2)
		set("groundwater", i, math.Max(f("groundwater", i), water*.85))
		set("fertility", i, clamp(f("fertility", i)+water*.3*(1-f("slope", i)), 0, 1))
		set("wetland", i, 0)
		set("oasis", i, 0)
		if f("lake", i) == 0 && f("slope", i) < .15 && f("temperature", i) > -5 {
			set("wetland", i, f("groundwater", i)*(1-f("slope", i)/.15)*math.Max(water, f("moisture", i)*.7))
		}
		if f("aridity", i) > .55 && f("groundwater", i) > .5 {
			set("oasis", i, f("aridity", i)*f("groundwater", i))
		}
	}
}

func (e *Environment) transportSand() {
	w, h := e.Options.Columns, e.Options.Rows
	f, set := e.get, e.set
	for i := range e.Mask {
		set("dune", i, 0)
		if e.Mask[i] == 0 || f("lake", i) > 0 {
			continue
		}
		source := f("layering", i) * f("erosion", i) * .5
		if f("oceanDistance", i) <= 1 {
			source += .5
		}
		if f("freshwaterDistance", i) < 2 {
			source += .35
		}
		set("sandSupply", i, clamp(source, 0, 1)*(1-f("moisture", i))*(1-f("vegetation", i)))
	}
	load := append([]float64{}, e.Fields["sandSupply"]...)
	for pass := 0; pass < 8; pass++ {
		next := make([]float64, len(load))
		for i, v := range load {
			if e.Mask[i] == 0 || f("lake", i) > 0 {
				continue
			}
			// Split transport into the two wind components, retaining fractional Y.
			wx, wy := f("windX", i), f("windY", i)
			dx, dy := 1, 1
			if wx < 0 {
				dx = -1
			}
			if wy < 0 {
				dy = -1
			}
			direction := []struct {
				x, y  int
				share float64
			}{{dx, 0, math.Abs(wx)}, {0, dy, math.Abs(wy)}}
			for _, d := range direction {
				x, y := i%w+d.x, i/w+d.y
				if x < 0 || x >= w || y < 0 || y >= h {
					continue
				}
				j := y*w + x
				if e.Mask[j] == 0 || f("lake", j) > 0 {
					continue
				}
				flux := v * d.share / (math.Abs(wx) + math.Abs(wy)) * f("windStrength", i)
				trap := clamp(.15+math.Max(0, f("elevation", j)-f("elevation", i))/1000+f("vegetation", j)*.6, 0, .9)
				set("sandTransport", j, f("sandTransport", j)+flux/8)
				set("deposition", j, f("deposition", j)+flux*trap/8)
				next[j] += flux * (1 - trap)
			}
		}
		for i := range load {
			load[i] = next[i] + f("sandSupply", i)*.3
		}
	}
	for i := range e.Mask {
		if e.Mask[i] != 0 && f("aridity", i) > .5 && f("slope", i) < .25 && f("summer", i) > 5 {
			set("dune", i, clamp(f("deposition", i)*10*f("aridity", i)*(1-f("vegetation", i)), 0, 1))
		}
	}
	for i := range e.Mask {
		if f("dune", i) <= .13 || f("duneField", i) > 0 {
			continue
		}
		id := len(e.Entities.DuneFields) + 1
		q := []int{i}
		set("duneField", i, float64(id))
		sx, sy := 0., 0.
		for head := 0; head < len(q); head++ {
			c := q[head]
			sx += f("windX", c)
			sy += f("windY", c)
			for _, j := range nb(c, w, h) {
				if f("dune", j) > .13 && f("duneField", j) == 0 {
					set("duneField", j, float64(id))
					q = append(q, j)
				}
			}
		}
		a := math.Atan2(sy, sx) + math.Pi/2
		for _, c := range q {
			set("duneOrientation", c, a)
		}
		e.Entities.DuneFields = append(e.Entities.DuneFields, Region{id, q, a})
	}
}

func (e *Environment) buildVolcanoes() {
	w, h := e.Options.Columns, e.Options.Rows
	f, set := e.get, e.set
	activity := noise(Seed(e.Options.Seed) ^ 0x45e217)
	for i := range e.Mask {
		if f("volcano", i) < .48 {
			continue
		}
		peak := true
		// Regional maxima make discrete, spaced vents along an arc instead of
		// interpreting every small grid-scale fluctuation as a volcano.
		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				x, y := i%w+dx, i/w+dy
				if x < 0 || x >= w || y < 0 || y >= h {
					continue
				}
				j := y*w + x
				if f("volcano", j) > f("volcano", i) || (f("volcano", j) == f("volcano", i) && j < i) {
					peak = false
				}
			}
		}
		if !peak {
			continue
		}
		origin := "subduction"
		if f("hotspot", i) > .48 {
			origin = "hotspot"
		} else if f("boundary", i) == 2 {
			origin = "rift"
		}
		v := Volcano{ID: len(e.Entities.Volcanoes) + 1, Cell: i, Origin: origin, Active: activity(float64(i%w)*1.71, float64(i/w)*1.37) > clamp(.68-(e.Options.Volcanism-1)*.2, .2, .85)}
		set("vent", i, float64(v.ID))
		// Submarine vents are recorded, but cannot paint land lava or ash.
		if v.Active && e.Mask[i] != 0 && f("lake", i) == 0 {
			at := i
			for step := 0; step < 9; step++ {
				v.Lava = append(v.Lava, at)
				set("lava", at, math.Max(f("lava", at), 1-float64(step)/10))
				j := -1
				z := f("elevation", at)
				for _, c := range nb(at, w, h) {
					if f("elevation", c) < z {
						j = c
						z = f("elevation", c)
					}
				}
				if j < 0 || e.Mask[j] == 0 || f("lake", j) > 0 {
					break
				}
				at = j
			}
			x, y := float64(i%w), float64(i/w)
			for step := 1; step <= 12; step++ {
				at := int(round(y))*w + int(round(x))
				x += f("windX", at)
				y += f("windY", at)
				if round(x) < 0 || round(x) >= float64(w) || round(y) < 0 || round(y) >= float64(h) {
					break
				}
				j := int(round(y))*w + int(round(x))
				if e.Mask[j] == 0 || f("lake", j) > 0 {
					continue
				}
				v.Ash = append(v.Ash, j)
				set("ash", j, math.Max(f("ash", j), 1-float64(step)/14))
			}
		}
		e.Entities.Volcanoes = append(e.Entities.Volcanoes, v)
	}
}

func (e *Environment) placeSettlements() {
	w, h := e.Options.Columns, e.Options.Rows
	f, set := e.get, e.set
	order := []int{}
	for i := range e.Mask {
		set("farmland", i, 0)
		set("settlement", i, 0)
		if e.Mask[i] == 0 || f("lake", i) > 0 || f("slope", i) > .18 || f("temperature", i) < 2 || f("temperature", i) > 30 || f("growingSeason", i) < .4 || f("glacier", i) > .1 || f("lava", i) > .1 || f("ash", i) > .2 || f("wetland", i) > .55 {
			continue
		}
		water := math.Max(f("moisture", i)*.7, math.Exp(-f("freshwaterDistance", i)/3))
		set("farmland", i, f("fertility", i)*water*(1-f("slope", i)/.2)*f("growingSeason", i))
	}
	for i := range e.Mask {
		if f("farmland", i) < .15 || f("freshwaterDistance", i) > 2 {
			continue
		}
		support := 0.
		for _, j := range nb(i, w, h) {
			support += f("farmland", j)
		}
		set("settlement", i, f("farmland", i)*clamp(support, 0, 1))
		if f("settlement", i) > .08 {
			order = append(order, i)
		}
	}
	sort.SliceStable(order, func(a, b int) bool { return f("settlement", order[a]) > f("settlement", order[b]) })
	blocked := make([]bool, len(e.Mask))
	for _, i := range order {
		if blocked[i] {
			continue
		}
		s := Settlement{Cell: i}
		for _, j := range nb(i, w, h) {
			if f("farmland", j) > .15 && f("village", j) == 0 {
				s.Farms = append(s.Farms, j)
			}
		}
		if len(s.Farms) == 0 {
			continue
		}
		set("village", i, 1)
		for _, j := range s.Farms {
			set("cultivated", j, 1)
		}
		e.Entities.Settlements = append(e.Entities.Settlements, s)
		for dy := -4; dy <= 4; dy++ {
			for dx := -4; dx <= 4; dx++ {
				x, y := i%w+dx, i/w+dy
				if x >= 0 && x < w && y >= 0 && y < h {
					blocked[y*w+x] = true
				}
			}
		}
	}
}

func (e *Environment) realisticCandidates(i int) []string {
	f := e.get
	if f("lake", i) > 0 {
		if f("waterDepth", i) > 150 {
			return []string{"deep_water"}
		}
		return []string{"water"}
	}
	if e.Mask[i] == 0 {
		if f("reefType", i) > 0 {
			return []string{"reef"}
		}
		if f("bathymetry", i) > 700 {
			return []string{"deep_water"}
		}
		return []string{"water"}
	}
	for _, x := range []struct {
		field, id string
		threshold float64
	}{{"lava", "lava", .2}, {"ash", "ash", .2}, {"glacier", "glacier", .35}, {"snow", "snow", .3}, {"village", "village", .5}, {"cultivated", "farmland", .5}} {
		if f(x.field, i) > x.threshold {
			return []string{x.id}
		}
	}
	if e.Fields["beach"] != nil && f("beach", i) > .8 {
		return []string{"sand", "grass"}
	}
	if f("wetland", i) > .55 {
		return []string{"swamp"}
	}
	if f("oasis", i) > .5 {
		return []string{"oasis"}
	}
	if f("dune", i) > .13 {
		return []string{"dunes", "desert"}
	}
	if f("mesa", i) > .18 {
		return []string{"mesa", "hills"}
	}
	if f("elevation", i) > 2300 {
		return []string{"mountain"}
	}
	if f("elevation", i) > 900 && f("slope", i) > .15 {
		return []string{"hills", "mountain"}
	}
	switch int(f("climate", i)) {
	case 1, 2:
		return []string{"jungle"}
	case 4, 5:
		return []string{"desert"}
	case 9:
		return []string{"taiga"}
	case 10, 11, 12:
		return []string{"tundra"}
	}
	if f("moisture", i) > .65 && f("growingSeason", i) > .3 {
		return []string{"forest", "grass", "meadow"}
	}
	return []string{"grass", "meadow"}
}

func (e *Environment) Suitability(c int, id string) float64 {
	switch id {
	case "forest":
		return .1 + e.get("moisture", c)*e.get("growingSeason", c)*3
	case "meadow":
		return .1 + e.get("moisture", c)*clamp(1-math.Abs(e.get("temperature", c)-15)/25, 0, 1)
	case "grass":
		return .2 + 1 - e.get("moisture", c)*.7
	case "dunes":
		return .1 + e.get("dune", c)*8
	case "reef":
		return .1 + e.get("reef", c)*12
	case "mesa":
		return .1 + e.get("mesa", c)*5
	}
	return 1
}
