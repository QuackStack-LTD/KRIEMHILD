package httpapi

import (
	"encoding/json"
	"kriemhild/internal/history"
	"reflect"
	"testing"
)

func TestNamingGeneratedGeographyPreservesIdentityAndRelationships(t *testing.T) {
	v := projectFixture(t)
	s := NewWithOptions(t.TempDir(), Options{CacheDir: t.TempDir()})
	s.sessions["terrain"] = v
	e := v.solver.Environment
	if e.Hydrology == nil || len(e.Hydrology.Reaches) == 0 {
		t.Fatal("fixture needs drainage")
	}
	r := e.Hydrology.Reaches[0]
	x, y := r.From%v.solver.W, r.From/v.solver.W
	name := func(label string) map[string]any {
		payload, _ := json.Marshal(map[string]any{"x": x, "y": y, "kind": "river", "name": label, "revision": v.authored().Header.Revision})
		return autoRequest(t, s, "POST", "/api/sessions/terrain/name-geography", string(payload))
	}
	first := name("First River")["entity"].(map[string]any)
	second := name("Renamed River")["entity"].(map[string]any)
	if first["id"] != second["id"] || first["sourceId"] != r.RiverID || !reflect.DeepEqual(first["points"], second["points"]) {
		t.Fatal("renaming relocated or duplicated the river")
	}
	if len(v.authored().Entities) != 1 {
		t.Fatal("renaming created another entity")
	}
	land, err := geographyShape(v.solver, "continent", float64(x), float64(y))
	if err != nil {
		t.Fatal(err)
	}
	if len(land.Cells) == 0 || len(land.Points) < 3 {
		t.Fatal("named landmass did not retain spatial structure")
	}
	land.Layer, land.Name, land.Color = "natural-features", "Land", "#000000"
	d := history.New("Named world")
	m, _ := d.AddTerrain(d.World.CurrentAge, "Terra", history.ID(), v.solver.W, v.solver.H)
	d.Sync(m, v.authored())
	if _, err = v.authored().PutEntity(land); err != nil {
		t.Fatal(err)
	}
	d.Sync(m, v.authored())
	if err = d.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(d.Entities) != 2 {
		t.Fatal("logical entities not created for named geography")
	}
}
