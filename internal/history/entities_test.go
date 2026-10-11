package history

import (
	"encoding/json"
	"kriemhild/internal/world"
	"testing"
)

func person(t *testing.T, d *Document, age, id, kind string) {
	t.Helper()
	if err := d.Apply(Command{Kind: "entity", AgeID: age, Entity: &Entity{ID: id, Name: id, Kind: kind, Properties: json.RawMessage(`{"summary":"Original"}`)}}); err != nil {
		t.Fatal(err)
	}
}
func TestIndependentPeopleAndRelationshipInheritance(t *testing.T) {
	d := New("People")
	a := d.World.CurrentAge
	person(t, d, a, "parent", "character")
	person(t, d, a, "child", "character")
	person(t, d, a, "house", "family")
	d.Entity(a, "parent").Names = []EntityName{{Name: "Old name", Type: "historical", Language: "Common"}}
	for _, r := range []Relation{{ID: "parentage", Source: "parent", Destination: "child", Kind: "biological-parent", Description: "A complex history"}, {ID: "family", Source: "child", Destination: "house", Kind: "member-of-family"}, {ID: "rivalry", Source: "parent", Destination: "child", Kind: "rival", Description: "Political rivals"}} {
		if err := d.Apply(Command{Kind: "relation", AgeID: a, Relation: &r}); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.Apply(Command{Kind: "age", AgeID: a, Name: "Later", Mode: "after"}); err != nil {
		t.Fatal(err)
	}
	b := d.World.CurrentAge
	copy := *d.Entity(b, "parent")
	copy.Name = "Changed"
	copy.Names[0].Name = "Local name"
	copy.Properties = json.RawMessage(`{"summary":"New life"}`)
	if err := d.Apply(Command{Kind: "entity", AgeID: b, Entity: &copy}); err != nil {
		t.Fatal(err)
	}
	if d.Entity(a, "parent").Name != "parent" || d.Entity(a, "parent").Names[0].Name != "Old name" {
		t.Fatal("historical state was aliased")
	}
	if d.Entity(a, "parent").ContinuityID != d.Entity(b, "parent").ContinuityID {
		t.Fatal("lost continuity")
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := d.Apply(Command{Kind: "delete-entities", AgeID: b, EntityIDs: []string{"child"}}); err != nil {
		t.Fatal(err)
	}
	if d.Entity(a, "child").Deleted {
		t.Fatal("deletion crossed Age boundary")
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, r := range d.Relations {
		if r.AgeID == b && (r.Source == "child" || r.Destination == "child") {
			t.Fatal("dangling relation")
		}
	}
}
func TestRelationshipTypesAndIndependentPoliticalIdentity(t *testing.T) {
	d := New("Relations")
	a := d.World.CurrentAge
	for _, p := range [][2]string{{"person", "character"}, {"city", "city"}, {"country", "country"}, {"other", "character"}} {
		person(t, d, a, p[0], p[1])
	}
	for _, r := range []Relation{{Source: "city", Destination: "country", Kind: "capital-of"}, {Source: "person", Destination: "country", Kind: "born-in"}, {Source: "person", Destination: "other", Kind: "spouse"}} {
		if err := d.Apply(Command{Kind: "relation", AgeID: a, Relation: &r}); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []Relation{{Source: "country", Destination: "city", Kind: "capital-of"}, {Source: "person", Destination: "city", Kind: "biological-parent"}, {Source: "person", Destination: "missing", Kind: "friend"}, {Source: "person", Destination: "other", Kind: "friend", ContextID: "missing"}} {
		if err := d.Apply(Command{Kind: "relation", AgeID: a, Relation: &r}); err == nil {
			t.Fatal("invalid relation accepted", r)
		}
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestMultipleRegionsPreserveLogicalIdentityAndProfile(t *testing.T) {
	d := New("Territories")
	a := d.World.CurrentAge
	m, _ := d.AddTerrain(a, "Terrain", ID(), 32, 32)
	s := world.New(32, 32)
	region := testShape("realm", "country", [][2]float64{{1, 1}, {8, 1}, {8, 8}, {1, 8}})
	region.Layer = "countries"
	s.Entities[region.ID] = region
	d.Sync(m, s)
	e := d.Entity(a, "realm")
	e.Names = []EntityName{{Name: "An alternate name", Type: "alternate"}}
	e.Properties = json.RawMessage(`{"biography":"A long history"}`)
	island := testShape("island-region", "country", [][2]float64{{20, 20}, {25, 20}, {25, 25}, {20, 25}})
	island.Layer = "countries"
	island.LogicalID = "realm"
	s.Entities[island.ID] = island
	d.Sync(m, s)
	if len(d.Entities) != 1 || len(d.Shapes(m)) != 2 {
		t.Fatal("territories duplicated political identity or lost geometry")
	}
	if len(d.Entity(a, "realm").Names) != 1 {
		t.Fatal("Builder discarded alternate names")
	}
	var props map[string]any
	json.Unmarshal(d.Entity(a, "realm").Properties, &props)
	if props["biography"] != "A long history" {
		t.Fatal("Builder discarded biography")
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestInheritanceReviewExcludesEntitiesWithoutChangingSource(t *testing.T) {
	d := New("Review")
	a := d.World.CurrentAge
	person(t, d, a, "first", "character")
	person(t, d, a, "second", "character")
	r := Relation{Source: "first", Destination: "second", Kind: "friend"}
	d.Apply(Command{Kind: "relation", AgeID: a, Relation: &r})
	if err := d.Apply(Command{Kind: "age", AgeID: a, Name: "Branch", Mode: "branch", ExcludeIDs: []string{"second"}, EntityNames: map[string]string{"first": "Later name", "second": ""}}); err != nil {
		t.Fatal(err)
	}
	if d.Entity(d.World.CurrentAge, "first").Name != "Later name" || d.Entity(a, "first").Name != "first" {
		t.Fatal("inheritance review rename was not independent")
	}
	if !d.Entity(d.World.CurrentAge, "second").Deleted || d.Entity(a, "second").Deleted {
		t.Fatal("inheritance review changed source")
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestDeletingMapPreservesIndependentLogicalIdentity(t *testing.T) {
	d := New("An atlas is not ownership")
	age := d.World.CurrentAge
	m, _ := d.AddTerrain(age, "Terra", ID(), 32, 32)
	state := world.New(32, 32)
	state.Entities["city"] = testShape("city", "city", [][2]float64{{1, 1}, {5, 1}, {5, 5}, {1, 5}})
	d.Sync(m, state)
	person(t, d, age, "traveler", "character")
	r := Relation{Source: "traveler", Destination: "city", Kind: "born-in"}
	if err := d.Apply(Command{Kind: "relation", AgeID: age, Relation: &r}); err != nil {
		t.Fatal(err)
	}
	if err := d.Apply(Command{Kind: "delete-map", AgeID: age, MapID: m.ID}); err != nil {
		t.Fatal(err)
	}
	if e := d.Entity(age, "city"); e == nil || e.Deleted || e.HomeMap != "" || e.TerrainID != "" {
		t.Fatal("map deletion destroyed independent identity")
	}
	if len(d.Relations) != 1 {
		t.Fatal("map deletion removed historical birthplace")
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestContextLabelsDoNotRenameIdentityOrOtherMaps(t *testing.T) {
	d := New("Names")
	age := d.World.CurrentAge
	m, _ := d.AddTerrain(age, "Terra", ID(), 32, 32)
	state := world.New(32, 32)
	city := testShape("city", "city", [][2]float64{{2, 2}, {8, 2}, {8, 8}, {2, 8}})
	city.Name = "Canonical city"
	state.Entities[city.ID] = city
	d.Sync(m, state)
	child, err := d.Child(age, m.ID, city.ID, "Local city", world.Bounds{})
	if err != nil {
		t.Fatal(err)
	}
	city.LabelName = "Local exonym"
	state.Entities[city.ID] = city
	d.Sync(child, state)
	if d.Entity(age, city.ID).Name != "Canonical city" || d.Shapes(m)[city.ID].LabelName != "" || d.Shapes(child)[city.ID].LabelName != "Local exonym" {
		t.Fatal("local label changed canonical identity or parent label")
	}
	if err = d.Validate(); err != nil {
		t.Fatal(err)
	}
}
