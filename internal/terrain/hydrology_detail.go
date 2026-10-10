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
	for _, river := range e.Hydrology.Rivers {
		level := clamp(6-math.Log2(1+river.Discharge)*1.1, 2.5, 6)
		if river.Class == "major" {
			level = 0
		} else if river.Class == "regional" {
			level = 1.5
		}
		levels[river.ID] = level
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
	base := m.Features
	m.Features = nil
	for _, f := range base {
		r, ok := byID[f.ID]
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
		if r.Morphology == "braided" && len(f.Path) > 3 {
			for side := 0; side < 2; side++ {
				branch := f
				branch.Path = append([][3]float64(nil), f.Path...)
				branch.Widths = append([]float64(nil), f.Widths...)
				branch.Width *= .58
				branch.Discharge *= .5
				if side == 1 {
					branch.ID += "/braid"
				}
				sign := 1.
				if side == 1 {
					sign = -1
				}
				for k, p := range branch.Path {
					a, b := f.Path[max(0, k-1)], f.Path[min(len(f.Path)-1, k+1)]
					length := math.Hypot(b[0]-a[0], b[1]-a[1])
					if length == 0 {
						continue
					}
					t := float64(k) / float64(len(f.Path)-1)
					offset := math.Sin(math.Pi*t) * f.Width * .65 * sign
					branch.Path[k][0] = clamp(p[0]-(b[1]-a[1])/length*offset, 0, float64(e.Options.Columns-1))
					branch.Path[k][1] = clamp(p[1]+(b[0]-a[0])/length*offset, 0, float64(e.Options.Rows-1))
					branch.Widths[k] *= .58
				}
				m.Features = append(m.Features, branch)
			}
		} else {
			fraction := 0.
			for _, branch := range r.Branches {
				fraction += branch.Fraction
			}
			f.Discharge *= 1 - fraction
			for k := range f.Widths {
				f.Widths[k] *= math.Sqrt(1 - fraction)
			}
			f.Width *= math.Sqrt(1 - fraction)
			m.Features = append(m.Features, f)
			for _, b := range r.Branches {
				branch := f
				branch.ID = b.ID
				branch.ParentID = b.Downstream
				branch.Discharge = r.Discharge * b.Fraction
				branch.Path = nil
				branch.Widths = nil
				start := f.Path[0]
				end := [3]float64{float64(b.To % e.Options.Columns), float64(b.To / e.Options.Columns), e.get("waterLevel", b.To)}
				for k := 0; k <= 24; k++ {
					t := float64(k) / 24
					point := [3]float64{start[0] + (end[0]-start[0])*t, start[1] + (end[1]-start[1])*t, start[2] + (end[2]-start[2])*t}
					branch.Path = append(branch.Path, point)
					branch.Widths = append(branch.Widths, r.Width*math.Sqrt(b.Fraction))
					if wet := m.base(point[0], point[1]); k > 0 && wet.WaterBody > 0 {
						a, b := branch.Path[k-1], point
						lo, hi := 0., 1.
						for n := 0; n < 20; n++ {
							t := (lo + hi) / 2
							if m.base(a[0]+(b[0]-a[0])*t, a[1]+(b[1]-a[1])*t).WaterBody > 0 {
								hi = t
							} else {
								lo = t
							}
						}
						branch.Path[k] = [3]float64{a[0] + (b[0]-a[0])*lo, a[1] + (b[1]-a[1])*lo, wet.WaterLevel}
						for n := range branch.Path {
							t := float64(n) / float64(k)
							branch.Path[n][2] = start[2]*(1-t) + wet.WaterLevel*t
						}
						break
					}
				}
				branch.Width = r.Width * math.Sqrt(b.Fraction)
				m.Features = append(m.Features, branch)
			}
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
