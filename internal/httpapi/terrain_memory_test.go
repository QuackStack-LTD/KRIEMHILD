package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/biosphere"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	terrainmodel "kriemhild/internal/terrain/model"
	"kriemhild/internal/world"
	"net/http/httptest"
	"testing"
)

func biosphereDetailFixture(t testing.TB) (*Server, *storage.Store, *session) {
	t.Helper()
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	v := projectFixture(t)
	s.sessions["terrain"] = v
	d := history.New("Detail performance")
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	autoRequest(t, s, "POST", "/api/worlds/"+d.World.ID+"/accept", fmt.Sprintf(`{"session":"terrain","ageId":%q,"name":"Living terrain"}`, d.World.CurrentAge))
	d, _ = db.History(ctx, d.World.ID)
	r := httptest.NewRequest("GET", "/", nil)
	state, err := s.biosphereTerrain(r, d, d.World.CurrentAge, d.Terrains[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	state.Settings.Richness = "diverse"
	p, err := biosphere.Generate(state, state.Settings, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, o := range p.State.Occurrences {
		ids = append(ids, o.ID)
	}
	_, err = db.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(d *history.Document) error { return d.AcceptBiosphere(p, ids, nil, nil) })
	if err != nil {
		t.Fatal(err)
	}
	return s, db, v
}

func BenchmarkBiosphereTerrainDetails(b *testing.B) {
	s, _, _ := biosphereDetailFixture(b)
	inspect := func(b *testing.B) {
		r := httptest.NewRecorder()
		s.ServeHTTP(r, httptest.NewRequest("GET", "/api/sessions/terrain/terrain/resources?longitude=0&latitude=0&radiusKm=50", nil))
		if r.Code != 200 {
			b.Fatal(r.Code, r.Body.String())
		}
	}
	b.Run("cold", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			s.ecologyCache = nil
			s.ecologySourceKey = ""
			inspect(b)
		}
	})
	b.Run("cached", func(b *testing.B) {
		inspect(b)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			inspect(b)
		}
	})
}

