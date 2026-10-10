package terrain

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestNaturalStagesDeterministicAndConsistent(t *testing.T) {
	for _, seed := range []string{"KRIEMHILD", "resource-history", "island-arc", "fossil-sea"} {
		t.Run(seed, func(t *testing.T) {
			o := realismOptions(t, `{"columns":96,"rows":64}`)
			o.Seed = seed
			e := BuildEnvironment(o)
			if err := e.ValidateNaturalData(); err != nil {
				t.Fatal(err)
			}
			if len(e.Geology.Formations) <= len(e.Geology.Provinces) || len(e.Geology.Structures) == 0 || len(e.Resources.Occurrences) == 0 {
				t.Fatal("missing geological structure or resources")
			}
			before, _ := json.Marshal(e.Fields)
			beforeHydro, _ := json.Marshal(e.Hydrology)
			geology, _ := json.Marshal(e.Geology)
			resources, _ := json.Marshal(e.Resources)
			e.BuildGeologicalHistory()
			if e.Resources != nil {
				t.Fatal("old resources survived replacement geology")
			}
			e.BuildNaturalResources()
			after, _ := json.Marshal(e.Fields)
			afterHydro, _ := json.Marshal(e.Hydrology)
			g, _ := json.Marshal(e.Geology)
			r, _ := json.Marshal(e.Resources)
			if string(before) != string(after) || string(beforeHydro) != string(afterHydro) {
				t.Fatal("history changed established terrain or hydrology")
			}
			if string(geology) != string(g) || string(resources) != string(r) {
				t.Fatal("geology/resources are not deterministic")
			}
			var restored Environment
			data, _ := json.Marshal(e)
			if err := json.Unmarshal(data, &restored); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(e.Geology, restored.Geology) || !reflect.DeepEqual(e.Resources, restored.Resources) {
				t.Fatal("natural data did not round trip")
			}
			types := map[string]int{}
			for _, o := range e.Resources.Occurrences {
				types[o.Type]++
			}
			t.Logf("%d provinces, %d formations, %d faults, %d occurrences across %d goods", len(e.Geology.Provinces), len(e.Geology.Formations), len(e.Geology.Structures), len(e.Resources.Occurrences), len(types))
		})
	}
}

func TestNaturalQueryOverlapsNearbyAndLegacy(t *testing.T) {
	o := realismOptions(t, `{"columns":64,"rows":48,"seed":"overlapping-goods"}`)
	e := BuildEnvironment(o)
	index := NewNaturalIndex(e)
	var cell int
	for i, c := range e.Geology.Cells {
		if c.Cover >= 0 && len(index.cells[i]) > 2 {
			cell = i
			break
		}
	}
	x, y := float64(cell%o.Columns), float64(cell/o.Columns)
	q := index.Query(x, y, 2, false)
	if !q.Available || len(q.Geology) < 3 || len(q.Goods) < 2 {
		t.Fatal("lost overlapping layers", q)
	}
	seen := map[string]bool{}
	near := false
	lastDistance := -1.
	for _, g := range q.Goods {
		if seen[g.ID] {
			t.Fatal("duplicate occurrence")
		}
		seen[g.ID] = true
		if g.Location == "nearby" {
			near = true
		}
		if near && g.Location == "here" {
			t.Fatal("nearby before local")
		}
		if g.Distance < lastDistance {
			t.Fatal("distance ranking unstable")
		}
		lastDistance = g.Distance
	}
	if !near {
		t.Fatal("fixture needs nearby resources")
	}
	local := index.Query(x, y, 0, false)
	for _, g := range local.Goods {
		if g.Distance != 0 || g.Location != "here" {
			t.Fatal("radius zero returns nearby deposit")
		}
	}
	old := *e
	old.Geology = nil
	old.Resources = nil
	if q := NewNaturalIndex(&old).Query(x, y, 2, false); q.Available || len(q.Goods) > 0 {
		t.Fatal("legacy world regenerated resources")
	}
	// A biome with no occurrence must never invent one on hover.
	empty := *e
	empty.Resources = &ResourceState{Version: 1}
	if q := NewNaturalIndex(&empty).Query(x, y, 20, false); len(q.Goods) > 0 {
		t.Fatal("hover inferred resources from suitability")
	}
}

func TestNaturalResourcePrerequisitesAndValidation(t *testing.T) {
	o := realismOptions(t, `{"columns":96,"rows":64,"seed":"resource-history"}`)
	e := BuildEnvironment(o)
	contexts := e.resourceContexts()
	checked := map[string]bool{}
	for _, occ := range e.Resources.Occurrences {
		if occ.Variant == "placer" {
			continue
		}
		i := 0
		if occ.Geometry.Kind == "point" {
			i = int(occ.Geometry.Point[1])*o.Columns + int(occ.Geometry.Point[0])
		} else {
			i = occ.Geometry.Cells[0]
		}
		c := contexts[i]
		if got := resourceEligibility(occ.Type, c); got.Score <= 0 || got.Variant != occ.Variant {
			t.Fatal("unsupported resource", occ.Type)
		}
		checked[occ.Type] = true
		if occ.Type == "coal" {
			c.p.History.AncientWetland = false
			if resourceEligibility("coal", c).Score != 0 {
				t.Fatal("coal without ancient wetlands")
			}
		}
		if occ.Type == "crude-oil" {
			c.p.History.Trap = ""
			if resourceEligibility("crude-oil", c).Score != 0 {
				t.Fatal("oil without a trap")
			}
		}
		if occ.Type == "halite" {
			c.p.History.Evaporative = false
			if resourceEligibility("halite", c).Score != 0 {
				t.Fatal("salt without evaporative history")
			}
		}
	}
	for _, id := range []string{"coal", "crude-oil", "halite", "groundwater-aquifer", "timber", "solar-energy", "wind-energy"} {
		if !checked[id] {
			t.Fatal("missing resource rule coverage", id)
		}
	}
	copyJSON := func() *Environment {
		data, _ := json.Marshal(e)
		var c Environment
		json.Unmarshal(data, &c)
		return &c
	}
	bad := copyJSON()
	bad.Resources.Occurrences[0].References[0].ID = "missing"
	if bad.ValidateNaturalData() == nil {
		t.Fatal("missing causal reference accepted")
	}
	bad = copyJSON()
	bad.Geology.Provinces[0].Events[1].AgeMa = 5000
	if bad.ValidateNaturalData() == nil {
		t.Fatal("backwards chronology accepted")
	}
	bad = copyJSON()
	bad.Geology.Provinces[0].Geometry.Cells = bad.Geology.Provinces[0].Geometry.Cells[1:]
	if bad.ValidateNaturalData() == nil {
		t.Fatal("missing reciprocal membership accepted")
	}
	bad = copyJSON()
	bad.Resources.Occurrences[0].Properties = map[string]float64{"temperatureC": math.Inf(1)}
	if bad.ValidateNaturalData() == nil {
		t.Fatal("nonfinite resource property accepted")
	}
	bad = copyJSON()
	bad.Resources.Potential["wind-energy"][0] = math.NaN()
	if bad.ValidateNaturalData() == nil {
		t.Fatal("nonfinite potential accepted")
	}
}

