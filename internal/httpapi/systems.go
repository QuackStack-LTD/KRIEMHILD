package httpapi

import (
	"fmt"
	"kriemhild/internal/history"
	"kriemhild/internal/terrain"
	"math"
	"net/http"
	"slices"
)

func (s *Server) systemRoutes() {
	s.mux.HandleFunc("POST /api/worlds/{world}/systems/analyze", s.analyzeSystems)
	s.mux.HandleFunc("GET /api/worlds/{world}/systems/sources", s.systemSources)
}
func (s *Server) systemEnvironment(r *http.Request, d *history.Document, age, id string) (*terrain.Environment, error) {
	for _, m := range d.Maps {
		if m.AgeID == age && m.TerrainID == id && m.ParentID == "" {
			raw, err := s.projects.SnapshotPart(r.Context(), m.Snapshot, "environment")
			if err != nil {
				return nil, err
			}
			var e *terrain.Environment
			if err = expandJSON(raw, &e); err != nil || e == nil {
				return nil, fmt.Errorf("environment unavailable")
			}
			return e, nil
		}
	}
	return nil, fmt.Errorf("saved terrain unavailable")
}
func (s *Server) systemSources(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	d, err := s.projects.History(r.Context(), r.PathValue("world"))
	if err != nil {
		fail(w, 404, "World not found")
		return
	}
	age, id := r.URL.Query().Get("age"), r.URL.Query().Get("terrain")
	if d.Age(age) == nil {
		fail(w, 400, "Age not found")
		return
	}
	e, err := s.systemEnvironment(r, d, age, id)
	if err != nil {
		fail(w, 404, err.Error())
		return
	}
	type Source struct {
		ID        string  `json:"id"`
		Type      string  `json:"type"`
		Cause     string  `json:"cause"`
		Abundance float64 `json:"abundance"`
	}
	list := []Source{}
	if e.Resources != nil {
		for _, o := range e.Resources.Occurrences {
			list = append(list, Source{o.ID, o.Type, o.Cause, o.Abundance})
		}
	}
	respond(w, 200, list)
}

type systemSupply struct {
	Implementation string                 `json:"implementation"`
	Resource       string                 `json:"resource"`
	Location       string                 `json:"location"`
	Status         string                 `json:"status"`
	Explanation    string                 `json:"explanation"`
	Sources        []history.SystemSource `json:"sources"`
}

