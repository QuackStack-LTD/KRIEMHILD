package generate

import (
	"context"
	"fmt"
	"math"
	"testing"

	"kriemhild/internal/terrain/model"
)

func atmosphericCoastFixture(depth float64, version int) *model.Planet {
	p := &model.Planet{Spec: model.DefaultSpec(), Grid: model.Grid{Width: 48, Height: 24, Fields: map[string][]float64{}}}
	p.Spec.ClimateVersion = version
	g := &p.Grid
	for _, key := range []string{"bed", "coast", "windX", "windY", "temperature", "precipitation", "pet", "moisture"} {
		g.Fields[key] = make([]float64, g.Width*g.Height)
	}
	for y := 0; y < g.Height; y++ {
		for x := 0; x < g.Width; x++ {
			i := y*g.Width + x
			g.Fields["bed"][i] = -depth
			g.Fields["coast"][i] = -500
			if x >= 16 && x < 36 {
				g.Fields["bed"][i] = 100
				g.Fields["coast"][i] = float64(min(x-15, 36-x)) * 100
			}
			if x == 26 {
				g.Fields["bed"][i] = 1800
			}
		}
	}
	return p
}

func TestAtmosphericOrographyIgnoresSeabedDepth(t *testing.T) {
	a, b := atmosphericCoastFixture(100, 3), atmosphericCoastFixture(4000, 3)
	for _, p := range []*model.Planet{a, b} {
		if err := climate(context.Background(), p); err != nil {
			t.Fatal(err)
		}
	}
	maxDifference := 0.
	for i, v := range a.Grid.Fields["precipitation"] {
		maxDifference = math.Max(maxDifference, math.Abs(v-b.Grid.Fields["precipitation"][i]))
	}
	t.Logf("changing only seabed from -100 to -4000 m changes annual precipitation by at most %.9f mm", maxDifference)
	for _, key := range []string{"precipitation", "temperature", "pet", "soil", "exposure", "rainShadow", "slope"} {
		for i, v := range a.Grid.Fields[key] {
			if math.Abs(v-b.Grid.Fields[key][i]) > 1e-9 {
				t.Errorf("%s depends on submerged relief at cell %d: %.6f vs %.6f", key, i, v, b.Grid.Fields[key][i])
				break
			}
		}
	}
	for m := 0; m < 12; m++ {
		key := fmt.Sprintf("rainfall-%02d", m)
		for i, v := range a.Grid.Fields[key] {
			if math.Abs(v-b.Grid.Fields[key][i]) > 1e-9 {
				t.Errorf("month %d rain differs", m)
				break
			}
		}
	}
}

func TestOceanEvaporationDiagnostic(t *testing.T) {
	p := atmosphericCoastFixture(4000, 3)
	if err := climate(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	actual, potential := 0., 0.
	for _, b := range p.Diagnostics.ClimateBudget {
		actual += b.OceanEvaporation
		potential += b.OceanPotential
		if math.Abs(b.OceanEvaporation+b.LandEvaporation-b.Precipitation-b.StorageChange) > 1e-7 {
			t.Fatal("diagnostic water budget fails to close")
		}
	}
	t.Logf("ocean evaporation %.6f, ocean PET %.6f mm over whole planet; ratio %.6f", actual, potential, actual/potential)
	// The temperature-only reference-vegetation PET is not a calibrated ocean
	// energy budget. The attempted strict cap is documented separately: it
	// over-dried the full-world comparison and was rejected, not promoted.
	if actual <= 0 || potential <= 0 || !model.Finite(actual/potential) {
		t.Fatal("invalid evaporation diagnostic")
	}
}

func TestExperimentalAtmosphericMountainStillProducesRainShadow(t *testing.T) {
	p := atmosphericCoastFixture(4000, 3)
	if err := climate(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	windward, lee := 0., 0.
	for _, y := range []int{5, 6, 7, 16, 17, 18} {
		for x := 20; x <= 24; x++ {
			windward += p.Grid.Fields["precipitation"][y*p.Grid.Width+x]
		}
		for x := 28; x <= 32; x++ {
			lee += p.Grid.Fields["precipitation"][y*p.Grid.Width+x]
		}
	}
	if windward <= lee {
		t.Fatalf("real positive mountain lost its rain shadow: windward %.3f lee %.3f", windward, lee)
	}
	if p.Diagnostics.MoistureBudgetError > 1e-9 {
		t.Fatal("candidate loses atmospheric water mass")
	}
}
