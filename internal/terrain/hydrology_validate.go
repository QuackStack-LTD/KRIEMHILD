package terrain

import (
	"fmt"
	"math"
)

// Diagnostics identify stable objects/cells and survive in the project. The
// validator is independent of rendering and also runs when importing a world.
func (e *Environment) ValidateHydrology() []HydroDiagnostic {
	out := []HydroDiagnostic{}
	if e.Hydrology == nil {
		return out
	}
	add := func(id, reason string) {
		out = append(out, HydroDiagnostic{Object: id, Reason: reason, Severity: "error"})
	}
	n := len(e.Mask)
	for _, name := range HydrologyFields {
		if len(e.Fields[name]) != n {
			add("hydrology", "missing field "+name)
			return out
		}
	}
	f := e.get
	if e.Hydrology.NetworkVersion > 0 {
		for _, name := range []string{"drainageDistance", "riverClass", "riverSystem"} {
			if len(e.Fields[name]) != n {
				add("hydrology", "missing field "+name)
				return out
			}
		}
	}
	if e.Hydrology.Version != 1 {
		add("hydrology", "unsupported simulation schema")
	}
	coverage := make([]bool, n)
	for _, body := range e.WaterBodies {
		members := map[int]bool{}
		for _, i := range body.Cells {
			if i < 0 || i >= n {
				add(e.hydroID("water", body.ID), "water cell outside world")
				continue
			}
			if coverage[i] || int(f("waterBody", i)) != body.ID {
				add(e.hydroID("water", body.ID), "water footprint disagrees with stored body")
			}
			coverage[i] = true
			members[i] = true
		}
		if len(members) == 0 {
			continue
		}
		root := -1
		for _, i := range body.Cells {
			if members[i] {
				root = i
				break
			}
		}
		delete(members, root)
		cells := []int{root}
		for head := 0; head < len(cells); head++ {
			for _, j := range nb(cells[head], e.Options.Columns, e.Options.Rows) {
				if members[j] {
					delete(members, j)
					cells = append(cells, j)
				}
			}
		}
		if len(members) > 0 {
			add(e.hydroID("water", body.ID), "disconnected water body")
		}
	}
	for i := 0; i < n; i++ {
		if f("waterBody", i) > 0 && !coverage[i] {
			add(e.hydroID("cell", i), "water has no registered body")
		}
	}
	degree := make([]int, n)
	valid := true
	for i := 0; i < n; i++ {
		j := int(f("flow", i))
		if j < -1 || j >= n || j == i {
			add(e.hydroID("cell", i), "invalid downstream cell")
			valid = false
			continue
		}
		if j >= 0 {
			degree[j]++
			a, b := f("elevation", i), f("elevation", j)
			if f("waterBody", i) > 0 {
				a = f("waterLevel", i)
			}
			if f("waterBody", j) > 0 {
				b = f("waterLevel", j)
			}
			if b > a+.02 {
				add(e.hydroID("cell", i), "natural flow climbs the actual terrain or water surface")
			}
		}
		if f("ocean", i) > 0 && (f("elevation", i) > 0 || f("waterLevel", i) != 0 || f("waterBody", i) != 1) {
			add(e.hydroID("cell", i), "marine water disagrees with sea level")
		}
		if f("ocean", i) > 0 && j >= 0 {
			add(e.hydroID("cell", i), "ocean cannot be a natural river source")
		}
	}
	if !valid {
		return out
	}
	q := []int{}
	for i, d := range degree {
		if d == 0 {
			q = append(q, i)
		}
	}
	for head := 0; head < len(q); head++ {
		if j := int(f("flow", q[head])); j >= 0 {
			degree[j]--
			if degree[j] == 0 {
				q = append(q, j)
			}
		}
	}
	if len(q) != n {
		for i, d := range degree {
			if d > 0 {
				add(e.hydroID("cell", i), "cycle in drainage network")
				break
			}
		}
	}
	ids := map[string]bool{}
	for _, body := range e.WaterBodies {
		ids[e.hydroID("water", body.ID)] = true
	}
	for i := 0; i < n; i++ {
		if f("terminalBasin", i) > 0 {
			ids[e.hydroID("terminal-basin", i)] = true
		}
	}
	reaches := map[string]HydroReach{}
	lakeFed := map[int]bool{}
	for _, reach := range e.Hydrology.Reaches {
		for at, steps := reach.To, 0; at >= 0 && at < n && steps < n && f("lake", at) > 0; steps++ {
			lakeFed[int(f("waterBody", at))] = true
			at = int(f("flow", at))
		}
	}
	for _, r := range e.Hydrology.Reaches {
		if r.ID == "" {
			add("hydrology", "river has no persistent ID")
		}
		if ids[r.ID] {
			add(r.ID, "duplicate hydrological identity")
		}
		ids[r.ID] = true
		reaches[r.ID] = r
	}
	for _, r := range e.Hydrology.Reaches {
		if r.From < 0 || r.To < 0 || r.From >= n || r.To >= n {
			add(r.ID, "reach outside world")
			continue
		}
		if int(f("flow", r.From)) != r.To {
			add(r.ID, "reach disagrees with drainage route")
		}
		if f("waterBody", r.From) > 0 || f("ocean", r.From) > 0 {
			add(r.ID, "river starts inside marine or lake water instead of a land channel or shoreline outlet")
		}
		if r.Downstream != e.waterRef(r.To) {
			add(r.ID, "destination identity disagrees with the receiving terrain")
		}
		if e.Hydrology.NetworkVersion >= 2 && len(r.Upstream) == 0 {
			if r.Source == "lake-outlet" {
				supplied := false
				for _, i := range e.drainageNeighbors(r.From) {
					if int(f("flow", i)) == r.From && f("lake", i) > 0 && lakeFed[int(f("waterBody", i))] && f("accumulation", i) > f("hydroLoss", i) {
						supplied = true
					}
				}
				if !supplied {
					add(r.ID, "lake outlet lacks a connected inlet and positive outflow")
				}
			} else if !e.supportedRiverHeadwater(r.From) {
				add(r.ID, "river has no supported mountain or glacier headwater")
			}
		}
		if !ids[r.Downstream] {
			add(r.ID, "destination is missing")
		}
		if receiving, ok := reaches[r.Downstream]; ok {
			connected := false
			for _, id := range receiving.Upstream {
				if id == r.ID {
					connected = true
				}
			}
			if !connected {
				add(r.ID, "receiving river omits this tributary")
			}
		}
		fraction := 0.
		for _, branch := range r.Branches {
			fraction += branch.Fraction
			if branch.ID == "" || branch.To < 0 || branch.To >= n || !ids[branch.Downstream] || branch.Fraction <= 0 {
				add(r.ID, "invalid distributary connection")
				continue
			}
			if f("waterBody", branch.To) == 0 || f("waterLevel", branch.To) > f("elevation", r.From) {
				add(branch.ID, "distributary has no downhill receiving water")
			}
		}
		if fraction >= 1 {
			add(r.ID, "distributaries over-allocate parent discharge")
		}
		if !finite(r.Discharge) || r.Discharge <= 0 || !finite(r.Width) || r.Width <= 0 || r.DryDischarge < 0 || r.DryDischarge > r.Discharge+.001 {
			add(r.ID, "invalid discharge or channel dimensions")
		}
		minimumSupply := 2.
		if e.Hydrology.NetworkVersion > 0 {
			minimumSupply = .35 * math.Max(1, float64(n)/16000)
		}
		if len(r.Upstream) == 0 && r.Source == "headwater-catchment" && (f("catchmentArea", r.From) < 3 || f("accumulation", r.From) < minimumSupply-1e-6) {
			add(r.ID, "unsupported headwater catchment")
		}
		for _, id := range r.Upstream {
			u, ok := reaches[id]
			if !ok || u.To != r.From || u.Downstream != r.ID {
				add(r.ID, "tributary does not connect to its receiving reach")
			}
		}
	}
	for _, b := range e.Hydrology.Basins {
		if b.Outlet >= 0 && !ids[b.Downstream] {
			add(b.ID, "lake outlet has no receiving water system")
		}
		for _, id := range b.Inlets {
			if _, ok := reaches[id]; !ok {
				add(b.ID, "lake inlet references a missing river")
			}
		}
		if b.Level < b.Floor-.01 || b.Level > b.Spill+.01 {
			add(b.ID, "lake level outside its basin")
		}
		for _, v := range []float64{b.Budget.Inflow, b.Budget.Rain, b.Budget.Evaporation, b.Budget.Infiltration, b.Budget.Outflow} {
			if !finite(v) || v < 0 {
				add(b.ID, "invalid water balance")
				break
			}
		}
		residual := b.Budget.Inflow + b.Budget.Rain - b.Budget.Evaporation - b.Budget.Infiltration - b.Budget.Outflow
		if math.Abs(residual) > math.Max(.03, (b.Budget.Inflow+b.Budget.Rain)*.03) {
			add(b.ID, fmt.Sprintf("annual water balance does not close: %.4f", residual))
		}
		if b.WaterBody > 0 && b.Budget.Inflow+b.Budget.Rain <= 0 {
			add(b.ID, "lake has no water supply")
		}
		if b.Outlet >= 0 && (b.Outlet >= n || int(f("flow", b.Outlet)) < 0 || b.Level < b.Spill-.01) {
			add(b.ID, "outlet below spill elevation or disconnected")
		}
	}
	for _, sp := range e.Hydrology.Springs {
		if sp.Cell < 0 || sp.Cell >= n {
			add(sp.ID, "spring outside world")
			continue
		}
		if f("groundFlow", sp.Cell) <= .075 || sp.Discharge <= 0 || sp.Discharge > f("accumulation", sp.Cell)+.001 || sp.Origin == "" {
			add(sp.ID, "spring lacks supplied aquifer/geological emergence")
		}
		if !ids[sp.Downstream] {
			add(sp.ID, "spring has no receiving water system")
		}
		if sp.Temperature > math.Max(4, f("temperature", sp.Cell))+10 && f("geothermal", sp.Cell) < 40 {
			add(sp.ID, "hot spring lacks geothermal support")
		}
	}
	for _, wet := range e.Hydrology.Wetlands {
		if wet.Source == "" {
			add(wet.ID, "wetland has no water source")
		}
		for _, i := range wet.Cells {
			if i < 0 || i >= n || f("wetland", i) < .6 || f("slope", i) > .1 {
				add(wet.ID, "wetland lacks saturation or suitable slope")
				break
			}
		}
	}
	allocated := map[string]float64{}
	for _, c := range e.Hydrology.Canals {
		if c.Purpose == "" || c.Destination == "" || !ids[c.Source] || len(c.Path) < 2 || !finite(c.Supply) || c.Supply <= 0 {
			add(c.ID, "engineered canal lacks purpose, supply, or endpoints")
			continue
		}
		allocated[c.Source] += c.Supply
		engineering := map[int]bool{}
		pathCells := map[int]bool{}
		for _, i := range c.Path {
			pathCells[i] = true
		}
		for _, sites := range [][]int{c.Locks, c.Pumps, c.Aqueducts} {
			for _, i := range sites {
				if i < 0 || i >= n || !pathCells[i] {
					add(c.ID, "engineering site is outside the canal route")
				}
				engineering[i] = true
			}
		}
		for k, i := range c.Path {
			if i < 0 || i >= n {
				add(c.ID, "canal outside world")
				break
			}
			previous := 0.
			if k > 0 {
				previous = f("elevation", c.Path[k-1])
				if f("waterBody", c.Path[k-1]) > 0 {
					previous = f("waterLevel", c.Path[k-1])
				}
			}
			if k > 0 && f("elevation", i) > previous+.01 && !engineering[i] {
				add(c.ID, "canal crosses elevation without engineering")
			}
		}
	}
	for id, amount := range allocated {
		if r, ok := reaches[id]; ok && amount > r.Discharge {
			add(id, "canals over-allocate available discharge")
		}
	}
	dry := 0
	for _, m := range e.Mask {
		if m != 0 {
			dry++
		}
	}
	if len(e.Hydrology.Reaches) > dry/2 {
		add("hydrology", "visible stream density exceeds half the land cells")
	}
	if e.Hydrology.NetworkVersion > 0 {
		e.validateRiverSystems(add)
	}
	return out
}