func (s *Server) analyzeSystems(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	var req struct {
		AgeID     string           `json:"ageId"`
		SystemID  string           `json:"systemId"`
		ChangedID string           `json:"changedId"`
		Revision  int              `json:"revision"`
		Draft     *history.Command `json:"draft,omitempty"`
	}
	if err := decode(w, r, &req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	d, err := s.projects.History(r.Context(), r.PathValue("world"))
	if err != nil {
		fail(w, 404, "World not found")
		return
	}
	if req.Revision != d.World.Revision {
		fail(w, 409, "World changed; reload before reviewing this draft")
		return
	}
	if d.Age(req.AgeID) == nil {
		fail(w, 400, "Age not found")
		return
	}
	before := d.AnalyzeSystems(req.AgeID, req.SystemID, req.ChangedID)
	if req.Draft != nil {
		if req.Draft.Kind != "entity" && req.Draft.Kind != "batch" {
			fail(w, 400, "Review accepts entity or batch edits only")
			return
		}
		req.Draft.AgeID = req.AgeID
		if err = d.Apply(*req.Draft); err != nil {
			fail(w, 400, err.Error())
			return
		}
	}
	if req.SystemID != "" {
		root := d.Entity(req.AgeID, req.SystemID)
		if root == nil || root.Deleted || root.Kind != "system" {
			fail(w, 400, "System not found in this Age")
			return
		}
	}
	after := d.AnalyzeSystems(req.AgeID, req.SystemID, req.ChangedID)
	// Dependency removal also affects former dependents; preserve both paths.
	for _, p := range before.Affected {
		found := false
		for _, q := range after.Affected {
			if q.EntityID == p.EntityID {
				found = true
			}
		}
		if !found {
			after.Affected = append(after.Affected, p)
		}
	}
	supply := s.systemSupply(r, d, req.AgeID, req.SystemID, after.Requirements)
	respond(w, 200, map[string]any{"before": before, "analysis": after, "supply": supply, "revision": d.World.Revision, "advisory": true})
}
func (s *Server) systemSupply(r *http.Request, d *history.Document, age, system string, deps map[string][]string) []systemSupply {
	out := []systemSupply{}
	environments := map[string]*terrain.Environment{}
	attempted := map[string]bool{}
	env := func(id string) *terrain.Environment {
		if !attempted[id] {
			attempted[id] = true
			environments[id], _ = s.systemEnvironment(r, d, age, id)
		}
		return environments[id]
	}
	for _, implementation := range d.Entities {
		if implementation.AgeID != age || implementation.Deleted || implementation.System == nil || !slices.Contains([]string{"system-implementation", "system-application"}, implementation.Kind) {
			continue
		}
		// Include implementations from other systems affected through shared prerequisites.
		seen := map[string]bool{implementation.ID: true}
		chain := []string{implementation.ID}
		for i := 0; i < len(chain); i++ {
			for _, id := range deps[chain[i]] {
				if !seen[id] {
					seen[id] = true
					chain = append(chain, id)
				}
			}
		}
		relevant := system == "" || implementation.System.SystemID == system
		for _, id := range chain {
			v := d.Entity(age, id)
			if v != nil && v.System != nil && (v.ID == system || v.System.SystemID == system) {
				relevant = true
			}
		}
		if !relevant {
			continue
		}
		for _, place := range d.Relations {
			if place.AgeID != age || place.Source != implementation.ID || place.Kind != "system-implemented-in" {
				continue
			}
			for _, id := range chain {
				resource := d.Entity(age, id)
				if resource == nil || resource.Kind != "system-resource" {
					continue
				}
				sources := resource.System.Sources
				row := systemSupply{Implementation: implementation.ID, Resource: id, Location: place.Destination, Status: "unverified", Explanation: "No local source has been verified. Record sources or an explicit supply arrangement; this does not establish impossibility.", Sources: sources}
				for _, link := range d.Relations {
					if link.AgeID == age && link.Source == id && link.Destination == place.Destination && link.Kind == "system-supply" {
						row.Status = "declared-supply"
						row.Explanation = "Authored supply arrangement: " + link.Description
					}
				}
				local, unknown := false, true
				for _, rep := range d.Representations {
					if rep.AgeID == age && rep.EntityID == place.Destination && len(rep.Shape.Points) > 0 {
						unknown = false
					}
				}
				for _, source := range sources {
					if source.OccurrenceID != "" {
						e := env(source.TerrainID)
						if e == nil || e.Resources == nil {
							unknown = true
							continue
						}
						found := false
						for _, occ := range e.Resources.Occurrences {
							if occ.ID == source.OccurrenceID {
								found = true
								local = local || sourceInPlace(d, age, place.Destination, source.TerrainID, e.Options.Columns, occ.Geometry)
							}
						}
						if !found {
							unknown = true
						}
					}
					if source.EntityID != "" {
						v := d.Entity(age, source.EntityID)
						if v == nil || v.Deleted {
							unknown = true
							continue
						}
						found := false
						for _, state := range d.Biospheres {
							if state.AgeID != age {
								continue
							}
							for _, o := range state.Occurrences {
								if o.SpeciesID != source.EntityID || o.Abundance <= 0 || slices.Contains([]string{"absent", "locally-extinct"}, o.Status) {
									continue
								}
								for _, hab := range state.Habitats {
									if hab.ID == o.HabitatID {
										found = true
										if state.Stale {
											unknown = true
										} else {
											local = local || sourceInPlace(d, age, place.Destination, state.TerrainID, state.Width, terrain.NaturalGeometry{Kind: "cells", Cells: hab.Cells})
										}
									}
								}
							}
						}
						if !found {
							for _, rep := range d.Representations {
								if rep.AgeID == age && rep.EntityID == source.EntityID {
									m := d.Map(age, rep.MapID)
									if m != nil {
										found = true
										local = local || sourceInPlace(d, age, place.Destination, m.TerrainID, 0, terrain.NaturalGeometry{Path: rep.Shape.Points})
									}
								}
							}
						}
						if !found {
							unknown = true
						}
					}
				}
				if row.Status != "declared-supply" {
					if local {
						row.Status = "local-source"
						row.Explanation = "A recorded source intersects this location. Access, quantity, extraction and transport capacity still require author confirmation."
					} else if len(sources) > 0 && !unknown {
						row.Status = "supply-gap"
						row.Explanation = "Recorded sources do not intersect this location. An import route or other supply is not recorded; feasibility needs review."
					} else if unknown {
						row.Explanation = "Some source or distribution data is missing or stale. Local supply cannot be verified."
					}
				}
				out = append(out, row)
			}
		}
	}
	return out
}

// Geometry is measured in the saved terrain's parent grid, including half-cell
// footprints; small settlements must not miss deposits merely between centres.
func sourceInPlace(d *history.Document, age, place, terrainID string, width int, g terrain.NaturalGeometry) bool {
	for _, rep := range d.Representations {
		if rep.AgeID != age || rep.EntityID != place {
			continue
		}
		m := d.Map(age, rep.MapID)
		if m == nil || m.TerrainID != terrainID {
			continue
		}
		polygon := rep.Shape.Points
		if len(polygon) == 0 {
			continue
		}
		if rep.Shape.Geometry == "point" {
			p := polygon[0]
			polygon = [][2]float64{{p[0] - .5, p[1] - .5}, {p[0] + .5, p[1] - .5}, {p[0] + .5, p[1] + .5}, {p[0] - .5, p[1] + .5}}
		} else if rep.Shape.Geometry != "polygon" {
			continue
		}
		minX, maxX, minY, maxY := polygon[0][0], polygon[0][0], polygon[0][1], polygon[0][1]
		for _, p := range polygon {
			minX = math.Min(minX, p[0])
			maxX = math.Max(maxX, p[0])
			minY = math.Min(minY, p[1])
			maxY = math.Max(maxY, p[1])
		}
		for _, cell := range g.Cells {
			if width <= 0 {
				continue
			}
			x, y := float64(cell%width), float64(cell/width)
			if x+.5 < minX || x-.5 > maxX || y+.5 < minY || y-.5 > maxY {
				continue
			}
			square := [][2]float64{{x - .5, y - .5}, {x + .5, y - .5}, {x + .5, y + .5}, {x - .5, y + .5}}
			if systemGeometryOverlap(polygon, square, true) {
				return true
			}
		}
		if g.Point != nil && systemPointInside(*g.Point, polygon) {
			return true
		}
		if systemGeometryOverlap(polygon, g.Path, g.Kind == "polygon") {
			return true
		}
	}
	return false
}
func systemPointInside(p [2]float64, polygon [][2]float64) bool {
	inside := false
	for i, j := 0, len(polygon)-1; i < len(polygon); j, i = i, i+1 {
		a, b := polygon[i], polygon[j]
		if systemOnSegment(p, a, b) {
			return true
		}
		if (a[1] > p[1]) != (b[1] > p[1]) && p[0] < (b[0]-a[0])*(p[1]-a[1])/(b[1]-a[1])+a[0] {
			inside = !inside
		}
	}
	return inside
}
func systemCross(a, b, c [2]float64) float64 {
	return (b[0]-a[0])*(c[1]-a[1]) - (b[1]-a[1])*(c[0]-a[0])
}
func systemOnSegment(p, a, b [2]float64) bool {
	return math.Abs(systemCross(a, b, p)) < 1e-9 && p[0] >= math.Min(a[0], b[0])-1e-9 && p[0] <= math.Max(a[0], b[0])+1e-9 && p[1] >= math.Min(a[1], b[1])-1e-9 && p[1] <= math.Max(a[1], b[1])+1e-9
}
func systemSegmentsCross(a, b, c, d [2]float64) bool {
	return systemOnSegment(a, c, d) || systemOnSegment(b, c, d) || systemOnSegment(c, a, b) || systemOnSegment(d, a, b) || systemCross(a, b, c)*systemCross(a, b, d) < 0 && systemCross(c, d, a)*systemCross(c, d, b) < 0
}
func systemGeometryOverlap(polygon, path [][2]float64, closed bool) bool {
	if len(path) == 0 {
		return false
	}
	for _, p := range path {
		if systemPointInside(p, polygon) {
			return true
		}
	}
	if closed && systemPointInside(polygon[0], path) {
		return true
	}
	end := len(path) - 1
	if closed {
		end++
	}
	for i := 0; i < end; i++ {
		a, b := path[i], path[(i+1)%len(path)]
		for j := range polygon {
			if systemSegmentsCross(a, b, polygon[j], polygon[(j+1)%len(polygon)]) {
				return true
			}
		}
	}
	return false
}
