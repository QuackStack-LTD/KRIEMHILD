package history

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

const MaxLibraryFileSize = 64 << 20

// File versions point to immutable, content-addressed blobs, shared across Ages.
type LibraryVersion struct {
	Hash      string `json:"hash"`
	Name      string `json:"name"`
	MIME      string `json:"mime"`
	Size      int64  `json:"size"`
	CreatedAt int64  `json:"createdAt"`
}
type LibraryFile struct {
	ID        string           `json:"id"`
	AgeID     string           `json:"ageId"`
	ArchiveID string           `json:"archiveId"`
	Versions  []LibraryVersion `json:"versions"`
}

func (d *Document) LibraryFile(age, id string) *LibraryFile {
	for i := range d.LibraryFiles {
		f := &d.LibraryFiles[i]
		if f.AgeID == age && f.ID == id {
			return f
		}
	}
	return nil
}
func ArchiveParent(e Entity) string {
	var p struct {
		ParentArchive string `json:"parentArchive"`
	}
	_ = json.Unmarshal(e.Properties, &p)
	return p.ParentArchive
}
func (d *Document) validateLibrary() error {
	seen := map[string]bool{}
	for _, f := range d.LibraryFiles {
		key := f.AgeID + "/" + f.ID
		e := d.Entity(f.AgeID, f.ID)
		a := d.Entity(f.AgeID, f.ArchiveID)
		if seen[key] || d.Age(f.AgeID) == nil || e == nil || e.Deleted || Registry.Types[e.Kind].Category != "records" || e.Kind == "archive" || a == nil || a.Deleted || a.Kind != "archive" || len(f.Versions) == 0 {
			return fmt.Errorf("invalid Library file or archive reference")
		}
		seen[key] = true
		for _, v := range f.Versions {
			h, err := hex.DecodeString(v.Hash)
			if err != nil || len(h) != 32 || strings.TrimSpace(v.Name) == "" || len(v.Name) > 255 || strings.ContainsAny(v.Name, "/\\\x00\r\n") || v.Size < 0 || v.Size > MaxLibraryFileSize || len(v.MIME) > 160 {
				return fmt.Errorf("invalid Library file version")
			}
		}
	}
	for _, e := range d.Entities {
		if e.Deleted || e.Kind != "archive" {
			continue
		}
		visited := map[string]bool{e.ID: true}
		parent := ArchiveParent(e)
		for parent != "" {
			p := d.Entity(e.AgeID, parent)
			if p == nil || p.Deleted || p.Kind != "archive" || visited[parent] {
				return fmt.Errorf("invalid or cyclic archive folder")
			}
			visited[parent] = true
			parent = ArchiveParent(*p)
		}
	}
	return nil
}
func (d *Document) inheritLibrary(from, to string, overwrite bool) {
	for _, f := range append([]LibraryFile(nil), d.LibraryFiles...) {
		if f.AgeID != from {
			continue
		}
		f.AgeID = to
		f.Versions = append([]LibraryVersion(nil), f.Versions...)
		if old := d.LibraryFile(to, f.ID); old == nil {
			d.LibraryFiles = append(d.LibraryFiles, f)
		} else if overwrite {
			*old = f
		}
	}
}
func (d *Document) pruneLibrary(age string) {
	// Excluding an archive during inheritance also excludes its contents in
	// that new Age. Ordinary deletion rejects nonempty folders before this runs.
	changed := true
	for changed {
		changed = false
		for i := range d.Entities {
			e := &d.Entities[i]
			if e.AgeID != age || e.Deleted || e.Kind != "archive" || ArchiveParent(*e) == "" {
				continue
			}
			parent := d.Entity(age, ArchiveParent(*e))
			if parent == nil || parent.Deleted {
				e.Deleted = true
				changed = true
			}
		}
	}
	for _, f := range d.LibraryFiles {
		if f.AgeID != age {
			continue
		}
		a := d.Entity(age, f.ArchiveID)
		if a == nil || a.Deleted {
			if e := d.Entity(age, f.ID); e != nil {
				e.Deleted = true
			}
		}
	}
	d.LibraryFiles = slices.DeleteFunc(d.LibraryFiles, func(f LibraryFile) bool {
		e := d.Entity(f.AgeID, f.ID)
		return f.AgeID == age && (e == nil || e.Deleted)
	})
}
func (d *Document) validateArchiveDeletion(age string, ids []string) error {
	for _, f := range d.LibraryFiles {
		if f.AgeID == age && slices.Contains(ids, f.ArchiveID) && !slices.Contains(ids, f.ID) {
			return fmt.Errorf("move or delete the archive's documents first")
		}
	}
	for _, e := range d.Entities {
		if e.AgeID == age && !e.Deleted && e.Kind == "archive" && slices.Contains(ids, ArchiveParent(e)) && !slices.Contains(ids, e.ID) {
			return fmt.Errorf("move or delete the archive's subfolders first")
		}
	}
	return nil
}
