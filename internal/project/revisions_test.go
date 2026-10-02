package project

import (
	"errors"
	"testing"
)

func TestRestoreRevisionPreservesInterveningHistoryAndChecksExpectedHead(t *testing.T) {
	s, st := fixture(t)
	city := Record{ID: NewID(), Kind: "entity", Type: "Settlement", Name: "Founding harbor"}
	st = put(t, s, st, city)
	first := st
	city.Name = "Later harbor"
	st = put(t, s, st, city)
	st = apply(t, s, st, Command{Action: "copy-age", Name: "Later Age"})
	later := st
	page, e := s.Revisions("")
	if e != nil || len(page.Items) < 4 || page.Items[0].Revision != later.Revision {
		t.Fatal("revision list", e, page)
	}
	preview, e := s.RestoreRevision(RestoreRequest{Expected: st.Revision, Revision: first.Revision})
	if e != nil || preview.Applied || len(preview.Changes) != 2 {
		t.Fatal("restore preview", e, preview)
	}
	unchanged, _ := s.State(st.Age.ID)
	if unchanged.Revision != later.Revision {
		t.Fatal("preview changed active state")
	}
	if _, e = s.RestoreRevision(RestoreRequest{Expected: first.Revision, Revision: first.Revision, Apply: true}); !errors.Is(e, ErrConflict) {
		t.Fatal("stale restoration accepted", e)
	}
	restored, e := s.RestoreRevision(RestoreRequest{Expected: later.Revision, Revision: first.Revision, Apply: true})
	if e != nil || !restored.Applied {
		t.Fatal(e)
	}
	now, e := s.State(first.Age.ID)
	if e != nil || len(now.Root.Ages) != 1 || now.Records[city.ID].Name != "Founding harbor" || now.Root.Parent != later.Revision {
		t.Fatal("restore contents/ancestry", e)
	}
	if _, e = s.RestoreRevision(RestoreRequest{Expected: restored.Revision, Revision: later.Revision, Apply: true}); e != nil {
		t.Fatal("intervening revision was lost", e)
	}
	recovered, e := s.State(later.Age.ID)
	if e != nil || len(recovered.Root.Ages) != 2 || recovered.Records[city.ID].Name != "Later harbor" {
		t.Fatal("could not recover intervening work", e)
	}
	unrelated := recovered.Root
	unrelated.Parent = ""
	unrelated.MergeParents = nil
	unrelated.Message = "Unreachable staging revision"
	hash, e := s.put("revisions", unrelated)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.RestoreRevision(RestoreRequest{Expected: recovered.Revision, Revision: hash, Apply: true}); e == nil {
		t.Fatal("unreachable revision restored")
	}
	if _, e = s.Revisions(hash); e == nil {
		t.Fatal("unreachable history cursor accepted")
	}
	dir := s.Dir
	s.Close()
	reopened, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	final, e := reopened.State(later.Age.ID)
	if e != nil || final.Revision != recovered.Revision {
		t.Fatal("restore did not survive restart", e)
	}
}
