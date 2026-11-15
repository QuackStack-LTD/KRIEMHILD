package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/history"
	"kriemhild/internal/world"
	"reflect"
	"testing"
)

func TestRenameHistoryPreservesBiologyOtherAgesAndAudit(t *testing.T) {
	s, initial, project := historyViewFixture(t)
	ctx := context.Background()
	u, err := s.ResolveUser(ctx, "test", "rename-author", "Author", "rename@example.test", true)
	if err != nil {
		t.Fatal(err)
	}
	ctx = WithActor(ctx, u.ID)
	if err = s.AssignLegacy(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	d, err := s.UpdateHistory(ctx, initial.World.ID, initial.World.Revision, func(d *history.Document) error {
		m := d.Project(project)
		e := d.Entity(m.AgeID, "city")
		e.Names = []history.EntityName{{Type: "canonical", Name: e.Name}, {Type: "local", Name: "Local name"}}
		_, err := d.Child(m.AgeID, m.ID, e.ID, e.Name+" - settlement map", world.Bounds{})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	base := d.World.Revision
	age := d.Project(project).AgeID
	changes, err := s.RenameHistory(ctx, d.World.ID, history.Command{Kind: "rename-entity", AgeID: age, EntityID: "city", Name: "New City", Revision: base})
	if err != nil {
		t.Fatal(err)
	}
	if len(changes.Entities) != 1 || len(changes.Representations) != 2 || len(changes.Maps) != 1 || len(changes.Terrains) != 0 {
		t.Fatal("rename returned unrelated records", changes)
	}
	if !s.EcologyUnchangedSince(ctx, d.World.ID, base, changes.World.Revision) || s.MapUnchangedSince(ctx, d.World.ID, base, changes.World.Revision) {
		t.Fatal("name edit freshness classification is wrong")
	}
	renamed, err := s.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(renamed.Biospheres, d.Biospheres) {
		t.Fatal("renaming rewrote biology")
	}
	for _, e := range d.Entities {
		if e.ID != "city" || e.AgeID != age {
			if !reflect.DeepEqual(e, *renamed.Entity(e.AgeID, e.ID)) {
				t.Fatal("unrelated entity changed", e.ID, e.AgeID)
			}
		}
	}
	if renamed.Entity(age, "city").Names[0].Name != "New City" || renamed.Entity(age, "city").Names[1].Name != "Local name" {
		t.Fatal("canonical/local names were not preserved")
	}
	terrain, err := s.RenameHistory(ctx, d.World.ID, history.Command{Kind: "rename-terrain", AgeID: age, TerrainID: "terrain", Name: "New Terrain", Revision: changes.World.Revision})
	if err != nil {
		t.Fatal(err)
	}
	combined, err := s.NameChangesSince(ctx, d.World.ID, base, terrain.World.Revision)
	if err != nil || combined == nil || len(combined.Entities) != 1 || len(combined.Terrains) != 1 || len(combined.Maps) != 2 {
		t.Fatal("consecutive rename changes did not merge", combined, err)
	}
	raw, err := s.RevisionSnapshot(ctx, d.World.ID, terrain.World.Revision)
	if err != nil {
		t.Fatal(err)
	}
	var restored history.Document
	if err = json.Unmarshal([]byte(raw), &restored); err != nil {
		t.Fatal(err)
	}
	if err = restored.Validate(); err != nil {
		t.Fatal(err)
	}
	if restored.Entity(age, "city").Name != "New City" || restored.Map(age, d.Project(project).ID).Name != "New Terrain" || !reflect.DeepEqual(restored.Biospheres, d.Biospheres) {
		t.Fatal("audit expansion lost names or biology")
	}
	if _, err = s.RenameHistory(ctx, d.World.ID, history.Command{Kind: "rename-entity", AgeID: age, EntityID: "city", Name: "Stale", Revision: base}); err == nil {
		t.Fatal("accepted stale rename")
	}
	for _, name := range []string{" ", string(make([]byte, 201))} {
		if _, err = s.RenameHistory(ctx, d.World.ID, history.Command{Kind: "rename-entity", AgeID: age, EntityID: "city", Name: name, Revision: terrain.World.Revision}); err == nil {
			t.Fatal("accepted invalid name")
		}
	}
	_, err = s.UpdateHistory(ctx, d.World.ID, terrain.World.Revision, func(d *history.Document) error { d.World.Name = "Other edit"; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if mixed, err := s.NameChangesSince(ctx, d.World.ID, base, terrain.World.Revision+1); err != nil || mixed != nil {
		t.Fatal("mixed revisions were classified as names only", err)
	}
	biologyBase := terrain.World.Revision + 1
	life, err := s.RenameHistory(ctx, d.World.ID, history.Command{Kind: "rename-entity", AgeID: age, EntityID: "species-00000000", Name: "Renamed lifeform", Revision: biologyBase})
	if err != nil {
		t.Fatal(err)
	}
	if s.EcologyUnchangedSince(ctx, d.World.ID, biologyBase, life.World.Revision) {
		t.Fatal("species label change reused stale ecological labels")
	}
}

func TestRenameHistoryDoesNotReadUnrelatedBiologicalJSON(t *testing.T) {
	s, d, project := historyViewFixture(t)
	ctx := context.Background()
	for _, q := range []string{
		"UPDATE history_biospheres SET data='unreadable'",
		"UPDATE history_entities SET data='unreadable' WHERE kind='lifeform'",
		`CREATE TRIGGER protect_species_update BEFORE UPDATE ON history_entities WHEN OLD.kind='lifeform' BEGIN SELECT RAISE(ABORT,'species rewritten'); END`,
		`CREATE TRIGGER protect_biosphere_update BEFORE UPDATE ON history_biospheres BEGIN SELECT RAISE(ABORT,'biology rewritten'); END`,
	} {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	change, err := s.RenameHistory(ctx, d.World.ID, history.Command{Kind: "rename-entity", AgeID: d.Project(project).AgeID, EntityID: "city", Name: "New City", Revision: d.World.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if change.Entities[0].Name != "New City" {
		t.Fatal("rename failed")
	}
}

func BenchmarkStoredEntityRename(b *testing.B) {
	for _, n := range []int{2000, 8000} {
		for _, projected := range []bool{false, true} {
			b.Run(fmt.Sprintf("species-%d/projected-%t", n, projected), func(b *testing.B) {
				ctx := context.Background()
				s, err := Open(ctx, b.TempDir(), "")
				if err != nil {
					b.Fatal(err)
				}
				defer s.Close()
				u, err := s.ResolveUser(ctx, "test", "author", "Author", "author@example.test", true)
				if err != nil {
					b.Fatal(err)
				}
				ctx = WithActor(ctx, u.ID)
				d := storedBiosphereWorld(n)
				d.Entities = append(d.Entities, history.Entity{ID: "city", AgeID: d.World.CurrentAge, Name: "Old City", Kind: "city", Properties: json.RawMessage(`{}`)})
				if err = s.CreateHistory(ctx, d); err != nil {
					b.Fatal(err)
				}
				revision := d.World.Revision
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					c := history.Command{Kind: "rename-entity", AgeID: d.World.CurrentAge, EntityID: "city", Name: fmt.Sprintf("City %d", i), Revision: revision}
					if projected {
						change, e := s.RenameHistory(ctx, d.World.ID, c)
						if e != nil {
							b.Fatal(e)
						}
						revision = change.World.Revision
					} else {
						updated, e := s.UpdateHistory(ctx, d.World.ID, revision, func(d *history.Document) error { return d.Apply(c) })
						if e != nil {
							b.Fatal(e)
						}
						revision = updated.World.Revision
					}
				}
			})
		}
	}
}
