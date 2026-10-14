package terrain

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
)

func TestIntermediateClimateBudgetMatchesFinalClimate(t *testing.T) {
	o := realismOptions(t, `{"columns":32,"rows":24,"seed":"budget-equivalence"}`)
	e := BuildEnvironment(o)
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var intermediate, final Environment
	if err = json.Unmarshal(data, &intermediate); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &final); err != nil {
		t.Fatal(err)
	}
	intermediate.updateClimateBudget()
	final.BuildClimate()
	if !reflect.DeepEqual(intermediate.Fields, final.Fields) {
		for name, values := range final.Fields {
			if !reflect.DeepEqual(values, intermediate.Fields[name]) {
				t.Errorf("budget mismatch: %s", name)
			}
		}
	}
}

func TestSeparatedFoundationsAtMultipleScales(t *testing.T) {
	for _, size := range [][2]int{{80, 50}, {160, 100}, {240, 150}} {
		for _, gap := range []string{"narrow", "standard", "wide"} {
			for _, seed := range []string{"KRIEMHILD", "2", "7"} {
				e := shapeFixture(seed, size[0], size[1])
				e.Options.OceanSeparation = gap
				clear(e.Mask)
				e.generateContinentalMask(nil, noise(Seed(seed)))
				r := e.LandShapeDiagnostics()
				if r.MajorCount != e.Layout.Count || r.MajorCount < 4 || r.MajorCount > 7 {
					t.Fatalf("%s %v %s: majors %d / %d", seed, size, gap, r.MajorCount, e.Layout.Count)
				}
				if r.MinimumGap+1e-6 < e.Options.oceanGap() || r.ReservedLandCells > 0 {
					t.Fatalf("gap violated: %s %v %s: %+v", seed, size, gap, r)
				}
				if math.Abs(r.LandFraction-.42) > .015 && e.Layout.Warning == "" {
					t.Fatal("unreported coverage conflict", r.LandFraction)
				}
				if r.OpenOceanFraction < .40 || r.LargestOceanFraction < .95 {
					t.Fatalf("%s %v %s: ocean reduced to cracks or disconnected basins %.3f %.3f", seed, size, gap, r.OpenOceanFraction, r.LargestOceanFraction)
				}
			}
		}
	}
}

func TestFullPipelineContinentAndClimateDistribution(t *testing.T) {
	subtropical, equatorial, storm, steps, scrub := 0., 0., 0., 0, 0
	for _, seed := range []string{"KRIEMHILD", "2", "7"} {
		o := realismOptions(t, `{"columns":160,"rows":100,"landPercent":42}`)
		o.Seed = seed
		e := BuildEnvironment(o)
		r := e.LandShapeDiagnostics()
		c := e.ClimateDiagnostics()
		t.Logf("%s: majors=%v gap=%.2f land=%.2f%% open=%.1f%% reserve=%d biomes=%v bands=%+v smallDry=%d/%d", seed, r.MajorAreas, r.MinimumGap, 100*r.LandFraction, 100*r.OpenOceanFraction, r.ReservedLandCells, c.Biomes, c.Bands, c.SmallDryComponents, c.SmallDryCells)
		if r.MajorCount < 4 || r.MajorCount > 7 || r.MinimumGap < o.oceanGap() || r.ReservedLandCells > 0 {
			t.Error("continental separation lost in physical pipeline")
		}
		if math.Abs(r.LandFraction-.42) > .025 || r.LargestOceanFraction < .95 || r.OpenOceanFraction < .40 {
			t.Error("coverage or ocean basin regression")
		}
		if d := e.ValidateHydrology(); len(d) > 0 {
			t.Fatal(d[0])
		}
		subtropical += c.Bands[2].DryPercent
		equatorial += c.Bands[0].DryPercent
		storm += c.Bands[4].DryPercent
		steps += c.Biomes["steppe"]
		scrub += c.Biomes["scrubland"]
		if c.SmallDryCells > max(20, (c.Kinds["desert"]+c.Kinds["steppe"])/10) {
			t.Error("drylands fragmented into tiny speckles")
		}
		for i, land := range e.Mask {
			if land == 0 {
				continue
			}
			kind := e.ClimateKind(i)
			if e.desertAt(i) != (kind == "desert") {
				t.Fatalf("desert mismatch at %d: %s", i, kind)
			}
			if (e.get("steppe", i) > 0) != (kind == "steppe") {
				t.Fatal("steppe field contradicts monthly climate")
			}
			if e.Climate.Cells[i].Biome != e.BiomeAt(i) {
				t.Fatal("persisted biome mismatch")
			}
		}
	}
	if subtropical <= equatorial || subtropical <= storm || steps == 0 || scrub == 0 {
		t.Fatal("missing circulation/dryland transitions", subtropical, equatorial, storm, steps, scrub)
	}
}

