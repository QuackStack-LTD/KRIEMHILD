package generate

import (
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
	"math"
)

// V4 retains the conservative v3 moisture supply and seasonal inertia. The
// lower tropical intercept and gentler high-latitude gradient correct thermal
// regimes without a global rainfall reduction. Maritime heat redistribution
// tapers smoothly inland. Regional forcing is continuous on the sphere.
func annualThermalV4(seed uint64, lon, lat, coast float64) float64 {
	continental := 1 - math.Exp(-math.Max(0, coast)/700)
	return 27 - 46*math.Pow(math.Sin(lat), 2) +
		6*(1-continental)*math.Pow(math.Sin(2*lat), 2) +
		1.25*landmass.Noise(seed+8741, lon, lat, 2)
}

// Hargreaves reference evaporation, FAO-56 equation 52:
// https://www.fao.org/4/X0490E/x0490e07.htm
// Ra is converted from W/m2 to equivalent mm/day (0.0864 * 0.408).
// Daily temperature range is not simulated: 8-12 C is an explicit, reduced
// maritime/continental approximation, not observed daily weather. Unlike the
// historical linear formula, demand follows available sunlight and polar night.
func potentialEvaporationV4(temp, lat, decl, coast float64) float64 {
	continental := 1 - math.Exp(-math.Max(0, coast)/700)
	ra := dailyInsolation(lat, decl) * .0864 * .408
	return .0023 * math.Max(0, temp+17.8) * math.Sqrt(8+4*continental) * ra * (365.2425 / 12)
}

// MonthlyClimateAt exposes version-4 monthly values at the actual atmospheric
// elevation for inspection/projection. Month is zero based, in [0,11].
func MonthlyClimateAt(p *model.Planet, lon, lat, bed float64, month int) (temp, rain, pet float64) {
	temp = p.Grid.At(temperatureMonths[month], lon, lat) - p.AtmosphericLapse(lon, lat, bed)
	rain = p.Grid.At(rainfallMonths[month], lon, lat)
	decl := p.Spec.Tilt * math.Pi / 180 * math.Sin(model.Tau*(float64(month)-2)/12)
	pet = potentialEvaporationV4(temp, lat, decl, p.Grid.At("coast", lon, lat))
	return
}

// A shared interpolation stencil avoids repeating coordinate calculations for
// the monthly fields. Monthly extrema and counts are computed after sampling,
// never interpolated as if they were continuous climate quantities.
type climateStencil struct {
	i [4]int
	w [4]float64
}

func monthlyStencil(g model.Grid, lon, lat float64) climateStencil {
	x := (model.Wrap(lon) + math.Pi) / model.Tau * float64(g.Width)
	y := model.Clamp((.5-lat/math.Pi)*float64(g.Height)-.5, 0, float64(g.Height-1))
	i, j := int(x)%g.Width, int(y)
	u, v := x-math.Floor(x), y-float64(j)
	i1, j1 := (i+1)%g.Width, min(j+1, g.Height-1)
	return climateStencil{[4]int{j*g.Width + i, j*g.Width + i1, j1*g.Width + i, j1*g.Width + i1}, [4]float64{(1 - u) * (1 - v), u * (1 - v), (1 - u) * v, u * v}}
}

func (s climateStencil) at(a []float64) float64 {
	if len(a) == 0 {
		return 0
	}
	return a[s.i[0]]*s.w[0] + a[s.i[1]]*s.w[1] + a[s.i[2]]*s.w[2] + a[s.i[3]]*s.w[3]
}

func climateRegimeV4(p *model.Planet, lon, lat, bed float64) ClimateRegime {
	g := p.Grid
	s := monthlyStencil(g, lon, lat)
	delta := p.AtmosphericLapse(lon, lat, bed)
	c := ClimateRegime{Coldest: math.Inf(1), Warmest: math.Inf(-1), Driest: math.Inf(1), SummerDriest: math.Inf(1), MinimumMonthlyBalance: math.Inf(1), Thermal: "temperate", Rainfall: "seasonal"}
	c.Continentality = 1 - math.Exp(-math.Max(0, s.at(g.Fields["coast"]))/700)
	var dry [12]bool
	for m := 0; m < 12; m++ {
		temp := s.at(g.Fields[temperatureMonths[m]]) - delta
		rain := s.at(g.Fields[rainfallMonths[m]])
		// Re-evaluate PET at the sampled elevation using the same equation as
		// climate/materializedMoisture. Thus detail and grid use one water demand.
		decl := p.Spec.Tilt * math.Pi / 180 * math.Sin(model.Tau*(float64(m)-2)/12)
		pet := potentialEvaporationV4(temp, lat, decl, s.at(g.Fields["coast"]))
		c.Coldest = math.Min(c.Coldest, temp)
		c.Warmest = math.Max(c.Warmest, temp)
		c.Driest = math.Min(c.Driest, rain)
		c.Wettest = math.Max(c.Wettest, rain)
		c.Rain += rain
		c.PET += pet
		c.WaterDeficit += math.Max(0, pet-rain)
		c.MeanTemperature += temp / 12
		if pet > 0 {
			c.MinimumMonthlyBalance = math.Min(c.MinimumMonthlyBalance, rain/pet)
		}
		dry[m] = rain < pet*.5
		if dry[m] {
			c.DryMonths++
		}
		if temp >= 10 {
			c.GrowingMonths++
			if dry[m] {
				c.GrowingDryMonths++
			}
		}
		if temp < 0 {
			c.FreezingMonths++
		}
		summer := m >= 3 && m <= 8
		if lat < 0 {
			summer = !summer
		}
		if summer {
			c.SummerRain += rain
			c.SummerDriest = math.Min(c.SummerDriest, rain)
		} else {
			c.WinterRain += rain
			c.WinterWettest = math.Max(c.WinterWettest, rain)
		}
	}
	consecutive := 0.
	for m := 0; m < 24; m++ {
		if dry[m%12] {
			consecutive = math.Min(12, consecutive+1)
		} else {
			consecutive = 0
		}
		c.LongestDrySeason = math.Max(c.LongestDrySeason, consecutive)
	}
	c.TemperatureRange = c.Warmest - c.Coldest
	if math.IsInf(c.MinimumMonthlyBalance, 1) {
		c.MinimumMonthlyBalance = 0
	}
	c.Aridity = c.Rain / math.Max(100, c.PET)
	switch {
	case c.Warmest < 0:
		c.Thermal = "ice"
	case c.Warmest < 10 || c.GrowingMonths < 2:
		c.Thermal = "tundra"
	case c.Coldest >= 18:
		c.Thermal = "tropical"
	case c.Coldest < -3 && (c.GrowingMonths < 4 || (c.Coldest < -18 && c.GrowingMonths < 6)):
		c.Thermal = "boreal"
	}
	switch {
	case c.Aridity < .22:
		c.Moisture = "arid"
	case c.Aridity < .5:
		c.Moisture = "semi-arid"
	case c.Aridity < .8:
		c.Moisture = "subhumid"
	case c.Aridity < 1:
		c.Moisture = "moist"
	default:
		c.Moisture = "humid"
	}
	c.Perhumid = c.Driest >= 60 && c.MinimumMonthlyBalance >= .65 && c.DryMonths == 0
	if c.Perhumid {
		c.Rainfall = "perennial-wet"
	}
	if c.Thermal == "temperate" && c.Coldest > -3 && c.Warmest > 18 && c.SummerDriest < 40 && c.SummerDriest*3 < c.WinterWettest && c.SummerRain*2 < c.WinterRain {
		c.Rainfall = "summer-dry"
	}
	return c
}
