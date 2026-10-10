package terrain

import "testing"

func TestOffshoreMarginsDoNotStampLongRectangularCoasts(t *testing.T) {
	for _, seed := range []string{"KRIEMHILD", "island-arc", "river-basin"} {
		o := realismOptions(t, `{"columns":128,"rows":96,"landPercent":55}`)
		o.Seed = seed
		e := BuildEnvironment(o)
		maxRun := 0
		for side := 0; side < 4; side++ {
			length, depth := o.Columns, o.Rows
			if side >= 2 {
				length, depth = depth, length
			}
			last, run := -1, 0
			for along := 0; along < length; along++ {
				shore := -1
				for away := 0; away < depth/6; away++ {
					x, y := along, away
					if side == 1 {
						y = o.Rows - 1 - away
					}
					if side == 2 {
						x, y = away, along
					}
					if side == 3 {
						x, y = o.Columns-1-away, along
					}
					if e.get("ocean", y*o.Columns+x) == 0 {
						shore = away
						break
					}
				}
				// Land crossing the projection boundary is not a shoreline.
				if shore > 0 && shore == last {
					run++
				} else {
					run = 1
				}
				last = shore
				if shore > 0 {
					maxRun = max(maxRun, run)
				}
			}
		}
		t.Logf("%s longest straight outer shoreline=%d cells", seed, maxRun)
		if maxRun > 20 {
			t.Errorf("%s coastline follows the rectangular frame for %d cells", seed, maxRun)
		}
	}
}

func TestWetlandsNeedSurfaceRetentionOrFloodwater(t *testing.T) {
	e := BuildEnvironment(realismOptions(t, `{"columns":16,"rows":16}`))
	for _, field := range e.Fields {
		clear(field)
	}
	e.Hydrology.Wetlands = nil
	for i := range e.Mask {
		e.Mask[i] = 1
		e.set("elevation", i, 100)
		e.set("drainageElevation", i, 100)
		e.set("summer", i, 25)
		e.set("temperature", i, 20)
		e.set("moisture", i, .85)
		e.set("groundwater", i, 1)
		e.set("permeability", i, .8)
		e.set("waterTableDepth", i, 3)
	}
	visible := make([]bool, len(e.Mask))
	e.buildHydroWetlands(visible)
	if len(e.Hydrology.Wetlands) != 0 {
		t.Fatal("well-drained aquifers became swamps")
	}
	i := 8*16 + 8
	e.set("permeability", i, .1)
	e.set("waterTableDepth", i, .15)
	e.set("slope", i, .03)
	lake := 4*16 + 4
	e.set("lake", lake, 1)
	e.set("waterBody", lake, 2)
	e.set("waterLevel", lake, 100)
	e.buildHydroWetlands(visible)
	if e.get("wetland", i) < .72 || e.get("wetland", lake+1) < .72 {
		t.Fatal("supported saturated ground or lake banks lost their wetlands")
	}
	if e.get("wetland", 0) != 0 {
		t.Fatal("wetland spread into ordinary drained land")
	}
}
