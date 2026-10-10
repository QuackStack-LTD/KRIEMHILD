package terrain

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestPlanetaryScaleAreaDistancesAndResolution(t *testing.T) {
	o := realismOptions(t, `{"columns":64,"rows":32}`)
	g, err := o.geographicScale()
	if err != nil {
		t.Fatal(err)
	}
	area := 0.
	for _, a := range g.CellAreaKM2 {
		area += a * float64(o.Columns)
	}
	if math.Abs(area-4*math.Pi*g.Radius*g.Radius) > 1e-5 {
		t.Fatal("sphere area not conserved", area)
	}
	e := &Environment{Options: o, Geography: g}
	equator := 16 * 64
	polar := 0
	if e.stepKM(polar, polar+1) >= e.stepKM(equator, equator+1)*.1 {
		t.Fatal("polar longitude distortion ignored")
	}
	if math.Abs(e.stepKM(equator, equator+1)-e.stepKM(equator, equator+63)) > 1e-8 {
		t.Fatal("longitude seam has a false long distance")
	}
	o.Geography = &GeographicOptions{WidthPX: 1280, HeightPX: 640, PixelsPerCell: 20}
	high, err := o.geographicScale()
	if err != nil || !reflect.DeepEqual(high.CellAreaKM2, g.CellAreaKM2) {
		t.Fatal("render resolution changed geographic area", err)
	}
	o.Geography.WorldScale = 1
	if _, err = o.geographicScale(); err == nil {
		t.Fatal("inconsistent whole-planet scale accepted")
	}
	o.Geography.Coverage = "local"
	local, err := o.geographicScale()
	if err != nil || math.Abs(local.WorldScale-1) > 1e-8 || local.WrapX {
		t.Fatal("regional extent not derived", err)
	}
}
func TestPlanetaryLowlandTrunkAndRegionalBoundary(t *testing.T) {
	o := realismOptions(t, `{"columns":64,"rows":32,"latitudeNorth":45,"latitudeSouth":25,"rainfall":3,"geography":{"map_coverage":"continent"},"erosion":{"iterations":0}}`)
	o.Heights = make([]float64, o.Columns*o.Rows)
	for i := range o.Heights {
		x, y := i%64, i/64
		o.Heights[i] = (30 + float64(63-x)*2 + math.Abs(float64(y)-16)*4) / 4
	}
	e := BuildEnvironment(o)
	for i := range e.Mask {
		if e.get("ocean", i) > 0 {
			t.Fatal("regional boundary invented ocean", i)
		}
	}
	for i := range e.Mask {
		e.set("precipitation", i, 1800)
		e.set("moisture", i, .9)
		e.set("aridity", i, 0)
		e.set("mountainCore", i, 0)
		e.set("glacier", i, 0)
	}
	e.prepareHydrology()
	e.hydrology()
	e.finishHydrology()
	longest := 0.
	for _, r := range e.Hydrology.Rivers {
		longest = math.Max(longest, r.Length)
		if r.Length > 45 && len(r.Tributaries) == 0 {
			t.Fatal("trunk has no tributaries")
		}
	}
	if longest < 45 {
		t.Fatal("no long rain-fed lowland river", longest)
	}
	if d := e.ValidateHydrology(); len(d) > 0 {
		t.Fatal(d[:min(4, len(d))])
	}
	external := false
	for _, r := range e.Hydrology.Rivers {
		if r.Downstream == e.hydroID("external-drainage", r.Mouth) {
			external = true
		}
	}
	if !external {
		t.Fatal("regional outflow lost its explicit continuation")
	}
}
func TestFluvialEvolutionDeterminismBudgetAndStoredHistory(t *testing.T) {
	o := realismOptions(t, `{"columns":48,"rows":32,"seed":"erosion-basins","rainfall":2,"erosion":{"iterations":2,"duration_ma":4,"strength":1}}`)
	a, b := BuildEnvironment(o), BuildEnvironment(o)
	if a.Geomorphology == nil || a.Geomorphology.ErodedM3 <= 0 || len(a.Geomorphology.Landforms) == 0 {
		t.Fatal("erosion did not affect the surface")
	}
	if !reflect.DeepEqual(a.Geomorphology, b.Geomorphology) || !reflect.DeepEqual(a.Fields["elevation"], b.Fields["elevation"]) {
		t.Fatal("erosion is not deterministic")
	}
	if err := a.ValidateGeographicData(); err != nil {
		t.Fatal(err)
	}
	if d := a.ValidateHydrology(); len(d) > 0 {
		t.Fatal(d[:min(3, len(d))])
	}
	for _, f := range a.Geomorphology.Landforms {
		if f.Incision > 0 && a.get("fluvialIncision", f.Cell) != f.Incision {
			t.Fatal("history differs from physical incision")
		}
	}
	raw, _ := json.Marshal(a)
	var restored Environment
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.Geography, restored.Geography) || !reflect.DeepEqual(a.Geomorphology, restored.Geomorphology) {
		t.Fatal("history regenerated on decode")
	}
	if err := restored.ValidateGeographicData(); err != nil {
		t.Fatal(err)
	}
	restored.Geography.CellAreaKM2[0] *= 2
	if restored.ValidateGeographicData() == nil {
		t.Fatal("invalid saved spherical area accepted")
	}
}
func TestPhysicalRiverControlsAndDryCatchments(t *testing.T) {
	o := realismOptions(t, `{"columns":32,"rows":16,"seed":"selective-water"}`)
	e := BuildEnvironment(o)
	found := -1
	for i := range e.Mask {
		if e.supportedRiverHeadwater(i) {
			found = i
			break
		}
	}
	if found < 0 {
		t.Fatal("fixture has no water supply")
	}
	e.set("mountainCore", found, 0)
	e.set("glacier", found, 0)
	if !e.supportedRiverHeadwater(found) {
		t.Fatal("mountain-only headwater gate remains")
	}
	e.Options.RiverOptions = &RiverOptions{MinAreaKM2: 1e12}
	if e.supportedRiverHeadwater(found) {
		t.Fatal("catchment setting ignored")
	}
	e.Options.RiverOptions = nil
	e.set("dischargeM3s", found, 0)
	if e.supportedRiverHeadwater(found) {
		t.Fatal("dry catchment became a river")
	}
}
