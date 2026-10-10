package terrain

import "math"

func (e *Environment) waterRef(i int) string {
	if id := int(e.get("waterBody", i)); id > 0 {
		return e.hydroID("water", id)
	}
	if e.get("flow", i) < 0 {
		if e.edgeOutlet(i) {
			return e.hydroID("external-drainage", i)
		}
		return e.hydroID("terminal-basin", i)
	}
	return e.riverID(i)
}
func (e *Environment) finishHydrology() {
	f, set := e.get, e.set
	w, h := e.Options.Columns, e.Options.Rows
	e.buildMarineRegions()
	visible := e.VisibleChannels()
	incoming := make([][]int, len(e.Mask))
	fresh := make([]int, len(e.Mask))
	for i := range e.Mask {
		if j := int(f("flow", i)); j >= 0 {
			incoming[j] = append(incoming[j], i)
		}
		if visible[i] || f("lake", i) > 0 {
			fresh[i] = 1
		}
	}

	lakeFed := map[int]bool{}
	for i := range visible {
		if !visible[i] || f("waterBody", i) > 0 {
			continue
		}
		for at, steps := int(f("flow", i)), 0; at >= 0 && steps < len(e.Mask) && f("lake", at) > 0; steps++ {
			lakeFed[int(f("waterBody", at))] = true
			at = int(f("flow", at))
		}
	}
	e.Fields["freshwaterDistance"] = distance(fresh, w, h, 1)
	for i := range e.Mask {
		set("river", i, 0)
		if visible[i] {
			set("river", i, clamp(math.Log1p(f("accumulation", i))/8, 0, 1))
		}
		saturation := clamp(f("groundFlow", i)/(.45+f("catchmentArea", i)*.035)+f("moisture", i)*.35+math.Exp(-f("freshwaterDistance", i))*.25-f("slope", i)*.4, 0, 1)
		set("groundwater", i, saturation)
		set("oasis", i, 0)
		if f("aridity", i) > .55 && saturation > .5 {
			set("oasis", i, saturation*f("aridity", i))
		}
		set("waterTableDepth", i, (1-saturation)*(3+12*f("permeability", i)))
		if f("waterBody", i) > 0 {
			set("waterTableDepth", i, 0)
			set("groundwater", i, 1)
			continue
		}
		// Emergence requires supplied groundwater plus a geological outlet. A
		// volcanic location alone is insufficient to create a hot spring.
		contrast := 0.
		maximum := true
		for _, j := range e.neighbors(i) {
			contrast = math.Max(contrast, f("permeability", j)-f("permeability", i))
			if f("groundFlow", j) > f("groundFlow", i) && f("elevation", j) >= f("elevation", i) {
				maximum = false
			}
		}
		support := math.Max(contrast, f("tectonicStress", i)*.6)
		if maximum && f("groundFlow", i) > .075 && support > .15 && f("slope", i) < .5 && f("flow", i) >= 0 {
			origin := "aquifer-contact"
			if f("tectonicStress", i) > .4 {
				origin = "fracture"
			}
			if int(f("lithology", i)) == 3 {
				origin = "karst"
			}
			minerals := map[string]float64{"silica": 10 + 30*f("permeability", i)}
			switch int(f("lithology", i)) {
			case 3:
				minerals["calcium"] = 80
				minerals["magnesium"] = 25
			case 5:
				minerals["sodium"] = 180
				minerals["sulfate"] = 120
			case 4:
				minerals["silica"] = 90
				minerals["sulfate"] = 35
			}
			temp := math.Max(4, f("temperature", i)) + f("geothermal", i)*support*.28
			set("spring", i, math.Min(f("groundFlow", i), f("accumulation", i)))
			e.Hydrology.Springs = append(e.Hydrology.Springs, HydroSpring{ID: e.hydroID("spring", i), Cell: i, Origin: origin, Downstream: e.waterRef(int(f("flow", i))), Discharge: f("spring", i), Temperature: temp, Minerals: minerals})
		}
	}
	for i := range e.Mask {
		j := int(f("flow", i))
		if !visible[i] || j < 0 || f("waterBody", i) > 0 {
			continue
		}
		r := HydroReach{ID: e.riverID(i), From: i, To: j, Downstream: e.waterRef(j), Watershed: e.hydroID("watershed", int(f("watershed", i))), Order: int(f("streamOrder", i)), Discharge: f("accumulation", i), DryDischarge: f("dryDischarge", i), Morphology: "confined", Regime: "perennial"}
		for _, k := range incoming[i] {
			if visible[k] && f("waterBody", k) == 0 {
				r.Upstream = append(r.Upstream, e.riverID(k))
			}
		}
		if len(r.Upstream) == 0 {
			r.Source = "mountain-runoff"
			if e.Hydrology.NetworkVersion < 2 || e.Hydrology.NetworkVersion >= 3 && f("mountainCore", i) < .09 {
				r.Source = "headwater-catchment"
			}
			if f("snowmelt", i) > .05 {
				r.Source = "snowmelt"
			}
			if f("glacier", i) > .1 {
				r.Source = "glacier"
			}
			if (e.Hydrology.NetworkVersion < 2 || e.Hydrology.NetworkVersion >= 3) && f("spring", i) > 0 {
				r.Source = "spring"
			}
			for _, k := range incoming[i] {
				if f("lake", k) > 0 && f("accumulation", k) > f("hydroLoss", k) && (e.Hydrology.NetworkVersion != 2 || lakeFed[int(f("waterBody", k))]) {
					r.Source = "lake-outlet"
				}
			}
		} else {
			r.Source = "tributaries"
		}
		ratio := r.DryDischarge / math.Max(.00001, r.Discharge)
		if ratio < .08 {
			r.Regime = "ephemeral"
		} else if ratio < .3 {
			r.Regime = "seasonal"
		}
		if f("summer", i) < 1 {
			r.Regime = "frozen-seasonal"
		}
		grade := math.Max(0, f("drainageElevation", i)-f("drainageElevation", j))
		r.Width = (.018 + math.Min(.32, math.Sqrt(r.Discharge)*.012)) * (.45 + .55/(1+grade/180))
		if e.Geography != nil {
			r.DischargeM3s = f("dischargeM3s", i)
			r.WidthMetres = 4.8 * math.Sqrt(r.DischargeM3s) * (.45 + .55/(1+grade/180))
			r.Width = clamp(r.WidthMetres/(math.Max(.01, e.stepKM(i, j))*1000), .006, .32)
		}
		if grade < 40 {
			r.Morphology = "meandering"
		}
		if grade < 90 && grade > 2 && f("sediment", i) > .45 && r.Discharge > 8 {
			r.Morphology = "braided"
		}
		if f("ocean", j) > 0 && grade < 20 && f("sediment", i) > .35 && r.Discharge > 8 {
			r.Morphology = "delta"
		}
		if r.Morphology == "delta" {
			for _, mouth := range e.neighbors(i) {
				if mouth != j && f("ocean", mouth) > 0 && f("waterLevel", mouth) <= f("elevation", i) {
					r.Branches = append(r.Branches, HydroBranch{ID: r.ID + "/distributary", To: mouth, Downstream: e.waterRef(mouth), Fraction: .3})
					break
				}
			}
		}
		e.Hydrology.Reaches = append(e.Hydrology.Reaches, r)
	}
	// Springs may emerge above the visible network; store their complete
	// receiving route rather than a dangling reference to an invisible stream.
	for k := range e.Hydrology.Springs {
		sp := &e.Hydrology.Springs[k]
		at := int(f("flow", sp.Cell))
		for steps := 0; at >= 0 && steps < len(e.Mask); steps++ {
			if visible[at] || f("waterBody", at) > 0 || f("flow", at) < 0 {
				sp.Downstream = e.waterRef(at)
				break
			}
			at = int(f("flow", at))
		}
	}
	e.buildHydroWetlands(visible)
	e.buildHydroCoasts(visible)
	e.buildHydroCanals(visible)
	for b := range e.Hydrology.Basins {
		basin := &e.Hydrology.Basins[b]
		for _, r := range e.Hydrology.Reaches {
			if int(f("basin", r.To)) == b+1 && f("waterBody", r.To) > 0 && f("waterBody", r.From) == 0 {
				basin.Inlets = append(basin.Inlets, r.ID)
			}
		}
		if basin.Outlet >= 0 {
			at := int(f("flow", basin.Outlet))
			for steps := 0; at >= 0 && steps < len(e.Mask); steps++ {
				if visible[at] || f("waterBody", at) > 0 || f("flow", at) < 0 {
					basin.Downstream = e.waterRef(at)
					break
				}
				at = int(f("flow", at))
			}
		}
	}
	if e.Hydrology.NetworkVersion > 0 {
		e.summarizeRiverSystems()
	}
}

