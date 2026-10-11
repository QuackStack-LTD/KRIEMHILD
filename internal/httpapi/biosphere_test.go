package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"kriemhild/internal/biosphere"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"
)

func TestBiosphereGenerationRestartInheritanceAndZIP(t *testing.T) {
	ctx, dir := context.Background(), t.TempDir()
	db, err := storage.Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	server := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	v := projectFixture(t)
	server.sessions["terrain"] = v
	d := history.New("Living world")
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	age := d.World.CurrentAge
	payload, _ := json.Marshal(map[string]any{"session": "terrain", "ageId": age, "name": "Living planet"})
	autoRequest(t, server, "POST", "/api/worlds/"+d.World.ID+"/accept", string(payload))
	d, _ = db.History(ctx, d.World.ID)
	terrainID := d.Terrains[0].ID
	path := "/api/worlds/" + d.World.ID + "/biosphere/" + terrainID
	request := func(method, url string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, httptest.NewRequest(method, url, bytes.NewReader(raw)))
		return rr
	}
	response := request("GET", path+"?age="+age, nil)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	var env struct {
		Environment biosphere.State `json:"environment"`
	}
	json.Unmarshal(response.Body.Bytes(), &env)
	if len(env.Environment.Habitats) < 3 {
		t.Fatal("missing terrain habitats")
	}
	settings := env.Environment.Settings
	settings.Richness = "diverse"
	body := map[string]any{"ageId": age, "revision": d.World.Revision, "settings": settings}
	response = request("POST", path, body)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var preview biosphere.Preview
	json.Unmarshal(response.Body.Bytes(), &preview)
	if len(preview.Species) < 8 || len(preview.State.Interactions) == 0 {
		t.Fatal("incomplete generated community")
	}
	before, _ := db.History(ctx, d.World.ID)
	if len(before.Biospheres) != 0 {
		t.Fatal("preview saved without acceptance")
	}
	species := map[string]biosphere.Lifeform{}
	for _, sp := range preview.Species {
		species[sp.ID] = sp.Biology
	}
	if err = biosphere.ValidateState(preview.State, species); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, o := range preview.State.Occurrences {
		ids = append(ids, o.ID)
	}
	body["accept"] = true
	body["acceptIds"] = ids
	body["lockIds"] = []string{ids[0]}
	body["source"] = "outdated"
	if response = request("POST", path, body); response.Code != 409 {
		t.Fatal("stale terrain accepted", response.Code)
	}
	body["source"] = preview.State.Source
	body["acceptAll"] = true
	response = request("POST", path, body)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	json.Unmarshal(response.Body.Bytes(), &d)
	saved := biosphere.Clone(*d.Biosphere(age, terrainID))
	// Rerunning generation preserves accepted names, traits and all confirmed ranges.
	first := d.Lifeforms(age)[0].ID
	edited := biosphere.Clone(*d.Entity(age, first))
	edited.Name = "Authored lifeform"
	edited.Biology.Traits["authorship"] = "Keep this"
	edited.Properties = json.RawMessage(`{"startYear":123}`)
	d, err = db.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(n *history.Document) error {
		return n.Apply(history.Command{Kind: "entity", AgeID: age, Entity: &edited})
	})
	if err != nil {
		t.Fatal(err)
	}
	body["revision"] = d.World.Revision
	response = request("POST", path, body)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	json.Unmarshal(response.Body.Bytes(), &d)
	if d.Entity(age, first).Name != "Authored lifeform" || d.Entity(age, first).Biology.Traits["authorship"] != "Keep this" {
		t.Fatal("regeneration replaced author data")
	}
	if !reflect.DeepEqual(saved.Occurrences, d.Biosphere(age, terrainID).Occurrences) {
		t.Fatal("confirmed ranges changed")
	}
	if len(d.Years) == 0 {
		t.Fatal("lifeform dates missing from years")
	}
	d, err = db.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(n *history.Document) error {
		return n.Apply(history.Command{Kind: "age", AgeID: age, Name: "After the winter", Mode: "after"})
	})
	if err != nil {
		t.Fatal(err)
	}
	future := d.World.CurrentAge
	if !reflect.DeepEqual(d.Entity(age, first).Biology, d.Entity(future, first).Biology) {
		t.Fatal("biology not inherited")
	}
	editState := biosphere.Clone(*d.Biosphere(future, terrainID))
	editState.Occurrences[0].Status = "locally-extinct"
	d, err = db.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(n *history.Document) error {
		return n.Apply(history.Command{Kind: "biosphere", AgeID: future, BiosphereEdit: &history.BiosphereEdit{TerrainID: terrainID, Occurrence: &editState.Occurrences[0]}})
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Biosphere(age, terrainID).Occurrences[0].Status == "locally-extinct" {
		t.Fatal("future extinction leaked backwards")
	}
	futureEntity := biosphere.Clone(*d.Entity(future, first))
	futureEntity.Biology.Traits["authorship"] = "Future trait"
	d, err = db.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(n *history.Document) error {
		return n.Apply(history.Command{Kind: "entity", AgeID: future, Entity: &futureEntity})
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Entity(age, first).Biology.Traits["authorship"] != "Keep this" {
		t.Fatal("Age traits not isolated")
	}
	db.Close()
	db, err = storage.Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	restored, err := db.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	sortStates := func(d *history.Document) {
		sort.Slice(d.Biospheres, func(i, j int) bool { return d.Biospheres[i].AgeID < d.Biospheres[j].AgeID })
	}
	sortStates(d)
	sortStates(restored)
	if !reflect.DeepEqual(d.Biospheres, restored.Biospheres) {
		t.Fatal("database restart lost ecological state")
	}
	server = NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	export := request("GET", "/api/worlds/"+d.World.ID+"/export", nil)
	if export.Code != 200 {
		t.Fatal(export.Body.String())
	}
	target, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	fresh := NewWithOptions(t.TempDir(), Options{Projects: target, CacheDir: t.TempDir()})
	out := httptest.NewRecorder()
	fresh.ServeHTTP(out, httptest.NewRequest("POST", "/api/worlds/import", bytes.NewReader(export.Body.Bytes())))
	if out.Code != 200 && out.Code != 201 {
		t.Fatal(out.Code, out.Body.String())
	}
	restored, err = target.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	sortStates(restored)
	if !reflect.DeepEqual(d.Biospheres, restored.Biospheres) || !reflect.DeepEqual(d.Entity(age, first).Biology, restored.Entity(age, first).Biology) {
		t.Fatal("portable ZIP lost biology, ranges, interactions or rules")
	}
	if _, err = target.SnapshotParts(ctx, restored.Maps[0].Snapshot); err != nil {
		t.Fatal("portable terrain missing", err)
	}
}