func TestLayoutDefaultsConstraintsAndIndependence(t *testing.T) {
	for _, raw := range []string{`{"majorContinents":-1}`, `{"majorContinents":13}`, `{"oceanSeparation":"tiny"}`} {
		if _, err := DecodeEnvironment([]byte(raw)); err == nil {
			t.Fatal("invalid layout option accepted", raw)
		}
	}
	a := shapeFixture("separate-controls", 160, 100)
	b := shapeFixture("separate-controls", 160, 100)
	b.Options.ContinentCount = 40
	b.Options.PlateCount = 30
	b.Options.IslandFrequency = 3
	if !reflect.DeepEqual(a.layoutCenters(), b.layoutCenters()) {
		t.Fatal("layout centers depend on plates, islands or legacy points")
	}
	a.Options.MajorContinents = 12
	a.Options.LandPercent = 75
	a.Options.OceanSeparation = "wide"
	clear(a.Mask)
	a.generateContinentalMask(nil, noise(Seed(a.Options.Seed)))
	if a.Layout.Warning == "" || a.LandShapeDiagnostics().ReservedLandCells != 0 {
		t.Fatal("conflicting constraints silently filled ocean gaps")
	}
	old, err := DecodeEnvironment([]byte(`{"continentCount":9}`))
	if err != nil || old.MajorContinents != 0 || old.OceanSeparation != "standard" {
		t.Fatal("old configuration defaults", old, err)
	}
}

func TestAuthoritativeBiomeFixturesAndPersistence(t *testing.T) {
	for _, tc := range []struct {
		name            string
		temp, rain, lat float64
		biome           string
	}{
		{"humid temperate", 14, 1200, 45, "forest"}, {"semi arid", 22, 400, 35, "steppe"}, {"xeric", 22, 650, 35, "scrubland"}, {"arid", 28, 25, 25, "desert"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := climateFixture(t, tc.lat, tc.temp, tc.rain, 10, 100)
			i := 100
			t.Log(e.ClimateKind(i), e.BiomeAt(i), e.get("aridityIndex", i))
			if e.BiomeAt(i) != tc.biome {
				t.Fatalf("got %s, want %s", e.BiomeAt(i), tc.biome)
			}
			for _, candidate := range e.Candidates(i) {
				found := false
				config, _ := DecodeConfig(Defaults)
				for _, def := range config.Types {
					found = found || candidate == def.ID
				}
				if !found {
					t.Fatal("missing type", candidate)
				}
			}
			data, _ := json.Marshal(e)
			var loaded Environment
			if err := json.Unmarshal(data, &loaded); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(e.Fields["biome"], loaded.Fields["biome"]) || !reflect.DeepEqual(e.Climate, loaded.Climate) {
				t.Fatal("climate changed in persistence")
			}
			model := NewDetailModel(&loaded)
			for level := 0; level <= 6; level++ {
				p := model.Sample(4, 6, level)
				if p.Biome != e.BiomeAt(6*16+4) {
					t.Fatal("zoom changed biome", level, p.Biome)
				}
			}
		})
	}
}

func TestTransportBarrierAndRegionalLatitude(t *testing.T) {
	for _, width := range []int{64, 128} {
		e := climateFixture(t, 45, 15, 1000, 4, 100)
		// A controlled regional transect: sea in the west, a broad 2 km ridge,
		// then a low lee plain. Double resolution preserves kilometre dimensions.
		o := realismOptions(t, fmt.Sprintf(`{"columns":%d,"rows":32,"latitudeNorth":50,"latitudeSouth":40}`, width))
		e = BuildEnvironment(o)
		for i := range e.Mask {
			x := float64(i%width) / float64(width)
			e.Mask[i] = 1
			z := 100.
			if x < .2 {
				e.Mask[i] = 0
				z = -100
			}
			if x > .4 && x < .5 {
				z = 2100
			}
			e.set("elevation", i, z)
			e.set("temperature", i, 15)
			e.set("windX", i, 1)
			e.set("windY", i, 0)
			e.set("current", i, 0)
		}
		e.precipitation()
		windward, lee := 0., 0.
		for y := 4; y < 28; y++ {
			windward += e.get("precipitation", y*width+int(float64(width)*.39))
			lee += e.get("precipitation", y*width+int(float64(width)*.55))
		}
		if lee >= windward*.85 {
			t.Fatal("mountain barrier did not dry its lee", width, windward, lee)
		}
		for _, lat := range e.Fields["latitude"] {
			if lat < 40 || lat > 50 {
				t.Fatal("regional latitude escaped bounds")
			}
		}
	}
}
