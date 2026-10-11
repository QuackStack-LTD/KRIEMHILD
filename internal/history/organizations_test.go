package history

import (
	"encoding/json"
	"kriemhild/internal/world"
	"reflect"
	"testing"
)

func TestInstitutionsIndependentAgesAndValidation(t *testing.T) {
	d := New("Institutions")
	a := d.World.CurrentAge
	put := func(id, kind, props string) {
		t.Helper()
		e := Entity{ID: id, Name: id, Kind: kind, Properties: json.RawMessage(props)}
		if err := d.Apply(Command{Kind: "entity", AgeID: a, Entity: &e}); err != nil {
			t.Fatal(err)
		}
	}
	put("guild", "organization", `{"governance":"decentralized"}`)
	put("league", "organization", `{}`)
	put("person", "character", `{}`)
	put("unit", "organization-unit", `{"organizationId":"guild"}`)
	put("child", "organization-unit", `{"organizationId":"guild","parentUnitId":"unit"}`)
	put("office", "organization-position", `{"organizationId":"guild","unitId":"child","shared":true}`)
	for _, r := range []Relation{{ID: "member", Source: "person", Destination: "guild", Kind: "member-of", UnitID: "child", Role: "Treasurer", Rank: "Senior", Status: "active", ValidFrom: "12"}, {ID: "second", Source: "person", Destination: "league", Kind: "member-of"}, {ID: "office", Source: "person", Destination: "office", Kind: "appointed-to"}, {ID: "federation", Source: "guild", Destination: "league", Kind: "member-of"}} {
		if err := d.Apply(Command{Kind: "relation", AgeID: a, Relation: &r}); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := json.Marshal(d)
	bad := *d.Entity(a, "unit")
	bad.Properties = json.RawMessage(`{"organizationId":"guild","parentUnitId":"child"}`)
	if err := d.Apply(Command{Kind: "entity", AgeID: a, Entity: &bad}); err == nil {
		t.Fatal("accepted unit cycle")
	}
	r := Relation{Source: "person", Destination: "league", Kind: "member-of", UnitID: "unit"}
	if err := d.Apply(Command{Kind: "relation", AgeID: a, Relation: &r}); err == nil {
		t.Fatal("accepted membership in another institution's unit")
	}
	if err := d.Apply(Command{Kind: "delete-entities", AgeID: a, EntityIDs: []string{"unit"}}); err == nil {
		t.Fatal("deleted parent of an occupied unit")
	}
	after, _ := json.Marshal(d)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed commands changed the document")
	}
	if err := d.Apply(Command{Kind: "age", AgeID: a, Name: "Later", Mode: "after"}); err != nil {
		t.Fatal(err)
	}
	b := d.World.CurrentAge
	e := *d.Entity(b, "unit")
	e.Name = "New division"
	if err := d.Apply(Command{Kind: "entity", AgeID: b, Entity: &e}); err != nil {
		t.Fatal(err)
	}
	r = d.Relations[0]
	r.Status = "former"
	r.ValidTo = "42"
	if err := d.Apply(Command{Kind: "relation", AgeID: b, Relation: &r}); err != nil {
		t.Fatal(err)
	}
	if d.Entity(a, "unit").Name != "unit" || d.Relations[0].Status != "active" {
		t.Fatal("historical organization state changed")
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestInstitutionBatchRollsBackEventOnInvalidRelationship(t *testing.T) {
	d := New("Atomic")
	a := d.World.CurrentAge
	commands := []Command{{Kind: "entity", Entity: &Entity{ID: "event", Kind: "event", Name: "Foundation", Properties: json.RawMessage(`{"startYear":"12"}`)}}, {Kind: "relation", Relation: &Relation{Source: "missing", Destination: "event", Kind: "institution-event"}}}
	if err := d.Apply(Command{Kind: "batch", AgeID: a, Commands: commands}); err == nil {
		t.Fatal("invalid batch accepted")
	}
	if len(d.Entities) != 0 || len(d.Years) != 0 || len(d.Relations) != 0 {
		t.Fatal("batch left partial state")
	}
}

func TestUnnamedDrawablesBecomeEntitiesOnlyWhenNamed(t *testing.T) {
	d := New("Names")
	a := d.World.CurrentAge
	m, err := d.AddTerrain(a, "Terrain", "project", 100, 100)
	if err != nil {
		t.Fatal(err)
	}
	s := world.New(100, 100)
	shape := world.Entity{ID: "shape", Name: "example-unique", Kind: "city", Layer: "settlements", Geometry: "polygon", Color: "#000000", Points: [][2]float64{{1, 1}, {5, 1}, {5, 5}, {1, 5}}}
	s.Entities[shape.ID] = shape
	d.Sync(m, s)
	if len(d.Entities) != 0 || len(d.Representations) != 0 {
		t.Fatal("placeholder became logical entity")
	}
	s.Saved()
	s.RefreshRepresentations(d.Shapes(m))
	if s.Entities[shape.ID].Name != "example-unique" {
		t.Fatal("stored unnamed shape was discarded")
	}
	shape.Name = "Rivergate"
	s.Entities[shape.ID] = shape
	d.Sync(m, s)
	if d.Entity(a, shape.ID) == nil || d.Entity(a, shape.ID).Name != "Rivergate" || len(d.Representations) != 1 {
		t.Fatal("authored name did not create identity")
	}
	shape.Name = "example-renamed"
	s.Entities[shape.ID] = shape
	d.Sync(m, s)
	if !d.Entity(a, shape.ID).Deleted || len(d.Representations) != 0 {
		t.Fatal("placeholder remained active logical entity")
	}
	shape.Name = "Rivergate restored"
	s.Entities[shape.ID] = shape
	d.Sync(m, s)
	if d.Entity(a, shape.ID).Deleted || d.Entity(a, shape.ID).ContinuityID != shape.ID {
		t.Fatal("renaming lost stable identity")
	}
}
func TestVacantSharedAndSuccessiveOffices(t *testing.T) {
	d := New("Offices")
	a := d.World.CurrentAge
	for _, id := range []string{"one", "two"} {
		person(t, d, a, id, "character")
	}
	person(t, d, a, "org", "organization")
	office := Entity{ID: "office", Name: "Treasurer", Kind: "organization-position", Properties: json.RawMessage(`{"organizationId":"org"}`)}
	if err := d.Apply(Command{Kind: "entity", AgeID: a, Entity: &office}); err != nil {
		t.Fatal(err)
	}
	put := func(r Relation) error { return d.Apply(Command{Kind: "relation", AgeID: a, Relation: &r}) }
	first := Relation{ID: "one", Source: "one", Destination: "office", Kind: "appointed-to", ValidFrom: "1", ValidTo: "10"}
	if err := put(first); err != nil {
		t.Fatal(err)
	}
	second := Relation{ID: "two", Source: "two", Destination: "office", Kind: "appointed-to", ValidFrom: "8"}
	if err := put(second); err == nil {
		t.Fatal("overlapping exclusive office holders accepted")
	}
	second.ValidFrom = "11"
	if err := put(second); err != nil {
		t.Fatal("non-overlapping succession rejected", err)
	}
	office.Properties = json.RawMessage(`{"organizationId":"org","shared":true}`)
	if err := d.Apply(Command{Kind: "entity", AgeID: a, Entity: &office}); err != nil {
		t.Fatal(err)
	}
	second.ValidFrom = "8"
	if err := put(second); err != nil {
		t.Fatal("shared appointment rejected", err)
	}
	office.Properties = json.RawMessage(`{"organizationId":"org","shared":false}`)
	if err := d.Apply(Command{Kind: "entity", AgeID: a, Entity: &office}); err == nil {
		t.Fatal("shared office became exclusive with overlapping holders")
	}
}
