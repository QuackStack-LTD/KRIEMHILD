package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/history"
	"time"
)

// Gathering, provenance, coverage ledger and immutable file blobs commit together.
func (s *Store) GatherArchive(ctx context.Context, worldID string, req history.GatherRequest) (*history.Document, history.GatherResult, error) {
	tx, err := s.beginHistory(ctx, false)
	if err != nil {
		return nil, history.GatherResult{}, err
	}
	defer tx.Rollback()
	d, err := readHistory(ctx, tx, worldID)
	if err != nil {
		return nil, history.GatherResult{}, err
	}
	if d.World.Revision != req.Revision {
		return nil, history.GatherResult{}, fmt.Errorf("World changed; reload before gathering")
	}
	generated, result, err := d.GatherDocuments(req)
	if err != nil {
		return nil, result, err
	}
	if req.Preview || len(generated) == 0 {
		return d, result, nil
	}
	for _, g := range generated {
		f := d.LibraryFile(req.AgeID, g.Entity.ID)
		if f != nil {
			if req.ReviseID != f.ID {
				return nil, result, fmt.Errorf("document identity collision")
			}
			// An explicit revision changes the file, not the author's edited entity properties.
			g.Entity = *d.Entity(req.AgeID, f.ID)
			g.Record.CreationYear = f.Record.CreationYear
		}
		if err = d.Apply(history.Command{Kind: "entity", AgeID: req.AgeID, Entity: &g.Entity}); err != nil {
			return nil, result, err
		}
		f = d.LibraryFile(req.AgeID, g.Entity.ID)
		if f == nil {
			d.LibraryFiles = append(d.LibraryFiles, history.LibraryFile{ID: g.Entity.ID, AgeID: req.AgeID, ArchiveID: req.ArchiveID, Status: "present"})
			f = &d.LibraryFiles[len(d.LibraryFiles)-1]
		}
		h := hash(g.Data)
		f.Record = &g.Record
		versionRecord := g.Record
		f.Versions = append(f.Versions, history.LibraryVersion{Hash: h, Name: g.Name, MIME: g.MIME, Size: int64(len(g.Data)), CreatedAt: time.Now().UnixMilli(), Provenance: &versionRecord})
		if _, err = tx.ExecContext(ctx, "INSERT INTO history_blobs(hash,data) VALUES($1,$2) ON CONFLICT(hash) DO NOTHING", h, g.Data); err != nil {
			return nil, result, err
		}
		a := d.Entity(req.AgeID, req.ArchiveID).Archive
		alreadyCovered := false
		for _, coverage := range a.Coverage {
			if coverage.Key == g.Record.CoverageKey {
				alreadyCovered = true
				break
			}
		}
		if !alreadyCovered {
			a.Coverage = append(a.Coverage, history.ArchiveCoverage{Key: g.Record.CoverageKey, DocumentID: g.Entity.ID, SourceAge: g.Record.SourceAge})
		}
	}
	if err = writeHistory(ctx, tx, d, req.Revision, false); err != nil {
		return nil, result, err
	}
	return d, result, tx.Commit()
}

type ArchiveCustodyRequest struct {
	Revision    int    `json:"revision"`
	AgeID       string `json:"ageId"`
	DocumentID  string `json:"documentId"`
	Destination string `json:"destination"`
	Action      string `json:"action"`
	Year        int64  `json:"year"`
	Reason      string `json:"reason"`
}

func (s *Store) ArchiveCustody(ctx context.Context, world string, req ArchiveCustodyRequest) (*history.Document, error) {
	return s.UpdateHistory(ctx, world, req.Revision, func(d *history.Document) error {
		f := d.LibraryFile(req.AgeID, req.DocumentID)
		if f == nil {
			return fmt.Errorf("document not found")
		}
		if req.Reason == "" || req.Year < -9007199254740991 || req.Year > 9007199254740991 {
			return fmt.Errorf("supply a historical year and reason")
		}
		if f.Record != nil && f.Record.RecordingAge == req.AgeID && req.Year < f.Record.CreationYear {
			return fmt.Errorf("custody cannot precede document creation")
		}
		for _, c := range f.Custody {
			if c.AgeID == req.AgeID && req.Year < c.Year {
				return fmt.Errorf("custody changes must follow the document's recorded history")
			}
		}
		if f.Status == "destroyed" {
			return fmt.Errorf("a destroyed physical document cannot be recovered or copied")
		}
		from := f.ArchiveID
		to := req.Destination
		switch req.Action {
		case "copy", "transfer", "confiscate", "inherit":
			if f.Status == "lost" {
				return fmt.Errorf("recover the lost document before using it")
			}
			a := d.Entity(req.AgeID, to)
			if a == nil || a.Deleted || a.Kind != "archive" || to == from {
				return fmt.Errorf("select a different destination archive")
			}
			if req.Action == "copy" {
				// New physical identity, same covered information and immutable bytes.
				raw, _ := json.Marshal(f)
				var copy history.LibraryFile
				_ = json.Unmarshal(raw, &copy)
				original := f.ID
				copy.ID = history.ID()
				copy.CopyOf = original
				copy.Custody = nil
				e := *d.Entity(req.AgeID, original)
				e.ID = copy.ID
				e.ContinuityID = copy.ID
				e.Name = "Copy: " + e.Name
				if len([]rune(e.Name)) > 150 {
					e.Name = string([]rune(e.Name)[:150])
				}
				if err := d.Apply(history.Command{Kind: "entity", AgeID: req.AgeID, Entity: &e}); err != nil {
					return err
				}
				d.LibraryFiles = append(d.LibraryFiles, copy)
				f = &d.LibraryFiles[len(d.LibraryFiles)-1]
			}
			f.ArchiveID = to
			f.Status = "present"
			a = d.Entity(req.AgeID, to)
			if a.Archive != nil && f.Record != nil {
				a.Archive.Coverage = append(a.Archive.Coverage, history.ArchiveCoverage{Key: f.Record.CoverageKey, DocumentID: f.ID, SourceAge: f.Record.SourceAge})
			}
		case "lose", "destroy":
			to = from
			f.Status = "lost"
			if req.Action == "destroy" {
				f.Status = "destroyed"
			}
		case "recover":
			if f.Status != "lost" {
				return fmt.Errorf("only a lost document can be recovered")
			}
			to = from
			f.Status = "present"
		default:
			return fmt.Errorf("invalid custody action")
		}
		f.Custody = append(f.Custody, history.CustodyEvent{Action: req.Action, AgeID: req.AgeID, Year: req.Year, From: from, To: to, Reason: req.Reason})
		return nil
	})
}
