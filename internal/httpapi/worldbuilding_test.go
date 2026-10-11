package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"kriemhild/internal/world"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestAtlasPreviewReadsSavedTerrainWithoutOpeningSession(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	v := projectFixture(t)
	s.sessions["terrain"] = v
	d := history.New("Atlas")
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{"session": "terrain", "ageId": d.World.CurrentAge, "name": "Preview terrain"})
	autoRequest(t, s, "POST", "/api/worlds/"+d.World.ID+"/accept", string(payload))
	d, _ = db.History(ctx, d.World.ID)
	m := d.Maps[0]
	response := httptest.NewRecorder()
	s.ServeHTTP(response, httptest.NewRequest("GET", "/api/worlds/"+d.World.ID+"/maps/"+m.ID+"/preview?age="+m.AgeID, nil))
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	img, err := png.Decode(response.Body)
	if err != nil || img.Bounds().Dx() != v.solver.W {
		t.Fatal("invalid saved terrain preview", err)
	}
	if len(s.sessions) != 1 {
		t.Fatal("preview opened a mutable session")
	}
}

func TestWorldbuildingZIPRestoresPeopleNamesAndNetworks(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := storage.Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	d := history.New("People without a terrain")
	a := d.World.CurrentAge
	for _, e := range []history.Entity{{ID: "mira", Name: "Mira", Kind: "character", Properties: json.RawMessage(`{"biography":"A cartographer","status":"living"}`), Names: []history.EntityName{{Name: "Miri", Type: "local", Language: "River speech"}}}, {ID: "school", Name: "The Observatory", Kind: "school", Properties: json.RawMessage(`{}`)}} {
		if err = d.Apply(history.Command{Kind: "entity", AgeID: a, Entity: &e}); err != nil {
			t.Fatal(err)
		}
	}
	relation := history.Relation{ID: "attendance", Source: "mira", Destination: "school", Kind: "attended", ValidFrom: "42", ValidTo: "47", Description: "Learned navigation"}
	if err = d.Apply(history.Command{Kind: "relation", AgeID: a, Relation: &relation}); err != nil {
		t.Fatal(err)
	}
	view := history.NetworkView{ID: "network", Name: "Mira's connections", CenterID: "mira"}
	d.Apply(history.Command{Kind: "network", AgeID: a, Network: &view})
	if err = d.Apply(history.Command{Kind: "age", AgeID: a, Mode: "branch", Name: "Later"}); err != nil {
		t.Fatal(err)
	}
	b := d.World.CurrentAge
	e := *d.Entity(b, "mira")
	e.Name = "Mira the Navigator"
	d.Apply(history.Command{Kind: "entity", AgeID: b, Entity: &e})
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = storage.Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	response := httptest.NewRecorder()
	s.ServeHTTP(response, httptest.NewRequest("GET", "/api/worlds/"+d.World.ID+"/export", nil))
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	target, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	s2 := NewWithOptions(t.TempDir(), Options{Projects: target, CacheDir: t.TempDir()})
	restoredResponse := httptest.NewRecorder()
	s2.ServeHTTP(restoredResponse, httptest.NewRequest("POST", "/api/worlds/import", bytes.NewReader(response.Body.Bytes())))
	if restoredResponse.Code != 200 && restoredResponse.Code != 201 {
		t.Fatal(restoredResponse.Code, restoredResponse.Body.String())
	}
	loaded, err := target.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Entity(a, "mira").Name != "Mira" || loaded.Entity(b, "mira").Name != "Mira the Navigator" {
		t.Fatal("Age states conflated")
	}
	if !reflect.DeepEqual(loaded.Entity(a, "mira").Names, d.Entity(a, "mira").Names) || len(loaded.Relations) != 2 || len(loaded.World.Networks) != 2 {
		t.Fatal("names, relations or saved graph views lost")
	}
}
func TestWritingStorageIsIndependentAndDetectsConflicts(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	draft := storage.WritingDocument{ID: "draft", Name: "Author notes", Text: "Not a fictional book"}
	if err = db.SaveWriting(ctx, &draft); err != nil {
		t.Fatal(err)
	}
	stale := draft
	draft.Text = "Revised"
	if err = db.SaveWriting(ctx, &draft); err != nil {
		t.Fatal(err)
	}
	if err = db.SaveWriting(ctx, &stale); err == nil {
		t.Fatal("stale draft overwrote newer text")
	}
	worlds, err := db.Histories(ctx)
	if err != nil || len(worlds) != 0 {
		t.Fatal("writing created a fictional world", err)
	}
	notes, err := db.Writing(ctx)
	if err != nil || len(notes) != 1 || notes[0].Text != "Revised" {
		t.Fatal("draft not stored", err)
	}
}

