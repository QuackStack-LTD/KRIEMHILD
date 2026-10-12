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

func TestArchiveGatherAPIAndZIPPreserveCoverage(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	d := history.New("Archive API")
	age := d.World.CurrentAge
	for _, e := range []history.Entity{{ID: "owner", Name: "Owner", Kind: "character", Properties: json.RawMessage(`{"startYear":2}`)}, {ID: "archive", Name: "Private archive", Kind: "archive", Archive: &history.ArchiveConfig{Version: 1, OwnerID: "owner", Purpose: "personal", Intensity: "detailed", Categories: []string{"chronicles"}, KnowledgeScope: "direct", Retention: "preserve"}}} {
		if err = d.Apply(history.Command{Kind: "entity", AgeID: age, Entity: &e}); err != nil {
			t.Fatal(err)
		}
	}
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	req := history.GatherRequest{AgeID: age, ArchiveID: "archive", Scope: "age", SourceAge: age, RecordingYear: 10, Revision: d.World.Revision}
	call := func(s *Server, req history.GatherRequest) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(req)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest("POST", "/api/worlds/"+d.World.ID+"/archives/gather", bytes.NewReader(raw)))
		return w
	}
	result := call(s, req)
	if result.Code != 200 {
		t.Fatal(result.Body.String())
	}
	var payload struct {
		Document history.Document     `json:"document"`
		Result   history.GatherResult `json:"result"`
	}
	if err = json.Unmarshal(result.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Result.Created) != 1 {
		t.Fatal("no generated file")
	}
	zip := httptest.NewRecorder()
	s.ServeHTTP(zip, httptest.NewRequest("GET", "/api/worlds/"+d.World.ID+"/export", nil))
	if zip.Code != 200 {
		t.Fatal(zip.Body.String())
	}
	portable, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer portable.Close()
	target := NewWithOptions(t.TempDir(), Options{Projects: portable, CacheDir: t.TempDir()})
	imported := httptest.NewRecorder()
	target.ServeHTTP(imported, httptest.NewRequest("POST", "/api/worlds/import", bytes.NewReader(zip.Body.Bytes())))
	if imported.Code != 200 && imported.Code != 201 {
		t.Fatal(imported.Body.String())
	}
	restored, err := portable.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	req.Revision = restored.World.Revision
	result = call(target, req)
	if result.Code != 200 {
		t.Fatal(result.Body.String())
	}
	_ = json.Unmarshal(result.Body.Bytes(), &payload)
	if len(payload.Result.Created) != 0 || len(restored.Entity(age, "archive").Archive.Coverage) != 1 {
		t.Fatal("ZIP lost idempotent coverage")
	}
	f := restored.LibraryFiles[0]
	content := httptest.NewRecorder()
	target.ServeHTTP(content, httptest.NewRequest("GET", "/api/worlds/"+d.World.ID+"/library/"+f.ID+"?age="+age, nil))
	if content.Code != 200 || !bytes.Contains(content.Body.Bytes(), []byte("Owner")) {
		t.Fatal("ZIP lost generated file")
	}
	req.ArchiveID = "missing"
	if bad := call(target, req); bad.Code != 409 {
		t.Fatal("invalid archive accepted")
	}
}
