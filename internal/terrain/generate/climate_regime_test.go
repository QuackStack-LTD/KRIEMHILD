package generate

import (
	"math/rand"
	"testing"
)

func TestClimateRegimeSeparationPreservesVersionOne(t *testing.T) {
	r := rand.New(rand.NewSource(328))
	for i := 0; i < 2000; i++ {
		bed := r.Float64() * 4500
		p := seasonalFixture(-20+r.Float64()*55, r.Float64()*25, r.Float64()*4000, 100+r.Float64()*2000, bed, i%3 == 0)
		p.Grid.Fields["wetland"] = []float64{float64(i % 2), float64(i % 2), float64(i % 2), float64(i % 2)}
		if i%2 == 0 {
			summarizeSeasons(&p.Grid)
		}
		for _, lat := range []float64{-.5, .5} {
			for _, wet := range []bool{false, true} {
				a, b := seasonalBiomeBeforeRegimes(p, .1, lat, bed, wet), seasonalBiome(p, .1, lat, bed, wet)
				if a != b {
					t.Fatalf("case %d: old %s new %s, regime %+v", i, a, b, ClimateRegimeAt(p, .1, lat, bed))
				}
			}
		}
	}
}
func TestClimateRegimesHaveExplicitPhysicalAxes(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		mean, amp, rain, pet       float64
		summerDry                  bool
		thermal, moisture, pattern string
	}{
		{"continental", 14, 12, 1100, 1000, false, "temperate", "humid", "seasonal"},
		{"Mediterranean", 16, 10, 450, 1000, true, "temperate", "semi-arid", "summer-dry"},
		{"steppe", 14, 12, 450, 1000, false, "temperate", "semi-arid", "seasonal"},
		{"desert", 25, 8, 150, 1400, false, "temperate", "arid", "seasonal"},
		{"maritime", 12, 4, 2200, 900, false, "temperate", "humid", "perennial-wet"},
		{"tropical", 26, 2, 2400, 1500, false, "tropical", "humid", "perennial-wet"},
		{"boreal", -2, 16, 800, 650, false, "boreal", "humid", "seasonal"},
		{"tundra", -9, 14, 600, 400, false, "tundra", "humid", "seasonal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := seasonalFixture(tc.mean, tc.amp, tc.rain, tc.pet, 200, tc.summerDry)
			c := ClimateRegimeAt(p, 0, .5, 200)
			if c.Thermal != tc.thermal || c.Moisture != tc.moisture || c.Rainfall != tc.pattern {
				t.Fatalf("%+v", c)
			}
		})
	}
}

func BenchmarkSeasonalVegetationStages(b *testing.B) {
	for _, mean := range []float64{-30, 14, 26} {
		p := seasonalFixture(mean, 8, 2000, 1000, 200, false)
		summarizeSeasons(&p.Grid)
		name := map[float64]string{-30: "cold", 14: "temperate", 26: "tropical"}[mean]
		b.Run(name+"/before", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				seasonalBiomeBeforeRegimes(p, .1, .5, 200, false)
			}
		})
		b.Run(name+"/regimes", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				seasonalBiome(p, .1, .5, 200, false)
			}
		})
	}
}
