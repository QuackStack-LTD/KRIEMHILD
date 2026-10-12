package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"net/http/httptest"
	"testing"
)

func TestWriterAPIScopesConflictsBackupAndWorldIsolation(t *testing.T) {
	ctx := context.Background()
	db, e := storage.Open(ctx, t.TempDir(), "")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	d := history.New("Writer World")
	if e = db.CreateHistory(ctx, d); e != nil {
		t.Fatal(e)
	}
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	call := func(method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRecorder()
		s.ServeHTTP(r, httptest.NewRequest(method, path, bytes.NewReader(raw)))
		return r
	}
	p := storage.WritingProject{ID: "manuscript", WorldID: d.World.ID, Name: "A novel"}
	r := call("POST", "/api/writer/projects", p)
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	json.Unmarshal(r.Body.Bytes(), &p)
	n := storage.WritingNode{ID: "chapter", ProjectID: p.ID, Type: "chapter", Title: "Chapter One", Schema: 1, Status: "Idea", Content: json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Once upon a time"}]}]}`)}
	r = call("POST", "/api/writer/projects/manuscript/nodes", map[string]any{"node": n})
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var saved storage.WritingNode
	json.Unmarshal(r.Body.Bytes(), &saved)
	r = call("POST", "/api/writer/projects/manuscript/nodes", map[string]any{"node": n})
	if r.Code != 409 {
		t.Fatalf("expected conflict, got %d %s", r.Code, r.Body.String())
	}
	r = call("POST", "/api/writer/projects/other/nodes", map[string]any{"node": saved})
	if r.Code == 200 {
		t.Fatal("cross project accepted")
	}
	r = call("GET", "/api/writer/projects?world=another", nil)
	if r.Code != 200 || r.Body.String() != "[]\n" {
		t.Fatal("world filter", r.Body.String())
	}
	r = call("GET", "/api/writer/projects/manuscript?backup=1", nil)
	var b storage.WriterBundle
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &b) != nil || len(b.Versions) != 1 {
		t.Fatal("backup missing history")
	}
	r = call("POST", "/api/writer/import?world="+d.World.ID, b)
	if r.Code != 201 {
		t.Fatal(r.Body.String())
	}
	var restored storage.WritingProject
	json.Unmarshal(r.Body.Bytes(), &restored)
	if restored.ID == p.ID {
		t.Fatal("backup replaced original")
	}
	world, e := db.History(ctx, d.World.ID)
	if e != nil || len(world.Entities) != 0 || len(world.LibraryFiles) != 0 || world.World.Revision != d.World.Revision {
		t.Fatal("manuscript mutated fictional history")
	}
	// Invalid import must leave no partial project behind.
	before, _ := db.WriterProjects(ctx, d.World.ID)
	b.Nodes[0].ParentID = "missing"
	r = call("POST", "/api/writer/import?world="+d.World.ID, b)
	if r.Code == 201 {
		t.Fatal("broken import accepted")
	}
	after, _ := db.WriterProjects(ctx, d.World.ID)
	if len(before) != len(after) {
		t.Fatal("partial project leaked")
	}
}