func (e *Environment) buildHydroWetlands(visible []bool) {
	f, set := e.get, e.set
	kinds := make([]string, len(e.Mask))
	sources := make([]string, len(e.Mask))
	for i := range e.Mask {
		set("wetland", i, 0)
		if f("waterBody", i) > 0 || f("slope", i) > .065 || f("summer", i) < 0 {
			continue
		}
		// A supplied aquifer is not automatically a swamp. Persistent surface
		// saturation additionally needs poor drainage or a real emergence point.
		retention := 1 - smooth((f("permeability", i)-.18)/.55)
		confinement := clamp((f("drainageElevation", i)-f("elevation", i))/12, 0, 1)
		retention = math.Max(retention, math.Max(confinement, clamp(f("spring", i)*3, 0, 1)))
		supply := f("groundwater", i) * (.35 + .65*retention)
		if f("waterTableDepth", i) > 1.2 {
			supply *= .55
		}
		// Concentrated runoff can perch above clay-rich soil even while the
		// deeper aquifer is unsaturated. Retain these selective seasonal pools.
		ponding := math.Min(1, f("accumulation", i)/3) * (1 - f("permeability", i)) * retention
		runoffPonding := ponding > supply
		supply = math.Max(supply, ponding)
		floodSource := ""
		// Overbank water and a shallow river-connected water table sustain
		// inland floodplains, even where the immediate cell has little runoff.
		for _, j := range e.drainageNeighbors(i) {
			// Lake-connected water tables saturate low banks without flooding
			// higher terrain or assuming every coastline is a swamp.
			if f("lake", j) > 0 {
				bank := math.Max(0, f("elevation", i)-f("waterLevel", j))
				lakeSupply := .95 * math.Exp(-bank/12)
				if lakeSupply > supply {
					supply, floodSource = lakeSupply, e.waterRef(j)
				}
			}
			if !visible[j] || f("accumulation", j) < 2 {
				continue
			}
			bank := math.Abs(f("elevation", i) - f("elevation", j))
			if bank > 16 || f("slope", i) > .08 {
				continue
			}
			flood := (.65 + .25*clamp(f("accumulation", j)/12, 0, 1)) * math.Exp(-bank/24)
			if flood > supply {
				supply = flood
				floodSource = e.riverID(j)
			}
		}
		strength := supply * (1 - smooth(f("slope", i)/.11))
		if strength < .70 {
			continue
		}
		kind, source := "marsh", e.hydroID("groundwater", int(f("watershed", i)))
		if runoffPonding {
			source = e.hydroID("watershed", int(f("watershed", i)))
		}
		if floodSource != "" {
			kind = "floodplain-marsh"
			source = floodSource
		}
		if f("moisture", i) > .65 && f("temperature", i) > 8 {
			kind = "swamp"
		}
		if f("temperature", i) < 12 && f("moisture", i) > .9 && f("permeability", i) < .35 && int(f("lithology", i)) == 0 {
			kind = "bog"
			source = "precipitation"
		}
		if f("groundFlow", i) > .3 && (int(f("lithology", i)) == 3 || f("spring", i) > 0) {
			kind = "fen"
		}
		if f("dryDischarge", i) < f("accumulation", i)*.3 {
			kind = "seasonal-wetland"
		}
		for _, j := range e.neighbors(i) {
			if visible[j] {
				source = e.riverID(j)
			}
			if f("lake", j) > 0 {
				source = e.waterRef(j)
			}
		}
		kinds[i], sources[i] = kind, source
		set("wetland", i, strength)
	}
	seen := make([]bool, len(e.Mask))
	for i, kind := range kinds {
		if kind == "" || seen[i] {
			continue
		}
		cells := []int{i}
		seen[i] = true
		for head := 0; head < len(cells); head++ {
			for _, j := range e.neighbors(cells[head]) {
				if !seen[j] && kinds[j] == kind && sources[j] == sources[i] {
					seen[j] = true
					cells = append(cells, j)
				}
			}
		}
		e.Hydrology.Wetlands = append(e.Hydrology.Wetlands, HydroWetland{ID: e.hydroID("wetland", i), Kind: kind, Cells: cells, Source: sources[i], Seasonal: kind == "seasonal-wetland"})
	}
}

