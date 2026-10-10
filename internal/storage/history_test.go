package storage

import (
	"context"
	"kriemhild/internal/history"
	"testing"
)

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
