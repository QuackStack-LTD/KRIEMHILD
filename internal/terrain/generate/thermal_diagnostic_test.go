package generate

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"kriemhild/internal/terrain/model"
)

// A baseline independent of the biome classifier and materialization. These are
// precisely the sea-level climate equations used by versions 1 and 2.
func baselineThermal(lat, bed, coast, tilt float64) (out [12]float64) {
	c := 1 - math.Exp(-math.Max(0, coast)/700)
	for m := range out {
		decl := tilt * math.Pi / 180 * math.Sin(model.Tau*(float64(m)-2)/12)
		out[m] = 29 - 48*math.Pow(math.Sin(lat), 2) - math.Max(0, bed)*.0065 + decl*math.Sin(lat)*(12+26*c)
		out[m] += 8 * (1 - c) * math.Pow(math.Sin(2*lat), 2)
	}
	return
}

func TestThermalBaselineDiagnostic(t *testing.T) {
	type sample struct {
		Latitude, Elevation, CoastKM  float64
		Monthly                       [12]float64
		Mean, Range, Warmest, Coldest float64
		GrowingMonths, FreezingMonths int
	}
	var samples []sample
	for _, degrees := range []float64{-65, -60, -55, -45, -15, 0, 15, 45, 55, 60, 65} {
		for _, coast := range []float64{0, 20, 700, 2000} {
			for _, bed := range []float64{-4000, 0, 100, 2500} {
				s := sample{Latitude: degrees, Elevation: bed, CoastKM: coast, Monthly: baselineThermal(degrees*math.Pi/180, bed, coast, 23.4), Coldest: math.Inf(1), Warmest: math.Inf(-1)}
				for _, temp := range s.Monthly {
					s.Mean += temp / 12
					s.Coldest = math.Min(s.Coldest, temp)
					s.Warmest = math.Max(s.Warmest, temp)
					if temp > 10 {
						s.GrowingMonths++
					}
					if temp < 0 {
						s.FreezingMonths++
					}
				}
				s.Range = s.Warmest - s.Coldest
				samples = append(samples, s)
			}
		}
	}
	if out := os.Getenv("KRIEMHILD_THERMAL_OUT"); out != "" {
		if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
			t.Fatal(err)
		}
		b, _ := json.MarshalIndent(samples, "", "  ")
		if err := os.WriteFile(out, b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Reproduce the failure; do not encode preferred biome percentages. Even a
	// 100-m lowland at 55 degrees has no month warm enough for tree growth.
	for _, coast := range []float64{20, 2000} {
		monthly := baselineThermal(55*math.Pi/180, 100, coast, 23.4)
		hi := math.Inf(-1)
		for _, v := range monthly {
			hi = math.Max(hi, v)
		}
		if hi >= 10 {
			t.Fatalf("baseline changed: coast %g warmest %g", coast, hi)
		}
		t.Logf("baseline 55 degrees, 100 m, coast %g km: warmest %.3f C", coast, hi)
	}
}