func (e *Environment) buildHydroCoasts(visible []bool) {
	f, set := e.get, e.set
	for i := range e.Mask {
		if f("waterBody", i) > 0 {
			continue
		}
		body, water := 0, -1
		wetNeighbors := 0
		for _, j := range e.neighbors(i) {
			if f("waterBody", j) > 0 {
				body = int(f("waterBody", j))
				water = j
				wetNeighbors++
			}
		}
		if body == 0 {
			continue
		}
		above := math.Max(0, f("elevation", i)-f("waterLevel", water))
		sediment := clamp(f("sediment", i)*(.6+.4*(1-f("slope", i))), 0, 1)
		set("coastalSediment", i, sediment)
		kind, code := "rocky-shore", 2.
		if f("slope", i) < .12 && sediment > .15 {
			kind, code = "beach", 1
			set("beach", i, sediment*math.Exp(-above/4)*(1-smooth(f("slope", i)/.12)))
		}
		river := ""
		if visible[i] && f("ocean", water) > 0 {
			river = e.riverID(i)
			kind, code = "estuary", 3
			if f("accumulation", i) > 8 && sediment > .35 && f("slope", i) < .06 {
				kind, code = "delta", 4
			}
		}
		landSides := 0
		for _, k := range e.neighbors(water) {
			if f("waterBody", k) == 0 {
				landSides++
			}
		}
		if f("ocean", water) > 0 && f("glacier", i) > .1 && f("slope", i) > .12 && landSides >= 2 {
			kind, code = "fjord", 5
		}
		if f("ocean", water) > 0 && wetNeighbors == 1 && f("shelf", water) > .8 && f("waterDepth", water) < 12 && sediment > .4 && f("slope", i) < .03 {
			kind, code = "lagoon-margin", 6
		}
		set("coastType", i, code)
		e.Hydrology.Coasts = append(e.Hydrology.Coasts, HydroCoast{ID: e.hydroID("coast", i), Cell: i, Kind: kind, WaterBody: body, River: river})
	}
}

