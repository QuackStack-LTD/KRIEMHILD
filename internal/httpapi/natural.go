package httpapi

import (
	"math"
	"net/http"
	"strconv"

	"kriemhild/internal/terrain"
)

func (s *Server) worldNatural(w http.ResponseWriter, r *http.Request) {
	x, y, radius := 0., 0., 2.
	radiusKey, maximum := "radius", 20.
	kilometres := r.URL.Query().Has("radiusKm")
	if kilometres {
		radiusKey, maximum = "radiusKm", 160000
	}
	for key, dst := range map[string]*float64{"x": &x, "y": &y, radiusKey: &radius} {
		raw := r.URL.Query().Get(key)
		if raw == "" && key == "radius" {
			continue
		}
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			fail(w, 400, "Invalid natural-resource query coordinate or radius")
			return
		}
		*dst = value
	}
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	if x < 0 || y < 0 || x > float64(v.solver.W-1) || y > float64(v.solver.H-1) || radius < 0 || radius > maximum {
		fail(w, 400, "Natural-resource query outside world or supported radius")
		return
	}
	if v.natural == nil {
		v.natural = terrain.NewNaturalIndex(v.solver.Environment)
	}
	var result terrain.NaturalQuery
	if kilometres {
		if v.solver.Environment == nil || v.solver.Environment.Geography == nil {
			fail(w, 400, "This legacy world has no geographic distance scale")
			return
		}
		result = v.natural.QueryKM(x, y, radius)
	} else {
		result = v.natural.Query(x, y, radius, v.solver.WrapX)
	}
	result.Climate = v.climateLayer().Describe(x, y)
	cell := int(math.Round(y))*v.solver.W + int(math.Round(x))
	result.Terrain = "Unresolved terrain"
	if t := v.solver.TypeAt(cell); t >= 0 {
		result.Terrain = v.solver.Rules.Config.Types[t].Name
	}
	respond(w, 200, struct {
		terrain.NaturalQuery
		Biosphere *spotBiosphere `json:"biosphere,omitempty"`
	}{result, s.spotBiosphere(r, v, cell)})
}

// Rendering consumes physical fields. Invisible natural layers are queried on
// demand, avoiding multi-megabyte transfers and browser clones on each edit.
// Save/ZIP paths retain the complete environment, including both layers.
func renderEnvironment(e *terrain.Environment) *terrain.Environment {
	if e == nil {
		return nil
	}
	copy := *e
	copy.Geology, copy.Resources = nil, nil
	copy.Climate = nil
	if e.Climate != nil {
		copy.Fields = make(map[string][]float64, len(e.Fields)+2)
		for k, v := range e.Fields {
			copy.Fields[k] = v
		}
		copy.Fields["seasonalClimate"] = make([]float64, len(e.Mask))
		copy.Fields["seasonCount"] = make([]float64, len(e.Mask))
		indices := map[string]int{}
		for i, c := range e.Climate.Cells {
			zone := e.Climate.Zones[c.Zone]
			index, ok := indices[zone.Name]
			if !ok {
				index = len(copy.SeasonalZones)
				indices[zone.Name] = index
				copy.SeasonalZones = append(copy.SeasonalZones, zone.Name)
			}
			copy.Fields["seasonalClimate"][i] = float64(index)
			copy.Fields["seasonCount"][i] = float64(len(c.Seasons))
		}
	}
	return &copy
}
