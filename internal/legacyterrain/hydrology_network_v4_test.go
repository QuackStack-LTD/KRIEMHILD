package terrain

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestDistributedHillslopeRunoffConservesAndConverges(t *testing.T) {
	o := realismOptions(t, `{"columns":32,"rows":16,"erosion":{"iterations":0},"geography":{"map_coverage":"local","world_scale":1}}`)
	e := BuildEnvironment(o)
	rank := make([]int, len(e.Mask))
	for i := range e.Mask {
		x, y := float64(i%32), float64(i/32)
		e.set("elevation", i, 1000-x*3+math.Pow(y-7.5, 2)*2)
		e.set("waterBody", i, 0)
		e.set("runoff", i, 1)
		rank[i] = i
	}
	e.routeWatersheds(rank, func(i int) float64 { return e.get("elevation", i) })
	total, spread := 0., false
	for i := range e.Mask {
		q := e.get("hillslopeRunoff", i)
		if math.Abs(q-math.Round(q)) > .01 {
			spread = true
		}
		if e.get("flow", i) < 0 {
			total += q
		}
		if j := int(e.get("flow", i)); j >= 0 && e.get("elevation", j) > e.get("elevation", i) {
			t.Fatal("uphill extraction")
		}
	}
	if !spread || math.Abs(total-float64(len(e.Mask))) > .01 {
		t.Fatal("MFD must split and conserve water", spread, total)
	}
	e.accumulateWater()
	if e.get("catchmentArea", 7*32+31) < float64(len(e.Mask))*.35 {
		t.Fatal("valley failed to collect hillslope drainage")
	}
	a := append([]float64(nil), e.Fields["flow"]...)
	e.routeWatersheds(rank, func(i int) float64 { return e.get("elevation", i) })
	if !reflect.DeepEqual(a, e.Fields["flow"]) {
		t.Fatal("nondeterministic extraction")
	}
}

func branchTestWorld(t *testing.T) *Environment {
	e := BuildEnvironment(realismOptions(t, `{"columns":16,"rows":16,"erosion":{"iterations":0},"geography":{"map_coverage":"local","world_scale":0.1}}`))
	for i := range e.Mask {
		e.Mask[i] = 1
		for _, name := range []string{"waterBody", "lake", "ocean", "waterLevel"} {
			e.set(name, i, 0)
		}
		e.set("elevation", i, 1000)
		e.set("flow", i, -1)
		e.set("sediment", i, .8)
	}
	e.Hydrology = &HydrologyState{Version: 1, NetworkVersion: 4}
	return e
}