// Engineered irrigation is justified by a settlement's dry cultivated fields.
// It is a separate graph and never edits the natural flow raster.
func (e *Environment) buildHydroCanals(visible []bool) {
	if e.Entities == nil {
		return
	}
	w := e.Options.Columns
	f := e.get
	surface := func(i int) float64 {
		if f("waterBody", i) > 0 {
			return f("waterLevel", i)
		}
		return f("elevation", i)
	}
	allocated := map[int]float64{}
	for _, settlement := range e.Entities.Settlements {
		for _, farm := range settlement.Farms {
			if f("moisture", farm) > .6 {
				continue
			}
			source := -1
			var path []int
			// Reverse search from the receiving field finds a gravity-fed source.
			q := []int{farm}
			next := map[int]int{farm: -1}
			for head := 0; head < len(q) && head < 100; head++ {
				i := q[head]
				if i != farm && (visible[i] || f("lake", i) > 0) && f("accumulation", i)-allocated[i] > .5 {
					source = i
					break
				}
				for _, j := range e.neighbors(i) {
					if _, ok := next[j]; ok {
						continue
					}
					if math.Abs(float64(j%w-farm%w))+math.Abs(float64(j/w-farm/w)) > 6 {
						continue
					}
					if surface(j) >= surface(i) && f("slope", j) < .12 {
						next[j] = i
						q = append(q, j)
					}
				}
			}
			if source < 0 {
				continue
			}
			for at := source; at >= 0; at = next[at] {
				path = append(path, at)
			}
			supply := math.Min(.1, f("accumulation", source)*.05)
			allocated[source] += supply
			e.Hydrology.Canals = append(e.Hydrology.Canals, HydroCanal{ID: e.hydroID("canal", farm), Purpose: "irrigation", Source: e.waterRef(source), Destination: e.hydroID("farm", farm), Path: path, Supply: supply})
			break
		}
	}
}
