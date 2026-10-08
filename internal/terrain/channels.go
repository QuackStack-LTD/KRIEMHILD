package terrain

import "math"

// Runoff is retained everywhere in the hydrology fields. Only concentrated,
// sufficiently supplied catchments become visible channels. Once established,
// a channel continues downstream, including through a drier climate zone.
func (e *Environment) VisibleChannels() []bool {
	n := len(e.Mask)
	visible := make([]bool, n)
	for i := 0; i < n; i++ {
		if e.Mask[i] == 0 || e.get("waterBody", i) > 0 || e.get("flow", i) < 0 {
			continue
		}
		mountain := e.get("mountainCore", i)
		area := e.get("catchmentArea", i)
		supply := e.get("accumulation", i)
		// Accumulation already integrates precipitation over the upstream basin.
		// Seasonal thaw adds only a modest supplement, never a river on each peak.
		if e.get("summer", i) > 0 {
			supply += math.Min(supply*.15, e.get("snow", i)*area*.08)
		}
		threshold := math.Max(8, float64(n)/1200) * (1 + mountain*1.8 + e.get("slope", i)*.6 + e.get("aridity", i)*.8)
		minArea := math.Max(6, float64(n)/2400) * (1 + mountain*2)
		if supply < threshold || area < minArea {
			continue
		}
		for at, steps := i, 0; at >= 0 && steps < n; steps++ {
			if visible[at] || e.get("waterBody", at) > 0 {
				break
			}
			visible[at] = true
			at = int(e.get("flow", at))
		}
	}
	return visible
}
