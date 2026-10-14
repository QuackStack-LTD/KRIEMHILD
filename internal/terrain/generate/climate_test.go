package generate

import (
	"context"
	"fmt"
	"kriemhild/internal/terrain/model"
	"math"
	"testing"
)

func TestMonthlyClimateBudgetAndFields(t *testing.T) {
	for _, seed := range []string{"Kriemhild", "gallery-001", "gallery-002"} {
		spec := model.DefaultSpec()
		spec.Seed = seed
		p, err := Generate(context.Background(), spec, nil)
		if err != nil {
			t.Fatal(err)
		}
		if p.Diagnostics.MoistureBudgetError > 1e-9 {
			t.Fatalf("%s moisture budget: %g", seed, p.Diagnostics.MoistureBudgetError)
		}
		for i := range p.Grid.Fields["bed"] {
			rain, pet, temp := 0., 0., 0.
			for m := 0; m < 12; m++ {
				r := p.Grid.Fields[fmt.Sprintf("rainfall-%02d", m)][i]
				e := p.Grid.Fields[fmt.Sprintf("pet-%02d", m)][i]
				v := p.Grid.Fields[fmt.Sprintf("temperature-%02d", m)][i]
				if r < 0 || e < 0 || !model.Finite(r+e+v) {
					t.Fatal("invalid monthly climate")
				}
				rain += r
				pet += e
				temp += v / 12
			}
			if math.Abs(rain-p.Grid.Fields["precipitation"][i]) > 1e-8 || math.Abs(pet-p.Grid.Fields["pet"][i]) > 1e-8 || math.Abs(temp-p.Grid.Fields["temperature"][i]) > 1e-8 {
				t.Fatal("annual/monthly mismatch")
			}
		}
		if p.Diagnostics.BiomeShares["desert"] > .45 {
			t.Fatalf("systematic desert dominance in %s", seed)
		}
	}
}

func TestMoistureTransportProducesRainShadow(t *testing.T) {
	g := model.Grid{Width: 96, Height: 48, Fields: map[string][]float64{}}
	for _, key := range []string{"bed", "coast", "windX", "windY", "temperature", "precipitation", "pet", "moisture"} {
		g.Fields[key] = make([]float64, 96*48)
	}
	for i := range g.Fields["bed"] {
		x := i % 96
		g.Fields["bed"][i] = -100
		if x >= 20 && x <= 70 {
			g.Fields["bed"][i] = 100
			g.Fields["coast"][i] = 500
		}
		if x >= 36 && x <= 38 {
			g.Fields["bed"][i] = 2400
		}
	}
	p := &model.Planet{Spec: model.DefaultSpec(), Grid: g}
	if err := climate(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	windward, lee := 0., 0.
	for y := 10; y <= 14; y++ {
		windward += g.Fields["precipitation"][y*96+28]
		lee += g.Fields["precipitation"][y*96+48]
	}
	if windward <= lee*1.5 {
		t.Fatalf("no rain shadow: windward %f leeward %f", windward, lee)
	}
	if p.Diagnostics.MoistureBudgetError > 1e-9 {
		t.Fatal("fixture loses water mass")
	}
}
