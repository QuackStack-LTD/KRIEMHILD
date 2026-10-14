package terrain

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
)

func TestHydrologyNetworkValidation(t *testing.T) {
	for _, seed := range []string{"KRIEMHILD", "mountain-drainage", "dry-world", "island-arc", "polar-mountain", "river-basin"} {
		o := realismOptions(t, `{"columns":96,"rows":64}`)
		o.Seed = seed
		e := BuildEnvironment(o)
		t.Logf("%s: basins %d reaches %d springs %d wetlands %d canals %d", seed, len(e.Hydrology.Basins), len(e.Hydrology.Reaches), len(e.Hydrology.Springs), len(e.Hydrology.Wetlands), len(e.Hydrology.Canals))
		if d := e.ValidateHydrology(); len(d) > 0 {
			t.Errorf("%s: %d invalid features: %+v", seed, len(d), d[:min(10, len(d))])
		}
	}
}

func TestHydrologyClimatesAndPortableState(t *testing.T) {
	for k := 0; k < 24; k++ {
		o := realismOptions(t, `{"columns":64,"rows":48}`)
		o.Seed = fmt.Sprint("hydrology-", k)
		o.Realism = k%2 == 0
		o.Rainfall = []float64{.1, 1, 3}[k%3]
		o.TemperatureOffset = []float64{-20, 0, 20}[k/3%3]
		e := BuildEnvironment(o)
		if d := e.ValidateHydrology(); len(d) > 0 {
			t.Fatalf("seed %d: %+v", k, d[:min(5, len(d))])
		}
		data, _ := json.Marshal(e)
		var restored Environment
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(e.Hydrology, restored.Hydrology) || len(restored.ValidateHydrology()) != 0 {
			t.Fatal("hydrology changed on portable round trip")
		}
	}
}

func waterSupplyFixture(rain bool) *Environment {
	e := &Environment{Options: EnvironmentOptions{Columns: 48, Rows: 32, Seed: "catchment"}, Mask: make([]int, 48*32), Heights: make([]int, 48*32), Fields: map[string][]float64{}}
	for _, name := range append(append([]string{}, FieldNames...), SurfaceFields...) {
		e.Fields[name] = make([]float64, len(e.Mask))
	}
	for i := range e.Mask {
		x, y := i%48, i/48
		z := -30.
		if x > 2 && x < 45 && y > 2 && y < 29 {
			z = float64(x-2)*10 + math.Abs(float64(y-16))*5
		}
		e.set("elevation", i, z)
		e.set("temperature", i, 22)
		e.set("summer", i, 28)
		e.set("winter", i, 10)
		e.set("aridity", i, .9)
		e.set("moisture", i, .1)
		e.set("slope", i, .01)
		if x > 30 && z > 0 {
			e.set("mountainCore", i, .3)
		}
		if rain && x > 30 {
			e.set("precipitation", i, 2500)
		}
	}
	e.connectOcean()
	e.prepareHydrology()
	e.simulateHydrology()
	e.finishHydrology()
	return e
}
func TestDistantWetCatchmentSustainsDesertRiver(t *testing.T) {
	wet, dry := waterSupplyFixture(true), waterSupplyFixture(false)
	found := false
	for _, r := range wet.Hydrology.Reaches {
		if r.From%48 < 20 && wet.get("precipitation", r.From) == 0 && r.Discharge > 2 {
			found = true
		}
	}
	if !found {
		t.Fatal("desert erased upstream water supply")
	}
	if len(dry.Hydrology.Reaches) != 0 || len(dry.Hydrology.Springs) != 0 {
		t.Fatal("water created without precipitation/recharge")
	}
	if d := wet.ValidateHydrology(); len(d) > 0 {
		t.Fatal(d)
	}
}

func TestHydrologyDiagnosticsIdentifyBrokenObject(t *testing.T) {
	e := waterSupplyFixture(true)
	r := &e.Hydrology.Reaches[0]
	id := r.ID
	r.Downstream = "missing"
	found := false
	for _, d := range e.ValidateHydrology() {
		if d.Object == id && d.Reason == "destination is missing" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing object-level connectivity diagnostic")
	}
}
