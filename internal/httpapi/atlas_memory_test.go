package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"kriemhild/internal/history"
	"kriemhild/internal/legacyterrain"
	"kriemhild/internal/storage"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestAtlasThumbnailCachePreservesPixelsAndDoesNotHideMissingHighlights(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	s.sessions["terrain"] = projectFixture(t)
	d := history.New("Atlas cache")
	if err := db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	autoRequest(t, s, "POST", "/api/worlds/"+d.World.ID+"/accept", fmt.Sprintf(`{"session":"terrain","ageId":%q,"name":"Land"}`, d.World.CurrentAge))
	d, err = db.HistoryMetadata(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	url := "/api/worlds/" + d.World.ID + "/maps/" + d.Maps[0].ID + "/preview?age=" + d.World.CurrentAge
	read := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	first, cached := read(url), read(url)
	if first.Code != 200 || cached.Code != 200 || first.Header().Get("X-Terrain-Cache") != "miss" || cached.Header().Get("X-Terrain-Cache") != "hit" || !bytes.Equal(first.Body.Bytes(), cached.Body.Bytes()) {
		t.Fatal("immutable thumbnail was not reused exactly", first.Code, cached.Code)
	}
	if highlight := read(url + "&resource=missing"); highlight.Code != 404 {
		t.Fatal("base thumbnail cache hid an invalid highlighted resource", highlight.Code)
	}
}

func TestAtlasReadModelPreservesPixelsAndLoadsResourceGeometryOnlyWhenNeeded(t *testing.T) {
	v := projectFixture(t)
	raw, err := compressJSON(v.solver.Environment)
	if err != nil {
		t.Fatal(err)
	}
	view, err := decodeAtlasEnvironment(raw, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"elevation", "temperature", "aridity", "ocean", "waterDepth"} {
		if !reflect.DeepEqual(view.Fields[field], v.solver.Environment.Fields[field]) {
			t.Fatal("Atlas scalar pixels changed", field)
		}
	}
	if view.Climate != nil || view.Geology != nil || view.Resources != nil || len(view.Fields) != 5 {
		t.Fatal("Atlas retained non-display climate/geology")
	}
	withResources, err := decodeAtlasEnvironment(raw, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(withResources.Resources, v.solver.Environment.Resources) {
		t.Fatal("resource highlighting lost its saved geometry")
	}
}

func BenchmarkAtlasEnvironmentReads(b *testing.B) {
	v := projectFixture(b)
	raw, err := compressJSON(v.solver.Environment)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("full-environment", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var e *terrain.Environment
			if err := expandJSON(raw, &e); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("thumbnail", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := decodeAtlasEnvironment(raw, false); err != nil {
				b.Fatal(err)
			}
		}
	})
}
