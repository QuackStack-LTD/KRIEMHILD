package terrain

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

var FieldNames = []string{"elevation", "bathymetry", "slope", "oceanDistance", "latitude", "temperature", "summer", "winter", "current", "windX", "windY", "windStrength", "precipitation", "moisture", "aridity", "plate", "boundary", "tectonicStress", "volcano", "flow", "accumulation", "groundwater", "sediment", "fertility", "snow", "glacier", "dune", "duneOrientation", "reef", "salinity", "clarity", "mesa", "wetland", "oasis", "farmland", "settlement", "lava", "ash", "lake"}

type EnvironmentOptions struct {
	Radius2            int       `json:"radius2"`
	Volcanism          float64   `json:"volcanism"`
	Realism            bool      `json:"realism"`
	Columns            int       `json:"columns"`
	Rows               int       `json:"rows"`
	Seed               string    `json:"seed"`
	LandPercent        float64   `json:"landPercent"`
	Ruggedness         float64   `json:"ruggedness"`
	PlateCount         int       `json:"plateCount"`
	ContinentCount     int       `json:"continentCount"`
	TemperatureOffset  float64   `json:"temperatureOffset"`
	Rainfall           float64   `json:"rainfall"`
	LatitudeNorth      float64   `json:"latitudeNorth"`
	LatitudeSouth      float64   `json:"latitudeSouth"`
	WindDirection      float64   `json:"windDirection"`
	Stability          float64   `json:"stability"`
	Selection          string    `json:"selection"`
	Cleanup            int       `json:"cleanup"`
	Heights            []float64 `json:"heights,omitempty"`
	Sea                float64   `json:"sea,omitempty"`
	ClimateAdjustments []float64 `json:"climateAdjustments,omitempty"`
}

func DecodeEnvironment(data []byte) (EnvironmentOptions, error) {
	o := EnvironmentOptions{Radius2: 1, Volcanism: 1, Columns: 96, Rows: 64, Seed: "Kriemhild", LandPercent: 42, Ruggedness: 100, PlateCount: 12, ContinentCount: 12, Rainfall: 1, LatitudeNorth: 90, LatitudeSouth: -90, Stability: 3, Selection: "entropy", Cleanup: 2}
	err := json.Unmarshal(data, &o)
	if err != nil {
		return o, err
	}
	if o.Radius2 < 1 || o.Radius2 > 25 {
		return o, fmt.Errorf("invalid environmental neighborhood radius")
	}
	if !finite(o.Volcanism) || o.Volcanism < .1 || o.Volcanism > 3 {
		return o, fmt.Errorf("volcanism must be between 0.1 and 3")
	}
	for _, v := range []struct {
		name      string
		v, lo, hi float64
	}{{"columns", float64(o.Columns), 16, 256}, {"rows", float64(o.Rows), 16, 256}, {"landPercent", o.LandPercent, 1, 85}, {"ruggedness", o.Ruggedness, 1, 200}, {"plateCount", float64(o.PlateCount), 3, 32}, {"continentCount", float64(o.ContinentCount), 2, 40}, {"temperatureOffset", o.TemperatureOffset, -25, 25}, {"rainfall", o.Rainfall, .1, 3}, {"latitudeNorth", o.LatitudeNorth, -90, 90}, {"latitudeSouth", o.LatitudeSouth, -90, 90}, {"windDirection", o.WindDirection, -1, 1}, {"stability", o.Stability, 0, 10}, {"cleanup", float64(o.Cleanup), 0, 10}} {
		if !finite(v.v) || v.v < v.lo || v.v > v.hi {
			return o, fmt.Errorf("invalid environmental setting: %s", v.name)
		}
	}
	if o.Columns*o.Rows > 24576 {
		return o, fmt.Errorf("choose at most 24,576 terrain cells")
	}
	if o.LatitudeNorth <= o.LatitudeSouth {
		return o, fmt.Errorf("north latitude must exceed south latitude")
	}
	if len(o.Seed) > 1200 {
		return o, fmt.Errorf("seed is too long")
	}
	if o.Heights != nil && len(o.Heights) != o.Columns*o.Rows {
		return o, fmt.Errorf("invalid height array")
	}
	if o.ClimateAdjustments != nil {
		if len(o.ClimateAdjustments) != o.Columns*o.Rows {
			return o, fmt.Errorf("invalid regional rainfall adjustments")
		}
		for _, v := range o.ClimateAdjustments {
			if !finite(v) || v < .01 || v > 20 {
				return o, fmt.Errorf("invalid regional rainfall multiplier")
			}
		}
	}
	return o, nil
}

