package terrain

import "math"

// Coarse samples must not turn a local depression into a thousand-kilometre
// excavation. Keep one resolvable centre; only expand its apron where the
// represented distance permits it. Deep tectonic basins can be broader than
// ordinary storage depressions and calderas.
func (e *Environment) basinRadiusCells(i int, cells, kilometres float64) float64 {
	if e.Geography == nil {
		return cells
	}
	spacing := math.Sqrt(e.cellArea(i))
	return math.Min(cells, math.Max(.65, kilometres/math.Max(1, spacing)))
}

// Open water remains exposed to evaporative demand after surrounding soils
// have dried. Dry-air advection and wind therefore matter in terminal basins;
// a temperature-only budget systematically overfilled broad desert lakes.
func (e *Environment) lakeEvaporation(i int, temperature float64) float64 {
	base := math.Max(.12, (temperature+25)*.028)
	dryAir := clamp(e.get("aridity", i), 0, 1)
	wind := clamp(e.get("windStrength", i), 0, 1)
	return base * (1 + .65*dryAir + .35*wind*dryAir)
}
