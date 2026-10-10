package terrain

import (
	"math"
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
		// Subsidence and calderas require geological support. Noise only
		// varies their shape; it cannot punch arbitrary decorative lake holes.
		rift := (e.get("boundary", i) == 2 || e.get("boundary", i) == 3) && e.get("tectonicStress", i) > .3
		caldera := e.get("volcano", i) > .65
		if !rift && !caldera {
			continue
		}
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