func TestBuilderEntitiesPersistImmediatelyWithoutSavingTerrain(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	v := projectFixture(t)
	s.sessions["terrain"] = v
	d := history.New("Immediate places")
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{"session": "terrain", "ageId": d.World.CurrentAge, "name": "Terrain"})
	autoRequest(t, s, "POST", "/api/worlds/"+d.World.ID+"/accept", string(payload))
	d, _ = db.History(ctx, d.World.ID)
	snapshot := d.Maps[0].Snapshot
	country := world.Entity{ID: "country", Name: "The March", Kind: "country", Layer: "countries", Geometry: "polygon", Color: "#000000", Points: [][2]float64{{1, 1}, {9, 1}, {9, 9}, {1, 9}}}
	city := world.Entity{ID: "city", Name: "Rivergate", Kind: "city", Layer: "settlements", ParentID: "country", Geometry: "polygon", Color: "#000000", Points: [][2]float64{{2, 2}, {3, 2}, {3, 3}, {2, 3}}}
	send := func(kind string, e world.Entity) {
		t.Helper()
		b, _ := json.Marshal(map[string]any{"kind": kind, "entity": e, "revision": v.world.Header.Revision})
		autoRequest(t, s, "POST", "/api/sessions/terrain/world", string(b))
	}
	send("entity", country)
	send("entity", city)
	d, err = db.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if e := d.Entity(d.World.CurrentAge, "city"); e == nil || e.Name != "Rivergate" {
		t.Fatal("city absent until map save")
	}
	if d.Maps[0].Snapshot != snapshot {
		t.Fatal("entity update saved the terrain snapshot")
	}
	count := 0
	for _, r := range d.Relations {
		if r.Source == "city" && r.Destination == "country" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("duplicate spatial relationship: %d", count)
	}
	send("undo", world.Entity{})
	d, _ = db.History(ctx, d.World.ID)
	if !d.Entity(d.World.CurrentAge, "city").Deleted {
		t.Fatal("undo did not update logical identity")
	}
	send("redo", world.Entity{})
	d, _ = db.History(ctx, d.World.ID)
	if d.Entity(d.World.CurrentAge, "city").Deleted {
		t.Fatal("redo did not restore logical identity")
	}
	house := world.Entity{ID: "house", Name: "example-house-unique", Kind: "house", Layer: "buildings", Geometry: "point", Color: "#000000", Points: [][2]float64{{2.5, 2.5}}}
	send("entity", house)
	d, _ = db.History(ctx, d.World.ID)
	if d.Entity(d.World.CurrentAge, "house") != nil {
		t.Fatal("placeholder house must remain a map drawable, not a logical entity")
	}
	if v.world.Entities["house"].Name != "example-house-unique" {
		t.Fatal("unnamed drawable was lost")
	}
	if err = d.Validate(); err != nil {
		t.Fatal(err)
	}
	autoRequest(t, s, "POST", "/api/sessions/terrain/save", `{}`)
	d, _ = db.History(ctx, d.World.ID)
	bundle, err := db.HistoryBundle(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = bundle.Validate(); err != nil {
		t.Fatal(err)
	}
	restored, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if err = restored.ImportHistory(ctx, bundle); err != nil {
		t.Fatal(err)
	}
	copy, err := restored.History(ctx, d.World.ID)
	if err != nil || copy.Entity(d.World.CurrentAge, "city").Name != "Rivergate" {
		t.Fatal("immediate entity lost on portable import", err)
	}
	fresh := NewWithOptions(t.TempDir(), Options{Projects: restored, CacheDir: t.TempDir()})
	opened := autoRequest(t, fresh, "POST", "/api/projects/"+copy.Maps[0].ProjectID+"/open", `{}`)
	loaded := fresh.sessions[opened["id"].(string)]
	if got := loaded.authored().Entities["house"]; got.Name != "example-house-unique" || len(got.Points) != 1 {
		t.Fatal("unnamed geometry lost on save and portable restore", got)
	}
	if copy.Entity(copy.World.CurrentAge, "house") != nil {
		t.Fatal("saved unnamed map object leaked into logical table")
	}
	named := loaded.authored().Entities["house"]
	named.Name = "The Old Hall"
	body, _ := json.Marshal(map[string]any{"kind": "entity", "entity": named, "revision": loaded.authored().Header.Revision})
	autoRequest(t, fresh, "POST", "/api/sessions/"+opened["id"].(string)+"/world", string(body))
	copy, _ = restored.History(ctx, copy.World.ID)
	if got := copy.Entity(copy.World.CurrentAge, "house"); got == nil || got.Name != "The Old Hall" {
		t.Fatal("naming restored drawable failed to create logical record")
	}
}

func TestFamilyGraphDatabaseAndArchiveRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	d := history.New("Family graphs")
	a := d.World.CurrentAge
	for _, id := range []string{"parent", "child"} {
		e := history.Entity{ID: id, Name: id, Kind: "character"}
		if err = d.Apply(history.Command{Kind: "entity", AgeID: a, Entity: &e}); err != nil {
			t.Fatal(err)
		}
	}
	relation := history.Relation{ID: "connection", Source: "parent", Destination: "child", Kind: "parent-of", Description: "Raised at the coast", SourcePort: "se", DestinationPort: "nw"}
	if err = d.Apply(history.Command{Kind: "relation", AgeID: a, Relation: &relation}); err != nil {
		t.Fatal(err)
	}
	n := history.NetworkView{ID: "family-view", Name: "House of the Coast", Genealogy: true, MemberIDs: []string{"parent", "child"}}
	if err = d.Apply(history.Command{Kind: "network", AgeID: a, Network: &n}); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	server := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	out := httptest.NewRecorder()
	server.ServeHTTP(out, httptest.NewRequest("GET", "/api/worlds/"+d.World.ID+"/export", nil))
	if out.Code != 200 {
		t.Fatal(out.Code, out.Body.String())
	}
	target, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	next := NewWithOptions(t.TempDir(), Options{Projects: target, CacheDir: t.TempDir()})
	imported := httptest.NewRecorder()
	next.ServeHTTP(imported, httptest.NewRequest("POST", "/api/worlds/import", bytes.NewReader(out.Body.Bytes())))
	if imported.Code != 200 && imported.Code != 201 {
		t.Fatal(imported.Code, imported.Body.String())
	}
	loaded, err := target.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.World.Networks, d.World.Networks) || !reflect.DeepEqual(loaded.Relations, d.Relations) || len(loaded.Entities) != 2 {
		t.Fatal("graph settings, ports or identities changed after archive round trip")
	}
}
