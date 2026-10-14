package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"kriemhild/internal/history"
	"kriemhild/internal/legacyterrain"
	"kriemhild/internal/storage"
	"kriemhild/internal/world"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"
)

func TestSystemReviewRestartAndZIP(t *testing.T) {
	ctx, dir := context.Background(), t.TempDir()
	db, err := storage.Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	server := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	d := history.New("System workshop")
	age := d.World.CurrentAge
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	fixture := projectFixture(t)
	server.sessions["terrain"] = fixture
	raw, _ := json.Marshal(map[string]any{"session": "terrain", "ageId": age, "name": "Source planet"})
	autoRequest(t, server, "POST", "/api/worlds/"+d.World.ID+"/accept", string(raw))
	d, _ = db.History(ctx, d.World.ID)
	terrainID := d.Terrains[0].ID
	if fixture.solver.Environment.Resources == nil || len(fixture.solver.Environment.Resources.Occurrences) == 0 {
		t.Fatal("fixture needs generated sources")
	}
	occurrence := fixture.solver.Environment.Resources.Occurrences[0]
	for _, e := range []history.Entity{
		{ID: "system", Kind: "system", Name: "Crystal engineering", System: &history.SystemRecord{Version: 1, Category: "hybrid", Mode: "advanced"}},
		{ID: "rule", Kind: "system-rule", Name: "Conservation", System: &history.SystemRecord{Version: 1, SystemID: "system", Classification: "hard", Fields: []history.TypedField{{Definition: history.FieldDefinition{ID: "equation", Name: "Equation", Type: "text"}, Value: json.RawMessage(`"E = stored energy"`)}}}},
		{ID: "fuel", Kind: "system-resource", Name: "Catalyst role", System: &history.SystemRecord{Version: 1, SystemID: "system", Sources: []history.SystemSource{{TerrainID: terrainID, OccurrenceID: occurrence.ID, Notes: "Selected existing deposit"}}}},
		{ID: "capability", Kind: "system-capability", Name: "Teleportation", System: &history.SystemRecord{Version: 1, SystemID: "system", Possibility: "possible"}},
		{ID: "portal", Kind: "system-implementation", Name: "North gate", System: &history.SystemRecord{Version: 1, SystemID: "system", Development: "prototype"}, Properties: json.RawMessage(`{"startYear":83}`)},
		{ID: "person", Kind: "character", Name: "Scholar"},
		{ID: "city", Kind: "city", Name: "Remote city"},
	} {
		if err = d.Apply(history.Command{Kind: "entity", AgeID: age, Entity: &e}); err != nil {
			t.Fatal(err)
		}
	}
	for i, pair := range [][3]string{{"capability", "rule", "system-requires"}, {"portal", "capability", "system-requires"}, {"portal", "city", "system-implemented-in"}, {"person", "rule", "system-knowledge"}} {
		r := history.Relation{ID: []string{"support", "gate", "location", "knowledge"}[i], Source: pair[0], Destination: pair[1], Kind: pair[2]}
		if r.Kind == "system-knowledge" {
			r.Status = "incorrect-theory"
		}
		if err = d.Apply(history.Command{Kind: "relation", AgeID: age, Relation: &r}); err != nil {
			t.Fatal(err)
		}
	}
	// A remote, defined footprint is needed to distinguish supply gaps from
	// unknown locations. Its coordinates remain in the parent terrain CRS.
	d.Representations = append(d.Representations, history.Representation{ID: "remote-footprint", AgeID: age, MapID: d.Maps[0].ID, EntityID: "city", Shape: world.Entity{ID: "city-shape", LogicalID: "city", Layer: "settlements", Color: "#000000", Name: "Remote city", Kind: "city", Geometry: "polygon", Points: [][2]float64{{0.01, 0.01}, {0.1, 0.01}, {0.1, 0.1}, {0.01, 0.1}}}})
	// Save all logical records through the same transactional store used by commands.
	current, _ := db.History(ctx, d.World.ID)
	d, err = db.UpdateHistory(ctx, d.World.ID, current.World.Revision, func(saved *history.Document) error {
		saved.Entities = d.Entities
		saved.Relations = d.Relations
		saved.Representations = d.Representations
		saved.Years = d.Years
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	call := func(s *Server, method, path string, body any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		rr := httptest.NewRecorder()
		s.ServeHTTP(rr, httptest.NewRequest(method, path, bytes.NewReader(b)))
		return rr
	}
	endpoint := "/api/worlds/" + d.World.ID + "/systems/analyze"
	dep := history.Relation{ID: "new-fuel", Source: "rule", Destination: "fuel", Kind: "system-consumes"}
	draft := history.Command{Kind: "batch", Commands: []history.Command{{Kind: "relation", Relation: &dep}}}
	savedBefore, _ := db.History(ctx, d.World.ID)
	before, _ := json.Marshal(savedBefore)
	response := call(server, "POST", endpoint, map[string]any{"ageId": age, "systemId": "system", "changedId": "rule", "revision": d.World.Revision, "draft": draft})
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	var result struct {
		Analysis history.SystemAnalysis `json:"analysis"`
		Supply   []systemSupply         `json:"supply"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Analysis.Contexts) != 2 {
		t.Fatalf("knowledge holders and affected locations missing: %+v", result.Analysis.Contexts)
	}
	if len(result.Analysis.Affected) != 2 || len(result.Supply) != 1 || result.Supply[0].Status != "supply-gap" {
		t.Fatalf("impact or source context missing: %s", response.Body.String())
	}
	loaded, _ := db.History(ctx, d.World.ID)
	after, _ := json.Marshal(loaded)
	if !bytes.Equal(before, after) {
		t.Fatal("draft review wrote to database")
	}
	response = call(server, "POST", endpoint, map[string]any{"ageId": age, "revision": -1})
	if response.Code != 409 {
		t.Fatal("stale revision accepted")
	}
	if err = d.Apply(history.Command{Kind: "relation", AgeID: age, Relation: &dep}); err != nil {
		t.Fatal(err)
	}
	if err = d.Apply(history.Command{Kind: "age", AgeID: age, Name: "Later", Mode: "branch"}); err != nil {
		t.Fatal(err)
	}
	later := d.World.CurrentAge
	revised := *d.Entity(later, "rule")
	revised.System = &history.SystemRecord{Version: 1, SystemID: "system", Classification: "hypothesis", Statement: "A revised model"}
	if err = d.Apply(history.Command{Kind: "entity", AgeID: later, Entity: &revised}); err != nil {
		t.Fatal(err)
	}
	d, err = db.UpdateHistory(ctx, d.World.ID, loaded.World.Revision, func(saved *history.Document) error { *saved = *d; return nil })
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = storage.Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	d, err = db.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	server = NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	export := httptest.NewRecorder()
	server.ServeHTTP(export, httptest.NewRequest("GET", "/api/worlds/"+d.World.ID+"/export", nil))
	if export.Code != 200 {
		t.Fatal(export.Body.String())
	}
	target, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	fresh := NewWithOptions(t.TempDir(), Options{Projects: target, CacheDir: t.TempDir()})
	imported := httptest.NewRecorder()
	fresh.ServeHTTP(imported, httptest.NewRequest("POST", "/api/worlds/import", bytes.NewReader(export.Body.Bytes())))
	if imported.Code != 201 && imported.Code != 200 {
		t.Fatal(imported.Code, imported.Body.String())
	}
	restored, err := target.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	sortEntities := func(v []history.Entity) {
		sort.Slice(v, func(i, j int) bool { return v[i].AgeID+v[i].ID < v[j].AgeID+v[j].ID })
	}
	sortEntities(d.Entities)
	sortEntities(restored.Entities)
	sortRelations := func(v []history.Relation) {
		sort.Slice(v, func(i, j int) bool { return v[i].AgeID+v[i].ID < v[j].AgeID+v[j].ID })
	}
	sortRelations(d.Relations)
	sortRelations(restored.Relations)
	if !reflect.DeepEqual(d.Entities, restored.Entities) || !reflect.DeepEqual(d.Relations, restored.Relations) || !reflect.DeepEqual(d.Years, restored.Years) {
		t.Fatalf("ZIP comparison entities=%v relations=%v years=%v\n%+v\n%+v", reflect.DeepEqual(d.Entities, restored.Entities), reflect.DeepEqual(d.Relations, restored.Relations), reflect.DeepEqual(d.Years, restored.Years), d.Years, restored.Years)
	}
	response = call(fresh, "GET", "/api/worlds/"+d.World.ID+"/systems/sources?age="+age+"&terrain="+terrainID, nil)
	if response.Code != 200 || !bytes.Contains(response.Body.Bytes(), []byte(occurrence.ID)) {
		t.Fatal("saved source cannot be resolved after import without original server")
	}
	preview := call(fresh, "GET", "/api/worlds/"+d.World.ID+"/maps/"+restored.Maps[0].ID+"/preview?age="+restored.Maps[0].AgeID+"&resource="+occurrence.ID, nil)
	if preview.Code != 200 || preview.Header().Get("Content-Type") != "image/png" {
		t.Fatal("source highlight lost after export:", preview.Code, preview.Body.String())
	}
	if restored.Entity(age, "rule").System.Classification != "hard" || restored.Entity(later, "rule").System.Classification != "hypothesis" {
		t.Fatal("historical isolation lost")
	}
}

func TestSystemSourceGeometry(t *testing.T) {
	d := history.New("Sources")
	age := d.World.CurrentAge
	d.Maps = []history.Map{{ID: "m", AgeID: age, TerrainID: "terrain"}}
	d.Representations = []history.Representation{{AgeID: age, MapID: "m", EntityID: "town", Shape: world.Entity{Geometry: "polygon", Points: [][2]float64{{1, 1}, {5, 1}, {5, 5}, {1, 5}}}}}
	if !sourceInPlace(d, age, "town", "terrain", 10, terrain.NaturalGeometry{Cells: []int{33}}) {
		t.Fatal("local cell not found")
	}
	if sourceInPlace(d, age, "town", "other-planet", 10, terrain.NaturalGeometry{Cells: []int{33}}) {
		t.Fatal("matched different terrain")
	}
	if !sourceInPlace(d, age, "town", "terrain", 10, terrain.NaturalGeometry{Path: [][2]float64{{0, 3}, {6, 3}}}) {
		t.Fatal("source corridor crossing place was missed")
	}
	d.Representations[0].Shape.Points = [][2]float64{{3.1, 3.1}, {3.2, 3.1}, {3.2, 3.2}, {3.1, 3.2}}
	if !sourceInPlace(d, age, "town", "terrain", 10, terrain.NaturalGeometry{Cells: []int{33}}) {
		t.Fatal("cell footprint intersection was missed")
	}
	if sourceInPlace(d, age, "town", "terrain", 10, terrain.NaturalGeometry{Cells: []int{88}}) {
		t.Fatal("distant deposit counted as local")
	}
}
