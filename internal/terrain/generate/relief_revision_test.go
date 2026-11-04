package generate

import (
	"context"
	"kriemhild/internal/terrain/model"
	"testing"
)

func TestContinentalOrogensIncreaseConnectedHighRelief(t *testing.T) {
	for _, seed := range []string{"orogen-1", "orogen-2", "orogen-3"} {
		measurements := [][3]float64{}
		for _, version := range []int{1, 2} {
			spec := model.DefaultSpec()
			spec.Seed = seed
			spec.ReliefVersion = version
			p := &model.Planet{Spec: spec, Grid: model.Grid{Width: 192, Height: 96, Fields: map[string][]float64{}}}
			layout(p)
			plates(p)
			if err := surface(context.Background(), p); err != nil {
				t.Fatal(err)
			}
			land, high, peak := 0., 0., 0.
			for i, bed := range p.Grid.Fields["bed"] {
				if p.Emerged(i) {
					land++
					if bed > 2500 {
						high++
					}
					peak = max(peak, bed)
				}
			}
			measurements = append(measurements, [3]float64{high / land, peak, float64(len(p.Plates))})
		}
		t.Log(seed, "previous/new highland share, peak, plates", measurements)
		if measurements[1][0] <= measurements[0][0] || measurements[1][1] <= measurements[0][1] || measurements[1][1] < 4500 {
			t.Fatal("continental collision uplift did not increase mountain extent and peaks")
		}
	}
}

func TestRequestedContinentsAreUsedByEveryNewPreset(t *testing.T) {
	for _, preset := range []string{"earthlike", "archipelago", "supercontinent"} {
		for _, n := range []int{1, 4, 9} {
			spec := model.DefaultSpec()
			spec.Preset = preset
			spec.Continents = n
			p := &model.Planet{Spec: spec}
			layout(p)
			// Island systems are additional masses, with explicitly different IDs.
			count := 0
			for _, m := range p.Masses {
				if len(m.ID) >= 10 && m.ID[:10] == "continent-" {
					count++
				}
			}
			if count != n {
				t.Fatalf("%s requested %d continental nuclei, got %d", preset, n, count)
			}
		}
	}
}
