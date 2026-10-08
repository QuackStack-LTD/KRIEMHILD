package terrain

import (
	"math"
	"sort"
)

var SurfaceFields = []string{"ocean", "waterLevel", "waterDepth", "waterBody", "basin", "catchmentArea", "drainageElevation", "shelf", "seamount", "reefType", "light", "landform", "highland", "mountainCore", "geology"}

type WaterBody struct {
	ID       int     `json:"id"`
	Kind     string  `json:"kind"`
	Level    float64 `json:"level"`
	MaxDepth float64 `json:"maxDepth"`
	Cells    []int   `json:"cells"`
}
type ReefRegion struct {
	Kind  string `json:"kind"`
	Cells []int  `json:"cells"`
}

func (e *Environment) carveDepressions(sample func(float64, float64) float64) {
	if e.Options.Heights != nil {
		return
	}
	w, h := e.Options.Columns, e.Options.Rows
	d := distance(e.Mask, w, h, 0)
	used := make([]bool, w*h)
	for i := range e.Mask {
		if used[i] || d[i] < 3 || e.get("elevation", i) > 1400 || sample(float64(i%w)*1.73+411, float64(i/w)*1.31) < .985 {
			continue
		}
		radius := 2. + sample(float64(i%w)+7, float64(i/w)+19)*2
		floor := -100 - 500*sample(float64(i%w)+531, float64(i/w)+617)
		for dy := -5; dy <= 5; dy++ {
			for dx := -5; dx <= 5; dx++ {
				x, y := i%w+dx, i/w+dy
				if x < 0 || x >= w || y < 0 || y >= h {
					continue
				}
				j := y*w + x
				r := math.Hypot(float64(dx), float64(dy)) / radius
				if r > 1 || d[j] < 2 || e.Mask[j] == 0 {
					continue
				}
				z := e.get("elevation", j)
				// The positive enclosing rim prevents an automatic ocean connection.
				weight := math.Pow(1-r*r, 2)
				e.set("elevation", j, z*(1-weight)+floor*weight)
				e.Heights[j] = int(round(e.get("elevation", j) / 4))
				e.set("elevation", j, float64(e.Heights[j]*4))
				used[j] = true
			}
		}
	}
}

// Water coverage depends on connectivity, never just elevation's sign.
func (e *Environment) connectOcean() {
	w, h := e.Options.Columns, e.Options.Rows
	q := []int{}
	for i := range e.Mask {
		e.Mask[i] = 1
		if (i%w == 0 || i%w == w-1 || i < w || i >= w*(h-1)) && e.get("elevation", i) <= 0 {
			e.Mask[i] = 0
			e.set("ocean", i, 1)
			q = append(q, i)
		}
	}
	for head := 0; head < len(q); head++ {
		for _, j := range nb(q[head], w, h) {
			if e.Mask[j] != 0 && e.get("elevation", j) <= 0 {
				e.Mask[j] = 0
				e.set("ocean", j, 1)
				q = append(q, j)
			}
		}
	}
	body := WaterBody{ID: 1, Kind: "ocean", Cells: q}
	for _, i := range q {
		depth := math.Max(0, -e.get("elevation", i))
		e.set("waterDepth", i, depth)
		e.set("bathymetry", i, depth)
		e.set("waterBody", i, 1)
		body.MaxDepth = math.Max(body.MaxDepth, depth)
	}
	e.WaterBodies = []WaterBody{body}
	e.Fields["oceanDistance"] = distance(e.Mask, w, h, 0)
}

