package generate

import (
	"context"
	"encoding/json"
	"kriemhild/internal/terrain/model"
	"math"
	"os"
	"testing"
)

func TestSeasonalThermalPhysicalRelationships(t *testing.T) {
	var diagnostics []map[string]any
	for _, latitude := range []float64{0, 15, 35, 45, 55, 60, 65, 85} {
		lat := latitude * math.Pi / 180
		var ranges []float64
		for _, coast := range []float64{20, 700, 2000} {
			candidate := seasonalThermalAnomaly(lat, coast, 100, 23.4)
			baseline := baselineThermal(lat, 100, coast, 23.4)
			mean := 0.
			for _, v := range baseline {
				mean += v / 12
			}
			lo, hi, total := math.Inf(1), math.Inf(-1), 0.
			south := seasonalThermalAnomaly(-lat, coast, 100, 23.4)
			for m, v := range candidate {
				total += v
				if math.Abs(v-south[(m+6)%12]) > 1e-9 {
					t.Fatal("hemisphere reversal")
				}
				lo = math.Min(lo, mean+v)
				hi = math.Max(hi, mean+v)
				candidate[m] += mean
			}
			if math.Abs(total) > 1e-9 {
				t.Fatal("seasonal anomaly altered annual mean")
			}
			ranges = append(ranges, hi-lo)
			diagnostics = append(diagnostics, map[string]any{"latitude": latitude, "coastKM": coast, "bed": 100, "monthly": candidate, "baselineMonthly": baseline, "annual": mean, "coldest": lo, "warmest": hi})
			// The chosen Earth-like lowland cases must have a viable growing season,
			// without prescribing a biome, global share or polar forest outcome.
			if latitude == 55 && hi < 10 {
				t.Errorf("55-degree lowland still has no growing season: coast %g, max %g", coast, hi)
			}
			if latitude <= 60 && hi-lo > 65 {
				t.Errorf("unbounded seasonal range %g", hi-lo)
			}
		}
		if ranges[0] >= ranges[2] {
			t.Errorf("maritime range must be smaller at %g", latitude)
		}
	}
	for _, coast := range []float64{20, 2000} {
		zero := seasonalThermalAnomaly(.7, coast, 100, 0)
		for _, v := range zero {
			if math.Abs(v) > 1e-10 {
				t.Fatal("zero tilt has seasonal forcing")
			}
		}
	}
	for _, coast := range []float64{-500, -.001, 0, .001, 500} {
		a := seasonalThermalAnomaly(.9, coast, -4000, 23.4)
		b := seasonalThermalAnomaly(.9, coast, 0, 23.4)
		if a != b {
			t.Fatal("seabed depth affected atmospheric seasonality")
		}
		near := seasonalThermalAnomaly(.9, coast+.001, 0, 23.4)
		for m := range a {
			if math.Abs(a[m]-near[m]) > .002 {
				t.Fatal("coastal thermal discontinuity")
			}
		}
	}
	if dailyInsolation(85*math.Pi/180, -23.4*math.Pi/180) > 1e-9 {
		t.Fatal("polar night")
	}
	if dailyInsolation(85*math.Pi/180, 23.4*math.Pi/180) <= 0 {
		t.Fatal("midnight sun")
	}
	if path := os.Getenv("KRIEMHILD_THERMAL_CANDIDATE_OUT"); path != "" {
		b, _ := json.MarshalIndent(diagnostics, "", "  ")
		if err := os.WriteFile(path, b, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHistoricalThermalEquationsAreFrozen(t *testing.T) {
	for _, version := range []int{0, 1, 2} {
		p := atmosphericCoastFixture(4000, version)
		if err := climate(context.Background(), p); err != nil {
			t.Fatal(err)
		}
		for i, bed := range p.Grid.Fields["bed"] {
			lat := p.Grid.Point(i%p.Grid.Width, i/p.Grid.Width)[1]
			expected := baselineThermal(lat, bed, p.Grid.Fields["coast"][i], p.Spec.Tilt)
			for m, key := range temperatureMonths {
				want := expected[m]
				if version == 0 {
					c := 1 - math.Exp(-math.Max(0, p.Grid.Fields["coast"][i])/700)
					want -= 8 * (1 - c) * math.Pow(math.Sin(2*lat), 2)
				}
				if math.Abs(want-p.Grid.Fields[key][i]) > 1e-12 {
					t.Fatalf("version %d cell %d month %d changed", version, i, m)
				}
			}
		}
	}
}

func TestExperimentalThermalMonthlyFieldsPersist(t *testing.T) {
	spec := model.DefaultSpec()
	spec.ClimateVersion = 3
	spec.SurfaceWidth = 1024
	p, err := Generate(context.Background(), spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	meta, fields, err := p.Encode()
	if err != nil {
		t.Fatal(err)
	}
	q, err := model.Decode(meta, fields)
	if err != nil {
		t.Fatal(err)
	}
	if q.Spec.ClimateVersion != 3 || len(q.Diagnostics.ClimateBudget) != 12 {
		t.Fatal("candidate metadata lost")
	}
	for key, field := range p.Grid.Fields {
		for i, v := range field {
			if v != q.Grid.Fields[key][i] {
				t.Fatal("monthly/summary field changed on cold decode", key)
			}
		}
	}
	for _, point := range []model.Point{{0, 0}, {1, .9}, {-2, -.9}} {
		bed := p.Surface.At(p.Surface.Bed, point[0], point[1])
		if ClimateRegimeAt(p, point[0], point[1], bed) != ClimateRegimeAt(q, point[0], point[1], bed) {
			t.Fatal("cold restore changed vegetation inputs")
		}
	}
}
