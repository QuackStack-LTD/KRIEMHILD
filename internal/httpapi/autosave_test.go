package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/storage"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
)

func autoRequest(t *testing.T, s *Server, method, path, body string) map[string]any {
	t.Helper()
	r := httptest.NewRecorder()
	s.ServeHTTP(r, httptest.NewRequest(method, path, bytes.NewBufferString(body)))
	if r.Code < 200 || r.Code >= 300 {
		t.Fatalf("%s: %d %s", path, r.Code, r.Body.String())
	}
	var result map[string]any
	if r.Body.Len() > 0 {
		if err := json.Unmarshal(r.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func TestAutosaveResumesExactUnfinishedSolverAfterRestart(t *testing.T) {
	dir := t.TempDir()
	db, err := storage.Open(context.Background(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	created := autoRequest(t, s, "POST", "/api/sessions", `{"options":{"width":24,"height":24,"radius2":2,"seed":12345}}`)
	id := created["id"].(string)
	v := s.sessions[id]
	autoRequest(t, s, "POST", "/api/sessions/"+id+"/step", `{"count":3}`)
	if v.solver.Status != "running" {
		t.Fatal("fixture must be unfinished")
	}
	autoRequest(t, s, "POST", "/api/sessions/"+id+"/autosave", `{"ui":{"camera2d":{"scale":90},"paused":true},"client":"test","sequence":2}`)
	// A delayed page request must not overwrite the last camera state.
	autoRequest(t, s, "POST", "/api/sessions/"+id+"/autosave", `{"ui":{"camera2d":{"scale":1}},"client":"test","sequence":1}`)
	before, _ := json.Marshal(v.solver)
	world := v.worldID
	autoRequest(t, s, "DELETE", "/api/sessions/"+id, "")
	db.Close()
	db, err = storage.Open(context.Background(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	fresh := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	opened := autoRequest(t, fresh, "POST", "/api/projects/"+world+"/open", `{}`)
	restored := fresh.sessions[opened["id"].(string)]
	after, _ := json.Marshal(restored.solver)
	if !bytes.Equal(before, after) {
		t.Fatal("running solver/RNG/queues changed across restart")
	}
	if !bytes.Contains(restored.projectUI, []byte(`90`)) {
		t.Fatal("stale camera request won")
	}
	for i := 0; i < 80; i++ {
		v.solver.Step()
		restored.solver.Step()
	}
	if !reflect.DeepEqual(v.solver.Dom, restored.solver.Dom) || v.solver.RNG != restored.solver.RNG {
		t.Fatal("resumed solve diverged")
	}
}

func TestAutosaveExplorationAndEditsWithoutExport(t *testing.T) {
	db, err := storage.Open(context.Background(), t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	v := projectFixture(t)
	s.sessions["map"] = v
	// The fixture predates the database; first action establishes its checkpoint.
	autoRequest(t, s, "POST", "/api/sessions/map/cleanup", `{"count":1}`)
	typeIndex := v.solver.TypeAt(0)
	autoRequest(t, s, "POST", "/api/sessions/map/paint", fmt.Sprintf(`{"cells":[0],"type":%d}`, typeIndex))
	for v.solver.Status == "running" {
		autoRequest(t, s, "POST", "/api/sessions/map/step", `{"count":10000}`)
	}
	autoRequest(t, s, "GET", "/api/sessions/map/detail/3/2/1", "")
	tile, err := v.storedDetail(3, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	partsBefore, err := db.LoadParts(context.Background(), v.worldID)
	if err != nil {
		t.Fatal(err)
	}
	autoRequest(t, s, "POST", "/api/sessions/map/autosave", `{"ui":{"camera2d":{"scale":140},"offsets":[1,2]}}`)
	autoRequest(t, s, "POST", "/api/sessions/map/autosave", `{"ui":{"camera2d":{"scale":160}},"merge":true}`)
	partsAfter, _ := db.LoadParts(context.Background(), v.worldID)
	for _, key := range []string{"checkpoint", "environment", "tile/3/2/1"} {
		if !bytes.Equal(partsBefore[key], partsAfter[key]) {
			t.Fatal("camera save rewrote geography", key)
		}
	}
	original := v.solver.Snapshot()
	world := v.worldID
	autoRequest(t, s, "DELETE", "/api/sessions/map", "")
	if _, err = os.Stat(v.dir); !os.IsNotExist(err) {
		t.Fatal("cache survived")
	}
	fresh := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	opened := autoRequest(t, fresh, "POST", "/api/projects/"+world+"/open", `{}`)
	restored := fresh.sessions[opened["id"].(string)]
	if !reflect.DeepEqual(restored.solver.Snapshot(), original) {
		t.Fatal("edited world changed")
	}
	got, err := restored.storedDetail(3, 2, 1)
	if err != nil || !bytes.Equal(tile, got) {
		t.Fatal("explored tile regenerated", err)
	}
	if !bytes.Contains(restored.projectUI, []byte(`"offsets":[1,2]`)) {
		t.Fatal("compact view save lost existing display state")
	}
	// ZIP export remains self-contained after incremental database restoration.
	archive, err := encodeProject(restored)
	if err != nil {
		t.Fatal(err)
	}
	files, err := readProjectFiles(httptest.NewRequest("POST", "/", bytes.NewReader(archive)))
	if err != nil {
		t.Fatal(err)
	}
	imported, err := decodeProject(files, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got, err = imported.storedDetail(3, 2, 1)
	if err != nil || !bytes.Equal(tile, got) {
		t.Fatal("autosaved detail missing from ZIP", err)
	}
	// Importing a backup of an active world must retire all stale writers.
	autoRequest(t, fresh, "POST", "/api/projects/import", string(archive))
	if !restored.retired {
		t.Fatal("import left competing world session active")
	}
	if err = fresh.autosave(restored, true); err == nil {
		t.Fatal("stale session overwrote imported world")
	}
}

func TestAutosaveFailureRetainsDirtyCheckpointForRetry(t *testing.T) {
	dir := t.TempDir()
	db, err := storage.Open(context.Background(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	created := autoRequest(t, s, "POST", "/api/sessions", `{"options":{"width":24,"height":24,"radius2":2,"seed":12}}`)
	id := created["id"].(string)
	v := s.sessions[id]
	before, _ := db.LoadParts(context.Background(), v.worldID)
	db.Close()
	result := autoRequest(t, s, "POST", "/api/sessions/"+id+"/step", `{"count":3}`)
	if result["autosaveError"] == nil || !v.dirty {
		t.Fatal("failed save was silently accepted")
	}
	db, err = storage.Open(context.Background(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s.projects = db
	after, _ := db.LoadParts(context.Background(), v.worldID)
	if !bytes.Equal(before["checkpoint"], after["checkpoint"]) {
		t.Fatal("failed save corrupted committed checkpoint")
	}
	autoRequest(t, s, "POST", "/api/sessions/"+id+"/autosave", `{"ui":{}}`)
	if v.dirty {
		t.Fatal("dirty state not cleared after commit")
	}
	parts, _ := db.LoadParts(context.Background(), v.worldID)
	restored, err := restoreCheckpoint(v.worldID, parts, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.solver.Dom, v.solver.Dom) || restored.solver.Steps != v.solver.Steps {
		t.Fatal("retry omitted failed mutation")
	}
}
