package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/history"
	"kriemhild/internal/resourcesurvey"
	"strconv"
	"strings"
	"time"
)

// Geography is optional: chart/document reads never load terrain snapshots,
// species, populations, climate arrays or generated habitat profiles.
func readResourceArchives(ctx context.Context, q historyQuery, world, age string, geography bool) (*history.Document, error) {
	d := &history.Document{}
	var err error
	if d.World, err = readViewWorld(ctx, q, world); err != nil {
		return nil, err
	}
	if d.Ages, err = readViewRows[history.Age](ctx, q, "ages", "world_id=$1", world); err != nil {
		return nil, err
	}
	if d.Age(age) == nil {
		return nil, fmt.Errorf("Age not found")
	}
	where := "world_id=$1 AND age_id=$2"
	args := []any{world, age}
	condition := where + " AND kind='archive'"
	if geography {
		condition = where + " AND kind<>'lifeform'"
	}
	if d.Entities, err = readViewRows[history.Entity](ctx, q, "entities", condition, args...); err != nil {
		return nil, err
	}
	if d.LibraryFiles, err = readViewRows[history.LibraryFile](ctx, q, "library_files", where, args...); err != nil {
		return nil, err
	}
	if d.Terrains, err = readViewRows[history.Terrain](ctx, q, "terrains", where, args...); err != nil {
		return nil, err
	}
	if d.Maps, err = readViewRows[history.Map](ctx, q, "maps", where, args...); err != nil {
		return nil, err
	}
	if geography {
		if d.Representations, err = readViewRows[history.Representation](ctx, q, "representations", where, args...); err != nil {
			return nil, err
		}
		if d.Relations, err = readViewRows[history.Relation](ctx, q, "relations", where, args...); err != nil {
			return nil, err
		}
	}
	return d, nil
}
func (s *Store) ResourceArchives(ctx context.Context, world, age string, geography bool) (*history.Document, error) {
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d, err := readResourceArchives(ctx, tx, world, age, geography)
	if err != nil {
		return nil, err
	}
	return d, tx.Commit()
}
func ResourceArchiveDescendants(d *history.Document, age, archive string) map[string]bool {
	ids := map[string]bool{archive: true}
	for changed := true; changed; {
		changed = false
		for _, e := range d.Entities {
			if e.AgeID == age && !e.Deleted && e.Kind == "archive" && !ids[e.ID] && ids[history.ArchiveParent(e)] {
				ids[e.ID] = true
				changed = true
			}
		}
	}
	return ids
}

type SavedResourceSurvey struct {
	ID        string                  `json:"id"`
	ArchiveID string                  `json:"archiveId"`
	Survey    history.ResourceSurvey  `json:"survey"`
	Manifest  resourcesurvey.Manifest `json:"-"`
}

