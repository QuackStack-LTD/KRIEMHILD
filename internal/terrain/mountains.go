package terrain

// A hard elevation clamp turned tall summits into identical flat plateaus.
// Compress only extreme generated elevations, retaining their relative heights.
func mountainCeiling(elevation float64) float64 {
	if elevation <= 6000 {
		return elevation
	}
	excess := elevation - 6000
	return 6000 + 2000*excess/(excess+2000)
}
