package httpapi

import (
	"context"
	"encoding/json"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"kriemhild/internal/world"
	"testing"
)

func TestLegacyRootAttachmentPreservesUndoAndDeletedTombstonesAcrossRestart(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	v := projectFixture(t)
	s.sessions["root"] = v
	shape := world.Entity{ID: "country", Name: "First country", Kind: "country", Layer: "countries", Geometry: "polygon", Color: "#aabbcc", Points: [][2]float64{{1, 1}, {5, 1}, {5, 5}}}
	if _, err = v.authored().PutEntity(shape); err != nil {
		t.Fatal(err)
	}
	doc := history.New("Root")
	if err = db.CreateHistory(ctx, doc); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"session": "root", "ageId": doc.World.CurrentAge, "name": "Root terrain"})
	autoRequest(t, s, "POST", "/api/worlds/"+doc.World.ID+"/accept", string(body))
	project := v.worldID
	fresh := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	opened := autoRequest(t, fresh, "POST", "/api/projects/"+project+"/open", `{}`)
	id := opened["id"].(string)
	cold := fresh.sessions[id]
	if len(cold.world.Header.History) != 1 {
		t.Fatal("attaching a legacy root discarded its undo history")
	}
	payload, _ := json.Marshal(map[string]any{"kind": "undo", "revision": cold.world.Header.Revision})
	autoRequest(t, fresh, "POST", "/api/sessions/"+id+"/world", string(payload))
	if err = fresh.saveWorld(cold, false); err != nil {
		t.Fatal(err)
	}
	restarted := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	again := autoRequest(t, restarted, "POST", "/api/projects/"+project+"/open", `{}`)
	restored := restarted.sessions[again["id"].(string)]
	if !restored.world.Entities[shape.ID].Deleted || len(restored.world.Header.History) != 1 {
		t.Fatal("cold history lost its tombstone")
	}
	if _, err = restored.world.Undo(true); err != nil {
		t.Fatal(err)
	}
	if restored.world.Entities[shape.ID].Deleted {
		t.Fatal("cold redo failed")
	}
}
