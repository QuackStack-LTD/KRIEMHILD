package storage

import (
	"context"
	"kriemhild/internal/history"
	"kriemhild/internal/world"
	"testing"
)

func TestDeletedAgeSurvivesRestartAndPortableBundle(t *testing.T) {
	ctx, dir := context.Background(), t.TempDir()
	s, err := Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	d := history.New("Deletion")
	if err = s.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	first := d.World.CurrentAge
	parts := map[string][]byte{"terrain.json": []byte("immutable terrain"), "detail/0.json": []byte("explored detail")}
	d, m, err := s.AttachTerrain(ctx, d.World.ID, first, history.ID(), "Terra", 32, 32, parts, world.New(32, 32))
	if err != nil {
		t.Fatal(err)
	}
	oldProject := m.ProjectID
	d, err = s.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(d *history.Document) error {
		return d.Apply(history.Command{Kind: "age", Mode: "branch", AgeID: first, Name: "Successor"})
	})
	if err != nil {
		t.Fatal(err)
	}
	survivor := d.World.CurrentAge
	d, err = s.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(d *history.Document) error { return d.Apply(history.Command{Kind: "delete-age", AgeID: first}) })
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, _, err = s.MapContext(ctx, oldProject); err == nil {
		t.Fatal("deleted Age map remains openable")
	}
	loaded, err := s.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Ages) != 1 || loaded.Ages[0].ID != survivor || len(loaded.Maps) != 1 || loaded.Maps[0].Snapshot != m.Snapshot {
		t.Fatal("deletion lost or inherited snapshot altered")
	}
	bundle, err := s.HistoryBundle(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	portable, err := Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer portable.Close()
	if err = portable.ImportHistory(ctx, bundle); err != nil {
		t.Fatal(err)
	}
	restored, err := portable.History(ctx, d.World.ID)
	if err != nil || restored.Age(first) != nil || restored.World.CurrentAge != survivor {
		t.Fatal("deleted Age resurrected on import", err)
	}
	data, err := portable.SnapshotParts(ctx, restored.Maps[0].Snapshot)
	if err != nil || string(data["detail/0.json"]) != "explored detail" {
		t.Fatal("surviving Age detail lost", err)
	}
}

func TestHistoricalTransactionsRejectStaleAndInvalidChanges(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d := history.New("Chronicles")
	if err = s.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	first, revision := d.World.CurrentAge, d.World.Revision
	d, err = s.UpdateHistory(ctx, d.World.ID, revision, func(d *history.Document) error {
		return d.Apply(history.Command{Kind: "age", AgeID: first, Mode: "branch", Name: "Alternative"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpdateHistory(ctx, d.World.ID, revision, func(d *history.Document) error {
		return d.Apply(history.Command{Kind: "rename-world", Name: "Stale overwrite"})
	}); err == nil {
		t.Fatal("stale World command accepted")
	}
	branch := d.World.CurrentAge
	if d, err = s.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(d *history.Document) error {
		return d.Apply(history.Command{Kind: "connect", AgeID: branch, OtherAge: first})
	}); err != nil {
		t.Fatal("cyclic transaction rejected", err)
	}
	loaded, err := s.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.World.Revision != d.World.Revision || loaded.World.Name != "Chronicles" || len(loaded.Edges) != 2 {
		t.Fatal("failed command changed saved history")
	}
	if loaded.Timelines[0].Order != 0 || loaded.Timelines[1].Order != 1 || loaded.Timelines[0].Ages[0] != first {
		t.Fatal("reload reordered timeline lanes")
	}
	bundle, err := s.HistoryBundle(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	portable, err := Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer portable.Close()
	if err = portable.ImportHistory(ctx, bundle); err != nil {
		t.Fatal("chronological loop rejected by portable history", err)
	}
	restored, err := portable.History(ctx, d.World.ID)
	if err != nil || len(restored.Edges) != 2 {
		t.Fatal("chronological loop lost on import", err)
	}
}
