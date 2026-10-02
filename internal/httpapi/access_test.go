package httpapi

import (
	"bytes"
	"encoding/json"
	"kriemhild/internal/project"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHostedAuthenticationAndServerPermissions(t *testing.T) {
	server, e := New(t.TempDir(), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer server.Close()
	if e = server.EnableHosting("https://world.test", "correct-horse-example"); e != nil {
		t.Fatal(e)
	}
	var cookie *http.Cookie
	request := func(method, path string, body any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(method, "https://world.test"+path, bytes.NewReader(b))
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		server.ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "/api/v1/projects", nil); w.Code != 401 {
		t.Fatal("anonymous access", w.Code)
	}
	w := request("POST", "/api/v1/login", map[string]string{"user": "admin", "password": "correct-horse-example"})
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	admin := w.Result().Cookies()[0]
	cookie = admin
	if !cookie.Secure || !cookie.HttpOnly {
		t.Fatal("insecure hosted cookie")
	}
	w = request("POST", "/api/v1/accounts", map[string]string{"user": "reader", "password": "a-long-reader-password"})
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	w = request("POST", "/api/v1/projects", map[string]string{"name": "Private world", "age": "First"})
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	var state project.State
	json.Unmarshal(w.Body.Bytes(), &state)
	base := "/api/v1/projects/" + state.Root.World.ID
	cookie = nil
	w = request("POST", "/api/v1/login", map[string]string{"user": "reader", "password": "a-long-reader-password"})
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	reader := w.Result().Cookies()[0]
	cookie = reader
	if w = request("GET", base+"/state", nil); w.Code != 404 {
		t.Fatal("nonmember read world", w.Code)
	}
	if w = request("GET", "/api/v1/projects", nil); w.Body.String() != "[]\n" {
		t.Fatal("private world listed")
	}
	cookie = admin
	if w = request("POST", base+"/members", map[string]string{"user": "reader", "role": "viewer"}); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	cookie = reader
	if w = request("GET", base+"/state", nil); w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(`"readOnly":true`)) {
		t.Fatal("viewer could not read")
	}
	if w = request("POST", base+"/commands", map[string]any{"action": "rename-age", "name": "Hacked", "age": state.Age.ID, "expected": state.Revision}); w.Code != 403 {
		t.Fatal("viewer wrote world", w.Code)
	}
	if w = request("GET", base+"/repository", nil); w.Code != 403 {
		t.Fatal("viewer accessed repository")
	}
	if w = request("POST", base+"/restore", map[string]any{"expected": state.Revision, "revision": state.Revision, "apply": true}); w.Code != 403 {
		t.Fatal("viewer accessed restoration")
	}
	if w = request("POST", "/api/v1/clone-preview", map[string]string{"remote": "https://example.invalid/world"}); w.Code != 403 {
		t.Fatal("non-admin attempted hosted clone")
	}
	if w = request("POST", "/api/v1/accounts", map[string]string{"user": "attacker", "password": "long-password-value"}); w.Code != 403 {
		t.Fatal("nonadmin created account")
	}
	cookie = admin
	if w = request("POST", base+"/members", map[string]string{"user": "reader", "role": "editor"}); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	cookie = reader
	if w = request("POST", base+"/commands", map[string]any{"action": "rename-age", "name": "Authored", "age": state.Age.ID, "expected": state.Revision}); w.Code != 200 {
		t.Fatal("editor unable to write", w.Body.String())
	}
	cookie = admin
	request("POST", base+"/members", map[string]string{"user": "reader", "role": "remove"})
	cookie = reader
	if w = request("GET", base+"/state", nil); w.Code != 404 {
		t.Fatal("revocation not enforced")
	}
}
