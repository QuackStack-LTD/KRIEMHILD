package history

import (
	"encoding/json"
	"testing"
)

func TestFamilyParentageIsDirectionalAtomicAndAgeScoped(t *testing.T) {
	d := New("Family")
	age := d.World.CurrentAge
	for _, id := range []string{"a", "b", "c", "d"} {
		person(t, d, age, id, "character")
	}
	apply := func(r Relation) error { return d.Apply(Command{Kind: "relation", AgeID: age, Relation: &r}) }
	if err := apply(Relation{ID: "p", Source: "a", Destination: "b", Kind: "biological-parent", SourcePort: "se", DestinationPort: "nw"}); err != nil {
		t.Fatal(err)
	}
	for _, r := range []Relation{{Source: "a", Destination: "b", Kind: "child-of"}, {Source: "b", Destination: "a", Kind: "child-of"}, {Source: "a", Destination: "b", Kind: "sibling"}, {Source: "a", Destination: "b", Kind: "spouse"}} {
		if err := apply(r); err == nil {
			t.Fatalf("accepted contradictory relation %+v", r)
		}
	}
	if err := apply(Relation{ID: "q", Source: "b", Destination: "c", Kind: "parent-of"}); err != nil {
		t.Fatal(err)
	}
	if err := apply(Relation{Source: "c", Destination: "a", Kind: "parent-of"}); err == nil {
		t.Fatal("accepted ancestry cycle")
	}
	before, _ := json.Marshal(d.Relations)
	err := d.Apply(Command{Kind: "relations", AgeID: age, RemoveRelationIDs: []string{"p"}, Relations: []Relation{{Source: "b", Destination: "d", Kind: "child-of"}, {Source: "d", Destination: "b", Kind: "child-of"}}})
	if err == nil {
		t.Fatal("invalid two-parent batch accepted")
	}
	after, _ := json.Marshal(d.Relations)
	if string(before) != string(after) {
		t.Fatal("failed batch partially modified records")
	}
	if err = d.Apply(Command{Kind: "relations", AgeID: age, RemoveRelationIDs: []string{"p"}, Relations: []Relation{{ID: "first", Source: "b", Destination: "a", Kind: "child-of"}, {ID: "second", Source: "b", Destination: "d", Kind: "child-of"}}}); err != nil {
		t.Fatal(err)
	}
	if err = apply(Relation{Source: "b", Destination: "c", Kind: "child-of"}); err == nil {
		t.Fatal("accepted third parent")
	}
	if err = d.Apply(Command{Kind: "age", AgeID: age, Name: "Next", Mode: "after"}); err != nil {
		t.Fatal(err)
	}
	next := d.World.CurrentAge
	if err = d.Apply(Command{Kind: "delete-relation", AgeID: next, EdgeID: "first"}); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range d.Relations {
		if r.AgeID == age && r.ID == "first" {
			found = true
		}
	}
	if !found {
		t.Fatal("historical parentage changed")
	}
}

func TestFamilyViewsMigrateWithoutDuplicatingCharacters(t *testing.T) {
	d := New("Old family")
	a := d.World.CurrentAge
	person(t, d, a, "person", "character")
	person(t, d, a, "family", "dynasty")
	d.Relations = append(d.Relations, Relation{ID: "membership", AgeID: a, Source: "person", Destination: "family", Kind: "member-of-family"})
	d.World.Networks = append(d.World.Networks, NetworkView{ID: "old-view", AgeID: a, Name: "Old view", FamilyID: "family"})
	d.MigrateFamilyViews()
	d.MigrateFamilyViews()
	if len(d.Entities) != 1 || d.Entities[0].ID != "person" || len(d.Relations) != 0 || len(d.World.Networks) != 2 {
		t.Fatal("family not converted into a graph view")
	}
	for _, v := range d.World.Networks {
		if !v.Genealogy || len(v.MemberIDs) != 1 || v.MemberIDs[0] != "person" || v.FamilyID != "" {
			t.Fatal("membership lost")
		}
		if v.ID == "family" && len(v.LegacyData) == 0 {
			t.Fatal("legacy metadata lost")
		}
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := d.Apply(Command{Kind: "age", AgeID: a, Name: "Next", Mode: "after"}); err != nil {
		t.Fatal(err)
	}
	for i := range d.World.Networks {
		if d.World.Networks[i].AgeID == d.World.CurrentAge {
			d.World.Networks[i].MemberIDs[0] = "changed"
		}
	}
	for _, v := range d.World.Networks {
		if v.AgeID == a && v.MemberIDs[0] != "person" {
			t.Fatal("inherited view aliases source Age")
		}
	}
}

func TestReciprocalConnectionsPreserveChosenPorts(t *testing.T) {
	d := New("Ports")
	a := d.World.CurrentAge
	person(t, d, a, "a", "character")
	person(t, d, a, "z", "character")
	r := Relation{Source: "z", Destination: "a", Kind: "spouse", SourcePort: "e", DestinationPort: "nw"}
	if err := d.Apply(Command{Kind: "relation", AgeID: a, Relation: &r}); err != nil {
		t.Fatal(err)
	}
	got := d.Relations[0]
	if got.Source != "a" || got.SourcePort != "nw" || got.DestinationPort != "e" {
		t.Fatal("normalization changed connector locations")
	}
	r.Kind = "friend"
	r.SourcePort = "centre"
	if err := d.Apply(Command{Kind: "relation", AgeID: a, Relation: &r}); err == nil {
		t.Fatal("invalid connection port accepted")
	}
}
