package project

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type failedArchiveWriter struct{}

func (failedArchiveWriter) Write(p []byte) (int, error) {
	return 0, errors.New("simulated full backup disk")
}

func TestStreamedArchiveRoundTripAndFailures(t *testing.T) {
	s, st := fixture(t)
	st = put(t, s, st, Record{ID: NewID(), Kind: "note", Name: "Private note", Notes: "A whole-world backup retains private writing."})
	st = apply(t, s, st, Command{Action: "copy-age", Name: "Second age"})
	if e := s.WriteArchive(failedArchiveWriter{}); e == nil {
		t.Fatal("writer failure ignored")
	}
	temp, e := os.CreateTemp(t.TempDir(), "backup-*.zip")
	if e != nil {
		t.Fatal(e)
	}
	defer temp.Close()
	if e = s.WriteArchive(temp); e != nil {
		t.Fatal(e)
	}
	info, _ := temp.Stat()
	z, e := zip.NewReader(temp, info.Size())
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range z.File {
		if !archivePath(f.Name) {
			t.Fatalf("private runtime file exported: %s", f.Name)
		}
		input, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		_, e = io.Copy(io.Discard, input)
		input.Close()
		if e != nil {
			t.Fatal("bad ZIP checksum", e)
		}
	}
	library := t.TempDir()
	preview, e := PrepareImportReader(library, temp, info.Size())
	if e != nil {
		t.Fatal(e)
	}
	if e = AcceptImport(library, preview); e != nil {
		t.Fatal(e)
	}
	restored, e := Open(filepath.Join(library, preview.World.ID))
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close()
	after, e := restored.State(st.Age.ID)
	if e != nil || !reflect.DeepEqual(after.Records, st.Records) {
		t.Fatal("stream round trip changed records", e)
	}
	root, _ := os.ReadFile(filepath.Join(s.Dir, "project.head.json"))
	var snap Snapshot
	if e = s.read("snapshots", st.Age.Snapshot, &snap); e != nil {
		t.Fatal(e)
	}
	for _, hash := range snap.Records {
		os.WriteFile(filepath.Join(s.Dir, "objects", hash), []byte("corrupt"), 0600)
		break
	}
	var out bytes.Buffer
	if e = s.WriteArchive(&out); e == nil {
		t.Fatal("corruption exported")
	}
	unchanged, _ := os.ReadFile(filepath.Join(s.Dir, "project.head.json"))
	if !bytes.Equal(root, unchanged) {
		t.Fatal("backup changed canonical head")
	}
}

func TestStreamedImportRejectsTruncationAndCleansStaging(t *testing.T) {
	s, _ := fixture(t)
	data, e := s.Archive()
	if e != nil {
		t.Fatal(e)
	}
	library := t.TempDir()
	if _, e = PrepareImportReader(library, bytes.NewReader(data[:len(data)/2]), int64(len(data)/2)); e == nil {
		t.Fatal("truncated ZIP accepted")
	}
	var invalid bytes.Buffer
	z := zip.NewWriter(&invalid)
	for i := 0; i < 2; i++ {
		w, _ := z.Create("project.head.json")
		w.Write([]byte("{}"))
	}
	z.Close()
	if _, e = PrepareImportReader(library, bytes.NewReader(invalid.Bytes()), int64(invalid.Len())); e == nil {
		t.Fatal("duplicate path accepted")
	}
	staged, _ := os.ReadDir(filepath.Join(library, ".imports"))
	if len(staged) != 0 {
		t.Fatal("failed preview left staging data")
	}
}
