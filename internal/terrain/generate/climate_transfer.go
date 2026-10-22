package generate

import (
	"fmt"
	"kriemhild/internal/terrain/model"
	"math"
)

// Remove the lapse correction at source vertices BEFORE interpolation. Averaging
// seabed and land elevations first incorrectly treats bathymetry as air height.
func seaLevelClimate(g model.Grid) model.Grid {
	out := model.Grid{Width: g.Width, Height: g.Height, Fields: map[string][]float64{}}
	keys := append([]string{"temperature"}, temperatureMonths[:]...)
	for _, key := range keys {
		a := make([]float64, g.Width*g.Height)
		for i, t := range g.Fields[key] {
			a[i] = t + math.Max(0, g.Fields["bed"][i])*.0065
		}
		out.Fields[key] = a
	}
	return out
}

// Version 2 keeps the existing precipitation simulation and PET formula, but
// evaluates evaporative demand and dry-season summaries at the corrected local
// temperature. Soil remains the original transport model's coarse reservoir.
func materializedMoisture(g *model.Grid, spec model.Spec) {
	n := g.Width * g.Height
	for _, key := range []string{"pet", "dryMonths", "warmDry", "coldRain", "climateElevation"} {
		g.Fields[key] = make([]float64, n)
	}
	for i, b := range g.Fields["bed"] {
		g.Fields["climateElevation"][i] = math.Max(0, b)
	}
	for m := 0; m < 12; m++ {
		key := fmt.Sprintf("pet-%02d", m)
		g.Fields[key] = make([]float64, n)
		decl := spec.Tilt * math.Pi / 180 * math.Sin(model.Tau*(float64(m)-2)/12)
		for i, t := range g.Fields[temperatureMonths[m]] {
			lat := g.Point(i%g.Width, i/g.Width)[1]
			pet := math.Max(4, 38+t*3.8) * (1 + .15*math.Cos(lat-decl))
			rain := g.Fields[rainfallMonths[m]][i]
			g.Fields[key][i] = pet
			g.Fields["pet"][i] += pet
			if rain < pet*.5 {
				g.Fields["dryMonths"][i]++
				if t > 15 {
					g.Fields["warmDry"][i]++
				}
			}
			if t < 15 {
				g.Fields["coldRain"][i] += rain
			}
		}
	}
	for i, rain := range g.Fields["precipitation"] {
		ratio := rain / math.Max(100, g.Fields["pet"][i])
		g.Fields["aridity"][i] = ratio
		g.Fields["moisture"][i] = model.Clamp(ratio*.6, 0, 1)
	}
}
