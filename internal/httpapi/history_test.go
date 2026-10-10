package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"kriemhild/internal/world"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestHistoricalHierarchySurvivesRestartAndPortableArchive(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := storage.Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	v := projectFixture(t)
	s.sessions["terrain"] = v
	d := history.New("A fictional reality")
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	first := d.World.CurrentAge
	tile, err := v.storedDetail(3, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{"session": "terrain", "ageId": first, "name": "Terra"})
	autoRequest(t, s, "POST", "/api/worlds/"+d.World.ID+"/accept", string(payload))
	d, _ = db.History(ctx, d.World.ID)
	m := d.Maps[0]
	if len(d.Terrains) != 1 || m.Snapshot == "" {
		t.Fatal("terrain not accepted into hierarchy")
	}
	city := world.Entity{ID: "capital", Name: "Capital", Kind: "city", Layer: "settlements", Geometry: "polygon", Points: [][2]float64{{8, 8}, {11, 8}, {12, 10}, {11, 12}, {8, 12}, {7, 10}}, Color: "#000000"}
	if _, err = v.authored().PutEntity(city); err != nil {
		t.Fatal(err)
	}
	if err = s.saveWorld(v, false); err != nil {
		t.Fatal(err)
	}
	d, _ = db.History(ctx, d.World.ID)
	d, err = db.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(d *history.Document) error {
		return d.Apply(history.Command{Kind: "age", AgeID: first, Name: "Second Age", Mode: "after"})
	})
	if err != nil {
		t.Fatal(err)
	}
	second := d.World.CurrentAge
	future := *d.Map(second, m.ID)
	if future.ProjectID == m.ProjectID {
		t.Fatal("two Ages share a mutable session identity")
	}
	originalSnapshot := d.Map(first, m.ID).Snapshot
	if future.Snapshot != originalSnapshot {
		t.Fatal("unchanged terrain was duplicated")
	}
	// Open the inherited version through the ordinary editor API.
	opened := autoRequest(t, s, "POST", "/api/projects/"+future.ProjectID+"/open", `{}`)
	fv := s.sessions[opened["id"].(string)]
	city.Name = "Future Capital"
	fv.authored().PutEntity(city)
	if err = s.saveWorld(fv, false); err != nil {
		t.Fatal(err)
	}
	d, _ = db.History(ctx, d.World.ID)
	if d.Shapes(*d.Map(first, m.ID))["capital"].Name != "Capital" {
		t.Fatal("future rewrote historical entity")
	}
	var child history.Map
	d, err = db.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(d *history.Document) error {
		var err error
		child, err = d.Child(second, m.ID, "capital", "City interior", world.Bounds{})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	opened = autoRequest(t, s, "POST", "/api/projects/"+child.ProjectID+"/open", `{}`)
	cv := s.sessions[opened["id"].(string)]
	if !reflect.DeepEqual(cv.world.Entities["capital"].Points, city.Points) {
		t.Fatal("child lost footprint")
	}
	building := world.Entity{ID: "keep", Name: "Keep", Kind: "castle", Layer: "buildings", Geometry: "point", Points: [][2]float64{{9, 9}}, Color: "#000000"}
	cv.world.PutEntity(building)
	city.Points[0][0] = 7.5
	cv.world.PutEntity(city)
	if err = s.saveWorld(cv, false); err != nil {
		t.Fatal(err)
	}
	d, _ = db.History(ctx, d.World.ID)
	if _, ok := d.Shapes(*d.Map(second, m.ID))["keep"]; ok {
		t.Fatal("city interior leaked into parent")
	}
	if d.Shapes(*d.Map(second, m.ID))["capital"].Points[0][0] != 7.5 {
		t.Fatal("footprint not propagated")
	}
	if d.Shapes(*d.Map(first, m.ID))["capital"].Points[0][0] != 8 {
		t.Fatal("footprint leaked into earlier Age")
	}
	// Saving an older open parent without editing its geometry must not undo
	// the child's new footprint.
	if err = s.saveWorld(fv, false); err != nil {
		t.Fatal(err)
	}
	d, _ = db.History(ctx, d.World.ID)
	if d.Shapes(*d.Map(second, m.ID))["capital"].Points[0][0] != 7.5 {
		t.Fatal("stale parent overwrote child footprint")
	}
	export := httptest.NewRecorder()
	s.ServeHTTP(export, httptest.NewRequest("GET", "/api/worlds/"+d.World.ID+"/export", nil))
	if export.Code != 200 {
		t.Fatal(export.Code, export.Body.String())
	}
	archive := append([]byte(nil), export.Body.Bytes()...)
	db.Close()
	db, err = storage.Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	d2, err := db.History(ctx, d.World.ID)
	if err != nil || !reflect.DeepEqual(d, d2) {
		t.Fatal("database restart changed hierarchy", err)
	}
	// A genuinely independent database has no original cache or stored blobs.
	clean, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer clean.Close()
	server := NewWithOptions(t.TempDir(), Options{Projects: clean, CacheDir: t.TempDir()})
	// Missing snapshot data must fail before adding even the World metadata.
	files := unzipProject(t, archive)
	var corrupt bytes.Buffer
	writer := zip.NewWriter(&corrupt)
	omitted := false
	for path, data := range files {
		if path != "manifest.json" && !omitted {
			omitted = true
			continue
		}
		entry, err := writer.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	rejected := httptest.NewRecorder()
	server.ServeHTTP(rejected, httptest.NewRequest("POST", "/api/worlds/import", bytes.NewReader(corrupt.Bytes())))
	if rejected.Code != 422 {
		t.Fatalf("missing terrain blob accepted: %d", rejected.Code)
	}
	if worlds, err := clean.Histories(ctx); err != nil || len(worlds) != 0 {
		t.Fatal("invalid import partially committed", err)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest("POST", "/api/worlds/import", bytes.NewReader(archive)))
	if response.Code != 201 {
		t.Fatal(response.Code, response.Body.String())
	}
	var imported history.Document
	json.Unmarshal(response.Body.Bytes(), &imported)
	if imported.World.ID != d.World.ID || imported.World.CurrentAge != d.World.CurrentAge || imported.Map(first, m.ID).ProjectID != m.ProjectID {
		t.Fatal("fresh-server import changed historical identities")
	}
	if len(imported.Ages) != 2 || len(imported.Maps) != 3 || len(imported.Entities) != 3 {
		t.Fatalf("archive lost hierarchy: ages=%d maps=%d entities=%d", len(imported.Ages), len(imported.Maps), len(imported.Entities))
	}
	restoredMap := imported.Map(first, m.ID)
	opened = autoRequest(t, server, "POST", "/api/projects/"+restoredMap.ProjectID+"/open", `{}`)
	restored := server.sessions[opened["id"].(string)]
	got, err := restored.storedDetail(3, 2, 1)
	if err != nil || !bytes.Equal(tile, got) {
		t.Fatal("explored terrain changed after import", err)
	}
}
