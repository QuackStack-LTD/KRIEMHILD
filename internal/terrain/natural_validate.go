package terrain

import (
	"fmt"
	"math"
)

func (e *Environment) ValidateNaturalData() error {
	if e.Geology == nil && e.Resources == nil {
		return nil
	} // portable legacy worlds
	if e.Geology == nil || e.Resources == nil || e.Hydrology == nil {
		return fmt.Errorf("natural layers require terrain, hydrology, geology and resources together")
	}
	g, r := e.Geology, e.Resources
	n, w, h := len(e.Mask), e.Options.Columns, e.Options.Rows
	if g.Version != 1 || r.Version != 1 || r.GeologyVersion != g.Version || len(g.Cells) != n || g.Coordinates == "" || r.Units == "" || len(g.Provinces) == 0 || len(g.Provinces) > n || len(g.Formations) > 3*n || len(g.Structures) > n || len(r.Occurrences) > len(ResourceCatalog)*n {
		return fmt.Errorf("invalid natural-layer schema or dimensions")
	}
	ids := map[string]string{}
	strata := map[string]bool{}
	register := func(id, kind string) error {
		if id == "" || len(id) > 240 || ids[id] != "" {
			return fmt.Errorf("invalid or duplicate %s ID %q", kind, id)
		}
		ids[id] = kind
		return nil
	}
	pointValid := func(p [2]float64) bool {
		return finite(p[0]) && finite(p[1]) && p[0] >= 0 && p[1] >= 0 && p[0] <= float64(w-1) && p[1] <= float64(h-1)
	}
	geometry := func(id string, geo NaturalGeometry) error {
		switch geo.Kind {
		case "region":
			if len(geo.Cells) == 0 || len(geo.Cells) > n || len(geo.Path) > 0 || geo.Point != nil {
				return fmt.Errorf("invalid region %s", id)
			}
			seen := map[int]bool{}
			for _, cell := range geo.Cells {
				if cell < 0 || cell >= n || seen[cell] {
					return fmt.Errorf("invalid cell in %s", id)
				}
				seen[cell] = true
			}
		case "line":
			if len(geo.Path) < 2 || len(geo.Path) > n || len(geo.Cells) > 0 || geo.Point != nil {
				return fmt.Errorf("invalid line %s", id)
			}
			for _, p := range geo.Path {
				if !pointValid(p) {
					return fmt.Errorf("line outside world: %s", id)
				}
			}
		case "point":
			if geo.Point == nil || !pointValid(*geo.Point) || len(geo.Cells) > 0 || len(geo.Path) > 0 {
				return fmt.Errorf("invalid point %s", id)
			}
		default:
			return fmt.Errorf("unknown geometry for %s", id)
		}
		return nil
	}
	membership := make([][4]bool, n) // province, basement, cover, intrusion
	unit := func(v float64) bool { return finite(v) && v >= 0 && v <= 1 }
	for k, p := range g.Provinces {
		if err := register(p.ID, "province"); err != nil {
			return err
		}
		if err := geometry(p.ID, p.Geometry); err != nil {
			return err
		}
		if p.Geometry.Kind != "region" || p.Setting == "" || p.Rock == "" || p.Process == "" || !finite(p.AgeMa) || p.AgeMa < 0 || p.AgeMa > 4600 || !unit(p.History.OrganicPreservation) || !finite(p.History.MaxBurial) || p.History.MaxBurial < 0 || !finite(p.History.MaxTemperature) || p.History.MaxTemperature < 0 {
			return fmt.Errorf("invalid geological history %s", p.ID)
		}
		age := p.AgeMa
		for _, event := range p.Events {
			if !finite(event.AgeMa) || event.AgeMa < 0 || event.AgeMa > age || event.Kind == "" {
				return fmt.Errorf("inconsistent event chronology %s", p.ID)
			}
			age = event.AgeMa
		}
		for _, i := range p.Geometry.Cells {
			if g.Cells[i].Province != k || membership[i][0] {
				return fmt.Errorf("province/cell relationship mismatch %s", p.ID)
			}
			membership[i][0] = true
		}
	}
	for k, f := range g.Formations {
		if err := register(f.ID, "formation"); err != nil {
			return err
		}
		if err := geometry(f.ID, f.Geometry); err != nil {
			return err
		}
		if f.Geometry.Kind != "region" || ids[f.ProvinceID] != "province" || len(f.Strata) == 0 || len(f.Strata) > 32 || !finite(f.AgeMa) || f.AgeMa < 0 || f.Rock == "" || f.Process == "" {
			return fmt.Errorf("invalid formation %s", f.ID)
		}
		age, depth := 0., 0.
		for _, s := range f.Strata {
			if s.ID == "" || strata[s.ID] || s.Rock == "" || s.Environment == "" || !finite(s.AgeMa) || s.AgeMa < age || s.AgeMa > f.AgeMa || !finite(s.Top) || !finite(s.Bottom) || s.Top < depth || s.Bottom <= s.Top || s.Bottom > 50000 || !unit(s.Permeability) || !unit(s.Organic) {
				return fmt.Errorf("inconsistent strata %s", f.ID)
			}
			strata[s.ID] = true
			age, depth = s.AgeMa, s.Bottom
		}
		slot := map[string]int{"basement": 1, "sedimentary-basin": 2, "intrusion": 3}[f.Kind]
		if slot == 0 {
			return fmt.Errorf("unsupported formation kind %s", f.ID)
		}
		for _, i := range f.Geometry.Cells {
			c := g.Cells[i]
			if c.Province < 0 || c.Province >= len(g.Provinces) || g.Provinces[c.Province].ID != f.ProvinceID || f.AgeMa > g.Provinces[c.Province].AgeMa || ([]int{c.Province, c.Basement, c.Cover, c.Intrusion}[slot] != k || membership[i][slot]) {
				return fmt.Errorf("formation/cell relationship mismatch %s", f.ID)
			}
			membership[i][slot] = true
		}
	}
	for _, s := range g.Structures {
		if err := register(s.ID, "structure"); err != nil {
			return err
		}
		if err := geometry(s.ID, s.Geometry); err != nil {
			return err
		}
		if s.Geometry.Kind != "line" || s.Kind != "fault" || !finite(s.AgeMa) || s.AgeMa < 0 {
			return fmt.Errorf("invalid structure %s", s.ID)
		}
	}
	for _, f := range g.Formations {
		for _, id := range f.References {
			if ids[id] == "" {
				return fmt.Errorf("missing formation reference %s", id)
			}
		}
	}
	for _, s := range g.Structures {
		for _, id := range s.References {
			if ids[id] == "" {
				return fmt.Errorf("missing structure reference %s", id)
			}
		}
	}
	for i, c := range g.Cells {
		if !membership[i][0] || !membership[i][1] || membership[i][2] != (c.Cover >= 0) || membership[i][3] != (c.Intrusion >= 0) {
			return fmt.Errorf("missing geological region membership at cell %d", i)
		}
		if c.Province < 0 || c.Province >= len(g.Provinces) || c.Basement < 0 || c.Basement >= len(g.Formations) || c.Cover < -1 || c.Cover >= len(g.Formations) || c.Intrusion < -1 || c.Intrusion >= len(g.Formations) || c.Fault < -1 || c.Fault >= len(g.Structures) || !unit(c.Weathering) || !unit(c.Erosion) || !unit(c.Deposition) || !unit(c.Soil) {
			return fmt.Errorf("invalid geological cell %d", i)
		}
	}
	for _, s := range e.Hydrology.Springs {
		ids[s.ID] = "spring"
	}
	for _, s := range e.Hydrology.Wetlands {
		ids[s.ID] = "wetland"
	}
	for _, s := range e.Hydrology.Reaches {
		ids[s.ID] = "river"
	}
	for _, s := range e.WaterBodies {
		ids[e.hydroID("water", s.ID)] = "water"
	}
	for name := range e.Fields {
		ids[name] = "field"
	}
	catalog := map[string]ResourceDefinition{}
	for _, d := range ResourceCatalog {
		catalog[d.ID] = d
	}
	contexts := e.resourceContexts()
	resources := map[string]ResourceOccurrence{}
	coverage := map[string]map[int]bool{}
	for _, o := range r.Occurrences {
		if err := register(o.ID, "resource"); err != nil {
			return err
		}
		if err := geometry(o.ID, o.Geometry); err != nil {
			return err
		}
		if catalog[o.Type].ID == "" || o.Cause == "" || o.Variant == "" || !unit(o.Abundance) || o.Abundance == 0 || !unit(o.Quality) || !unit(o.Confidence) || !unit(o.Accessibility) || !finite(o.Depth) || o.Depth < 0 || len(o.References) == 0 || len(o.References) > n {
			return fmt.Errorf("invalid resource %s", o.ID)
		}
		if len(o.Properties) > 32 {
			return fmt.Errorf("too many resource properties %s", o.ID)
		}
		for name, value := range o.Properties {
			if name == "" || len(name) > 80 || !finite(value) || value < 0 || value > 1e6 {
				return fmt.Errorf("invalid property in resource %s", o.ID)
			}
		}
		resources[o.ID] = o
		cells := o.Geometry.Cells
		if o.Geometry.Kind == "point" {
			cells = []int{int(math.Round(o.Geometry.Point[1]))*w + int(math.Round(o.Geometry.Point[0]))}
		}
		if o.Geometry.Kind == "line" {
			return fmt.Errorf("unsupported occurrence geometry %s", o.ID)
		}
		for _, i := range cells {
			if o.Variant != "placer" {
				candidate := resourceEligibility(o.Type, contexts[i])
				if candidate.Score <= .02 || candidate.Variant != o.Variant {
					return fmt.Errorf("resource prerequisites absent: %s at cell %d", o.ID, i)
				}
			}
			if catalog[o.Type].Continuous {
				if coverage[o.Type] == nil {
					coverage[o.Type] = map[int]bool{}
				}
				coverage[o.Type][i] = true
			}
		}
	}
	for _, o := range r.Occurrences {
		for _, ref := range o.References {
			if ids[ref.ID] != ref.Kind {
				return fmt.Errorf("invalid reference %s in %s", ref.ID, o.ID)
			}
		}
		if o.Variant != "placer" {
			continue
		}
		if o.Type != "gold" && o.Type != "tin" && o.Type != "gemstones" {
			return fmt.Errorf("unsupported placer %s", o.ID)
		}
		reachable := map[int]bool{}
		sources := 0
		for _, ref := range o.References {
			if ref.Kind != "resource" {
				continue
			}
			primary := resources[ref.ID]
			if primary.Type != o.Type || primary.Variant == "placer" {
				return fmt.Errorf("invalid placer source %s", o.ID)
			}
			sources++
			for _, i := range primary.Geometry.Cells {
				for at, k := i, 0; at >= 0 && k < n; k++ {
					if reachable[at] {
						break
					}
					reachable[at] = true
					if e.get("ocean", at) > 0 {
						break
					}
					at = int(e.get("flow", at))
				}
			}
		}
		if sources == 0 {
			return fmt.Errorf("placer has no primary source %s", o.ID)
		}
		for _, i := range o.Geometry.Cells {
			if !reachable[i] || e.get("river", i) <= 0 || e.get("slope", i) >= .08 {
				return fmt.Errorf("placer has no downstream transport route %s", o.ID)
			}
		}
	}
	for id, values := range r.Potential {
		if !catalog[id].Continuous || len(values) != n {
			return fmt.Errorf("invalid potential field %s", id)
		}
		for i, v := range values {
			if !unit(v) || (v > 0) != coverage[id][i] {
				return fmt.Errorf("potential/occurrence mismatch %s at %d", id, i)
			}
		}
	}
	for _, d := range ResourceCatalog {
		if d.Continuous && len(r.Potential[d.ID]) != n {
			return fmt.Errorf("missing potential field %s", d.ID)
		}
	}
	return nil
}
