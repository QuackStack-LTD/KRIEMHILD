package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"kriemhild/internal/storage"
	"net/http/httptest"
	"os"
	"testing"
)

func TestDatabaseProjectReopensWithoutSessionOrTileCache(t *testing.T) {
	dir := t.TempDir()
	db, err := storage.Open(context.Background(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	v := projectFixture(t)
	tile, err := v.storedDetail(3, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	first := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	first.sessions["original"] = v
	req := httptest.NewRequest("POST", "/api/sessions/original/project", bytes.NewBufferString(`{"ui":{"seedUsed":42}}`))
	res := httptest.NewRecorder()
	first.ServeHTTP(res, req)
	if res.Code != 200 {
		t.Fatal(res.Code, res.Body.String())
	}
	worldID := v.worldID
	first.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("DELETE", "/api/sessions/original", nil))
	if _, err = os.Stat(v.dir); !os.IsNotExist(err) {
		t.Fatal("original cache survived")
	}
	db.Close()
	db, err = storage.Open(context.Background(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	fresh := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	res = httptest.NewRecorder()
	fresh.ServeHTTP(res, httptest.NewRequest("POST", "/api/projects/"+worldID+"/open", bytes.NewBufferString(`{}`)))
	if res.Code != 201 {
		t.Fatal(res.Code, res.Body.String())
	}
	var state struct {
		ID    string `json:"id"`
		Count int    `json:"storedDetailCount"`
	}
	json.Unmarshal(res.Body.Bytes(), &state)
	if state.Count < 4 {
		t.Fatal("saved detail missing")
	}
	got, err := fresh.sessions[state.ID].storedDetail(3, 2, 1)
	if err != nil || !bytes.Equal(got, tile) {
		t.Fatal("database restore lost explored geography", err)
	}
}

func TestDatabaseOutageFailsReadinessAndSave(t *testing.T) {
	db, err := storage.Open(context.Background(), t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	s.sessions["world"] = projectFixture(t)
	check := func(path string, want int) {
		t.Helper()
		res := httptest.NewRecorder()
		s.ServeHTTP(res, httptest.NewRequest("GET", path, nil))
		if res.Code != want {
			t.Fatalf("%s: got %d, want %d", path, res.Code, want)
		}
	}
	check("/api/ready", 200)
	db.Close()
	check("/api/ready", 503)
	check("/api/health", 200)
	check("/api/projects", 503)
	res := httptest.NewRecorder()
	s.ServeHTTP(res, httptest.NewRequest("POST", "/api/sessions/world/project", bytes.NewBufferString(`{"ui":{}}`)))
	if res.Code != 503 {
		t.Fatalf("save reported success despite unavailable storage: %d", res.Code)
	}
}
