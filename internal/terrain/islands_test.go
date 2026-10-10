package terrain

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestArchipelagoProcessesDensityAndDeterminism(t *testing.T) {
	processes, settings := map[string]int{}, map[string]int{}
	before, after := 0, 0
	for _, seed := range []string{"KRIEMHILD", "island-arc", "ocean-chains", "1"} {
		o := realismOptions(t, `{"columns":128,"rows":96,"landPercent":32}`)
		o.Seed = seed
		o.IslandFrequency = 0
		base := BuildEnvironment(o)
		o.IslandFrequency = 1
		e := BuildEnvironment(o)
		if len(e.Archipelagos) < 4 {
			t.Fatalf("too few supported groups: %s %d", seed, len(e.Archipelagos))
		}
		if err := e.ValidateArchipelagos(); err != nil {
			t.Fatal(err)
		}
		if diagnostics := e.ValidateHydrology(); len(diagnostics) > 0 {
			t.Fatal(diagnostics[0])
		}
		if err := e.ValidateNaturalData(); err != nil {
			t.Fatal(err)
		}
		before += len(landmasses(base))
		after += len(landmasses(e))
		banks, land := 0, 0
		for _, g := range e.Archipelagos {
			processes[g.Process]++
			settings[g.Setting]++
			for _, f := range g.Foundations {
				if f.Summit < 0 {
					banks++
				} else {
					land++
				}
			}
		}
		if banks == 0 || land == 0 {
			t.Fatal("groups lack mixed island/bank structure")
		}
		t.Logf("%s groups=%d members=%d banks=%d landmasses %d -> %d", seed, len(e.Archipelagos), land, banks, len(landmasses(base)), len(landmasses(e)))
		if seed == "KRIEMHILD" {
			again := BuildEnvironment(o)
			if !reflect.DeepEqual(e.Archipelagos, again.Archipelagos) || !reflect.DeepEqual(e.Fields, again.Fields) {
				t.Fatal("island geography regenerated differently")
			}
			o.IslandFrequency = 2.5
			dense := BuildEnvironment(o)
			if len(dense.Archipelagos) <= len(e.Archipelagos) || len(landmasses(dense)) <= len(landmasses(e)) {
				t.Fatal("frequency does not increase island groups")
			}
		}
	}
	if after < before+20 {
		t.Fatalf("island abundance did not meaningfully increase: %d -> %d", before, after)
	}
	for _, process := range []string{"subduction-arc", "spreading-chain", "hotspot-track", "continental-fragments", "shelf-remnants"} {
		if processes[process] == 0 {
			t.Fatal("missing island formation process", process, processes)
		}
	}
	if settings["marginal-sea"] == 0 || settings["open-ocean"] == 0 {
		t.Fatal("missing distribution diversity", settings)
	}
	t.Log(processes, settings)
}

func TestArchipelagoDistributionControlsAndHeightfields(t *testing.T) {
	for _, value := range []string{`{"islandFrequency":-1}`, `{"islandFrequency":4}`, `{"islandCoastalShare":-0.1}`, `{"islandCoastalShare":1.1}`} {
		if _, err := DecodeEnvironment([]byte(value)); err == nil {
			t.Fatal("invalid island control accepted", value)
		}
	}
	defaults, _ := DecodeEnvironment([]byte(`{}`))
	if defaults.IslandFrequency != 1 || defaults.IslandCoastalShare != .6 {
		t.Fatal("missing backward-compatible request defaults")
	}
	for _, coastal := range []float64{0, 1} {
		o := realismOptions(t, `{"columns":96,"rows":64,"seed":"coastal-controls"}`)
		o.IslandCoastalShare = coastal
		e := BuildEnvironment(o)
		if len(e.Archipelagos) == 0 {
			t.Fatal("no supported groups for distribution control")
		}
		for _, g := range e.Archipelagos {
			if (g.Setting == "open-ocean") != (coastal == 0) {
				t.Fatal("coastal distribution ignored", coastal, g.Setting)
			}
		}
	}
	// Supplied heightfields are authoritative, even with maximum island frequency.
	o := realismOptions(t, `{"columns":32,"rows":24,"islandFrequency":3}`)
	o.Heights = make([]float64, o.Columns*o.Rows)
	for i := range o.Heights {
		o.Heights[i] = -100
	}
	e := BuildEnvironment(o)
	if len(e.Archipelagos) != 0 {
		t.Fatal("islands generated over imported heights")
	}
	for _, z := range e.Fields["elevation"] {
		if z != -400 {
			t.Fatal("imported ocean floor modified", z)
		}
	}
}

