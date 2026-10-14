package generate

import (
	"kriemhild/internal/terrain/model"
	"testing"
)

func TestBiomeUsesAnnualAridityAndSeasonalFields(t *testing.T) {
	for _, tc := range []struct {
		name                                   string
		temp, rain, pet, soil, dry, warm, cold float64
		want                                   string
	}{
		{"arid remains desert", 25, 150, 1000, 20, 9, 6, 20, "desert"},
		{"semi arid remains steppe", 15, 350, 1000, 30, 6, 4, 100, "steppe"},
		{"humid tropical", 26, 2400, 1600, 100, 1, 0, 0, "rainforest"},
		{"seasonal tropical", 26, 1400, 1400, 70, 6, 6, 0, "savanna"},
		{"maritime", 12, 2000, 900, 100, 0, 0, 900, "temperate-rainforest"},
		{"Mediterranean", 17, 900, 1000, 60, 3, 3, 700, "shrubland"},
		{"saturated lowland", 15, 1500, 1000, 150, 0, 0, 600, "wetland"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &model.Planet{Spec: model.DefaultSpec(), Grid: model.Grid{Width: 2, Height: 2, Fields: map[string][]float64{}}}
			for k, v := range map[string]float64{"temperature": tc.temp, "precipitation": tc.rain, "pet": tc.pet, "soil": tc.soil, "dryMonths": tc.dry, "warmDry": tc.warm, "coldRain": tc.cold, "bed": 100} {
				p.Grid.Fields[k] = []float64{v, v, v, v}
			}
			if got := BiomeAt(p, 0, 0, 100, false); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
			if got := BiomeAt(p, 0, 0, 100, true); got != "water" {
				t.Fatal("water lost precedence")
			}
			p.Spec.Version = "continuous-1"
			if got := BiomeAt(p, 0, 0, 100, false); got != Biome(tc.temp, tc.rain, tc.pet, 100, false) {
				t.Fatal("legacy classification changed")
			}
		})
	}
}
