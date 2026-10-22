package generate

import (
	"context"
	"fmt"
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
	"math"
)

// Reduced circulation, not a GCM. Transport conserves water mass on spherical
// cells; evaporation adds it and condensation removes the recorded precipitation.
// No biome shares enter the moisture calculation.
func climate(ctx context.Context, p *model.Planet) error {
	g := &p.Grid
	n := g.Width * g.Height
	for _, key := range []string{"soil", "aridity", "rainShadow", "exposure", "seasonality", "dryMonths", "warmDry", "coldRain", "slope"} {
		g.Fields[key] = make([]float64, n)
	}
	area := make([]float64, n)
	soil := make([]float64, n)
	for i := range area {
		area[i] = math.Cos(g.Point(i%g.Width, i/g.Width)[1])
		soil[i] = 80
	}
	var seasonal [][12]float64
	if p.Spec.ClimateVersion >= 3 {
		seasonal = make([][12]float64, n)
		for i := range seasonal {
			seasonal[i] = seasonalThermalAnomaly(g.Point(i%g.Width, i/g.Width)[1], g.Fields["coast"][i], g.Fields["bed"][i], p.Spec.Tilt)
		}
	}
	for month := 0; month < 12; month++ {
		ts, rs, es := make([]float64, n), make([]float64, n), make([]float64, n)
		vapor, next := make([]float64, n), make([]float64, n)
		capacity, condensation := make([]float64, n), make([]float64, n)
		toX, toY := make([]int, n), make([]int, n)
		fluxX, fluxY := make([]float64, n), make([]float64, n)
		decl := p.Spec.Tilt * math.Pi / 180 * math.Sin(model.Tau*(float64(month)-2)/12)
		for i := range ts {
			x, y := i%g.Width, i/g.Width
			pt := g.Point(x, y)
			lat, bed := pt[1], g.Fields["bed"][i]
			continental := 1 - math.Exp(-math.Max(0, g.Fields["coast"][i])/700)
			ts[i] = 29 - 48*math.Pow(math.Sin(lat), 2) - math.Max(0, bed)*.0065 + decl*math.Sin(lat)*(12+26*continental)
			if p.Spec.ClimateVersion >= 3 {
				ts[i] = 29 - 48*math.Pow(math.Sin(lat), 2) - math.Max(0, bed)*.0065 + seasonal[i][month]
			}
			if p.Spec.ClimateVersion >= 1 && p.Spec.Version == model.Version {
				// Reduced maritime heat redistribution. The old model damped coastal
				// summers around the same frigid mean as the interior, leaving 50-degree
				// coasts below the forest growing-season limit even at sea level.
				// Tapers inland, and to zero at equator/poles; no biome or area targets.
				ts[i] += 8 * (1 - continental) * math.Pow(math.Sin(2*lat), 2)
			}
			// Trade winds, midlatitude westerlies and polar easterlies.
			// Meridional transport converges on the moving tropical rain belt.
			latitude := lat - decl*.35
			wx := math.Tanh((math.Abs(latitude)-.52)*10) * -math.Tanh((math.Abs(latitude)-1.15)*10)
			wy := -.4*math.Sin(3*latitude) + .08*landmass.Noise(p.Seed+412, pt[0], lat, 4)
			if p.Spec.Version == "continuous-1" {
				g.Fields["windX"][i], g.Fields["windY"][i] = wx, wy
			} else {
				g.Fields["windX"][i] += wx / 12
				g.Fields["windY"][i] += wy / 12
			}
			dx, dy := 1, -1 // y increases southward, windY northward
			if wx < 0 {
				dx = -1
			}
			if wy < 0 {
				dy = 1
			}
			toX[i] = y*g.Width + (x+dx+g.Width)%g.Width
			toY[i] = max(0, min(g.Height-1, y+dy))*g.Width + x
			fluxX[i] = .52 * math.Abs(wx)
			fluxY[i] = .28 * math.Abs(wy)
			upwind := y*g.Width + (x-dx+g.Width)%g.Width
			height, upwindHeight := bed, g.Fields["bed"][upwind]
			if p.Spec.ClimateVersion >= 3 {
				// Air crosses the sea surface, never the sea floor. Keep the
				// historical rule in saved versions 0/1/2 for reproducibility.
				height, upwindHeight = math.Max(0, height), math.Max(0, upwindHeight)
			}
			uplift := math.Max(0, height-upwindHeight)
			g.Fields["exposure"][i] += uplift / 12000
			g.Fields["rainShadow"][i] += math.Max(0, upwindHeight-height) / 12000
			g.Fields["slope"][i] = math.Max(g.Fields["slope"][i], math.Abs(height-upwindHeight)/(p.Spec.RadiusKM*1000*model.Tau/float64(g.Width)*math.Max(.1, math.Cos(lat))))
			convection := math.Exp(-math.Pow((lat-decl*.6)/.19, 2))
			storm := math.Exp(-math.Pow((math.Abs(latitude)-.88)/.23, 2))
			subsidence := math.Exp(-math.Pow((math.Abs(latitude)-.49)/.18, 2))
			condensation[i] = model.Clamp(.009+.11*convection+.065*storm-.018*subsidence+uplift/18000, .001, .5)
			capacity[i] = 40 * math.Exp(model.Clamp(ts[i]*.045-.9, -3, 1)) * p.Spec.Rainfall
			vapor[i] = capacity[i] * .5 * area[i]
			es[i] = math.Max(4, 38+ts[i]*3.8) * (1 + .15*math.Cos(lat-decl))
		}
		// Six-hour steps: spin up the circulation, then record 30 days. Scattering
		// mass preserves it across the seam and unequal latitude cell areas.
		initial, added, removed := 0., 0., 0.
		oceanAdded, landAdded, oceanPotential, planetArea := 0., 0., 0., 0.
		for i, weight := range area {
			planetArea += weight
			if g.Fields["bed"][i] <= 0 && (p.Spec.Version == "continuous-1" || g.Fields["coast"][i] <= 0) {
				oceanPotential += es[i] * weight
			}
		}
		for step := 0; step < 216; step++ {
			if step%8 == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			if step == 96 {
				for _, v := range vapor {
					initial += v
				}
			}
			clear(next)
			for i, v := range vapor {
				next[i] += v * (1 - fluxX[i] - fluxY[i])
				next[toX[i]] += v * fluxX[i]
				next[toY[i]] += v * fluxY[i]
			}
			for i := range vapor {
				q := next[i] / area[i]
				evap := 0.
				ocean := g.Fields["bed"][i] <= 0 && (p.Spec.Version == "continuous-1" || g.Fields["coast"][i] <= 0)
				if ocean {
					evap = math.Max(0, capacity[i]-q) * .18
				} else if ts[i] > 0 {
					// Soil-limited transpiration returns part of prior rain to
					// the air. Dry ground cannot manufacture inland moisture.
					evap = math.Min(soil[i], es[i]/120*model.Clamp(soil[i]/100, 0, 1))
					soil[i] -= evap
				}
				q += evap
				rain := math.Max(q*condensation[i], q-capacity[i])
				if g.Fields["bed"][i] > 0 || (p.Spec.Version != "continuous-1" && g.Fields["coast"][i] > 0) {
					soil[i] = math.Min(200, soil[i]+rain*.7)
				}
				vapor[i] = (q - rain) * area[i]
				if step >= 96 {
					if ocean {
						oceanAdded += evap * area[i]
					} else {
						landAdded += evap * area[i]
					}
					rs[i] += rain
					added += evap * area[i]
					removed += rain * area[i]
				}
			}
		}
		remaining := 0.
		for _, v := range vapor {
			remaining += v
		}
		residual := math.Abs(initial+added-removed-remaining) / math.Max(1, initial+added)
		if p.Spec.ClimateVersion >= 3 {
			p.Diagnostics.ClimateBudget = append(p.Diagnostics.ClimateBudget, model.ClimateWaterBudget{Month: month, OceanEvaporation: oceanAdded / planetArea, LandEvaporation: landAdded / planetArea, OceanPotential: oceanPotential / planetArea, Precipitation: removed / planetArea, StorageChange: (remaining - initial) / planetArea})
		}
		p.Diagnostics.MoistureBudgetError = math.Max(p.Diagnostics.MoistureBudgetError, residual)
		for i := range ts {
			g.Fields["soil"][i] += soil[i] / 12
			if rs[i] < es[i]*.5 {
				g.Fields["dryMonths"][i]++
				if ts[i] > 15 {
					g.Fields["warmDry"][i]++
				}
			}
			if ts[i] < 15 {
				g.Fields["coldRain"][i] += rs[i]
			}
			g.Fields["temperature"][i] += ts[i] / 12
			g.Fields["precipitation"][i] += rs[i]
			g.Fields["pet"][i] += es[i]
		}
		g.Fields[fmt.Sprintf("temperature-%02d", month)] = ts
		g.Fields[fmt.Sprintf("rainfall-%02d", month)] = rs
		g.Fields[fmt.Sprintf("pet-%02d", month)] = es
	}
	for i := range g.Fields["moisture"] {
		g.Fields["moisture"][i] = model.Clamp(g.Fields["precipitation"][i]/math.Max(100, g.Fields["pet"][i])*0.6, 0, 1)
		g.Fields["aridity"][i] = g.Fields["precipitation"][i] / math.Max(100, g.Fields["pet"][i])
		mean := g.Fields["precipitation"][i] / 12
		variance := 0.
		for month := 0; month < 12; month++ {
			v := g.Fields[fmt.Sprintf("rainfall-%02d", month)][i] - mean
			variance += v * v / 12
		}
		g.Fields["seasonality"][i] = math.Sqrt(variance) / math.Max(1, mean)
	}
	if p.Spec.ClimateVersion >= 1 && p.Spec.Version == model.Version {
		summarizeSeasons(g)
	}
	return nil
}
