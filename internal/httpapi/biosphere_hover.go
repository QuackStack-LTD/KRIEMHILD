package httpapi

import (
	"kriemhild/internal/history"
	"net/http"
	"sort"
	"strings"
)

type spotLifeform struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Group     string   `json:"group"`
	Roles     []string `json:"roles"`
	Abundance float64  `json:"abundance"`
	Status    string   `json:"status"`
	Seasons   []string `json:"seasons"`
}
type spotHabitat struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}
type spotBiosphere struct {
	Available bool           `json:"available"`
	Stale     bool           `json:"stale"`
	WorldID   string         `json:"worldId,omitempty"`
	AgeID     string         `json:"ageId,omitempty"`
	Habitats  []spotHabitat  `json:"habitats"`
	Lifeforms []spotLifeform `json:"lifeforms"`
}
type biosphereHoverCache struct {
	Revision                  int
	AgeID, TerrainID, WorldID string
	Cells                     map[int]spotBiosphere
}

// Cells share immutable community slices; do not duplicate every species for every pixel.
func biosphereSpots(d *history.Document, age, terrainID string) map[int]spotBiosphere {
	out := map[int]spotBiosphere{}
	species := map[string]history.Entity{}
	for _, e := range d.Entities {
		if e.AgeID == age && !e.Deleted && e.Biology != nil {
			species[e.ID] = e
		}
	}
	cells := map[int][]string{}
	habitats := map[string]spotHabitat{}
	residents := map[string][]spotLifeform{}
	stale := false
	for _, state := range d.Biospheres {
		if state.AgeID != age || state.TerrainID != terrainID {
			continue
		}
		stale = stale || state.Stale
		for _, o := range state.Occurrences {
			e, ok := species[o.SpeciesID]
			if !ok || o.Abundance <= 0 || o.Status == "absent" || o.Status == "locally-extinct" {
				continue
			}
			residents[o.HabitatID] = append(residents[o.HabitatID], spotLifeform{e.ID, e.Name, e.Biology.Group, e.Biology.Roles, o.Abundance, o.Status, o.Seasons})
		}
		for _, h := range state.Habitats {
			habitats[h.ID] = spotHabitat{h.ID, h.Name, h.Kind, h.Description}
			for _, cell := range h.Cells {
				cells[cell] = append(cells[cell], h.ID)
			}
		}
	}
	communities := map[string]spotBiosphere{}
	for cell, ids := range cells {
		sort.Strings(ids)
		key := strings.Join(ids, "/")
		v, exists := communities[key]
		if !exists {
			v = spotBiosphere{Available: true, Stale: stale, WorldID: d.World.ID, AgeID: age, Lifeforms: []spotLifeform{}}
			unique := map[string]spotLifeform{}
			seen := map[string]bool{}
			for _, id := range ids {
				if seen[id] {
					continue
				}
				seen[id] = true
				v.Habitats = append(v.Habitats, habitats[id])
				for _, e := range residents[id] {
					previous, ok := unique[e.ID]
					if !ok || e.Abundance > previous.Abundance {
						unique[e.ID] = e
					}
				}
			}
			for _, e := range unique {
				v.Lifeforms = append(v.Lifeforms, e)
			}
			sort.Slice(v.Lifeforms, func(i, j int) bool {
				if v.Lifeforms[i].Group != v.Lifeforms[j].Group {
					return v.Lifeforms[i].Group < v.Lifeforms[j].Group
				}
				return v.Lifeforms[i].Name < v.Lifeforms[j].Name
			})
			communities[key] = v
		}
		out[cell] = v
	}
	return out
}
func (s *Server) spotBiosphere(r *http.Request, v *session, cell int) *spotBiosphere {
	if s.projects == nil || v.historical == nil || v.historyWorld == "" {
		return nil
	}
	revision, err := s.projects.HistoryRevision(r.Context(), v.historyWorld)
	if err != nil {
		return nil
	}
	m := v.historical
	c := v.biosphereHover
	if c == nil || c.Revision != revision || c.AgeID != m.AgeID || c.TerrainID != m.TerrainID || c.WorldID != v.historyWorld {
		d, err := s.projects.History(r.Context(), v.historyWorld)
		if err != nil {
			return nil
		}
		c = &biosphereHoverCache{Revision: d.World.Revision, AgeID: m.AgeID, TerrainID: m.TerrainID, WorldID: v.historyWorld, Cells: biosphereSpots(d, m.AgeID, m.TerrainID)}
		v.biosphereHover = c
	}
	spot := c.Cells[cell]
	return &spot
}
