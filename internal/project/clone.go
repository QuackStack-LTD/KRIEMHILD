package project

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func validateRemote(remote string) error {
	u, e := url.Parse(remote)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(remote, "\r\n\x00") {
		return fmt.Errorf("use an HTTPS repository URL without credentials; configure authentication with your Git credential helper")
	}
	return nil
}

// Clone downloads Git objects without checking out remote-controlled paths.
// A bounded archive is validated and materialized before the library can see it.
func PrepareClone(library, remote string) (ImportPreview, error) {
	if e := validateRemote(remote); e != nil {
		return ImportPreview{}, e
	}
	return prepareClone(library, func(base, target string) error {
		runner := &Store{Dir: base}
		_, e := runner.git("clone", "--no-checkout", "--no-local", "--single-branch", "--branch", "main", "--", remote, target)
		return e
	})
}

func prepareClone(library string, clone func(base, target string) error) (result ImportPreview, err error) {
	result.Token = NewID()
	base := filepath.Join(library, ".imports", result.Token)
	target := filepath.Join(base, "project")
	if e := os.MkdirAll(base, 0700); e != nil {
		return result, e
	}
	defer func() {
		if err != nil {
			_ = DiscardImport(library, result)
		}
	}()
	if e := clone(base, target); e != nil {
		return result, e
	}
	staged := &Store{Dir: target}
	archive, e := staged.git("archive", "--format=zip", "HEAD")
	if e != nil {
		return result, e
	}
	z, e := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if e != nil {
		return result, e
	}
	if len(z.File) > 100000 {
		return result, fmt.Errorf("repository has too many files")
	}
	seen := map[string]bool{}
	total := uint64(0)
	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if (!archivePath(f.Name) && f.Name != ".gitignore") || seen[f.Name] || !f.Mode().IsRegular() {
			return result, fmt.Errorf("repository has unsafe or unexpected files")
		}
		seen[f.Name] = true
		if f.UncompressedSize64 > ArchiveLimit || total+f.UncompressedSize64 > ArchiveLimit {
			return result, fmt.Errorf("repository working files exceed the 256 MiB clone budget")
		}
		total += f.UncompressedSize64
		in, e := f.Open()
		if e != nil {
			return result, e
		}
		data, e := io.ReadAll(io.LimitReader(in, ArchiveLimit+1))
		in.Close()
		if e != nil || len(data) > ArchiveLimit {
			return result, fmt.Errorf("cannot read repository file")
		}
		p := filepath.Join(target, filepath.FromSlash(f.Name))
		if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			return result, e
		}
		if e = os.WriteFile(p, data, 0600); e != nil {
			return result, e
		}
	}
	for _, dir := range []string{"assets", "objects", "snapshots", "revisions", ".kriemhild"} {
		if e = os.MkdirAll(filepath.Join(target, dir), 0700); e != nil {
			return result, e
		}
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
	var marker Marker
	raw, e := os.ReadFile(filepath.Join(target, "kriemhild.json"))
	if e != nil {
		return result, e
	}
	if json.Unmarshal(raw, &marker) != nil || marker != root.World {
		return result, fmt.Errorf("repository marker does not match its project head")
	}
	if _, e = os.Lstat(filepath.Join(library, root.World.ID)); !os.IsNotExist(e) {
		return result, fmt.Errorf("this world already exists in the library; fetch and merge it instead")
	}
	// Verify all imported immutable bytes, including revisions not at the head.
	for _, dir := range []string{"assets", "objects", "snapshots", "revisions"} {
		entries, e := os.ReadDir(filepath.Join(target, dir))
		if e != nil {
			return result, e
		}
		for _, entry := range entries {
			if _, e = staged.objectBytes(dir, entry.Name()); e != nil {
				return result, e
			}
			if dir == "objects" {
				result.Records++
			}
		}
	}
	if _, e = staged.git("reset", "--mixed", "HEAD"); e != nil {
		return result, e
	}
	result.World = root.World
	result.Ages = len(root.Ages)
	result.SourceFormat = Format
	result.Directory = target
	result.Repository = true
	result.Warnings = []string{"Cloning preserves this world's identity and Git history. Existing worlds are never overwritten.", "Only main is cloned. Remote working files are validated before import; no remote hooks or checkout filters run.", "Uncommitted changes, accounts and live manuscript drafts are not carried by Git."}
	return result, nil
}