func retainedResourceFiles(ctx context.Context, q historyQuery, d *history.Document, age, archive string, selection ...*history.ResourceSurvey) ([]SavedResourceSurvey, error) {
	a := d.Entity(age, archive)
	if a == nil || a.Deleted || a.Kind != "archive" {
		return nil, fmt.Errorf("Archive not found")
	}
	descendants := ResourceArchiveDescendants(d, age, archive)
	out := []SavedResourceSurvey{}
	for _, f := range d.LibraryFiles {
		if f.AgeID != age || !descendants[f.ArchiveID] || f.ResourceSurvey == nil || f.Status == "lost" || f.Status == "destroyed" || len(f.Versions) == 0 {
			continue
		}
		if len(selection) > 0 && selection[0] != nil {
			a, b := selection[0], f.ResourceSurvey
			if a.TerrainID != b.TerrainID || resourcesurvey.Distance(a.Circle.Longitude, a.Circle.Latitude, b.Circle.Longitude, b.Circle.Latitude, a.Circle.PlanetRadiusKM) > a.Circle.RadiusKM+b.Circle.RadiusKM {
				continue
			}
		}
		// A deleted record can temporarily remain in imported/legacy file lists.
		var raw []byte
		if err := q.QueryRowContext(ctx, "SELECT data FROM history_entities WHERE world_id=$1 AND id=$2", d.World.ID, age+"/"+f.ID).Scan(&raw); err != nil {
			return nil, err
		}
		var e history.Entity
		if json.Unmarshal(raw, &e) != nil || e.Deleted {
			continue
		}
		var manifestRaw []byte
		if err := q.QueryRowContext(ctx, "SELECT data FROM history_blobs WHERE hash=$1", f.ResourceSurvey.ManifestHash).Scan(&manifestRaw); err != nil {
			return nil, err
		}
		var m resourcesurvey.Manifest
		if resourcesurvey.Hash(manifestRaw) != f.ResourceSurvey.ManifestHash || json.Unmarshal(manifestRaw, &m) != nil || m.Version != resourcesurvey.Version {
			return nil, fmt.Errorf("Invalid saved resource manifest")
		}
		out = append(out, SavedResourceSurvey{f.ID, f.ArchiveID, *f.ResourceSurvey, m})
	}
	return out, nil
}
func (s *Store) ResourceSurveys(ctx context.Context, world, age, archive string, selectedID ...string) (*history.Document, []SavedResourceSurvey, map[string][]byte, error) {
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return nil, nil, nil, err
	}
	defer tx.Rollback()
	d, err := readResourceArchives(ctx, tx, world, age, false)
	if err != nil {
		return nil, nil, nil, err
	}
	var selection *history.ResourceSurvey
	if len(selectedID) > 0 {
		for _, f := range d.LibraryFiles {
			if f.ID == selectedID[0] {
				selection = f.ResourceSurvey
				break
			}
		}
		if selection == nil {
			return nil, nil, nil, fmt.Errorf("Resource report not found")
		}
	}
	files, err := retainedResourceFiles(ctx, tx, d, age, archive, selection)
	if err != nil {
		return nil, nil, nil, err
	}
	blobs := map[string][]byte{}
	wanted := map[string]bool{}
	var queryCells []resourcesurvey.Cell
	if selection != nil && !(len(selectedID) > 1 && selectedID[1] == "original") {
		queryCells = resourcesurvey.Cells(selection.Circle, resourcesurvey.Level(selection.Circle))
	}
	for _, f := range files {
		if selection == nil {
			for _, h := range f.Manifest.References() {
				wanted[h] = true
			}
			continue
		}
		if f.ID == selectedID[0] && len(selectedID) > 1 && selectedID[1] == "original" {
			wanted[f.Manifest.Report] = true
			if f.Manifest.ReportBiosphere != "" {
				wanted[f.Manifest.ReportBiosphere] = true
			}
		}
		if len(selectedID) > 1 && selectedID[1] == "original" {
			continue
		}
		for _, cell := range queryCells {
			lon, lat := cell.Center()
			if selection.Circle.Contains(lon, lat) && f.Survey.Circle.Contains(lon, lat) {
				if h := f.Manifest.Cells[resourcesurvey.At(f.Manifest.Level, lon, lat).Key()]; h != "" {
					wanted[h] = true
				}
			}
		}
		if f.Survey.Circle.Contains(selection.Circle.Longitude, selection.Circle.Latitude) {
			if h := f.Manifest.Cells[resourcesurvey.At(f.Manifest.Level, selection.Circle.Longitude, selection.Circle.Latitude).Key()]; h != "" {
				wanted[h] = true
			}
		}
	}
	if err = loadResourceBlobs(ctx, tx, wanted, blobs); err != nil {
		return nil, nil, nil, err
	}
	resources := map[string]bool{}
	for _, raw := range blobs {
		var state resourcesurvey.State
		if json.Unmarshal(raw, &state) == nil {
			if state.Biosphere != "" {
				resources[state.Biosphere] = true
			}
			for _, h := range state.Resources {
				resources[h] = true
			}
		}
	}
	if err = loadResourceBlobs(ctx, tx, resources, blobs); err != nil {
		return nil, nil, nil, err
	}
	return d, files, blobs, tx.Commit()
}

