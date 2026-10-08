package terrain

import "math"

// Geology: 0 ocean basin, 1 passive margin/lowland, 2 active margin,
// 3 young collision range, 4 eroded range, 5 plateau, 6 volcanic edifice.
// These are formation types, independent of the final terrain/biome palette.
func (e *Environment) buildGeologicalSurface(sample func(float64, float64) float64, hotspots [][2]float64) {
	w, h := e.Options.Columns, e.Options.Rows
	f, set := e.get, e.set
	scale := math.Max(.6, float64(min(w, h))/100)
	landDist, seaDist := distance(e.Mask, w, h, 1), distance(e.Mask, w, h, 0)
	crestNoise := noise(Seed(e.Options.Seed) ^ 0xb47c329d)
	for i := range e.Mask {
		x, y := float64(i%w)/float64(w), float64(i/w)/float64(h)
		// Active margins are explicitly associated with subduction/volcanism.
		active := smooth((f("volcano", i) - .35) / .4)
		kind := 0.
		shelfWidth := scale * (4 + 4*sample(x*4+233, y*4+271)) * (1 - .65*active)
		d := math.Max(0, landDist[i]-.5)
		shelf := 1 - smooth((d-shelfWidth)/(7*scale))
		deep := 3600 + 1600*sample(x*5+431, y*5+371)
		// The shelf itself slopes gently; the continental slope begins beyond it.
		depth := 8 + 140*math.Pow(math.Min(1, d/shelfWidth), 1.5) + (1-shelf)*deep
		// Sparse drowned offshore ridges can support barrier reefs, with lagoons
		// behind them. No hard replacement of whole bands with shallow water.
		bank := smooth((sample(x*7+611, y*7+631) - .58) / .18)
		depth -= bank * math.Exp(-math.Pow((d-shelfWidth*.55)/(scale*.9), 2)) * math.Max(0, depth-22)
		set("shelf", i, shelf)
		z := -depth
		if e.Mask[i] != 0 {
			kind = 1
			inland := math.Max(0, seaDist[i]-.5) / scale
			coastal := smooth(inland / (9 - 4*active))
			regional := f("highland", i)
			core := f("mountainCore", i)
			age := sample(x*4+517, y*4+293)
			plateau := smooth((sample(x*5+301, y*5+227) - .54) / .28)
			// Lowlands -> regional highlands -> broad mountain mass, before detail.
			lowland := 65 + 240*sample(x*8+121, y*8+83)
			uplands := regional*1450 + plateau*750
			massif := core * (2900 + 1500*sample(x*8+17, y*8+23)) * (1 - .35*age)
			// Coherent passes and drainage valleys dissect the existing structure.
			valley := math.Exp(-math.Pow((sample(x*14+47, y*14+79)-.5)/.11, 2))
			massif *= 1 - .35*valley
			ridges := core * (1 - age*.6) * 650 * crestNoise(x*24+53, y*24+71)
			peaks := core * core * 650 * math.Pow(crestNoise(x*39+113, y*39+137), 2)
			fine := (sample(x*48+33, y*48+39) - .5) * (35 + 120*core)
			z = 12 + smooth(inland/5)*lowland + coastal*(uplands+massif+ridges+peaks+fine)*e.Options.Ruggedness/100
			if plateau > .5 {
				kind = 5
			}
			if regional > .35 {
				kind = 4
			}
			if core > .4 && age < .55 {
				kind = 3
			}
			if active > .5 && inland < 4 {
				kind = 2
			}
		} else {
			if active > .5 {
				kind = 2
			}
			// Mid-ocean ridges/trenches are nested inside the basin, outside shelves.
			if f("boundary", i) == 2 {
				z += (1 - shelf) * 700 * f("volcano", i)
			}
			if f("boundary", i) == 1 {
				z -= (1 - shelf) * 1300 * f("tectonicStress", i)
			}
		}
		for _, p := range hotspots {
			r := math.Hypot(float64(i%w)-p[0], float64(i/w)-p[1]) / scale
			base := math.Exp(-r * r / 65)
			if e.Mask[i] != 0 {
				// A shield/base surrounds the occasional steeper volcanic summit.
				z += smooth((seaDist[i]-.5)/(3*scale)) * (900*base + 1500*math.Exp(-r*r/9)) * e.Options.Ruggedness / 100
				if base > .3 {
					kind = 6
				}
			} else {
				center := min(w*h-1, max(0, int(p[1])*w+int(p[0])))
				if landDist[center] > 5*scale {
					// Drowned volcanic platform and rim blend all the way into the
					// deep base. There is no cutoff wall around the atoll lagoon.
					rim := math.Exp(-math.Pow((r-2.4)/.8, 2))
					platform := 1 - smooth((r-3)/7)
					target := -150 + 132*rim
					z = z*(1-platform) + math.Max(z, target)*platform
					set("seamount", i, math.Max(f("seamount", i), rim))
				} else {
					z = math.Min(-8, z+1000*base)
				}
				if base > .3 {
					kind = 6
				}
			}
		}
		if e.Options.Heights != nil {
			z = e.Options.Heights[i]*4 - e.Options.Sea*4
		}
		set("geology", i, kind)
		set("elevation", i, mountainCeiling(z))
		if e.Options.Heights != nil {
			set("elevation", i, z)
		}
	}
}

// Resolve sub-grid discontinuities before water, climate and drainage are
// derived. Passive shores remain shallow; deep slopes and mountain gradients
// span several cells. Only identified active/volcanic terrain gets extra relief.
// Imported heightfields are authoritative and are deliberately left untouched.
func (e *Environment) limitSurfaceGradients() {
	w, h := e.Options.Columns, e.Options.Rows
	if e.Options.Heights == nil {
		for pass := 0; pass < w+h; pass++ {
			changed := false
			for at := range e.Mask {
				i := at
				if pass%2 != 0 {
					i = len(e.Mask) - 1 - at
				}
				z := e.get("elevation", i)
				for _, j := range nb(i, w, h) {
					limit := 650.
					gi, gj := e.get("geology", i), e.get("geology", j)
					if (gi == 2 || gi == 6) && (gj == 2 || gj == 6) {
						limit = 1100
					}
					other := e.get("elevation", j)
					if e.Mask[i] != 0 {
						z = math.Min(z, other+limit)
					} else {
						z = math.Min(-4, math.Max(z, other-limit))
					}
				}
				if math.Abs(z-e.get("elevation", i)) > .01 {
					changed = true
					e.set("elevation", i, z)
				}
			}
			if !changed {
				break
			}
		}
	}
	for i := range e.Mask {
		z := e.get("elevation", i)
		e.Heights[i] = int(round(clamp(z/4, -3000, 2000)))
		e.set("elevation", i, float64(e.Heights[i]*4))
	}
}
