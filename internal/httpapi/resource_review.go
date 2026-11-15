package httpapi

import (
	"encoding/json"
	"fmt"
	"kriemhild/internal/history"
	terrain "kriemhild/internal/legacyterrain"
	"kriemhild/internal/resourcesurvey"
	"kriemhild/internal/storage"
	"math"
	"net/http"
	"sort"
	"strconv"
)

type archivedResourceStatistics struct {
	Biosphere               spotBiosphere                `json:"biosphere"`
	BiologyCoverageFraction float64                      `json:"biologyCoverageFraction"`
	LifeformGroups          map[string]int               `json:"lifeformGroups"`
	LifeformCount           int                          `json:"lifeformCount"`
	HabitatCount            int                          `json:"habitatCount"`
	Samples                 int                          `json:"samples"`
	SampleAreaKM2           float64                      `json:"sampleAreaKm2"`
	CoverageFraction        float64                      `json:"coverageFraction"`
	Fields                  map[string]resourceStatistic `json:"fields"`
	Biomes                  map[string]float64           `json:"biomes"`
	Resources               []terrain.ResourceOccurrence `json:"resources"`
	ResourceCounts          map[string]int               `json:"resourceCounts"`
	Quantities              map[string]float64           `json:"quantities"`
	FreshWaterFraction      float64                      `json:"freshWaterFraction"`
	RechargeMMYear          float64                      `json:"rechargeMmYear"`
}

type archivedStatisticsCache struct {
	states      map[string]resourcesurvey.State
	resources   map[string]terrain.ResourceOccurrence
	communities map[string]spotBiosphere
}

func newArchivedStatisticsCache() *archivedStatisticsCache {
	return &archivedStatisticsCache{states: map[string]resourcesurvey.State{}, resources: map[string]terrain.ResourceOccurrence{}, communities: map[string]spotBiosphere{}}
}
func archivedStatistics(circle resourcesurvey.Circle, obs []resourcesurvey.Observation, blobs map[string][]byte, year *int64, caches ...*archivedStatisticsCache) (archivedResourceStatistics, error) {
	cache := newArchivedStatisticsCache()
	if len(caches) > 0 {
		cache = caches[0]
	}
	out := archivedResourceStatistics{SampleAreaKM2: resourcesurvey.CellArea(circle, resourcesurvey.Level(circle)), Fields: map[string]resourceStatistic{}, Biomes: map[string]float64{}, Resources: []terrain.ResourceOccurrence{}, ResourceCounts: map[string]int{}, Quantities: map[string]float64{}}
	out.Biosphere = spotBiosphere{Habitats: []spotHabitat{}, Lifeforms: []spotLifeform{}}
	out.LifeformGroups = map[string]int{}
	resolved := resourcesurvey.Resolve(circle, obs, year)
	out.Samples = len(resolved)
	if len(resolved) == 0 {
		return out, nil
	}
	interiorArea := 0.
	for _, c := range resourcesurvey.Cells(circle, resourcesurvey.Level(circle)) {
		lon, lat := c.Center()
		if circle.Contains(lon, lat) {
			interiorArea += c.Area(circle.PlanetRadiusKM)
		}
	}
	knownArea := 0.
	for _, r := range resolved {
		knownArea += r.Cell.Area(circle.PlanetRadiusKM)
	}
	if interiorArea == 0 {
		interiorArea = knownArea
	}
	out.CoverageFraction = math.Min(1, knownArea/interiorArea)
	out.SampleAreaKM2 = knownArea / float64(len(resolved))
	rank := map[string]resourcesurvey.Observation{}
	for _, o := range obs {
		rank[o.ID] = o
	}
	type knownResource struct{ hash, report string }
	resources := map[string]knownResource{}
	fieldWeights := map[string]float64{}
	life := map[string]spotLifeform{}
	abundance := map[string]float64{}
	lifeReport := map[string]string{}
	habitats := map[string]spotHabitat{}
	type weightedCommunity struct {
		hash, report string
		weight       float64
	}
	bioWeights := map[string]weightedCommunity{}
	for _, r := range resolved {
		weight := r.Cell.Area(circle.PlanetRadiusKM) / knownArea
		state, ok := cache.states[r.Hash]
		if !ok {
			if json.Unmarshal(blobs[r.Hash], &state) != nil {
				return out, fmt.Errorf("Saved resource sample is unavailable")
			}
			cache.states[r.Hash] = state
		}
		for k, v := range state.Fields {
			a, ok := out.Fields[k]
			if !ok {
				a.Min = v
				a.Max = v
			}
			a.Min = math.Min(a.Min, v)
			a.Max = math.Max(a.Max, v)
			a.Mean += v * weight
			fieldWeights[k] += weight
			out.Fields[k] = a
		}
		if state.Biome != "" {
			out.Biomes[state.Biome] += weight
		}
		if state.FreshWater {
			out.FreshWaterFraction += weight
		}
		out.RechargeMMYear += state.RechargeMMYear * weight
		if state.Biosphere != "" {
			key := state.Biosphere + "/" + r.ReportID
			v := bioWeights[key]
			v.hash = state.Biosphere
			v.report = r.ReportID
			v.weight += weight
			bioWeights[key] = v
		}
		for id, h := range state.Resources {
			old, ok := resources[id]
			if !ok || resourcesurvey.Earlier(rank[old.report], rank[r.ReportID]) {
				resources[id] = knownResource{h, r.ReportID}
			}
		}
	}
	for _, entry := range bioWeights {
		bio, ok := cache.communities[entry.hash]
		if !ok {
			if json.Unmarshal(blobs[entry.hash], &bio) != nil {
				return out, fmt.Errorf("Saved biological observation is unavailable")
			}
			cache.communities[entry.hash] = bio
		}
		if !bio.Available {
			continue
		}
		out.BiologyCoverageFraction += entry.weight
		out.Biosphere.Available = true
		out.Biosphere.Stale = out.Biosphere.Stale || bio.Stale
		for _, h := range bio.Habitats {
			habitats[h.ID] = h
		}
		for _, v := range bio.Lifeforms {
			abundance[v.ID] += v.Abundance * entry.weight
			if old, ok := lifeReport[v.ID]; !ok || resourcesurvey.Earlier(rank[old], rank[entry.report]) {
				life[v.ID] = v
				lifeReport[v.ID] = entry.report
			}
		}
	}
	for k, v := range out.Fields {
		v.Mean /= fieldWeights[k]
		out.Fields[k] = v
	}
	for id, v := range life {
		if out.BiologyCoverageFraction > 0 {
			v.Abundance = abundance[id] / out.BiologyCoverageFraction
		}
		out.Biosphere.Lifeforms = append(out.Biosphere.Lifeforms, v)
		out.LifeformGroups[v.Group]++
	}
	for _, v := range habitats {
		out.Biosphere.Habitats = append(out.Biosphere.Habitats, v)
	}
	sort.Slice(out.Biosphere.Lifeforms, func(i, j int) bool { return out.Biosphere.Lifeforms[i].ID < out.Biosphere.Lifeforms[j].ID })
	sort.Slice(out.Biosphere.Habitats, func(i, j int) bool { return out.Biosphere.Habitats[i].ID < out.Biosphere.Habitats[j].ID })
	out.LifeformCount = len(life)
	out.HabitatCount = len(habitats)
	ids := []string{}
	for id := range resources {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		h := resources[id].hash
		value, ok := cache.resources[h]
		if !ok {
			if json.Unmarshal(blobs[h], &value) != nil {
				return out, fmt.Errorf("Saved resource inventory is unavailable")
			}
			cache.resources[h] = value
		}
		out.Resources = append(out.Resources, value)
		out.ResourceCounts[value.Type]++
		out.Quantities[value.Type+" | "+value.QuantityUnit] += value.Quantity
	}
	return out, nil
}