func TestStoredBranchesSplitRejoinAndConserve(t *testing.T) {
	for _, kind := range []string{"delta", "braid", "inland"} {
		t.Run(kind, func(t *testing.T) {
			e := branchTestWorld(t)
			from, to := 5*16+7, 6*16+8
			e.set("elevation", from, 20)
			e.set("elevation", to, 10)
			e.set("flow", from, float64(to))
			e.set("flow", to, float64(to+16))
			if kind != "braid" {
				e.set("elevation", from, 8)
				e.set("elevation", to, -1)
				e.set("waterLevel", to, 0)
				e.set("waterBody", to, 1)
				e.set("ocean", to, 1)
			}
			for _, i := range []int{5*16 + 8, 6*16 + 7} {
				e.set("elevation", i, 14)
			}
			if kind != "braid" {
				for _, i := range []int{5*16 + 8, 6*16 + 7} {
					e.set("elevation", i, 5)
				}
				for _, i := range []int{5*16 + 9, 7*16 + 7} {
					e.set("elevation", i, -1)
					e.set("waterBody", i, 1)
					e.set("ocean", i, 1)
				}
			}
			if kind == "inland" {
				for i := range e.Mask {
					if e.get("waterBody", i) > 0 {
						e.set("lake", i, 1)
						e.set("ocean", i, 0)
					}
				}
				e.set("aridity", from, .8)
			}
			r := HydroReach{ID: e.riverID(from), From: from, To: to, Discharge: 100, DischargeM3s: 1000, ValleyWidthMetres: 500}
			e.Hydrology.Reaches = []HydroReach{r}
			e.buildHydroBranches()
			r = e.Hydrology.Reaches[0]
			if len(r.Branches) < 2 {
				t.Fatal("expected multiple connected side channels", kind, r.Branches)
			}
			fraction := 0.
			for _, b := range r.Branches {
				e.validateHydroBranch(r, b, func(id, reason string) { t.Error(id, reason) })
				fraction += b.Fraction
				if kind == "braid" && b.To != to {
					t.Fatal("braid never rejoins")
				}
			}
			q := r.Discharge * (1 - fraction)
			for _, b := range r.Branches {
				q += b.Discharge
			}
			if math.Abs(q-r.Discharge) > 1e-9 {
				t.Fatal("split budget")
			}
			raw, _ := json.Marshal(e.Hydrology)
			var loaded HydrologyState
			_ = json.Unmarshal(raw, &loaded)
			if !reflect.DeepEqual(*e.Hydrology, loaded) {
				t.Fatal("branch geometry or discharge lost on persistence")
			}
			b := r.Branches[0]
			b.Discharge *= 2
			errors := 0
			e.validateHydroBranch(r, b, func(string, string) { errors++ })
			if errors == 0 {
				t.Fatal("invalid discharge undetected")
			}
		})
	}
}

func TestPhysicalWidthsDoNotCollapseOnPlanetaryGrid(t *testing.T) {
	previous := 0.
	for _, q := range []float64{.1, 1, 10, 100, 1000, 10000, 100000} {
		width := physicalChannelWidth(q, .0005, .5)
		if width <= previous*2 {
			t.Fatal("width distribution collapsed", q, width, previous)
		}
		previous = width
	}
	if physicalChannelWidth(100, .05, .2) >= physicalChannelWidth(100, .0001, .8) {
		t.Fatal("steep confined reach not narrower")
	}
}

func TestErosionConvergenceAndIndependentValleyWidth(t *testing.T) {
	e := BuildEnvironment(realismOptions(t, `{"columns":32,"rows":16,"seed":"feedback","erosion":{"iterations":4,"duration_ma":0.0001,"strength":0.001,"tolerance_metres":1}}`))
	g := e.Geomorphology
	if !g.Converged || g.Iterations >= 4 || len(g.MaxChanges) != g.Iterations || len(g.RoutingChanges) != g.Iterations {
		t.Fatal("feedback did not stop on stable terrain and drainage", g.Iterations, g.Converged)
	}
	for i := range e.Mask {
		slope := 0.
		for _, j := range e.neighbors(i) {
			slope = math.Max(slope, math.Abs(e.get("elevation", i)-e.get("elevation", j))/2000)
		}
		if math.Abs(e.get("slope", i)-clamp(slope, 0, 1)) > 1e-6 {
			t.Fatal("stale gradient", i)
		}
	}
	f := DetailFeature{Path: [][3]float64{{0, 0, 20}, {1, 0, 10}}, Width: .001, ValleyWidth: .15, Discharge: 100}
	a := SampleChannel(f, .5, .02)
	f.Width *= 10
	b := SampleChannel(f, .5, .02)
	if a.Valley != b.Valley || a.Width == b.Width {
		t.Fatal("valley extent coupled to channel width")
	}
}