func TestIslandFoundationsPreserveContinentsAndHaveGradualBases(t *testing.T) {
	e := &Environment{Options: EnvironmentOptions{Columns: 40, Rows: 30, Seed: "base"}, Margin: 2, Mask: make([]int, 1200), Fields: map[string][]float64{}, Heights: make([]int, 1200)}
	for _, k := range append(append([]string{}, FieldNames...), SurfaceFields...) {
		e.Fields[k] = make([]float64, 1200)
	}
	for i := range e.Mask {
		e.Fields["elevation"][i] = -4500
		if i%40 < 8 {
			e.Mask[i] = 1
			e.Fields["elevation"][i] = 1600
		}
	}
	original := append([]int(nil), e.Mask...)
	f := IslandFoundation{Center: [2]float64{18, 15}, Radius: 2, Aspect: 1, Summit: 700, Age: 3, Stage: "volcanic-island"}
	if !e.applyIslandFoundation(f, true, original, 100, noise(17)) {
		t.Fatal("supported edifice rejected")
	}
	for i, v := range original {
		if v != 0 && e.Fields["elevation"][i] != 1600 {
			t.Fatal("existing mountain modified")
		}
	}
	e.limitSurfaceGradients()
	if e.get("elevation", 15*40+18) < 200 {
		t.Fatal("island summit missing")
	}
	depths := map[int]bool{}
	for x := 19; x < 31; x++ {
		z := e.get("elevation", 15*40+x)
		if z < 0 {
			depths[int(z)] = true
		}
	}
	if len(depths) < 5 {
		t.Fatal("flat/absent submarine foundation", depths)
	}
	for i := range e.Mask {
		for _, j := range nb(i, 40, 30) {
			if math.Abs(e.get("elevation", i)-e.get("elevation", j)) > 1104 {
				t.Fatal("unbounded island cliff")
			}
		}
	}
}

func TestArchipelagoRecordsRoundTripAndRejectInvalidOrigins(t *testing.T) {
	e := BuildEnvironment(realismOptions(t, `{"columns":96,"rows":64,"seed":"archipelago-roundtrip"}`))
	data, _ := json.Marshal(e)
	var restored Environment
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if err := restored.ValidateArchipelagos(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(e.Archipelagos, restored.Archipelagos) {
		t.Fatal("island identities/geometry changed")
	}
	restored.Archipelagos[0].Foundations[0].Center[0] = -1
	if restored.ValidateArchipelagos() == nil {
		t.Fatal("out-of-world foundation accepted")
	}
	restored.Archipelagos = nil
	// A legacy world predates both group metadata and volcano group links.
	if restored.Entities != nil {
		for i := range restored.Entities.Volcanoes {
			restored.Entities.Volcanoes[i].Archipelago = ""
		}
	}
	if restored.ValidateArchipelagos() != nil {
		t.Fatal("legacy project rejected")
	}
}

func TestSubsidingArchipelagoReefsRequireWarmMarineHabitat(t *testing.T) {
	e := &Environment{Options: EnvironmentOptions{Columns: 24, Rows: 24, Seed: "subsiding-rim"}, Mask: make([]int, 576), Fields: map[string][]float64{}}
	for _, name := range append(append([]string{}, FieldNames...), SurfaceFields...) {
		e.Fields[name] = make([]float64, 576)
	}
	x, y := 12, 12
	for detailHash(Seed(e.Options.Seed), x, y, 361) < .35 {
		x++
	}
	e.Archipelagos = []Archipelago{{Process: "hotspot-track", Foundations: []IslandFoundation{{Center: [2]float64{float64(x), float64(y)}, Radius: 2, Age: 12, Stage: "subsiding-atoll"}}}}
	for i := range e.Mask {
		e.Fields["ocean"][i] = 1
		e.Fields["waterDepth"][i] = 150
		e.Fields["temperature"][i] = 26
		e.Fields["clarity"][i] = .9
		e.Fields["salinity"][i] = 35
		d := math.Hypot(float64(i%24-x), float64(i/24-y))
		if d > 1.8 && d < 3 {
			e.Fields["seamount"][i] = .9
			e.Fields["waterDepth"][i] = 18
		}
	}
	e.buildReefs(noise(Seed(e.Options.Seed)))
	coral := 0
	for _, v := range e.Fields["reefType"] {
		if v == 4 {
			coral++
		}
	}
	if coral < 4 {
		t.Fatal("mature supplied rim did not form a coherent atoll", coral)
	}
	for i := range e.Mask {
		e.Fields["temperature"][i] = 5
	}
	e.buildReefs(noise(Seed(e.Options.Seed)))
	if len(e.Reefs) > 0 {
		t.Fatal("cold reef-building coral")
	}
	for i := range e.Mask {
		e.Fields["temperature"][i] = 26
		e.Fields["ocean"][i] = 0
	}
	e.buildReefs(noise(Seed(e.Options.Seed)))
	if len(e.Reefs) > 0 {
		t.Fatal("freshwater reef-building coral")
	}
}
