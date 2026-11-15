package generate

import (
	"context"
	"fmt"
	"kriemhild/internal/terrain/model"
	"math"
	"reflect"
	"testing"
)

func versionFourFixture(mean, amplitude, rain, coast float64, summerDry bool) *model.Planet {
	p := seasonalFixture(mean, amplitude, rain, 1200, 200, summerDry)
	p.Spec.ClimateVersion = 4
	p.Grid.Fields["climateElevation"] = []float64{200, 200, 200, 200}
	p.Grid.Fields["coast"] = []float64{coast, coast, coast, coast}
	return p
}

func TestVersionFourMonthlyVegetation(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		mean, amplitude, rain, coast float64
		seasonal                     bool
		want                         string
	}{
		{"perennial tropical", 26, 2, 2400, 100, false, "rainforest"},
		{"same annual rain with drought", 26, 2, 2400, 100, true, "tropical-woodland"},
		{"water limited tropical", 26, 2, 1100, 100, false, "savanna"},
		{"warm annual mean cool winter", 22, 12, 2400, 100, false, "forest"},
		{"ordinary temperate forest", 14, 8, 1400, 500, false, "forest"},
		{"moderately dry temperate woodland", 14, 8, 1100, 500, false, "woodland"},
		{"temperate grassland", 14, 8, 850, 500, false, "grassland"},
		{"temperate steppe", 14, 14, 450, 1500, false, "temperate-steppe"},
		{"subtropical steppe", 20, 10, 500, 1500, false, "subtropical-steppe"},
		{"subtropical summer dry steppe", 20, 8, 500, 1500, true, "subtropical-steppe"},
		{"tropical steppe", 26, 2, 700, 1500, false, "tropical-steppe"},
		{"wet ocean facing rainforest", 12, 4, 3000, 20, false, "temperate-rainforest"},
		{"equally wet continental forest", 12, 4, 3000, 1500, false, "forest"},
		{"ordinary coastal rain is not rainforest", 12, 4, 2000, 20, false, "forest"},
		{"Mediterranean", 16, 10, 900, 100, true, "shrubland"},
		{"taiga", -2, 16, 1100, 1500, false, "taiga"},
		{"severe winter warm summer taiga", -4, 25, 1100, 1500, false, "taiga"},
		{"tundra", -9, 14, 800, 1500, false, "tundra"},
		{"ice", -15, 10, 600, 1500, false, "ice"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := versionFourFixture(tc.mean, tc.amplitude, tc.rain, tc.coast, tc.seasonal)
			c := ClimateRegimeAt(p, 0, .5, 200)
			if got := BiomeAt(p, 0, .5, 200, false); got != tc.want {
				t.Fatalf("got %s want %s; %+v", got, tc.want, c)
			}
			if BiomeAt(p, 0, .5, 200, true) != "water" {
				t.Fatal("lost water precedence")
			}
			before := c
			summarizeSeasons(&p.Grid)
			if c = ClimateRegimeAt(p, 0, .5, 200); before != c {
				t.Fatal("cached extrema changed monthly regime")
			}
		})
	}
}

func TestVersionFourDrySeasonWrapAndHemispheres(t *testing.T) {
	p := versionFourFixture(26, 2, 2400, 100, false)
	for m, key := range rainfallMonths {
		rain := 295.
		if m == 10 || m == 11 || m == 0 || m == 1 {
			rain = 10
		}
		for i := range p.Grid.Fields[key] {
			p.Grid.Fields[key][i] = rain
		}
	}
	c := ClimateRegimeAt(p, 0, .5, 200)
	if c.LongestDrySeason != 4 || c.DryMonths != 4 || BiomeAt(p, 0, .5, 200, false) != "tropical-woodland" {
		t.Fatalf("December wrap %+v", c)
	}
	for m, key := range rainfallMonths {
		rain := 390.
		if m < 6 {
			rain = 10
		}
		for i := range p.Grid.Fields[key] {
			p.Grid.Fields[key][i] = rain
		}
	}
	if c := ClimateRegimeAt(p, 0, .5, 200); c.LongestDrySeason != 6 || c.Aridity < 1 || BiomeAt(p, 0, .5, 200, false) != "tropical-woodland" {
		t.Fatal("wet seasonal forest incorrectly became savanna", c)
	}
	north := versionFourFixture(16, 10, 900, 100, true)
	south := versionFourFixture(16, 10, 900, 100, true)
	for _, keys := range [][12]string{temperatureMonths, rainfallMonths} {
		for m, key := range keys {
			south.Grid.Fields[key] = append([]float64(nil), north.Grid.Fields[keys[(m+6)%12]]...)
		}
	}
	if BiomeAt(north, 0, .5, 200, false) != "shrubland" || BiomeAt(south, 0, -.5, 200, false) != "shrubland" {
		t.Fatal("seasonal hemisphere reversal")
	}
}

