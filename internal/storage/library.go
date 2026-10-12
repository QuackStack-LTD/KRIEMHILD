package storage

import (
	"context"
	"database/sql"
	"fmt"
	"kriemhild/internal/history"
	"time"
)

func (s *Store) SaveLibraryFile(ctx context.Context, worldID string, revision int, e history.Entity, archive, name, mime string, data []byte) (*history.Document, error) {
	if len(data) > history.MaxLibraryFileSize {
		return nil, fmt.Errorf("file exceeds 64 MiB")
	}
	tx, err := s.beginHistory(ctx, false)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d, err := readHistory(ctx, tx, worldID)
	if err != nil {
		return nil, err
	}
	if revision != d.World.Revision {
		return nil, fmt.Errorf("World changed; reload before retrying")
	}
	if history.Registry.Types[e.Kind].Category != "records" || e.Kind == "archive" {
		return nil, fmt.Errorf("choose an in-world document type")
	}
	if err = d.Apply(history.Command{Kind: "entity", AgeID: e.AgeID, Entity: &e}); err != nil {
		return nil, err
	}
	f := d.LibraryFile(e.AgeID, e.ID)
	h := hash(data)
	if f != nil && (f.Status == "lost" || f.Status == "destroyed") && (len(f.Versions) == 0 || f.Versions[len(f.Versions)-1].Hash != h) {
		return nil, fmt.Errorf("the contents of lost or destroyed documents cannot be edited")
	}
	if f != nil && f.Record != nil && f.ArchiveID != archive {
		return nil, fmt.Errorf("use Preservation & custody to transfer a historical document")
	}
	if f == nil {
		d.LibraryFiles = append(d.LibraryFiles, history.LibraryFile{ID: e.ID, AgeID: e.AgeID})
		f = &d.LibraryFiles[len(d.LibraryFiles)-1]
	}
	f.ArchiveID = archive
	var provenance *history.ArchiveRecord
	if f.Record != nil {
		copy := *f.Record
		copy.AuthoredEdits = copy.AuthoredEdits || len(f.Versions) == 0 || f.Versions[len(f.Versions)-1].Hash != h
		f.Record = &copy
		versionCopy := copy
		provenance = &versionCopy
	}
	f.Versions = append(f.Versions, history.LibraryVersion{Hash: h, Name: name, MIME: mime, Size: int64(len(data)), CreatedAt: time.Now().UnixMilli(), Provenance: provenance})
	if _, err = tx.ExecContext(ctx, "INSERT INTO history_blobs(hash,data) VALUES($1,$2) ON CONFLICT(hash) DO NOTHING", h, data); err != nil {
		return nil, err
	}
	if err = writeHistory(ctx, tx, d, revision, false); err != nil {
		return nil, err
	}
	return d, tx.Commit()
}
func (s *Store) LibraryContent(ctx context.Context, worldID, age, id, version string) ([]byte, history.LibraryVersion, error) {
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return nil, history.LibraryVersion{}, err
	}
	defer tx.Rollback()
	d, err := readHistory(ctx, tx, worldID)
	if err != nil {
		return nil, history.LibraryVersion{}, err
	}
	f := d.LibraryFile(age, id)
	if f == nil || len(f.Versions) == 0 {
		return nil, history.LibraryVersion{}, sql.ErrNoRows
	}
	v := f.Versions[len(f.Versions)-1]
	if version != "" {
		found := false
		for _, candidate := range f.Versions {
			if candidate.Hash == version {
				v = candidate
				found = true
				break
			}
		}
		if !found {
			return nil, v, sql.ErrNoRows
		}
	}
	var data []byte
	if err = tx.QueryRowContext(ctx, "SELECT data FROM history_blobs WHERE hash=$1", v.Hash).Scan(&data); err != nil {
		return nil, v, err
	}
	return data, v, tx.Commit()
}
