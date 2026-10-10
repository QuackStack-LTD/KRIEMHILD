package terrain

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func realismOptions(t *testing.T, raw string) EnvironmentOptions {
	t.Helper()
	o, err := DecodeEnvironment([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	o.Realism = true
	return o
}

func TestRealismDeterminismAndConstraints(t *testing.T) {
	for _, seed := range []string{"KRIEMHILD", "island-arc", "polar-mountain", "river-basin"} {
		o := realismOptions(t, `{"columns":96,"rows":64}`)
		o.Seed = seed
		e := BuildEnvironment(o)
		if !reflect.DeepEqual(e, BuildEnvironment(o)) {
			t.Fatal("nondeterministic", seed)
		}
		for name, field := range e.Fields {
			for i, v := range field {
				if !finite(v) {
					t.Fatalf("nonfinite %s[%d]", name, i)
				}
			}
		}
		f := e.get
		for i := range e.Mask {
			if f("reef", i) > .2 && (e.Mask[i] != 0 || f("bathymetry", i) >= 70 || f("temperature", i) <= 20 || f("temperature", i) >= 31 || f("salinity", i) <= 30 || f("clarity", i) <= .6 || f("substrate", i) == 0) {
				t.Fatal("invalid reef", i)
			}
			if f("dune", i) > .13 && (f("deposition", i) <= 0 || f("aridity", i) <= .5 || f("slope", i) >= .25 || f("lake", i) > 0) {
				t.Fatal("invalid dune", i)
			}
			if f("mesa", i) > 0 && (f("layering", i) == 0 || f("caprock", i) == 0 || f("erosion", i) == 0) {
				t.Fatal("invalid mesa", i)
			}
			if f("oasis", i) > .5 && (f("aridity", i) <= .55 || f("groundwater", i) <= .5) {
				t.Fatal("invalid oasis", i)
			}
		}
		for _, r := range e.Entities.Rivers {
			if len(r.Path) < 2 || r.Source != r.Path[0] || r.Mouth != r.Path[len(r.Path)-1] {
				t.Fatal("invalid river endpoints")
			}
			for k := 1; k < len(r.Path); k++ {
				a, b := r.Path[k-1], r.Path[k]
				if int(f("flow", a)) != b || f("drainageElevation", a) <= f("drainageElevation", b) || f("accumulation", a) > f("accumulation", b)+.001 {
					t.Fatal("river climbs or loses discharge", a, b)
				}
			}
		}
		for _, v := range e.Entities.Volcanoes {
			if f("volcano", v.Cell) < .48 {
				t.Fatal("vent without tectonic/hotspot source")
			}
			if v.Archipelago != "" {
				if err := e.ValidateArchipelagos(); err != nil {
					t.Fatal(err)
				}
			} else if v.Origin == "hotspot" && f("hotspot", v.Cell) <= .48 || v.Origin == "subduction" && f("boundary", v.Cell) != 1 || v.Origin == "rift" && f("boundary", v.Cell) != 2 {
				t.Fatal("volcanic origin disagrees with tectonics", v)
			}
			if !v.Active && (len(v.Lava) > 0 || len(v.Ash) > 0) {
				t.Fatal("dormant volcano erupted")
			}
			for k := 1; k < len(v.Lava); k++ {
				if f("elevation", v.Lava[k]) >= f("elevation", v.Lava[k-1]) {
					t.Fatal("lava climbs")
				}
			}
			for _, j := range v.Ash {
				if f("ash", j) <= 0 {
					t.Fatal("missing ash deposit")
				}
			}
		}
		for _, s := range e.Entities.Settlements {
			if len(s.Farms) == 0 || f("freshwaterDistance", s.Cell) > 2 || f("farmland", s.Cell) < .15 {
				t.Fatal("settlement lacks water/farms")
			}
			for _, j := range s.Farms {
				if f("farmland", j) < .15 || f("cultivated", j) != 1 || f("slope", j) > .18 {
					t.Fatal("invalid farm")
				}
			}
		}
		t.Logf("%s: rivers=%d volcanoes=%d dunes=%d settlements=%d", seed, len(e.Entities.Rivers), len(e.Entities.Volcanoes), len(e.Entities.DuneFields), len(e.Entities.Settlements))
	}
}

// A controlled equatorial continent isolates coast, interior and mountain effects.
func calibrationContinent(t *testing.T, ridge bool) *Environment {
	o := realismOptions(t, `{"columns":96,"rows":64,"latitudeNorth":20,"latitudeSouth":-20,"windDirection":1}`)
	o.Heights = make([]float64, o.Columns*o.Rows)
	for i := range o.Heights {
		x, y := i%o.Columns, i/o.Columns
		o.Heights[i] = -5
		if x > 8 && x < 88 && y > 8 && y < 55 {
			o.Heights[i] = 100
			if ridge && x >= 42 && x <= 46 {
				o.Heights[i] = 850
			}
		}
	}
	return BuildEnvironment(o)
}
func TestRealismClimateCalibration(t *testing.T) {
	flat, ridge := calibrationContinent(t, false), calibrationContinent(t, true)
	mean := func(e *Environment, name string, x int) float64 {
		s := 0.
		for y := 24; y < 40; y++ {
			s += e.get(name, y*96+x)
		}
		return s / 16
	}
	coast, interior := mean(flat, "precipitation", 10), mean(flat, "precipitation", 80)
	if coast <= interior*2 {
		t.Fatalf("interior not drier: coast=%f interior=%f", coast, interior)
	}
	if mean(flat, "summer", 80)-mean(flat, "winter", 80) <= mean(flat, "summer", 10)-mean(flat, "winter", 10) {
		t.Fatal("ocean does not moderate seasons")
	}
	windward, leeward := mean(ridge, "precipitation", 42), mean(ridge, "precipitation", 49)
	if windward <= leeward*2 || leeward >= mean(flat, "precipitation", 49) {
		t.Fatalf("missing rain shadow: windward=%f leeward=%f flat=%f", windward, leeward, mean(flat, "precipitation", 49))
	}
	if mean(ridge, "temperature", 44) >= mean(flat, "temperature", 44)-15 {
		t.Fatal("missing elevation lapse rate")
	}
	if mean(flat, "precipitation", 10) < 1200 {
		t.Fatal("equatorial coast not wet")
	}
	t.Logf("coast/interior rain %.0f/%.0f, ridge windward/leeward %.0f/%.0f", coast, interior, windward, leeward)
}

func TestRealismReefAndIceCalibration(t *testing.T) {
	tropical := calibrationContinent(t, false)
	count := 0
	for _, v := range tropical.Fields["reef"] {
		if v > .2 {
			count++
		}
	}
	if count == 0 {
		t.Fatal("no warm shallow reef habitat")
	}
	o := tropical.Options
	o.LatitudeNorth = 88
	o.LatitudeSouth = 65
	o.Rainfall = 3
	for i, v := range o.Heights {
		if v > 0 {
			o.Heights[i] = 900
		}
	}
	polar := BuildEnvironment(o)
	ice := 0
	for i, v := range polar.Fields["glacier"] {
		if v > .35 {
			ice++
			if polar.get("snowBalance", i) <= 0 && polar.get("summer", i) >= 8 {
				t.Fatal("ice survives strong ablation")
			}
		}
	}
	if ice == 0 {
		t.Fatal("no polar mountain glaciers")
	}
	for _, v := range polar.Fields["reef"] {
		if v > 0 {
			t.Fatal("polar reef")
		}
	}
}

func TestRealismSandFollowsWind(t *testing.T) {
	e := calibrationContinent(t, false)
	for i := range e.Mask {
		for _, name := range []string{"sandSupply", "deposition", "sandTransport", "duneField", "dune"} {
			e.set(name, i, 0)
		}
		e.set("moisture", i, 0)
		e.set("vegetation", i, 0)
		e.set("aridity", i, 1)
		e.set("windX", i, 1)
		e.set("windY", i, 0)
		e.set("windStrength", i, 1)
		e.set("summer", i, 25)
		e.set("layering", i, 0)
		e.set("freshwaterDistance", i, 100)
		e.set("oceanDistance", i, 100)
	}
	source := 32*96 + 30
	e.set("layering", source, 1)
	e.set("erosion", source, 1)
	e.Entities.DuneFields = nil
	e.transportSand()
	if e.get("deposition", source+1) <= 0 || e.get("deposition", source-1) > 0 {
		t.Fatal("sand not transported downwind")
	}
	if len(e.Entities.DuneFields) == 0 {
		t.Fatal("no dune fields from available sand")
	}
	for _, r := range e.Entities.DuneFields {
		if math.Abs(r.Orientation-math.Pi/2) > .001 {
			t.Fatal("dune orientation ignores wind")
		}
	}
}

func TestRealismSolverAndOffCompatibility(t *testing.T) {
	o := realismOptions(t, `{"columns":48,"rows":32,"seed":"KRIEMHILD"}`)
	var c Config
	if err := json.Unmarshal(Defaults, &c); err != nil {
		t.Fatal(err)
	}
	s, err := PrepareEnvironment(o, c)
	if err != nil {
		t.Fatal(err)
	}
	for k := 0; k < 10000 && s.Status == "running"; k++ {
		s.Step()
	}
	s.Cleanup()
	if s.Status != "done" {
		t.Fatal(s.Status)
	}
	for i, m := range s.Dom {
		if m&s.EnvironmentMasks[i] != m {
			t.Fatal("WFC escaped physical constraints")
		}
	}
	o.Realism = false
	legacy := BuildEnvironment(o)
	if legacy.Entities != nil || legacy.Fields["climate"] != nil || legacy.Version != "kriemhild-environment-v3" {
		t.Fatal("realism leaks into disabled mode")
	}
}
