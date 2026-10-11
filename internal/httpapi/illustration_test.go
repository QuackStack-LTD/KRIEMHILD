package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"kriemhild/internal/world"
	"net/http/httptest"
	"reflect"
	"testing"
)

func artworkFixture() world.Artwork {
	return world.Artwork{ID: "art-test", Name: "The Marches", MapID: "map-test", SourceRevision: 0, RendererVersion: "medieval-1", Style: "medieval-white", Width: 1600, Height: 1100, Bounds: world.Bounds{Width: 47, Height: 31}, SVG: `<svg xmlns="http://www.w3.org/2000/svg" width="1600" height="1100"><defs><g id="hut"><path d="M0,0L2,3"/></g></defs><rect width="1600" height="1100" fill="#FFFFFF"/><use href="#hut"/><text>A &amp; B</text></svg>`}
}

func TestIllustrationHierarchyIncludesChildSettlementsAndCurrentNames(t *testing.T) {
	d := history.New("Atlas")
	m := history.Map{ID: "country-map", AgeID: d.World.CurrentAge, TerrainID: "terra", EntityID: "country", Bounds: world.Bounds{Width: 40, Height: 30}}
	d.Entities = []history.Entity{
		{ID: "country", AgeID: m.AgeID, TerrainID: m.TerrainID, Kind: "country", Name: "The New Kingdom"},
		{ID: "city", AgeID: m.AgeID, TerrainID: m.TerrainID, Kind: "city", Name: "Harbour"},
		{ID: "house", AgeID: m.AgeID, TerrainID: m.TerrainID, Kind: "house"},
		{ID: "old", AgeID: "previous", TerrainID: m.TerrainID, Kind: "village", Name: "Old village"},
	}
	country := world.Entity{ID: "country-shape", LogicalID: "country", Kind: "country", Name: "Stale name", Geometry: "polygon", Points: [][2]float64{{0, 0}, {30, 0}, {30, 20}, {0, 20}}}
	for _, e := range d.Entities[1:] {
		d.Representations = append(d.Representations, history.Representation{AgeID: e.AgeID, MapID: "city-detail", EntityID: e.ID, Shape: world.Entity{ID: e.ID, Kind: e.Kind, Points: [][2]float64{{10, 10}}}})
	}
	d.Relations = []history.Relation{{ID: "membership", AgeID: m.AgeID, Source: "city", Destination: "country", Kind: "part-of"}, {ID: "old-relation", AgeID: "previous"}}
	shapes, _, relations := illustrationHierarchy(d, m, []world.Entity{country})
	if len(shapes) != 2 || len(relations) != 1 {
		t.Fatalf("wrong illustration context: %#v %#v", shapes, relations)
	}
	for _, e := range shapes {
		if e.ID == country.ID && e.Name != "The New Kingdom" {
			t.Fatal("stale country label")
		}
		if e.ID == "city" && (e.LogicalID != "city" || e.Name != "Harbour") {
			t.Fatal("lost child settlement identity")
		}
	}
}

func TestArtworkValidation(t *testing.T) {
	a := artworkFixture()
	if err := a.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, svg := range []string{`<svg><script>alert(1)</script></svg>`, `<svg onload="x()"/>`, `<svg><use href="https://example.org/evil.svg"/></svg>`, `<svg><rect style="fill:url(https://x)"/></svg>`, `<svg/><svg/>`, `<!DOCTYPE svg><svg/>`, `<svg><path fill="url(https://x)"/></svg>`} {
		a.SVG = svg
		if a.Validate() == nil {
			t.Fatal("unsafe artwork accepted", svg)
		}
	}
}

