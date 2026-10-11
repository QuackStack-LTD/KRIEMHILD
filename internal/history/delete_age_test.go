package history

import (
	"kriemhild/internal/world"
	"testing"
)

func TestDeleteAgePreservesIndependentHistory(t *testing.T) {
	d := New("Chronicles")
	first := d.World.CurrentAge
	m, err := d.AddTerrain(first, "Planet", ID(), 32, 32)
	if err != nil {
		t.Fatal(err)
	}
	d.Map(first, m.ID).Snapshot = "shared-immutable-snapshot"
	state := world.New(32, 32)
	city := testShape("city", "city", [][2]float64{{2, 2}, {8, 2}, {8, 8}, {2, 8}})
	state.Entities[city.ID] = city
	d.Sync(m, state)
	create := func(from, mode string) string {
		t.Helper()
		if err := d.Apply(Command{Kind: "age", AgeID: from, Name: mode, Mode: mode}); err != nil {
			t.Fatal(err)
		}
		return d.World.CurrentAge
	}
	middle := create(first, "after")
	last := create(middle, "after")
	branch := create(middle, "branch")
	if err := d.AddEdge(branch, first, "merge"); err != nil {
		t.Fatal(err)
	}
	d.World.CurrentAge = middle
	if err := d.Apply(Command{Kind: "delete-age", AgeID: middle}); err != nil {
		t.Fatal(err)
	}
	if d.Age(middle) != nil || d.World.CurrentAge != first {
		t.Fatal("deleted Age remains selected")
	}
	joined := false
	for _, edge := range d.Edges {
		if edge.Source == middle || edge.Destination == middle {
			t.Fatal("dangling edge")
		}
		joined = joined || edge.Source == first && edge.Destination == last && edge.Kind == "linear"
	}
	if !joined {
		t.Fatal("linear neighbours not reconnected")
	}
	for _, age := range []string{first, last, branch} {
		if d.Map(age, m.ID).Snapshot != "shared-immutable-snapshot" {
			t.Fatal("inherited map changed")
		}
	}
	for _, entity := range d.Entities {
		if entity.AgeID == middle {
			t.Fatal("deleted Age retained entities")
		}
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := d.Apply(Command{Kind: "delete-age", AgeID: branch}); err != nil {
		t.Fatal(err)
	}
	if len(d.Timelines) != 1 {
		t.Fatal("empty timeline retained")
	}
	if err := d.Apply(Command{Kind: "delete-age", AgeID: first}); err != nil {
		t.Fatal(err)
	}
	if d.World.CurrentAge != last {
		t.Fatal("current Age not moved to successor")
	}
	if err := d.Apply(Command{Kind: "delete-age", AgeID: last}); err == nil {
		t.Fatal("last Age deleted")
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}
