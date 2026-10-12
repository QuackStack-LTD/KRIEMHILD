package storage

import (
	"context"
	"encoding/json"
	"kriemhild/internal/history"
	"testing"
)

func TestArchiveGatherAtomicIdempotentPortableAndCustody(t *testing.T) {
	ctx, dir := context.Background(), t.TempDir()
	s, err := Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	d := history.New("Documented world")
	age := d.World.CurrentAge
	for _, e := range []history.Entity{{ID: "owner", Name: "Village", Kind: "village", Properties: json.RawMessage(`{"startYear":2,"population":100}`)}, {ID: "archive", Name: "Village archive", Kind: "archive", Archive: &history.ArchiveConfig{Version: 1, OwnerID: "owner", Purpose: "general", Intensity: "exhaustive", Categories: []string{"chronicles", "administrative"}, KnowledgeScope: "direct", Retention: "preserve"}}, {ID: "destination", Name: "Monastery archive", Kind: "archive", Archive: &history.ArchiveConfig{Version: 1, OwnerID: "owner", Purpose: "religious", Intensity: "moderate", Categories: []string{"chronicles"}, KnowledgeScope: "direct", Retention: "preserve"}}} {
		if err = d.Apply(history.Command{Kind: "entity", AgeID: age, Entity: &e}); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	req := history.GatherRequest{AgeID: age, ArchiveID: "archive", SourceAge: age, Scope: "age", RecordingYear: 10, Revision: d.World.Revision, Preview: true}
	before := d.World.Revision
	_, preview, err := s.GatherArchive(ctx, d.World.ID, req)
	if err != nil || len(preview.Created) != 2 {
		t.Fatalf("preview %v %v", preview, err)
	}
	stored, _ := s.History(ctx, d.World.ID)
	if stored.World.Revision != before || len(stored.LibraryFiles) != 0 {
		t.Fatal("preview mutated persistence")
	}
	req.Preview = false
	d, result, err := s.GatherArchive(ctx, d.World.ID, req)
	if err != nil || len(result.Created) != 2 {
		t.Fatal("gather", err)
	}
	fileID := result.Created[0].DocumentID
	original, _, err := s.LibraryContent(ctx, d.World.ID, age, fileID, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.GatherArchive(ctx, d.World.ID, req); err == nil {
		t.Fatal("stale revision accepted")
	}
	req.Revision = d.World.Revision
	d, result, err = s.GatherArchive(ctx, d.World.ID, req)
	if err != nil || len(result.Created) != 0 {
		t.Fatal("repeat duplicated", err)
	}
	// Authored text retains provenance and is not overwritten by subsequent gathering.
	e := *d.Entity(age, fileID)
	d, err = s.SaveLibraryFile(ctx, d.World.ID, d.World.Revision, e, "archive", "account.md", "text/markdown", []byte("An authored interpretation"))
	if err != nil {
		t.Fatal(err)
	}
	f := d.LibraryFile(age, fileID)
	if !f.Record.AuthoredEdits || f.Versions[0].Provenance.AuthoredEdits {
		t.Fatal("version provenance mutated")
	}
	req.Revision = d.World.Revision
	d, result, err = s.GatherArchive(ctx, d.World.ID, req)
	if err != nil || len(result.Created) != 0 {
		t.Fatal("authored content regenerated")
	}
	req.ReviseID = fileID
	req.Revision = d.World.Revision
	d, result, err = s.GatherArchive(ctx, d.World.ID, req)
	if err != nil || len(result.Created) != 1 {
		t.Fatal("explicit revision", err)
	}
	if len(d.LibraryFile(age, fileID).Versions) != 3 {
		t.Fatal("revision replaced history")
	}
	req.ReviseID = ""
	d, err = s.ArchiveCustody(ctx, d.World.ID, ArchiveCustodyRequest{Revision: d.World.Revision, AgeID: age, DocumentID: fileID, Destination: "destination", Action: "copy", Year: 11, Reason: "Sent to monastery"})
	if err != nil {
		t.Fatal(err)
	}
	var copyID string
	for _, f := range d.LibraryFiles {
		if f.CopyOf == fileID {
			copyID = f.ID
		}
	}
	if copyID == "" || d.LibraryFile(age, fileID).ArchiveID != "archive" || d.LibraryFile(age, copyID).ArchiveID != "destination" {
		t.Fatal("copy lost physical identity")
	}
	d, err = s.ArchiveCustody(ctx, d.World.ID, ArchiveCustodyRequest{Revision: d.World.Revision, AgeID: age, DocumentID: fileID, Action: "destroy", Year: 12, Reason: "Archive fire"})
	if err != nil {
		t.Fatal(err)
	}
	req.Revision = d.World.Revision
	d, result, err = s.GatherArchive(ctx, d.World.ID, req)
	if err != nil || len(result.Created) != 0 {
		t.Fatal("destroyed document regenerated")
	}
	if _, err = s.ArchiveCustody(ctx, d.World.ID, ArchiveCustodyRequest{Revision: d.World.Revision, AgeID: age, DocumentID: fileID, Action: "recover", Year: 13, Reason: "Found"}); err == nil {
		t.Fatal("destroyed physical document recovered")
	}
	d, err = s.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(d *history.Document) error {
		return d.Apply(history.Command{Kind: "age", AgeID: age, Mode: "branch", Name: "Later age"})
	})
	if err != nil {
		t.Fatal(err)
	}
	next := d.World.CurrentAge
	d, err = s.ArchiveCustody(ctx, d.World.ID, ArchiveCustodyRequest{Revision: d.World.Revision, AgeID: next, DocumentID: copyID, Action: "lose", Year: 1, Reason: "Lost during migration"})
	if err != nil {
		t.Fatal(err)
	}
	if d.LibraryFile(age, copyID).Status != "present" || d.LibraryFile(next, copyID).Status != "lost" {
		t.Fatal("custody leaked across Ages")
	}
	s.Close()
	s, err = Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
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
	if err != nil {
		t.Fatal(err)
	}
	if restored.Entity(next, "archive").Archive.OwnerID != "owner" || restored.LibraryFile(next, copyID).CopyOf != fileID || restored.LibraryFile(next, copyID).Status != "lost" {
		t.Fatal("bundle lost archive or custody")
	}
	bytes, _, err := portable.LibraryContent(ctx, d.World.ID, age, fileID, restored.LibraryFile(age, fileID).Versions[0].Hash)
	if err != nil || string(bytes) != string(original) {
		t.Fatal("bundle lost original generated artifact")
	}
	req.Revision = restored.World.Revision
	_, result, err = portable.GatherArchive(ctx, d.World.ID, req)
	if err != nil || len(result.Created) != 0 {
		t.Fatal("restoration lost coverage", err)
	}
}
