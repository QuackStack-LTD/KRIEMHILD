package project

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const ArchiveLimit = 256 << 20

type ImportPreview struct {
	Repository   bool     `json:"repository,omitempty"`
	Token        string   `json:"token"`
	World        Marker   `json:"world"`
	Ages         int      `json:"ages"`
	Records      int      `json:"records"`
	SourceFormat int      `json:"sourceFormat"`
	Warnings     []string `json:"warnings"`
	Directory    string   `json:"-"`
}

func (s *Store) Archive() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return ArchiveDirectory(s.Dir)
}

// ArchiveDirectory can copy a running format-1 or format-2 project read-only.
// The head is captured once; immutable files referenced by it cannot drift.
func ArchiveDirectory(dir string) ([]byte, error) {
	var output bytes.Buffer
	if e := WriteArchiveDirectory(dir, &output); e != nil {
		return nil, e
	}
	return output.Bytes(), nil
}
func archivePath(name string) bool {
	if name == "kriemhild.json" || name == "project.head.json" {
		return true
	}
	parts := strings.Split(name, "/")
	return len(parts) == 2 && (parts[0] == "objects" || parts[0] == "snapshots" || parts[0] == "revisions" || parts[0] == "assets") && validHash(parts[1])
}

// Preview imports into a new staging directory. Both legacy and current input
// are copied through a hash-preserving graph reader; source bytes are untouched.
func PrepareImport(library string, data []byte) (result ImportPreview, err error) {
	return PrepareImportReader(library, bytes.NewReader(data), int64(len(data)))
}

