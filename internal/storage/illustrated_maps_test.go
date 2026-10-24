package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"kriemhild/internal/history"
	"kriemhild/internal/world"
	"testing"
)

func TestIllustratedMapAtomicFilingBannerVersionsAndRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	d := history.New("Illustrations")
	if err = s.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	state := world.New(32, 32)
	country := world.Entity{ID: "zenon", Name: "Zenon", Kind: "country", Layer: "countries", Geometry: "polygon", Color: "#aabbcc", Points: [][2]float64{{4, 4}, {20, 4}, {20, 20}, {4, 20}}}
	if _, err = state.PutEntity(country); err != nil {
		t.Fatal(err)
	}
	d, m, err := s.AttachTerrain(ctx, d.World.ID, d.World.CurrentAge, history.ID(), "Geography", 32, 32, map[string][]byte{"terrain.json": []byte("immutable")}, state)
	if err != nil {
		t.Fatal(err)
	}
	age := d.World.CurrentAge
	before, _ := json.Marshal(d.Representations)
	art := world.Illustration{Version: 1, ID: "medieval-world", Title: "World atlas", Scope: "world", Style: "medieval", Bounds: [4]float64{0, 0, 2, 1}, Layers: []world.IllustrationLayer{{ID: "labels", Name: "Labels", Visible: true, Opacity: 1}}, Objects: map[string]world.IllustrationObject{}}
	picture := func(c color.NRGBA) []byte {
		im := image.NewNRGBA(image.Rect(0, 0, 16, 16))
		im.SetNRGBA(0, 0, c)
		var b bytes.Buffer
		png.Encode(&b, im)
		return b.Bytes()
	}
	req := IllustratedMapRequest{MapID: m.ID, AgeID: age, SourceRevision: state.Header.Revision, Illustration: art, PNG: picture(color.NRGBA{R: 200, A: 255})}
	d, id, inbox, err := s.SaveIllustratedMap(ctx, d.World.ID, d.World.Revision, req)
	if err != nil {
		t.Fatal(err)
	}
	if d.Entity(age, inbox).Name != "Illustrated maps" || d.LibraryFile(age, id).ArchiveID != inbox {
		t.Fatal("global map not filed in inbox")
	}
	if d.Entity(age, country.ID) == nil {
		t.Fatal("country record missing")
	}
	req.FocusID = country.ID
	req.Illustration.ID = "medieval-country"
	req.Illustration.Scope = "country"
	req.Illustration.Focus = country.ID
	req.Illustration.Title = "Zenon atlas"
	d, focusFile, folder, err := s.SaveIllustratedMap(ctx, d.World.ID, d.World.Revision, req)
	if err != nil {
		t.Fatal(err)
	}
	if d.Entity(age, folder).Name != "Zenon maps" || folder == inbox {
		t.Fatal("focused map not filed with place")
	}
	var props struct {
		IllustratedMap struct {
			FileID  string `json:"fileId"`
			Version string `json:"version"`
		} `json:"illustratedMap"`
	}
	json.Unmarshal(d.Entity(age, country.ID).Properties, &props)
	if props.IllustratedMap.FileID != focusFile || props.IllustratedMap.Version == "" {
		t.Fatal("country banner not linked")
	}
	revision := d.World.Revision
	if _, _, _, err = s.SaveIllustratedMap(ctx, d.World.ID, revision-1, req); err == nil {
		t.Fatal("accepted stale save")
	}
	bad := req
	bad.PNG = bad.PNG[:len(bad.PNG)-10]
	if _, _, _, err = s.SaveIllustratedMap(ctx, d.World.ID, revision, bad); err == nil {
		t.Fatal("accepted truncated PNG")
	}
	bad = req
	bad.FocusID = "missing"
	if _, _, _, err = s.SaveIllustratedMap(ctx, d.World.ID, revision, bad); err == nil {
		t.Fatal("accepted missing place")
	}
	current, _ := s.History(ctx, d.World.ID)
	if current.World.Revision != revision {
		t.Fatal("failed save changed database")
	}
	// Repeated images are deduplicated; user filing survives subsequent versions.
	d, _, _, err = s.SaveIllustratedMap(ctx, d.World.ID, d.World.Revision, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.LibraryFile(age, focusFile).Versions) != 1 {
		t.Fatal("duplicate image version")
	}
	d, err = s.UpdateHistory(ctx, d.World.ID, d.World.Revision, func(doc *history.Document) error { doc.LibraryFile(age, focusFile).ArchiveID = inbox; return nil })
	if err != nil {
		t.Fatal(err)
	}
	req.PNG = picture(color.NRGBA{B: 200, A: 255})
	d, _, filed, err := s.SaveIllustratedMap(ctx, d.World.ID, d.World.Revision, req)
	if err != nil {
		t.Fatal(err)
	}
	if filed != inbox || len(d.LibraryFile(age, focusFile).Versions) != 2 {
		t.Fatal("version overwrote filing")
	}
	after, _ := json.Marshal(d.Representations)
	if !bytes.Equal(before, after) {
		t.Fatal("illustration changed geographic coordinates")
	}
	if err = d.Validate(); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	data, _, err := s.LibraryContent(ctx, d.World.ID, age, focusFile, "")
	if err != nil || !bytes.Equal(data, req.PNG) {
		t.Fatal("restart lost illustration", err)
	}
	bundle, err := s.HistoryBundle(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	portable, err := Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer portable.Close()
	if err = portable.ImportHistory(ctx, bundle); err != nil {
		t.Fatal(err)
	}
	data, _, err = portable.LibraryContent(ctx, d.World.ID, age, focusFile, props.IllustratedMap.Version)
	if err != nil || bytes.Equal(data, req.PNG) {
		t.Fatal("portable bundle lost earlier image", err)
	}
}
