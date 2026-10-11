package terrain

import "math"

// Morphology thresholds describe relief over a regional reach, not the entire
// width of a planetary cell. A 100 m fall across 200 km is a gentle floodplain,
// not a steep mountain channel. Retain local/legacy thresholds below 20 km.
func (e *Environment) channelMorphologyGrade(from, to int, drop float64) float64 {
	if e.Geography != nil {
		return drop / math.Max(1, e.stepKM(from, to)/20)
	}
	return drop
}

// Physical bankfull width, independent of cartographic visibility and valley
// extent. Mild nonlinear exponents retain differences over many discharge orders.
func physicalChannelWidth(discharge, slope, sediment float64) float64 {
	return 3.6 * math.Pow(math.Max(0, discharge), .56) * (1 + .35*clamp(sediment, 0, 1)) / math.Pow(1+math.Max(0, slope)*400, .18)
}
func (e *Environment) channelDimensions(i, j int) (metres, cells float64) {
	grade := math.Max(0, e.get("drainageElevation", i)-e.get("drainageElevation", j))
	if e.Geography == nil || e.Hydrology == nil {
		return 0, (.018 + .012*math.Sqrt(math.Max(0, e.get("accumulation", i)))) * (.45 + .55/(1+grade/180)) * e.channelBankFactor(i)
	}
	return e.ChannelDimensions(i, j, e.get("dischargeM3s", i), grade, e.get("sediment", i))
}

// ChannelDimensions is shared by generated reaches and authored river edits.
// Discharge is in m3/s, drop in metres, and returned widths in metres/grid cells.
func (e *Environment) ChannelDimensions(i, j int, discharge, drop, sediment float64) (metres, cells float64) {
	distance := math.Max(.001, e.stepKM(i, j)) * 1000
	metres = physicalChannelWidth(discharge, drop/distance, sediment) * e.channelBankFactor(i)
	return metres, metres / distance * e.hydroStep(i, j)
}

// Bank material and a seeded regional fabric are physical model inputs. Store
// the field so restoring a world never rerolls channel geometry. Resistant,
// confined bedrock supports narrow channels; erodible alluvium supports broad
// channels at the same discharge. Neighbour averaging avoids cell-edge jumps.
func (e *Environment) prepareChannelGeology() {
	n := len(e.Mask)
	factor := make([]float64, n)
	seed := Seed(e.Options.Seed) ^ 0x572ecbd1
	for i := range factor {
		resistance := .5
		if e.Geology != nil && i < len(e.Geology.Cells) {
			c := e.Geology.Cells[i]
			f := c.Basement
			if c.Cover >= 0 {
				f = c.Cover
			}
			if f >= 0 && f < len(e.Geology.Formations) {
				switch e.Geology.Formations[f].Rock {
				case "granite", "gneiss", "basalt":
					resistance = .9
				case "schist", "slate":
					resistance = .7
				case "limestone", "sandstone":
					resistance = .5
				case "shale", "mudstone", "evaporite":
					resistance = .25
				case "alluvium":
					resistance = .08
				}
			}
		}
		x, y := i%e.Options.Columns, i/e.Options.Columns
		u, v := float64(x)/12, float64(y)/12
		ix, iy := int(u), int(v)
		a, b := smooth(u-float64(ix)), smooth(v-float64(iy))
		noise := (detailHash(seed, ix, iy, 31)*(1-a)+detailHash(seed, ix+1, iy, 31)*a)*(1-b) + (detailHash(seed, ix, iy+1, 31)*(1-a)+detailHash(seed, ix+1, iy+1, 31)*a)*b
		confinement := 1 / (1 + e.get("slope", i)*2 + e.get("mountainCore", i)*.8)
		factor[i] = clamp(math.Exp((noise-.5)*1.6)*(1.9-1.45*resistance)*(.55+.45*confinement), .3, 2.8)
	}
	for pass := 0; pass < 2; pass++ {
		next := make([]float64, n)
		for i := range factor {
			sum, weight := factor[i]*4, 4.
			for _, j := range e.neighbors(i) {
				sum += factor[j]
				weight++
			}
			next[i] = sum / weight
		}
		factor = next
	}
	e.Fields["channelWidthFactor"] = factor
}
func (e *Environment) channelBankFactor(i int) float64 {
	if f := e.Fields["channelWidthFactor"]; i >= 0 && i < len(f) && f[i] > 0 {
		return f[i]
	}
	return 1
}

