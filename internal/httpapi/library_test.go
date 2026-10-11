package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"mime/multipart"
	"net/http/httptest"
	"testing"
)

func TestLibraryUploadDownloadAndWorldZIP(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	d := history.New("Portable archive")
	age := d.World.CurrentAge
	e := history.Entity{ID: "archive", Name: "Archive", Kind: "archive"}
	if err = d.Apply(history.Command{Kind: "entity", AgeID: age, Entity: &e}); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	server := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	content := []byte{0, 1, 255, 254, 17, 22}
	body := new(bytes.Buffer)
	form := multipart.NewWriter(body)
	meta, _ := json.Marshal(map[string]any{"revision": d.World.Revision, "archiveId": "archive", "entity": history.Entity{ID: "file", AgeID: age, Name: "Seal", Kind: "certificate", Properties: json.RawMessage(`{}`)}})
	form.WriteField("metadata", string(meta))
	f, _ := form.CreateFormFile("file", "seal.bin")
	f.Write(content)
	form.Close()
	req := httptest.NewRequest("POST", "/api/worlds/"+d.World.ID+"/library", body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("upload: %d %s", rr.Code, rr.Body)
	}
	get := func(s *Server, path string) *httptest.ResponseRecorder {
		r := httptest.NewRecorder()
		s.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		return r
	}
	endpoint := "/api/worlds/" + d.World.ID + "/library/file?age=" + age
	r := get(server, endpoint)
	if r.Code != 200 || !bytes.Equal(r.Body.Bytes(), content) || r.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("download changed bytes or lacks safe content headers")
	}
	if r = get(server, endpoint+"&version=unrelated"); r.Code != 404 {
		t.Fatal("arbitrary blob exposed")
	}
	if r = get(server, "/api/worlds/"+d.World.ID+"/library/file?age=other"); r.Code != 404 {
		t.Fatal("file leaked across Age")
	}
	archive := get(server, "/api/worlds/"+d.World.ID+"/export")
	if archive.Code != 200 {
		t.Fatal(archive.Body.String())
	}
	portable, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer portable.Close()
	destination := NewWithOptions(t.TempDir(), Options{Projects: portable, CacheDir: t.TempDir()})
	imported := httptest.NewRecorder()
	destination.ServeHTTP(imported, httptest.NewRequest("POST", "/api/worlds/import", bytes.NewReader(archive.Body.Bytes())))
	if imported.Code != 200 && imported.Code != 201 {
		t.Fatalf("import: %d %s", imported.Code, imported.Body)
	}
	r = get(destination, endpoint)
	if r.Code != 200 || !bytes.Equal(r.Body.Bytes(), content) {
		t.Fatal("World ZIP did not restore attachment")
	}
}
