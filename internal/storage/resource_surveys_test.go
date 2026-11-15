package storage

import (
	"context"
	"encoding/json"
	"kriemhild/internal/history"
	"kriemhild/internal/resourcesurvey"
	"kriemhild/internal/world"
	"math"
	"strings"
	"testing"
	"time"
)

func surveyStoreFixture(t *testing.T) (*Store, *history.Document, context.Context, history.Map) {
	t.Helper()
	ctx := context.Background()
	s, err := Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	user, err := s.ResolveUser(ctx, "local", "survey-editor", "Survey editor", "", true)
	if err != nil {
		t.Fatal(err)
	}
	ctx = WithActor(ctx, user.ID)
	d := history.New("Survey World")
	m, err := d.AddTerrain(d.World.CurrentAge, "Land", history.ID(), 48, 32)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"archive", "other"} {
		e := history.Entity{ID: id, Kind: "archive", Name: id}
		if err = d.Apply(history.Command{Kind: "entity", AgeID: m.AgeID, Entity: &e}); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	state := world.New(48, 32)
	state.Header.Name = "Land"
	if _, err = s.CommitHistoricalMap(ctx, m.ProjectID, map[string][]byte{"environment": []byte("saved environment")}, state); err != nil {
		t.Fatal(err)
	}
	d, err = s.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s, d, ctx, *d.Project(m.ProjectID)
}
func storedSurvey(m history.Map, area float64, year *int64, value float64) ResourceArchiveSave {
	circle := resourcesurvey.Circle{Longitude: -44.654288, Latitude: 34.081332, RadiusKM: 2 * 6371 * math.Asin(math.Sqrt(area/(4*math.Pi*6371*6371))), PlanetRadiusKM: 6371}
	blobs := map[string][]byte{}
	manifest := resourcesurvey.Manifest{Version: 1, Level: resourcesurvey.Level(circle), Cells: map[string]string{}, Resources: map[string]string{}, Report: resourcesurvey.Put(blobs, map[string]any{"resources": []any{}, "areaKm2": area})}
	for _, cell := range resourcesurvey.Cells(circle, manifest.Level) {
		manifest.Cells[cell.Key()] = resourcesurvey.Put(blobs, resourcesurvey.State{Fields: map[string]float64{"elevation": value}, Biome: "grassland"})
	}
	return ResourceArchiveSave{ArchiveID: "archive", Survey: history.ResourceSurvey{Version: 1, TerrainID: m.TerrainID, MapID: m.ID, SourceAge: m.AgeID, SourceSnapshot: m.Snapshot, Circle: circle, Year: year, RecordedAt: time.Now().UnixMilli()}, Manifest: manifest, Blobs: blobs, Name: "Resource report", Format: "md", Document: []byte("# Complete resource report\n\nObserved area and inventory.")}
}
func TestResourceSurveyFoldersDedupeAuditAndPortableWorld(t *testing.T) {
	s, d, ctx, m := surveyStoreFixture(t)
	year := int64(10)
	base := d.World.Revision
	a, err := s.SaveResourceSurvey(ctx, d.World.ID, m.AgeID, storedSurvey(m, 300, &year, 100))
	if err != nil {
		t.Fatal(err)
	}
	if a.NewCells == 0 || a.ChangedCells != 0 {
		t.Fatal("incorrect initial delta", a)
	}
	year = 12
	b, err := s.SaveResourceSurvey(ctx, d.World.ID, m.AgeID, storedSurvey(m, 400, &year, 100))
	if err != nil {
		t.Fatal(err)
	}
	if b.UnchangedCells == 0 || b.NewCells == 0 || b.ChangedCells != 0 {
		t.Fatal("larger circle duplicated overlap", b)
	}
	year = 15
	c, err := s.SaveResourceSurvey(ctx, d.World.ID, m.AgeID, storedSurvey(m, 150, &year, 200))
	if err != nil {
		t.Fatal(err)
	}
	if c.ChangedCells == 0 || c.UnchangedCells != 0 || c.NewCells != 0 {
		t.Fatal("smaller changed circle lost overlap", c)
	}
	again, err := s.SaveResourceSurvey(ctx, d.World.ID, m.AgeID, storedSurvey(m, 150, &year, 200))
	if err != nil || again.ChangedCells != 0 || again.UnchangedCells == 0 {
		t.Fatal("unchanged capture created new cell states", again, err)
	}
	if !s.MapUnchangedSince(ctx, d.World.ID, base, again.Revision) || !s.EcologyUnchangedSince(ctx, d.World.ID, base, again.Revision) {
		t.Fatal("record-only saves invalidate terrain or ecology")
	}
	full, err := s.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = full.Validate(); err != nil {
		t.Fatal("invalid World after resource save", err)
	}
	for _, f := range full.LibraryFiles {
		folder := full.Entity(m.AgeID, f.ArchiveID)
		if folder == nil || !strings.HasPrefix(folder.Name, "Year ") {
			t.Fatal("missing year folder")
		}
		ageFolder := full.Entity(m.AgeID, history.ArchiveParent(*folder))
		if ageFolder == nil || ageFolder.Name != full.Age(m.AgeID).Name {
			t.Fatal("missing Age folder")
		}
	}
	raw, err := s.RevisionSnapshot(ctx, d.World.ID, again.Revision)
	if err != nil {
		t.Fatal(err)
	}
	var restored history.Document
	if json.Unmarshal([]byte(raw), &restored) != nil || len(restored.LibraryFiles) != 4 || restored.Validate() != nil {
		t.Fatal("audit patch lost Library file groups", raw)
	}
	if _, err = s.CommitHistoricalMap(ctx, m.ProjectID, map[string][]byte{"view": []byte("new camera")}, world.New(48, 32)); err != nil {
		t.Fatal(err)
	}
	bundle, err := s.HistoryBundle(ctx, d.World.ID)
	if err != nil || bundle.Schema != 6 || bundle.Validate() != nil {
		t.Fatal("resource bundle not portable", err, bundle.Validate())
	}
	if bundle.Snapshots[m.Snapshot] == nil {
		t.Fatal("portable report lost its original terrain snapshot after a later map save")
	}
	bundle.Schema = 5
	if bundle.Validate() == nil {
		t.Fatal("old schema silently accepts resource observations")
	}
	bundle.Schema = 6
	delete(bundle.Blobs, full.LibraryFiles[0].ResourceSurvey.ManifestHash)
	if bundle.Validate() == nil {
		t.Fatal("missing structured observations accepted")
	}
	bytes, version, err := s.LibraryContent(ctx, d.World.ID, m.AgeID, a.ID, "")
	if err != nil || !strings.Contains(string(bytes), "Complete resource report") || version.MIME != "text/markdown; charset=utf-8" {
		t.Fatal("readable report missing", err)
	}
}
func TestResourceSurveyCustodyInheritanceAndArchiveIsolation(t *testing.T) {
	s, d, ctx, m := surveyStoreFixture(t)
	year := int64(10)
	a, err := s.SaveResourceSurvey(ctx, d.World.ID, m.AgeID, storedSurvey(m, 300, &year, 100))
	if err != nil {
		t.Fatal(err)
	}
	full, _, _, err := s.ResourceSurveys(ctx, d.World.ID, m.AgeID, "archive")
	if err != nil {
		t.Fatal(err)
	}
	full, err = s.ArchiveCustody(ctx, d.World.ID, ArchiveCustodyRequest{Revision: full.World.Revision, AgeID: m.AgeID, DocumentID: a.ID, Action: "lose", Year: 11, Reason: "Fire"})
	if err != nil {
		t.Fatal(err)
	}
	_, files, _, err := s.ResourceSurveys(ctx, d.World.ID, m.AgeID, "archive")
	if err != nil || len(files) != 0 {
		t.Fatal("lost report leaked into knowledge", err)
	}
	full, err = s.ArchiveCustody(ctx, d.World.ID, ArchiveCustodyRequest{Revision: full.World.Revision, AgeID: m.AgeID, DocumentID: a.ID, Action: "recover", Year: 12, Reason: "Recovered"})
	if err != nil {
		t.Fatal(err)
	}
	full, err = s.ArchiveCustody(ctx, d.World.ID, ArchiveCustodyRequest{Revision: full.World.Revision, AgeID: m.AgeID, DocumentID: a.ID, Action: "transfer", Destination: "other", Year: 13, Reason: "Transferred"})
	if err != nil {
		t.Fatal(err)
	}
	_, files, _, err = s.ResourceSurveys(ctx, d.World.ID, m.AgeID, "archive")
	if err != nil || len(files) != 0 {
		t.Fatal("former archive retained transferred report")
	}
	_, files, _, err = s.ResourceSurveys(ctx, d.World.ID, m.AgeID, "other")
	if err != nil || len(files) != 1 {
		t.Fatal("recipient did not gain resources", err)
	}
	leaf := full.Entity(m.AgeID, files[0].ArchiveID)
	if leaf == nil || leaf.Name != "Year 10" {
		t.Fatal("transferred report did not enter resource hierarchy")
	}
	full, err = s.UpdateHistory(ctx, d.World.ID, full.World.Revision, func(d *history.Document) error {
		return d.Apply(history.Command{Kind: "age", AgeID: m.AgeID, Mode: "branch", Name: "Later"})
	})
	if err != nil {
		t.Fatal(err)
	}
	_, files, _, err = s.ResourceSurveys(ctx, d.World.ID, full.World.CurrentAge, "other")
	if err != nil || len(files) != 1 || files[0].Survey.SourceAge != m.AgeID {
		t.Fatal("inheritance lost observed Age", err)
	}
	if full.LibraryFile(m.AgeID, a.ID).ResourceSurvey == full.LibraryFile(full.World.CurrentAge, a.ID).ResourceSurvey {
		t.Fatal("Ages share mutable survey provenance")
	}
}

