package terrain

import (
	"fmt"
	"math"
)

func (e *Environment) climateField(name string, i int) float64 {
	if f := e.Fields[name]; len(f) > i {
		return f[i]
	}
	return 0
}

// Monthly states are climatic means for twelve equal time intervals, not weather
// forecasts. Kepler's equation changes orbital speed and radiative forcing.
func climateOrbit(p ClimateParameters, month int) (declination, forcing float64) {
	t := (float64(month) + .5) / 12
	mean := 2 * math.Pi * (t - p.Perihelion)
	ecc := mean
	for k := 0; k < 7; k++ {
		ecc -= (ecc - p.Eccentricity*math.Sin(ecc) - mean) / (1 - p.Eccentricity*math.Cos(ecc))
	}
	trueAnomaly := 2 * math.Atan2(math.Sqrt(1+p.Eccentricity)*math.Sin(ecc/2), math.Sqrt(1-p.Eccentricity)*math.Cos(ecc/2))
	longitude := trueAnomaly + 2*math.Pi*(p.Perihelion-.25)
	declination = math.Asin(math.Sin(p.AxialTilt*math.Pi/180) * math.Sin(longitude))
	r := 1 - p.Eccentricity*math.Cos(ecc)
	forcing = 1/(r*r) - 1/math.Sqrt(1-p.Eccentricity*p.Eccentricity)
	return
}

func (e *Environment) climateMonths(i int, p ClimateParameters, scale ClimateScale, waterDistance []float64) ([12]ClimateMonth, ClimateModifiers) {
	f := e.climateField
	lat := f("latitude", i)
	r := lat * math.Pi / 180
	cellKM := math.Sqrt(scale.NorthSouthKM * scale.EquatorialEastWestKM * math.Max(.02, math.Cos(r)))
	maritime := math.Exp(-f("oceanDistance", i) * cellKM / 650)
	water := math.Exp(-waterDistance[i] * cellKM / math.Max(80, cellKM*.8))
	if f("waterBody", i) > 0 {
		water = 1
	}
	land := f("waterBody", i) == 0
	height := math.Max(0, f("elevation", i))
	w := e.Options.Columns
	up := i
	if f("windX", i) > 0 && i%w > 0 {
		up = i - 1
	} else if f("windX", i) < 0 && i%w < w-1 {
		up = i + 1
	}
	shadow := clamp((f("elevation", up)-f("elevation", i))/1400, 0, 1)
	monsoon := math.Exp(-math.Pow((math.Abs(lat)-17)/14, 2)) * p.Circulation * (.25 + .75*(1-maritime)) * clamp(f("precipitation", i)/650, 0, 1)
	if !land {
		monsoon *= .25
	}
	forest := clamp(f("vegetation", i), 0, 1)
	m := ClimateModifiers{Elevation: f("elevation", i), Latitude: lat, Maritime: maritime, WaterProximity: water, RainShadow: shadow, Current: f("current", i), Geothermal: f("geothermal", i), Wetland: f("wetland", i), Glacier: f("glacier", i), Forest: forest, River: f("river", i), Monsoon: clamp(monsoon, 0, 1)}
	// Ocean/lake thermal inertia moderates the seasonal amplitude. The annual
	// mean already incorporates the terrain lapse rate and ocean currents.
	continental := 1 - maritime
	amplitude := (3 + math.Abs(lat)*.10 + 18*continental*math.Pow(math.Abs(math.Sin(r)), .7)) * math.Sin(p.AxialTilt*math.Pi/180) / math.Sin(23.44*math.Pi/180)
	amplitude *= clamp(math.Sqrt(p.OrbitalDays/365), .4, 2) * (1 - .15*water) * (1 - .08*forest)
	if !land {
		amplitude *= .35
	}
	var out [12]ClimateMonth
	var tempDelta, rainWeights [12]float64
	meanDelta, weight := 0., 0.
	for month := range out {
		decl, forcing := climateOrbit(p, month)
		season := 0.
		if p.AxialTilt > 1e-8 {
			season = math.Sin(decl) / math.Sin(p.AxialTilt*math.Pi/180)
		}
		local := season
		if lat < 0 {
			local = -local
		}
		tempDelta[month] = amplitude*local + forcing*(10+8*continental)
		meanDelta += tempDelta[month] / 12
		// Migrating convergence zones allow equatorial double rainfall peaks;
		// monsoons and subtropical coastal winter rain emerge from circulation.
		itcz := p.AxialTilt * .65 * season
		belt := math.Exp(-math.Pow((lat-itcz)/13, 2))
		med := math.Exp(-math.Pow((math.Abs(lat)-36)/8, 2)) * maritime * p.Circulation
		rainWeights[month] = math.Max(.06, 1+monsoon*1.05*local-med*.85*local+(belt-.5)*.5*p.Circulation)
		weight += rainWeights[month]
		dayAngle := math.Acos(clamp(-math.Tan(r)*math.Tan(decl), -1, 1))
		out[month].Daylight = rounded(dayAngle * 24 / math.Pi)
	}
	annual := f("precipitation", i) * p.OrbitalDays / 365
	for month := range out {
		temp := f("temperature", i) + tempDelta[month] - meanDelta
		// Geothermal heat affects the immediate ground microclimate; even a hot
		// spring does not turn an entire province into tropical climate.
		temp += clamp(m.Geothermal, 0, 1) * .15
		rain := annual * rainWeights[month] / weight
		days := p.OrbitalDays / 12
		pet := math.Max(.15, (temp+5)*.11) * days * (.45 + out[month].Daylight/24) * (1 - .12*forest)
		seasonalWind := monsoon * math.Sin(2*math.Pi*(float64(month)+.5)/12)
		x, y := f("windX", i)*(1-1.7*math.Max(0, seasonalWind)), f("windY", i)+seasonalWind*.8
		length := math.Max(.01, math.Hypot(x, y))
		out[month].Temperature = rounded(temp)
		out[month].Rain = rounded(rain)
		out[month].Evaporation = rounded(pet)
		out[month].WindX = rounded(x / length)
		out[month].WindY = rounded(y / length)
		out[month].WindSpeed = rounded((2 + f("windStrength", i)*9) * (1 + math.Abs(seasonalWind)*.35) * (1 + height/16000))
		out[month].Humidity = rounded(clamp(.22+.48*rain/(rain+pet+1)+.22*water+.12*forest+.12*m.Wetland, .08, .99))
	}
	// Two spin-up years prevent an arbitrary January water/snow state. Soil
	// capacity responds to permeability and vegetation, not rock-name labels.
	capacity := 100 + forest*80 + clamp(f("permeability", i), 0, 1)*100
	storage, snow := capacity*.5, 0.
	for year := 0; year < 3; year++ {
		for month := range out {
			v := &out[month]
			frozen := 1 - smooth((v.Temperature+2)/5)
			snow += v.Rain * frozen
			melt := math.Min(snow, math.Max(0, v.Temperature)*p.OrbitalDays/12*.8)
			snow -= melt
			supply := v.Rain*(1-frozen) + melt + water*m.Wetland*15
			storage = clamp(storage+supply-v.Evaporation, 0, capacity)
			v.SoilMoisture = rounded(storage / capacity)
			v.Snow = rounded(clamp(snow/100+m.Glacier*.7, 0, 1))
			v.Drought = rounded((1 - v.SoilMoisture) * clamp((v.Evaporation-v.Rain)/math.Max(1, v.Evaporation), 0, 1))
			v.FloodPotential = rounded(clamp((supply-v.Evaporation)/math.Max(25, annual/12), 0, 1) * clamp(m.Wetland+water*.35+m.River*.5, 0, 1))
		}
	}
	return out, m
}

