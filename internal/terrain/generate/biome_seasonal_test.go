package generate

import (
	"fmt"
	"kriemhild/internal/terrain/model"
	"math"
	"testing"
)

func seasonalFixture(mean, amplitude, rain, pet, bed float64, mediterranean bool) *model.Planet {
	p := &model.Planet{Spec: model.DefaultSpec(), Grid: model.Grid{Width: 2, Height: 2, Fields: map[string][]float64{}}}
	set := func(k string, v float64) { p.Grid.Fields[k] = []float64{v, v, v, v} }
	set("temperature", mean)
	set("precipitation", rain)
	set("pet", pet)
	set("bed", bed)
	set("soil", 60)
	dry, warm, cold := 0., 0., 0.
	for m := 0; m < 12; m++ {
		season := math.Sin(model.Tau * (float64(m) - 2) / 12)
		temp := mean + amplitude*season
		r := rain / 12
		if mediterranean {
			r *= 1 - .9*season
		}
		set(fmt.Sprintf("temperature-%02d", m), temp)
		set(fmt.Sprintf("rainfall-%02d", m), r)
		set(fmt.Sprintf("pet-%02d", m), pet/12)
		if r < pet/24 {
			dry++
			if temp > 15 {
				warm++
			}
		}
		if temp < 15 {
			cold += r
		}
	}
	set("dryMonths", dry)
	set("warmDry", warm)
	set("coldRain", cold)
	return p
}
func TestSeasonalBiomeControls(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		mean, amplitude, rain, pet, bed float64
		summerDry                       bool
		want                            string
	}{
		{"moderately wet cool forest is not rainforest", 12, 5, 1100, 600, 200, false, "forest"},
		{"wet maritime rainforest", 12, 4, 2200, 900, 200, false, "temperate-rainforest"},
		{"warm annual mean with cool winters is not tropical", 22, 12, 2400, 1400, 200, false, "forest"},
		{"year round tropical rainforest", 26, 2, 2400, 1500, 200, false, "rainforest"},
		{"seasonal tropical woodland", 25, 3, 1400, 1300, 200, false, "tropical-woodland"},
		{"summer drought precedes steppe", 16, 10, 450, 1000, 200, true, "shrubland"},
		{"semiarid continental steppe", 14, 12, 450, 1000, 200, false, "steppe"},
		{"desert", 25, 8, 150, 1400, 200, false, "desert"},
		{"humid grassland", 14, 8, 700, 1000, 200, false, "grassland"},
		{"open woodland", 14, 8, 900, 1000, 200, false, "woodland"},
		{"cold winters but viable boreal summer", -2, 16, 800, 650, 200, false, "taiga"},
		{"negative annual mean does not imply ice", -9, 14, 600, 400, 200, false, "tundra"},
		{"perennial ice", -15, 10, 500, 200, 200, false, "ice"},
		{"cold alpine meadow", 2, 6, 1000, 700, 2400, false, "alpine"},
		{"warm highland can retain montane forest", 18, 4, 1400, 1000, 3700, false, "forest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := seasonalFixture(tc.mean, tc.amplitude, tc.rain, tc.pet, tc.bed, tc.summerDry)
			if got := BiomeAt(p, 0, .5, tc.bed, false); got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
			if got := BiomeAt(p, 0, .5, tc.bed, true); got != "water" {
				t.Fatal("lost water precedence")
			}
		})
	}
}

func TestSeasonalClassifierSpatialContinuity(t *testing.T) {
	p := seasonalFixture(14, 6, 1100, 1000, 200, false)
	for i := 0; i < 100; i++ {
		lon := -math.Pi + model.Tau*float64(i)/100
		if got := BiomeAt(p, lon, .4, 200+float64(i)*.01, false); got != "forest" {
			t.Fatalf("uniform temperate region fragmented: %s", got)
		}
	}
}

func TestMoistureGradientHasConnectedVegetationTransitions(t *testing.T) {
	p := seasonalFixture(14, 6, 1100, 1000, 200, false)
	p.Grid.Width, p.Grid.Height = 64, 4
	for key, a := range p.Grid.Fields {
		value := a[0]
		a = make([]float64, 64*4)
		for i := range a {
			a[i] = value
		}
		p.Grid.Fields[key] = a
	}
	for i := range p.Grid.Fields["precipitation"] {
		rain := 100 + float64(i%64)*20
		p.Grid.Fields["precipitation"][i] = rain
		for m := 0; m < 12; m++ {
			p.Grid.Fields[rainfallMonths[m]][i] = rain / 12
		}
	}
	rank := map[string]int{"desert": 0, "steppe": 1, "grassland": 2, "woodland": 3, "forest": 4}
	previous := -1
	seen := map[string]bool{}
	for i := 0; i < 63*8; i++ {
		lon := -math.Pi + model.Tau*float64(i)/(64*8)
		biome := BiomeAt(p, lon, .4, 200, false)
		current, ok := rank[biome]
		if !ok || current < previous {
			t.Fatalf("smooth moisture gradient fragmented at %d: %s", i, biome)
		}
		previous = current
		seen[biome] = true
	}
	if len(seen) != 5 {
		t.Fatal("lost ecological transitions", seen)
	}
}
