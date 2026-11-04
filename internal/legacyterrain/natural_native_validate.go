package terrain

import "math"

// Native continuous rivers remain in the continuous snapshot, not a second
// generated legacy network. Validate their explicitly declared read projection
// using saved receiver-anchor levels, annual recharge and actual emergence.
func (e *Environment) validateContinuousHydrology() []HydroDiagnostic {
	out := []HydroDiagnostic{}
	bad := func(reason string) {
		out = append(out, HydroDiagnostic{Object: "continuous-hydrology", Reason: reason, Severity: "error"})
	}
	if e.Hydrology.Projection != "continuous-receivers/1" || len(e.NaturalSource) != 64 || e.Hydrology.Version != 1 {
		bad("unsupported continuous receiver projection")
		return out
	}
	n := len(e.Mask)
	for _, k := range append(append([]string{}, HydrologyFields...), "flow", "drainageElevation", "ocean", "waterBody", "precipitation") {
		if len(e.Fields[k]) != n {
			bad("missing physical field " + k)
			return out
		}
	}
	covered := make([]bool, n)
	bodies := map[int]string{}
	for _, b := range e.WaterBodies {
		if b.ID <= 0 || bodies[b.ID] != "" || len(b.Cells) == 0 {
			bad("invalid water body")
			continue
		}
		bodies[b.ID] = b.Kind
		for _, i := range b.Cells {
			if i < 0 || i >= n || covered[i] || int(e.Fields["waterBody"][i]) != b.ID {
				bad("invalid water footprint")
				continue
			}
			covered[i] = true
		}
	}
	for i := 0; i < n; i++ {
		f := func(k string) float64 { return e.Fields[k][i] }
		v := f("flow")
		j := int(v)
		if v != float64(j) || j < -1 || j >= n || j == i {
			bad("invalid receiver")
			continue
		}
		if j >= 0 && e.Fields["drainageElevation"][j] >= f("drainageElevation") {
			bad("non-descending saved drainage anchor")
		}
		if (f("waterBody") > 0) != covered[i] {
			bad("unregistered water")
		}
		if f("ocean") > 0 && (bodies[int(f("waterBody"))] != "ocean" || f("waterLevel") != 0 || j >= 0) {
			bad("marine classification/source mismatch")
		}
		if !finite(f("recharge")) || f("recharge") < 0 || f("recharge")*1000 > math.Max(0, f("precipitation"))+.001 || f("runoff") < 0 || f("groundFlow") < 0 || f("baseflow") > f("groundFlow")+.001 {
			bad("groundwater exceeds physical supply")
		}
	}
	for _, s := range e.Hydrology.Springs {
		if s.Cell < 0 || s.Cell >= n {
			bad("spring outside terrain")
			continue
		}
		i := s.Cell
		if !finite(s.Discharge) || s.Discharge <= 0 || s.Discharge > e.Fields["groundFlow"][i] || e.Fields["waterBody"][i] > 0 || s.Origin == "" || int(e.Fields["flow"][i]) < 0 || !finite(s.Temperature) {
			bad("spring lacks groundwater/emergence")
		}
		if s.Temperature > 35 && e.Fields["geothermal"][i] < 55 {
			bad("thermal spring lacks elevated heat flow")
		}
	}
	for _, wet := range e.Hydrology.Wetlands {
		if wet.Source == "" {
			bad("wetland lacks source")
		}
		for _, i := range wet.Cells {
			if i < 0 || i >= n || e.Fields["wetland"][i] < .6 || e.Fields["slope"][i] > .1 {
				bad("invalid wetland")
			}
		}
	}
	return out
}
