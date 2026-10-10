package httpapi

import (
	"bytes"
	"context"
	"kriemhild/internal/storage"
	"net/http/httptest"
	"testing"
)

func TestWorldOnlyPersistsOnSaveAndDeletionRetiresWriters(t *testing.T) {
	db, err := storage.Open(context.Background(), t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	created := autoRequest(t, s, "POST", "/api/sessions", `{"options":{"width":24,"height":24,"radius2":2,"seed":12}}`)
	id := created["id"].(string)
	v := s.sessions[id]
	autoRequest(t, s, "POST", "/api/sessions/"+id+"/step", `{"count":3}`)
	autoRequest(t, s, "POST", "/api/sessions/"+id+"/view", `{"ui":{"paused":true}}`)
	autoRequest(t, s, "POST", "/api/sessions/"+id+"/world", `{"kind":"metadata","revision":0,"name":"Explicit save","description":""}`)
	projects, _ := db.List(context.Background())
	if len(projects) != 0 {
		t.Fatal("draft was persisted automatically")
	}
	autoRequest(t, s, "POST", "/api/sessions/"+id+"/save", `{}`)
	before, _ := db.LoadParts(context.Background(), v.worldID)
	autoRequest(t, s, "POST", "/api/sessions/"+id+"/step", `{"count":3}`)
	after, _ := db.LoadParts(context.Background(), v.worldID)
	if !bytes.Equal(before["checkpoint"], after["checkpoint"]) || !v.unsaved {
		t.Fatal("edit overwrote saved checkpoint")
	}
	cold, err := restoreCheckpoint(v.worldID, after, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cold.solver.Steps == v.solver.Steps {
		t.Fatal("cold reopen used unsaved state")
	}
	autoRequest(t, s, "DELETE", "/api/projects/"+v.worldID, "")
	projects, _ = db.List(context.Background())
	parts, _ := db.LoadParts(context.Background(), v.worldID)
	if len(projects) != 0 || len(parts) != 0 || !v.retired {
		t.Fatal("delete left world data or a live writer")
	}
	if s.saveWorld(v, true) == nil {
		t.Fatal("stale writer resurrected deleted world")
	}
	res := httptest.NewRecorder()
	s.ServeHTTP(res, httptest.NewRequest("POST", "/api/sessions/"+id+"/save", nil))
	if res.Code != 404 {
		t.Fatal("deleted session can still save", res.Code)
	}
}

func TestExplorationExportAndImportAreDraftOnly(t *testing.T) {
	db, err := storage.Open(context.Background(), t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	v := projectFixture(t)
	s.sessions["map"] = v
	autoRequest(t, s, "GET", "/api/sessions/map/detail/3/2/1", "")
	res := httptest.NewRecorder()
	s.ServeHTTP(res, httptest.NewRequest("POST", "/api/sessions/map/project", bytes.NewBufferString(`{"ui":{}}`)))
	if res.Code != 200 {
		t.Fatal(res.Code, res.Body.String())
	}
	imported := autoRequest(t, s, "POST", "/api/projects/import", res.Body.String())
	projects, _ := db.List(context.Background())
	if len(projects) != 0 {
		t.Fatal("exploration, export or import created a database world")
	}
	next := s.sessions[imported["id"].(string)]
	if !next.unsaved || !v.retired {
		t.Fatal("import must be a staged replacement")
	}
	autoRequest(t, s, "POST", "/api/sessions/"+imported["id"].(string)+"/save", `{}`)
	parts, _ := db.LoadParts(context.Background(), next.worldID)
	if parts["tile/3/2/1"] == nil {
		t.Fatal("explicit save lost explored tile")
	}
	// Re-importing an earlier archive must not change the committed project until Save.
	if err = db.SaveParts(context.Background(), storage.Project{ID: next.worldID}, map[string][]byte{"obsolete": []byte("old")}); err != nil {
		t.Fatal(err)
	}
	again := autoRequest(t, s, "POST", "/api/projects/import", res.Body.String())
	parts, _ = db.LoadParts(context.Background(), next.worldID)
	if parts["obsolete"] == nil {
		t.Fatal("import changed committed data")
	}
	autoRequest(t, s, "POST", "/api/sessions/"+again["id"].(string)+"/save", `{}`)
	parts, _ = db.LoadParts(context.Background(), next.worldID)
	if parts["obsolete"] != nil {
		t.Fatal("saving imported replacement retained stale parts")
	}
}