func TestSubcellBraidUsesSavedJunctionsAndTerrainCorridor(t *testing.T) {
	e := branchTestWorld(t)
	for i := range e.Mask {
		e.set("elevation", i, 30-float64(i%16))
		e.set("drainageElevation", i, e.get("elevation", i))
	}
	main := make([][3]float64, 25)
	for k := range main {
		x := 5 + float64(k)/24
		main[k] = [3]float64{x, 5, 30 - x}
	}
	m := &DetailModel{World: e, Seed: 1}
	path := m.routeFloodplainChannel(main, 2, 22, .03, 3)
	if len(path) != 21 {
		t.Fatal("flat sedimentary corridor failed to route a braid")
	}
	from, to := 5*16+5, 5*16+6
	r := HydroReach{ID: e.riverID(from), From: from, To: to, ChannelPath: main, Width: .01, WidthMetres: 10, ValleyWidthMetres: 300, Discharge: 100, DischargeM3s: 500}
	b := HydroBranch{ID: r.ID + "/loop", Kind: "braid", Path: path, SplitNode: 2, JoinNode: 22, Downstream: r.ID, To: to, Fraction: .25, Discharge: 25, DischargeM3s: 125}
	r.Branches = []HydroBranch{b}
	e.Hydrology.Reaches = []HydroReach{r}
	e.validateCorridorBranch(r, b, func(id, reason string) { t.Fatal(id, reason) })
	widths := make([]float64, len(main))
	for k := range widths {
		widths[k] = r.Width
	}
	m.Features = []DetailFeature{{ID: r.ID, Kind: "river", Path: main, Widths: widths, Width: r.Width, Discharge: r.Discharge}}
	m.refineHydroFeatures()
	if len(m.Features) != 2 {
		t.Fatal("subcell network lost a channel")
	}
	trunk, branch := m.Features[0], m.Features[1]
	if branch.Path[0] != trunk.Path[2] || branch.Path[len(branch.Path)-1] != trunk.Path[22] {
		t.Fatal("rendered braid disconnected")
	}
	if trunk.DischargesM3s[0] != 500 || trunk.DischargesM3s[12]+branch.DischargeM3s != 500 || trunk.DischargesM3s[24] != 500 {
		t.Fatal("split/rejoin did not conserve water locally")
	}
	// A real ridge in the corridor prevents routing; it cannot be hidden by
	// drawing an offset ribbon over it.
	for i := range e.Mask {
		e.set("elevation", i, 1000)
	}
	if len(m.routeFloodplainChannel(main, 2, 22, .03, 3)) != 0 {
		t.Fatal("side channel crosses unsupported relief")
	}
}

func TestCrossingDrainageFacetsBecomeConfluence(t *testing.T) {
	e := branchTestWorld(t)
	a, b, c, d := 5*16+5, 5*16+6, 6*16+5, 6*16+6
	for i, z := range map[int]float64{a: 10, b: 9, c: 8, d: 7} {
		e.set("elevation", i, z)
	}
	e.set("flow", a, float64(d))
	e.set("flow", b, float64(c))
	e.joinCrossingFacets(make([]int, len(e.Mask)), func(i int) float64 { return e.get("elevation", i) })
	if int(e.get("flow", a)) == d && int(e.get("flow", b)) == c {
		t.Fatal("independent diagonals still cross")
	}
	for _, i := range []int{a, b} {
		j := int(e.get("flow", i))
		if e.get("elevation", j) > e.get("elevation", i) {
			t.Fatal("confluence climbs")
		}
	}
}

func TestNumericalBreachOpensDiagonalSaddle(t *testing.T) {
	e := branchTestWorld(t)
	source, a, b, outlet := 5*16+5, 5*16+6, 6*16+5, 6*16+6
	for i, z := range map[int]float64{source: 100, a: 100.2, b: 100.1, outlet: 99} {
		e.set("elevation", i, z)
	}
	e.set("accumulation", source, 10)
	e.set("flow", source, float64(outlet))
	filled := append([]float64(nil), e.Fields["elevation"]...)
	filled[source] = 100.201
	e.breachNumericalPits(filled)
	if e.get("elevation", b) >= e.get("elevation", source) || e.get("drainageBreach", b) <= 0 {
		t.Fatal("tiny corner sill still blocks the continental trunk")
	}
	if e.get("elevation", a) != f32(100.2) {
		t.Fatal("unnecessary ridge erosion")
	}
}
