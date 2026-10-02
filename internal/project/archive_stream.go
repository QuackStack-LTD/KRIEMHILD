package project

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// WriteArchive captures the head once and streams verified immutable objects.
// The caller must discard partial output on failure.
func (s *Store) WriteArchive(output io.Writer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return WriteArchiveDirectory(s.Dir, output)
}

func WriteArchiveDirectory(dir string, output io.Writer) error {
	s := &Store{Dir: dir}
	raw, e := os.ReadFile(filepath.Join(dir, "project.head.json"))
	if e != nil {
		return e
	}
	var head Head
	if e = json.Unmarshal(raw, &head); e != nil {
		return e
	}
	var root Root
	if e = s.read("revisions", head.Revision, &root); e != nil {
		return e
	}
	if root.Version != 1 && root.Version != Format {
		return fmt.Errorf("unsupported backup format")
	}
	z := zip.NewWriter(output)
	// Close only on success: truncated streams must not look like valid archives.
	total := int64(0)
	entries := 0
	write := func(name string, input io.Reader, size int64, hash string) error {
		entries++
		if size < 0 || size > ArchiveLimit-total || entries > 100000 {
			return fmt.Errorf("archive exceeds the 256 MiB or 100000-entry safety budget")
		}
		total += size
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0600)
		dest, e := z.CreateHeader(header)
		if e != nil {
			return e
		}
		digest := sha256.New()
		n, e := io.Copy(io.MultiWriter(dest, digest), io.LimitReader(input, size+1))
		if e != nil {
			return e
		}
		if n != size {
			return fmt.Errorf("archive source changed size")
		}
		if hash != "" && hex.EncodeToString(digest.Sum(nil)) != hash {
			return fmt.Errorf("corrupt archive object: %s", name)
		}
		return nil
	}
	for _, item := range []struct {
		name  string
		value any
	}{{"project.head.json", head}, {"kriemhild.json", root.World}} {
		b, e := json.Marshal(item.value)
		if e != nil {
			return e
		}
		if e = write(item.name, bytes.NewReader(b), int64(len(b)), ""); e != nil {
			return e
		}
	}
	for _, folder := range []string{"objects", "snapshots", "revisions", "assets"} {
		base := filepath.Join(dir, folder)
		info, e := os.Lstat(base)
		if e != nil {
			return e
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive source is not a regular directory")
		}
		files, e := os.ReadDir(base)
		if e != nil {
			return e
		}
		for _, entry := range files {
			if !validHash(entry.Name()) {
				continue
			}
			path := filepath.Join(base, entry.Name())
			info, e := os.Lstat(path)
			if e != nil {
				return e
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("archive object is not a regular file")
			}
			file, e := os.Open(path)
			if e != nil {
				return e
			}
			current, e := file.Stat()
			if e == nil && !os.SameFile(info, current) {
				e = fmt.Errorf("archive source changed identity")
			}
			if e == nil {
				e = write(folder+"/"+entry.Name(), file, info.Size(), entry.Name())
			}
			closeErr := file.Close()
			if e != nil {
				return e
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	return z.Close()
}
