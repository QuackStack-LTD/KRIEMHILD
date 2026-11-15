package storage

import (
	"context"
	"encoding/json"
	"kriemhild/internal/history"
	"kriemhild/internal/world"
	"reflect"
	"testing"
)

func TestMapEditsPreserveBiospheresOtherAgesAndAuditRestore(t *testing.T) {
	s, initial, project := historyViewFixture(t)
	ctx := context.Background()
	initial, err := s.History(ctx, initial.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	age := initial.Project(project).AgeID
	user, err := s.ResolveUser(ctx, "local", "terrain-editor", "Editor", "editor@example.test", true)
	if err != nil {
		t.Fatal(err)
	}
	ctx = WithActor(ctx, user.ID)
	if err = s.AssignLegacy(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	state := world.New(16, 16)
	state.Header.Name = "Land"
	shape := initial.Shapes(*initial.Project(project))["city"]
	shape.Name = "New City"
	state.Entities[shape.ID] = shape
	updated, err := s.SyncMapEntities(ctx, project, initial.World.Revision, state)
	if err != nil {
		t.Fatal(err)
	}
	baseRevision := updated.World.Revision
	// Database guards prove that geographic commits never rewrite biology.
	for _, statement := range []string{
		`CREATE TRIGGER protect_species_update BEFORE UPDATE ON history_entities WHEN OLD.kind='lifeform' BEGIN SELECT RAISE(ABORT,'species rewritten'); END`,
		`CREATE TRIGGER protect_species_delete BEFORE DELETE ON history_entities WHEN OLD.kind='lifeform' BEGIN SELECT RAISE(ABORT,'species deleted'); END`,
		`CREATE TRIGGER protect_biosphere_update BEFORE UPDATE ON history_biospheres BEGIN SELECT RAISE(ABORT,'biosphere rewritten'); END`,
		`CREATE TRIGGER protect_biosphere_delete BEFORE DELETE ON history_biospheres BEGIN SELECT RAISE(ABORT,'biosphere deleted'); END`,
	} {
		if _, err = s.db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	shape.Points = [][2]float64{{7, 7}}
	state.Entities[shape.ID] = shape
	updated, err = s.SyncMapEntities(ctx, project, updated.World.Revision, state)
	if err != nil {
		t.Fatal(err)
	}
	movedRevision := updated.World.Revision
	shape.Deleted = true
	state.Entities[shape.ID] = shape
	updated, err = s.SyncMapEntities(ctx, project, updated.World.Revision, state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SyncMapEntities(ctx, project, movedRevision, state); err == nil {
		t.Fatal("stale write accepted")
	}
	if _, err = s.CommitHistoricalMap(ctx, project, map[string][]byte{"view": []byte("camera")}, state); err != nil {
		t.Fatal(err)
	}
	current, err := s.History(ctx, initial.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(current.Biospheres, initial.Biospheres) {
		t.Fatal("map edit changed saved biological ranges")
	}
	for _, e := range initial.Entities {
		if e.Kind == "lifeform" || e.AgeID != age {
			if !reflect.DeepEqual(*current.Entity(e.AgeID, e.ID), e) {
				t.Fatal("unrelated record changed", e.ID, e.AgeID)
			}
		}
	}
	for _, revision := range []int{baseRevision, movedRevision, updated.World.Revision} {
		raw, err := s.RevisionSnapshot(ctx, initial.World.ID, revision)
		if err != nil {
			t.Fatal(err)
		}
		var restored history.Document
		if err = json.Unmarshal([]byte(raw), &restored); err != nil {
			t.Fatal(err)
		}
		if err = restored.Validate(); err != nil {
			t.Fatal("invalid expanded audit snapshot", err)
		}
		if !reflect.DeepEqual(restored.Biospheres, initial.Biospheres) {
			t.Fatal("audit expansion lost biology")
		}
		city := restored.Entity(age, shape.ID)
		if city.Deleted != (revision == updated.World.Revision) {
			t.Fatal("audit restored wrong deletion state")
		}
		if revision == movedRevision && restored.Shapes(*restored.Project(project))["city"].Points[0] != ([2]float64{7, 7}) {
			t.Fatal("audit lost moved geometry")
		}
	}
}

func TestTerrainSnapshotSaveDoesNotReadBiologicalJSON(t *testing.T) {
	s, d, project := historyViewFixture(t)
	ctx := context.Background()
	// Deliberately unreadable biology proves neither edit nor snapshot paths
	// deserialize unrelated data. The database lives only for this test.
	if _, err := s.db.ExecContext(ctx, "UPDATE history_biospheres SET data='unreadable'"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, "UPDATE history_entities SET data='unreadable' WHERE kind='lifeform'"); err != nil {
		t.Fatal(err)
	}
	state := world.New(16, 16)
	state.Header.Name = "Land"
	state.Entities["city"] = d.Representations[0].Shape
	if _, err := s.SyncMapEntities(ctx, project, d.World.Revision, state); err != nil {
		t.Fatal("entity edit read biology", err)
	}
	if _, err := s.CommitHistoricalMap(ctx, project, map[string][]byte{"builder/entity/city": []byte("shape")}, state); err != nil {
		t.Fatal("terrain save read biology", err)
	}
	state.Entities = nil
	if _, err := s.CommitHistoricalMap(ctx, project, map[string][]byte{"view": []byte("camera")}, state); err != nil {
		t.Fatal("view save read biology", err)
	}
}

func TestMapDeletionCleansBiologicalNameContextsWithoutReplacingSpecies(t *testing.T) {
	s, d, project := historyViewFixture(t)
	ctx := context.Background()
	age := d.Project(project).AgeID
	updated, err := s.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(d *history.Document) error {
		d.Entity(age, "species-00000000").Names = []history.EntityName{{Name: "City tree", Type: "local", ContextID: "city"}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	state := world.New(16, 16)
	shape := updated.Shapes(*updated.Project(project))["city"]
	shape.Deleted = true
	state.Entities[shape.ID] = shape
	if _, err = s.SyncMapEntities(ctx, project, updated.World.Revision, state); err != nil {
		t.Fatal(err)
	}
	current, err := s.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = current.Validate(); err != nil {
		t.Fatal(err)
	}
	if current.Entity(age, "species-00000000").Names[0].ContextID != "" {
		t.Fatal("deleted place left a dangling species name context")
	}
	shape.ID, shape.Deleted = "species-00000001", false
	state.Entities = map[string]world.Entity{shape.ID: shape}
	if _, err = s.SyncMapEntities(ctx, project, current.World.Revision, state); err == nil {
		t.Fatal("map drawing overwrote a biological identity")
	}
}

func BenchmarkTerrainMapAutosave(b *testing.B) {
	s, d, project := historyViewFixture(b)
	ctx := context.Background()
	state := world.New(16, 16)
	state.Header.Name = "Land"
	state.Entities["city"] = d.Representations[0].Shape
	for _, path := range []string{"whole-world-update", "map-snapshot", "one-entity"} {
		b.Run(path, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var err error
				switch path {
				case "whole-world-update":
					_, err = s.UpdateHistory(ctx, d.World.ID, -1, func(*history.Document) error { return nil })
				case "map-snapshot":
					_, err = s.CommitHistoricalMap(ctx, project, map[string][]byte{"view": []byte("camera")}, state)
				case "one-entity":
					_, err = s.SyncMapEntities(ctx, project, -1, state)
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
