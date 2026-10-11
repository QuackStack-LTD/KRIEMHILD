package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestYearTimelineRestartAndWorldZIP(t *testing.T) {
	if !validHistoryRoute("/worlds/0123456789abcdef0123456789abcdef/years") {
		t.Fatal("year timeline cannot be opened directly")
	}
	ctx, dir := context.Background(), t.TempDir()
	db, err := storage.Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	d := history.New("Chronicle")
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	a := d.World.CurrentAge
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	for _, id := range []string{"ada", "ben"} {
		body, _ := json.Marshal(history.Command{Kind: "entity", AgeID: a, Revision: d.World.Revision, Entity: &history.Entity{ID: id, Name: id, Kind: "character", Properties: json.RawMessage(`{"born":"42","died":"81","summary":"A life by the river"}`)}})
		autoRequest(t, s, "POST", "/api/worlds/"+d.World.ID, string(body))
		d, err = db.History(ctx, d.World.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(d.Years) != 2 || len(d.Year(a, "year/42").Events) != 2 {
		t.Fatal("profile save did not persist grouped events")
	}
	body, _ := json.Marshal(history.Command{Kind: "age", AgeID: a, Revision: d.World.Revision, Name: "After the crossing", Mode: "after", SourceYearID: "year/42"})
	autoRequest(t, s, "POST", "/api/worlds/"+d.World.ID, string(body))
	d, err = db.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantYears, wantOrigin := d.Years, d.Age(d.World.CurrentAge).StartedFromYear
	db.Close()
	db, err = storage.Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	loaded, err := db.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Years, wantYears) {
		t.Fatal("database changed year events")
	}
	s = NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	export := httptest.NewRecorder()
	s.ServeHTTP(export, httptest.NewRequest("GET", "/api/worlds/"+d.World.ID+"/export", nil))
	if export.Code != 200 {
		t.Fatal(export.Body.String())
	}
	clean, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer clean.Close()
	other := NewWithOptions(t.TempDir(), Options{Projects: clean, CacheDir: t.TempDir()})
	response := httptest.NewRecorder()
	other.ServeHTTP(response, httptest.NewRequest("POST", "/api/worlds/import", bytes.NewReader(export.Body.Bytes())))
	if response.Code != 201 {
		t.Fatal(response.Code, response.Body.String())
	}
	fresh, err := clean.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fresh.Years, wantYears) || !reflect.DeepEqual(fresh.Age(fresh.World.CurrentAge).StartedFromYear, wantOrigin) {
		t.Fatal("archive lost year timeline or Age origin")
	}
}
