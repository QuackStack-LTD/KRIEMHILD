package httpapi

import (
	"kriemhild/internal/history"
	"net/http"
)

// A diverse generated terrain can have tens of thousands of populations. Keep
// that common case reusable while bounding retention across all open sessions.
const ecologyCacheBudget = 32 << 20

// One bounded read cache is shared by resource circles and natural inspection.
// It holds display labels and ranges, never the editable biological document.
type ecologyReadCache struct {
	world, age, terrain string
	revision            int
	root                history.Map
	savedSource         string
	hasState            bool
	sourceVerified      bool
	cells               map[int]spotBiosphere
	sourceStaleCells    map[int]spotBiosphere
}

func (s *Server) ecologySpots(r *http.Request, worldID, age, terrainID string, checkSource bool) (map[int]spotBiosphere, error) {
	revision, err := s.projects.CurrentRevision(r.Context(), worldID)
	if err != nil {
		return nil, err
	}
	s.ecologyMu.Lock()
	defer s.ecologyMu.Unlock()
	c := s.ecologyCache
	if c != nil && c.world == worldID && c.age == age && c.terrain == terrainID && c.revision != revision && s.projects.EcologyUnchangedSince(r.Context(), worldID, c.revision, revision) {
		c.revision = revision
	}
	if c == nil || c.world != worldID || c.age != age || c.terrain != terrainID || c.revision != revision {
		d, err := s.projects.EcologyView(r.Context(), worldID, age, terrainID)
		if err != nil {
			return nil, err
		}
		c = &ecologyReadCache{world: worldID, age: age, terrain: terrainID, revision: d.World.Revision, cells: biosphereSpots(d, age, terrainID)}
		if saved := d.Biosphere(age, terrainID); saved != nil {
			c.hasState, c.savedSource = true, saved.Source
		}
		for _, m := range d.Maps {
			if m.AgeID == age && m.TerrainID == terrainID && m.ParentID == "" {
				c.root = m
				break
			}
		}
		if saved := d.Biosphere(age, terrainID); saved != nil {
			c.sourceVerified = saved.SourceSnapshot != "" && saved.SourceSnapshot == c.root.Snapshot
		}
		if ecologyCacheBytes(c.cells) <= ecologyCacheBudget {
			s.ecologyCache = c
		} else {
			s.ecologyCache = nil
		}
	}
	if checkSource && c.hasState && !c.sourceVerified {
		// Habitat derivation depends on the immutable terrain snapshot, not on
		// names or unrelated World revisions. Rebuild it only when that changes.
		key := c.root.Snapshot + "/" + terrainID
		source := s.ecologySource
		if s.ecologySourceKey != key {
			d := &history.Document{World: history.World{ID: worldID}, Maps: []history.Map{c.root}}
			fresh, err := s.biosphereTerrain(r, d, age, terrainID)
			if err != nil {
				return staleEcology(c.cells), nil
			}
			source = fresh.Source
			s.ecologySourceKey, s.ecologySource = key, source
		}
		if c.savedSource != source {
			c.sourceStaleCells = staleEcology(c.cells)
			if ecologyCacheBytes(c.cells)+len(c.sourceStaleCells)*160 > ecologyCacheBudget && s.ecologyCache == c {
				s.ecologyCache = nil
			}
		} else {
			c.sourceStaleCells = nil
		}
		c.sourceVerified = true
	}
	if c.sourceStaleCells != nil {
		return c.sourceStaleCells, nil
	}
	return c.cells, nil
}

func staleEcology(cells map[int]spotBiosphere) map[int]spotBiosphere {
	copy := make(map[int]spotBiosphere, len(cells))
	for cell, value := range cells {
		value.Stale = true
		copy[cell] = value
	}
	return copy
}

func ecologyCacheBytes(cells map[int]spotBiosphere) int {
	bytes := len(cells) * 160
	seen := map[*spotLifeform]bool{}
	habitats := map[*spotHabitat]bool{}
	for _, c := range cells {
		if len(c.Habitats) > 0 && !habitats[&c.Habitats[0]] {
			habitats[&c.Habitats[0]] = true
			bytes += cap(c.Habitats) * 128
			for _, h := range c.Habitats {
				bytes += len(h.ID) + len(h.Name) + len(h.Kind) + len(h.Description)
			}
		}
		if len(c.Lifeforms) == 0 || seen[&c.Lifeforms[0]] {
			continue
		}
		seen[&c.Lifeforms[0]] = true
		bytes += cap(c.Lifeforms) * 192
		for _, e := range c.Lifeforms {
			bytes += len(e.ID) + len(e.Name) + len(e.Group) + len(e.Status)
			for _, v := range e.Roles {
				bytes += len(v)
			}
			for _, v := range e.Seasons {
				bytes += len(v)
			}
		}
	}
	return bytes
}
