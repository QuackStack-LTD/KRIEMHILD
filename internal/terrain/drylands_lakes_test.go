package terrain

import (
	"encoding/json"
	"math"
	"testing"
)

func TestDesertsFollowAnnualClimateWaterBalance(t *testing.T) {
	e := BuildEnvironment(realismOptions(t, `{"columns":96,"rows":64,"seed":"dryland-belts"}`))
	deserts, wet := 0, 0
	for i, c := range e.Climate.Cells {
		if e.get("waterBody", i) > 0 {
			continue
		}
		zone := e.Climate.Zones[c.Zone].Kind
		if zone == "desert" {
			deserts++
		} else {
			wet++
		}
		if e.desertAt(i) != (zone == "desert") {
			t.Fatalf("cell %d desert classification disagrees with %s", i, zone)
		}
	}
	if deserts == 0 || wet == 0 {
		t.Fatal("fixture lacks climate contrast", deserts, wet)
	}
	m := NewDetailModel(e)
	for i := range e.Mask {
		p := m.Sample(float64(i%e.Options.Columns), float64(i/e.Options.Columns), 0)
		if p.Desert == nil || math.Abs(*p.Desert-e.get("desert", i)) > 1e-6 {
			t.Fatal("detail lost climate suitability", i)
		}
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var restored Environment
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	for i := range e.Mask {
		if restored.desertAt(i) != e.desertAt(i) {
			t.Fatal("saved desert distribution changed")
		}
	}
}

func TestBasinApronsRespectKilometreScale(t *testing.T) {
	e := BuildEnvironment(realismOptions(t, `{"columns":64,"rows":48,"seed":"lake-scale"}`))
	i := 24*64 + 32
	if r := e.basinRadiusCells(i, 2.4, 100); r >= 1 {
		t.Fatal("planetary storage basin still spans multiple huge cells", r)
	}
	// The same broad footprint is valid on a regional grid with smaller cells.
	e.Geography.CellAreaKM2[24] = 100
	if r := e.basinRadiusCells(i, 2.4, 100); r != 2.4 {
		t.Fatal("regional basin unnecessarily shrunk", r)
	}
	e.Geography = nil
	if r := e.basinRadiusCells(i, 2.4, 100); r != 2.4 {
		t.Fatal("unscaled legacy behaviour changed", r)
	}
}

func TestLakeEvaporationRespondsToDryAirAndWind(t *testing.T) {
	e := &Environment{Fields: map[string][]float64{"aridity": {0, 1, 1}, "windStrength": {0, 0, 1}}}
	humid, dry, windy := e.lakeEvaporation(0, 25), e.lakeEvaporation(1, 25), e.lakeEvaporation(2, 25)
	if !(windy > dry && dry > humid && windy <= humid*2.01) {
		t.Fatal("invalid open-water losses", humid, dry, windy)
	}
	if e.lakeEvaporation(2, -40) <= 0 {
		t.Fatal("negative cold-water evaporation")
	}
}
