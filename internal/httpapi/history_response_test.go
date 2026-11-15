package httpapi

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHistoricalAgeResponseCompressionPreservesFullDocument(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	d := history.New("Compressed World")
	first := d.World.CurrentAge
	d.Entities = []history.Entity{{ID: "person", AgeID: first, Name: "Historian", Kind: "character", Properties: json.RawMessage(`{"summary":"A recorded life"}`)}}
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	s := NewWithOptions(t.TempDir(), Options{Projects: db})
	r := httptest.NewRequest("POST", "/api/worlds/"+d.World.ID, strings.NewReader(`{"kind":"age","ageId":"`+first+`","name":"Later","mode":"after","revision":1}`))
	r.Header.Set("Accept-Encoding", "gzip, deflate")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 200 || w.Header().Get("Content-Encoding") != "gzip" || w.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatal("missing negotiated compressed response", w.Code, w.Header())
	}
	reader, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	var result history.Document
	if err = json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Ages) != 2 || len(result.Entities) != 2 || result.Entity(first, "person").Name != "Historian" || result.Entity(result.World.CurrentAge, "person").Name != "Historian" {
		t.Fatal("compressed response lost historical records")
	}
	// Clients that do not negotiate compression retain the existing JSON API.
	w = httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("GET", "/api/worlds/"+d.World.ID, nil))
	if w.Code != 200 || w.Header().Get("Content-Encoding") != "" || !json.Valid(w.Body.Bytes()) {
		t.Fatal("uncompressed compatibility changed")
	}
}

func TestHistoryCompressionRespectsEncodingPreferences(t *testing.T) {
	for _, c := range []struct {
		header string
		want   bool
	}{
		{"", false}, {"br", false}, {"gzip, deflate", true}, {"GZIP;q=0.5", true}, {"gzip;q=0", false}, {"*;q=1", true}, {"gzip;q=0, *;q=1", false}, {"*;q=1, gzip;q=0", false}, {"gzip;q=invalid", false},
	} {
		if got := acceptsHistoryGzip(c.header); got != c.want {
			t.Errorf("%q: got %v, want %v", c.header, got, c.want)
		}
	}
}
