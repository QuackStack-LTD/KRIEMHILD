package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"kriemhild/internal/storage"
	"kriemhild/internal/world"
	"os"
	"reflect"
	"testing"
)

func TestClimateArchiveDatabaseAndAuthoredRestoration(t *testing.T) {
	v := projectFixture(t)
	// A deliberate persisted value proves imports load data rather than run
	// today's generator again. Zones, curves and season definitions are all data.
	v.solver.Environment.Climate.Cells[0].Months[0].Humidity = .4321
	base, _ := json.Marshal(v.solver.Environment.Climate)
	geography, _ := json.Marshal(v.solver.Environment.Geography)
	erosion, _ := json.Marshal(v.solver.Environment.Geomorphology)
	drainage, _ := json.Marshal(v.solver.Environment.Hydrology)
	cell := 0
	for i, b := range v.solver.Environment.Fields["waterBody"] {
		if b == 0 {
			cell = i
			break
		}
	}
	x, y := float64(cell%v.solver.W), float64(cell/v.solver.W)
	before := v.climateLayer().Describe(x, y)
	_, err := v.authored().AddOperation(world.Operation{ID: "climate-uplift", Kind: "raise", Layer: "elevation", Center: [2]float64{x, y}, Radius: 2, Amount: 600})
	if err != nil {
		t.Fatal(err)
	}
	after := v.climateLayer().Describe(x, y)
	if reflect.DeepEqual(before, after) {
		t.Fatal("authored elevation left stale climate")
	}
	forest := v.climateLayer().Cells[cell].Modifiers.Forest
	kind := "plant"
	if forest > .5 {
		kind = "clear"
	}
	if _, err = v.authored().AddOperation(world.Operation{ID: "climate-cover", Kind: kind, Layer: "vegetation", Center: [2]float64{x, y}, Radius: 2, Amount: 1}); err != nil {
		t.Fatal(err)
	}
	if v.climateLayer().Cells[cell].Modifiers.Forest == forest {
		t.Fatal("surface vegetation edit did not refresh climate")
	}
	v.climate.State.Cells[cell].Months[1].Humidity = .3219
	expected, _ := json.Marshal(v.climate.State)
	archive, err := encodeProject(v)
	if err != nil {
		t.Fatal(err)
	}
	files := unzipProject(t, archive)
	var m projectManifest
	json.Unmarshal(files["KRIEMHILD/manifest.json"], &m)
	if m.Schema != 4 || m.Climate == nil || m.AuthoredClimate == nil || len(m.Climate.Chunks) != 2 {
		t.Fatal("climate chunk hierarchy missing", m.Climate)
	}
	loaded, err := decodeProject(files, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if loaded.dir != "" {
			os.RemoveAll(loaded.dir)
		}
	})
	assert := func(v *session) {
		t.Helper()
		g, _ := json.Marshal(v.solver.Environment.Geography)
		er, _ := json.Marshal(v.solver.Environment.Geomorphology)
		h, _ := json.Marshal(v.solver.Environment.Hydrology)
		if !bytes.Equal(g, geography) || !bytes.Equal(er, erosion) || !bytes.Equal(h, drainage) {
			t.Fatal("planetary scale, erosion history or drainage changed on restoration")
		}
		b, _ := json.Marshal(v.solver.Environment.Climate)
		a, _ := json.Marshal(v.climateLayer())
		if !bytes.Equal(b, base) || !bytes.Equal(a, expected) {
			t.Fatal("climate regenerated or changed on restoration")
		}
	}
	assert(loaded)
	dir := t.TempDir()
	db, err := storage.Open(context.Background(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	s.sessions["climate"] = loaded
	if err = s.saveWorld(loaded, true); err != nil {
		t.Fatal(err)
	}
	id := loaded.worldID
	q := autoRequest(t, s, "GET", "/api/sessions/climate/world/natural?x=0&y=0&radius=0", "")
	if q["climate"].(map[string]any)["available"] != true {
		t.Fatal("climate not exposed by point query")
	}
	overlay := autoRequest(t, s, "GET", "/api/sessions/climate/world/climate", "")
	if overlay["available"] != true || len(overlay["fields"].(map[string]any)["seasonCount"].([]any)) != v.solver.N {
		t.Fatal("climate overlay coverage missing")
	}
	db.Close()
	db, err = storage.Open(context.Background(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	fresh := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	opened := autoRequest(t, fresh, "POST", "/api/projects/"+id+"/open", `{}`)
	restored := fresh.sessions[opened["id"].(string)]
	t.Cleanup(func() {
		if restored.dir != "" {
			os.RemoveAll(restored.dir)
		}
	})
	assert(restored)
	// Clear all new climate data to model an older terrain. Loading must not
	// silently add a new simulation to a historically saved world.
	legacy := projectFixture(t)
	legacy.solver.Environment.Climate = nil
	blob, err := encodeProject(legacy)
	if err != nil {
		t.Fatal(err)
	}
	old := unzipProject(t, blob)
	m = projectManifest{}
	json.Unmarshal(old["KRIEMHILD/manifest.json"], &m)
	m.Schema = 3
	old["KRIEMHILD/manifest.json"], _ = json.Marshal(m)
	restoredOld, err := decodeProject(old, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if restoredOld.dir != "" {
			os.RemoveAll(restoredOld.dir)
		}
	})
	if restoredOld.climateLayer() != nil {
		t.Fatal("legacy import regenerated climate")
	}
}

func TestClimateArchiveRejectsBrokenReferences(t *testing.T) {
	v := projectFixture(t)
	archive, err := encodeProject(v)
	if err != nil {
		t.Fatal(err)
	}
	files := unzipProject(t, archive)
	var m projectManifest
	json.Unmarshal(files["KRIEMHILD/manifest.json"], &m)
	m.Climate.Chunks[1] = m.Climate.Chunks[0]
	files["KRIEMHILD/manifest.json"], _ = json.Marshal(m)
	if _, err = decodeProject(files, t.TempDir()); err == nil {
		t.Fatal("duplicate climate chunks accepted")
	}
}
