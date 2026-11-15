package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/biosphere"
	"kriemhild/internal/history"
	"testing"
)

func storedBiosphereWorld(n int) *history.Document {
	d := history.New("Large Biosphere")
	age := d.World.CurrentAge
	d.Terrains = []history.Terrain{{ID: "terrain", AgeID: age, Name: "Earth", Width: 16, Height: 16}}
	s := biosphere.State{ID: "terrain", AgeID: age, TerrainID: "terrain", Version: 1, Width: 16, Height: 16, Stale: true, Habitats: []biosphere.Habitat{{ID: "forest", Cells: []int{1, 2, 3}}}, Settings: biosphere.Settings{Mode: "guided", Richness: "moderate", Coherence: "realistic", Scope: "entire", Familiarity: .5, Seed: "seed"}}
	biology := biosphere.Catalogue()[0].Biology
	biology.Dependencies = nil
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("species-%08d", i)
		b := biology
		if i > 0 {
			b.ParentID = "species-00000000"
		}
		d.Entities = append(d.Entities, history.Entity{ID: id, AgeID: age, TerrainID: "terrain", Name: id, Kind: "lifeform", Biology: &b, Properties: json.RawMessage(`{}`)})
		for j := 0; j < 3; j++ {
			s.Occurrences = append(s.Occurrences, biosphere.Occurrence{ID: fmt.Sprintf("%s/%d", id, j), SpeciesID: id, HabitatID: "forest", TerrainID: "terrain", Status: "native", Abundance: .5})
		}
	}
	d.Biospheres = []biosphere.State{s}
	return d
}

func BenchmarkStoredBiosphereAge(b *testing.B) {
	for _, n := range []int{2000, 8000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				s, err := Open(context.Background(), b.TempDir(), "")
				if err != nil {
					b.Fatal(err)
				}
				d := storedBiosphereWorld(n)
				if err = s.CreateHistory(context.Background(), d); err != nil {
					b.Fatal(err)
				}
				b.StartTimer()
				_, err = s.UpdateHistory(context.Background(), d.World.ID, d.World.Revision, func(d *history.Document) error {
					return d.Apply(history.Command{Kind: "age", AgeID: d.World.CurrentAge, Name: "Later", Mode: "after"})
				})
				if err != nil {
					b.Fatal(err)
				}
				b.StopTimer()
				s.Close()
				b.StartTimer()
			}
		})
	}
}

func TestLargeBiosphereAgePersistsIndependentStateAndRevision(t *testing.T) {
	ctx, dir := context.Background(), t.TempDir()
	s, err := Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	u, err := s.ResolveUser(ctx, "test", "age-author", "Author", "age@example.test", true)
	if err != nil {
		t.Fatal(err)
	}
	ctx = WithActor(ctx, u.ID)
	d := storedBiosphereWorld(2000)
	if err = s.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	first := d.World.CurrentAge
	d, err = s.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(d *history.Document) error {
		return d.Apply(history.Command{Kind: "age", AgeID: first, Mode: "after", Name: "Later", ExcludeIDs: []string{"species-00000000"}})
	})
	if err != nil {
		t.Fatal(err)
	}
	worldID, later := d.World.ID, d.World.CurrentAge
	if _, err = s.UpdateHistory(ctx, worldID, 1, func(d *history.Document) error { d.World.Name = "Stale"; return nil }); err == nil {
		t.Fatal("accepted stale revision")
	}
	s.Close()
	s, err = Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	d, err = s.History(ctx, worldID)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Entities) != 4000 || d.World.CurrentAge != later || len(d.Biosphere(first, "terrain").Occurrences) != 6000 || len(d.Biosphere(later, "terrain").Occurrences) != 5997 {
		t.Fatal("reopen lost Biosphere inheritance or source populations")
	}
	if d.Entity(first, "species-00000000").Deleted || !d.Entity(later, "species-00000000").Deleted || d.Entity(later, "species-00000001").Biology.ParentID != "" {
		t.Fatal("exclusion leaked across Ages")
	}
	raw, err := s.RevisionSnapshot(ctx, worldID, 2)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot history.Document
	if err = json.Unmarshal([]byte(raw), &snapshot); err != nil || len(snapshot.Entities) != 4000 {
		t.Fatal("revision snapshot lost inherited entities", err)
	}
	if err = d.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestAgeSaveDoesNotRewriteUnchangedSourceRows(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d := storedBiosphereWorld(1000)
	if err = s.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	first := d.World.CurrentAge
	// Make redundant writes fail, rather than relying on wall-clock timing.
	for _, table := range []string{"entities", "biospheres"} {
		for _, operation := range []string{"UPDATE", "DELETE"} {
			query := fmt.Sprintf("CREATE TRIGGER preserve_%s_%s BEFORE %s ON history_%s WHEN OLD.age_id='%s' BEGIN SELECT RAISE(ABORT, 'unchanged source was rewritten'); END", table, operation, operation, table, first)
			if _, err = s.db.ExecContext(ctx, query); err != nil {
				t.Fatal(err)
			}
		}
	}
	d, err = s.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(d *history.Document) error {
		return d.Apply(history.Command{Kind: "age", AgeID: first, Mode: "after", Name: "Later"})
	})
	if err != nil {
		t.Fatal("inheritance rewrote existing Age rows", err)
	}
	later := d.World.CurrentAge
	d, err = s.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(d *history.Document) error { d.Entity(later, "species-00000001").Name = "Adapted"; return nil })
	if err != nil {
		t.Fatal("edit rewrote unrelated source rows", err)
	}
	for _, table := range []string{"entities", "biospheres"} {
		for _, operation := range []string{"UPDATE", "DELETE"} {
			if _, err = s.db.ExecContext(ctx, "DROP TRIGGER preserve_"+table+"_"+operation); err != nil {
				t.Fatal(err)
			}
		}
	}
	d, err = s.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(d *history.Document) error { return d.Apply(history.Command{Kind: "delete-age", AgeID: first}) })
	if err != nil {
		t.Fatal(err)
	}
	d, err = s.History(ctx, d.World.ID)
	if err != nil || len(d.Entities) != 1000 || len(d.Biospheres) != 1 || d.Entity(later, "species-00000001").Name != "Adapted" {
		t.Fatal("differential updates or deletions lost state", err)
	}
}
