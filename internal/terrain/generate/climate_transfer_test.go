package generate

import (
	"context"
	"fmt"
	"kriemhild/internal/terrain/model"
	"math"
	"reflect"
	"testing"
)

// This fails under the previous materialization rule: sea-level air must not
// acquire a lapse correction from the elevation of the sea floor beneath it.
func TestMaterializedClimateUsesAtmosphericElevation(t *testing.T) {
	spec := model.DefaultSpec()
	spec.SurfaceWidth = 1024
	spec.ClimateVersion = 2
	p := &model.Planet{Schema: model.Schema, Spec: spec, Grid: model.Grid{Width: 192, Height: 96, Fields: map[string][]float64{}}}
	layout(p)
	plates(p)
	if err := surface(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if err := climate(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	reference := p.Grid
	reference.Fields = map[string][]float64{"sea": make([]float64, 192*96)}
	for i, b := range p.Grid.Fields["bed"] {
		reference.Fields["sea"][i] = p.Grid.Fields["temperature"][i] + math.Max(0, b)*.0065
	}
	if err := materialize(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	maxError := 0.
	for i, b := range p.Grid.Fields["bed"] {
		pt := p.Grid.Point(i%512, i/512)
		expected := reference.At("sea", pt[0], pt[1]) - math.Max(0, b)*.0065
		maxError = math.Max(maxError, math.Abs(p.Grid.Fields["temperature"][i]-expected))
		sum := 0.
		for m := 0; m < 12; m++ {
			sum += p.Grid.Fields[fmt.Sprintf("temperature-%02d", m)][i] / 12
		}
		if math.Abs(sum-p.Grid.Fields["temperature"][i]) > 1e-9 {
			t.Fatal("annual/monthly mismatch")
		}
	}
	if maxError > 1e-9 {
		t.Fatalf("temperature resampling error %.6f C", maxError)
	}
	for i, annual := range p.Grid.Fields["pet"] {
		total, dry := 0., 0.
		for m := 0; m < 12; m++ {
			pet := p.Grid.Fields[fmt.Sprintf("pet-%02d", m)][i]
			total += pet
			if p.Grid.Fields[rainfallMonths[m]][i] < pet*.5 {
				dry++
			}
		}
		if total != annual || dry != p.Grid.Fields["dryMonths"][i] {
			t.Fatal("moisture summary does not match corrected months")
		}
	}
	meta, fields, err := p.Encode()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := model.Decode(meta, fields)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Spec.ClimateVersion != 2 || !reflect.DeepEqual(p.Grid, restored.Grid) {
		t.Fatal("corrected climate did not round-trip")
	}
	for _, pt := range []model.Point{{0, 0}, {math.Pi, .9}, {-2.6384469551633027, .3620194659410114}} {
		bed := p.Surface.At(p.Surface.Bed, pt[0], pt[1])
		if ClimateRegimeAt(p, pt[0], pt[1], bed) != ClimateRegimeAt(restored, pt[0], pt[1], bed) {
			t.Fatal("reopen changed regime")
		}
	}
}

func TestCorrectedThermalTransferAcrossCoasts(t *testing.T) {
	for _, version := range []int{0, 1, 2, 3} {
		p := &model.Planet{Spec: model.DefaultSpec(), Grid: model.Grid{Width: 2, Height: 2, Fields: map[string][]float64{"bed": {-2000, 1000, -2000, 1000}, "climateElevation": {0, 1000, 0, 1000}}}}
		p.Spec.ClimateVersion = version
		// Midway between ocean and highland: interpolated air height is 500 m,
		// whereas interpolated seabed/land height is -500 m.
		got := p.AtmosphericLapse(-math.Pi/2, 0, 100)
		want := 3.9
		if version >= 2 {
			want = -2.6
		}
		if math.Abs(got-want) > 1e-10 {
			t.Fatalf("version %d: got %g want %g", version, got, want)
		}
	}
}
