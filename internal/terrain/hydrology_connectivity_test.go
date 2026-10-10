package terrain

import (
	"math"
	"testing"
)

func TestRiverSourcesDestinationsAndContinuousGeometry(t *testing.T) {
	for _, seed := range []string{"KRIEMHILD", "mountain-drainage", "island-arc", "river-basin"} {
		o := realismOptions(t, `{"columns":96,"rows":64}`)
		o.Seed = seed
		e := BuildEnvironment(o)
		if len(e.Hydrology.Reaches) == 0 {
			t.Fatal("fixture has no drainage", seed)
		}
		byCell := map[int]HydroReach{}
		for _, r := range e.Hydrology.Reaches {
			byCell[r.From] = r
		}
		for _, r := range e.Hydrology.Reaches {
			if len(r.Upstream) == 0 && r.Source != "lake-outlet" && !e.supportedRiverHeadwater(r.From) {
				t.Fatal("unsupported source", seed, r)
			}
			at := r.From
			for steps := 0; steps <= len(e.Mask); steps++ {
				if steps == len(e.Mask) {
					t.Fatal("cyclic river", seed, r.ID)
				}
				if e.get("waterBody", at) > 0 {
					break
				}
				if e.get("flow", at) < 0 {
					if e.get("terminalBasin", at) == 0 {
						t.Fatal("river vanishes on dry land", seed, r.ID)
					}
					break
				}
				next, ok := byCell[at]
				if !ok {
					t.Fatal("gap in visible drainage graph", seed, r.ID, at)
				}
				if e.get("drainageElevation", next.To) > e.get("drainageElevation", at)+.01 {
					t.Fatal("backwards river", seed, r.ID)
				}
				at = next.To
			}
		}
		model := NewDetailModel(e)
		features := map[string]DetailFeature{}
		levels := map[string]float64{}
		for _, feature := range model.Features {
			if feature.Kind != "river" || feature.RiverID == "" {
				continue
			}
			if level, ok := levels[feature.RiverID]; ok && level != feature.Level {
				t.Fatal("river appears as disconnected zoom fragments", seed, feature.RiverID)
			}
			levels[feature.RiverID] = feature.Level
			features[feature.ID] = feature
		}
		for _, r := range e.Hydrology.Reaches {
			feature, ok := features[r.ID]
			if !ok {
				t.Fatal("reach geometry missing", seed, r.ID)
			}
			if e.get("waterBody", r.To) > 0 {
				continue
			}
			end := feature.Path[len(feature.Path)-1]
			if math.Hypot(end[0]-float64(r.To%o.Columns), end[1]-float64(r.To/o.Columns)) > 1e-6 {
				t.Fatal("interior channel clipped before its junction", seed, r.ID)
			}
			for i := 1; i < len(feature.Path); i++ {
				if feature.Path[i][2] > feature.Path[i-1][2]+.01 {
					t.Fatal("detail flows uphill", seed, r.ID)
				}
			}
		}
		if diagnostics := e.ValidateHydrology(); len(diagnostics) > 0 {
			t.Fatal(seed, diagnostics)
		}
	}
}

func TestUnsupportedLowlandAndOceanCannotSeedRivers(t *testing.T) {
	e := waterSupplyFixture(true)
	for i := range e.Mask {
		e.set("mountainCore", i, 0)
		e.set("glacier", i, 0)
	}
	e.Hydrology.Reaches = nil
	e.Hydrology.Statistics = nil
	visible := e.selectBasinChannels()
	for i, on := range visible {
		if on {
			t.Fatal("unanchored coastal/lowland fragment", i)
		}
	}
	e.set("flow", 0, 1)
	found := false
	for _, diagnostic := range e.ValidateHydrology() {
		if diagnostic.Reason == "ocean cannot be a natural river source" {
			found = true
		}
	}
	if !found {
		t.Fatal("marine source escaped validation")
	}
}
