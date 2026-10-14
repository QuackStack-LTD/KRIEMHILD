package terrain

import (
	"math"
	"sort"
)

type climatePhase struct{ months []int }

func climateSeasons(curve [12]ClimateMonth, mod ClimateModifiers, p ClimateParameters) ([]ClimateSeason, int) {
	lo, hi, dry, wet, rain, evap := math.Inf(1), math.Inf(-1), math.Inf(1), 0., 0., 0.
	for _, m := range curve {
		lo = math.Min(lo, m.Temperature)
		hi = math.Max(hi, m.Temperature)
		dry = math.Min(dry, m.Rain)
		wet = math.Max(wet, m.Rain)
		rain += m.Rain
		evap += m.Evaporation
	}
	thermal := clamp((hi-lo)/35, 0, 1)
	pluvial := clamp((wet-dry)/(wet+dry+20), 0, 1)
	estimate := int(clamp(math.Round(1+3*thermal+2*pluvial+mod.Monsoon), 1, 6))
	target := estimate
	// Calibrate the score against actual curves: aridity is not four seasons;
	// perennial cold and year-round rain can be single regimes. Wet/dry cycles
	// remain meaningful even with almost no thermal change.
	switch {
	case hi-lo < 4 && pluvial < .35:
		target = 1
	case rain < evap*.25:
		target = 1
		if hi-lo >= 12 {
			target = 2
		}
	case hi < 10:
		target = 2
		if hi-lo < 5 {
			target = 1
		}
	case lo >= 18 && mod.Monsoon > .45 && pluvial > .4:
		target = 2
	case hi-lo < 8:
		target = 1
		if pluvial > .40 && wet > 25 {
			target = 2
		}
	case hi-lo > 12:
		target = 4
		if p.AxialTilt > 55 && p.Eccentricity > .15 {
			target = estimate
		}
	default:
		target = 2
		if pluvial > .6 && hi-lo > 8 {
			target = 3
		}
	}
	if p.Mode == "custom" {
		target = p.RequestedCount
	}
	target = max(p.MinSeasons, min(p.MaxSeasons, target))
	phases := make([]climatePhase, 12)
	for i := range phases {
		phases[i].months = []int{i}
	}
	vector := func(s climatePhase) [4]float64 {
		v := [4]float64{}
		for _, i := range s.months {
			m := curve[i]
			v[0] += m.Temperature / 8
			v[1] += m.Rain / math.Max(30, rain/12)
			v[2] += m.Snow
			v[3] += m.SoilMoisture
		}
		for i := range v {
			v[i] /= float64(len(s.months))
		}
		return v
	}
	for len(phases) > 1 {
		best, cost := 0, math.Inf(1)
		for i, a := range phases {
			b := phases[(i+1)%len(phases)]
			u, v := vector(a), vector(b)
			d := 0.
			for k := range u {
				d += (u[k] - v[k]) * (u[k] - v[k])
			}
			if p.Mode != "custom" && target == 2 && hi < 10 && hi > 0 && lo < 0 && (curve[a.months[0]].Temperature > 0) != (curve[b.months[0]].Temperature > 0) {
				d += 1000
			}
			d *= float64(len(a.months)*len(b.months)) / float64(len(a.months)+len(b.months))
			if d < cost {
				best, cost = i, d
			}
		}
		if len(phases) <= target && (p.Mode == "custom" || cost > .06 || len(phases) <= p.MinSeasons) {
			break
		}
		j := (best + 1) % len(phases)
		phases[best].months = append(phases[best].months, phases[j].months...)
		phases = append(phases[:j], phases[j+1:]...)
	}
	if p.Mode == "custom" && hi-lo < .25 && wet-dry < 1 {
		// With no measurable seasonal forcing, use honest calendar divisions
		// instead of a long accidental segment produced by tied merge costs.
		phases = make([]climatePhase, target)
		for i := range phases {
			for month := i * 12 / target; month < (i+1)*12/target; month++ {
				phases[i].months = append(phases[i].months, month)
			}
		}
	}
	sort.Slice(phases, func(i, j int) bool { return phases[i].months[0] < phases[j].months[0] })
	seasons := make([]ClimateSeason, len(phases))
	means := make([]ClimateMonth, len(phases))
	for i, s := range phases {
		for _, m := range s.months {
			means[i].Temperature += curve[m].Temperature / float64(len(s.months))
			means[i].Rain += curve[m].Rain / float64(len(s.months))
			means[i].Daylight += curve[m].Daylight / float64(len(s.months))
		}
	}
	for i, s := range phases {
		m, previous := means[i], means[(i+len(means)-1)%len(means)]
		name := "Stable season"
		if len(phases) > 1 {
			switch {
			case hi < 10:
				name = "Cold season"
				if m.Daylight < 2 {
					name = "Polar night"
				} else if m.Temperature > 0 {
					name = "Thaw season"
				} else if m.Temperature > (hi+lo)/2 {
					name = "Brief warm season"
				}
			case hi-lo < 8 || lo >= 18 && mod.Monsoon > .45 || rain < evap*.25:
				name = "Dry season"
				if m.Rain > math.Max(25, rain/12*1.1) {
					name = "Wet season"
					if mod.Monsoon > .45 {
						name = "Monsoon season"
					}
				} else if rain < evap*.25 && m.Temperature > (hi+lo)/2 {
					name = "Hot dry season"
				} else if rain < evap*.25 {
					name = "Cool dry season"
				}
			default:
				if m.Temperature >= lo+(hi-lo)*.72 {
					name = "Summer"
				} else if m.Temperature <= lo+(hi-lo)*.28 {
					name = "Winter"
				} else if m.Temperature > previous.Temperature {
					name = "Spring"
				} else {
					name = "Autumn"
				}
			}
		} else if rain < evap*.25 {
			name = "Arid year"
		} else if hi < 0 {
			name = "Frozen year"
		} else if dry > 60*p.OrbitalDays/365 {
			name = "Perennial wet season"
		}
		seasons[i] = ClimateSeason{name, s.months[0], len(s.months), p.Mode == "custom"}
	}
	// Custom subdivisions can honestly have similar conditions. Label them
	// separately rather than fabricating additional changes in the simulation.
	counts := map[string]int{}
	for _, s := range seasons {
		counts[s.Name]++
	}
	seen := map[string]int{}
	for i := range seasons {
		name := seasons[i].Name
		if counts[name] > 1 {
			seen[name]++
			qualifiers := []string{"Early ", "Middle ", "Late ", "Later ", "Closing ", "Final "}
			seasons[i].Name = qualifiers[seen[name]-1] + name
		}
	}
	if len(p.Names) == len(seasons) {
		for i := range seasons {
			seasons[i].Name = p.Names[i]
		}
	}
	return seasons, estimate
}
