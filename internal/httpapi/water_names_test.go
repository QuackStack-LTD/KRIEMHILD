package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"kriemhild/internal/terrain/model"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestWaterNamesAreStableMetadataAndSurviveReopen(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	v := projectFixture(t)
	s.sessions["water"] = v
	d := history.New("Water names")
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	autoRequest(t, s, "POST", "/api/worlds/"+d.World.ID+"/accept", fmt.Sprintf(`{"session":"water","ageId":%q,"name":"Land"}`, d.World.CurrentAge))
	// A compact immutable physical source is sufficient for metadata integration;
	// the already saved archive provides the real persistence/reopen boundary.
	v.planet = &model.Planet{Spec: model.DefaultSpec(), Grid: model.Grid{Width: 4, Height: 2, Fields: map[string][]float64{"coast": {-1, -2, 1, 1, -1, -2, 1, 1}, "bed": {-10, -20, 10, 10, -10, -20, 10, 10}}}, Lakes: []model.Lake{{Center: model.Point{1, .2}, Radius: .01, Level: 20}}, Rivers: []model.River{{ID: "river-42", Path: []model.Point{{.5, .4}, {.7, .2}}, Bed: []float64{100, 0}, Surface: []float64{101, 1}, Discharge: []float64{10, 20}, Width: []float64{5, 6}, Destination: "ocean"}}}
	physical, _ := json.Marshal(v.planet)
	before, _ := db.History(ctx, d.World.ID)
	snapshot := before.Maps[0].Snapshot
	list := autoRequest(t, s, "GET", "/api/sessions/water/terrain/water-bodies", "")
	bodies := list["bodies"].([]any)
	if len(bodies) != 3 {
		t.Fatal("missing generated water types", bodies)
	}
	for i, raw := range bodies {
		body := raw.(map[string]any)
		source := body["sourceId"].(string)
		current, _ := db.History(ctx, d.World.ID)
		name := fmt.Sprintf("Named %s %d", body["kind"], i)
		autoRequest(t, s, "POST", "/api/sessions/water/terrain/water-bodies/"+source, fmt.Sprintf(`{"name":%q,"revision":%d}`, name, current.World.Revision))
	}
	current, _ := db.History(ctx, d.World.ID)
	if current.Maps[0].Snapshot != snapshot || len(current.Entities) != 3 {
		t.Fatal("naming committed terrain or duplicated entities")
	}
	riverID := waterNames(current, *v.historical)["river-42"].ID
	autoRequest(t, s, "POST", "/api/worlds/"+d.World.ID, fmt.Sprintf(`{"kind":"rename-entity","ageId":%q,"entityId":%q,"name":"Silver River","revision":%d}`, d.World.CurrentAge, riverID, current.World.Revision))
	features := autoRequest(t, s, "GET", "/api/sessions/water/terrain/features", "")
	if features["rivers"].([]any)[0].(map[string]any)["name"] != "Silver River" {
		t.Fatal("canonical river name absent from terrain features")
	}
	after, _ := json.Marshal(v.planet)
	if !reflect.DeepEqual(physical, after) {
		t.Fatal("naming mutated physical terrain")
	}
	current, _ = db.History(ctx, d.World.ID)
	if current.Maps[0].Snapshot != snapshot || current.Entity(d.World.CurrentAge, riverID).SourceID != "river-42" {
		t.Fatal("rename changed saved source identity")
	}
	// A stale rename is rejected rather than overwriting a newer canonical name.
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("POST", "/api/sessions/water/terrain/water-bodies/river-42", strings.NewReader(`{"name":"Stale name","revision":0}`)))
	if w.Code != 409 {
		t.Fatal("stale rename accepted", w.Code, w.Body)
	}
	delete(s.sessions, "water")
	opened := autoRequest(t, s, "POST", "/api/projects/"+v.worldID+"/open", `{}`)
	cold := s.sessions[opened["id"].(string)]
	if cold.authored().Entities[riverID].Name != "Silver River" {
		t.Fatal("cold reopen lost canonical name")
	}
}