func TestResourceSurveyUndatedAndYearZeroAreDifferentFolders(t *testing.T) {
	s, d, ctx, m := surveyStoreFixture(t)
	zero := int64(0)
	a, err := s.SaveResourceSurvey(ctx, d.World.ID, m.AgeID, storedSurvey(m, 300, nil, 1))
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.SaveResourceSurvey(ctx, d.World.ID, m.AgeID, storedSurvey(m, 300, &zero, 1))
	if err != nil {
		t.Fatal(err)
	}
	full, err := s.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if a.FolderID == b.FolderID || full.Entity(m.AgeID, a.FolderID).Name != "Undated" || full.Entity(m.AgeID, b.FolderID).Name != "Year 0" {
		t.Fatal("undated reports invented year zero")
	}
}

func TestResourceSurveySharesBiologyAndLoadsDetailsOnlyOnRequest(t *testing.T) {
	s, d, ctx, m := surveyStoreFixture(t)
	save := storedSurvey(m, 300, nil, 1)
	bio := resourcesurvey.Put(save.Blobs, map[string]any{"available": true, "lifeforms": []map[string]any{{"id": "plant", "name": "Willow", "group": "plant", "abundance": .8}}, "habitats": []any{}})
	save.Manifest.ReportBiosphere = bio
	save.Manifest.Communities = []string{bio}
	for k, h := range save.Manifest.Cells {
		var state resourcesurvey.State
		_ = json.Unmarshal(save.Blobs[h], &state)
		state.Biosphere = bio
		save.Manifest.Cells[k] = resourcesurvey.Put(save.Blobs, state)
	}
	saved, err := s.SaveResourceSurvey(ctx, d.World.ID, m.AgeID, save)
	if err != nil {
		t.Fatal(err)
	}
	_, _, blobs, err := s.ResourceSurveys(ctx, d.World.ID, m.AgeID, "archive", saved.ID, "")
	if err != nil || blobs[bio] == nil || blobs[save.Manifest.Report] != nil {
		t.Fatal("statistics loaded full original report or lost shared biology", err)
	}
	_, _, blobs, err = s.ResourceSurveys(ctx, d.World.ID, m.AgeID, "archive", saved.ID, "original")
	if err != nil || len(blobs) != 2 || blobs[bio] == nil || blobs[save.Manifest.Report] == nil {
		t.Fatal("original report loaded unrelated sample bodies", len(blobs), err)
	}
	bundle, err := s.HistoryBundle(ctx, d.World.ID)
	if err != nil || bundle.Validate() != nil || bundle.Blobs[bio] == nil {
		t.Fatal("biological observations missing from portable archive", err)
	}
}
func TestResourceSurveyReadsAndWritesDoNotDecodeBiologicalCatalogue(t *testing.T) {
	s, d, project := historyViewFixture(t)
	ctx := context.Background()
	age := d.Project(project).AgeID
	d, err := s.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(d *history.Document) error {
		e := history.Entity{ID: "archive", Kind: "archive", Name: "Archive"}
		return d.Apply(history.Command{Kind: "entity", AgeID: age, Entity: &e})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"UPDATE history_biospheres SET data='unreadable'", "UPDATE history_entities SET data='unreadable' WHERE kind='lifeform'"} {
		if _, err = s.db.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	save := storedSurvey(*d.Project(project), 300, nil, 1)
	saved, err := s.SaveResourceSurvey(ctx, d.World.ID, age, save)
	if err != nil {
		t.Fatal("report save decoded biology", err)
	}
	if _, _, _, err = s.ResourceSurveys(ctx, d.World.ID, age, "archive", saved.ID); err != nil {
		t.Fatal("review decoded biology", err)
	}
	if _, err = s.ResourceArchives(ctx, d.World.ID, age, true); err != nil {
		t.Fatal("archive location picker decoded biology", err)
	}
	if _, _, err = s.LibraryContent(ctx, d.World.ID, age, saved.ID, ""); err != nil {
		t.Fatal("report download decoded biology", err)
	}
}
