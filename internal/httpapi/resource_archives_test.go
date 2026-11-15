package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/history"
	terrain "kriemhild/internal/legacyterrain"
	"kriemhild/internal/resourcesurvey"
	"kriemhild/internal/storage"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/world"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestResourceArchiveAPICompleteSavedKnowledgeAndSessionContinuity(t *testing.T) {
	s, db, v := biosphereDetailFixture(t)
	ctx := context.Background()
	d, err := db.History(ctx, v.historyWorld)
	if err != nil {
		t.Fatal(err)
	}
	d, err = db.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(d *history.Document) error {
		e := history.Entity{ID: "archive", Kind: "archive", Name: "Settlement archive"}
		if err := d.Apply(history.Command{Kind: "entity", AgeID: v.historical.AgeID, Entity: &e}); err != nil {
			return err
		}
		place := history.Entity{ID: "site", Kind: "city", Name: "Survey site", TerrainID: v.historical.TerrainID, HomeMap: v.historical.ID}
		if err := d.Apply(history.Command{Kind: "entity", AgeID: v.historical.AgeID, Entity: &place}); err != nil {
			return err
		}
		d.Representations = append(d.Representations, history.Representation{ID: "site", AgeID: v.historical.AgeID, MapID: v.historical.ID, EntityID: "site", Shape: world.Entity{ID: "site", Name: "Survey site", Kind: "city", Layer: "settlements", Geometry: "point", Color: "#c5ad76", Points: [][2]float64{{float64(v.solver.W-1) / 2, float64(v.solver.H-1) / 2}}}})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.applyHistoricalMap(ctx, v); err != nil {
		t.Fatal(err)
	}
	base := "/api/sessions/terrain/terrain/"
	path := base + "resources?longitude=0&latitude=0&radiusKm=10"
	response := httptest.NewRecorder()
	s.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
	var report terrainResourceReport
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &report) != nil || report.Fingerprint == "" || report.Context == nil {
		t.Fatal("preview lacks archive context", response.Code, response.Body)
	}
	choices := autoRequest(t, s, "GET", base+"resource-archives?longitude=0&latitude=0&radiusKm=10", "")
	if len(choices["archives"].([]any)) != 1 {
		t.Fatal("missing archives")
	}
	payload := map[string]any{"archiveId": "archive", "year": 10, "format": "md", "longitude": 0, "latitude": 0, "radiusKm": 10, "fingerprint": report.Fingerprint}
	raw, _ := json.Marshal(payload)
	saved := autoRequest(t, s, "POST", base+"resource-reports", string(raw))
	id := saved["id"].(string)
	if v.historyRevision != int(saved["revision"].(float64)) {
		t.Fatal("saving report invalidated the open terrain session")
	}
	again := httptest.NewRecorder()
	s.ServeHTTP(again, httptest.NewRequest("GET", path, nil))
	var repeat terrainResourceReport
	_ = json.Unmarshal(again.Body.Bytes(), &repeat)
	if again.Code != 200 || repeat.Fingerprint != report.Fingerprint {
		t.Fatal("record save changed the preview")
	}
	document, _, err := db.LibraryContent(ctx, d.World.ID, v.historical.AgeID, id, "")
	if err != nil || !strings.Contains(string(document), "Terrain resource report") || !strings.Contains(string(document), "fresh Water") {
		t.Fatal("human-readable report incomplete", err, string(document[:min(len(document), 250)]))
	}
	payload["fingerprint"] = "changed"
	raw, _ = json.Marshal(payload)
	bad := httptest.NewRecorder()
	s.ServeHTTP(bad, httptest.NewRequest("POST", base+"resource-reports", bytes.NewReader(raw)))
	if bad.Code != 409 {
		t.Fatal("stale preview archived", bad.Code)
	}
	// Saved statistics remain available with all live terrain sessions closed.
	s.sessions = map[string]*session{}
	profileBase := fmt.Sprintf("/api/worlds/%s/settlements/site", d.World.ID)
	profileURL := profileBase + "/resources?age=" + v.historical.AgeID + "&radiusKm=10"
	profile := autoRequest(t, s, "GET", profileURL, "")
	profilePayload, _ := json.Marshal(map[string]any{"archiveId": "archive", "longitude": profile["longitude"], "latitude": profile["latitude"], "radiusKm": profile["radiusKm"], "fingerprint": profile["fingerprint"], "format": "txt"})
	profileSaved := autoRequest(t, s, "POST", profileBase+"/resource-reports?age="+v.historical.AgeID, string(profilePayload))
	if profileSaved["id"] == "" || len(s.sessions) != 0 {
		t.Fatal("profile archiving opened a hidden terrain session")
	}
	url := fmt.Sprintf("/api/worlds/%s/resource-surveys?age=%s&archive=archive&area=%s", d.World.ID, v.historical.AgeID, id)
	reviewed := autoRequest(t, s, "GET", url, "")
	stats := reviewed["current"].(map[string]any)
	original := autoRequest(t, s, "GET", url+"&original=1", "")
	if stats["samples"].(float64) == 0 || original["report"].(map[string]any)["areaKm2"].(float64) != report.AreaKM2 {
		t.Fatal("archive did not use saved observations")
	}
	history := reviewed["history"].([]any)
	if len(history) != 1 || history[0].(map[string]any)["ageId"] != v.historical.AgeID {
		t.Fatal("observed Ages merged")
	}
}

