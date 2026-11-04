package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"kriemhild/internal/history"
	"kriemhild/internal/world"
	"strings"
	"time"
)

type IllustratedMapRequest struct {
	MapID          string
	AgeID          string
	FocusID        string
	SourceRevision int
	Illustration   world.Illustration
	PNG            []byte
	BannerPNG      []byte
}

// The file, its folder, place link and banner commit together. Geographic shapes
// and terrain snapshots are never edited by saving an illustration.
func (s *Store) SaveIllustratedMap(ctx context.Context, worldID string, revision int, req IllustratedMapRequest) (*history.Document, string, string, error) {
	a := req.Illustration
	if err := a.Validate(); err != nil {
		return nil, "", "", err
	}
	if strings.TrimSpace(a.Title) == "" || len(a.Title) > 160 || len(req.PNG)+len(req.BannerPNG) > history.MaxLibraryFileSize {
		return nil, "", "", fmt.Errorf("name the illustration using up to 160 characters and keep its image under 64 MiB")
	}
	config, err := png.DecodeConfig(bytes.NewReader(req.PNG))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 8192 || config.Height > 8192 || config.Width*config.Height > 16777216 {
		return nil, "", "", fmt.Errorf("choose a valid PNG illustration within 8192 pixels per side and 16 megapixels")
	}
	if _, err = png.Decode(bytes.NewReader(req.PNG)); err != nil || req.SourceRevision < 0 {
		return nil, "", "", fmt.Errorf("the illustration image or its source revision is invalid")
	}
	var banner *history.LibraryImage
	if len(req.BannerPNG) > 0 {
		c, e := png.DecodeConfig(bytes.NewReader(req.BannerPNG))
		if e != nil || len(req.BannerPNG) > history.MaxLibraryFileSize || c.Width < 1 || c.Height < 1 || c.Width > 8192 || c.Height > 8192 || c.Width*c.Height > 16777216 {
			return nil, "", "", fmt.Errorf("invalid banner image dimensions")
		}
		if _, e = png.Decode(bytes.NewReader(req.BannerPNG)); e != nil {
			return nil, "", "", fmt.Errorf("invalid banner PNG")
		}
		banner = &history.LibraryImage{Hash: hash(req.BannerPNG), Size: int64(len(req.BannerPNG)), Width: c.Width, Height: c.Height}
	}
	tx, err := s.beginHistory(ctx, false)
	if err != nil {
		return nil, "", "", err
	}
	defer tx.Rollback()
	d, err := readHistory(ctx, tx, worldID)
	if err != nil {
		return nil, "", "", err
	}
	if revision != d.World.Revision {
		return nil, "", "", fmt.Errorf("World changed; reopen the map before saving the illustration")
	}
	m := d.Map(req.AgeID, req.MapID)
	if m == nil {
		return nil, "", "", fmt.Errorf("the source map no longer exists in this Age")
	}
	var focus *history.Entity
	if req.FocusID != "" {
		focus = d.Entity(req.AgeID, req.FocusID)
		if focus == nil || focus.Deleted || focus.TerrainID != m.TerrainID {
			return nil, "", "", fmt.Errorf("the selected place is no longer available on this terrain")
		}
	}
	// IDs depend on map/config identity, so repeat saves create file versions.
	fileID := hash([]byte("illustrated-map/" + m.ID + "/" + a.ID))[:32]
	existing := d.LibraryFile(req.AgeID, fileID)
	if existing != nil && (existing.Status == "lost" || existing.Status == "destroyed") {
		return nil, "", "", fmt.Errorf("this saved map is marked lost or destroyed; choose a new illustration")
	}
	folder := func(id, name, parent, owner string) string {
		if d.Entity(req.AgeID, id) == nil {
			props, _ := json.Marshal(map[string]any{"parentArchive": parent, "illustratedMapInbox": owner == "", "illustratedMapOwner": owner, "summary": "Illustrated maps saved from the map workspace."})
			d.Entities = append(d.Entities, history.Entity{ID: id, AgeID: req.AgeID, Kind: "archive", Name: name, Properties: props})
		}
		return id
	}
	inboxID := hash([]byte("illustrated-maps/inbox/" + worldID))[:32]
	folder(inboxID, "Illustrated maps", "", "")
	archiveID := inboxID
	if focus != nil {
		archiveID = folder(hash([]byte("illustrated-maps/place/" + worldID + "/" + focus.ID))[:32], focus.Name+" maps", inboxID, focus.ID)
	}
	if existing != nil {
		archiveID = existing.ArchiveID
	} // Respect a user's later filing choice.
	properties, _ := json.Marshal(map[string]any{"summary": "An illustrated map of " + a.Title, "illustration": map[string]any{"sourceMapId": m.ID, "focusId": req.FocusID, "sourceRevision": req.SourceRevision, "design": a}})
	e := d.Entity(req.AgeID, fileID)
	if e == nil {
		d.Entities = append(d.Entities, history.Entity{ID: fileID, AgeID: req.AgeID, Name: a.Title, Kind: "illustrated-map", Properties: properties})
	} else {
		e.Name = a.Title
		e.Properties = properties
	}
	if existing == nil {
		d.LibraryFiles = append(d.LibraryFiles, history.LibraryFile{ID: fileID, AgeID: req.AgeID, ArchiveID: archiveID})
		existing = &d.LibraryFiles[len(d.LibraryFiles)-1]
	}
	name := strings.NewReplacer("/", "-", "\\", "-", "\r", "", "\n", "").Replace(a.Title) + ".png"
	h := hash(req.PNG)
	if len(existing.Versions) == 0 || existing.Versions[len(existing.Versions)-1].Hash != h {
		existing.Versions = append(existing.Versions, history.LibraryVersion{Hash: h, Name: name, MIME: "image/png", Size: int64(len(req.PNG)), CreatedAt: time.Now().UnixMilli(), Banner: banner})
	} else if banner != nil {
		existing.Versions[len(existing.Versions)-1].Banner = banner
	}
	if banner != nil {
		if _, err = tx.ExecContext(ctx, "INSERT INTO history_blobs(hash,data) VALUES($1,$2) ON CONFLICT(hash) DO NOTHING", banner.Hash, req.BannerPNG); err != nil {
			return nil, "", "", err
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO history_blobs(hash,data) VALUES($1,$2) ON CONFLICT(hash) DO NOTHING", h, req.PNG); err != nil {
		return nil, "", "", err
	}
	if focus != nil {
		// Appending folders/documents may have relocated the entities slice.
		focus = d.Entity(req.AgeID, req.FocusID)
		p := map[string]json.RawMessage{}
		json.Unmarshal(focus.Properties, &p)
		if p == nil {
			p = map[string]json.RawMessage{}
		}
		p["illustratedMap"], _ = json.Marshal(map[string]any{"fileId": fileID, "version": h, "title": a.Title, "sourceMapId": m.ID})
		focus.Properties, _ = json.Marshal(p)
	}
	if err = writeHistory(ctx, tx, d, revision, false); err != nil {
		return nil, "", "", err
	}
	if err = tx.Commit(); err != nil {
		return nil, "", "", err
	}
	return d, fileID, archiveID, nil
}
