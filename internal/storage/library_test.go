package storage

import (
	"context"
	"encoding/json"
	"kriemhild/internal/history"
	"testing"
)

func TestLibraryVersionsAgeRestartAndPortableBundle(t *testing.T) {
	ctx, dir := context.Background(), t.TempDir()
	s, err := Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	d := history.New("Archives")
	age := d.World.CurrentAge
	for _, e := range []history.Entity{{ID: "city", Kind: "city", Name: "Harbor"}, {ID: "archive", Kind: "archive", Name: "City archive"}, {ID: "org", Kind: "organization", Name: "Keepers"}} {
		if err = d.Apply(history.Command{Kind: "entity", AgeID: age, Entity: &e}); err != nil {
			t.Fatal(err)
		}
	}
	for _, target := range []string{"city", "org"} {
		r := history.Relation{ID: target, Source: "archive", Destination: target, Kind: "archive-located-at"}
		if err = d.Apply(history.Command{Kind: "relation", AgeID: age, Relation: &r}); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	e := history.Entity{ID: "document", AgeID: age, Name: "Decree", Kind: "decree", Properties: json.RawMessage(`{"startYear":12}`)}
	d, err = s.SaveLibraryFile(ctx, d.World.ID, d.World.Revision, e, "archive", "law.txt", "text/plain", []byte("original"))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Years) != 1 {
		t.Fatal("document date missing from historical events")
	}
	originalHash := d.LibraryFile(age, e.ID).Versions[0].Hash
	// Review exclusions remove a whole archive only in the inherited Age.
	var review history.Document
	raw, _ := json.Marshal(d)
	_ = json.Unmarshal(raw, &review)
	if err = review.Apply(history.Command{Kind: "age", Mode: "branch", AgeID: age, Name: "Without archive", ExcludeIDs: []string{"archive"}}); err != nil {
		t.Fatal(err)
	}
	if err = review.Validate(); err != nil {
		t.Fatal(err)
	}
	if review.LibraryFile(review.World.CurrentAge, e.ID) != nil || review.LibraryFile(age, e.ID) == nil {
		t.Fatal("archive exclusion lost source content or retained orphan")
	}

	if _, err = s.SaveLibraryFile(ctx, d.World.ID, d.World.Revision-1, e, "archive", "law.txt", "text/plain", []byte("stale")); err == nil {
		t.Fatal("accepted stale revision")
	}
	d, err = s.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(d *history.Document) error {
		return d.Apply(history.Command{Kind: "age", Mode: "branch", AgeID: age, Name: "Reform"})
	})
	if err != nil {
		t.Fatal(err)
	}
	next := d.World.CurrentAge
	e.AgeID = next
	d, err = s.SaveLibraryFile(ctx, d.World.ID, d.World.Revision, e, "archive", "law.txt", "text/plain", []byte("revised"))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.LibraryFile(age, e.ID).Versions) != 1 || len(d.LibraryFile(next, e.ID).Versions) != 2 {
		t.Fatal("file versions shared mutable Age state")
	}
	if _, err = s.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(d *history.Document) error {
		return d.Apply(history.Command{Kind: "delete-entities", AgeID: next, EntityIDs: []string{"archive"}})
	}); err == nil {
		t.Fatal("nonempty archive deleted")
	}
	s.Close()
	s, err = Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	data, _, err := s.LibraryContent(ctx, d.World.ID, age, e.ID, "")
	if err != nil || string(data) != "original" {
		t.Fatalf("restart lost original: %s %v", data, err)
	}
	b, err := s.HistoryBundle(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Blobs) != 2 {
		t.Fatalf("expected two distinct content blobs, got %d", len(b.Blobs))
	}
	portable, err := Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer portable.Close()
	if err = portable.ImportHistory(ctx, b); err != nil {
		t.Fatal(err)
	}
	data, _, err = portable.LibraryContent(ctx, d.World.ID, next, e.ID, "")
	if err != nil || string(data) != "revised" {
		t.Fatal("bundle lost revised file")
	}
	data, _, err = portable.LibraryContent(ctx, d.World.ID, next, e.ID, originalHash)
	if err != nil || string(data) != "original" {
		t.Fatal("bundle lost earlier file version")
	}
	b.Blobs[originalHash] = []byte("corrupt")
	if err = b.Validate(); err == nil {
		t.Fatal("corrupt attachment accepted")
	}
	d, err = s.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(d *history.Document) error {
		return d.Apply(history.Command{Kind: "delete-entities", AgeID: next, EntityIDs: []string{e.ID}})
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.LibraryFile(next, e.ID) != nil || d.LibraryFile(age, e.ID) == nil {
		t.Fatal("deletion affected another Age or retained active file")
	}
}

func TestLibraryRejectsInvalidReferences(t *testing.T) {
	d := history.New("Archive validation")
	age := d.World.CurrentAge
	d.Entities = []history.Entity{{ID: "a", AgeID: age, Name: "Archive", Kind: "archive", Properties: json.RawMessage(`{"parentArchive":"b"}`)}, {ID: "b", AgeID: age, Name: "Archive B", Kind: "archive", Properties: json.RawMessage(`{"parentArchive":"a"}`)}}
	if err := d.Validate(); err == nil {
		t.Fatal("cyclic folder hierarchy accepted")
	}
	d.Entities[0].Properties = nil
	d.Entities[1].Properties = nil
	d.LibraryFiles = []history.LibraryFile{{ID: "missing", AgeID: age, ArchiveID: "a", Versions: []history.LibraryVersion{{Hash: hash([]byte("x")), Name: "x.txt", Size: 1}}}}
	if err := d.Validate(); err == nil {
		t.Fatal("missing logical document accepted")
	}
}