func loadResourceBlobs(ctx context.Context, q historyQuery, wanted map[string]bool, blobs map[string][]byte) error {
	hashes := []string{}
	for h := range wanted {
		if _, ok := blobs[h]; !ok {
			hashes = append(hashes, h)
		}
	}
	for start := 0; start < len(hashes); start += 256 {
		args := []any{}
		places := []string{}
		for _, h := range hashes[start:min(start+256, len(hashes))] {
			args = append(args, h)
			places = append(places, fmt.Sprintf("$%d", len(args)))
		}
		rows, err := q.QueryContext(ctx, "SELECT hash,data FROM history_blobs WHERE hash IN ("+strings.Join(places, ",")+")", args...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var h string
			var raw []byte
			if err = rows.Scan(&h, &raw); err != nil {
				rows.Close()
				return err
			}
			if hash(raw) != h {
				rows.Close()
				return fmt.Errorf("Corrupt resource observation")
			}
			blobs[h] = raw
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	for h := range wanted {
		if _, ok := blobs[h]; !ok {
			return fmt.Errorf("Missing resource observation")
		}
	}
	return nil
}

type ResourceArchiveSave struct {
	ArchiveID    string
	Survey       history.ResourceSurvey
	Manifest     resourcesurvey.Manifest
	Blobs        map[string][]byte
	Name, Format string
	Document     []byte
}
type ResourceArchiveResult struct {
	ID                string `json:"id"`
	ArchiveID         string `json:"archiveId"`
	ResourcesFolderID string `json:"resourcesFolderId"`
	FolderID          string `json:"folderId"`
	Revision          int    `json:"revision"`
	NewCells          int    `json:"newCells"`
	ChangedCells      int    `json:"changedCells"`
	UnchangedCells    int    `json:"unchangedCells"`
}

func resourceFolderMarker(e history.Entity) (kind, source string) {
	var p struct {
		ResourceFolder string `json:"resourceFolder"`
		SourceAge      string `json:"sourceAge"`
	}
	_ = json.Unmarshal(e.Properties, &p)
	return p.ResourceFolder, p.SourceAge
}
func (s *Store) SaveResourceSurvey(ctx context.Context, world, age string, save ResourceArchiveSave) (ResourceArchiveResult, error) {
	out := ResourceArchiveResult{}
	if save.Format != "md" && save.Format != "txt" {
		return out, fmt.Errorf("Choose Markdown or text for the archive report")
	}
	if len(save.Document) > history.MaxLibraryFileSize || len(save.Document) == 0 || len(save.Name) > 200 {
		return out, fmt.Errorf("Invalid resource report document")
	}
	if err := save.Manifest.Validate(save.Blobs); err != nil {
		return out, err
	}
	manifestRaw, _ := json.Marshal(save.Manifest)
	save.Survey.ManifestHash = hash(manifestRaw)
	tx, err := s.beginHistory(ctx, false)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	d, err := readResourceArchives(ctx, tx, world, age, false)
	if err != nil {
		return out, err
	}
	if save.Survey.SourceAge != age {
		return out, fmt.Errorf("Record resources in the observed Age")
	}
	if err = save.Survey.Validate(d, age); err != nil {
		return out, err
	}
	archive := d.Entity(age, save.ArchiveID)
	if archive == nil || archive.Deleted || archive.Kind != "archive" {
		return out, fmt.Errorf("Archive not found")
	}
	if kind, _ := resourceFolderMarker(*archive); kind != "" {
		return out, fmt.Errorf("Choose the owning archive, rather than a resource report folder")
	}
	existing, err := retainedResourceFiles(ctx, tx, d, age, save.ArchiveID, &save.Survey)
	if err != nil {
		return out, err
	}
	// Compare by historical date, not insertion order. Backdated reports never
	// become the current observation merely because they were entered last.
	obs := []resourcesurvey.Observation{}
	for _, f := range existing {
		if f.Survey.SourceAge == age && f.Survey.TerrainID == save.Survey.TerrainID {
			obs = append(obs, resourcesurvey.Observation{ID: f.ID, Circle: f.Survey.Circle, Year: f.Survey.Year, RecordedAt: f.Survey.RecordedAt, Manifest: f.Manifest})
		}
	}
	resolved := resourcesurvey.Resolve(save.Survey.Circle, obs, save.Survey.Year)
	prior := map[string]string{}
	for _, r := range resolved {
		lon, lat := r.Cell.Center()
		prior[resourcesurvey.At(save.Manifest.Level, lon, lat).Key()] = r.Hash
	}
	for _, cell := range resourcesurvey.Cells(save.Survey.Circle, save.Manifest.Level) {
		lon, lat := cell.Center()
		if !save.Survey.Circle.Contains(lon, lat) {
			continue
		}
		h := save.Manifest.Cells[cell.Key()]
		old := prior[cell.Key()]
		if old == "" {
			out.NewCells++
		} else if old == h {
			out.UnchangedCells++
		} else {
			out.ChangedCells++
		}
	}
	additions := &history.Document{World: d.World}
	now := time.Now().UnixMilli()
	start := len(d.Entities)
	out.ResourcesFolderID, out.FolderID, err = resourceSurveyFolders(d, age, save.ArchiveID, save.Survey.SourceAge, save.Survey.Year)
	if err != nil {
		return out, err
	}
	additions.Entities = append(additions.Entities, d.Entities[start:]...)
	out.ID = history.ID()
	out.ArchiveID = save.ArchiveID
	props, _ := json.Marshal(map[string]any{"parentArchive": out.FolderID, "resourceReport": true, "terrainId": save.Survey.TerrainID, "resourceCircle": save.Survey.Circle})
	e := history.Entity{ID: out.ID, AgeID: age, Kind: "document", Name: save.Name, TerrainID: save.Survey.TerrainID, HomeMap: save.Survey.MapID, Properties: props, CreatedAt: now, UpdatedAt: now}
	file := history.LibraryFile{ID: e.ID, AgeID: age, ArchiveID: out.FolderID, ResourceSurvey: &save.Survey, Versions: []history.LibraryVersion{{Hash: hash(save.Document), Name: save.Name + "." + save.Format, MIME: "text/plain; charset=utf-8", Size: int64(len(save.Document)), CreatedAt: now}}}
	if save.Format == "md" {
		file.Versions[0].MIME = "text/markdown; charset=utf-8"
	}
	additions.Entities = append(additions.Entities, e)
	additions.LibraryFiles = []history.LibraryFile{file}
	save.Blobs[save.Survey.ManifestHash] = manifestRaw
	save.Blobs[hash(save.Document)] = save.Document
	for h, raw := range save.Blobs {
		if hash(raw) != h || len(raw) > history.MaxLibraryFileSize {
			return out, fmt.Errorf("Invalid resource data checksum or size")
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO history_blobs(hash,data) VALUES($1,$2) ON CONFLICT(hash) DO NOTHING", h, raw); err != nil {
			return out, err
		}
	}
	rows := map[string]map[string]json.RawMessage{"entities": {}, "library_files": {}}
	for _, e := range additions.Entities {
		raw, _ := json.Marshal(e)
		rows["entities"][age+"/"+e.ID] = raw
	}
	raw, _ := json.Marshal(file)
	rows["library_files"][age+"/"+file.ID] = raw
	if err = writeRecordPatch(ctx, tx, additions, map[string]map[string]json.RawMessage{}, rows); err != nil {
		return out, err
	}
	out.Revision = additions.World.Revision
	return out, tx.Commit()
}

func resourceSurveyFolders(d *history.Document, age, archive, sourceAge string, year *int64) (string, string, error) {
	a := d.Entity(age, archive)
	if a == nil || a.Deleted || a.Kind != "archive" || d.Age(sourceAge) == nil {
		return "", "", fmt.Errorf("Archive or source Age not found")
	}
	if kind, _ := resourceFolderMarker(*a); kind != "" {
		return "", "", fmt.Errorf("Choose the owning archive, rather than a resource report folder")
	}
	now := time.Now().UnixMilli()
	folder := func(parent, name, kind, source string) string {
		for _, e := range d.Entities {
			k, s := resourceFolderMarker(e)
			if e.AgeID == age && !e.Deleted && e.Kind == "archive" && history.ArchiveParent(e) == parent && k == kind && s == source {
				return e.ID
			}
		}
		props, _ := json.Marshal(map[string]string{"parentArchive": parent, "resourceFolder": kind, "sourceAge": source})
		e := history.Entity{ID: history.ID(), AgeID: age, Kind: "archive", Name: name, Properties: props, CreatedAt: now, UpdatedAt: now}
		d.Entities = append(d.Entities, e)
		return e.ID
	}
	root := folder(archive, "Resources", "resources", "")
	ageFolder := folder(root, d.Age(sourceAge).Name, "age", sourceAge)
	yearName, yearKey := "Undated", "undated"
	if year != nil {
		yearKey = strconv.FormatInt(*year, 10)
		yearName = "Year " + yearKey
	}
	return root, folder(ageFolder, yearName, "year", sourceAge+"/"+yearKey), nil
}

func collectResourceSurveyBlobs(ctx context.Context, q historyQuery, d *history.Document, blobs map[string][]byte) error {
	for _, f := range d.LibraryFiles {
		if f.ResourceSurvey == nil {
			continue
		}
		h := f.ResourceSurvey.ManifestHash
		if blobs[h] == nil {
			var raw []byte
			if err := q.QueryRowContext(ctx, "SELECT data FROM history_blobs WHERE hash=$1", h).Scan(&raw); err != nil {
				return err
			}
			blobs[h] = raw
		}
		var m resourcesurvey.Manifest
		if json.Unmarshal(blobs[h], &m) != nil {
			return fmt.Errorf("Invalid resource manifest")
		}
		for _, h := range m.References() {
			if blobs[h] != nil {
				continue
			}
			var raw []byte
			if err := q.QueryRowContext(ctx, "SELECT data FROM history_blobs WHERE hash=$1", h).Scan(&raw); err != nil {
				return err
			}
			blobs[h] = raw
		}
	}
	return nil
}
func validateResourceSurveyBlobs(d *history.Document, blobs map[string][]byte) error {
	for _, f := range d.LibraryFiles {
		if f.ResourceSurvey == nil {
			continue
		}
		h := f.ResourceSurvey.ManifestHash
		raw, ok := blobs[h]
		if !ok || hash(raw) != h {
			return fmt.Errorf("Missing or corrupt resource manifest")
		}
		var m resourcesurvey.Manifest
		if json.Unmarshal(raw, &m) != nil {
			return fmt.Errorf("Invalid resource manifest")
		}
		if err := m.Validate(blobs); err != nil {
			return err
		}
	}
	return nil
}
