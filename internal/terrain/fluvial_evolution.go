package terrain

import "math"

type ErosionOptions struct {
	Iterations int     `json:"iterations"`
	YearsMa    float64 `json:"duration_ma"`
	Strength   float64 `json:"strength"`
}
type FluvialLandform struct {
	ID         string  `json:"id"`
	Kind       string  `json:"kind"`
	Cell       int     `json:"cell"`
	FormerTo   int     `json:"formerTo"`
	River      string  `json:"river,omitempty"`
	AgeMa      float64 `json:"ageMa"`
	Incision   float64 `json:"incisionMetres"`
	Deposition float64 `json:"depositionMetres"`
	WidthKM    float64 `json:"widthKm"`
	Formation  string  `json:"formation"`
	Process    string  `json:"process"`
}
type Geomorphology struct {
	Version     int               `json:"version"`
	Parameters  ErosionOptions    `json:"parameters"`
	Iterations  int               `json:"iterations"`
	ErodedM3    float64           `json:"erodedM3"`
	DepositedM3 float64           `json:"depositedM3"`
	ExportedM3  float64           `json:"exportedM3"`
	Landforms   []FluvialLandform `json:"landforms"`
}

// Bounded stream-power / sediment-capacity approximation. All modifications
// affect the saved elevation surface. Re-routing runs after every pass; old
// courses are retained only when actual erosion left an abandoned valley.
func (e *Environment) evolveRiverLandscape() {
	if e.Geography == nil && e.Options.Erosion == nil {
		return
	}
	opt := ErosionOptions{Iterations: 2, YearsMa: 2, Strength: 1}
	if e.Options.Erosion != nil {
		opt = *e.Options.Erosion
	}
	g := &Geomorphology{Version: 1, Parameters: opt}
	e.Geomorphology = g
	n := len(e.Mask)
	for _, name := range []string{"fluvialIncision", "fluvialDeposition", "valleyWidthKM", "paleochannel", "sedimentTransport"} {
		e.Fields[name] = make([]float64, n)
	}
	if opt.Iterations == 0 || opt.Strength == 0 || opt.YearsMa == 0 {
		return
	}
	e.BuildGeologicalHistory() // Bedrock history exists before erosion, resources after it.
	rocks := make([]float64, n)
	ages := make([]float64, n)
	oldFlow := make([]int, n)
	for i, c := range e.Geology.Cells {
		rock := e.Geology.Formations[c.Basement].Rock
		if c.Cover >= 0 {
			rock = e.Geology.Formations[c.Cover].Rock
		}
		resistance := map[string]float64{"granite": .9, "basalt": .85, "gneiss": .9, "schist": .65, "slate": .6, "sandstone": .45, "limestone": .55, "shale": .25, "mudstone": .18, "alluvium": .08, "evaporite": .15}[rock]
		if resistance == 0 {
			resistance = .5
		}
		rocks[i] = resistance
		ages[i] = math.Min(opt.YearsMa, e.Geology.Provinces[c.Province].AgeMa)
		oldFlow[i] = int(e.get("flow", i))
	}
	f, set := e.get, e.set
	airMean := append([]float64(nil), e.Fields["temperature"]...)
	summerAmplitude := make([]float64, n)
	winterAmplitude := make([]float64, n)
	initialZ := append([]float64(nil), e.Fields["elevation"]...)
	for i := range e.Mask {
		summerAmplitude[i] = f("summer", i) - airMean[i]
		winterAmplitude[i] = f("winter", i) - airMean[i]
		if f("lake", i) > 0 {
			airMean[i] += f("waterDepth", i) * .0065
			summerAmplitude[i] /= .45
			winterAmplitude[i] /= .45
		}
	}
	for pass := 0; pass < opt.Iterations; pass++ {
		before := append([]float64(nil), e.Fields["elevation"]...)
		cut := make([]float64, n)
		load := make([]float64, n)
		order := e.accumulateWater()
		dt := opt.YearsMa / float64(opt.Iterations)
		for _, i := range order {
			j := int(f("flow", i))
			if j < 0 || f("waterBody", i) > 0 || f("catchmentArea", i) < 3 {
				continue
			}
			drop := math.Max(0, before[i]-math.Max(before[j], f("waterLevel", j)))
			km := math.Max(.01, e.stepKM(i, j))
			grade := drop / (km * 1000)
			discharge := f("accumulation", i)
			if e.Geography != nil {
				discharge = f("dischargeM3s", i)
			}
			permanence := .2 + .8*clamp(f("dryDischarge", i)/math.Max(1e-9, f("accumulation", i))/.35, 0, 1)
			uplift := 1 + f("tectonicStress", i)*.7
			power := math.Pow(math.Max(0, discharge), .38) * math.Sqrt(grade) * dt * opt.Strength * 85 * permanence * uplift / (.25 + rocks[i])
			// Coarse cells store area-averaged valley lowering; do not turn an entire
			// 200-km planetary cell into a narrow canyon or erode existing mountains away.
			width := clamp(.05+math.Sqrt(math.Max(0, discharge))*.06, .05, 35)
			fraction := clamp(width/km, .04, 1)
			incision := math.Min(120*dt, math.Min(drop*.3, power*fraction))
			if before[i] > 0 {
				incision = math.Min(incision, math.Max(0, before[i]-1))
			}
			cut[i] = math.Max(cut[i], incision)
			set("valleyWidthKM", i, math.Max(f("valleyWidthKM", i), width))
			// Lateral bank erosion and wider low-gradient floodplains use the same
			// discharge-driven valley, bounded by relief above the river channel.
			for _, bank := range e.neighbors(i) {
				if f("waterBody", bank) > 0 || before[bank] < before[i] {
					continue
				}
				lateral := incision * .35 * math.Exp(-math.Pow(e.stepKM(i, bank)/math.Max(width, .01), 2))
				cut[bank] = math.Max(cut[bank], math.Min(lateral, (before[bank]-before[i])*.15))
			}
		}
		if e.Options.Heights == nil {
			for i := range cut {
				for _, j := range e.neighbors(i) {
					limit := 650.
					a, b := f("geology", i), f("geology", j)
					if (a == 2 || a == 6) && (b == 2 || b == 6) {
						limit = 1100
					}
					cut[i] = math.Min(cut[i], math.Max(0, limit-(before[j]-before[i])))
				}
			}
		}
		for i, v := range cut {
			set("elevation", i, before[i]-v)
			set("fluvialIncision", i, f("fluvialIncision", i)+v)
			volume := v * e.cellArea(i) * 1e6
			load[i] = volume
			g.ErodedM3 += volume
		}
		for _, i := range order {
			j := int(f("flow", i))
			volume := load[i]
			if volume <= 0 {
				continue
			}
			deposit := 0.
			if j < 0 || f("waterBody", i) > 0 {
				deposit = volume * .8
			} else {
				slope := math.Max(0, f("elevation", i)-f("elevation", j)) / (math.Max(.01, e.stepKM(i, j)) * 1000)
				deposit = volume * clamp(1-slope/.002, 0, .65)
			}
			metres := deposit / (e.cellArea(i) * 1e6)
			// Deposits cannot block an existing upstream channel or raise the sea bed
			// above water. Fresh rerouting, basin storage and base-level changes follow.
			ceiling := before[i] + 2
			if e.Options.Heights == nil {
				for _, j := range e.neighbors(i) {
					limit := 650.
					a, b := f("geology", i), f("geology", j)
					if (a == 2 || a == 6) && (b == 2 || b == 6) {
						limit = 1100
					}
					ceiling = math.Min(ceiling, f("elevation", j)+limit)
				}
			}
			if f("waterBody", i) > 0 {
				ceiling = math.Min(ceiling, f("waterLevel", i)-1)
			}
			for _, up := range e.drainageNeighbors(i) {
				if int(f("flow", up)) == i {
					ceiling = math.Min(ceiling, f("elevation", up)-.01)
				}
			}
			metres = math.Min(metres, math.Max(0, ceiling-f("elevation", i)))
			set("elevation", i, f("elevation", i)+metres)
			set("fluvialDeposition", i, f("fluvialDeposition", i)+metres)
			deposited := metres * e.cellArea(i) * 1e6
			g.DepositedM3 += deposited
			remaining := volume - deposited
			set("sedimentTransport", i, remaining)
			if j >= 0 {
				load[j] += remaining
			} else {
				g.ExportedM3 += remaining
			}
		}
		// Undo the old lake temperature modifier before reconnecting water at the
		// new bed. Current climate then follows the actual changed elevation.
		for i := range e.Mask {
			temperature := airMean[i] - (f("elevation", i)-initialZ[i])*.0065
			set("temperature", i, temperature)
			set("summer", i, temperature+summerAmplitude[i])
			set("winter", i, temperature+winterAmplitude[i])
			e.Heights[i] = int(round(f("elevation", i) / 4))
		}

		e.connectOcean()
		e.precipitation()
		for i, factor := range e.Options.ClimateAdjustments {
			set("precipitation", i, f("precipitation", i)*factor)
			potential := math.Max(120, (f("temperature", i)+25)*24)
			set("aridity", i, clamp(potential/math.Max(1, f("precipitation", i))/5, 0, 1))
			set("moisture", i, clamp(f("precipitation", i)/potential, 0, 1))
		}
		e.prepareHydrology()
		e.hydrology()
		g.Iterations++
		maxChange := 0.
		for i := range before {
			maxChange = math.Max(maxChange, math.Abs(f("elevation", i)-before[i]))
		}
		if maxChange < .01 {
			break
		}
	}
	for i := range e.Mask {
		slope := 0.
		for _, j := range e.neighbors(i) {
			slope = math.Max(slope, math.Abs(f("elevation", i)-f("elevation", j))/2000)
		}
		set("slope", i, clamp(slope, 0, 1))
		inc, dep := f("fluvialIncision", i), f("fluvialDeposition", i)
		if inc < .2 && dep < .2 {
			continue
		}
		kind, process := "river-valley", "persistent runoff, bedrock incision and sediment transport"
		if f("catchmentArea", i) < 8 {
			kind = "tributary-gully"
		}
		if dep > inc && slope < .06 {
			kind = "alluvial-floodplain"
			process = "declining sediment transport capacity deposits upstream eroded material"
		}
		if inc > 30 && rocks[i] > .6 && f("tectonicStress", i) > .25 {
			kind = "entrenched-gorge"
			process = "long-lived incision through resistant, uplifted bedrock"
		}
		if inc > 8 && f("tectonicStress", i) > .2 && slope < .1 {
			kind = "river-terrace"
			process = "incision below an older valley surface during uplift"
		}
		if oldFlow[i] >= 0 && oldFlow[i] != int(f("flow", i)) && inc > 2 {
			kind = "abandoned-channel"
			set("paleochannel", i, 1)
			process = "erosion-driven rerouting leaves an inherited former drainage course"
		}
		if dep > 1 && slope < .04 {
			for _, j := range e.neighbors(i) {
				if f("slope", j) > .12 {
					kind = "alluvial-fan"
					break
				}
			}
		}
		g.Landforms = append(g.Landforms, FluvialLandform{ID: e.hydroID("fluvial", i), Kind: kind, Cell: i, FormerTo: oldFlow[i], AgeMa: ages[i], Incision: inc, Deposition: dep, WidthKM: f("valleyWidthKM", i), Process: process})
		set("sediment", i, clamp(f("sediment", i)+dep*.08, 0, 1))
	}
}
func (e *Environment) attachFluvialHistory() {
	if e.Geomorphology == nil || e.Geology == nil {
		return
	}
	rivers := map[int]string{}
	for _, r := range e.Hydrology.Reaches {
		rivers[r.From] = r.RiverID
	}
	for k := range e.Geomorphology.Landforms {
		v := &e.Geomorphology.Landforms[k]
		c := e.Geology.Cells[v.Cell]
		v.Formation = e.Geology.Formations[c.Basement].ID
		v.River = rivers[v.Cell]
		e.Geology.Cells[v.Cell].Erosion = math.Max(c.Erosion, clamp(v.Incision/100, 0, 1))
		e.Geology.Cells[v.Cell].Deposition = math.Max(c.Deposition, clamp(v.Deposition/10, 0, 1))
	}
}

// Seam columns represent adjacent meridians, not coastline boundaries. Blend
// only a narrow band of procedural coarse relief; supplied heightfields remain
// authoritative. Land and marine connectivity are then derived from that bed.
func (e *Environment) reconcilePlanetSeam() {
	if e.Geography == nil || !e.Geography.WrapX || e.Options.Heights != nil {
		return
	}
	w, h := e.Options.Columns, e.Options.Rows
	band := max(2, w/32)
	for _, name := range []string{"elevation", "highland", "mountainCore", "volcano", "tectonicStress"} {
		a := e.Fields[name]
		for y := 0; y < h; y++ {
			left, right := a[y*w+band], a[y*w+w-1-band]
			for k := 0; k < band; k++ {
				t := (float64(band-k) - .5) / float64(2*band)
				a[y*w+k] = f32(left*(1-t) + right*t)
				a[y*w+w-1-k] = f32(right*(1-t) + left*t)
			}
		}
	}
	for i := range e.Mask {
		e.Heights[i] = int(round(e.get("elevation", i) / 4))
	}
}
