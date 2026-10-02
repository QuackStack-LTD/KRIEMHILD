package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestClonePreviewPreservesIdentityWithoutCheckingOutUnsafeFiles(t *testing.T) {
	if _, e := exec.LookPath("git"); e != nil {
		t.Skip("git unavailable")
	}
	source, st := fixture(t)
	st = put(t, source, st, Record{ID: NewID(), Kind: "entity", Type: "Place", Name: "Harbor"})
	for _, c := range []GitCommand{{Action: "init"}, {Action: "commit", Message: "Authored world"}} {
		if e := source.GitAction(c); e != nil {
			t.Fatal(e)
		}
	}
	clone := func(base, target string) error {
		cmd := exec.Command("git", "-C", base, "-c", "protocol.file.allow=always", "clone", "--no-checkout", "--no-hardlinks", source.Dir, target)
		out, e := cmd.CombinedOutput()
		if e != nil {
			t.Log(string(out))
		}
		return e
	}
	library := t.TempDir()
	preview, e := prepareClone(library, clone)
	if e != nil {
		t.Fatal(e)
	}
	if !preview.Repository || preview.World.ID != st.Root.World.ID || preview.Ages != 1 {
		t.Fatal("clone identity changed")
	}
	if _, e = os.Stat(filepath.Join(library, preview.World.ID)); !os.IsNotExist(e) {
		t.Fatal("preview exposed project before acceptance")
	}
	if e = AcceptImport(library, preview); e != nil {
		t.Fatal(e)
	}
	imported, e := Open(filepath.Join(library, preview.World.ID))
	if e != nil {
		t.Fatal(e)
	}
	defer imported.Close()
	restored, e := imported.State(st.Age.ID)
	if e != nil || restored.Revision != st.Revision {
		t.Fatal("clone rewrote saved history", e)
	}
	status, e := imported.GitStatus()
	if e != nil || !status.Initialized || status.Changes != "" {
		t.Fatal("clone Git tree is not clean", e, status)
	}
	if _, e = prepareClone(library, clone); e == nil {
		t.Fatal("duplicate world clone accepted")
	}
	stages, e := os.ReadDir(filepath.Join(library, ".imports"))
	if e != nil {
		t.Fatal(e)
	}
	// The accepted preview's empty token directory remains until the server
	// clears it; a rejected clone must not leave another staging directory.
	if len(stages) != 1 || stages[0].Name() != preview.Token {
		t.Fatal("failed clone left staging files")
	}
	if e = os.WriteFile(filepath.Join(source.Dir, "untrusted.txt"), []byte("unexpected checkout file"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = source.git("add", "--", "untrusted.txt"); e != nil {
		t.Fatal(e)
	}
	if _, e = source.git("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "Unexpected file"); e != nil {
		t.Fatal(e)
	}
	unsafeLibrary := t.TempDir()
	if _, e = prepareClone(unsafeLibrary, clone); e == nil {
		t.Fatal("unexpected repository file imported")
	}
	stages, e = os.ReadDir(filepath.Join(unsafeLibrary, ".imports"))
	if e != nil || len(stages) != 0 {
		t.Fatal("unsafe clone not discarded", e)
	}
	for _, remote := range []string{"file:///tmp/world", "http://example.test/world", "https://secret@example.test/world", "https://example.test/world?token=secret", "ext::command", "https://example.test/world\n"} {
		if _, e = PrepareClone(t.TempDir(), remote); e == nil {
			t.Fatal("unsafe remote accepted", remote)
		}
	}
}
