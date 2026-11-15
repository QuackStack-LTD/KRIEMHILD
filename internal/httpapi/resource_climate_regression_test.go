package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"kriemhild/internal/world"
	"math"
	"net/http/httptest"
	"testing"
)

func TestResourceHabitatRecheckWithContinuousAdapterAndAuthoredFeatures(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	v := projectFixture(t)
	// Continuous terrain's compatibility environment has physical fields and
	// climate, including duneField, but no legacy generated feature catalogue.
	v.solver.Environment.Entities = nil
	v.authored().Header.Version = 2
	s.sessions["terrain"] = v
	d := history.New("Resource regression")
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	autoRequest(t, s, "POST", "/api/worlds/"+d.World.ID+"/accept", fmt.Sprintf(`{"session":"terrain","ageId":%q,"name":"Terrain"}`, d.World.CurrentAge))
	d, _ = db.History(ctx, d.World.ID)
	r := httptest.NewRequest("GET", "/", nil)
	baseline, err := s.biosphereTerrain(r, d, v.historical.AgeID, v.historical.TerrainID)
	if err != nil {
		t.Fatal(err)
	}
	// Continuous saves persist authored records without generating the legacy
	// climate overlay. Keep this fixture lightweight by writing the same parts.
	saveAuthored := func() {
		t.Helper()
		parts := map[string][]byte{}
		for path, value := range v.world.Parts(false) {
			raw, err := compressJSON(value)
			if err != nil {
				t.Fatal(err)
			}
			parts[path] = raw
		}
		if _, err := db.CommitHistoricalMap(ctx, v.worldID, parts, v.world); err != nil {
			t.Fatal(err)
		}
		v.world.Saved()
		if err := s.applyHistoricalMap(ctx, v); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = v.world.PutEntity(world.Entity{ID: "settlement-marker", Name: "Settlement marker", RecordStatus: "draft", Kind: "label", Layer: "labels", Geometry: "point", Points: [][2]float64{{10, 10}}, Color: "#123456"}); err != nil {
		t.Fatal(err)
	}
	saveAuthored()
	// Habitat provenance predates the camera/drawing save, exactly as it does
	// when resource inspection first opens after a cold application restart.
	d, err = db.UpdateHistory(ctx, d.World.ID, v.historyRevision, func(d *history.Document) error {
		d.Biospheres = append(d.Biospheres, baseline)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.applyHistoricalMap(ctx, v); err != nil {
		t.Fatal(err)
	}
	radius := v.solver.Environment.Geography.Radius
	radius = 2 * radius * math.Asin(math.Sqrt(314/(4*math.Pi*radius*radius)))
	path := fmt.Sprintf("/api/sessions/terrain/terrain/resources?longitude=0&latitude=0&radiusKm=%.12g", radius)
	check := func(stale bool) string {
		t.Helper()
		doc, err := db.History(ctx, d.World.ID)
		if err != nil {
			t.Fatal(err)
		}
		fresh, err := s.biosphereTerrain(r, doc, v.historical.AgeID, v.historical.TerrainID)
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		s.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		var report terrainResourceReport
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &report) != nil {
			t.Fatal("resource request failed", response.Code, response.Body)
		}
		if math.Abs(report.AreaKM2-314) > .0001 {
			t.Fatal("circle area changed", report.AreaKM2)
		}
		if (fresh.Source != baseline.Source) != stale {
			t.Fatal("habitat provenance changed for a metadata-only save or ignored a terrain edit")
		}
		if report.Biosphere.Stale != stale {
			t.Fatal("resource report lost habitat validity", report.Biosphere.Stale)
		}
		return fresh.Source
	}
	check(false)
	_, err = v.world.AddOperation(world.Operation{ID: "hill", Kind: "raise", Layer: "elevation", Center: [2]float64{10, 10}, Radius: 2, Amount: 1000})
	if err != nil {
		t.Fatal(err)
	}
	saveAuthored()
	check(true)
	layers := append([]world.Layer(nil), v.world.Header.Layers...)
	for i := range layers {
		if layers[i].ID == "elevation" {
			layers[i].Visible = false
		}
	}
	if _, err = v.world.SetLayers(layers); err != nil {
		t.Fatal(err)
	}
	saveAuthored()
	check(false)
	if _, err = v.world.Undo(false); err != nil {
		t.Fatal(err)
	}
	if _, err = v.world.Undo(false); err != nil {
		t.Fatal(err)
	}
	saveAuthored()
	check(false)
}
