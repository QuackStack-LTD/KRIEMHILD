package httpapi

import (
	"bytes"
	"context"
	"kriemhild/internal/legacyterrain"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/world"
	"math"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCorrectedClimateAuthoredAtmosphericElevation(t *testing.T) {
	for _, version := range []int{0, 1, 2} {
		p := &model.Planet{Spec: model.DefaultSpec()}
		p.Spec.ClimateVersion = version
		for _, amount := range []float64{50, 300} {
			ops := []world.Operation{{ID: "raise", Kind: "raise", Layer: "elevation", Center: [2]float64{50, 50}, Radius: 2, Amount: amount}}
			f := continuousTransform(ops, 101, 101, p)
			s := f(model.Sample{Bed: -100, Temperature: 20, Water: "dry", Biome: "forest"}, 0, 0)
			want := 20 - amount*.0065
			if version == 2 {
				want = 20 - math.Max(0, -100+amount)*.0065
			}
			if math.Abs(s.Temperature-want) > 1e-10 {
				t.Fatalf("version %d amount %g: got %g want %g", version, amount, s.Temperature, want)
			}
		}
	}
}

func TestContinuousTileRevisionAndContentCache(t *testing.T) {
	spec := model.DefaultSpec()
	spec.ClimateVersion = 0
	p := &model.Planet{Spec: spec, Grid: model.Grid{Width: 2, Height: 2, Fields: map[string][]float64{}}, Surface: &model.Surface{Width: 2, Height: 2, Bed: []float32{1000, 1000, 1000, 1000}, Coast: []float32{100, 100, 100, 100}, Roughness: make([]float32, 4), LakeLevel: make([]float32, 4)}}
	for key, value := range map[string]float64{"bed": 1000, "temperature": 14, "precipitation": 1000, "pet": 900, "soil": 60} {
		p.Grid.Fields[key] = []float64{value, value, value, value}
	}
	server := New(t.TempDir())
	v := &session{planet: p, solver: &terrain.Solver{W: 192, H: 96, Status: "done"}, tileIdentity: "revision-fixture", last: time.Now()}
	server.sessions["tiles"] = v
	get := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRecorder()
		server.ServeHTTP(r, httptest.NewRequest("GET", "/api/sessions/tiles/terrain/tiles/"+path, nil))
		return r
	}
	before := get("5/27/11?style=elevation&strict=1&revision=0")
	if before.Code != 200 {
		t.Fatal(before.Code, before.Body.String())
	}
	hit := get("5/27/11?style=elevation&strict=1&revision=0")
	if hit.Header().Get("X-Terrain-Cache") != "hit" || !bytes.Equal(before.Body.Bytes(), hit.Body.Bytes()) {
		t.Fatal("unchanged tile did not hit cache")
	}
	unrelated := get("5/10/1?style=elevation&strict=1&revision=0")
	_, err := v.authored().AddOperation(world.Operation{ID: "raise", Kind: "raise", Layer: "elevation", Center: [2]float64{27.5 / 32 / 2 * 191, 11.5 / 32 * 95}, Radius: 2, Amount: 1000})
	if err != nil {
		t.Fatal(err)
	}
	stale := get("5/27/11?style=elevation&strict=1&revision=0")
	if stale.Code != 409 {
		t.Fatal("stale strict request accepted", stale.Code)
	}
	after := get("5/27/11?style=elevation&strict=1&revision=1")
	if after.Code != 200 || after.Header().Get("X-Terrain-Revision") != "1" || bytes.Equal(before.Body.Bytes(), after.Body.Bytes()) {
		t.Fatal("edit missing from tile")
	}
	unchanged := get("5/10/1?style=elevation&strict=1&revision=1")
	if unchanged.Header().Get("X-Terrain-Cache") != "hit" || unchanged.Header().Get("X-Terrain-Revision") != "1" || !bytes.Equal(unrelated.Body.Bytes(), unchanged.Body.Bytes()) {
		t.Fatal("unaffected content cache invalidated")
	}
	if _, err = v.authored().Undo(false); err != nil {
		t.Fatal(err)
	}
	undo := get("5/27/11?style=elevation&strict=1&revision=2")
	if undo.Code != 200 || undo.Header().Get("X-Terrain-Cache") != "hit" || !bytes.Equal(before.Body.Bytes(), undo.Body.Bytes()) {
		t.Fatal("undo did not restore content cache")
	}
	// A cancelled request waiting on all three render slots releases promptly.
	for i := 0; i < 3; i++ {
		server.detailSlots <- struct{}{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		server.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/sessions/tiles/terrain/tiles/6/55/23?style=biome", nil).WithContext(ctx))
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancelled renderer remained queued")
	}
	for i := 0; i < 3; i++ {
		<-server.detailSlots
	}
}
