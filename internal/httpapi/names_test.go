package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/auth"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"kriemhild/internal/world"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenameChangesUpdateOpenLabelsWithoutTerrainReload(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	d := history.New("Fast names")
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	identity, err := auth.New(ctx, auth.Config{Mode: "local"}, db, "127.0.0.1:8124")
	if err != nil {
		t.Fatal(err)
	}
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir(), Auth: identity})
	v := projectFixture(t)
	v.creatorID = identity.Local.ID
	s.sessions["names"] = v
	autoRequest(t, s, "POST", "/api/worlds/"+d.World.ID+"/accept", fmt.Sprintf(`{"session":"names","ageId":%q,"name":"Terrain"}`, d.World.CurrentAge))
	e := world.Entity{ID: "city", Name: "Old City", LabelName: "Local label", Kind: "city", Layer: "settlements", Geometry: "polygon", Points: [][2]float64{{5, 5}, {6, 5}, {5, 6}}, Color: "#112233"}
	raw, _ := json.Marshal(map[string]any{"kind": "entity", "entity": e, "publish": true, "revision": v.authored().Header.Revision})
	autoRequest(t, s, "POST", "/api/sessions/names/world", string(raw))
	base := v.historyRevision
	raster := v.authored().Header.Revision
	collaboration := v.collabRevision
	busy := &session{historyWorld: d.World.ID}
	busy.mu.Lock()
	defer busy.mu.Unlock()
	s.sessions["busy"] = busy
	rename := func(name string, revision int) map[string]any {
		return autoRequest(t, s, "POST", "/api/worlds/"+d.World.ID+"?view=changes", fmt.Sprintf(`{"kind":"rename-entity","ageId":%q,"entityId":"city","name":%q,"revision":%d}`, d.World.CurrentAge, name, revision))
	}
	result := rename("New City", base)
	change := result["nameChange"].(map[string]any)
	if len(change["entities"].([]any)) != 1 || result["document"] != nil || change["biospheres"] != nil {
		t.Fatal("rename downloaded a World document")
	}
	if v.authored().Entities["city"].Name != "New City" || v.authored().Entities["city"].LabelName != "Local label" || v.authored().Header.Revision != raster || v.collabRevision != collaboration {
		t.Fatal("rename invalidated terrain or lost a local label")
	}
	poll := autoRequest(t, s, "GET", fmt.Sprintf("/api/worlds/%s/names?revision=%d&view=workspace&delta=names", d.World.ID, base), "")
	if poll["nameChange"] == nil || poll["document"] != nil {
		t.Fatal("name polling downloaded the workspace")
	}
	// A session that did not observe the in-process notification catches up from
	// the audit patch and still keeps its raster revision and current geometry.
	v.historyRevision = base
	old := v.authored().Entities["city"]
	old.Name = "Old City"
	old.Points = [][2]float64{{8, 8}}
	v.authored().Entities["city"] = old
	names := autoRequest(t, s, "GET", fmt.Sprintf("/api/sessions/names/world/names?revision=%d", base), "")
	if names["namesOnly"] != true || v.authored().Entities["city"].Name != "New City" || v.authored().Entities["city"].Points[0][0] != 8 || v.authored().Header.Revision != raster {
		t.Fatal("lazy name sync rebuilt the terrain")
	}
	stale := httptest.NewRecorder()
	s.ServeHTTP(stale, httptest.NewRequest("POST", "/api/worlds/"+d.World.ID+"?view=changes", strings.NewReader(fmt.Sprintf(`{"kind":"rename-entity","ageId":%q,"entityId":"city","name":"Stale","revision":%d}`, d.World.CurrentAge, base))))
	if stale.Code != 409 {
		t.Fatal("stale rename accepted", stale.Code)
	}
}
