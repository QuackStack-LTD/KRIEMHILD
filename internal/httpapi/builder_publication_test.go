package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/world"
	"net/http/httptest"
	"testing"
)

func TestSettlementBatchMoveUndoRedoPublishesEveryRecord(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	v := projectFixture(t)
	s.sessions["builder"] = v
	d := history.New("Builder records")
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"session": "builder", "ageId": d.World.CurrentAge, "name": "Geography"})
	autoRequest(t, s, "POST", "/api/worlds/"+d.World.ID+"/accept", string(body))
	// The publication tests need a v2 authored graph, not a generated terrain.
	v.planet = &model.Planet{}
	v.world.Header.Version = 2
	shapes := []world.Entity{
		{ID: "capital", Name: "Rivergate", Kind: "capital", Layer: "settlements", Geometry: "polygon", Color: "#aabbcc", Points: [][2]float64{{3, 3}, {6, 3}, {6, 6}, {3, 6}}},
		{ID: "road", Name: "Mill road", Kind: "road", Layer: "roads", ParentID: "capital", Geometry: "line", Color: "#aabbcc", Points: [][2]float64{{3.5, 4}, {5, 4}}},
		{ID: "inn", Name: "The Oak", Kind: "house", Layer: "buildings", ParentID: "capital", Geometry: "point", Color: "#aabbcc", Points: [][2]float64{{4, 4}}},
	}
	v.settlement = &settlementDraft{ID: "draft", Revision: v.world.Header.Revision, Entities: shapes}
	body, _ = json.Marshal(map[string]any{"id": "draft", "revision": v.world.Header.Revision})
	autoRequest(t, s, "POST", "/api/sessions/builder/settlements", string(body))
	send := func(kind string) {
		t.Helper()
		b, _ := json.Marshal(map[string]any{"kind": kind, "entity": map[string]any{"id": "capital"}, "delta": [2]float64{1, 2}, "revision": v.world.Header.Revision})
		autoRequest(t, s, "POST", "/api/sessions/builder/world", string(b))
	}
	check := func(deleted bool, dx, dy float64) {
		t.Helper()
		doc, e := db.History(ctx, d.World.ID)
		if e != nil {
			t.Fatal(e)
		}
		for _, shape := range shapes {
			entity := doc.Entity(d.World.CurrentAge, shape.ID)
			if entity == nil || entity.Deleted != deleted {
				t.Fatalf("missing or wrong record for %s", shape.ID)
			}
			if _, ok := history.Registry.Types[entity.Kind]; !ok {
				t.Fatalf("record has no editing interface: %s", entity.Kind)
			}
			if !deleted {
				found := false
				for _, r := range doc.Representations {
					if r.EntityID == shape.ID {
						found = true
						for i, p := range r.Shape.Points {
							if p != ([2]float64{shape.Points[i][0] + dx, shape.Points[i][1] + dy}) {
								t.Fatal("record/map positions differ", shape.ID)
							}
						}
					}
				}
				if !found {
					t.Fatal("missing map representation")
				}
			}
		}
		if e = doc.Validate(); e != nil {
			t.Fatal(e)
		}
	}
	check(false, 0, 0)
	// Failed publication must retain pending writes from the accepted batch.
	broken := shapes[0]
	broken.ID = "bad-region"
	broken.LogicalID = "missing-identity"
	bad, _ := json.Marshal(map[string]any{"kind": "entity", "entity": broken, "revision": v.world.Header.Revision})
	response := httptest.NewRecorder()
	s.ServeHTTP(response, httptest.NewRequest("POST", "/api/sessions/builder/world", bytes.NewReader(bad)))
	if response.Code != 503 {
		t.Fatalf("expected rejected publication, got %d: %s", response.Code, response.Body.String())
	}
	if _, exists := v.world.Entities[broken.ID]; exists {
		t.Fatal("failed publication left an orphan")
	}
	if v.world.Parts(false)["builder/entity/capital"] == nil {
		t.Fatal("failed publication discarded pending writes")
	}
	send("move-family")
	check(false, 1, 2)
	send("undo")
	check(false, 0, 0)
	send("undo")
	check(true, 0, 0)
	send("redo")
	check(false, 0, 0)
	send("redo")
	check(false, 1, 2)
	// A second accepted batch must retain the first batch's pending part writes.
	extra := world.Entity{ID: "second-place", Name: "Second place", Kind: "house", Layer: "buildings", Geometry: "point", Color: "#aabbcc", Points: [][2]float64{{8, 8}}}
	v.settlement = &settlementDraft{ID: "second", Revision: v.world.Header.Revision, Entities: []world.Entity{extra}}
	body, _ = json.Marshal(map[string]any{"id": "second", "revision": v.world.Header.Revision})
	autoRequest(t, s, "POST", "/api/sessions/builder/settlements", string(body))
	if err = s.saveWorld(v, false); err != nil {
		t.Fatal(err)
	}
	doc, err := db.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	parts, err := db.SnapshotParts(ctx, doc.Maps[0].Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"capital", "road", "inn", "second-place"} {
		if parts["builder/entity/"+id] == nil {
			t.Fatal("batch discarded a pending entity part", id)
		}
	}
	cold, err := restoreCheckpoint(v.worldID, parts, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cold.world.RecoveredParts() != 0 {
		t.Fatal("new saves should not require recovery")
	}
	if err = s.applyHistoricalMap(ctx, cold); err != nil {
		t.Fatal(err)
	}
	if _, err = cold.world.Undo(false); err != nil {
		t.Fatal(err)
	}
	if !cold.world.Entities[extra.ID].Deleted {
		t.Fatal("cold undo did not remove complete second batch")
	}
	if _, err = cold.world.Undo(true); err != nil {
		t.Fatal(err)
	}
}