func (e *Environment) validateRiverSystems(add func(string, string)) {
	h := e.Hydrology
	if (h.NetworkVersion < 1 || h.NetworkVersion > 2) || h.Statistics == nil {
		add("hydrology", "missing or unsupported river-system summary")
		return
	}
	byID := map[string]HydroReach{}
	valid := map[string]bool{}
	owned := map[string]bool{}
	rivers := map[string]RiverSystem{}
	for _, r := range h.Reaches {
		byID[r.ID] = r
	}
	for _, r := range h.Rivers {
		if valid[r.ID] {
			add(r.ID, "duplicate river-system identity")
		}
		valid[r.ID] = true
		rivers[r.ID] = r
	}
	for _, b := range e.WaterBodies {
		valid[e.hydroID("water", b.ID)] = true
	}
	for i := range e.Mask {
		if e.get("terminalBasin", i) > 0 {
			valid[e.hydroID("terminal-basin", i)] = true
		}
	}
	for _, r := range h.Rivers {
		if !valid[r.Downstream] || r.Downstream == r.ID {
			add(r.ID, "river system has no valid destination")
		}
		if len(r.Reaches) == 0 || !finite(r.Length) || r.Length <= 0 || r.Class != "major" && r.Class != "regional" && r.Class != "stream" {
			add(r.ID, "invalid river system extent or class")
		}
		length := 0.
		for k, id := range r.Reaches {
			reach, ok := byID[id]
			if !ok || owned[id] || reach.RiverID != r.ID || reach.Class != r.Class {
				add(r.ID, "river system has missing, duplicated or mismatched reaches")
				continue
			}
			if reach.From < 0 || reach.From >= len(e.Mask) || reach.To < 0 || reach.To >= len(e.Mask) {
				add(r.ID, "river system reach is outside world")
				continue
			}
			owned[id] = true
			length += e.hydroStep(reach.From, reach.To)
			if k == 0 && reach.From != r.Source || k == len(r.Reaches)-1 && reach.To != r.Mouth {
				add(r.ID, "river endpoints disagree with reach geometry")
			}
			if k > 0 {
				prev := byID[r.Reaches[k-1]]
				at := prev.To
				for steps := 0; at >= 0 && at != reach.From && steps < len(e.Mask); steps++ {
					at = int(e.get("flow", at))
				}
				if at != reach.From {
					add(r.ID, "river segments are not connected by drainage")
				}
			}
		}
		if math.Abs(length-r.Length) > .001 {
			add(r.ID, "river length disagrees with stored reaches")
		}
		for _, id := range r.Tributaries {
			child, ok := rivers[id]
			if !ok || child.Downstream != r.ID {
				add(r.ID, "river system has disconnected tributary")
			}
		}
	}
	if len(owned) != len(h.Reaches) {
		add("hydrology", "river reaches missing from river-system hierarchy")
	}
	if h.Statistics.InvalidDestinations != 0 {
		add("hydrology", "summary reports invalid river destinations")
	}
}
