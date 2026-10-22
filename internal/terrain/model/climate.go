package model

import "math"

// AtmosphericLapse preserves old climate semantics. V2 uses the interpolated
// atmospheric reference height, which is never the depth of the seabed.
func (p *Planet) AtmosphericLapse(lon, lat, bed float64) float64 {
	if p.Spec.Version == Version && p.Spec.ClimateVersion >= 2 {
		return (math.Max(0, bed) - p.Grid.At("climateElevation", lon, lat)) * .0065
	}
	return (bed - p.Grid.At("bed", lon, lat)) * .0065
}
