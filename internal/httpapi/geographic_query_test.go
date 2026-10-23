package httpapi

import (
	"encoding/json"
	terrain "kriemhild/internal/legacyterrain"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/world"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestGeographicQueryIndependentOfRasterWorkersAndRevisionGuard(t *testing.T) {
	s := New(t.TempDir())
	p := &model.Planet{Spec: model.DefaultSpec(), Grid: model.Grid{Width: 4, Height: 2, Fields: map[string][]float64{}}, Surface: &model.Surface{Width: 4, Height: 2}}
	p.Surface.Bed = make([]float32, 8)
	p.Surface.Coast = make([]float32, 8)
	p.Surface.Roughness = make([]float32, 8)
	p.Surface.LakeLevel = make([]float32, 8)
	for i := range p.Surface.Bed {
		p.Surface.Bed[i] = 100
		p.Surface.Coast[i] = 1
	}
	v := &session{planet: p, solver: &terrain.Solver{W: 4, H: 2}, world: world.New(4, 2)}
	s.sessions["query"] = v
	for i := 0; i < cap(s.detailSlots); i++ {
		s.detailSlots <- struct{}{}
	}
	defer func() {
		for len(s.detailSlots) > 0 {
			<-s.detailSlots
		}
	}()
	revision := v.world.Header.Revision
	for _, coords := range []string{"lon=-180&lat=90", "lon=180&lat=90", "lon=540&lat=-90", "lon=0&lat=0"} {
		done := make(chan *httptest.ResponseRecorder, 1)
		go func() {
			r := httptest.NewRecorder()
			s.ServeHTTP(r, httptest.NewRequest("GET", "/api/sessions/query/terrain/sample?"+coords+"&revision="+strconv.Itoa(revision), nil))
			done <- r
		}()
		select {
		case r := <-done:
			if r.Code != 200 {
				t.Fatal(r.Code, r.Body.String())
			}
			var sample struct {
				model.Sample
				Revision int `json:"revision"`
			}
			if err := json.Unmarshal(r.Body.Bytes(), &sample); err != nil {
				t.Fatal(err)
			}
			if sample.Bed != 100 || sample.Revision != revision {
				t.Fatalf("wrong authoritative result: %+v", sample)
			}
		case <-time.After(time.Second):
			t.Fatal("query waited for a raster worker")
		}
	}
	r := httptest.NewRecorder()
	s.ServeHTTP(r, httptest.NewRequest("GET", "/api/sessions/query/terrain/sample?lon=0&lat=0&revision="+strconv.Itoa(revision+1), nil))
	if r.Code != 409 {
		t.Fatal("stale query accepted", r.Code)
	}
	r = httptest.NewRecorder()
	s.ServeHTTP(r, httptest.NewRequest("GET", "/api/sessions/query/terrain/sample?lon=0&lat=91", nil))
	if r.Code != 400 {
		t.Fatal("invalid latitude accepted", r.Code)
	}
}
