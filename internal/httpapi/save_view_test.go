package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/storage"
	"testing"
)

func TestSaveCommitsCurrentViewAndRejectsDelayedStaging(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	v := projectFixture(t)
	s.sessions["view"] = v
	autoRequest(t, s, "POST", "/api/sessions/view/view", `{"ui":{"continuousView":{"camera":{"x":0,"y":0,"w":2,"h":1}},"otherEditor":"keep"},"client":"builder","sequence":1}`)
	view := `{"camera":{"x":1.05,"y":0.51,"w":0.02,"h":0.01},"style":"terrain","mode":"map"}`
	response := autoRequest(t, s, "POST", "/api/sessions/view/save", fmt.Sprintf(`{"revision":%d,"view":%s,"client":"builder","sequence":3}`, v.authored().Header.Revision, view))
	if response["saved"] != true || v.unsaved {
		t.Fatal("save not acknowledged")
	}
	autoRequest(t, s, "POST", "/api/sessions/view/view", `{"ui":{"continuousView":{"camera":{"x":0,"w":2}}},"merge":true,"client":"builder","sequence":2}`)
	if v.unsaved {
		t.Fatal("delayed camera dirtied committed save")
	}
	parts, err := db.LoadParts(ctx, v.worldID)
	if err != nil {
		t.Fatal(err)
	}
	cold, err := restoreCheckpoint(v.worldID, parts, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !sameJSON(cold.authored().Header.View, []byte(view)) {
		t.Fatal("header camera did not survive cold open")
	}
	var ui map[string]json.RawMessage
	if err = json.Unmarshal(cold.projectUI, &ui); err != nil {
		t.Fatal(err)
	}
	if !sameJSON(ui["continuousView"], []byte(view)) || string(ui["otherEditor"]) != `"keep"` {
		t.Fatal("saved UI camera or unrelated editor state was lost")
	}
}