// PrepareImportReader extracts a seekable archive without retaining its complete
// compressed or expanded contents in memory. The caller owns the source file.
func PrepareImportReader(library string, input io.ReaderAt, size int64) (result ImportPreview, err error) {
	if size < 0 || size > ArchiveLimit {
		return result, fmt.Errorf("archive too large")
	}
	z, e := zip.NewReader(input, size)
	if e != nil {
		return result, e
	}
	if len(z.File) > 100000 {
		return result, fmt.Errorf("archive has too many entries")
	}
	token := NewID()
	base := filepath.Join(library, ".imports", token)
	source := filepath.Join(base, "source")
	target := filepath.Join(base, "project")
	if e = os.MkdirAll(source, 0700); e != nil {
		return result, e
	}
	defer func() {
		if err != nil {
			DiscardImport(library, ImportPreview{Token: token})
		}
	}()
	seen := map[string]bool{}
	total := int64(0)
	for _, f := range z.File {
		if !archivePath(f.Name) || seen[f.Name] || !f.Mode().IsRegular() {
			return result, fmt.Errorf("unsafe or duplicate archive path")
		}
		seen[f.Name] = true
		if f.UncompressedSize64 > ArchiveLimit {
			return result, fmt.Errorf("archive entry too large")
		}
		total += int64(f.UncompressedSize64)
		if total > ArchiveLimit {
			return result, fmt.Errorf("expanded archive exceeds 256 MiB")
		}
		in, e := f.Open()
		if e != nil {
			return result, e
		}
		path := filepath.Join(source, filepath.FromSlash(f.Name))
		if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			in.Close()
			return result, e
		}
		file, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			in.Close()
			return result, e
		}
		n, copyErr := io.Copy(file, io.LimitReader(in, int64(f.UncompressedSize64)+1))
		in.Close()
		closeErr := file.Close()
		if copyErr != nil || n != int64(f.UncompressedSize64) {
			return result, fmt.Errorf("cannot safely read archive entry: %s", f.Name)
		}
		if closeErr != nil {
			return result, closeErr
		}
	}
	var head Head
	b, e := os.ReadFile(filepath.Join(source, "project.head.json"))
	if e != nil {
		return result, e
	}
	if e = json.Unmarshal(b, &head); e != nil {
		return result, e
	}
	old := &Store{Dir: source}
	var oldRoot Root
	if e = old.read("revisions", head.Revision, &oldRoot); e != nil {
		return result, e
	}
	if oldRoot.Version != 1 && oldRoot.Version != Format {
		return result, fmt.Errorf("unsupported import format %d", oldRoot.Version)
	}
	for _, dir := range []string{"objects", "snapshots", "revisions", "assets", ".kriemhild"} {
		if e = os.MkdirAll(filepath.Join(target, dir), 0700); e != nil {
			return result, e
		}
	}
	migrated := &Store{Dir: target}
	marker := Marker{Version: Format, ID: NewID(), Name: oldRoot.World.Name}
	maps := map[string]string{}
	active := map[string]bool{}
	nodes := 0
	var record func(string) (string, error)
	var snapshot func(string) (string, error)
	var revision func(string) (string, error)
	guard := func(key string) error {
		nodes++
		if nodes > 200000 || len(active) > 1000 {
			return fmt.Errorf("migration graph exceeds safety budget")
		}
		if active[key] {
			return fmt.Errorf("cyclic object graph")
		}
		active[key] = true
		return nil
	}
	record = func(hash string) (string, error) {
		key := "o" + hash
		if mapped := maps[key]; mapped != "" {
			return mapped, nil
		}
		if e := guard(key); e != nil {
			return "", e
		}
		defer delete(active, key)
		var r Record
		if e := old.read("objects", hash, &r); e != nil {
			return "", e
		}
		if r.SettingSnapshot != "" {
			id, e := snapshot(r.SettingSnapshot)
			if e != nil {
				return "", e
			}
			r.SettingSnapshot = id
		}
		if r.Chronology != nil {
			id, e := snapshot(r.Chronology.Baseline)
			if e != nil {
				return "", e
			}
			r.Chronology.Baseline = id
		}
		if r.Asset != "" {
			b, e := old.Asset(r.Asset)
			if e != nil {
				return "", e
			}
			if _, e = migrated.putBytes("assets", b); e != nil {
				return "", e
			}
		}
		if r.Event != nil {
			for i, ch := range r.Event.Changes {
				for _, pair := range []struct {
					source string
					dest   *string
				}{{ch.Before, &r.Event.Changes[i].Before}, {ch.After, &r.Event.Changes[i].After}} {
					if pair.source != "" {
						h, e := record(pair.source)
						if e != nil {
							return "", e
						}
						*pair.dest = h
					}
				}
			}
		}
		h, e := migrated.put("objects", r)
		maps[key] = h
		return h, e
	}
	snapshot = func(hash string) (string, error) {
		key := "s" + hash
		if mapped := maps[key]; mapped != "" {
			return mapped, nil
		}
		if e := guard(key); e != nil {
			return "", e
		}
		defer delete(active, key)
		var v Snapshot
		if e := old.read("snapshots", hash, &v); e != nil {
			return "", e
		}
		if (v.Version != 1 && v.Version != Format) || v.Records == nil {
			return "", fmt.Errorf("unsupported snapshot")
		}
		v.Version = Format
		for id, hash := range v.Records {
			h, e := record(hash)
			if e != nil {
				return "", e
			}
			v.Records[id] = h
		}
		h, e := migrated.put("snapshots", v)
		maps[key] = h
		return h, e
	}
	revision = func(hash string) (string, error) {
		if hash == "" {
			return "", nil
		}
		key := "r" + hash
		if mapped := maps[key]; mapped != "" {
			return mapped, nil
		}
		if e := guard(key); e != nil {
			return "", e
		}
		defer delete(active, key)
		var v Root
		if e := old.read("revisions", hash, &v); e != nil {
			return "", e
		}
		if v.Version != 1 && v.Version != Format {
			return "", fmt.Errorf("unsupported historical revision")
		}
		v.Version = Format
		v.World.ID = marker.ID
		v.World.Version = Format
		parent, e := revision(v.Parent)
		if e != nil {
			return "", e
		}
		v.Parent = parent
		for i, id := range v.MergeParents {
			v.MergeParents[i], e = revision(id)
			if e != nil {
				return "", e
			}
		}
		for id, a := range v.Ages {
			a.Snapshot, e = snapshot(a.Snapshot)
			if e != nil {
				return "", e
			}
			if a.SourceSnapshot != "" {
				a.SourceSnapshot, e = snapshot(a.SourceSnapshot)
				if e != nil {
					return "", e
				}
			}
			a.SourceRevision, e = revision(a.SourceRevision)
			if e != nil {
				return "", e
			}
			for i, h := range a.Undo {
				a.Undo[i], e = snapshot(h)
				if e != nil {
					return "", e
				}
			}
			for i, h := range a.Redo {
				a.Redo[i], e = snapshot(h)
				if e != nil {
					return "", e
				}
			}
			v.Ages[id] = a
		}
		h, e := migrated.put("revisions", v)
		maps[key] = h
		return h, e
	}
	newHead, e := revision(head.Revision)
	if e != nil {
		return result, e
	}
	b, _ = json.Marshal(Head{newHead})
	if e = atomicWrite(filepath.Join(target, "project.head.json"), b); e != nil {
		return result, e
	}
	b, _ = json.Marshal(marker)
	if e = atomicWrite(filepath.Join(target, "kriemhild.json"), b); e != nil {
		return result, e
	}
	verified, e := Open(target)
	if e != nil {
		return result, e
	}
	_, root, e := verified.Root()
	verified.Close()
	if e != nil {
		return result, e
	}
	result = ImportPreview{Token: token, World: marker, Ages: len(root.Ages), SourceFormat: oldRoot.Version, Warnings: []string{"Imported as a separate world with a new world ID; Age and entity identities are preserved.", "No credentials, repository configuration or executable files are imported."}, Directory: target}
	if oldRoot.Version == 1 {
		result.Warnings = append(result.Warnings, "Format 1 snapshots, scene pins and revision links are migrated to format 2 in this copy.")
	}
	for key := range maps {
		if strings.HasPrefix(key, "o") {
			result.Records++
		}
	}
	return result, nil
}
func AcceptImport(library string, p ImportPreview) error {
	expected := filepath.Join(library, ".imports", p.Token, "project")
	if !validID(p.Token) || !validID(p.World.ID) || filepath.Clean(p.Directory) != filepath.Clean(expected) {
		return fmt.Errorf("invalid import token")
	}
	target := filepath.Join(library, p.World.ID)
	if _, e := os.Stat(target); !os.IsNotExist(e) {
		return fmt.Errorf("target already exists")
	}
	if e := os.Rename(expected, target); e != nil {
		return e
	}
	return nil
}

// Only a server-created staging token may be discarded. Resolve and verify the
// absolute containment before recursive cleanup on Windows as well as Unix.
func DiscardImport(library string, p ImportPreview) error {
	if !validID(p.Token) {
		return fmt.Errorf("invalid import token")
	}
	root, e := filepath.Abs(filepath.Join(library, ".imports"))
	if e != nil {
		return e
	}
	target, e := filepath.Abs(filepath.Join(root, p.Token))
	if e != nil {
		return e
	}
	if filepath.Dir(target) != root || filepath.Base(target) != p.Token {
		return fmt.Errorf("invalid import staging path")
	}
	return os.RemoveAll(target)
}
