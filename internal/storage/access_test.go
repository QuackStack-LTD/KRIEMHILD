package storage

import (
	"context"
	"kriemhild/internal/history"
	"path/filepath"
	"testing"
)

func TestLegacyOwnershipAndDatabaseRecovery(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, e := Open(ctx, dir, "")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	legacy := history.New("Legacy world")
	if e = db.CreateHistory(ctx, legacy); e != nil {
		t.Fatal(e)
	}
	u, e := db.ResolveUser(ctx, "issuer", "immutable", "Author", "first@example.test", true)
	if e != nil {
		t.Fatal(e)
	}
	same, e := db.ResolveUser(ctx, "issuer", "immutable", "New name", "changed@example.test", true)
	if e != nil || same.ID != u.ID {
		t.Fatal("email changed identity", e)
	}
	if e = db.AssignLegacy(ctx, u.ID); e != nil {
		t.Fatal(e)
	}
	if e = db.AssignLegacy(ctx, u.ID); e != nil {
		t.Fatal("migration not idempotent", e)
	}
	ctx = WithActor(ctx, u.ID)
	if _, e = db.UpdateHistory(ctx, legacy.World.ID, 1, func(d *history.Document) error { d.World.Name = "Edited legacy world"; return nil }); e != nil {
		t.Fatal(e)
	}
	p := WritingProject{ID: "project", WorldID: legacy.World.ID, Name: "Novel"}
	if e = db.SaveWriterProject(ctx, &p); e != nil {
		t.Fatal(e)
	}
	n := WritingNode{ID: "scene", ProjectID: p.ID, Type: "scene", Title: "Scene", Status: "Drafting", Schema: 1, Content: plainWriterContent("Saved text")}
	if e = db.SaveWriterNode(ctx, p.ID, &n, "Named version"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.AppendDocumentUpdate(ctx, n.ID, []byte{1, 2, 3}, 0); e != nil {
		t.Fatal(e)
	}
	backupDir := t.TempDir()
	if e = db.BackupSQLite(ctx, filepath.Join(backupDir, "kriemhild.sqlite")); e != nil {
		t.Fatal(e)
	}
	recovered, e := Open(ctx, backupDir, "")
	if e != nil {
		t.Fatal(e)
	}
	defer recovered.Close()
	if role, e := recovered.Role(ctx, legacy.World.ID, u.ID); e != nil || role != "owner" {
		t.Fatal(role, e)
	}
	if revs, e := recovered.Revisions(ctx, legacy.World.ID, 0); e != nil || len(revs) != 1 || revs[0].Author != u.ID {
		t.Fatal(revs, e)
	}
	if updates, e := recovered.DocumentUpdates(ctx, n.ID); e != nil || len(updates) != 1 {
		t.Fatal(updates, e)
	}
	if bundle, e := recovered.WriterBundle(ctx, p.ID); e != nil || len(bundle.Versions) != 1 {
		t.Fatal(bundle, e)
	}
}

func TestLocalOwnershipCanMigrateOnlyToExplicitHostedOwner(t *testing.T) {
	ctx := context.Background()
	db, e := Open(ctx, t.TempDir(), "")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	local, _ := db.ResolveUser(ctx, "kriemhild:local", "local", "Local", "", false)
	hosted, _ := db.ResolveUser(ctx, "issuer", "hosted", "Hosted", "hosted@example.test", true)
	d := history.New("Local world")
	if e = db.CreateHistory(WithActor(ctx, local.ID), d); e != nil {
		t.Fatal(e)
	}
	if role, _ := db.Role(ctx, d.World.ID, hosted.ID); role != "" {
		t.Fatal("login alone granted ownership")
	}
	if e = db.AssignLegacy(ctx, hosted.ID); e != nil {
		t.Fatal(e)
	}
	if role, _ := db.Role(ctx, d.World.ID, hosted.ID); role != "owner" {
		t.Fatal("explicit migration failed")
	}
	if role, _ := db.Role(ctx, d.World.ID, local.ID); role != "" {
		t.Fatal("legacy actor retained access")
	}
	if e = db.AssignLegacy(ctx, local.ID); e != nil {
		t.Fatal(e)
	}
	if role, _ := db.Role(ctx, d.World.ID, local.ID); role != "" {
		t.Fatal("local mode reclaimed a hosted world")
	}
}

func TestNamedManuscriptRecoveryCreatesNewVersion(t *testing.T) {
	ctx := context.Background()
	db, e := Open(ctx, t.TempDir(), "")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	u, _ := db.ResolveUser(ctx, "issuer", "author", "Author", "author@example.test", true)
	ctx = WithActor(ctx, u.ID)
	d := history.New("World")
	if e = db.CreateHistory(ctx, d); e != nil {
		t.Fatal(e)
	}
	p := WritingProject{ID: "project", WorldID: d.World.ID, Name: "Novel"}
	if e = db.SaveWriterProject(ctx, &p); e != nil {
		t.Fatal(e)
	}
	n := WritingNode{ID: "scene", ProjectID: p.ID, Title: "Scene", Type: "scene", Status: "Drafting", Schema: 1, Content: plainWriterContent("Original text")}
	if e = db.SaveWriterNode(ctx, p.ID, &n, "Named version"); e != nil {
		t.Fatal(e)
	}
	n.Content = plainWriterContent("Changed text")
	if e = db.SaveWriterNode(ctx, p.ID, &n, ""); e != nil {
		t.Fatal(e)
	}
	if _, e = db.AppendDocumentUpdate(ctx, n.ID, []byte{1, 2, 3}, 0); e != nil {
		t.Fatal(e)
	}
	restored, e := db.RestoreManuscript(ctx, p.ID, n.ID, 1, 2)
	if e != nil || restored.Revision != 3 || restored.Text != "Original text" || restored.AuthorID != u.ID {
		t.Fatal(restored, e)
	}
	if updates, e := db.DocumentUpdates(ctx, n.ID); e != nil || len(updates) != 0 {
		t.Fatal("old collaboration state retained", e)
	}
	if _, e = db.RestoreManuscript(ctx, p.ID, n.ID, 1, 2); e != ErrWritingConflict {
		t.Fatal("stale restore accepted", e)
	}
	bundle, e := db.WriterBundle(ctx, p.ID)
	if e != nil || len(bundle.Versions) != 3 {
		t.Fatal("history lost", e)
	}
}
func TestWorldBundleKeepsWritingWithoutSharingPermissions(t *testing.T) {
	ctx := context.Background()
	db, e := Open(ctx, t.TempDir(), "")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	a, _ := db.ResolveUser(ctx, "issuer", "a", "A", "a@example.test", true)
	b, _ := db.ResolveUser(ctx, "issuer", "b", "B", "b@example.test", true)
	ctx = WithActor(ctx, a.ID)
	d := history.New("World")
	if e = db.CreateHistory(ctx, d); e != nil {
		t.Fatal(e)
	}
	p := WritingProject{ID: "project", Name: "Novel", WorldID: d.World.ID}
	if e = db.SaveWriterProject(ctx, &p); e != nil {
		t.Fatal(e)
	}
	n := WritingNode{ID: "scene", ProjectID: p.ID, Title: "Scene", Type: "scene", Schema: 1, Status: "Drafting", Content: plainWriterContent("Manuscript")}
	if e = db.SaveWriterNode(ctx, p.ID, &n, "First"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.AppendDocumentUpdate(ctx, n.ID, []byte{1, 2, 3}, 0); e != nil {
		t.Fatal(e)
	}
	bundle, e := db.HistoryBundle(ctx, d.World.ID)
	if e != nil || len(bundle.Writing) != 1 {
		t.Fatal(e)
	}
	copyID := history.ID()
	bundle.Document.World.ID = copyID
	if e = db.ImportHistory(WithActor(ctx, b.ID), bundle); e != nil {
		t.Fatal(e)
	}
	if role, e := db.Role(ctx, copyID, b.ID); e != nil || role != "owner" {
		t.Fatal(role, e)
	}
	if role, _ := db.Role(ctx, copyID, a.ID); role != "" {
		t.Fatal("source owner granted access to imported copy")
	}
	projects, e := db.WriterProjects(ctx, copyID)
	if e != nil || len(projects) != 1 {
		t.Fatal(projects, e)
	}
	copy, e := db.WriterBundle(ctx, projects[0].ID)
	if e != nil || len(copy.Nodes) != 1 || len(copy.Updates[copy.Nodes[0].ID]) != 1 {
		t.Fatal(copy, e)
	}
}