// Basins share a spill elevation. Runoff relative to evaporation determines
// their local water level; dry endorheic basins remain land below sea level.
func (e *Environment) fillBasins(filled []float64) {
	w, h := e.Options.Columns, e.Options.Rows
	f, set := e.get, e.set
	basinID := 0
	type basin struct {
		id           int
		cells        []int
		spill, floor float64
	}
	basins := []basin{}
	for i := range e.Mask {
		if e.Mask[i] == 0 || f("basin", i) > 0 || filled[i]-f("elevation", i) < 4 {
			continue
		}
		basinID++
		q := []int{i}
		set("basin", i, float64(basinID))
		spill := filled[i]
		floor := f("elevation", i)
		for head := 0; head < len(q); head++ {
			for _, j := range nb(q[head], w, h) {
				if e.Mask[j] != 0 && f("basin", j) == 0 && filled[j]-f("elevation", j) > 4 && math.Abs(filled[j]-filled[i]) < 1 {
					set("basin", j, float64(basinID))
					q = append(q, j)
					spill = math.Min(spill, filled[j])
					floor = math.Min(floor, f("elevation", j))
				}
			}
		}
		basins = append(basins, basin{basinID, q, spill, floor})
	}
	// Resolve upstream basins first. Closed basins must not supply phantom
	// runoff to a downstream lake through the provisional filled drainage tree.
	sort.SliceStable(basins, func(i, j int) bool { return basins[i].spill > basins[j].spill })
	for _, b := range basins {
		basinID, q, spill, floor := b.id, b.cells, b.spill, b.floor
		runoff, evap := 0., 0.
		for _, j := range q {
			runoff += math.Max(.02, f("precipitation", j)/1000)
			for _, k := range nb(j, w, h) {
				if int(f("flow", k)) == j && f("basin", k) != float64(basinID) {
					runoff += f("accumulation", k)
				}
			}
			evap += math.Max(120, (f("temperature", j)+25)*28)
		}
		balance := runoff * 350 / math.Max(1, evap)
		fraction := clamp((balance-.18)/.82, 0, 1)
		level := floor + (spill-floor)*fraction
		if len(q) < 2 || spill-floor < 12 || fraction < .08 {
			fraction = 0
			level = floor
		}
		body := WaterBody{ID: len(e.WaterBodies) + 1, Kind: "lake", Level: f32(level)}
		for _, j := range q {
			if fraction > 0 && f("elevation", j) < level-2 {
				e.Mask[j] = 0
				set("lake", j, 1)
				set("waterLevel", j, level)
				set("waterDepth", j, level-f("elevation", j))
				set("bathymetry", j, level-f("elevation", j))
				set("waterBody", j, float64(body.ID))
				body.Cells = append(body.Cells, j)
				body.MaxDepth = math.Max(body.MaxDepth, f("waterDepth", j))
			}
		}
		if len(body.Cells) > 0 {
			e.WaterBodies = append(e.WaterBodies, body)
		}
		if fraction < 1 {
			for _, j := range q {
				next := int(f("flow", j))
				if next < 0 || f("basin", next) == float64(basinID) {
					continue
				}
				lost := f("accumulation", j)
				for at := next; at >= 0; at = int(f("flow", at)) {
					set("accumulation", at, math.Max(0, f("accumulation", at)-lost))
				}
			}
			for _, j := range q {
				set("flow", j, -1)
				set("drainageElevation", j, math.Max(f("elevation", j), f("waterLevel", j)))
				if e.Mask[j] == 0 {
					set("drainageElevation", j, level)
					continue
				}
				// No artificial uphill drainage through a dry or partially filled basin.
				set("drainageElevation", j, f("elevation", j))
				z := f("elevation", j)
				for _, k := range nb(j, w, h) {
					if f("basin", k) != float64(basinID) {
						continue
					}
					surface := f("elevation", k)
					if e.Mask[k] == 0 {
						surface = level
					}
					if surface < z {
						z = surface
						set("flow", j, float64(k))
					}
				}
			}
		}
	}
	for i := range e.Mask {
		if f("lake", i) == 0 {
			continue
		}
		old := f("temperature", i)
		temperature := old - f("waterDepth", i)*.0065
		set("summer", i, temperature+(f("summer", i)-old)*.45)
		set("winter", i, temperature+(f("winter", i)-old)*.45)
		set("temperature", i, temperature)
		set("moisture", i, 1)
		set("aridity", i, 0)
	}
	e.accumulateDrainage()
}

