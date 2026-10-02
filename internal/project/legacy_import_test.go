package project

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestTrueFormatOneImportRewritesSceneAndSourcePins(t *testing.T) {
	dir := filepath.Join(t.TempDir(), NewID())
	for _, ns := range []string{"objects", "snapshots", "revisions", "assets"} {
		if e := os.MkdirAll(filepath.Join(dir, ns), 0700); e != nil {
			t.Fatal(e)
		}
	}
	old := &Store{Dir: dir}
	putLegacy := func(ns string, v any) string {
		h, e := old.put(ns, v)
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	city := Record{ID: NewID(), Kind: "entity", Name: "Original port", Type: "Settlement"}
	cityHash := putLegacy("objects", city)
	base := putLegacy("snapshots", Snapshot{1, map[string]string{city.ID: cityHash}})
	firstID, secondID := NewID(), NewID()
	scene := Record{ID: NewID(), Kind: "scene", Name: "Old scene", SettingAge: firstID, SettingSnapshot: base, References: []string{city.ID}, Document: json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"The old port."}]}]}`)}
	sceneHash := putLegacy("objects", scene)
	var pixels bytes.Buffer
	png.Encode(&pixels, image.NewNRGBA(image.Rect(0, 0, 2, 2)))
	asset, e := old.putBytes("assets", pixels.Bytes())
	if e != nil {
		t.Fatal(e)
	}
	atlas := Record{ID: NewID(), Kind: "map", Name: "Old atlas", Width: 2, Height: 2, Asset: asset, Pins: []Pin{{city.ID, .5, .5}}}
	mapHash := putLegacy("objects", atlas)
	snap := putLegacy("snapshots", Snapshot{1, map[string]string{city.ID: cityHash, scene.ID: sceneHash, atlas.ID: mapHash}})
	marker := Marker{1, filepath.Base(dir), "Legacy format one"}
	a := Age{ID: firstID, Name: "First", Snapshot: snap, Undo: []string{base}, Redo: []string{}}
	r := Root{Version: 1, World: marker, Ages: map[string]Age{firstID: a}, Message: "First legacy revision", SavedAt: "2020-01-01T00:00:00Z"}
	revision := putLegacy("revisions", r)
	r.Parent = revision
	r.Ages[secondID] = Age{ID: secondID, Name: "Second", Snapshot: snap, SourceAge: firstID, SourceSnapshot: snap, SourceRevision: revision, Undo: []string{}, Redo: []string{}}
	revision = putLegacy("revisions", r)
	h, _ := json.Marshal(Head{revision})
	os.WriteFile(filepath.Join(dir, "project.head.json"), h, 0600)
	m, _ := json.Marshal(marker)
	os.WriteFile(filepath.Join(dir, "kriemhild.json"), m, 0600)
	archive, e := ArchiveDirectory(dir)
	if e != nil {
		t.Fatal(e)
	}
	library := t.TempDir()
	preview, e := PrepareImport(library, archive)
	if e != nil {
		t.Fatal(e)
	}
	if preview.SourceFormat != 1 {
		t.Fatal("incorrect source format")
	}
	if e = AcceptImport(library, preview); e != nil {
		t.Fatal(e)
	}
	migrated, e := Open(filepath.Join(library, preview.World.ID))
	if e != nil {
		t.Fatal(e)
	}
	defer migrated.Close()
	state, e := migrated.State(secondID)
	if e != nil {
		t.Fatal(e)
	}
	if state.Records[scene.ID].SettingSnapshot == base || state.Age.SourceSnapshot == snap || state.Age.SourceRevision == r.Parent {
		t.Fatal("legacy hash pins were not migrated")
	}
	setting, e := migrated.SnapshotRecords(state.Records[scene.ID].SettingSnapshot)
	if e != nil || setting[city.ID].Name != city.Name {
		t.Fatal("historical pin no longer resolves", e)
	}
	data, e := migrated.Asset(asset)
	if e != nil || !bytes.Equal(data, pixels.Bytes()) {
		t.Fatal("native import lost raster bytes", e)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "project.head.json"))
	if !bytes.Equal(h, after) {
		t.Fatal("migration changed original format-one project")
	}
}
