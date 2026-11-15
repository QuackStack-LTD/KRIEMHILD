package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriterArchiveAuthorizationFilesAndInworldProjection(t *testing.T) {
	s, db, a, b, d := secureFixture(t)
	ctx := storage.WithActor(context.Background(), a.u.ID)
	base := "/api/worlds/" + d.World.ID + "/writer-archive"
	for _, path := range []string{base, base + "/inworld"} {
		if rr := secured(s, b, "GET", path, ""); rr.Code != 404 {
			t.Fatal("archive leaked", rr.Code)
		}
	}
	if rr := secured(s, a, "POST", base, "{}"); rr.Code != 200 {
		t.Fatal(rr.Code, rr.Body)
	}
	p, err := db.EnsureWriterArchive(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	share(t, db, a, b, d.World.ID, "viewer")
	if rr := secured(s, b, "GET", base, ""); rr.Code != 200 {
		t.Fatal(rr.Code, rr.Body)
	}
	for _, path := range []string{base, base + "/actions", "/api/writer/projects/" + p.ID + "/files"} {
		if rr := secured(s, b, "POST", path, "{}"); rr.Code != 403 {
			t.Fatal("viewer wrote archive", path, rr.Code)
		}
	}
	n := storage.WritingNode{ID: "image", ProjectID: p.ID, Type: "asset", Title: "Map.png", Schema: 1, Status: "Idea", Content: json.RawMessage(`{"type":"doc","content":[{"type":"paragraph"}]}`)}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	meta, _ := json.Marshal(n)
	form.WriteField("metadata", string(meta))
	part, _ := form.CreateFormFile("file", "Map.png")
	payload := []byte{0, 1, 255, 22}
	part.Write(payload)
	form.Close()
	req := httptest.NewRequest("POST", "/api/writer/projects/"+p.ID+"/files", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "__Host-kriemhild", Value: a.secret})
	req.Header.Set("X-CSRF-Token", a.csrf)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatal(rr.Code, rr.Body)
	}
	path := "/api/writer/projects/" + p.ID + "/files/image"
	rr = secured(s, b, "GET", path, "")
	if rr.Code != 200 || !bytes.Equal(rr.Body.Bytes(), payload) || rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("binary bytes changed or unsafe headers", rr.Code, rr.Body)
	}
	if rr = secured(s, b, "GET", path+"?version=unrelated", ""); rr.Code != 404 {
		t.Fatal("unrelated file version exposed")
	}
	if rr = secured(s, a, "GET", base+"/inworld", ""); rr.Code != 200 {
		t.Fatal(rr.Code, rr.Body)
	} else {
		var projection history.Document
		if json.Unmarshal(rr.Body.Bytes(), &projection) != nil || len(projection.Ages) == 0 || len(projection.Biospheres) != 0 {
			t.Fatal("incorrect archive projection")
		}
	}
}

func TestArchivePDFModuleWorkerMIME(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pdf.worker.mjs"), []byte("export const ready = true;"), 0600); err != nil {
		t.Fatal(err)
	}
	s := NewWithOptions(dir, Options{CacheDir: t.TempDir()})
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, httptest.NewRequest("GET", "/pdf.worker.mjs", nil))
	if rr.Code != 200 || !strings.HasPrefix(rr.Header().Get("Content-Type"), "application/javascript") {
		t.Fatal("module worker cannot load", rr.Code, rr.Header())
	}
}
