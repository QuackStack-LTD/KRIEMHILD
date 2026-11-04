package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"kriemhild/internal/world"
	"mime/multipart"
	"net/http/httptest"
	"testing"
)

func TestIllustratedMapEndpointRevisionAndSafeBanner(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	v := projectFixture(t)
	s.sessions["map"] = v
	d := history.New("Saved maps")
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"session": "map", "ageId": d.World.CurrentAge, "name": "Geography"})
	autoRequest(t, s, "POST", "/api/worlds/"+d.World.ID+"/accept", string(body))
	art := world.Illustration{Version: 1, ID: "medieval-world", Title: "World atlas", Scope: "world", Style: "medieval", Bounds: [4]float64{0, 0, 2, 1}, Layers: []world.IllustrationLayer{{ID: "labels", Name: "Labels", Visible: true, Opacity: 1}}, Objects: map[string]world.IllustrationObject{}}
	if err = v.authored().PutIllustration(art); err != nil {
		t.Fatal(err)
	}
	var imageData bytes.Buffer
	png.Encode(&imageData, image.NewNRGBA(image.Rect(0, 0, 16, 16)))
	upload := func(revision int, design world.Illustration) *httptest.ResponseRecorder {
		t.Helper()
		var b bytes.Buffer
		form := multipart.NewWriter(&b)
		meta, _ := json.Marshal(map[string]any{"sourceRevision": revision, "illustration": design})
		form.WriteField("metadata", string(meta))
		file, _ := form.CreateFormFile("file", "map.png")
		file.Write(imageData.Bytes())
		form.Close()
		r := httptest.NewRequest("POST", "/api/sessions/map/illustrated-maps", &b)
		r.Header.Set("Content-Type", form.FormDataContentType())
		rr := httptest.NewRecorder()
		s.ServeHTTP(rr, r)
		return rr
	}
	if r := upload(v.authored().Header.Revision-1, art); r.Code != 409 {
		t.Fatal("stale render accepted", r.Code, r.Body)
	}
	modified := art
	modified.Title = "Unsaved title"
	if r := upload(v.authored().Header.Revision, modified); r.Code != 409 {
		t.Fatal("unsaved design accepted", r.Code, r.Body)
	}
	r := upload(v.authored().Header.Revision, art)
	if r.Code != 201 {
		t.Fatal(r.Code, r.Body)
	}
	var result struct {
		FileID    string `json:"fileId"`
		ArchiveID string `json:"archiveId"`
	}
	json.Unmarshal(r.Body.Bytes(), &result)
	if result.FileID == "" || result.ArchiveID == "" {
		t.Fatal("missing saved map identity")
	}
	endpoint := "/api/worlds/" + d.World.ID + "/library/" + result.FileID + "?age=" + d.World.CurrentAge
	for _, suffix := range []string{"", "&inline=1"} {
		rr := httptest.NewRecorder()
		s.ServeHTTP(rr, httptest.NewRequest("GET", endpoint+suffix, nil))
		if rr.Code != 200 || !bytes.Equal(rr.Body.Bytes(), imageData.Bytes()) || rr.Header().Get("Content-Type") != "image/png" {
			t.Fatal("saved PNG unavailable", rr.Code, rr.Body)
		}
		expected := "attachment"
		if suffix != "" {
			expected = "inline"
		}
		if !bytes.HasPrefix([]byte(rr.Header().Get("Content-Disposition")), []byte(expected)) {
			t.Fatal("wrong download disposition", rr.Header())
		}
	}
}