func climateClass(v [12]ClimateMonth, m ClimateModifiers, p ClimateParameters) (string, string) {
	mean, rain, lo, hi, wet, dry, pet := 0., 0., math.Inf(1), math.Inf(-1), 0., math.Inf(1), 0.
	for _, x := range v {
		mean += x.Temperature / 12
		rain += x.Rain
		lo = math.Min(lo, x.Temperature)
		hi = math.Max(hi, x.Temperature)
		wet = math.Max(wet, x.Rain)
		dry = math.Min(dry, x.Rain)
		pet += x.Evaporation
	}
	for _, rule := range p.Rules {
		if mean >= rule.MinTemperature && mean <= rule.MaxTemperature && rain >= rule.MinRain && rain <= rule.MaxRain {
			return "fictional-" + rule.ID, rule.Name
		}
	}
	if hi < 0 {
		return "polar", "Polar ice"
	}
	if hi < 10 {
		if m.Elevation > 1800 {
			return "alpine", "Alpine tundra"
		}
		return "tundra", "Tundra"
	}
	if rain < pet*.25 {
		return "desert", "Arid desert"
	}
	if rain < pet*.5 {
		return "steppe", "Semi-arid steppe"
	}
	if lo >= 18 {
		if dry >= 60*p.OrbitalDays/365 {
			return "rainforest", "Tropical rainforest"
		}
		if m.Monsoon > .45 && wet > dry*2 {
			return "monsoon", "Tropical monsoon"
		}
		return "tropical-seasonal", "Tropical seasonal"
	}
	warmRain, coldRain := 0., 0.
	for _, x := range v {
		if x.Temperature > mean {
			warmRain += x.Rain
		} else {
			coldRain += x.Rain
		}
	}
	if hi > 20 && lo > 0 && warmRain < coldRain*.5 {
		return "mediterranean", "Mediterranean"
	}
	if lo > 0 && hi > 22 {
		return "subtropical", "Humid subtropical"
	}
	if lo > -3 && hi-lo < 28 {
		return "oceanic", "Oceanic"
	}
	if mean < 5 {
		return "boreal", "Boreal continental"
	}
	return "continental", "Continental"
}

