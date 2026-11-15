package httpapi

import (
	"encoding/json"
	"kriemhild/internal/history"
	"kriemhild/internal/world"
	"reflect"
	"testing"
)

func TestAgeCreationAdvancesOpenMapsWithoutReadingOrRebuildingThem(t *testing.T) {
	d := history.New("Age inheritance")
	first := d.World.CurrentAge
	m, err := d.AddTerrain(first, "Land", history.ID(), 32, 32)
	if err != nil {
		t.Fatal(err)
	}
	state := world.New(32, 32)
	state.Header.Name = "Pending local title"
	state.Header.Revision = 12
	state.Entities["draft"] = world.Entity{ID: "draft", Name: "Unsaved tree"}
	v := &session{historyWorld: d.World.ID, historyRevision: 1, historical: &m, worldID: m.ProjectID, world: state, collabRevision: 7}
	before, _ := json.Marshal(state)
	if err = d.Apply(history.Command{Kind: "age", AgeID: first, Name: "Later", Mode: "after"}); err != nil {
		t.Fatal(err)
	}
	d.World.Revision = 2
	// No store or solver: rebuilding a projection or reloading the World would
	// fail. Age creation must use the committed metadata already in hand.
	s := &Server{sessions: map[string]*session{"open-map": v}}
	s.refreshHistorySessions(d, false)
	after, _ := json.Marshal(state)
	if !reflect.DeepEqual(before, after) || v.collabRevision != 7 {
		t.Fatal("Age creation invalidated an unchanged open map or discarded edits")
	}
	if v.historyRevision != 2 || v.historical.AgeID != first {
		t.Fatal("map did not retain its original Age with the new history token")
	}
}
