package storage

import (
	"context"
	"encoding/json"
	"errors"
	"kriemhild/internal/history"
	"testing"
)

func writerFixture(t *testing.T) (*Store, WritingProject) {
	t.Helper()
	s, e := Open(context.Background(), t.TempDir(), "")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	p := WritingProject{ID: "project", Name: "Novel", Type: "novel"}
	if e = s.SaveWriterProject(context.Background(), &p); e != nil {
		t.Fatal(e)
	}
	return s, p
}
func testWriterNode(id, parent string) WritingNode {
	return WritingNode{ID: id, ProjectID: "project", ParentID: parent, Type: "scene", Title: id, Schema: 1, Status: "Drafting", Content: plainWriterContent("First words.")}
}
func TestWriterMigrationRevisionAndRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, e := Open(ctx, dir, "")
	if e != nil {
		t.Fatal(e)
	}
	d := WritingDocument{ID: "old", Name: "Old draft", Text: "Preserve me"}
	if e = s.SaveWriting(ctx, &d); e != nil {
		t.Fatal(e)
	}
	d.Text = "Latest draft"
	if e = s.SaveWriting(ctx, &d); e != nil {
		t.Fatal(e)
	}
	s.Close()
	s, e = Open(ctx, dir, "")
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	b, e := s.WriterBundle(ctx, "legacy-loose-drafts")
	if e != nil || b.Project.WorldID != "" || len(b.Nodes) != 1 || b.Nodes[0].Text != d.Text || b.Nodes[0].Revision != d.Revision {
		t.Fatalf("migration %#v %v", b, e)
	}
	n := b.Nodes[0]
	stale := n
	n.Content = plainWriterContent("New editor")
	if e = s.SaveWriterNode(ctx, b.Project.ID, &n, "Manual"); e != nil {
		t.Fatal(e)
	}
	if e = s.SaveWriterNode(ctx, b.Project.ID, &stale, ""); !errors.Is(e, ErrWritingConflict) {
		t.Fatalf("stale save accepted: %v", e)
	}
	var old string
	if e = s.db.QueryRow(`SELECT body FROM writing_documents WHERE id='old'`).Scan(&old); e != nil || old != d.Text {
		t.Fatal("legacy source erased")
	}
	if e = s.migrateHistory(ctx); e != nil {
		t.Fatal(e)
	}
	b, _ = s.WriterBundle(ctx, b.Project.ID)
	if len(b.Nodes) != 1 || len(b.Versions) != 2 {
		t.Fatal("migration reran")
	}
}
func TestWriterHierarchySnapshotsAndPortableBackup(t *testing.T) {
	ctx := context.Background()
	s, p := writerFixture(t)
	parent := testWriterNode("chapter", "")
	parent.Type = "chapter"
	child := testWriterNode("scene", "chapter")
	for _, n := range []*WritingNode{&parent, &child} {
		if e := s.SaveWriterNode(ctx, p.ID, n, ""); e != nil {
			t.Fatal(e)
		}
	}
	bad := parent
	bad.ParentID = child.ID
	if e := s.SaveWriterNode(ctx, p.ID, &bad, ""); e == nil {
		t.Fatal("cycle accepted")
	}
	bad = parent
	bad.Deleted = true
	if e := s.SaveWriterNode(ctx, p.ID, &bad, ""); e == nil {
		t.Fatal("orphaning deletion accepted")
	}
	first := child
	child.Content = plainWriterContent("Later words with a changed ending.")
	if e := s.SaveWriterNode(ctx, p.ID, &child, "Ending revision"); e != nil {
		t.Fatal(e)
	}
	restored := first
	restored.Revision = child.Revision
	if e := s.SaveWriterNode(ctx, p.ID, &restored, "Restore original"); e != nil {
		t.Fatal(e)
	}
	b, e := s.WriterBundle(ctx, p.ID)
	if e != nil || len(b.Versions) != 4 {
		t.Fatalf("snapshots %v", e)
	}
	raw, _ := json.Marshal(b)
	var portable WriterBundle
	json.Unmarshal(raw, &portable)
	other, _ := writerFixture(t)
	imported, e := other.ImportWriter(ctx, portable, "")
	if e != nil {
		t.Fatal(e)
	}
	result, e := other.WriterBundle(ctx, imported.ID)
	if e != nil || len(result.Nodes) != 2 || len(result.Versions) != 4 {
		t.Fatal("backup loss", e)
	}
	for _, n := range result.Nodes {
		if n.ID == "scene" || n.ProjectID != imported.ID {
			t.Fatal("identities not remapped")
		}
		if n.Type == "scene" && n.ParentID == "chapter" {
			t.Fatal("parent reference not remapped")
		}
	}
	restored.Deleted = true
	if e = s.SaveWriterNode(ctx, p.ID, &restored, "Deleted"); e != nil {
		t.Fatal(e)
	}
	parent.Deleted = true
	if e = s.SaveWriterNode(ctx, p.ID, &parent, "Deleted"); e != nil {
		t.Fatal(e)
	}
	b, _ = s.WriterBundle(ctx, p.ID)
	if _, e = other.ImportWriter(ctx, b, ""); e != nil {
		t.Fatal("deleted history not portable", e)
	}
}
func TestWriterWorldReferenceValidationAndDerivedText(t *testing.T) {
	ctx := context.Background()
	s, _ := writerFixture(t)
	d := history.New("Reference world")
	age := d.World.CurrentAge
	e := history.Entity{ID: "hero", Name: "Hero", Kind: "character"}
	if err := d.Apply(history.Command{Kind: "entity", AgeID: age, Entity: &e}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	p := WritingProject{ID: "linked", Name: "Linked novel", WorldID: d.World.ID}
	if err := s.SaveWriterProject(ctx, &p); err != nil {
		t.Fatal(err)
	}
	n := testWriterNode("linked-scene", "")
	n.ProjectID = p.ID
	n.StoryAge = age
	n.POV = "hero"
	n.References = []WritingReference{{WorldID: d.World.ID, EntityID: "hero", AgeID: age, Type: "metadata"}}
	n.Text = "forged"
	if err := s.SaveWriterNode(ctx, p.ID, &n, ""); err != nil {
		t.Fatal(err)
	}
	if n.Text == "forged" {
		t.Fatal("plain text not derived")
	}
	bad := n
	bad.References = []WritingReference{{WorldID: "other", EntityID: "hero", AgeID: age, Type: "metadata"}}
	if err := s.SaveWriterNode(ctx, p.ID, &bad, ""); err == nil {
		t.Fatal("cross world accepted")
	}
	bad = n
	bad.ParentID = "chapter-in-other-project"
	if err := s.SaveWriterNode(ctx, p.ID, &bad, ""); err == nil {
		t.Fatal("cross project parent accepted")
	}
	bad = n
	bad.Content = json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"unsafe","marks":[{"type":"link","attrs":{"href":"javascript:alert(1)"}}]}]}]}`)
	if err := s.SaveWriterNode(ctx, p.ID, &bad, ""); err == nil {
		t.Fatal("unsafe link accepted")
	}
	b, _ := s.WriterBundle(ctx, p.ID)
	if _, err := s.ImportWriter(ctx, b, "different-world"); err == nil {
		t.Fatal("backup silently relinked")
	}
}
