package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"kriemhild/internal/history"
	"kriemhild/internal/legacyterrain"
	"kriemhild/internal/storage"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/terrain/generate"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/world"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestContinuousArchiveRoundTrip(t *testing.T) {
	spec := model.DefaultSpec()
	spec.SurfaceWidth = 1024
	p, err := generate.Generate(context.Background(), spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	solver, err := legacyProjection(p)
	if err != nil {
		t.Fatal(err)
	}
	v := &session{planet: p, solver: solver, worldID: "12345678901234567890123456789012", last: time.Now(), undo: map[string]terrain.Snapshot{}, cacheRoot: t.TempDir()}
	e := world.Entity{ID: "town", Name: "Old town", Kind: "town", Layer: "settlements", Geometry: "polygon", Points: [][2]float64{{70, 40}, {71, 40}, {70, 41}}, Color: "#aa8866"}
	if _, err = v.authored().PutEntity(e); err != nil {
		t.Fatal(err)
	}
	art := world.Illustration{Version: 1, ID: "medieval-world", Title: "World atlas", Scope: "world", Style: "medieval", Bounds: [4]float64{0, 0, 2, 1}, Layers: []world.IllustrationLayer{{ID: "labels", Name: "Labels", Visible: true, Opacity: 1}}, Objects: map[string]world.IllustrationObject{"label-town": {ID: "label-town", Source: "town", Kind: "text", Layer: "labels", Position: &[2]float64{.8, .4}}}}
	if err := v.authored().PutIllustration(art); err != nil {
		t.Fatal(err)
	}
	archive, err := encodeProject(v)
	if err != nil {
		t.Fatal(err)
	}
	_ = archive
	// Decode through the same checksum-validating archive importer used by projects.
	z, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[strings.TrimPrefix(f.Name, "KRIEMHILD/")] = b
	}
	restored, err := decodeProject(files, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if restored.planet == nil || !reflect.DeepEqual(v.planet.Grid, restored.planet.Grid) || !reflect.DeepEqual(v.planet.Masses, restored.planet.Masses) || restored.world.Entities["town"].Name != "Old town" {
		t.Fatal("archive lost terrain or authored identity")
	}
	if !reflect.DeepEqual(restored.world.Header.Illustrations[art.ID], art) {
		t.Fatal("archive lost editable illustration")
	}
}

func TestContinuousAgeIsolationAndColdRestart(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	spec := model.DefaultSpec()
	spec.SurfaceWidth = 1024
	p, err := generate.Generate(ctx, spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	solver, err := legacyProjection(p)
	if err != nil {
		t.Fatal(err)
	}
	v := &session{planet: p, solver: solver, worldID: token(), last: time.Now(), undo: map[string]terrain.Snapshot{}, cacheRoot: t.TempDir()}
	server := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	server.sessions["continuous"] = v
	doc := history.New("Continuous test world")
	if err = db.CreateHistory(ctx, doc); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"session": "continuous", "ageId": doc.World.CurrentAge, "name": "Planet"})
	autoRequest(t, server, "POST", "/api/worlds/"+doc.World.ID+"/accept", string(body))
	first := doc.World.CurrentAge
	doc, _ = db.History(ctx, doc.World.ID)
	m := doc.Maps[0]
	doc, err = db.UpdateHistory(ctx, doc.World.ID, doc.World.Revision, func(d *history.Document) error {
		return d.Apply(history.Command{Kind: "age", AgeID: first, Name: "Second Age", Mode: "after"})
	})
	if err != nil {
		t.Fatal(err)
	}
	future := doc.Map(doc.World.CurrentAge, m.ID)
	opened := autoRequest(t, server, "POST", "/api/projects/"+future.ProjectID+"/open", `{}`)
	fv := server.sessions[opened["id"].(string)]
	if fv.planet == nil || fv.world.Header.Version != 2 {
		t.Fatal("historical snapshot lost continuous data")
	}
	if _, err = fv.authored().AddOperation(world.Operation{ID: "edit", Kind: "raise", Layer: "elevation", Center: [2]float64{20, 20}, Radius: 1, Amount: 200}); err != nil {
		t.Fatal(err)
	}
	if err = server.saveWorld(fv, false); err != nil {
		t.Fatal(err)
	}
	if len(v.authored().Operations) != 0 {
		t.Fatal("edit leaked to prior Age")
	}
	parts, err := db.LoadParts(ctx, fv.worldID)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) == 0 {
		d, _ := db.History(ctx, doc.World.ID)
		parts, err = db.SnapshotParts(ctx, d.Map(doc.World.CurrentAge, m.ID).Snapshot)
		if err != nil {
			t.Fatal(err)
		}
	}
	cold, err := restoreCheckpoint(fv.worldID, parts, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cold.planet == nil || len(cold.world.Operations) != 1 || !reflect.DeepEqual(cold.planet.Grid, p.Grid) {
		t.Fatal("cold restore lost terrain or edits")
	}
	if detail.Sample(cold.planet, .3, .4) != detail.Sample(p, .3, .4) {
		t.Fatal("sampling changed on restart")
	}
}

func TestContinuousJobCancellationAndValidation(t *testing.T) {
	s := New(t.TempDir())
	response := httptest.NewRecorder()
	s.ServeHTTP(response, httptest.NewRequest("POST", "/api/terrain/generation-jobs", bytes.NewBufferString(`{"spec":{"seed":"test","continents":-3}}`)))
	if response.Code != 400 {
		t.Fatal("invalid spec accepted", response.Code)
	}
	response = httptest.NewRecorder()
	s.ServeHTTP(response, httptest.NewRequest("POST", "/api/terrain/generation-jobs", bytes.NewBufferString(`{}`)))
	if response.Code != 202 {
		t.Fatal(response.Code, response.Body.String())
	}
	var job struct {
		ID string `json:"id"`
	}
	json.Unmarshal(response.Body.Bytes(), &job)
	response = httptest.NewRecorder()
	s.ServeHTTP(response, httptest.NewRequest("DELETE", "/api/terrain/generation-jobs/"+job.ID, nil))
	if response.Code != 204 {
		t.Fatal(response.Code)
	}
	response = httptest.NewRecorder()
	s.ServeHTTP(response, httptest.NewRequest("GET", "/api/terrain/generation-jobs/"+job.ID, nil))
	var state map[string]any
	json.Unmarshal(response.Body.Bytes(), &state)
	if state["status"] != "cancelled" {
		t.Fatal(state)
	}
}
