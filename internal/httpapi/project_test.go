package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"kriemhild/internal/terrain"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func projectFixture(t *testing.T) *session {
	t.Helper()
	c, err := terrain.DecodeConfig(terrain.Defaults)
	if err != nil {
		t.Fatal(err)
	}
	opts, err := terrain.DecodeEnvironment([]byte(`{"columns":48,"rows":32,"realism":true,"seed":"portable-world"}`))
	if err != nil {
		t.Fatal(err)
	}
	s, err := terrain.PrepareEnvironment(opts, c)
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 100000 && s.Status == "running"; n++ {
		s.Step()
	}
	if s.Status != "done" {
		t.Fatal(s.Status)
	}
	v := &session{solver: s, last: time.Now(), undo: map[string]terrain.Snapshot{}, generation: json.RawMessage(`{"seed":"portable-world"}`), projectUI: json.RawMessage(`{"seedUsed":42,"camera2d":{"center":{"x":12.5,"y":11.25},"scale":300}}`)}
	v.detail = terrain.NewDetailModel(s.Environment)
	t.Cleanup(func() {
		if v.dir != "" {
			os.RemoveAll(v.dir)
		}
	})
	return v
}

func unzipProject(t *testing.T, archive []byte) map[string][]byte {
	t.Helper()
	r := httptest.NewRequest("POST", "/api/projects/import", bytes.NewReader(archive))
	files, err := readProjectFiles(r)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestWorldProjectColdRestartRestoresStoredWorldAndDetail(t *testing.T) {
	v := projectFixture(t)
	first, err := v.storedDetail(5, 12, 10)
	if err != nil {
		t.Fatal(err)
	}
	// Deliberately distinguish this stored interior sample from the generator's
	// output. A seed-only export or regeneration-on-import would lose this value.
	var tile terrain.DetailTile
	json.Unmarshal(first, &tile)
	tile.Points[17*33+17].Elevation += .125
	stored, _ := json.Marshal(tile)
	if err = v.putDetail("5/12/10", stored); err != nil {
		t.Fatal(err)
	}
	if _, err = v.storedDetail(4, 18, 9); err != nil {
		t.Fatal(err)
	} // another previously visited area
	original := v.solver.Snapshot()
	v.base = &original
	// A legal explicit pin is part of the authoring layer, separate from base data.
	v.solver.Pinned[100] = 1
	v.edits = []json.RawMessage{json.RawMessage(`{"action":"pin","cell":100}`)}
	archive, err := encodeProject(v)
	if err != nil {
		t.Fatal(err)
	}
	fieldCopy := map[string][]float64{}
	for name, f := range v.solver.Environment.Fields {
		fieldCopy[name] = append([]float64{}, f...)
	}
	expected := v.solver.Snapshot()
	worldID := v.worldID
	count := len(v.tiles)
	oldDir := v.dir
	if err = os.RemoveAll(oldDir); err != nil {
		t.Fatal(err)
	}
	v.dir = ""
	if _, err = os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatal("old server cache survived")
	}
	fresh := New(t.TempDir())
	req := httptest.NewRequest("POST", "/api/projects/import", bytes.NewReader(archive))
	response := httptest.NewRecorder()
	fresh.ServeHTTP(response, req)
	if response.Code != 201 {
		t.Fatal(response.Code, response.Body.String())
	}
	var state struct {
		ID     string `json:"id"`
		Stored int    `json:"storedDetailCount"`
	}
	json.Unmarshal(response.Body.Bytes(), &state)
	loaded := fresh.sessions[state.ID]
	defer os.RemoveAll(loaded.dir)
	if state.Stored != count || loaded.worldID != worldID {
		t.Fatal("lost tile inventory/world identity")
	}
	if !reflect.DeepEqual(fieldCopy, loaded.solver.Environment.Fields) || !reflect.DeepEqual(expected, loaded.solver.Snapshot()) {
		t.Fatal("stored world or edits changed on cold import")
	}
	if loaded.base.Pinned[100] != 0 || loaded.solver.Pinned[100] != 1 {
		t.Fatal("generated base and edit layer were conflated")
	}
	// Prove existing detail can be served with the detail generator unavailable.
	model := loaded.detail
	loaded.detail = nil
	got, err := loaded.storedDetail(5, 12, 10)
	if err != nil || !bytes.Equal(got, stored) {
		t.Fatal("explored detail regenerated instead of loading stored bytes", err)
	}
	loaded.detail = model
	descendant, err := loaded.storedDetail(6, 25, 21)
	if err != nil {
		t.Fatal(err)
	}
	var child terrain.DetailTile
	json.Unmarshal(descendant, &child)
	if child.Points[2*33+2].Elevation != tile.Points[17*33+17].Elevation {
		t.Fatal("new descendant replaced stored parent detail")
	}
	// New areas still derive from the stored physical model, not old server data.
	a, err := loaded.storedDetail(4, 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	b, err := v.detail.Tile(4, 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	expectedTile, _ := json.Marshal(b)
	if !bytes.Equal(a, expectedTile) {
		t.Fatal("missing detail changed after cold restart")
	}
	if _, err = encodeProject(loaded); err != nil {
		t.Fatal("imported project cannot be resaved", err)
	}
}

func TestWorldProjectFolderAndZIPUseSameHierarchy(t *testing.T) {
	v := projectFixture(t)
	v.storedDetail(3, 2, 1)
	archive, err := encodeProject(v)
	if err != nil {
		t.Fatal(err)
	}
	files := unzipProject(t, archive)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, b := range files {
		part, err := writer.CreateFormFile(name, filepath.Base(name))
		if err != nil {
			t.Fatal(err)
		}
		part.Write(b)
	}
	writer.Close()
	request := httptest.NewRequest("POST", "/api/projects/import", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	folder, err := readProjectFiles(request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(files, folder) {
		t.Fatal("folder import flattened the project")
	}
	loaded, err := decodeProject(folder)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(loaded.dir)
	if len(loaded.tiles) != len(v.tiles) {
		t.Fatal("folder lost detail")
	}
}

func TestWorldProjectRejectsCorruptionMissingDataAndBadHierarchy(t *testing.T) {
	v := projectFixture(t)
	v.storedDetail(3, 2, 1)
	archive, err := encodeProject(v)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"checksum", "missing", "version", "parent", "traversal", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			files := unzipProject(t, archive)
			var m projectManifest
			json.Unmarshal(files["KRIEMHILD/manifest.json"], &m)
			switch kind {
			case "checksum":
				files["KRIEMHILD/base/fields/0/0.f64"][0] ^= 1
			case "missing":
				delete(files, "KRIEMHILD/detail/tiles/3/2/1.json")
			case "version":
				m.Schema = 999
			case "parent":
				m.Regions[len(m.Regions)-1].Parent = "detail/0/999/999"
			case "traversal":
				m.Files[0].Path = "../solver.json"
			case "duplicate":
				m.Regions = append(m.Regions, m.Regions[0])
			}
			files["KRIEMHILD/manifest.json"], _ = json.Marshal(m)
			if loaded, err := decodeProject(files); err == nil {
				if loaded.dir != "" {
					os.RemoveAll(loaded.dir)
				}
				t.Fatal("invalid project accepted")
			}
		})
	}
}

func TestStoredDetailEndpointReturnsArchiveBytes(t *testing.T) {
	v := projectFixture(t)
	original, err := v.storedDetail(2, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	s := New(t.TempDir())
	s.sessions["test"] = v
	r := httptest.NewRequest(http.MethodGet, "/api/sessions/test/detail/2/1/1", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	result, _ := io.ReadAll(w.Result().Body)
	if w.Code != 200 || !bytes.Equal(result, original) {
		t.Fatal("detail endpoint did not use durable tile store", w.Code)
	}
}

func TestRulesOnlyWorldProjectRoundTrip(t *testing.T) {
	c, err := terrain.DecodeConfig(terrain.Defaults)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := terrain.Compile(c)
	if err != nil {
		t.Fatal(err)
	}
	solver := terrain.NewSolver(rules, terrain.Options{Width: 24, Height: 16, Radius2: 1, Seed: 71}, nil)
	for n := 0; n < 100000 && solver.Status == "running"; n++ {
		solver.Step()
	}
	if solver.Status != "done" {
		t.Fatal(solver.Status)
	}
	v := &session{solver: solver, generation: json.RawMessage(`{"options":{"seed":71}}`), projectUI: json.RawMessage(`{}`)}
	archive, err := encodeProject(v)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := decodeProject(unzipProject(t, archive))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.solver.Environment != nil || loaded.detail != nil || !reflect.DeepEqual(solver.Snapshot(), loaded.solver.Snapshot()) {
		t.Fatal("rules-only project changed")
	}
	if !loaded.solver.Paint([]int{100}, loaded.solver.TypeAt(100)) {
		t.Fatal("restored world cannot be edited")
	}
}
