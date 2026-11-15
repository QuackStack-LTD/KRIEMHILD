package history

import (
	"encoding/json"
	"kriemhild/internal/world"
	"testing"
)

func TestCanonicalRenamePreservesIdentityReferencesAndOtherAges(t *testing.T) {
	d := New("Names")
	age := d.World.CurrentAge
	m, _ := d.AddTerrain(age, "Old terrain", "project", 48, 32)
	state := world.New(48, 32)
	city := world.Entity{ID: "city", Name: "Old Harbor", Kind: "city", Layer: "settlements", Geometry: "point", Points: [][2]float64{{10, 10}}, Color: "#112233", RecordStatus: "published"}
	state.Entities[city.ID] = city
	d.Sync(m, state)
	child, err := d.Child(age, m.ID, city.ID, "Old Harbor - settlement map", world.Bounds{})
	if err != nil {
		t.Fatal(err)
	}
	future := "future"
	d.Ages = append(d.Ages, Age{ID: future, Name: "Later Age", TimelineID: d.Timelines[0].ID})
	d.Entities = append(d.Entities, Entity{ID: "city", AgeID: future, Name: "Historical Harbor", Kind: "city", Properties: json.RawMessage(`{}`)})
	d.Relations = append(d.Relations, Relation{ID: "ref", AgeID: age, Source: "someone", Destination: city.ID, Kind: "lives-in"})
	if err = d.Apply(Command{Kind: "rename-entity", AgeID: age, EntityID: city.ID, Name: "New Harbor"}); err != nil {
		t.Fatal(err)
	}
	if d.Entity(age, city.ID).Name != "New Harbor" || d.Entity(future, city.ID).Name != "Historical Harbor" || d.Relations[0].Destination != city.ID {
		t.Fatal("rename changed historical identity or reference")
	}
	for _, view := range []Map{m, child} {
		if d.Shapes(view)[city.ID].Name != "New Harbor" {
			t.Fatal("stale map representation", view.ID)
		}
	}
	if d.Map(age, child.ID).Name != "New Harbor - settlement map" {
		t.Fatal("stale derived map title")
	}
	if err = d.Apply(Command{Kind: "rename-terrain", AgeID: age, TerrainID: m.TerrainID, Name: "New terrain"}); err != nil {
		t.Fatal(err)
	}
	if d.Terrains[0].Name != "New terrain" || d.Map(age, m.ID).Name != "New terrain" {
		t.Fatal("terrain/root map names disagree")
	}
	if err = d.Apply(Command{Kind: "rename-entity", AgeID: age, EntityID: city.ID, Name: "  "}); err == nil {
		t.Fatal("blank canonical name accepted")
	}
}

func TestProfileAndBuilderRenameProjectOneCanonicalName(t *testing.T) {
	d := New("Names")
	age := d.World.CurrentAge
	m, _ := d.AddTerrain(age, "Land", "project", 48, 32)
	s := world.New(48, 32)
	e := world.Entity{ID: "river", Name: "First River", Kind: "river", Layer: "natural-features", Geometry: "line", Points: [][2]float64{{1, 1}, {2, 2}}, SourceID: "river-12", Color: "#112233"}
	s.Entities[e.ID] = e
	d.Sync(m, s)
	profile := *d.Entity(age, e.ID)
	profile.Name = "Profile River"
	if err := d.Apply(Command{Kind: "entity", AgeID: age, Entity: &profile}); err != nil {
		t.Fatal(err)
	}
	if d.Shapes(m)[e.ID].Name != profile.Name {
		t.Fatal("profile rename was not projected")
	}
	e.Name = "Builder River"
	s.Entities[e.ID] = e
	d.Sync(m, s)
	if d.Entity(age, e.ID).Name != e.Name || d.Entity(age, e.ID).SourceID != "river-12" {
		t.Fatal("builder rename lost source identity")
	}
}
