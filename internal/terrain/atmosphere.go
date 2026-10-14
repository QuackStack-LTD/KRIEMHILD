package terrain

import "math"

// Rainfall potential is an atmospheric forcing (mm / Earth year), not a biome
// mask. Signed latitude and orbital phase move the convergence zone; geography
// must still supply the vapor that can precipitate.
func circulationPotential(latitude float64, p ClimateParameters, month int) float64 {
	decl, _ := climateOrbit(p, month)
	convergence := decl * 180 / math.Pi * .65
	tropic := math.Exp(-math.Pow((latitude-convergence)/13, 2))
	subtropic := math.Exp(-math.Pow((math.Abs(latitude)-29)/9, 2))
	storm := math.Exp(-math.Pow((math.Abs(latitude)-53)/13, 2))
	polar := 1 - .8*smooth((math.Abs(latitude)-65)/20)
	cells := (450 + 3200*tropic + 1700*storm) * (1 - .65*subtropic) * polar
	return math.Max(30, 700+(cells-700)*clamp(p.Circulation, 0, 1.5))
}

func (e *Environment) transportKM(i int) float64 {
	if e.Geography == nil {
		return 180
	}
	lat := e.climateField("latitude", i) * math.Pi / 180
	east := e.Geography.Radius * (e.Geography.East - e.Geography.West) * math.Pi / 180 / float64(e.Options.Columns) * math.Max(.05, math.Cos(lat))
	wx, wy := math.Abs(e.get("windX", i)), math.Abs(e.get("windY", i))
	return math.Max(1, (east*wx+e.Geography.WorldScale*wy)/math.Max(.01, wx+wy))
}

func (e *Environment) upwindBarrier(i int) (lift, shadow float64) {
	w, h := e.Options.Columns, e.Options.Rows
	wx, wy := e.get("windX", i), e.get("windY", i)
	length := math.Max(.01, math.Hypot(wx, wy))
	km := e.transportKM(i)
	local := math.Max(0, e.get("elevation", i))
	// A fixed geographic transect, not a fixed number of cells at every resolution.
	for step := 1; step <= max(1, int(math.Ceil(1000/km))); step++ {
		x := int(round(float64(i%w) - wx/length*float64(step)))
		y := int(round(float64(i/w) - wy/length*float64(step)))
		if e.Geography != nil && e.Geography.WrapX {
			x = (x%w + w) % w
		}
		if x < 0 || x >= w || y < 0 || y >= h {
			break
		}
		j := y*w + x
		z := math.Max(0, e.get("elevation", j))
		decay := math.Exp(-float64(step-1) * km / 800)
		lift = math.Max(lift, (local-z)/1400*decay)
		shadow = math.Max(shadow, (z-local)/1400*decay)
		if e.Mask[j] == 0 {
			break
		}
	}
	return clamp(lift, 0, 2), clamp(shadow, 0, 1)
}

// Monthly circulation uses the actual seasonal upwind path. This modulates
// rainfall timing within the annual transported-water budget, not its total.
func (e *Environment) seasonalVaporAccess(i int, wx, wy float64) float64 {
	w, h := e.Options.Columns, e.Options.Rows
	km := e.transportKM(i)
	length := math.Max(.01, math.Hypot(wx, wy))
	barrier := 0.
	local := math.Max(0, e.climateField("elevation", i))
	for step := 1; step <= max(1, int(math.Ceil(1600/km))); step++ {
		x := int(round(float64(i%w) - wx/length*float64(step)))
		y := int(round(float64(i/w) - wy/length*float64(step)))
		if e.Geography != nil && e.Geography.WrapX {
			x = (x%w + w) % w
		}
		if x < 0 || x >= w || y < 0 || y >= h {
			break
		}
		j := y*w + x
		barrier = math.Max(barrier, math.Max(0, e.climateField("elevation", j)-local)/1800)
		if e.Mask[j] == 0 {
			return math.Exp(-float64(step)*km/2400 - barrier*.6)
		}
	}
	return clamp(e.climateField("moistureSupply", i)*.4*math.Exp(-barrier*.6), 0, .4)
}

func (e *Environment) precipitation() {
	w, h := e.Options.Columns, e.Options.Rows
	n := w * h
	f, set := e.get, e.set
	p, _ := e.Options.climateConfiguration()
	for _, name := range []string{"rainfallPotential", "rainShadow", "moistureSupply"} {
		e.Fields[name] = make([]float64, n)
	}
	lifting := make([]float64, n)
	for i := range e.Mask {
		potential := 0.
		for month := 0; month < 12; month++ {
			potential += math.Max(30, circulationPotential(f("latitude", i), p, month)) / 12
		}
		set("rainfallPotential", i, potential)
		lift, shadow := e.upwindBarrier(i)
		lifting[i] = lift
		set("rainShadow", i, shadow)
	}
	vapor, next := make([]float64, n), make([]float64, n)
	for i := range vapor {
		if e.Mask[i] == 0 {
			vapor[i] = 1
			next[i] = 1
		}
	}
	for pass := 0; pass < max(12, min(64, h)); pass++ {
		for y := 0; y < h; y++ {
			// Wind is computed per latitude band. Use the sign at this band, not a
			// global direction; each zonal sweep consumes its already-updated upwind cell.
			direction := 1
			if f("windX", y*w+w/2) < 0 {
				direction = -1
			}
			for k := 0; k < w; k++ {
				x := k
				if direction < 0 {
					x = w - 1 - k
				}
				i := y*w + x
				if e.Mask[i] == 0 {
					next[i] = clamp((f("temperature", i)+40)/60, .35, 1)
					set("precipitation", i, f("rainfallPotential", i)*e.Options.Rainfall)
					continue
				}
				ux := x - direction
				if e.Geography != nil && e.Geography.WrapX {
					ux = (ux + w) % w
				}
				ux = max(0, min(w-1, ux))
				uy := y - 1
				if f("windY", i) < 0 {
					uy = y + 1
				}
				uy = max(0, min(h-1, uy))
				mix := math.Abs(f("windY", i)) / (math.Abs(f("windX", i)) + math.Abs(f("windY", i)) + .001)
				input := (1-mix)*next[y*w+ux] + mix*vapor[uy*w+x]
				// Moisture decays along the upwind path, faster when crossing a
				// barrier. Inland cells receive no arbitrary local vapor source.
				km := e.transportKM(i)
				retention := math.Exp(-km / 5000)
				// Vapor loses water on the actual ascent once. Applying the full
				// 1000 km barrier at every step would rain out the same climb repeatedly.
				upHeight := (1-mix)*math.Max(0, f("elevation", y*w+ux)) + mix*math.Max(0, f("elevation", uy*w+x))
				ascent := math.Max(0, f("elevation", i)-upHeight) / 1400
				next[i] = input * retention * math.Exp(-ascent*.35)
				set("moistureSupply", i, input)
				coastal := math.Exp(-f("oceanDistance", i) * km / 650)
				current := 1 + clamp(f("current", i)/10, -.25, .25)*coastal
				rain := f("rainfallPotential", i) * input * (1 + lifting[i]*1.5) * (1 - .6*f("rainShadow", i)) * current
				set("precipitation", i, math.Max(5, rain)*e.Options.Rainfall)
			}
		}
		copy(vapor, next)
	}
	// Until BuildClimate reconciles the monthly budget, these are transport
	// diagnostics only; no terrain is selected from provisional moisture.
	for i := range e.Mask {
		set("moisture", i, clamp(f("precipitation", i)/1000, 0, 1))
		set("aridity", i, 1-f("moisture", i))
	}
}
