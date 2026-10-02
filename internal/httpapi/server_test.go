package httpapi

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"kriemhild/internal/project"
)

func TestHTTPWorkflowAndBoundaries(t *testing.T) {
	web := t.TempDir()
	os.WriteFile(filepath.Join(web, "index.html"), []byte("Next export"), 0600)
	server, e := New(t.TempDir(), web)
	if e != nil {
		t.Fatal(e)
	}
	defer server.Close()
	var cookie *http.Cookie
	request := func(method, path string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "http://127.0.0.1:4780"+path, bytes.NewReader(body))
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		server.ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "/api/v1/projects", nil); w.Code != 401 {
		t.Fatal("unauthenticated API access allowed")
	}
	session := request("GET", "/api/v1/session", nil)
	if session.Code != 200 {
		t.Fatal(session.Body.String())
	}
	cookie = session.Result().Cookies()[0]
	w := request("POST", "/api/v1/projects", []byte(`{"name":"HTTP world","age":"First Age"}`))
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	var state project.State
	json.Unmarshal(w.Body.Bytes(), &state)
	base := "/api/v1/projects/" + state.Root.World.ID
	entity := project.Record{ID: project.NewID(), Kind: "entity", Name: "Test port", Type: "Settlement"}
	c := project.Command{Expected: state.Revision, Age: state.Age.ID, Action: "put", Record: &entity}
	b, _ := json.Marshal(c)
	w = request("POST", base+"/commands", b)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w = request("POST", base+"/commands", b); w.Code != 409 {
		t.Fatal("stale HTTP mutation not rejected")
	}
	w = request("GET", base+"/search?age="+state.Age.ID+"&q=port", nil)
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte("Test port")) {
		t.Fatal("search failed")
	}
	img := image.NewRGBA(image.Rect(0, 0, 8, 6))
	img.Set(0, 0, color.RGBA{100, 150, 50, 255})
	var data bytes.Buffer
	png.Encode(&data, img)
	w = request("POST", base+"/assets", data.Bytes())
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	var upload struct {
		Asset string `json:"asset"`
		Width int    `json:"width"`
	}
	json.Unmarshal(w.Body.Bytes(), &upload)
	if upload.Width != 8 {
		t.Fatal("image metadata missing")
	}
	w = request("GET", base+"/assets/"+upload.Asset, nil)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), data.Bytes()) {
		t.Fatal("asset roundtrip failed")
	}
	if w = request("POST", base+"/assets", []byte(`<svg onload="alert(1)"/>`)); w.Code != 400 {
		t.Fatal("active image accepted")
	}
	for _, path := range []string{"/api/v1/projects/../../other/state", base + "/assets/not-a-hash"} {
		if w = request("GET", path, nil); w.Code == 200 {
			t.Fatal("bad path accepted")
		}
	}
	for _, mode := range []string{"origin", "host", "fetch"} {
		r := httptest.NewRequest("GET", "http://127.0.0.1:4780/api/v1/session", nil)
		switch mode {
		case "origin":
			r.Header.Set("Origin", "https://evil.example")
		case "host":
			r.Host = "evil.example:4780"
		case "fetch":
			r.Header.Set("Sec-Fetch-Site", "cross-site")
		}
		w = httptest.NewRecorder()
		server.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("%s boundary failed", mode)
		}
	}
	w = request("GET", "/?world="+state.Root.World.ID+"&age="+state.Age.ID+"&view=atlas", nil)
	if w.Code != 200 || w.Body.String() != "Next export" {
		t.Fatal("static deep link failed")
	}
}