// Each river inherits a coherent bank fabric, rather than independent random
// widths on every reach. The final widths drive both channel-bed sampling and
// detail ribbons. Discharge and branching budgets remain unchanged.
func (e *Environment) finishChannelWidths() {
	for k := range e.Hydrology.Reaches {
		r := &e.Hydrology.Reaches[k]
		if len(e.Fields["channelWidthFactor"]) != len(e.Mask) {
			continue
		}
		// A broader log-space range gives each system a stronger physical
		// character (roughly 0.25x to 4x), coherent along its entire course.
		// Tributaries inherit the same catchment fabric as their trunk. Giving
		// each named tributary an independent multiplier could make a headwater
		// four times wider than the combined river it flowed into.
		character := math.Exp((detailHash(Seed(e.Options.Seed+"/"+r.Watershed), 0, 0, 53) - .5) * 2.8)
		e.Fields["channelWidthFactor"][r.From] *= character
		r.BankWidthFactor = e.channelBankFactor(r.From)
		r.WidthMetres, r.Width = e.channelDimensions(r.From, r.To)
		if r.ValleyWidthMetres > 0 {
			r.ValleyWidthMetres = math.Max(r.ValleyWidthMetres, r.WidthMetres*1.8)
		}
	}
	byCell, byID := map[int]int{}, map[string]int{}
	for k, r := range e.Hydrology.Reaches {
		byCell[r.From], byID[r.ID] = k, k
	}
	sourceCapacity := make(map[string]float64, len(byID))
	for _, cell := range e.accumulateWater() {
		k, ok := byCell[cell]
		if !ok {
			continue
		}
		r := &e.Hydrology.Reaches[k]
		capacity, supply, support := 0., 0., 0.
		for _, id := range r.Upstream {
			if at, exists := byID[id]; exists {
				up := e.Hydrology.Reaches[at]
				capacity += math.Pow(up.WidthMetres, 1/.56)
				supply += up.DischargeM3s
				support += sourceCapacity[id]
			}
		}
		// The receiving channel accommodates the incoming currents together.
		// Preserve genuine modeled water losses rather than widening dry reaches.
		if r.WidthMetres > 0 {
			if support == 0 {
				// Resolved headwaters remain stream-sized until other source
				// branches join them. Aggregating thousands of square kilometres
				// into one planetary cell must not spawn a kilometre-wide source.
				limit := 8 + 12*clamp(r.BankWidthFactor, .2, 2.8)
				support = math.Pow(limit, 1/2.4)
			}
			sourceCapacity[r.ID] = support
			width := math.Min(r.WidthMetres, math.Pow(support, 2.4))
			if capacity > 0 && supply > 0 {
				joined := math.Pow(capacity*math.Min(1, r.DischargeM3s/supply), .56)
				width = math.Max(width, joined)
			}
			factor := width / r.WidthMetres
			r.WidthMetres *= factor
			r.Width *= factor
			e.Fields["channelWidthFactor"][cell] *= factor
			r.BankWidthFactor = e.channelBankFactor(cell)
			r.ValleyWidthMetres = math.Max(r.ValleyWidthMetres, r.WidthMetres*1.8)
		}
		if len(r.Upstream) == 0 && r.WidthMetres > 0 {
			// A coarse headwater cell aggregates runoff over a large catchment.
			// Resolve its source-to-bankfull transition in physical geometry: the
			// spring/gully begins small and collects that runoff along the reach.
			r.SourceWidthMetres = math.Min(8, r.WidthMetres*.08)
		}
	}
}
