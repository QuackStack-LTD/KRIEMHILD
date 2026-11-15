package storage

import (
	"bytes"
	"context"
	"errors"
	"kriemhild/internal/history"
	"testing"
)

func TestWriterArchiveFoldersTrashRestoreMoveAndAttachmentBackup(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d := history.New("Archive World")
	if err = s.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	p, err := s.EnsureWriterArchive(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.EnsureWriterArchive(ctx, d.World.ID)
	if err != nil || again.ID != p.ID || again.Revision != p.Revision {
		t.Fatal("archive initialization must be idempotent", err)
	}
	save := func(id, parent, kind string) WritingNode {
		t.Helper()
		n := testWriterNode(id, parent)
		n.ProjectID = p.ID
		n.Type = kind
		if err := s.SaveWriterNode(ctx, p.ID, &n, "Created"); err != nil {
			t.Fatal(err)
		}
		return n
	}
	folder := save("research", "", "folder")
	child := save("note", folder.ID, "document")
	olderTrash := save("previous-trash", folder.ID, "note")
	action := func(n WritingNode, kind string) error {
		return s.WriterArchiveAction(ctx, d.World.ID, WriterArchiveAction{Action: kind, ProjectID: n.ProjectID, NodeID: n.ID, Revision: n.Revision})
	}
	if err = action(olderTrash, "trash"); err != nil {
		t.Fatal(err)
	}
	asset := testWriterNode("drawing", folder.ID)
	asset.ProjectID = p.ID
	asset.Type = "asset"
	original := []byte{0, 1, 2, 255}
	if err = s.SaveWriterFile(ctx, p.ID, &asset, "drawing.png", "image/png", original); err != nil {
		t.Fatal(err)
	}
	if err = action(folder, "trash"); err != nil {
		t.Fatal(err)
	}
	if err = action(folder, "restore"); !errors.Is(err, ErrWritingConflict) {
		t.Fatalf("stale restore must conflict: %v", err)
	}
	b, err := s.WriterCurrent(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range b.Nodes {
		if !n.Deleted {
			t.Fatal("folder deletion left a live child", n.ID)
		}
		if n.ID == folder.ID {
			folder = n
		}
	}
	if err = action(folder, "restore"); err != nil {
		t.Fatal(err)
	}
	b, err = s.WriterCurrent(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range b.Nodes {
		if n.Deleted != (n.ID == olderTrash.ID) {
			t.Fatal("restore resurrected separately deleted file", n.ID)
		}
		if n.ID == folder.ID {
			folder = n
		}
		if n.ID == asset.ID {
			asset = n
		}
	}
	if err = s.WriterArchiveAction(ctx, d.World.ID, WriterArchiveAction{Action: "move", ProjectID: p.ID, NodeID: folder.ID, Revision: folder.Revision, TargetProject: p.ID, ParentID: folder.ID}); err == nil {
		t.Fatal("cyclic move accepted")
	}
	otherWorld := history.New("Other")
	if err = s.CreateHistory(ctx, otherWorld); err != nil {
		t.Fatal(err)
	}
	other, err := s.EnsureWriterArchive(ctx, otherWorld.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.WriterArchiveAction(ctx, d.World.ID, WriterArchiveAction{Action: "move", ProjectID: p.ID, NodeID: folder.ID, Revision: folder.Revision, TargetProject: other.ID}); err == nil {
		t.Fatal("cross-world move accepted")
	}
	target := WritingProject{ID: "manuscript-folder", WorldID: d.World.ID, Name: "Manuscript"}
	if err = s.SaveWriterProject(ctx, &target); err != nil {
		t.Fatal(err)
	}
	if err = s.WriterArchiveAction(ctx, d.World.ID, WriterArchiveAction{Action: "move", ProjectID: p.ID, NodeID: asset.ID, Revision: asset.Revision, TargetProject: target.ID}); err != nil {
		t.Fatal(err)
	}
	file, n, err := s.WriterFile(ctx, target.ID, asset.ID, "")
	if err != nil || !bytes.Equal(file, original) || n.ParentID != "" {
		t.Fatal("moved attachment lost", err)
	}
	if _, _, err = s.WriterFile(ctx, p.ID, asset.ID, ""); err == nil {
		t.Fatal("old project still exposes moved attachment")
	}
	// An embedded image has a World-scoped identity, even if its standalone
	// asset and document are moved into different manuscript folders.
	child.Content = []byte(`{"type":"doc","content":[{"type":"image","attrs":{"src":"/api/worlds/` + d.World.ID + `/writer-archive/files/` + asset.FileHash + `"}}]}`)
	currentChild, childErr := s.WriterNode(ctx, p.ID, child.ID)
	if childErr != nil {
		t.Fatal(childErr)
	}
	child.Revision = currentChild.Revision
	if err = s.SaveWriterNode(ctx, p.ID, &child, "Embedded image"); err != nil {
		t.Fatal(err)
	}
	docProject := WritingProject{ID: "image-document", WorldID: d.World.ID, Name: "Illustrated draft"}
	if err = s.SaveWriterProject(ctx, &docProject); err != nil {
		t.Fatal(err)
	}
	if err = s.WriterArchiveAction(ctx, d.World.ID, WriterArchiveAction{Action: "move", ProjectID: p.ID, NodeID: child.ID, Revision: child.Revision, TargetProject: docProject.ID}); err != nil {
		t.Fatal(err)
	}
	docBackup, err := s.WriterBundle(ctx, docProject.ID)
	if err != nil || !bytes.Equal(docBackup.Files[asset.FileHash], original) {
		t.Fatal("standalone document backup lost its embedded image", err)
	}
	if _, err = s.ImportWriter(ctx, docBackup, d.World.ID); err != nil {
		t.Fatal("illustrated document restore", err)
	}
	backup, err := s.WriterBundle(ctx, target.ID)
	if err != nil || !bytes.Equal(backup.Files[asset.FileHash], original) {
		t.Fatal("backup omits attachment", err)
	}
	restored, err := s.ImportWriter(ctx, backup, d.World.ID)
	if err != nil {
		t.Fatal("moved file history could not be imported", err)
	}
	copy, err := s.WriterBundle(ctx, restored.ID)
	if err != nil || len(copy.Nodes) != 1 || !bytes.Equal(copy.Files[copy.Nodes[0].FileHash], original) {
		t.Fatal("attachment restore failed", err)
	}
	index, err := s.WriterArchive(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range index.Nodes {
		if n.Content != nil || n.Text != "" {
			t.Fatal("archive listing loaded document bodies")
		}
	}
	current, err := s.HistoryMetadata(ctx, d.World.ID)
	if err != nil || current.World.Revision != d.World.Revision {
		t.Fatal("author archive mutated fictional history", err)
	}
	_ = child
}

func TestWriterArchiveRejectsInvalidFileWithoutPartialWrites(t *testing.T) {
	ctx := context.Background()
	s, p := writerFixture(t)
	n := testWriterNode("invalid-file", "")
	n.Type = "asset"
	n.ParentID = "missing"
	if err := s.SaveWriterFile(ctx, p.ID, &n, "image.png", "image/png", []byte("unchanged original")); err == nil {
		t.Fatal("invalid folder accepted")
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM writer_files`).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed upload left orphan bytes", count, err)
	}
	n = testWriterNode("forged", "")
	n.Type = "asset"
	n.FileHash = "missing"
	n.FileName = "image.png"
	if err := s.SaveWriterNode(ctx, p.ID, &n, ""); err == nil {
		t.Fatal("missing file accepted")
	}
}
