package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"kriemhild/internal/world"
	"net/http/httptest"
	"testing"
)

func TestDraftGeometryRequiresNamedApplyAndSurvivesReopen(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	v := projectFixture(t)
	s.sessions["draft"] = v
	d := history.New("Draft places")
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"session": "draft", "ageId": d.World.CurrentAge, "name": "Terrain"})
	autoRequest(t, s, "POST", "/api/worlds/"+d.World.ID+"/accept", string(raw))
	shape := world.Entity{ID: "house-12345678", Name: "house house-12", Kind: "house", Layer: "buildings", Geometry: "point", Color: "#aabbcc", Points: [][2]float64{{5, 5}}}
	send := func(publish bool) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(map[string]any{"kind": "entity", "entity": shape, "publish": publish, "revision": v.world.Header.Revision})
		rr := httptest.NewRecorder()
		s.ServeHTTP(rr, httptest.NewRequest("POST", "/api/sessions/draft/world", bytes.NewReader(raw)))
		return rr
	}
	if rr := send(false); rr.Code != 200 {
		t.Fatal(rr.Code, rr.Body)
	}
	doc, _ := db.History(ctx, d.World.ID)
	if doc.Entity(d.World.CurrentAge, shape.ID) != nil {
		t.Fatal("draft created a record")
	}
	if rr := send(true); rr.Code != 400 {
		t.Fatal("placeholder published", rr.Code, rr.Body)
	}
	shape.Name = "The White Oak"
	if rr := send(false); rr.Code != 200 {
		t.Fatal(rr.Body)
	}
	doc, _ = db.History(ctx, d.World.ID)
	if doc.Entity(d.World.CurrentAge, shape.ID) != nil {
		t.Fatal("rename alone published a record")
	}
	if err = s.saveWorld(v, false); err != nil {
		t.Fatal(err)
	}
	parts, err := db.LoadParts(ctx, v.worldID)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) == 0 {
		doc, _ = db.History(ctx, d.World.ID)
		parts, err = db.SnapshotParts(ctx, doc.Maps[0].Snapshot)
		if err != nil {
			t.Fatal(err)
		}
	}
	cold, err := restoreCheckpoint(v.worldID, parts, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.applyHistoricalMap(ctx, cold); err != nil {
		t.Fatal(err)
	}
	if cold.world.Entities[shape.ID].Deleted || cold.world.Entities[shape.ID].Name != shape.Name {
		t.Fatal("draft geography lost on cold reopen")
	}
	if rr := send(true); rr.Code != 200 {
		t.Fatal(rr.Code, rr.Body)
	}
	doc, _ = db.History(ctx, d.World.ID)
	if e := doc.Entity(d.World.CurrentAge, shape.ID); e == nil || e.Name != shape.Name || e.Deleted {
		t.Fatal("Apply did not publish named record")
	}
	raw, _ = json.Marshal(map[string]any{"kind": "undo", "revision": v.world.Header.Revision})
	autoRequest(t, s, "POST", "/api/sessions/draft/world", string(raw))
	doc, _ = db.History(ctx, d.World.ID)
	if e := doc.Entity(d.World.CurrentAge, shape.ID); e == nil || !e.Deleted {
		t.Fatal("undo publication retained active record")
	}
}
