package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/world"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAttachedTerrainAutosavesDrawingDeletionUndoAndView(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	v := projectFixture(t)
	s.sessions["terrain"] = v
	d := history.New("Automatic terrain saving")
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	autoRequest(t, s, "POST", "/api/worlds/"+d.World.ID+"/accept", fmt.Sprintf(`{"session":"terrain","ageId":%q,"name":"Terrain"}`, d.World.CurrentAge))
	// Already persisted terrain: these tests exercise authored transactions,
	// without generating a new planet for every persistence assertion.
	v.planet = &model.Planet{Spec: model.DefaultSpec()}
	v.world.Header.Version = 2
	send := func(kind string, fields map[string]any) map[string]any {
		t.Helper()
		if fields == nil {
			fields = map[string]any{}
		}
		fields["kind"], fields["revision"] = kind, v.world.Header.Revision
		raw, _ := json.Marshal(fields)
		result := autoRequest(t, s, "POST", "/api/sessions/terrain/world", string(raw))
		if result["unsaved"] != false || v.unsaved {
			t.Fatal("successful change was not durable")
		}
		return result
	}
	shape := world.Entity{ID: "marker", Kind: "label", Name: "New feature", Layer: "labels", Geometry: "point", Points: [][2]float64{{5, 5}}, Color: "#123456"}
	result := send("entity", map[string]any{"entity": shape})
	if len(result["entities"].([]any)) != 1 || result["dirty"] != nil {
		t.Fatal("drawing did not return a local feature delta", result)
	}
	check := func(deleted bool) {
		t.Helper()
		doc, err := db.History(ctx, d.World.ID)
		if err != nil {
			t.Fatal(err)
		}
		parts, err := db.SnapshotParts(ctx, doc.Project(v.worldID).Snapshot)
		if err != nil {
			t.Fatal(err)
		}
		var saved world.Entity
		if err = expandJSON(parts["builder/entity/marker"], &saved); err != nil {
			t.Fatal(err)
		}
		if saved.Deleted != deleted || saved.ID != shape.ID {
			t.Fatal("snapshot lost drawing or tombstone", saved)
		}
		// Finishing a drawing persists the draft but never publishes its name.
		if doc.Entity(v.historical.AgeID, shape.ID) != nil {
			t.Fatal("unnamed draft was published")
		}
	}
	check(false)
	shape.Deleted = true
	result = send("entity", map[string]any{"entity": shape})
	if len(result["entities"].([]any)) != 1 {
		t.Fatal("deletion did not return a tombstone")
	}
	check(true)
	send("undo", nil)
	check(false)
	send("redo", nil)
	check(true)
	result = send("operation", map[string]any{"operation": world.Operation{ID: "hill", Kind: "raise", Layer: "elevation", Center: [2]float64{10, 10}, Radius: 1, Amount: 100}})
	if result["dirty"] == nil || len(result["entities"].([]any)) != 0 {
		t.Fatal("brush edit reloaded unrelated drawings")
	}
	view := `{"ui":{"continuousView":{"camera":{"x":0.2,"y":0.3,"w":0.4,"h":0.2},"mode":"map"}},"merge":true,"client":"terrain-test","sequence":2}`
	autoRequest(t, s, "POST", "/api/sessions/terrain/view", view)
	autoRequest(t, s, "POST", "/api/sessions/terrain/view", `{"ui":{},"merge":true,"client":"terrain-test","sequence":1}`)
	doc, _ := db.History(ctx, d.World.ID)
	parts, err := db.SnapshotParts(ctx, doc.Project(v.worldID).Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var ui map[string]any
	if err = expandJSON(parts["view"], &ui); err != nil {
		t.Fatal(err)
	}
	if ui["continuousView"].(map[string]any)["mode"] != "map" || v.unsaved {
		t.Fatal("view was not automatically persisted")
	}
	if parts["builder/operation/hill"] == nil {
		t.Fatal("view save dropped brush operation")
	}
	// Failed persistence must leave the client revision retryable, including
	// brush edits that have no entity publication transaction of their own.
	revision := v.world.Header.Revision
	failed := fmt.Sprintf(`{"kind":"operation","revision":%d,"operation":{"id":"retry-hill","kind":"raise","layer":"elevation","center":[10,10],"radius":1,"amount":100}}`, revision)
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	s.ServeHTTP(response, httptest.NewRequest("POST", "/api/sessions/terrain/world", strings.NewReader(failed)))
	if response.Code != 503 || v.world.Header.Revision != revision || v.unsaved {
		t.Fatal("failed save cannot be retried from the displayed revision", response.Code, response.Body)
	}
	if _, exists := v.world.Operations["retry-hill"]; exists {
		t.Fatal("failed autosave left an unacknowledged terrain edit")
	}
}
