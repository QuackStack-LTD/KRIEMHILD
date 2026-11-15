package storage

import (
	"context"
	"encoding/json"
	"kriemhild/internal/biosphere"
	"kriemhild/internal/history"
	"kriemhild/internal/world"
	"reflect"
	"testing"
)

func historyViewFixture(t testing.TB) (*Store, *history.Document, string) {
	t.Helper()
	ctx := context.Background()
	s, err := Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	d := storedBiosphereWorld(2000)
	age := d.World.CurrentAge
	m := history.Map{ID: "map", AgeID: age, TerrainID: "terrain", ProjectID: history.ID(), Name: "Land", Bounds: world.Bounds{Width: 15, Height: 15}}
	d.Maps = []history.Map{m}
	d.Entities = append(d.Entities, history.Entity{ID: "city", AgeID: age, TerrainID: "terrain", HomeMap: m.ID, Name: "Old City", Kind: "city", Properties: json.RawMessage(`{}`)})
	d.Representations = []history.Representation{{ID: "city", AgeID: age, MapID: m.ID, EntityID: "city", Shape: world.Entity{ID: "city", Name: "Old City", Kind: "city", Layer: "settlements", Geometry: "point", Points: [][2]float64{{5, 5}}, Color: "#000000"}}}
	h := &d.Biospheres[0].Habitats[0]
	h.Profiles = []biosphere.Conditions{{Medium: "terrestrial", Substrates: []string{"loam"}}, {Medium: "terrestrial"}, {Medium: "terrestrial"}}
	for i := 0; i < 2; i++ {
		if err = d.Apply(history.Command{Kind: "age", AgeID: age, Name: "Later", Mode: "after"}); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	return s, d, m.ProjectID
}

func TestHistoryReadViewsScopeDataWithoutChangingSavedState(t *testing.T) {
	s, d, project := historyViewFixture(t)
	ctx := context.Background()
	age := d.Maps[0].AgeID
	meta, err := s.HistoryMetadata(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Ages) != 3 || len(meta.Entities) != 0 || len(meta.Biospheres) != 0 {
		t.Fatal("metadata loaded biological data")
	}
	atlas, err := s.HistoryAtlas(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(atlas.Entities) != 3 || len(atlas.Biospheres) != 0 || len(atlas.Timelines) != 1 || len(atlas.Representations) != 3 {
		t.Fatal("Atlas read loaded biology or lost history/navigation records")
	}
	projection, m, err := s.MapProjection(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	if m.AgeID != age || len(projection.Entities) != 1 || projection.Entity(age, "city").Name != "Old City" || len(projection.Biospheres) != 0 {
		t.Fatal("map projection lost canonical records or loaded lifeforms")
	}
	for _, row := range projection.Maps {
		if row.AgeID != age {
			t.Fatal("map projection read another Age")
		}
	}
	ecology, err := s.EcologyView(ctx, d.World.ID, age, "terrain")
	if err != nil {
		t.Fatal(err)
	}
	if len(ecology.Biospheres) != 1 || len(ecology.Entities) != 2000 || len(ecology.Biospheres[0].Occurrences) != 6000 {
		t.Fatal("ecology scope lost populations")
	}
	if len(ecology.Biospheres[0].Habitats[0].Profiles) != 0 || len(ecology.Entities[0].Biology.Traits) != 0 {
		t.Fatal("inspection retained generation profiles")
	}
	workspace, err := s.HistoryWorkspace(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(workspace.Biospheres) != 3 || len(workspace.Entities) != len(d.Entities) {
		t.Fatal("workspace omitted editable identities or ranges")
	}
	full, err := s.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i := range workspace.Biospheres {
		if len(workspace.Biospheres[i].Habitats[0].Profiles) != 0 || len(full.Biospheres[i].Habitats[0].Profiles) != 3 {
			t.Fatal("workspace retained per-cell generation profiles or modified saved data")
		}
		copy := biosphere.CloneState(full.Biospheres[i])
		copy.Habitats[0].Profiles = nil
		if !reflect.DeepEqual(workspace.Biospheres[i], copy) {
			t.Fatal("workspace discarded fields used by profiles and Biosphere editing")
		}
	}
}

func BenchmarkTerrainHistoryReads(b *testing.B) {
	s, d, project := historyViewFixture(b)
	ctx := context.Background()
	age := d.Maps[0].AgeID
	for _, view := range []string{"full-map-context", "map-projection", "metadata", "ecology"} {
		b.Run(view, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var err error
				switch view {
				case "full-map-context":
					_, _, err = s.MapContext(ctx, project)
				case "map-projection":
					_, _, err = s.MapProjection(ctx, project)
				case "metadata":
					_, err = s.HistoryMetadata(ctx, d.World.ID)
				case "ecology":
					_, err = s.EcologyView(ctx, d.World.ID, age, "terrain")
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
