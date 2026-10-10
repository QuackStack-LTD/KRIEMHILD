package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"kriemhild/internal/storage"
	"kriemhild/internal/terrain"
	"kriemhild/internal/world"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestBuilderPersistenceLocalInvalidationAndPortableArchive(t *testing.T) {
	dir := t.TempDir()
	db, err := storage.Open(context.Background(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	v := projectFixture(t)
	s.sessions["map"] = v
	if err = s.saveWorld(v, true); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(v.solver)
	base, err := v.storedDetail(2, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	command := func(kind string, extra map[string]any) map[string]any {
		t.Helper()
		extra["kind"] = kind
		extra["revision"] = v.authored().Header.Revision
		b, _ := json.Marshal(extra)
		return autoRequest(t, s, "POST", "/api/sessions/map/world", string(b))
	}
	command("metadata", map[string]any{"name": "Northern Realms"})
	command("entity", map[string]any{"entity": world.Entity{ID: "capital", Name: "Harbour", Kind: "capital", Layer: "settlements", Geometry: "polygon", Points: [][2]float64{{10, 10}, {12, 10}, {11, 12}}, Color: "#ccbb99"}})
	command("entity", map[string]any{"entity": world.Entity{ID: "castle", Name: "Keep", Kind: "castle", Layer: "buildings", ParentID: "capital", Geometry: "point", Points: [][2]float64{{10.1, 10.1}}, Color: "#abcdef", MinDetail: 4}})
	command("operation", map[string]any{"operation": world.Operation{ID: "crater", Kind: "crater", Layer: "elevation", Center: [2]float64{10, 10}, Radius: 2, Amount: 100}})
	changed, err := v.composedDetail(2, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(changed, base) {
		t.Fatal("crater did not alter terrain")
	}
	var a, b terrain.DetailTile
	json.Unmarshal(base, &a)
	json.Unmarshal(changed, &b)
	at := 8*33 + 8
	if b.Points[at].Elevation >= a.Points[at].Elevation-99 {
		t.Fatal("crater not excavated")
	}
	far, err := v.composedDetail(2, 5, 3)
	if err != nil {
		t.Fatal(err)
	}
	command("operation", map[string]any{"operation": world.Operation{ID: "raise", Kind: "raise", Layer: "elevation", Center: [2]float64{10, 10}, Radius: 1, Amount: 25}})
	if v.composed["2/1/1"] != nil || !bytes.Equal(v.composed["2/5/3"], far) {
		t.Fatal("local edit invalidated incorrect tiles")
	}
	unchanged, _ := v.storedDetail(2, 1, 1)
	after, _ := json.Marshal(v.solver)
	if !bytes.Equal(base, unchanged) || !bytes.Equal(before, after) {
		t.Fatal("Builder mutated generated base/solver")
	}
	command("view", map[string]any{"view": map[string]any{"camera": map[string]any{"center": map[string]any{"x": 10, "y": 10}, "scale": 120}}})
	command("operation", map[string]any{"operation": world.Operation{ID: "authored-river", Kind: "river", Layer: "water", Center: [2]float64{9, 10}, Radius: 2, Amount: 50, Path: [][2]float64{{9, 10}, {12, 11}, {15, 10}}}})
	expected, _ := v.composedDetail(2, 1, 1)
	header := v.authored().Header
	// Stale clients may not overwrite newer state.
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest("POST", "/api/sessions/map/world", bytes.NewBufferString(`{"kind":"metadata","revision":0,"name":"stale"}`)))
	if rec.Code != 409 {
		t.Fatal("missing revision protection", rec.Code)
	}
	if err = s.saveWorld(v, false); err != nil {
		t.Fatal(err)
	}
	worldID := v.worldID
	autoRequest(t, s, "DELETE", "/api/sessions/map", "")
	db.Close()
	db, err = storage.Open(context.Background(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	fresh := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	opened := autoRequest(t, fresh, "POST", "/api/projects/"+worldID+"/open", `{}`)
	restored := fresh.sessions[opened["id"].(string)]
	got, err := restored.composedDetail(2, 1, 1)
	if err != nil || !bytes.Equal(got, expected) {
		t.Fatal("edited detail changed across cold restart", err)
	}
	if !reflect.DeepEqual(header, restored.authored().Header) || restored.world.Entities["castle"].ParentID != "capital" {
		t.Fatal("world hierarchy/view/history not persisted")
	}
	archive, err := encodeProject(restored)
	if err != nil {
		t.Fatal(err)
	}
	files := unzipProject(t, archive)
	imported, err := decodeProject(files, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got, err = imported.composedDetail(2, 1, 1)
	if err != nil || !bytes.Equal(got, expected) || !reflect.DeepEqual(imported.world.Entities, restored.world.Entities) {
		t.Fatal("ZIP lost authored world", err)
	}
	imported.world.Undo(false)
	if !imported.world.Entities["castle"].Deleted || imported.world.Operations["raise"].Deleted {
		t.Fatal("portable undo failed")
	}
	list := autoRequest(t, fresh, "GET", "/api/projects", "")
	projects := list["projects"].([]any)
	if projects[0].(map[string]any)["name"] != "Northern Realms" {
		t.Fatal("project name not in library")
	}
}

func TestWorldRoutesAndViewportValidation(t *testing.T) {
	for _, p := range []string{"/world/0123456789abcdef0123456789abcdef/build", "/world/0123456789abcdef0123456789abcdef/generate"} {
		if !validWorldRoute(p) {
			t.Fatal(p)
		}
	}
	for _, p := range []string{"/assets/missing.js", "/world/nope/build", "/world/0123456789abcdef0123456789abcdef/delete"} {
		if validWorldRoute(p) {
			t.Fatal(p)
		}
	}
	s := New(t.TempDir())
	s.sessions["map"] = projectFixture(t)
	for _, q := range []string{"width=-1", "x=NaN", "detail=999"} {
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, httptest.NewRequest("GET", "/api/sessions/map/world/entities?"+q, nil))
		if rec.Code != 400 {
			t.Fatal("invalid viewport accepted", q)
		}
	}
}

func TestSchemaOneWorldOpensWithEmptyBuilder(t *testing.T) {
	v := projectFixture(t)
	archive, err := encodeProject(v)
	if err != nil {
		t.Fatal(err)
	}
	files := unzipProject(t, archive)
	for name, data := range files {
		if !strings.HasSuffix(name, "/manifest.json") {
			continue
		}
		var manifest projectManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatal(err)
		}
		manifest.Schema = 1
		kept := []projectFile{}
		for _, file := range manifest.Files {
			if file.Path != "world/project.json" {
				kept = append(kept, file)
			}
		}
		manifest.Files = kept
		files[name], _ = json.Marshal(manifest)
		delete(files, strings.TrimSuffix(name, "manifest.json")+"world/project.json")
	}
	loaded, err := decodeProject(files, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.authored().Header.Name != "Unnamed World" || len(loaded.world.Entities) != 0 {
		t.Fatal("legacy project did not get a fresh authored layer")
	}
	if !reflect.DeepEqual(v.solver.Environment.Fields, loaded.solver.Environment.Fields) {
		t.Fatal("migration changed legacy geography")
	}
}