type resourceHistoryPoint struct {
	Year       int64                      `json:"year"`
	Statistics archivedResourceStatistics `json:"statistics"`
}
type resourceAgeHistory struct {
	AgeID   string                 `json:"ageId"`
	Name    string                 `json:"name"`
	Points  []resourceHistoryPoint `json:"points"`
	Undated int                    `json:"undated"`
}

func resourceObservations(files []storage.SavedResourceSurvey, terrainID, sourceAge string) []resourcesurvey.Observation {
	out := []resourcesurvey.Observation{}
	for _, f := range files {
		if f.Survey.TerrainID == terrainID && f.Survey.SourceAge == sourceAge {
			out = append(out, resourcesurvey.Observation{ID: f.ID, Circle: f.Survey.Circle, Year: f.Survey.Year, RecordedAt: f.Survey.RecordedAt, Manifest: f.Manifest})
		}
	}
	return out
}
func (s *Server) reviewResourceSurveys(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	world, age, archive := r.PathValue("world"), r.URL.Query().Get("age"), r.URL.Query().Get("archive")
	// A lightweight list never reads sample bodies. Detail loads only surveys
	// retained by this archive; live terrain and other archives are never queried.
	if r.URL.Query().Get("area") == "" {
		d, err := s.projects.ResourceArchives(r.Context(), world, age, false)
		if err != nil {
			fail(w, 404, err.Error())
			return
		}
		a := d.Entity(age, archive)
		if a == nil || a.Deleted || a.Kind != "archive" {
			fail(w, 404, "Archive not found")
			return
		}
		ids := storage.ResourceArchiveDescendants(d, age, archive)
		files := []map[string]any{}
		folder := ""
		for _, e := range d.Entities {
			var p struct {
				ResourceFolder string `json:"resourceFolder"`
			}
			_ = json.Unmarshal(e.Properties, &p)
			if !e.Deleted && history.ArchiveParent(e) == archive && p.ResourceFolder == "resources" {
				folder = e.ID
			}
		}
		for _, f := range d.LibraryFiles {
			if f.AgeID == age && ids[f.ArchiveID] && f.ResourceSurvey != nil && f.Status != "lost" && f.Status != "destroyed" && len(f.Versions) > 0 {
				files = append(files, map[string]any{"id": f.ID, "archiveId": f.ArchiveID, "survey": f.ResourceSurvey})
			}
		}
		respond(w, 200, map[string]any{"archive": a, "ages": d.Ages, "terrains": d.Terrains, "reports": files, "resourcesFolderId": folder})
		return
	}
	mode := ""
	if r.URL.Query().Get("original") == "1" {
		mode = "original"
	}
	d, files, blobs, err := s.projects.ResourceSurveys(r.Context(), world, age, archive, r.URL.Query().Get("area"), mode)
	if err != nil {
		fail(w, 404, err.Error())
		return
	}
	var selected *storage.SavedResourceSurvey
	for i := range files {
		if files[i].ID == r.URL.Query().Get("area") {
			selected = &files[i]
			break
		}
	}
	if selected == nil {
		fail(w, 404, "This report is not retained by the selected archive")
		return
	}
	if mode == "original" {
		var original terrainResourceReport
		if json.Unmarshal(blobs[selected.Manifest.Report], &original) != nil {
			fail(w, 409, "Saved report is unavailable")
			return
		}
		if h := selected.Manifest.ReportBiosphere; h != "" {
			if json.Unmarshal(blobs[h], &original.Biosphere) != nil {
				fail(w, 409, "Saved biological report is unavailable")
				return
			}
		}
		original.Biosphere.WorldID = world
		original.Biosphere.AgeID = selected.Survey.SourceAge
		respondHistory(w, r, 200, map[string]any{"report": original})
		return
	}
	sourceAge := r.URL.Query().Get("sourceAge")
	if sourceAge == "" {
		sourceAge = selected.Survey.SourceAge
		for _, f := range files {
			if f.Survey.SourceAge == age {
				sourceAge = age
				break
			}
		}
	}
	if d.Age(sourceAge) == nil {
		fail(w, 400, "Unknown observed Age")
		return
	}
	var year *int64
	if text := r.URL.Query().Get("year"); text != "" {
		value, e := strconv.ParseInt(text, 10, 64)
		if e != nil || value < -9007199254740991 || value > 9007199254740991 {
			fail(w, 400, "Invalid historical year")
			return
		}
		year = &value
	}
	cache := newArchivedStatisticsCache()
	current, err := archivedStatistics(selected.Survey.Circle, resourceObservations(files, selected.Survey.TerrainID, sourceAge), blobs, year, cache)
	if err != nil {
		fail(w, 409, err.Error())
		return
	}
	if r.URL.Query().Get("lifeforms") == "1" {
		respondHistory(w, r, 200, map[string]any{"biosphere": current.Biosphere, "coverageFraction": current.BiologyCoverageFraction})
		return
	}
	histories := []resourceAgeHistory{}
	for _, a := range d.Ages {
		obs := resourceObservations(files, selected.Survey.TerrainID, a.ID)
		years := map[int64]bool{}
		undated := 0
		for _, o := range obs {
			if resourcesurvey.Distance(o.Circle.Longitude, o.Circle.Latitude, selected.Survey.Circle.Longitude, selected.Survey.Circle.Latitude, o.Circle.PlanetRadiusKM) > o.Circle.RadiusKM+selected.Survey.Circle.RadiusKM {
				continue
			}
			if o.Year == nil {
				undated++
			} else {
				years[*o.Year] = true
			}
		}
		if len(years) == 0 && undated == 0 {
			continue
		}
		h := resourceAgeHistory{AgeID: a.ID, Name: a.Name, Points: []resourceHistoryPoint{}, Undated: undated}
		ordered := []int64{}
		for y := range years {
			ordered = append(ordered, y)
		}
		sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
		for _, y := range ordered {
			if r.Context().Err() != nil {
				return
			}
			stats, e := archivedStatistics(selected.Survey.Circle, obs, blobs, &y, cache)
			if e != nil {
				fail(w, 409, e.Error())
				return
			}
			stats.Resources = nil // Historical charts need measures, not repeated full deposit records.
			stats.Biosphere.Lifeforms = nil
			stats.Biosphere.Habitats = nil
			h.Points = append(h.Points, resourceHistoryPoint{y, stats})
		}
		histories = append(histories, h)
	}
	current.Biosphere.Lifeforms = nil
	current.Biosphere.Habitats = nil
	respondHistory(w, r, 200, map[string]any{"area": selected, "sourceAge": sourceAge, "year": year, "current": current, "history": histories})
}
