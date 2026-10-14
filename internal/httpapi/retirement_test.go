package httpapi

import (
	"bytes"
	"net/http/httptest"
	"testing"
)

func TestLegacyGeneratorEndpointsAreRetired(t *testing.T) {
	s := New(t.TempDir())
	for _, path := range []string{"/api/sessions", "/api/sessions/archived/step", "/api/sessions/archived/paint", "/api/sessions/archived/cleanup", "/api/sessions/archived/restore"} {
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, httptest.NewRequest("POST", path, bytes.NewBufferString(`{}`)))
		if rec.Code != 410 {
			t.Fatalf("%s still accepts old generator actions: %d", path, rec.Code)
		}
	}
	if len(s.sessions) != 0 {
		t.Fatal("retired endpoint created a session")
	}
}
func TestArchiveDetailReaderNeverGeneratesMissingTiles(t *testing.T) {
	s := New(t.TempDir())
	v := projectFixture(t)
	s.sessions["archive"] = v
	count := len(v.tiles)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest("GET", "/api/sessions/archive/detail/5/20/12", nil))
	if rec.Code != 404 || len(v.tiles) != count {
		t.Fatal("archive reader generated new detail", rec.Code)
	}
}
