package history

import (
	"encoding/json"
	"kriemhild/internal/world"
	"testing"
)

func testShape(id, kind string, points [][2]float64) world.Entity {
	layer := "natural-features"
	if kind == "city" {
		layer = "settlements"
	}
	return world.Entity{ID: id, Name: id, Kind: kind, Layer: layer, Geometry: "polygon", Points: points, Color: "#000000"}
}
func TestTimelineInsertionBranchParallelMergeAndCycle(t *testing.T) {
	d := New("History")
	first := d.World.CurrentAge
	apply := func(mode, from, other string) string {
		t.Helper()
		if err := d.Apply(Command{Kind: "age", AgeID: from, OtherAge: other, Mode: mode, Name: mode, Policy: "primary"}); err != nil {
			t.Fatal(err)
		}
		if err := d.Validate(); err != nil {
			t.Fatal(err)
		}
		return d.World.CurrentAge
	}
	second := apply("after", first, "")
	middle := apply("after", first, "")
	before := apply("before", first, "")
	line := d.Timelines[0].Ages
	if len(line) != 4 || line[0] != before || line[1] != first || line[2] != middle || line[3] != second {
		t.Fatal("linear insertion failed", line)
	}
	branch := apply("branch", middle, "")
	parallel := apply("parallel", first, "")
	for _, e := range d.Edges {
		if e.Destination == parallel {
			t.Fatal("parallel Age has a chronological dependency")
		}
	}
	if err := d.AddEdge(branch, first, "merge"); err != nil {
		t.Fatal("chronological loop rejected", err)
	}
	// A cross-lane connection may point to an earlier displayed independent Age.
	if err := d.AddEdge(branch, parallel, "merge"); err != nil {
		t.Fatal(err)
	}
	merged := apply("merge", second, branch)
	parents := 0
	for _, e := range d.Edges {
		if e.Destination == merged {
			parents++
		}
	}
	if parents != 2 {
		t.Fatal("merge lost a parent")
	}
}
func TestIndependentAgeStateAndSpatialNamingOrder(t *testing.T) {
	d := New("Atlas")
	first := d.World.CurrentAge
	m, _ := d.AddTerrain(first, "Terra", ID(), 32, 32)
	m.Snapshot = "immutable"
	d.Map(first, m.ID).Snapshot = m.Snapshot
	state := world.New(32, 32)
	city := testShape("city", "city", [][2]float64{{8, 8}, {11, 8}, {12, 10}, {11, 12}, {8, 12}, {7, 10}})
	state.Entities[city.ID] = city
	d.Sync(m, state)
	island := testShape("island", "island", [][2]float64{{2, 2}, {25, 2}, {25, 25}, {2, 25}})
	state.Entities[island.ID] = island
	d.Sync(m, state)
	contained := false
	for _, r := range d.Relations {
		if r.Source == "city" && r.Destination == "island" && r.Kind == "contained-by" {
			contained = true
		}
	}
	if !contained {
		t.Fatal("late-named geography did not contain earlier city")
	}
	child, err := d.Child(first, m.ID, "city", "City map", world.Bounds{})
	if err != nil {
		t.Fatal(err)
	}
	shapes := d.Shapes(child)
	a, _ := json.Marshal(shapes["city"].Points)
	b, _ := json.Marshal(city.Points)
	if string(a) != string(b) {
		t.Fatal("child changed the exact polygon")
	}
	inner := world.New(32, 32)
	inner.Entities["building"] = world.Entity{ID: "building", Name: "Keep", Kind: "castle", Layer: "buildings", Geometry: "point", Points: [][2]float64{{9, 9}}, Color: "#000000"}
	d.Sync(child, inner)
	if _, ok := d.Shapes(m)["building"]; ok {
		t.Fatal("city details leaked to parent")
	}
	city.Points = append([][2]float64(nil), city.Points...)
	city.Points[0] = [2]float64{7.5, 8}
	inner.Entities["city"] = city
	d.Sync(child, inner)
	if d.Shapes(m)["city"].Points[0] != city.Points[0] {
		t.Fatal("city footprint did not reach parent")
	}
	if err = d.Apply(Command{Kind: "age", AgeID: first, Mode: "branch", Name: "Future"}); err != nil {
		t.Fatal(err)
	}
	future := d.World.CurrentAge
	copyMap := *d.Map(future, m.ID)
	if copyMap.Snapshot != m.Snapshot || copyMap.ProjectID == m.ProjectID {
		t.Fatal("Age did not share immutable data with independent map identity")
	}
	changed := world.New(32, 32)
	city.Name = "New capital"
	changed.Entities["city"] = city
	d.Sync(copyMap, changed)
	if d.Shapes(m)["city"].Name == city.Name {
		t.Fatal("future changed past")
	}
	if err = d.Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestMergeRequiresPolicyAndPreservesIdentity(t *testing.T) {
	d := New("Merge")
	source := d.World.CurrentAge
	m, _ := d.AddTerrain(source, "Terra", ID(), 32, 32)
	state := world.New(32, 32)
	state.Entities["city"] = testShape("city", "city", [][2]float64{{1, 1}, {4, 1}, {4, 4}, {1, 4}})
	d.Sync(m, state)
	if err := d.Apply(Command{Kind: "age", AgeID: source, Mode: "branch", Name: "Alternative"}); err != nil {
		t.Fatal(err)
	}
	other := d.World.CurrentAge
	shape := state.Entities["city"]
	shape.Name = "Alternative city"
	state.Entities["city"] = shape
	d.Sync(*d.Map(other, m.ID), state)
	if err := d.Apply(Command{Kind: "age", AgeID: source, OtherAge: other, Mode: "merge", Name: "Invalid"}); err == nil {
		t.Fatal("silent conflict resolution")
	}
	if err := d.Apply(Command{Kind: "age", AgeID: source, OtherAge: other, Mode: "merge", Name: "Reconciled", Policy: "secondary"}); err != nil {
		t.Fatal(err)
	}
	if d.Shapes(*d.Map(d.World.CurrentAge, m.ID))["city"].Name != "Alternative city" {
		t.Fatal("explicit conflict choice ignored")
	}
}

func TestMergeDeletionAndIndependentGeometry(t *testing.T) {
	d := New("Branches")
	first := d.World.CurrentAge
	m, _ := d.AddTerrain(first, "Terra", ID(), 32, 32)
	s := world.New(32, 32)
	s.Entities["city"] = testShape("city", "city", [][2]float64{{3, 3}, {6, 3}, {6, 6}, {3, 6}})
	d.Sync(m, s)
	if err := d.Apply(Command{Kind: "age", AgeID: first, Mode: "branch", Name: "Branch"}); err != nil {
		t.Fatal(err)
	}
	branch := d.World.CurrentAge
	for i := range d.Representations {
		if d.Representations[i].AgeID == branch {
			d.Representations[i].Shape.Points[0][0] = 2
		}
	}
	if d.Shapes(m)["city"].Points[0][0] != 3 {
		t.Fatal("inherited geometry aliases the previous Age")
	}
	shape := s.Entities["city"]
	shape.Deleted = true
	s.Entities["city"] = shape
	d.Sync(*d.Map(branch, m.ID), s)
	for _, policy := range []string{"primary", "secondary"} {
		if err := d.Apply(Command{Kind: "age", AgeID: first, OtherAge: branch, Mode: "merge", Name: "Merge", Policy: policy}); err != nil {
			t.Fatal(err)
		}
		_, present := d.Shapes(*d.Map(d.World.CurrentAge, m.ID))["city"]
		if present != (policy == "primary") {
			t.Fatalf("%s merge ignored deletion conflict", policy)
		}
		if err := d.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSpatialRelationsRespectHolesAndChildGeographyOwnership(t *testing.T) {
	d := New("Coasts")
	age := d.World.CurrentAge
	m, _ := d.AddTerrain(age, "Terra", ID(), 32, 32)
	s := world.New(32, 32)
	ocean := testShape("ocean", "ocean", [][2]float64{{0, 0}, {31, 0}, {31, 31}, {0, 31}})
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			if x < 8 || x > 15 || y < 8 || y > 15 {
				ocean.Cells = append(ocean.Cells, y*32+x)
			}
		}
	}
	s.Entities["ocean"] = ocean
	s.Entities["city"] = testShape("city", "city", [][2]float64{{10, 10}, {13, 10}, {13, 13}, {10, 13}})
	d.Sync(m, s)
	for _, r := range d.Relations {
		if r.Source == "city" && r.Destination == "ocean" {
			t.Fatal("island city incorrectly belongs to surrounding ocean")
		}
	}
	child, err := d.Child(age, m.ID, "city", "City", world.Bounds{})
	if err != nil {
		t.Fatal(err)
	}
	natural := world.New(32, 32)
	natural.Entities["lake"] = testShape("lake", "lake", [][2]float64{{10, 10}, {11, 10}, {11, 11}, {10, 11}})
	d.Sync(child, natural)
	if err := d.Apply(Command{Kind: "delete-map", AgeID: age, MapID: child.ID}); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.Shapes(m)["lake"]; !ok {
		t.Fatal("deleting settlement map deleted shared geography")
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}
