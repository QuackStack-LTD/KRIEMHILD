package terrain

import (
	"encoding/json"
	"math"
	"testing"
)

func TestContinentalDrainageProfile(t *testing.T) {
	for _, seed := range []string{"KRIEMHILD", "mountain-drainage", "dry-world", "island-arc", "polar-mountain", "river-basin"} {
		o := realismOptions(t, `{"columns":160,"rows":100}`)
		o.Seed = seed
		e := BuildEnvironment(o)
		land, sinks, basinSinks, lowland := 0, 0, 0, 0
		longest, maxArea := 0., 0.
		for i := range e.Mask {
			if e.get("ocean", i) > 0 {
				continue
			}
			land++
			if e.get("flow", i) < 0 && e.get("waterBody", i) == 0 {
				sinks++
				if e.get("basin", i) > 0 {
					basinSinks++
				}
			}
			maxArea = math.Max(maxArea, e.get("catchmentArea", i))
			length := 0.
			for at, k := i, 0; at >= 0 && k < len(e.Mask); k++ {
				j := int(e.get("flow", at))
				if j < 0 {
					break
				}
				length += math.Hypot(float64(j%o.Columns-at%o.Columns), float64(j/o.Columns-at/o.Columns))
				if e.get("ocean", j) > 0 {
					break
				}
				at = j
			}
			longest = math.Max(longest, length)
		}
		for _, r := range e.Hydrology.Reaches {
			if e.get("mountainCore", r.From) < .2 {
				lowland++
			}
		}
		t.Logf("%s land=%d sinks=%d basinSinks=%d longest=%.1f area=%.0f reaches=%d lowland=%d lakes=%d", seed, land, sinks, basinSinks, longest, maxArea, len(e.Hydrology.Reaches), lowland, len(e.WaterBodies)-1)
		t.Logf("systems: %+v lengths: %+v invalid=%d coverage=%.1f%%", e.Hydrology.Statistics.Classes, e.Hydrology.Statistics.Lengths, e.Hydrology.Statistics.InvalidDestinations, e.Hydrology.Statistics.CoveragePercent)
		stats := e.Hydrology.Statistics
		if stats.Classes[0].Count == 0 || lowland < len(e.Hydrology.Reaches)/2 || stats.InvalidDestinations != 0 || stats.CoveragePercent < 5 || stats.CoveragePercent > 25 {
			t.Fatalf("incomplete or excessive drainage hierarchy: %+v", stats)
		}
		if d := e.ValidateHydrology(); len(d) > 0 {
			t.Fatal(d[:min(5, len(d))])
		}
	}
}

func TestContinentalRiverCrossesExtensiveLowlands(t *testing.T) {
	o := realismOptions(t, `{"columns":160,"rows":100,"latitudeNorth":28,"latitudeSouth":-5,"rainfall":2}`)
	o.Heights = make([]float64, o.Columns*o.Rows)
	for i := range o.Heights {
		x, y := i%o.Columns, i/o.Columns
		z := -100.
		if x > 4 && x < 155 && y > 4 && y < 95 {
			valley := float64(25)
			if y >= 50 {
				valley = 75
			}
			z = 12 + float64(154-x)*2 + math.Abs(float64(y)-valley)*3
		}
		o.Heights[i] = z / 4
	}
	e := BuildEnvironment(o)
	count := 0
	for _, r := range e.Hydrology.Rivers {
		if r.Class == "major" && r.Length > 100 {
			count++
			if len(r.Tributaries) < 2 {
				t.Fatal("continental trunk has no tributary network")
			}
			if e.get("elevation", r.Source) > 600 {
				t.Fatal("lowland trunk depends on mountain source")
			}
		}
	}
	if count < 2 {
		t.Fatalf("expected multiple long lowland systems; got %d, stats %+v", count, e.Hydrology.Statistics)
	}
	if d := e.ValidateHydrology(); len(d) > 0 {
		t.Fatal(d[:min(5, len(d))])
	}
	model := NewDetailModel(e)
	seen := 0
	for _, f := range model.Features {
		if f.Class == "major" {
			seen++
			if f.Level != 0 || f.RiverID == "" {
				t.Fatal("trunk loses identity or vanishes at world scale")
			}
		}
	}
	if seen < 100 {
		t.Fatal("continental river geometry was fragmented or omitted")
	}
}

func TestRiverSystemPersistenceAndContributingWater(t *testing.T) {
	o := realismOptions(t, `{"columns":96,"rows":64,"seed":"island-arc"}`)
	e := BuildEnvironment(o)
	for _, r := range e.Hydrology.Reaches {
		expected := r.Discharge - math.Min(r.Discharge, e.get("hydroLoss", r.From))
		if e.get("accumulation", r.To)+1e-5 < expected {
			t.Fatal("downstream river lost water without a recorded loss")
		}
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var restored Environment
	if err = json.Unmarshal(b, &restored); err != nil {
		t.Fatal(err)
	}
	before := restored.VisibleChannels()
	// Cached selection must come from the saved graph, never a re-evaluation
	// using today's thresholds or even changes to climate inputs.
	for i := range restored.Fields["runoff"] {
		restored.Fields["runoff"][i] = 0
	}
	after := restored.VisibleChannels()
	for i := range before {
		if before[i] != after[i] {
			t.Fatal("saved river network was reselected")
		}
	}
	model := NewDetailModel(&restored)
	// World tiles expose the whole logical network to debug overlays, even
	// when normal terrain rendering hides streams until a closer zoom.
	for _, feature := range model.Features {
		if feature.Class != "stream" || feature.Level <= 1 {
			continue
		}
		p := feature.Path[0]
		tile, err := model.Tile(0, int(p[0]/32), int(p[1]/32))
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, f := range tile.Features {
			if f.ID == feature.ID {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("debug tile discarded a tributary from the saved network")
		}
		break
	}
	rivers := map[string]bool{}
	for _, r := range restored.Hydrology.Rivers {
		rivers[r.ID] = true
	}
	for _, f := range model.Features {
		if f.Class != "" && !rivers[f.RiverID] {
			t.Fatal("detailed river lost parent system")
		}
	}
	if len(restored.Hydrology.Rivers) == 0 {
		t.Fatal("fixture has no rivers")
	}
	id := restored.Hydrology.Rivers[0].ID
	restored.Hydrology.Rivers[0].Downstream = "missing-system"
	found := false
	for _, d := range restored.ValidateHydrology() {
		if d.Object == id {
			found = true
		}
	}
	if !found {
		t.Fatal("broken river-system destination was not diagnosed")
	}
}