func TestResourceFingerprintIgnoresViewMetadataAndCoordinateRoundoff(t *testing.T) {
	a := terrainResourceReport{Longitude: -44.654288, Latitude: 34.081332, RadiusKM: 10, Fields: map[string]resourceStatistic{"temperature": {Mean: 14.123456789}}, Context: &resourceReportContext{Snapshot: "one", Revision: 1}}
	b := a
	b.Context = &resourceReportContext{Snapshot: "two", Revision: 2}
	b.Longitude += 1e-14
	b.MinimumRadiusKM = .05
	b.MaximumRadiusKM = 50
	if resourceFingerprint(a) != resourceFingerprint(b) {
		t.Fatal("metadata or serialization noise invalidates observed data")
	}
	b.Fields = map[string]resourceStatistic{"temperature": {Mean: 15.123456789}}
	if resourceFingerprint(a) == resourceFingerprint(b) {
		t.Fatal("changed physical information accepted as identical")
	}
}

func TestResourceArchiveStatisticsDeduplicateDepositsAndKeepUnits(t *testing.T) {
	circle := resourcesurvey.Circle{Longitude: 0, Latitude: 0, RadiusKM: 10, PlanetRadiusKM: 6371}
	level := resourcesurvey.Level(circle)
	blobs := map[string][]byte{}
	old := terrain.ResourceOccurrence{ID: "ore", Type: "iron", Quantity: 100, QuantityUnit: "tonnes"}
	newer := old
	newer.Quantity = 80
	oldHash := resourcesurvey.Put(blobs, old)
	newHash := resourcesurvey.Put(blobs, newer)
	y1, y2 := int64(1), int64(2)
	a := resourcesurvey.Observation{ID: "old", Circle: circle, Year: &y1, Manifest: resourcesurvey.Manifest{Level: level, Cells: map[string]string{}}}
	b := resourcesurvey.Observation{ID: "new", Circle: circle, Year: &y2, Manifest: resourcesurvey.Manifest{Level: level, Cells: map[string]string{}}}
	for _, cell := range resourcesurvey.Cells(circle, level) {
		a.Manifest.Cells[cell.Key()] = resourcesurvey.Put(blobs, resourcesurvey.State{Fields: map[string]float64{"temperature": 10}, Resources: map[string]string{"ore": oldHash}, Biome: "forest"})
		b.Manifest.Cells[cell.Key()] = resourcesurvey.Put(blobs, resourcesurvey.State{Fields: map[string]float64{"temperature": 20}, Resources: map[string]string{"ore": newHash}, Biome: "grassland"})
	}
	stats, err := archivedStatistics(circle, []resourcesurvey.Observation{a, b}, blobs, nil)
	if err != nil || stats.ResourceCounts["iron"] != 1 || stats.Quantities["iron | tonnes"] != 80 || math.Abs(stats.Fields["temperature"].Mean-20) > 1e-8 {
		t.Fatal("overlap multiplied reserves or lost new values", stats, err)
	}
	before, err := archivedStatistics(circle, []resourcesurvey.Observation{a, b}, blobs, &y1)
	if err != nil || before.Quantities["iron | tonnes"] != 100 {
		t.Fatal("history includes future reserves")
	}
}

