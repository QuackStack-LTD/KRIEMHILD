package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/storage"
	"kriemhild/internal/terrain"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNaturalLayersZIPAndDatabaseColdRestoration(t *testing.T) {
	v := projectFixture(t)
	// Distinguish persisted state from today's deterministic generator. No
	// reopen/import path may replace an occurrence using seed-only regeneration.
	v.solver.Environment.Resources.Occurrences[0].Quality = .123
	if len(v.solver.Environment.Archipelagos) == 0 {
		t.Fatal("missing archive island fixture")
	}
	v.solver.Environment.Archipelagos[0].Cause += " Preserved provenance."
	geology, _ := json.Marshal(v.solver.Environment.Geology)
	resources, _ := json.Marshal(v.solver.Environment.Resources)
	archipelagos, _ := json.Marshal(v.solver.Environment.Archipelagos)
	archive, err := encodeProject(v)
	if err != nil {
		t.Fatal(err)
	}
	files := unzipProject(t, archive)
	var manifest projectManifest
	if err = json.Unmarshal(files["KRIEMHILD/manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Schema != projectSchema || manifest.GeologyFile != "natural/geology.json" || manifest.ResourcesFile != "natural/resources.json" {
		t.Fatal("missing versioned layer references", manifest)
	}
	if !bytes.Equal(files["KRIEMHILD/"+manifest.GeologyFile], geology) || !bytes.Equal(files["KRIEMHILD/"+manifest.ResourcesFile], resources) {
		t.Fatal("ZIP does not contain exact natural layers")
	}
	if v.dir != "" {
		os.RemoveAll(v.dir)
		v.dir = ""
	}
	dir := t.TempDir()
	db, err := storage.Open(context.Background(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	server := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest("POST", "/api/projects/import", bytes.NewReader(archive)))
	if response.Code != 201 {
		t.Fatal(response.Code, response.Body.String())
	}
	var imported struct {
		ID string `json:"id"`
	}
	json.Unmarshal(response.Body.Bytes(), &imported)
	loaded := server.sessions[imported.ID]
	assertLayers := func(v *session) {
		t.Helper()
		g, _ := json.Marshal(v.solver.Environment.Geology)
		r, _ := json.Marshal(v.solver.Environment.Resources)
		islands, _ := json.Marshal(v.solver.Environment.Archipelagos)
		if !bytes.Equal(islands, archipelagos) {
			t.Fatal("saved island foundations regenerated or relocated")
		}
		if !bytes.Equal(g, geology) || !bytes.Equal(r, resources) {
			t.Fatal("stored geology/resources regenerated or relocated")
		}
	}
	assertLayers(loaded)
	beforeQuery := autoRequest(t, server, "GET", "/api/sessions/"+imported.ID+"/world/natural?x=20&y=15&radius=3", "")
	if beforeQuery["available"] != true {
		t.Fatal("natural layers not queryable")
	}
	autoRequest(t, server, "POST", "/api/sessions/"+imported.ID+"/save", `{}`)
	worldID := loaded.worldID
	autoRequest(t, server, "DELETE", "/api/sessions/"+imported.ID, "")
	db.Close()
	db, err = storage.Open(context.Background(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	fresh := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	opened := autoRequest(t, fresh, "POST", "/api/projects/"+worldID+"/open", `{}`)
	restored := fresh.sessions[opened["id"].(string)]
	assertLayers(restored)
	afterQuery := autoRequest(t, fresh, "GET", "/api/sessions/"+opened["id"].(string)+"/world/natural?x=20&y=15&radius=3", "")
	a, _ := json.Marshal(beforeQuery)
	b, _ := json.Marshal(afterQuery)
	if !bytes.Equal(a, b) {
		t.Fatal("hover results changed across cold restart")
	}
	if restored.dir != "" {
		defer os.RemoveAll(restored.dir)
	}
	// Schema 1 and 2 worlds remain loadable without inventing resources.
	for _, schema := range []int{1, 2} {
		legacy := projectFixture(t)
		legacy.solver.Environment.Climate = nil
		legacy.solver.Environment.Geology = nil
		legacy.solver.Environment.Geomorphology = nil
		legacy.solver.Environment.Resources = nil
		blob, err := encodeProject(legacy)
		if err != nil {
			t.Fatal(err)
		}
		oldFiles := unzipProject(t, blob)
		var m projectManifest
		json.Unmarshal(oldFiles["KRIEMHILD/manifest.json"], &m)
		m.Schema = schema
		oldFiles["KRIEMHILD/manifest.json"], _ = json.Marshal(m)
		restored, err := decodeProject(oldFiles)
		if err != nil {
			t.Fatal(schema, err)
		}
		if restored.dir != "" {
			defer os.RemoveAll(restored.dir)
		}
		if restored.solver.Environment.Geology != nil || restored.solver.Environment.Resources != nil {
			t.Fatal("legacy import invented natural layers")
		}
	}
}

func TestNaturalQueryBoundsAndManifestValidation(t *testing.T) {
	v := projectFixture(t)
	s := New(t.TempDir())
	s.sessions["natural-test"] = v
	for _, query := range []string{"x=-1&y=2", "x=1&y=99", "x=NaN&y=2", "x=1&y=2&radius=21", "x=1&y=2&radius=-1", "y=2"} {
		r := httptest.NewRecorder()
		s.ServeHTTP(r, httptest.NewRequest("GET", "/api/sessions/natural-test/world/natural?"+query, nil))
		if r.Code != 400 {
			t.Fatal(query, r.Code)
		}
	}
	archive, err := encodeProject(v)
	if err != nil {
		t.Fatal(err)
	}
	files := unzipProject(t, archive)
	var m projectManifest
	json.Unmarshal(files["KRIEMHILD/manifest.json"], &m)
	m.ResourcesFile = "missing.json"
	files["KRIEMHILD/manifest.json"], _ = json.Marshal(m)
	if _, err := decodeProject(files); err == nil {
		t.Fatal("missing natural file reference accepted")
	}
	// Listing potential hosts must not create resources as a query side effect.
	v.solver.Environment.Resources.Occurrences = nil
	v.natural = nil
	result := autoRequest(t, s, "GET", fmt.Sprintf("/api/sessions/natural-test/world/natural?x=%d&y=%d&radius=0", 20, 15), "")
	if len(result["goods"].([]any)) != 0 {
		t.Fatal("hover invented occurrences")
	}
}

func TestNaturalMaximumPhysicalWorldArchive(t *testing.T) {
	options, err := terrain.DecodeEnvironment([]byte(`{"columns":192,"rows":128,"realism":true,"seed":"maximum-resources"}`))
	if err != nil {
		t.Fatal(err)
	}
	config, err := terrain.DecodeConfig(terrain.Defaults)
	if err != nil {
		t.Fatal(err)
	}
	solver, err := terrain.PrepareEnvironment(options, config)
	if err != nil {
		t.Fatal(err)
	}
	for solver.Status == "running" {
		solver.Step()
	}
	v := &session{solver: solver}
	archive, err := encodeProject(v)
	if err != nil {
		t.Fatal(err)
	}
	files := unzipProject(t, archive)
	t.Logf("natural layer bytes: geology=%d resources=%d", len(files["KRIEMHILD/natural/geology.json"]), len(files["KRIEMHILD/natural/resources.json"]))
	projected := renderEnvironment(solver.Environment)
	if projected.Geology != nil || projected.Resources != nil || solver.Environment.Geology == nil || solver.Environment.Resources == nil {
		t.Fatal("render projection lost or transferred invisible data")
	}
}
