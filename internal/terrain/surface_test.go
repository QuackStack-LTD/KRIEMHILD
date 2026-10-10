package terrain

import (
	"math"
	"testing"
)

func basinFixture(t *testing.T, wet, channel bool) *Environment {
	o := realismOptions(t, `{"columns":64,"rows":48,"latitudeNorth":35,"latitudeSouth":15,"windDirection":1,"rainfall":0.1,"temperatureOffset":15}`)
	if wet {
		o.Rainfall = 3
		o.TemperatureOffset = 0
		o.LatitudeNorth = 10
		o.LatitudeSouth = -10
		o.ClimateAdjustments = make([]float64, o.Columns*o.Rows)
		for i := range o.ClimateAdjustments {
			o.ClimateAdjustments[i] = 5
		}
	}
	o.Heights = make([]float64, o.Columns*o.Rows)
	for i := range o.Heights {
		x, y := i%64, i/64
		z := -500.
		if x > 5 && x < 59 && y > 5 && y < 43 {
			z = 450.
			r := math.Hypot(float64(x-32), float64(y-24))
			if r < 12 {
				z = -320 + 770*r*r/144
			}
		}
		if channel && y == 24 && x <= 32 {
			z = -350
		}
		o.Heights[i] = z / 4
	}
	return BuildEnvironment(o)
}

func TestDryDepressionAndLocalLakeLevels(t *testing.T) {
	dry := basinFixture(t, false, false)
	wet := basinFixture(t, true, false)
	open := basinFixture(t, false, true)
	i := 24*64 + 32
	if dry.Mask[i] != 1 || dry.get("elevation", i) >= 0 || dry.get("waterDepth", i) != 0 {
		t.Fatalf("closed dry basin flooded: z=%f depth=%f mask=%d", dry.get("elevation", i), dry.get("waterDepth", i), dry.Mask[i])
	}
	if wet.get("lake", i) != 1 || wet.get("waterDepth", i) <= 0 {
		t.Fatalf("wet catchment did not make a lake: depth=%f basin=%f rain=%f temp=%f runoff=%f drainage=%f", wet.get("waterDepth", i), wet.get("basin", i), wet.get("precipitation", i), wet.get("temperature", i), wet.get("accumulation", i), wet.get("drainageElevation", i))
	}
	if open.get("ocean", i) != 1 || open.get("waterLevel", i) != 0 {
		t.Fatal("sea-connected depression did not flood to sea level")
	}
	if wet.get("waterLevel", i) == 0 {
		t.Fatal("lake incorrectly forced to global sea level")
	}
	body := wet.WaterBodies[int(wet.get("waterBody", i))-1]
	depths := map[int]bool{}
	for _, j := range body.Cells {
		if math.Abs(wet.get("waterLevel", j)-body.Level) > .001 {
			t.Fatal("lake surface is not level")
		}
		depths[int(wet.get("waterDepth", j))] = true
	}
	if len(depths) < 10 {
		t.Fatal("lake has no shallow/deep variation")
	}
	t.Logf("dry floor %.0fm; lake surface %.0fm; lake depth %.0fm; depth levels %d", dry.get("elevation", i), body.Level, body.MaxDepth, len(depths))
}

func TestContinuousSurfaceAndHydrology(t *testing.T) {
	for _, realism := range []bool{false, true} {
		o := realismOptions(t, `{"columns":160,"rows":100,"seed":"KRIEMHILD"}`)
		o.Realism = realism
		e := BuildEnvironment(o)
		f := e.get
		lowland := map[int]bool{}
		depths := map[int]bool{}
		dryBelow, lakes, shelves, deep := 0, 0, 0, 0
		runoff, sinks := 0., 0.
		for i := range e.Mask {
			z, d := f("elevation", i), f("waterDepth", i)
			if e.Mask[i] != 0 {
				if z < 1800 {
					lowland[int(z)] = true
				}
				if z < 0 {
					dryBelow++
				}
				if d != 0 || f("waterBody", i) != 0 {
					t.Fatal("dry cell has water")
				}
				runoff += f("runoff", i)
			} else {
				if math.Abs(d-(f("waterLevel", i)-z)) > .01 {
					t.Fatal("inconsistent water depth")
				}
				depths[int(d)] = true
				if f("lake", i) > 0 {
					lakes++
				}
				if d < 200 {
					shelves++
				}
				if d > 3000 {
					deep++
				}
			}
			if e.Mask[i] == 0 {
				runoff += f("runoff", i)
			}
			if f("flow", i) >= 0 {
				sinks += math.Min(f("hydroLoss", i), f("accumulation", i))
			}
			if f("flow", i) < 0 {
				sinks += f("accumulation", i)
			}
			for name, field := range e.Fields {
				if !finite(field[i]) {
					t.Fatal("nonfinite", name, i)
				}
			}
			// Every drainage chain must terminate, including dry endorheic basins.
			at := i
			for steps := 0; at >= 0; steps++ {
				if steps >= len(e.Mask) {
					t.Fatal("drainage cycle")
				}
				at = int(f("flow", at))
			}
		}
		if math.Abs(runoff-sinks) > runoff*.0001 {
			t.Fatalf("runoff lost/cycled: %.3f -> %.3f", runoff, sinks)
		}
		if len(lowland) < 100 || len(depths) < 100 || shelves < 20 || deep < 20 || lakes == 0 {
			t.Fatalf("missing continuous relief lowland=%d depths=%d shelf=%d deep=%d dry=%d lake=%d", len(lowland), len(depths), shelves, deep, dryBelow, lakes)
		}
		t.Logf("realism=%v lowland elevations=%d water depths=%d dry below sea=%d lake cells=%d", realism, len(lowland), len(depths), dryBelow, lakes)
	}
}

func TestReefHabitatAndForms(t *testing.T) {
	forms := map[int]int{}
	totalReef, totalOcean := 0, 0
	for _, seed := range []string{"KRIEMHILD", "reef-islands", "atoll", "1", "2"} {
		o := realismOptions(t, `{"columns":160,"rows":100,"latitudeNorth":25,"latitudeSouth":-25,"landPercent":25,"continentCount":40,"plateCount":16}`)
		o.Seed = seed
		e := BuildEnvironment(o)
		f := e.get
		reefs, ocean := 0, 0
		for i, v := range e.Fields["reefType"] {
			if f("ocean", i) == 1 {
				ocean++
			}
			if v == 0 {
				continue
			}
			forms[int(v)]++
			reefs++
			if f("ocean", i) != 1 || f("waterDepth", i) < 2 || f("waterDepth", i) > 70 || f("temperature", i) < 20 || f("temperature", i) > 31 || f("light", i) < .22 || f("clarity", i) < .6 || f("salinity", i) < 30 {
				t.Fatal("invalid reef habitat")
			}
			if int(v) == 4 && f("seamount", i) <= .55 {
				t.Fatal("atoll lacks volcanic rim")
			}
			if choices := e.Candidates(i); len(choices) != 1 || choices[0] != "reef" {
				t.Fatal("reef geometry randomized away by WFC")
			}
		}
		if reefs*20 > ocean {
			t.Fatalf("excessive reef coverage for %s: %d/%d", seed, reefs, ocean)
		}
		totalReef += reefs
		totalOcean += ocean
	}
	if totalReef*100 > totalOcean*3 {
		t.Fatalf("reefs exceed 3%% of tropical ocean: %d/%d", totalReef, totalOcean)
	}
	for k := 1; k <= 4; k++ {
		if forms[k] == 0 {
			t.Fatalf("missing reef form %d: %v", k, forms)
		}
	}
	t.Logf("reef cells by form: %v", forms)
}