func TestArchivedBiologyResolvesOverlapsAndUnknownData(t *testing.T) {
	circle := resourcesurvey.Circle{Longitude: 0, Latitude: 0, RadiusKM: 10, PlanetRadiusKM: 6371}
	small := circle
	small.RadiusKM = 6
	blobs := map[string][]byte{}
	oldBio := spotBiosphere{Available: true, Lifeforms: []spotLifeform{{ID: "plant", Name: "Old plant name", Group: "plant", Abundance: .8}, {ID: "animal", Name: "Animal", Group: "animal", Abundance: .2}}}
	newBio := spotBiosphere{Available: true, Lifeforms: []spotLifeform{{ID: "plant", Name: "New plant name", Group: "plant", Abundance: .3}}}
	aHash, bHash := resourcesurvey.Put(blobs, oldBio), resourcesurvey.Put(blobs, newBio)
	y1, y2 := int64(1), int64(2)
	makeObs := func(id string, c resourcesurvey.Circle, year *int64, bio string) resourcesurvey.Observation {
		m := resourcesurvey.Manifest{Level: resourcesurvey.Level(c), Cells: map[string]string{}}
		for _, cell := range resourcesurvey.Cells(c, m.Level) {
			m.Cells[cell.Key()] = resourcesurvey.Put(blobs, resourcesurvey.State{Biosphere: bio})
		}
		return resourcesurvey.Observation{ID: id, Circle: c, Year: year, Manifest: m}
	}
	a, b := makeObs("old", circle, &y1, aHash), makeObs("new", small, &y2, bHash)
	stats, err := archivedStatistics(circle, []resourcesurvey.Observation{a, b}, blobs, nil)
	if err != nil || stats.LifeformCount != 2 || stats.LifeformGroups["plant"] != 1 || math.Abs(stats.BiologyCoverageFraction-1) > 1e-8 {
		t.Fatal("biology duplicated or lost", stats, err)
	}
	for _, life := range stats.Biosphere.Lifeforms {
		if life.ID == "plant" && (life.Name != "New plant name" || life.Abundance <= .3 || life.Abundance >= .8) {
			t.Fatal("overlapping biology did not preserve newer labels and area-weighted abundance", life)
		}
	}
	unknown := makeObs("unknown", circle, &y2, "")
	stats, err = archivedStatistics(circle, []resourcesurvey.Observation{a, unknown}, blobs, nil)
	if err != nil || stats.BiologyCoverageFraction != 0 || stats.Biosphere.Available {
		t.Fatal("unobserved biology treated as observed extinction")
	}
	emptyHash := resourcesurvey.Put(blobs, spotBiosphere{Available: true, Lifeforms: []spotLifeform{}})
	empty := makeObs("empty", circle, &y2, emptyHash)
	stats, err = archivedStatistics(circle, []resourcesurvey.Observation{a, empty}, blobs, nil)
	if err != nil || !stats.Biosphere.Available || stats.LifeformCount != 0 {
		t.Fatal("recorded absence not retained")
	}
}

func TestResourceArchiveChoicesNearestLocationAndUnknownLast(t *testing.T) {
	d := history.New("Archives")
	age := d.World.CurrentAge
	m, err := d.AddTerrain(age, "Earth", "project", 361, 181)
	if err != nil {
		t.Fatal(err)
	}
	for i, lon := range []float64{20, 1} {
		place := fmt.Sprintf("settlement-%d", i)
		archive := fmt.Sprintf("archive-%d", i)
		d.Entities = append(d.Entities, history.Entity{ID: place, AgeID: age, Kind: "city", Name: place, TerrainID: m.TerrainID}, history.Entity{ID: archive, AgeID: age, Kind: "archive", Name: archive})
		d.Relations = append(d.Relations, history.Relation{ID: archive, AgeID: age, Source: archive, Destination: place, Kind: "archive-located-at"})
		d.Representations = append(d.Representations, history.Representation{ID: place, AgeID: age, EntityID: place, MapID: m.ID, Shape: world.Entity{Points: [][2]float64{{lon + 180, 90}}}})
	}
	d.Entities = append(d.Entities, history.Entity{ID: "unknown", AgeID: age, Kind: "archive", Name: "A first alphabetically"})
	choices := resourceArchiveChoices(d, age, m.TerrainID, resourcesurvey.Circle{Longitude: 0, Latitude: 0, RadiusKM: 10, PlanetRadiusKM: 6371})
	if len(choices) != 3 || choices[0].ID != "archive-1" || choices[1].ID != "archive-0" || choices[2].DistanceKM != nil {
		t.Fatal("archive proximity order incorrect", choices)
	}
}