func (e *Environment) BuildClimate() {
	p, scale := e.Options.climateConfiguration()
	e.buildClimate(p, scale)
}

func (e *Environment) buildClimate(p ClimateParameters, scale ClimateScale) {
	state := &ClimateState{Version: ClimateModelVersion, Parameters: p, Scale: scale, Cells: make([]ClimateCell, len(e.Mask)), Zones: []ClimateRegion{}, Source: "terrain elevation, advected annual precipitation, circulation, water bodies, vegetation and geological heat; monthly means, not forecasts"}
	waterMask := make([]int, len(e.Mask))
	for i := range waterMask {
		if e.climateField("waterBody", i) > 0 || e.climateField("river", i) > 0 {
			waterMask[i] = 1
		}
	}
	waterDistance := e.gridDistance(waterMask, 1)
	kinds, names := make([]string, len(e.Mask)), make([]string, len(e.Mask))
	var representative [12]ClimateMonth
	landCount := 0
	for i := range state.Cells {
		c := &state.Cells[i]
		c.Months, c.Modifiers = e.climateMonths(i, p, scale, waterDistance)
		c.Seasons, c.Estimate = climateSeasons(c.Months, c.Modifiers, p)
		kinds[i], names[i] = climateClass(c.Months, c.Modifiers, p)
		if e.climateField("waterBody", i) == 0 {
			landCount++
			for month, v := range c.Months {
				target := month
				if c.Modifiers.Latitude < 0 {
					target = (month + 6) % 12
				}
				representative[target].Temperature += v.Temperature
				representative[target].Rain += v.Rain
				representative[target].Snow += v.Snow
				representative[target].SoilMoisture += v.SoilMoisture
			}
		}
	}
	if landCount == 0 {
		representative = state.Cells[0].Months
	} else {
		for i := range representative {
			representative[i].Temperature /= float64(landCount)
			representative[i].Rain /= float64(landCount)
			representative[i].Snow /= float64(landCount)
			representative[i].SoilMoisture /= float64(landCount)
		}
	}
	state.Calendar, _ = climateSeasons(representative, ClimateModifiers{}, p)
	state.FinalCount = len(state.Calendar)
	// Connected cell sets retain irregular geographic boundaries and stable IDs.
	seen := make([]bool, len(e.Mask))
	for start := range seen {
		if seen[start] {
			continue
		}
		seen[start] = true
		cells := []int{start}
		for head := 0; head < len(cells); head++ {
			for _, j := range e.neighbors(cells[head]) {
				if !seen[j] && kinds[j] == kinds[start] {
					seen[j] = true
					cells = append(cells, j)
				}
			}
		}
		zone := len(state.Zones)
		for _, i := range cells {
			state.Cells[i].Zone = zone
		}
		state.Zones = append(state.Zones, ClimateRegion{fmt.Sprintf("climate-%08x-%d", Seed(e.Options.Seed), start), kinds[start], names[start], cells})
	}
	e.Climate = state
}

// Recompute the climate of an edited surface without rebuilding or moving its
// terrain, rivers or deposits. Existing annual fields seed the water balance;
// coast distances, slope, orographic rain and lapse rates respond to the edit.
func (e *Environment) RebuildClimateForSurface(base *Environment) {
	for name, v := range e.Fields {
		e.Fields[name] = append([]float64(nil), v...)
	}
	e.Mask = append([]int(nil), base.Mask...)
	for i := range e.Mask {
		if e.climateField("waterBody", i) > 0 {
			e.Mask[i] = 0
		} else {
			e.Mask[i] = 1
		}
		oldHeight := math.Max(0, base.climateField("elevation", i))
		if base.climateField("waterBody", i) > 0 {
			oldHeight = 0
		}
		height := math.Max(0, e.climateField("elevation", i))
		if e.Mask[i] == 0 {
			height = 0
		}
		e.set("temperature", i, base.climateField("temperature", i)-(height-oldHeight)*.0065)
	}
	ocean := make([]int, len(e.Mask))
	for i := range ocean {
		if e.climateField("waterBody", i) == 1 {
			ocean[i] = 1
		}
	}
	e.Fields["oceanDistance"] = e.gridDistance(ocean, 1)
	e.precipitation()
	if e.Options.ClimateAdjustments != nil {
		for i, factor := range e.Options.ClimateAdjustments {
			e.set("precipitation", i, e.get("precipitation", i)*factor)
		}
	}
	if base.Climate != nil {
		// Saved planetary parameters take precedence over today's seeded defaults,
		// including when a loaded historical terrain is edited for the first time.
		e.buildClimate(base.Climate.Parameters, base.Climate.Scale)
	} else {
		e.BuildClimate()
	}
	e.Climate.Source += "; recalculated from authored terrain and water surfaces"
}