func TestNaturalStagesRequireTheirPredecessors(t *testing.T) {
	e := &Environment{}
	e.BuildNaturalResources()
	if e.Resources != nil {
		t.Fatal("resources generated before geology")
	}
	e.BuildGeologicalHistory()
	if e.Geology != nil {
		t.Fatal("history generated before terrain/hydrology")
	}
}

func TestNaturalPlacerTransportUsesActualUpstreamDeposit(t *testing.T) {
	e := &Environment{Options: EnvironmentOptions{Columns: 4, Rows: 2, Seed: "placer-test"}, Mask: make([]int, 8), Fields: map[string][]float64{}, Geology: &GeologicalState{Cells: make([]GeologicalCell, 8), Formations: []GeologicalFormation{{ID: "bedrock"}}}, Resources: &ResourceState{}}
	e.Fields["ocean"] = make([]float64, 8)
	e.Fields["flow"] = []float64{1, 2, 3, -1, 5, 6, 7, -1}
	e.Fields["river"] = []float64{0, 1, 1, 0, 0, 1, 1, 0}
	e.Fields["catchmentArea"] = []float64{1, 5, 10, 11, 1, 5, 10, 11}
	e.Fields["slope"] = []float64{.2, .02, .01, 0, .2, .02, .01, 0}
	e.Geology.Cells[0].Erosion = .8
	e.buildPlacerResources("gold")
	if len(e.Resources.Occurrences) != 0 {
		t.Fatal("placer generated without a source")
	}
	e.Resources.Occurrences = []ResourceOccurrence{{ID: "upstream-gold", Type: "gold", Variant: "primary-vein", Abundance: .8, Geometry: NaturalGeometry{Kind: "region", Cells: []int{0}}}}
	e.buildPlacerResources("gold")
	if len(e.Resources.Occurrences) < 2 {
		t.Fatal("downstream concentration absent")
	}
	for _, o := range e.Resources.Occurrences[1:] {
		if o.Variant != "placer" || o.References[0].ID != "upstream-gold" {
			t.Fatal("missing source identity")
		}
		for _, i := range o.Geometry.Cells {
			if i != 1 && i != 2 {
				t.Fatal("placer crossed a drainage divide", i)
			}
		}
	}
}

func TestNaturalGeothermalAndReefPrerequisites(t *testing.T) {
	e := &Environment{Fields: map[string][]float64{"geothermal": {180}, "groundFlow": {.5}}}
	for _, group := range [][]string{FieldNames, SurfaceFields, HydrologyFields} {
		for _, name := range group {
			if e.Fields[name] == nil {
				e.Fields[name] = []float64{0}
			}
		}
	}
	c := resourceContext{e: e, c: GeologicalCell{Fault: 0}, cover: &GeologicalFormation{Strata: []RockStratum{{Rock: "shale"}}}, spring: &HydroSpring{Temperature: 90, Discharge: .5}}
	if resourceEligibility("geyser", c).Score == 0 {
		t.Fatal("supported geyser rejected")
	}
	c.spring = nil
	if resourceEligibility("geyser", c).Score != 0 || resourceEligibility("hot-spring", c).Score != 0 {
		t.Fatal("invented surface water")
	}
	if resourceEligibility("geothermal-energy", c).Score == 0 {
		t.Fatal("regional potential requires surface spring")
	}
	c.spring = &HydroSpring{Temperature: 90, Discharge: .5}
	c.c.Fault = -1
	if resourceEligibility("geyser", c).Score != 0 {
		t.Fatal("geyser without circulation plumbing")
	}
	for name, value := range map[string]float64{"ocean": 1, "waterDepth": 15, "temperature": 26, "clarity": .9, "salinity": 35, "reef": .8} {
		e.Fields[name] = []float64{value}
	}
	if resourceEligibility("coral", c).Score == 0 {
		t.Fatal("supported reef rejected")
	}
	for _, name := range []string{"ocean", "temperature", "salinity", "clarity", "reef"} {
		original := e.Fields[name][0]
		e.Fields[name][0] = 0
		if resourceEligibility("coral", c).Score != 0 {
			t.Fatal("reef without", name)
		}
		e.Fields[name][0] = original
	}
	e.Fields["waterDepth"][0] = 500
	if resourceEligibility("coral", c).Score != 0 {
		t.Fatal("deep reef-building coral")
	}
}
