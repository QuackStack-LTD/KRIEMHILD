package terrain

import "math"

// Runoff is retained everywhere in the hydrology fields. Only concentrated,
// sufficiently supplied catchments become visible channels. Once established,
// a channel continues downstream, including through a drier climate zone.
func (e *Environment) VisibleChannels() []bool {
	if e.Hydrology != nil && e.Hydrology.NetworkVersion > 0 {
		if e.Hydrology.Statistics == nil {
			return e.selectBasinChannels()
		}
		// Saved networks are authoritative, including when generating previously
		// unexplored detail after loading a project made with older thresholds.
		visible := make([]bool, len(e.Mask))
		for _, r := range e.Hydrology.Reaches {
			visible[r.From] = true
			if e.get("waterBody", r.To) == 0 {
				visible[r.To] = true
			}
		}
		return visible
	}
	n := len(e.Mask)
	visible := make([]bool, n)
	for i := 0; i < n; i++ {
		if e.Mask[i] == 0 || e.get("waterBody", i) > 0 || e.get("flow", i) < 0 {
			continue
		}
		mountain := e.get("mountainCore", i)
		area := e.get("catchmentArea", i)
		supply := e.get("accumulation", i)
		// Physical hydrology already includes meltwater in accumulated runoff.
		// Only legacy environments need the seasonal thaw approximation.
		if e.Hydrology == nil && e.get("summer", i) > 0 {
			supply += math.Min(supply*.15, e.get("snow", i)*area*.08)
		}
		// Humid mountain catchments can support tributaries before becoming
		// lowland-sized basins. Keep aridity and concentration gates so isolated
		// slopes still remain runoff rather than a painted network of streams.
		mountainCatchment := clamp((mountain-.05)/.25, 0, 1)
		threshold := math.Max(8, float64(n)/1200) * (1 - mountainCatchment*.65) * (1 + e.get("slope", i)*.25 + e.get("aridity", i)*.8)
		minArea := math.Max(6, float64(n)/2400) * (1 - mountainCatchment*.5)
		if e.Hydrology != nil {
			// Admit more supplied tributary catchments without changing their
			// routes or drawing isolated slope runoff. Keep the discharge floor
			// and at least three contributing cells for every new headwater.
			threshold = math.Max(2, threshold*.27)
			minArea = math.Max(3, minArea*.7)
		}
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
