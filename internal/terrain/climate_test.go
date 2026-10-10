package terrain

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func climateNumber(v float64) *float64 { return &v }
func climateFixture(t *testing.T, lat, temp, rain, coast, elevation float64) *Environment {
	t.Helper()
	o := realismOptions(t, `{"columns":16,"rows":16}`)
	o.SeasonalClimate = &SeasonalClimateOptions{AxialTilt: climateNumber(23.44), OrbitalDays: climateNumber(365), Eccentricity: climateNumber(0), Perihelion: climateNumber(0), Circulation: climateNumber(1)}
	e := BuildEnvironment(o)
	for _, field := range e.Fields {
		clear(field)
	}
	for i := range e.Mask {
		e.Mask[i] = 1
		for name, v := range map[string]float64{"latitude": lat, "temperature": temp, "precipitation": rain, "oceanDistance": coast, "elevation": elevation, "windX": 1, "windStrength": .5, "permeability": .5} {
			e.set(name, i, v)
		}
	}
	e.BuildClimate()
	return e
}

func TestClimateRegimesAndSeasonCalibration(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		lat, temp, rain, coast, height float64
		kind                           string
		min, max                       int
	}{
		{"equatorial island", 1, 27, 2600, 0, 10, "rainforest", 1, 2},
		{"temperate interior", 45, 12, 1200, 25, 200, "continental", 4, 4},
		{"coastal region", 45, 12, 1200, 0, 20, "oceanic", 2, 4},
		{"high mountains", 45, -15, 1300, 5, 4000, "alpine", 2, 4},
		{"hot desert", 25, 28, 25, 20, 100, "desert", 1, 2},
		{"polar region", 85, -35, 150, 2, 100, "polar", 1, 2},
		{"monsoon", 10, 30, 1500, 6, 100, "monsoon", 2, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := climateFixture(t, tc.lat, tc.temp, tc.rain, tc.coast, tc.height)
			c := e.Climate.Cells[100]
			kind := e.Climate.Zones[c.Zone].Kind
			t.Log(kind, len(c.Seasons), c.Estimate)
			if kind != tc.kind {
				t.Fatalf("zone %s, expected %s", kind, tc.kind)
			}
			if len(c.Seasons) < tc.min || len(c.Seasons) > tc.max {
				t.Fatalf("unexpected seasonal phases %+v", c.Seasons)
			}
			if err := e.Climate.Validate(16, 16); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestClimateCustomCountAndPlanetaryForcing(t *testing.T) {
	e := climateFixture(t, 45, 15, 1000, 10, 100)
	for n := 1; n <= 6; n++ {
		e.Options.SeasonalClimate.Mode = "custom"
		e.Options.SeasonalClimate.Count = n
		e.BuildClimate()
		if len(e.Climate.Calendar) != n || len(e.Climate.Cells[0].Seasons) != n {
			t.Fatal("custom count lost", n)
		}
		if err := e.Climate.Validate(16, 16); err != nil {
			t.Fatal(err)
		}
	}
	e.Options.SeasonalClimate.Mode = "automatic"
	e.Options.SeasonalClimate.AxialTilt = climateNumber(0)
	e.Options.SeasonalClimate.Circulation = climateNumber(0)
	e.BuildClimate()
	flat := e.Climate.Cells[0]
	if len(flat.Seasons) != 1 {
		t.Fatal("invented seasons without annual forcing", flat.Seasons)
	}
	e.Options.SeasonalClimate.Mode = "custom"
	e.Options.SeasonalClimate.Count = 6
	e.BuildClimate()
	if e.Climate.Cells[0].Months != flat.Months {
		t.Fatal("custom count invented climate changes")
	}
	e.Options.SeasonalClimate.AxialTilt = climateNumber(65)
	e.Options.SeasonalClimate.Eccentricity = climateNumber(.25)
	e.Options.SeasonalClimate.OrbitalDays = climateNumber(600)
	e.BuildClimate()
	if e.Climate.Cells[0].Months == flat.Months {
		t.Fatal("fictional orbit did not alter climate")
	}
	if err := e.Climate.Validate(16, 16); err != nil {
		t.Fatal(err)
	}
}

func TestClimateSpatialInfluencesAndScaleIndependence(t *testing.T) {
	e := climateFixture(t, 45, 12, 1200, 0, 20)
	coast := e.Climate.Cells[0].Months
	e.Fields["oceanDistance"][0] = 25
	e.BuildClimate()
	interior := e.Climate.Cells[0].Months
	span := func(c [12]ClimateMonth) float64 {
		lo, hi := 1000., -1000.
		for _, m := range c {
			lo = math.Min(lo, m.Temperature)
			hi = math.Max(hi, m.Temperature)
		}
		return hi - lo
	}
	if span(interior) <= span(coast)*1.5 {
		t.Fatal("maritime influence missing")
	}
	e.Fields["temperature"][1] = -15
	e.Fields["elevation"][1] = 4200
	e.BuildClimate()
	if e.Climate.Cells[0].Zone == e.Climate.Cells[1].Zone {
		t.Fatal("latitude band erased mountain climate")
	}
	q := e.Climate.Describe(0, 0)
	if len(q.Transition) == 0 || len(q.Seasons) == 0 {
		t.Fatal("point description misses transitions or seasons")
	}
	before, _ := json.Marshal(e.Climate.Cells)
	e.Options.SeasonalClimate.WidthPX = 160
	e.Options.SeasonalClimate.HeightPX = 160
	e.Options.SeasonalClimate.PixelsPerCell = 10
	e.BuildClimate()
	after, _ := json.Marshal(e.Climate.Cells)
	if string(before) != string(after) {
		t.Fatal("pixel density changed climate")
	}
	if e.Climate.Scale.CellCount != 256 || !e.Climate.Scale.Approximate {
		t.Fatal("scale/provenance lost")
	}
}

func TestClimateDeterminismWaterBudgetAndValidation(t *testing.T) {
	o := realismOptions(t, `{"columns":24,"rows":16,"seasonalClimate":{"mode":"custom","count":3}}`)
	a, b := BuildEnvironment(o), BuildEnvironment(o)
	if !reflect.DeepEqual(a.Climate, b.Climate) {
		t.Fatal("climate not deterministic")
	}
	for i, c := range a.Climate.Cells {
		rain := 0.
		for _, m := range c.Months {
			rain += m.Rain
		}
		expected := a.get("precipitation", i) * a.Climate.Parameters.OrbitalDays / 365
		if math.Abs(rain-expected) > .02 {
			t.Fatal("monthly rain does not conserve annual supply")
		}
	}
	p, _ := o.climateConfiguration()
	o.Seed = "another planet"
	other, _ := o.climateConfiguration()
	if p.AxialTilt == other.AxialTilt && p.OrbitalDays == other.OrbitalDays {
		t.Fatal("unspecified planetary settings did not vary with seed")
	}
	b.Climate.Cells[0].Seasons[0].Months = 0
	if b.Climate.Validate(24, 16) == nil {
		t.Fatal("invalid seasonal coverage accepted")
	}
	for _, raw := range []string{`{"seasonalClimate":{"mode":"custom","count":7}}`, `{"seasonalClimate":{"axialTilt":91}}`, `{"seasonalClimate":{"width_px":100,"height_px":100,"pixels_per_cell":2}}`} {
		if _, err := DecodeEnvironment([]byte(raw)); err == nil {
			t.Fatal("invalid climate settings accepted", raw)
		}
	}
}

func TestClimateRefreshesEditedElevationAndWater(t *testing.T) {
	base := climateFixture(t, 40, 15, 900, 10, 100)
	base.Climate.Parameters.OrbitalDays = 410 // Stored metadata wins over generation defaults.
	e := *base
	e.Fields = make(map[string][]float64)
	for k, v := range base.Fields {
		e.Fields[k] = append([]float64(nil), v...)
	}
	i := 8*16 + 8
	e.Fields["elevation"][i] += 2500
	e.Fields["waterBody"][i+1] = 2
	e.Fields["waterDepth"][i+1] = 10
	e.RebuildClimateForSurface(base)
	if e.Climate.Parameters.OrbitalDays != 410 {
		t.Fatal("edit regenerated saved planetary settings")
	}
	before, after := base.Climate.Describe(8, 8), e.Climate.Describe(8, 8)
	if after.Months[0].Temperature >= before.Months[0].Temperature-10 {
		t.Fatal("elevation edit left stale climate")
	}
	if e.Climate.Cells[i].Modifiers.WaterProximity <= base.Climate.Cells[i].Modifiers.WaterProximity {
		t.Fatal("new lake not reflected in climate")
	}
	if base.Climate.Cells[i].Modifiers.Elevation != 100 {
		t.Fatal("edited climate mutated immutable parent")
	}
}

func TestClimateCustomNamesFictionalZonesAndShortThaw(t *testing.T) {
	e := climateFixture(t, 45, 12, 1200, 10, 100)
	e.Options.SeasonalClimate.Mode = "custom"
	e.Options.SeasonalClimate.Count = 3
	e.Options.SeasonalClimate.Names = []string{"Ember", "Rain", "Frost"}
	e.Options.SeasonalClimate.Rules = []ClimateRule{{ID: "silver", Name: "Silver sky", MinTemperature: -100, MaxTemperature: 100, MinRain: 0, MaxRain: 10000}}
	e.BuildClimate()
	if e.Climate.Describe(4, 4).Zone != "Silver sky" {
		t.Fatal("fictional classification ignored")
	}
	for i, s := range e.Climate.Cells[0].Seasons {
		if s.Name != e.Options.SeasonalClimate.Names[i] {
			t.Fatal("fictional season name lost")
		}
	}
	var curve [12]ClimateMonth
	for i := range curve {
		curve[i] = ClimateMonth{Temperature: -20 + float64(i), Rain: 15, Daylight: 4}
	}
	curve[5].Temperature = 3
	curve[6].Temperature = 5
	p, _ := e.Options.climateConfiguration()
	p.Mode = "automatic"
	p.Names = nil
	seasons, _ := climateSeasons(curve, ClimateModifiers{Latitude: 80}, p)
	if len(seasons) != 2 {
		t.Fatal("polar cycle should retain thaw and freeze", seasons)
	}
	found := false
	for _, s := range seasons {
		if s.Name == "Thaw season" && s.Months == 2 {
			found = true
		}
	}
	if !found {
		t.Fatal("short thaw was stretched into half a year", seasons)
	}
}