func TestArtworkCompleteDataAndPersistence(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	v := projectFixture(t)
	s.sessions["art"] = v
	for i := 0; i < 2005; i++ {
		e := world.Entity{ID: fmt.Sprintf("e%d", i), Name: "House", Kind: "house", Layer: "buildings", Geometry: "point", Points: [][2]float64{{10, 10}}, Color: "#000000"}
		v.authored().Entities[e.ID] = e
	}
	v.authored().Reindex(48, 32)
	// Hidden editor layers still belong to the source map.
	for i := range v.world.Header.Layers {
		v.world.Header.Layers[i].Visible = false
	}
	data := autoRequest(t, s, "POST", "/api/sessions/art/world/illustration", `{}`)
	if len(data["entities"].([]any)) != 2005 {
		t.Fatal("export truncated to viewport inventory")
	}
	if len(data["samples"].([]any)) == 0 || !v.persisted {
		t.Fatal("source not stored")
	}
	a := artworkFixture()
	a.SourceRevision = v.world.Header.Revision
	b, _ := json.Marshal(a)
	result := autoRequest(t, s, "POST", "/api/sessions/art/world/artworks", string(b))
	raw, _ := json.Marshal(result["artwork"])
	json.Unmarshal(raw, &a)
	parts, err := db.LoadParts(ctx, v.worldID)
	if err != nil {
		t.Fatal(err)
	}
	cold, err := restoreCheckpoint(v.worldID, parts, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cold.world.Artworks[a.ID], a) {
		t.Fatal("database altered artwork")
	}
	archive, err := encodeProject(v)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := decodeProject(unzipProject(t, archive), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.world.Artworks[a.ID], a) {
		t.Fatal("ZIP altered artwork")
	}
	v.world.Header.Revision++
	request := httptest.NewRecorder()
	s.ServeHTTP(request, httptest.NewRequest("POST", "/api/sessions/art/world/artworks", bytes.NewReader(b)))
	if request.Code != 409 {
		t.Fatal("stale illustration accepted", request.Code)
	}
	if v.world.Artworks[a.ID].SVG != a.SVG {
		t.Fatal("map edit changed artwork")
	}
}

func TestArtworkHistoricalZIPAndChildScope(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	v := projectFixture(t)
	s.sessions["art"] = v
	d := history.New("Illustrated history")
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{"session": "art", "ageId": d.World.CurrentAge, "name": "Terra"})
	autoRequest(t, s, "POST", "/api/worlds/"+d.World.ID+"/accept", string(payload))
	if err = s.applyHistoricalMap(ctx, v); err != nil {
		t.Fatal(err)
	}
	a := artworkFixture()
	a.SourceRevision = v.world.Header.Revision
	payload, _ = json.Marshal(a)
	result := autoRequest(t, s, "POST", "/api/sessions/art/world/artworks", string(payload))
	raw, _ := json.Marshal(result["artwork"])
	json.Unmarshal(raw, &a)
	if a.AgeID != d.World.CurrentAge || a.MapID != v.historical.ID {
		t.Fatal("artwork lost historical ownership")
	}
	d, _ = db.History(ctx, d.World.ID)
	parent := *v.historical
	var child history.Map
	d, err = db.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(d *history.Document) error {
		var err error
		child, err = d.Child(d.World.CurrentAge, parent.ID, "", "Detail", world.Bounds{X: 8, Y: 8, Width: 4, Height: 4})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	opened := autoRequest(t, s, "POST", "/api/projects/"+child.ProjectID+"/open", `{}`)
	cv := s.sessions[opened["id"].(string)]
	if len(cv.world.Artworks) != 0 {
		t.Fatal("child inherited parent's illustration as its own")
	}
	data := autoRequest(t, s, "POST", "/api/sessions/"+opened["id"].(string)+"/world/illustration", `{}`)
	bb, _ := json.Marshal(data["bounds"])
	var bounds world.Bounds
	json.Unmarshal(bb, &bounds)
	if bounds != child.Bounds {
		t.Fatal("child exports full terrain", bounds)
	}
	// A saved child checkpoint may still share immutable ancestor blobs.
	_, currentChild, err := db.MapContext(ctx, child.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	parts, err := db.SnapshotParts(ctx, currentChild.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := restoreCheckpoint(child.ProjectID, parts, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.applyHistoricalMap(ctx, reopened); err != nil {
		t.Fatal(err)
	}
	if len(reopened.world.Artworks) != 0 {
		t.Fatal("parent artwork resurfaced after child restart")
	}
	export := httptest.NewRecorder()
	s.ServeHTTP(export, httptest.NewRequest("GET", "/api/worlds/"+d.World.ID+"/export", nil))
	if export.Code != 200 {
		t.Fatal(export.Body.String())
	}
	clean, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer clean.Close()
	server := NewWithOptions(t.TempDir(), Options{Projects: clean, CacheDir: t.TempDir()})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest("POST", "/api/worlds/import", bytes.NewReader(export.Body.Bytes())))
	if response.Code != 201 {
		t.Fatal(response.Code, response.Body.String())
	}
	fresh := autoRequest(t, server, "POST", "/api/projects/"+parent.ProjectID+"/open", `{}`)
	fv := server.sessions[fresh["id"].(string)]
	if !reflect.DeepEqual(fv.world.Artworks[a.ID], a) {
		t.Fatal("portable historical ZIP altered artwork")
	}
}