func TestEcologyCacheInvalidatesNamesAgesAndStaleness(t *testing.T) {
	s, db, v := biosphereDetailFixture(t)
	r := httptest.NewRequest("GET", "/", nil)
	age, terrain := v.historical.AgeID, v.historical.TerrainID
	cells, err := s.ecologySpots(r, v.historyWorld, age, terrain, true)
	if err != nil {
		t.Fatal(err)
	}
	if s.ecologyCache == nil || !s.ecologyCache.sourceVerified || s.ecologySourceKey != "" {
		t.Fatal("current provenance rebuilt the full terrain or was not cached")
	}
	first := s.ecologyCache
	if _, err = s.ecologySpots(r, v.historyWorld, age, terrain, true); err != nil || s.ecologyCache != first {
		t.Fatal("unchanged inspection rebuilt ecology")
	}
	if ecologyCacheBytes(cells) > ecologyCacheBudget {
		t.Fatal("ecology cache exceeded its byte limit")
	}
	cell, id := -1, ""
	for i, spot := range cells {
		if len(spot.Lifeforms) > 0 {
			cell, id = i, spot.Lifeforms[0].ID
			break
		}
	}
	if cell < 0 {
		t.Fatal("fixture has no accepted populations")
	}
	d, _ := db.History(context.Background(), v.historyWorld)
	d, err = db.UpdateHistory(context.Background(), d.World.ID, d.World.Revision, func(d *history.Document) error { d.Entity(age, id).Name = "Renamed canopy species"; return nil })
	if err != nil {
		t.Fatal(err)
	}
	cells, err = s.ecologySpots(r, v.historyWorld, age, terrain, true)
	if err != nil {
		t.Fatal(err)
	}
	findName := func(cells map[int]spotBiosphere) string {
		for _, e := range cells[cell].Lifeforms {
			if e.ID == id {
				return e.Name
			}
		}
		return ""
	}
	if findName(cells) != "Renamed canopy species" || s.ecologyCache == first {
		t.Fatal("inspection served a stale canonical species name")
	}
	d, err = db.UpdateHistory(context.Background(), d.World.ID, d.World.Revision, func(d *history.Document) error {
		return d.Apply(history.Command{Kind: "age", AgeID: age, Mode: "branch", Name: "Later"})
	})
	if err != nil {
		t.Fatal(err)
	}
	later := d.World.CurrentAge
	d, err = db.UpdateHistory(context.Background(), d.World.ID, d.World.Revision, func(d *history.Document) error {
		d.Entity(later, id).Name = "Later adaptation"
		d.Biosphere(later, terrain).Stale = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	laterCells, err := s.ecologySpots(r, v.historyWorld, later, terrain, true)
	if err != nil {
		t.Fatal(err)
	}
	if findName(laterCells) != "Later adaptation" || !laterCells[cell].Stale {
		t.Fatal("new Age names or staleness lost")
	}
	oldCells, err := s.ecologySpots(r, v.historyWorld, age, terrain, true)
	if err != nil {
		t.Fatal(err)
	}
	if findName(oldCells) != "Renamed canopy species" || oldCells[cell].Stale {
		t.Fatal("new Age contaminated historical inspection")
	}
	// Older saved worlds without the provenance hint still verify against the
	// immutable snapshot, once, and remain usable without regenerating anything.
	d, err = db.UpdateHistory(context.Background(), d.World.ID, d.World.Revision, func(d *history.Document) error { d.Biosphere(age, terrain).SourceSnapshot = ""; return nil })
	if err != nil {
		t.Fatal(err)
	}
	oldCells, err = s.ecologySpots(r, v.historyWorld, age, terrain, true)
	if err != nil {
		t.Fatal(err)
	}
	if oldCells[cell].Stale || s.ecologySourceKey == "" {
		t.Fatal("legacy ecological provenance was not checked")
	}
}

func TestCompactTerrainStateKeepsEditorMetadataWithoutEnvironmentalArrays(t *testing.T) {
	v := projectFixture(t)
	v.authored().Header.Name = "Named terrain"
	v.planet = &terrainmodel.Planet{Spec: terrainmodel.DefaultSpec()}
	compact := sessionStateView(v, true)
	if _, ok := compact["environment"]; ok {
		t.Fatal("terrain opening copied the legacy environmental arrays")
	}
	if _, ok := compact["dom"]; ok {
		t.Fatal("continuous editor received legacy solver pixels")
	}
	header, ok := compact["project"].(world.Header)
	if compact["W"] != v.solver.W || !ok || header.Name != "Named terrain" {
		t.Fatal("compact terrain state lost editor metadata")
	}
	full := sessionState(v)
	small, _ := json.Marshal(compact)
	large, _ := json.Marshal(full)
	if len(small)*10 > len(large) {
		t.Fatalf("terrain state still carries environmental bulk: compact=%d full=%d", len(small), len(large))
	}
	v.planet = nil
	if _, ok := sessionStateView(v, true)["dom"]; !ok {
		t.Fatal("legacy archive lost its original pixel view")
	}
}

func TestEcologyCacheSurvivesMapSavesButRevalidatesHabitatEdits(t *testing.T) {
	s, _, v := biosphereDetailFixture(t)
	r := httptest.NewRequest("GET", "/", nil)
	if _, err := s.ecologySpots(r, v.historyWorld, v.historical.AgeID, v.historical.TerrainID, true); err != nil {
		t.Fatal(err)
	}
	cached := s.ecologyCache
	if err := s.applyHistoricalMap(context.Background(), v); err != nil {
		t.Fatal(err)
	}
	v.authored().Saved()
	if err := s.saveWorld(v, false); err != nil {
		t.Fatal(err)
	}
	if s.ecologyCache != cached || cached.revision != v.historyRevision || !cached.sourceVerified {
		t.Fatal("metadata save discarded the biosphere cache")
	}
	v.world.Header.Version = 2
	_, err := v.world.AddOperation(world.Operation{ID: "new-hill", Kind: "raise", Layer: "elevation", Center: [2]float64{10, 10}, Radius: 2, Amount: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.saveWorld(v, false); err != nil {
		t.Fatal(err)
	}
	if s.ecologyCache != cached || cached.sourceVerified {
		t.Fatal("brush edit discarded populations or failed to invalidate provenance")
	}
	cells, err := s.ecologySpots(r, v.historyWorld, v.historical.AgeID, v.historical.TerrainID, true)
	if err != nil {
		t.Fatal(err)
	}
	if s.ecologyCache != cached || !cached.sourceVerified || len(cached.sourceStaleCells) == 0 {
		t.Fatal("habitat provenance was not revalidated using the cached populations")
	}
	for _, spot := range cells {
		if !spot.Stale {
			t.Fatal("changed habitats presented as current")
		}
	}
	for _, spot := range cached.cells {
		if spot.Stale {
			t.Fatal("provenance validation mutated original saved-population state")
		}
	}
}