func (e *Environment) accumulateDrainage() {
	n := len(e.Mask)
	f, set := e.get, e.set
	degree := make([]int, n)
	q := []int{}
	for i := 0; i < n; i++ {
		set("accumulation", i, 0)
		set("catchmentArea", i, 1)
		if e.Mask[i] != 0 {
			set("accumulation", i, math.Max(.02, f("precipitation", i)/1000))
		}
		j := int(f("flow", i))
		if j >= 0 {
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
		j := int(f("flow", i))
		if j >= 0 {
			set("accumulation", j, f("accumulation", j)+f("accumulation", i))
			set("catchmentArea", j, f("catchmentArea", j)+f("catchmentArea", i))
			degree[j]--
			if degree[j] == 0 {
				q = append(q, j)
			}
		}
	}
}

// Four reef forms follow coastal/shelf or volcanic-ring geometry. Suitability
// gates every form before it becomes a coherent, connected terrain region.
func (e *Environment) buildReefs(sample func(float64, float64) float64) {
	w, h := e.Options.Columns, e.Options.Rows
	e.Reefs = nil
	f, set := e.get, e.set
	landDist := distance(e.Mask, w, h, 1)
	for i := range e.Mask {
		set("reef", i, 0)
		set("reefType", i, 0)
		if f("waterDepth", i) <= 0 {
			continue
		}
		depth := f("waterDepth", i)
		light := math.Exp(-depth / (18 + f("clarity", i)*42))
		set("light", i, light)
		if f("ocean", i) == 0 {
			continue
		}
		if depth < 2 || depth > 70 || f("temperature", i) < 20 || f("temperature", i) > 31 || f("salinity", i) < 30 || f("clarity", i) < .6 || light < .22 || f("slope", i) > .65 {
			continue
		}
		substrate := clamp(.95-f("sediment", i)*.7, 0, 1)
		if e.Options.Realism {
			substrate = f("substrate", i)
		}
		if substrate < .3 {
			continue
		}
		// Warm shallows are potential habitat, not automatic coral coverage.
		// Coherent provinces represent sustained recruitment and stable substrate;
		// a second scale leaves gaps rather than painting entire coastlines pink.
		x, y := float64(i%w)/float64(w), float64(i/w)/float64(h)
		province := sample(x*9+813, y*9+927)
		if province < .69 || sample(x*32+619, y*32+731) < .48 {
			continue
		}
		kind := 3.
		if f("seamount", i) > .55 && landDist[i] > 2 {
			kind = 4
		} else if landDist[i] <= 1 {
			kind = 1
		} else if landDist[i] >= 2 && landDist[i] <= 6 && f("shelf", i) > .5 && depth < 32 {
			kind = 2
		}
		// Patch reefs follow continuous exposed substrate, not independent draws.
		if kind == 3 && sample(float64(i%w)*.31+721, float64(i/w)*.31+411) < .58 {
			continue
		}
		set("reef", i, clamp(light*f("clarity", i)*substrate*1.6, 0, 1))
		if f("reef", i) > .2 {
			set("reefType", i, kind)
		}
	}
	seen := make([]bool, len(e.Mask))
	names := []string{"", "fringing", "barrier", "patch", "atoll"}
	for i := range seen {
		kind := int(f("reefType", i))
		if kind == 0 || seen[i] {
			continue
		}
		q := []int{i}
		seen[i] = true
		for head := 0; head < len(q); head++ {
			for _, j := range nb(q[head], w, h) {
				if !seen[j] && int(f("reefType", j)) == kind {
					seen[j] = true
					q = append(q, j)
				}
			}
		}
		e.Reefs = append(e.Reefs, ReefRegion{Kind: names[kind], Cells: q})
	}
}

func (e *Environment) classifyLandforms() {
	for i := range e.Mask {
		z, slope := e.get("elevation", i), e.get("slope", i)
		kind := 1.
		switch {
		case e.Mask[i] == 0:
			kind = 0
		case z < 0:
			kind = 8
		case e.get("basin", i) > 0:
			kind = 7
		case z > 4500:
			kind = 6
		case z > 1800:
			kind = 5
		case z > 600 && slope < .12:
			kind = 4
		case slope > .14:
			kind = 3
		case z > 200:
			kind = 2
		}
		e.set("landform", i, kind)
	}
}