func TestVersionFourCirculationAndNoBiomeFeedback(t *testing.T) {
	a, b := atmosphericCoastFixture(100, 4), atmosphericCoastFixture(4000, 4)
	a.Diagnostics.BiomeShares = map[string]float64{"rainforest": .99}
	b.Diagnostics.BiomeShares = map[string]float64{"desert": .99}
	for _, p := range []*model.Planet{a, b} {
		if err := climate(context.Background(), p); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range rainfallMonths {
		if !reflect.DeepEqual(a.Grid.Fields[key], b.Grid.Fields[key]) {
			t.Fatal("seabed or biome diagnostic altered precipitation")
		}
	}
	for _, tc := range []struct {
		row      int
		positive bool
	}{{9, false}, {6, true}, {1, false}} {
		v := a.Grid.Fields["windY"][tc.row*a.Grid.Width+2]
		if (v > 0) != tc.positive {
			t.Fatalf("incorrect surface circulation at row %d: %g", tc.row, v)
		}
	}
	windward, lee := 0., 0.
	for _, y := range []int{5, 6, 7, 16, 17, 18} {
		for x := 20; x <= 24; x++ {
			windward += a.Grid.Fields["precipitation"][y*a.Grid.Width+x]
		}
		for x := 28; x <= 32; x++ {
			lee += a.Grid.Fields["precipitation"][y*a.Grid.Width+x]
		}
	}
	if windward <= lee {
		t.Fatal("lost mountain rain shadow", windward, lee)
	}
	if a.Diagnostics.MoistureBudgetError > 1e-9 {
		t.Fatal("atmospheric conservation")
	}
}

func TestVersionFourEvaporationUsesSolarEnergyAndActualElevation(t *testing.T) {
	if potentialEvaporationV4(10, 85*math.Pi/180, -23.4*math.Pi/180, 1000) != 0 {
		t.Fatal("polar night evaporation demand")
	}
	p := versionFourFixture(-30, 0, 600, 1000, false)
	c := ClimateRegimeAt(p, 0, 1.4, 200)
	if !model.Finite(c.MinimumMonthlyBalance+c.PET) || c.PET != 0 || c.Thermal != "ice" {
		t.Fatal("frozen climate has invalid water balance", c)
	}
	p = versionFourFixture(20, 8, 1400, 1000, false)
	low, high := ClimateRegimeAt(p, 0, .6, 200), ClimateRegimeAt(p, 0, .6, 1200)
	if math.Abs(low.MeanTemperature-high.MeanTemperature-6.5) > 1e-10 || high.PET >= low.PET {
		t.Fatal("elevation did not change thermal demand consistently")
	}
	for m := 0; m < 12; m++ {
		temp, rain, pet := MonthlyClimateAt(p, 0, .6, 1200, m)
		if !model.Finite(temp+rain+pet) || rain < 0 || pet < 0 {
			t.Fatal("invalid monthly detail")
		}
	}
}

func TestVersionFourThermalContinuityAndContinentality(t *testing.T) {
	for _, latitude := range []float64{0, 15, 35, 45, 55, 65, 85} {
		lat := latitude * math.Pi / 180
		if math.Abs(annualThermalV4(42, math.Pi, lat, 100)-annualThermalV4(42, -math.Pi, lat, 100)) > 1e-12 {
			t.Fatal("longitude seam")
		}
		a, b := annualThermalV4(42, .4, lat, 100), annualThermalV4(42, .40001, lat, 100)
		if math.Abs(a-b) > .001 {
			t.Fatal("regional thermal discontinuity")
		}
		if latitude == 0 {
			continue
		}
		coast, inland := seasonalThermalAnomaly(lat, 20, 100, 23.4), seasonalThermalAnomaly(lat, 2000, 100, 23.4)
		rangeOf := func(a [12]float64) float64 {
			lo, hi := math.Inf(1), math.Inf(-1)
			for _, v := range a {
				lo = math.Min(lo, v)
				hi = math.Max(hi, v)
			}
			return hi - lo
		}
		if rangeOf(inland) <= rangeOf(coast) {
			t.Fatal("continental thermal range")
		}
	}
}

func TestVersionFourGeneratedClimateAndFixedTerrain(t *testing.T) {
	for _, seed := range []string{"Kriemhild", "gallery-001", "gallery-002"} {
		spec := model.DefaultSpec()
		spec.ClimateVersion = 4
		spec.SurfaceWidth = 1024
		spec.Seed = seed
		p, err := Generate(context.Background(), spec, nil)
		if err != nil {
			t.Fatal(err)
		}
		if p.Diagnostics.MoistureBudgetError > 1e-9 {
			t.Fatal("water conservation", p.Diagnostics.MoistureBudgetError)
		}
		for i, bed := range p.Grid.Fields["bed"] {
			if !p.Emerged(i) {
				continue
			}
			pt := p.Grid.Point(i%p.Grid.Width, i/p.Grid.Width)
			c := ClimateRegimeAt(p, pt[0], pt[1], bed)
			biome := BiomeAt(p, pt[0], pt[1], bed, false)
			if biome == "rainforest" && !TropicalRainforestEligible(c) {
				t.Fatal("ineligible rainforest")
			}
			if biome == "rainforest" {
				// Independent checks against persisted monthly fields, rather
				// than only invoking the predicate used by classification.
				rain, pet, cold, driest := 0., 0., math.Inf(1), math.Inf(1)
				for m := 0; m < 12; m++ {
					r, e := p.Grid.Fields[rainfallMonths[m]][i], p.Grid.Fields[fmt.Sprintf("pet-%02d", m)][i]
					rain += r
					pet += e
					cold = math.Min(cold, p.Grid.Fields[temperatureMonths[m]][i])
					driest = math.Min(driest, r)
					if r+1e-8 < .65*e {
						t.Fatal("rainforest has a monthly water deficit")
					}
				}
				if cold < 18-1e-8 || driest < 60-1e-8 || rain < 1800-1e-8 || rain/math.Max(100, pet) < 1.15-1e-8 {
					t.Fatal("monthly rainforest invariants failed")
				}
			}
			if biome == "temperate-rainforest" && !TemperateRainforestEligible(c) {
				t.Fatal("ineligible temperate rainforest")
			}
			if biome == "savanna" && !SavannaEligible(c) {
				t.Fatal("ineligible savanna")
			}
			if math.Abs(c.Rain-p.Grid.Fields["precipitation"][i]) > 1e-7 || math.Abs(c.PET-p.Grid.Fields["pet"][i]) > 1e-7 {
				t.Fatal("monthly summary disagreement")
			}
		}
		for _, point := range []model.Point{{0, 0}, {1, .7}, {-2, -.8}} {
			bed := p.Surface.At(p.Surface.Bed, point[0], point[1])
			c := ClimateRegimeAt(p, point[0], point[1], bed)
			biome := BiomeAt(p, point[0], point[1], bed, false)
			if biome == "rainforest" && !TropicalRainforestEligible(c) {
				t.Fatal("detail eligibility")
			}
		}
		m, f, err := p.Encode()
		if err != nil {
			t.Fatal(err)
		}
		q, err := model.Decode(m, f)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(p.Grid, q.Grid) || q.Spec.ClimateVersion != 4 || !reflect.DeepEqual(p.Diagnostics.ClimateBands, q.Diagnostics.ClimateBands) {
			t.Fatal("v4 cold restore")
		}
		fixed, err := ClimateOnFixedTerrain(context.Background(), p, 1)
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"bed", "coast", "lake", "wetland", "flow", "drainage", "discharge"} {
			if !reflect.DeepEqual(p.Grid.Fields[key], fixed.Grid.Fields[key]) {
				t.Fatal("fixed comparison altered geography", key)
			}
		}
		if fixed.Surface != p.Surface || !reflect.DeepEqual(fixed.Rivers, p.Rivers) {
			t.Fatal("fixed geography changed")
		}
		if p.Spec.ClimateVersion != 4 {
			t.Fatal("comparison mutated source")
		}
		if !reflect.DeepEqual(p.Grid, q.Grid) {
			t.Fatal("comparison mutated source monthly fields")
		}
	}
}