func TestResourceSurveyEndpointsRespectWorldRoles(t *testing.T) {
	s, db, owner, outsider, d := secureFixture(t)
	base := "/api/worlds/" + d.World.ID + "/resource-surveys?age=" + d.World.CurrentAge + "&archive=private"
	if r := secured(s, outsider, "GET", base, ""); r.Code != 404 {
		t.Fatal("foreign archive leaked", r.Code)
	}
	v := projectFixture(t)
	v.creatorID = owner.u.ID
	s.sessions["survey"] = v
	if r := secured(s, outsider, "GET", "/api/sessions/survey/terrain/resource-archives", ""); r.Code != 404 {
		t.Fatal("foreign session leaked archives", r.Code)
	}
	body := fmt.Sprintf(`{"session":"survey","ageId":%q,"name":"Land"}`, d.World.CurrentAge)
	if r := secured(s, owner, "POST", "/api/worlds/"+d.World.ID+"/accept", body); r.Code != 201 {
		t.Fatal(r.Code, r.Body)
	}
	share(t, db, owner, outsider, d.World.ID, "viewer")
	if r := secured(s, outsider, "POST", "/api/sessions/survey/terrain/resource-reports", `{}`); r.Code != 403 {
		t.Fatal("viewer archived resources", r.Code, r.Body)
	}
	ctx := storage.WithActor(context.Background(), owner.u.ID)
	current, err := db.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	current, err = db.UpdateHistory(ctx, d.World.ID, current.World.Revision, func(d *history.Document) error {
		e := history.Entity{ID: "archive", Kind: "archive", Name: "Resources"}
		return d.Apply(history.Command{Kind: "entity", AgeID: v.historical.AgeID, Entity: &e})
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.applyHistoricalMap(ctx, v); err != nil {
		t.Fatal(err)
	}
	opened := secured(s, owner, "POST", "/api/projects/"+v.worldID+"/open?view=terrain", `{}`)
	if opened.Code != 201 {
		t.Fatal(opened.Code, opened.Body.String()[:min(opened.Body.Len(), 512)])
	}
	var second struct {
		ID          string `json:"id"`
		MapRevision int    `json:"mapRevision"`
	}
	_ = json.Unmarshal(opened.Body.Bytes(), &second)
	path := "/api/sessions/survey/terrain/resources?longitude=0&latitude=0&radiusKm=10"
	preview := secured(s, owner, "GET", path, "")
	var report terrainResourceReport
	if preview.Code != 200 || json.Unmarshal(preview.Body.Bytes(), &report) != nil {
		t.Fatal("preview", preview.Code)
	}
	cached := s.ecologyCache
	beforeWorld, beforeHeader := v.historyRevision, v.authored().Header.Revision
	payload, _ := json.Marshal(map[string]any{"archiveId": "archive", "longitude": 0, "latitude": 0, "radiusKm": 10, "format": "txt", "fingerprint": report.Fingerprint})
	if r := secured(s, owner, "POST", "/api/sessions/survey/terrain/resource-reports", string(payload)); r.Code != 201 {
		t.Fatal("owner could not archive", r.Code, r.Body)
	}
	if r := secured(s, owner, "GET", path, ""); r.Code != 200 || s.ecologyCache != cached {
		t.Fatal("archive save rebuilt ecology", r.Code)
	}
	poll := secured(s, owner, "GET", fmt.Sprintf("/api/sessions/survey/world/names?revision=%d", beforeWorld), "")
	var polled map[string]any
	_ = json.Unmarshal(poll.Body.Bytes(), &polled)
	if poll.Code != 200 || polled["unchanged"] != true || v.authored().Header.Revision != beforeHeader {
		t.Fatal("record-only World polling invalidated terrain geometry", poll.Code, poll.Body)
	}
	req := httptest.NewRequest("POST", "/api/sessions/"+second.ID+"/view", strings.NewReader(`{"ui":{}}`))
	req.AddCookie(&http.Cookie{Name: "__Host-kriemhild", Value: owner.secret})
	req.Header.Set("X-CSRF-Token", owner.csrf)
	req.Header.Set("If-Match", strconv.Itoa(second.MapRevision))
	result := httptest.NewRecorder()
	s.ServeHTTP(result, req)
	if result.Code != 200 {
		t.Fatal("another terrain session was invalidated by a resource report", result.Code, result.Body)
	}
}

func TestContinuousFeatureSourceSurvivesNamesAndChangesWithPhysicalSource(t *testing.T) {
	p := &model.Planet{}
	first := continuousFeatureSource(p)
	if first == "" || continuousFeatureSource(p) != first {
		t.Fatal("immutable geometry identity is unstable")
	}
	clone := *p
	if continuousFeatureSource(&clone) == first {
		t.Fatal("a replacement physical source reused an old geometry identity")
	}
}
