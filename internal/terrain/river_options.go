package terrain

import (
	"fmt"
	"math"
)

type RiverOptions struct {
	MinAreaKM2       float64  `json:"minimum_area_km2"`
	MinDischarge     float64  `json:"minimum_discharge_m3s"`
	VisibleOrder     int      `json:"visible_stream_order"`
	MajorAreaKM2     float64  `json:"major_area_km2"`
	RegionalAreaKM2  float64  `json:"regional_area_km2"`
	EphemeralDensity *float64 `json:"ephemeral_density,omitempty"`
}

func (o EnvironmentOptions) validateRiverOptions() error {
	if r := o.RiverOptions; r != nil {
		for _, v := range []float64{r.MinAreaKM2, r.MinDischarge, r.MajorAreaKM2, r.RegionalAreaKM2} {
			if !finite(v) || v < 0 {
				return fmt.Errorf("river thresholds must be nonnegative")
			}
		}
		if r.VisibleOrder < 0 || r.VisibleOrder > 12 {
			return fmt.Errorf("visible stream order must be 0?12")
		}
		if r.EphemeralDensity != nil && (!finite(*r.EphemeralDensity) || *r.EphemeralDensity < 0 || *r.EphemeralDensity > 1) {
			return fmt.Errorf("ephemeral density must be 0?1")
		}
	}
	if r := o.Erosion; r != nil {
		if r.Iterations < 0 || r.Iterations > 6 || !finite(r.YearsMa) || r.YearsMa < 0 || r.YearsMa > 50 || !finite(r.Strength) || r.Strength < 0 || r.Strength > 3 {
			return fmt.Errorf("erosion requires 0?6 iterations, 0?50 Ma and strength 0?3")
		}
	}
	return nil
}
func (e *Environment) riverThresholds() RiverOptions {
	r := RiverOptions{MinAreaKM2: 80, MinDischarge: .5, VisibleOrder: 2, MajorAreaKM2: 20000, RegionalAreaKM2: 1500}
	if e.Options.RiverOptions != nil {
		v := e.Options.RiverOptions
		if v.MinAreaKM2 > 0 {
			r.MinAreaKM2 = v.MinAreaKM2
		}
		if v.MinDischarge > 0 {
			r.MinDischarge = v.MinDischarge
		}
		if v.VisibleOrder > 0 {
			r.VisibleOrder = v.VisibleOrder
		}
		if v.MajorAreaKM2 > 0 {
			r.MajorAreaKM2 = v.MajorAreaKM2
		}
		if v.RegionalAreaKM2 > 0 {
			r.RegionalAreaKM2 = v.RegionalAreaKM2
		}
		r.EphemeralDensity = v.EphemeralDensity
	}
	return r
}
func (e *Environment) physicalHeadwater(i int) bool {
	f := e.get
	r := e.riverThresholds()
	if f("waterBody", i) > 0 || f("flow", i) < 0 || f("catchmentArea", i) < 3 || f("catchmentKM2", i) < r.MinAreaKM2 || f("dischargeM3s", i) < r.MinDischarge {
		return false
	}
	// Contributing area and upstream water supply, never a mountain membership test.
	// Three resolved cells prevent every slope from becoming a visible source.
	if f("dryDischarge", i)/math.Max(1e-9, f("accumulation", i)) < .08 {
		density := .35
		if r.EphemeralDensity != nil {
			density = *r.EphemeralDensity
		}
		if density == 0 || f("catchmentArea", i) < 3/math.Max(.05, density) {
			return false
		}
	}
	return f("accumulation", i) >= .35*math.Max(1, float64(len(e.Mask))/16000)
}
