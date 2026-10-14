package terrain

import (
	"fmt"
	"math"
	"reflect"
	"testing"
)

func TestTwentySeedOceanFramingAndFootprintVariety(t *testing.T) {
	forms := map[string]bool{}
	counts := map[int]bool{}
	families := map[string]bool{}
	irregular := 0
	for seed := 0; seed < 20; seed++ {
		e := shapeFixture(fmt.Sprintf("framing-%d", seed), 160, 100)
		r := e.LandShapeDiagnostics()
		counts[r.MajorCount] = true
		families[e.Layout.Family] = true
		for i, land := range e.Mask {
			expected := e.Fields["oceanReserve"][i] == 0 && e.Fields["continentalShape"][i] >= e.Layout.SeaThreshold
			if (land != 0) != expected {
				t.Fatal("final mask no longer follows union field", seed, i)
			}
		}
		if r.MajorCount < 1 || r.MajorCount > 12 || r.ReservedLandCells > 0 {
			t.Fatalf("seed %d layout %+v", seed, r)
		}
		for i, land := range e.Mask {
			x, y := i%160, i/160
			if (x == 0 || x == 159 || y == 0 || y == 99) && land != 0 {
				t.Fatalf("seed %d clipped cell %d", seed, i)
			}
		}
		if math.Abs(r.LandFraction-.42) > .015 && e.Layout.Warning == "" {
			t.Fatal("unreported area conflict", seed, r.LandFraction)
		}
		for _, s := range e.ContinentalStructures {
			forms[s.Formation] = true
		}
		for _, mass := range r.Landmasses {
			if mass.Area > 160 && (mass.Solidity < .85 || mass.Aspect > 2 || mass.BayDepth > 5) {
				irregular++
			}
		}
	}
	if len(forms) < 7 || len(counts) < 3 || len(families) < 5 || irregular < 20 {
		t.Fatal("uniform layouts", forms, counts, irregular)
	}
}

func TestMonthlyYieldAndSeasonalDiagnostics(t *testing.T) {
	e := climateFixture(t, 45, 15, 1200, 12, 100)
	for i, c := range e.Climate.Cells {
		rain, pet, et, yield := 0., 0., 0., 0.
		for _, m := range c.Months {
			rain += m.Rain
			pet += m.Evaporation
			et += m.ActualEvaporation
			yield += m.WaterYield
			if m.ActualEvaporation < 0 || m.ActualEvaporation > m.Evaporation+1e-6 || m.WaterYield < 0 {
				t.Fatal("nonconservative monthly water budget")
			}
		}
		if math.Abs(pet-e.get("annualPET", i)) > .1 || math.Abs(yield-e.get("climateWaterYield", i)) > .1 || et+yield > rain+1 {
			t.Fatal("monthly totals disagree", rain, et, yield)
		}
	}
}

func TestDryClimateRetainsConservativeStormRunoff(t *testing.T) {
	e := climateFixture(t, 28, 26, 180, 12, 100)
	for i, c := range e.Climate.Cells {
		rain, et, yield := 0., 0., 0.
		for _, m := range c.Months {
			rain += m.Rain
			et += m.ActualEvaporation
			yield += m.WaterYield
		}
		if yield <= 0 || math.Abs(et+yield-rain) > 1 || e.get("aridityIndex", i) >= .25 {
			t.Fatal("dryland storms lost runoff or created water", rain, et, yield)
		}
	}
}

func TestOceanCurrentProxyStopsAtLandAndIsDeterministic(t *testing.T) {
	e := shapeFixture("currents", 64, 48)
	e.Fields["elevation"] = make([]float64, len(e.Mask))
	e.buildOceanCurrents()
	previous := append([]float64(nil), e.Fields["current"]...)
	e.buildOceanCurrents()
	if !reflect.DeepEqual(previous, e.Fields["current"]) {
		t.Fatal("current not deterministic")
	}
	for i, v := range previous {
		if math.IsNaN(v) || math.Abs(v) > 4 {
			t.Fatal("invalid current", i, v)
		}
	}
}