type Plate struct {
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	VX          float64 `json:"vx"`
	VY          float64 `json:"vy"`
	Continental bool    `json:"continental"`
}
type Environment struct {
	Geology *GeologicalState `json:"geology,omitempty"`
	Resources *ResourceState `json:"resources,omitempty"`
	Hydrology    *HydrologyState      `json:"hydrology,omitempty"`
	WaterBodies  []WaterBody          `json:"waterBodies"`
	Reefs        []ReefRegion         `json:"reefs"`
	Entities     *WorldEntities       `json:"entities,omitempty"`
	ClimateZones []string             `json:"climateZones,omitempty"`
	Version      string               `json:"version"`
	Options      EnvironmentOptions   `json:"options"`
	Fields       map[string][]float64 `json:"fields"`
	Heights      []int                `json:"heights"`
	Mask         []int                `json:"mask"`
	Margin       int                  `json:"margin"`
	Plates       []Plate              `json:"plates"`
	Points       []Point              `json:"points"`
}

func f32(v float64) float64                              { return float64(float32(v)) }
func (e *Environment) get(name string, i int) float64    { return e.Fields[name][i] }
func (e *Environment) set(name string, i int, v float64) { e.Fields[name][i] = f32(v) }
func nb(i, w, h int) []int {
	a := make([]int, 0, 4)
	x, y := i%w, i/w
	if x > 0 {
		a = append(a, i-1)
	}
	if x < w-1 {
		a = append(a, i+1)
	}
	if y > 0 {
		a = append(a, i-w)
	}
	if y < h-1 {
		a = append(a, i+w)
	}
	return a
}
func distance(mask []int, w, h, value int) []float64 {
	d := make([]float64, w*h)
	q := []int{}
	for i := range d {
		d[i] = float64(w + h)
		if mask[i] == value {
			d[i] = 0
			q = append(q, i)
		}
	}
	for head := 0; head < len(q); head++ {
		i := q[head]
		for _, j := range nb(i, w, h) {
			if d[j] > d[i]+1 {
				d[j] = d[i] + 1
				q = append(q, j)
			}
		}
	}
	return d
}
func noise(seed uint32) func(float64, float64) float64 {
	hash := func(x, y int) float64 {
		a := uint32(x+17)*374761393 ^ uint32(y+31)*668265263 ^ seed
		a = (a ^ (a >> 13)) * 1274126177
		return float64(a^(a>>16)) / 4294967295
	}
	return func(x, y float64) float64 {
		ix, iy := int(math.Floor(x)), int(math.Floor(y))
		dx, dy := smooth(x-float64(ix)), smooth(y-float64(iy))
		return (hash(ix, iy)*(1-dx)+hash(ix+1, iy)*dx)*(1-dy) + (hash(ix, iy+1)*(1-dx)+hash(ix+1, iy+1)*dx)*dy
	}
}
func BuildEnvironment(o EnvironmentOptions) *Environment {
	w, h := o.Columns, o.Rows
	n := w * h
	seed := Seed(o.Seed)
	rng := RNG(seed ^ 0x738ac52)
	sample := noise(seed)
	e := &Environment{Version: "kriemhild-environment-v3", Options: o, Fields: map[string][]float64{}, Heights: make([]int, n), Mask: make([]int, n), Margin: max(2, int(math.Ceil(float64(min(w, h))*.045)))}
	for _, k := range FieldNames {
		e.Fields[k] = make([]float64, n)
	}
	for _, k := range SurfaceFields {
		e.Fields[k] = make([]float64, n)
	}
	if o.Realism {
		e.Version = "kriemhild-realism-v2"
		e.Entities = &WorldEntities{}
		e.ClimateZones = ClimateZones
		for _, k := range RealismFields {
			e.Fields[k] = make([]float64, n)
		}
	}
	for i := 0; i < n; i++ {
		e.Fields["flow"][i] = -1
	}
	f, set := e.get, e.set
	mask := e.Mask
	macro := &Solver{Rules: &Rules{Config: Config{Continents: []Kind{{ID: "water", Name: "Water", Color: "#3a7bd5", Odds: .5}, {ID: "land", Name: "Land", Color: "#7cb342", Odds: .5}}}}, W: w, H: h, N: n, K: 2}
	macro.buildContinents(&ContinentOptions{o.ContinentCount, 5}, seed)
	e.Points = macro.ContPoints
	scores := make([]float64, n)
	sorted := []float64{}
	for i := 0; i < n; i++ {
		x, y := float64(i%w), float64(i/w)
		edge := min(i%w, i/w, w-1-i%w, h-1-i/w)
		// Fade only near the ocean frame. The old global radial envelope
		// overpowered continental influence and made every seed a central island.
		envelope := smooth(float64(edge-e.Margin) / (float64(min(w, h)) * .08))
		// Warped continental boundaries form ocean basins between neighboring
		// regions. More points create more, smaller regions, rather than merely
		// changing the texture of the same landmass. High land coverage can still
		// join them into a supercontinent; no continent count is forced afterward.
		px := x + (sample(x/float64(w)*5+71, y/float64(h)*5)-.5)*float64(w)*.12
		py := y + (sample(x/float64(w)*5, y/float64(h)*5+91)-.5)*float64(h)*.12
		nearest, second := math.Inf(1), math.Inf(1)
		for _, p := range macro.ContPoints {
			d := math.Hypot(px-p.X, py-p.Y)
			if d < nearest {
				second, nearest = nearest, d
			} else if d < second {
				second = d
			}
		}
		basin := math.Exp(-math.Pow((second-nearest)/(float64(min(w, h))*.035), 2))
		scores[i] = f32(float64(macro.ContShare[i*2+1])*.65 + sample(x/float64(w)*7, y/float64(h)*7)*.25 + sample(x/float64(w)*20, y/float64(h)*20)*.1 - (1-envelope)*.6 - basin*.45)
		if edge > e.Margin {
			sorted = append(sorted, scores[i])
		}
	}
	sort.Float64s(sorted)
	landCount := min(len(sorted)-1, int(round(float64(n)*o.LandPercent/100)))
	cutoff := sorted[max(0, len(sorted)-landCount-1)]
	for i := 0; i < n; i++ {
		edge := min(i%w, i/w, w-1-i%w, h-1-i/w)
		if scores[i] > cutoff && edge > e.Margin {
			mask[i] = 1
		}
	}
	for p := 0; p < o.PlateCount; p++ {
		e.Plates = append(e.Plates, Plate{rng.Next() * float64(w), rng.Next() * float64(h), rng.Next()*2 - 1, rng.Next()*2 - 1, rng.Next() > .45})
	}
	hotspots := make([][2]float64, max(1, int(round(3*o.Volcanism))))
	for p := range hotspots {
		hotspots[p] = [2]float64{(.15 + rng.Next()*.7) * float64(w), (.15 + rng.Next()*.7) * float64(h)}
	}

	for i := 0; i < n; i++ {
		x, y := float64(i%w), float64(i/w)
		a, b, da, db := 0, 1, math.Inf(1), math.Inf(1)
		for p, v := range e.Plates {
			d := (x-v.X)*(x-v.X) + (y-v.Y)*(y-v.Y)
			if d < da {
				b, db, a, da = a, da, p, d
			} else if d < db {
				b, db = p, d
			}
		}
		pa, pb := e.Plates[a], e.Plates[b]
		length := math.Hypot(pb.X-pa.X, pb.Y-pa.Y)
		if length == 0 {
			length = 1
		}
		convergence := ((pa.VX-pb.VX)*(pb.X-pa.X) + (pa.VY-pb.VY)*(pb.Y-pa.Y)) / length
		delta := (math.Sqrt(db) - math.Sqrt(da)) / 2.8
		proximity := math.Exp(-delta * delta)
		set("plate", i, float64(a))
		boundary := 0.
		if proximity > .3 {
			boundary = 3
			if convergence > .25 {
				boundary = 1
			} else if convergence < -.25 {
				boundary = 2
			}
		}
		set("boundary", i, boundary)
		set("tectonicStress", i, clamp(convergence, 0, 1)*proximity)
		subduction := convergence > .25 && (!pa.Continental || !pb.Continental)
		divergent := convergence < -.25
		hot := 0.
		for _, p := range hotspots {
			hot = math.Max(hot, math.Exp(-((x-p[0])*(x-p[0])+(y-p[1])*(y-p[1]))/8))
		}
		v := 0.
		if subduction {
			v = .8
		} else if divergent {
			v = .5
		}
		set("volcano", i, clamp(math.Max(hot, proximity*v), 0, 1))
		if o.Realism {
			set("hotspot", i, hot)
			// Continental arcs lie on the overriding continental side. For
			// ocean/ocean convergence use one deterministic overriding plate.
			if subduction && (!pa.Continental && (pb.Continental || a > b)) {
				v = 0
			}
			set("volcano", i, clamp(math.Max(hot, proximity*v), 0, 1))
		}
		// Regional structures precede ridges and summit detail. Their width varies
		// continuously along the collision belt and scales with map resolution.
		gap := math.Sqrt(db) - math.Sqrt(da)
		scale := math.Max(.6, float64(min(w, h))/100)
		width := scale * (7 + 8*sample(x/float64(w)*6+81, y/float64(h)*6+97))
		strength := math.Sqrt(clamp(convergence, 0, 1))
		set("highland", i, strength*math.Exp(-math.Pow(gap/width, 2)))
		set("mountainCore", i, strength*math.Exp(-math.Pow(gap/(width*.48), 2)))
	}
	e.buildGeologicalSurface(sample, hotspots)

	e.carveDepressions(sample)
	e.limitSurfaceGradients()
	e.connectOcean()
	for i := 0; i < n; i++ {
		slope := 0.
		for _, j := range nb(i, w, h) {
			slope = math.Max(slope, math.Abs(f("elevation", i)-f("elevation", j))/2000)
		}
		set("slope", i, clamp(slope, 0, 1))
		x, y := float64(i%w), float64(i/w)
		lat := o.LatitudeNorth + (o.LatitudeSouth-o.LatitudeNorth)*y/float64(h-1)
		ab, r := math.Abs(lat), lat*math.Pi/180
		set("latitude", i, lat)
		set("current", i, math.Sin(x/float64(w)*math.Pi*4)*math.Sin(r*2)*3*math.Exp(-f("oceanDistance", i)/3))
		base := 31 - 57*math.Pow(math.Sin(math.Abs(r)), 1.4) + o.TemperatureOffset + f("current", i)
		airHeight := 0.
		if mask[i] != 0 {
			airHeight = f("elevation", i)
		}
		set("temperature", i, base-airHeight*.0065)
		factor := .35
		if mask[i] != 0 {
			factor = 1
		}
		amplitude := (3 + ab*.1 + math.Min(18, f("oceanDistance", i)*1.3)) * factor
		set("summer", i, f("temperature", i)+amplitude)
		set("winter", i, f("temperature", i)-amplitude)
		east := -1.
		if ab >= 30 && ab < 60 {
			east = 1
		}
		if o.WindDirection != 0 {
			east = o.WindDirection
		}
		set("windX", i, east)
		hemisphere := 1.
		if lat < 0 {
			hemisphere = -1
		}
		meridian := .28
		if ab >= 30 && ab < 60 {
			meridian = -.28
		}
		set("windY", i, hemisphere*meridian+(sample(x/float64(w)*4, y/float64(h)*4)-.5)*.3)
		set("windStrength", i, .45+.4*math.Abs(math.Sin(r*3))+.15*sample(x/float64(w)*5, y/float64(h)*5))
	}
	if o.Realism {
		e.realisticWind()
	}
	e.precipitation()
	if o.ClimateAdjustments != nil {
		for i, factor := range o.ClimateAdjustments {
			set("precipitation", i, f("precipitation", i)*factor)
			potential := math.Max(120, (f("temperature", i)+25)*24)
			set("aridity", i, clamp(potential/math.Max(1, f("precipitation", i))/5, 0, 1))
			set("moisture", i, clamp(f("precipitation", i)/potential, 0, 1))
		}
	}
	e.prepareHydrology()
	if e.carveStorageBasins() {
		e.prepareHydrology()
	}
	e.hydrology()
	e.entities(sample)
	if o.Realism {
		e.realisticEntities(sample)
	}
	e.finishHydrology()
	e.buildReefs(sample)
	e.classifyLandforms()
	e.Hydrology.Diagnostics = e.ValidateHydrology()
	// Terrain and hydrology -> geological history -> natural resources.
	e.BuildGeologicalHistory()
	e.BuildNaturalResources()
	return e
}
func (e *Environment) precipitation() {
	w, h := e.Options.Columns, e.Options.Rows
	f, set := e.get, e.set
	vapor, next := make([]float64, w*h), make([]float64, w*h)
	for pass := 0; pass < 12; pass++ {
		for y := 0; y < h; y++ {
			east := f("windX", y*w+w/2) > 0
			for k := 0; k < w; k++ {
				x := k
				direction := 1
				if !east {
					x = w - 1 - k
					direction = -1
				}
				i := y*w + x
				ux := max(0, min(w-1, x-direction))
				up := y*w + ux
				uy := max(0, min(h-1, int(round(float64(y)-f("windY", i)))))
				cross := uy*w + ux
				input := .85*next[up] + .15*vapor[cross]
				if e.Options.Realism {
					// Fractional meridional advection: rounding windY erased trade winds.
					crossY := y - 1
					if f("windY", i) < 0 {
						crossY = y + 1
					}
					crossY = max(0, min(h-1, crossY))
					mix := math.Abs(f("windY", i)) / (math.Abs(f("windX", i)) + math.Abs(f("windY", i)) + .001)
					input = (1-mix)*next[up] + mix*vapor[crossY*w+x]
				}
				if e.Mask[i] == 0 {
					next[i] = f32(clamp((f("temperature", i)+30)/60, .2, 1) * 1.5)
					set("precipitation", i, 900)
					continue
				}
				lat := math.Abs(f("latitude", i))
				wetBelt := .065 + .11*math.Exp(-math.Pow(lat/15, 2)) + .06*math.Exp(-math.Pow((lat-55)/14, 2))
				dryBelt := 1 - .65*math.Exp(-math.Pow((lat-28)/9, 2))
				uplift := math.Max(0, f("elevation", i)-f("elevation", up)) / 1500
				loss := clamp(wetBelt*dryBelt+uplift*.6, .018, .85)
				if e.Options.Realism {
					loss = clamp(wetBelt*dryBelt*.3+uplift*.6, .006, .85)
				}
				rain := input * loss
				set("precipitation", i, (rain*10000+12)*e.Options.Rainfall)
				next[i] = f32(math.Max(0, input-rain) * .984)
				if e.Options.Realism {
					set("precipitation", i, (rain*22000+12)*e.Options.Rainfall)
					next[i] = f32(math.Max(0, input-rain) * .995)
				}
			}
		}
		copy(vapor, next)
	}
	for i := 0; i < w*h; i++ {
		potential := math.Max(120, (f("temperature", i)+25)*24)
		set("aridity", i, clamp(potential/math.Max(1, f("precipitation", i))/5, 0, 1))
		set("moisture", i, clamp(f("precipitation", i)/potential, 0, 1))
	}
}
func (e *Environment) hydrology() { e.simulateHydrology() }
func (e *Environment) entities(sample func(float64, float64) float64) {
	w, h := e.Options.Columns, e.Options.Rows
	n := w * h
	f, set := e.get, e.set
	mask := e.Mask
	for y := 0; y < h; y++ {
		east := f("windX", y*w+(w>>1)) > 0
		carried := 0.
		for k := 0; k < w; k++ {
			x := k
			if !east {
				x = w - 1 - k
			}
			i := y*w + x
			geology := sample(float64(x)/float64(w)*6+40, float64(y)/float64(h)*6+50)
			supply := 0.
			if mask[i] != 0 {
				supply = clamp(geology*.35+math.Log1p(f("accumulation", i))*.08, 0, 1)
			}
			carried = (carried*.88 + supply*.3) * f("windStrength", i)
			deposition := clamp(.25+f("slope", i)+f("moisture", i)*.2, 0, 1)
			set("sediment", i, clamp(supply*.5+carried*deposition, 0, 1))
			if mask[i] != 0 && f("aridity", i) > .5 && f("slope", i) < .25 {
				set("dune", i, clamp(f("aridity", i)*f("sediment", i)*f("windStrength", i)*(1-f("moisture", i))*2, 0, 1))
			}
			set("duneOrientation", i, math.Atan2(f("windY", i), f("windX", i))+math.Pi/2)
			if mask[i] != 0 && f("elevation", i) > 400 && geology > .58 && f("slope", i) > .08 && f("slope", i) < .5 {
				set("mesa", i, clamp(geology*f("aridity", i)*(.4+f("slope", i)), 0, 1))
			}
		}
	}
	for i := 0; i < n; i++ {
		cold := clamp((2-f("winter", i))/15, 0, 1)
		accum := f("precipitation", i) * cold
		melt := math.Max(0, f("summer", i)) * 120
		if mask[i] != 0 {
			set("snow", i, clamp((accum-melt)/700, 0, 1))
			if f("elevation", i) > 1800 || math.Abs(f("latitude", i)) > 60 {
				set("glacier", i, f("snow", i))
			}
			set("fertility", i, clamp(f("sediment", i)*.4+f("moisture", i)*.4+f("volcano", i)*.2, 0, 1))
			if f("lake", i) == 0 && f("slope", i) < .15 {
				set("wetland", i, clamp(f("groundwater", i)*(1-f("slope", i)*5)*math.Min(1, f("accumulation", i)/3), 0, 1))
			}
			if f("aridity", i) > .55 && f("groundwater", i) > .5 {
				set("oasis", i, clamp(f("groundwater", i)*f("aridity", i), 0, 1))
			}
			if f("lake", i) == 0 && f("summer", i) > 10 && f("temperature", i) > 0 && f("temperature", i) < 30 && f("slope", i) < .18 {
				set("farmland", i, clamp(f("fertility", i)*f("groundwater", i)*(1-f("slope", i)*4), 0, 1))
			}
		}
		set("settlement", i, f("farmland", i)*(.5+.5*math.Min(1, f("accumulation", i)/4)))
		runoff := f("accumulation", i)
		for _, j := range nb(i, w, h) {
			runoff = math.Max(runoff, f("accumulation", j))
		}
		set("clarity", i, clamp(1-math.Log1p(runoff)*.16, 0, 1))
		if mask[i] == 0 && f("oceanDistance", i) == 0 {
			set("salinity", i, 35-math.Min(22, runoff*.3))
		}
		if mask[i] == 0 && f("bathymetry", i) < 70 && f("temperature", i) > 20 && f("temperature", i) < 31 && f("salinity", i) > 30 && f("clarity", i) > .6 {
			set("reef", i, clamp((1-f("bathymetry", i)/80)*f("clarity", i), 0, 1))
		}
	}
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return f("elevation", order[b]) < f("elevation", order[a]) })
	for _, i := range order {
		j := int(f("flow", i))
		if j >= 0 && mask[j] != 0 && f("elevation", j) < f("elevation", i) && f("summer", j) < 8 {
			set("glacier", j, math.Max(f("glacier", j), f("glacier", i)*.72))
		}
	}
	for i := 0; i < n; i++ {
		peak := true
		for _, j := range nb(i, w, h) {
			if f("volcano", j) > f("volcano", i) {
				peak = false
			}
		}
		if mask[i] != 0 && f("volcano", i) > .62 && f("elevation", i) > 300 && sample(float64(i%w)*1.71+83, float64(i/w)*1.37) > .65 && peak {
			at := i
			for step := 0; step < 7; step++ {
				set("lava", at, 1-float64(step)/8)
				j := int(f("flow", at))
				if j < 0 || mask[j] == 0 || f("elevation", j) >= f("elevation", at) {
					break
				}
				at = j
			}
			x, y := i%w, i/w
			for k := 1; k <= 10; k++ {
				nx, ny := int(round(float64(x)+f("windX", i)*float64(k))), int(round(float64(y)+f("windY", i)*float64(k)))
				if nx < 0 || nx >= w || ny < 0 || ny >= h {
					break
				}
				j := ny*w + nx
				if mask[j] != 0 {
					set("ash", j, math.Max(f("ash", j), 1-float64(k)/12))
				}
			}
		}
	}
}
func (e *Environment) Candidates(i int) []string {
	if e.Options.Realism {
		return e.realisticCandidates(i)
	}
	f := e.get
	h, temp, wet := f("elevation", i), f("temperature", i), f("moisture", i)
	if f("lake", i) != 0 {
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
	for _, v := range []struct {
		name      string
		threshold float64
		types     []string
	}{{"lava", .2, []string{"lava"}}, {"ash", .2, []string{"ash"}}, {"glacier", .35, []string{"glacier"}}, {"snow", .3, []string{"snow"}}} {
		if f(v.name, i) > v.threshold {
			return v.types
		}
	}
	if e.Fields["beach"] != nil && f("beach", i) > .8 {
		return []string{"sand", "grass"}
	}
	if f("wetland", i) > .55 && temp > -8 {
		return []string{"swamp"}
	}
	if f("oasis", i) > .5 {
		return []string{"oasis"}
	}
	if f("dune", i) > .13 {
		return []string{"desert", "dunes"}
	}
	if f("mesa", i) > .3 {
		return []string{"mesa", "hills"}
	}
	if h > 2300 {
		return []string{"mountain"}
	}
	if f("summer", i) < 10 {
		return []string{"tundra"}
	}
	if wet < .28 {
		return []string{"desert"}
	}
	if temp > 20 && f("precipitation", i) > 1600 {
		return []string{"jungle"}
	}
	if temp < 5 && f("precipitation", i) > 300 {
		return []string{"taiga"}
	}
	if h > 900 && f("slope", i) > .15 {
		return []string{"hills", "mountain"}
	}
	if wet > .65 {
		return []string{"forest", "grass", "meadow"}
	}
	return []string{"grass", "meadow"}
}
func PrepareEnvironment(o EnvironmentOptions, c Config) (*Solver, error) {
	e := BuildEnvironment(o)
	if d := e.Hydrology.Diagnostics; len(d) > 0 {
		return nil, fmt.Errorf("hydrology validation: %s: %s", d[0].Object, d[0].Reason)
	}
	adapted := c
	adapted.Types = append([]Type{}, c.Types...)
	adapted.Climates = []Zone{}
	ids := []json.RawMessage{}
	for _, t := range c.Types {
		v, _ := json.Marshal(t.ID)
		ids = append(ids, v)
	}
	for i := range adapted.Types {
		adapted.Types[i].Neighbors = ids
		adapted.Types[i].Continent = ""
	}
	r, err := Compile(adapted)
	if err != nil {
		return nil, err
	}
	masks := make([]uint32, len(e.Heights))
	for i := range masks {
		choices := e.Candidates(i)
		for t, def := range c.Types {
			id := def.ID
			if def.EnvironmentType != "" {
				id = def.EnvironmentType
			}
			for _, choice := range choices {
				if choice == id {
					masks[i] |= bit(t)
					break
				}
			}
		}
		if masks[i] == 0 {
			return nil, fmt.Errorf("terrain configuration is missing environmental type: %v", choices)
		}
	}
	s := NewSolver(r, Options{Width: o.Columns, Height: o.Rows, Seed: Seed(o.Seed), Radius2: o.Radius2, Stability: o.Stability, Selection: o.Selection}, masks)
	s.Environment = e
	s.ContPoints = e.Points
	return s, nil
}
