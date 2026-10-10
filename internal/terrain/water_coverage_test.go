package terrain

import (
	"math"
	"reflect"
	"testing"
)

func TestWaterCoverageAcrossLandmasses(t *testing.T) {
	for _, seed := range []string{"KRIEMHILD", "mountain-drainage", "island-arc", "river-basin", "sharp-overview"} {
		t.Run(seed, func(t *testing.T) {
			o := realismOptions(t, `{"columns":160,"rows":100}`)
			o.Seed = seed
			e := BuildEnvironment(o)
			labels, sizes := e.hydroLandmasses()
			stats := e.Hydrology.Statistics
			// Guard against the previous globally sparse lakes, groundwater and
			// wetlands, while keeping most of the land free of visible channels.
			if stats.Lakes < int(float64(stats.LandCells)*.002) || stats.Wetlands < int(float64(stats.LandCells)*.025) || stats.Springs < int(float64(stats.LandCells)*.01) || stats.CoveragePercent < 7 || stats.CoveragePercent > 25 {
				t.Fatalf("incomplete water coverage: %+v", stats)
			}
			for id, size := range sizes {
				if id == 0 || size < 80 {
					continue
				}
				features := 0
				runoff, temp := 0., 0.
				for i, l := range labels {
					if l != id {
						continue
					}
					runoff += e.get("runoff", i)
					temp += e.get("temperature", i)
					if e.get("river", i) > 0 || e.get("lake", i) > 0 || e.get("spring", i) > 0 || e.get("wetland", i) >= .6 {
						features++
					}
				}
				if runoff/float64(size) > .08 && temp/float64(size) > -5 && features == 0 {
					t.Fatalf("supplied landmass %d (%d cells) has no freshwater features", id, size)
				}
			}
			if d := e.ValidateHydrology(); len(d) > 0 {
				t.Fatal(d[:min(5, len(d))])
			}
			t.Logf("lakes=%d wetlands=%d springs=%d river coverage=%.1f%%", stats.Lakes, stats.Wetlands, stats.Springs, stats.CoveragePercent)
		})
	}
}

func TestStorageLandformsPreserveMountainsCoastsAndAuthoredHeights(t *testing.T) {
	o := realismOptions(t, `{"columns":96,"rows":64,"seed":"river-basin"}`)
	e := BuildEnvironment(o)
	before := append([]float64{}, e.Fields["elevation"]...)
	mask := append([]int{}, e.Mask...)
	e.carveStorageBasins()
	changed := 0
	for i, z := range before {
		if z != e.get("elevation", i) {
			changed++
		}
		if (e.get("mountainCore", i) > .22 || e.get("ocean", i) > 0 || e.get("oceanDistance", i) < 2) && z != e.get("elevation", i) {
			t.Fatal("storage changed a mountain or coast", i)
		}
		if math.Abs(z-e.get("elevation", i)) > 80 {
			t.Fatal("storage made an abrupt deep hole", i)
		}
	}
	if changed == 0 || !reflect.DeepEqual(mask, e.Mask) {
		t.Fatal("storage must refine relief without inventing water")
	}
	e.Options.Heights = make([]float64, len(e.Mask))
	before = append([]float64{}, e.Fields["elevation"]...)
	if e.carveStorageBasins() || !reflect.DeepEqual(before, e.Fields["elevation"]) {
		t.Fatal("authored terrain changed")
	}
}

func TestWaterCoverageIsDeterministic(t *testing.T) {
	o := realismOptions(t, `{"columns":96,"rows":64,"seed":"water-storage"}`)
	a, b := BuildEnvironment(o), BuildEnvironment(o)
	if !reflect.DeepEqual(a.Fields, b.Fields) || !reflect.DeepEqual(a.Hydrology, b.Hydrology) {
		t.Fatal("water coverage depends on generation order")
	}
}
