package terrain

import "math"

// Morphology refines a saved reach, never replaces the catchment graph. Braid
// branches divide/rejoin at the same endpoints and share its network identity.
func (m *DetailModel) refineHydroFeatures() {
	e := m.World
	if e.Hydrology == nil {
		return
	}
	byID := map[string]HydroReach{}
	for _, r := range e.Hydrology.Reaches {
		byID[r.ID] = r
	}

	// Uniform visibility along each named river; no isolated mouth/head segments.
	levels := map[string]float64{}
	systems := map[string]RiverSystem{}
	for _, river := range e.Hydrology.Rivers {
		systems[river.ID] = river
		level := clamp(6-math.Log2(1+river.Discharge)*1.1, 2.5, 6)
		if river.Class == "major" {
			level = 0
		} else if river.Class == "regional" {
			level = 1.5
		}
		levels[river.ID] = level
	}
	// Significant tributaries are part of a world-scale trunk's silhouette.
	// Hiding all regional rivers until zoom 1.5 made connected continental
	// networks look like independent lines. Promote only supplied tributaries,
	// keeping unrelated small coastal catchments at their ordinary scale.
	for _, river := range e.Hydrology.Rivers {
		parent, ok := systems[river.Downstream]
		if ok && parent.Class == "major" && river.Class == "regional" && river.Discharge >= parent.Discharge*.08 {
			levels[river.ID] = 0
		}
	}
	// A tributary cannot become visible before its receiving river.
	for pass := 0; pass < len(e.Hydrology.Rivers); pass++ {
		changed := false
		for _, river := range e.Hydrology.Rivers {
			if parent, ok := levels[river.Downstream]; ok && levels[river.ID] < parent {
				levels[river.ID] = parent
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	if e.Geography != nil {
		minimum := e.riverThresholds().VisibleOrder
		for _, river := range e.Hydrology.Rivers {
			order := 0
			for _, id := range river.Reaches {
				order = max(order, byID[id].Order)
			}
			if river.Class == "stream" && order < minimum {
				levels[river.ID] = math.Max(levels[river.ID], 5)
			}
		}
	}
	base := m.Features
	m.Features = nil
	for _, f := range base {
		r, ok := byID[f.ID]
		if !ok && f.NetworkID != "" {
			r, ok = byID[f.NetworkID]
		}
		if !ok {
			m.Features = append(m.Features, f)
			continue
		}
		f.NetworkID = r.ID
		f.RiverID = r.RiverID
		f.Class = r.Class
		// A continental river is one anchored feature at world scale, rather
		// than disconnected high-discharge fragments appearing at different zooms.
		if r.Class == "major" {
			f.Level = 0
		} else if r.Class == "regional" {
			f.Level = math.Min(f.Level, 1.5)
		}
		if level, ok := levels[r.RiverID]; ok {
			f.Level = level
		}
		if r.Source == "lake-outlet" {
			m.connectLakeOutlet(&f, r)
		}
		f.ParentID = r.Downstream
		f.Regime = r.Regime
		f.Morphology = r.Morphology
		f.PhysicalWidthMetres = r.WidthMetres
		f.DischargeM3s = r.DischargeM3s
		f.ValleyWidthMetres = r.ValleyWidthMetres
		if r.WidthMetres > 0 && r.ValleyWidthMetres > 0 {
			f.ValleyWidth = r.ValleyWidthMetres / r.WidthMetres * r.Width
		}
		if len(r.ChannelPath) > 0 {
			f.Path = append([][3]float64(nil), r.ChannelPath...)
		}
		fraction := 0.
		for _, b := range r.Branches {
			if len(b.Path) == 0 {
				fraction += b.Fraction
			}
		}
		f.Discharge *= 1 - fraction
		f.DischargeM3s *= 1 - fraction
		factor := math.Pow(1-fraction, .56)
		f.Width *= factor
		f.PhysicalWidthMetres *= factor
		f.DischargesM3s = make([]float64, len(f.Path))
		for k := range f.Widths {
			share := fraction
			for _, b := range r.Branches {
				if len(b.Path) > 0 && k >= b.SplitNode && k <= b.JoinNode {
					share += b.Fraction
				}
			}
			f.Widths[k] *= math.Pow(1-share, .56)
			f.DischargesM3s[k] = r.DischargeM3s * (1 - share)
		}
		m.Features = append(m.Features, f)
		// Projection-seam copies share identity; create side channels once.
		if f.ID != r.ID {
			continue
		}
		for _, b := range r.Branches {
			branch := f
			branch.ID = b.ID
			branch.ParentID = b.Downstream
			branch.Discharge = r.Discharge * b.Fraction
			branch.DischargeM3s = r.DischargeM3s * b.Fraction
			factor := math.Pow(b.Fraction, .56)
			branch.Width = r.Width * factor
			branch.PhysicalWidthMetres = r.WidthMetres * factor
			branch.DischargesM3s = nil
			if len(b.Path) > 0 {
				separation := 0.
				for k, p := range b.Path {
					at := b.SplitNode + k
					if at >= 0 && at < len(r.ChannelPath) {
						anchor := r.ChannelPath[at]
						separation = math.Max(separation, math.Hypot(p[0]-anchor[0], p[1]-anchor[1]))
					}
				}
				// Reveal a saved braid as soon as its actual channel separation
				// becomes resolvable, instead of hiding every braid until level 5.
				branch.MorphologyLevel = clamp(math.Log2(.15/math.Max(1e-9, separation)), 0, 6)
				branch.Path = append([][3]float64(nil), b.Path...)
				branch.Widths = make([]float64, len(b.Path))
				for k := range branch.Widths {
					branch.Widths[k] = branch.Width
				}
				m.Features = append(m.Features, branch)
				continue
			}
			branch.Path = nil
			branch.Widths = nil
			cells := b.Cells
			if len(cells) == 0 {
				cells = []int{r.From, b.To}
			} // Stored legacy deltas.
			for k := 1; k < len(cells); k++ {
				a, c := cells[k-1], cells[k]
				z0, z1 := e.get("drainageElevation", a), e.get("drainageElevation", c)
				if e.get("waterBody", c) > 0 {
					z1 = e.get("waterLevel", c)
				}
				for step := 0; step < 12; step++ {
					t := float64(step) / 12
					branch.Path = append(branch.Path, [3]float64{float64(a%e.Options.Columns)*(1-t) + float64(c%e.Options.Columns)*t, float64(a/e.Options.Columns)*(1-t) + float64(c/e.Options.Columns)*t, z0*(1-t) + z1*t})
					branch.Widths = append(branch.Widths, branch.Width)
				}
			}
			end := [3]float64{float64(b.To % e.Options.Columns), float64(b.To / e.Options.Columns), e.get("drainageElevation", b.To)}
			if e.get("waterBody", b.To) > 0 {
				end[2] = e.get("waterLevel", b.To)
			}
			branch.Path = append(branch.Path, end)
			branch.Widths = append(branch.Widths, branch.Width)
			m.Features = append(m.Features, branch)
		}
	}
	for _, sp := range e.Hydrology.Springs {
		if e.Hydrology.NetworkVersion >= 2 {
			continue
		} // Groundwater feeds the model without inventing isolated visible rivers.
		if _, visible := byID[e.riverID(sp.Cell)]; visible {
			continue
		}
		f := DetailFeature{ID: sp.ID + "/outflow", NetworkID: sp.ID, ParentID: sp.Downstream, Kind: "river", Regime: "perennial", Morphology: "spring-fed", Level: 6, Discharge: sp.Discharge, Width: .01 + math.Sqrt(sp.Discharge)*.012}
		for at, steps := sp.Cell, 0; at >= 0 && steps < 4096; steps++ {
			f.Path = append(f.Path, [3]float64{float64(at % e.Options.Columns), float64(at / e.Options.Columns), e.get("drainageElevation", at)})
			f.Widths = append(f.Widths, f.Width)
			if at != sp.Cell {
				if _, visible := byID[e.riverID(at)]; visible || e.get("waterBody", at) > 0 || e.get("flow", at) < 0 {
					break
				}
			}
			at = int(e.get("flow", at))
		}
		if len(f.Path) > 1 && len(f.Path) < 4096 {
			m.Features = append(m.Features, f)
		}
	}
	for i := range m.Features {
		m.clipHydroShore(&m.Features[i])
	}
	for _, c := range e.Hydrology.Canals {
		f := DetailFeature{ID: c.ID, NetworkID: c.ID, ParentID: c.Destination, Kind: "canal", Regime: "perennial", Morphology: "engineered", Level: 4, Width: .018, Discharge: c.Supply, Depth: 2}
		for _, i := range c.Path {
			f.Path = append(f.Path, [3]float64{float64(i % e.Options.Columns), float64(i / e.Options.Columns), func() float64 {
				if e.get("waterBody", i) > 0 {
					return e.get("waterLevel", i)
				}
				return e.get("elevation", i)
			}()})
			f.Widths = append(f.Widths, f.Width)
		}
		if len(f.Path) > 1 {
			m.Features = append(m.Features, f)
		}
	}
}

func (m *DetailModel) clipHydroShore(f *DetailFeature) {
	for k := 1; k < len(f.Path); k++ {
		a, b := f.Path[k-1], f.Path[k]
		wet := m.base(b[0], b[1])
		if wet.WaterBody == 0 || (k == 1 && m.base(a[0], a[1]).WaterBody > 0) {
			continue
		}
		lo, hi := 0., 1.
		for n := 0; n < 26; n++ {
			t := (lo + hi) / 2
			if m.base(a[0]+(b[0]-a[0])*t, a[1]+(b[1]-a[1])*t).WaterBody > 0 {
				hi = t
			} else {
				lo = t
			}
		}
		f.Path[k] = [3]float64{a[0] + (b[0]-a[0])*lo, a[1] + (b[1]-a[1])*lo, wet.WaterLevel}
		f.Path = f.Path[:k+1]
		if len(f.Widths) > k {
			f.Widths = f.Widths[:k+1]
		}
		if len(f.DischargesM3s) > k {
			f.DischargesM3s = f.DischargesM3s[:k+1]
		}
		return
	}
}

// The routing graph contains a lake's outlet edge but land-only reach geometry
// starts at the next dry node. Include that shoreline-to-node section explicitly.
func (m *DetailModel) connectLakeOutlet(feature *DetailFeature, r HydroReach) {
	if len(feature.Path) < 2 {
		return
	}
	e := m.World
	for _, from := range e.drainageNeighbors(r.From) {
		if e.get("lake", from) == 0 || int(e.get("flow", from)) != r.From {
			continue
		}
		start := [3]float64{float64(from % e.Options.Columns), float64(from / e.Options.Columns), e.get("waterLevel", from)}
		end := feature.Path[0]
		lo, hi := 0., 1.
		for n := 0; n < 26; n++ {
			t := (lo + hi) / 2
			if m.base(start[0]+(end[0]-start[0])*t, start[1]+(end[1]-start[1])*t).WaterBody > 0 {
				lo = t
			} else {
				hi = t
			}
		}
		start[0] += (end[0] - start[0]) * hi
		start[1] += (end[1] - start[1]) * hi
		path := make([][3]float64, 0, len(feature.Path)+12)
		widths := make([]float64, 0, len(feature.Widths)+12)
		for k := 0; k < 12; k++ {
			t := float64(k) / 12
			path = append(path, [3]float64{start[0] + (end[0]-start[0])*t, start[1] + (end[1]-start[1])*t, start[2] + (end[2]-start[2])*t})
			widths = append(widths, feature.Widths[0])
		}
		feature.Path = append(path, feature.Path...)
		feature.Widths = append(widths, feature.Widths...)
		return
	}
}
