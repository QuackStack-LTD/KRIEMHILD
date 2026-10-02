package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPasswordChangesRevokeSessionsAndPersist(t *testing.T) {
	dir := t.TempDir()
	server, e := New(dir, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer server.Close()
	if e = server.EnableHosting("https://world.test", "original-admin-password"); e != nil {
		t.Fatal(e)
	}
	request := func(cookie *http.Cookie, method, path string, body any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(method, "https://world.test/api/v1"+path, bytes.NewReader(b))
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		server.ServeHTTP(w, r)
		return w
	}
	login := func(user, password string) *http.Cookie {
		t.Helper()
		w := request(nil, "POST", "/login", map[string]string{"user": user, "password": password})
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		return w.Result().Cookies()[0]
	}
	admin := login("admin", "original-admin-password")
	if w := request(admin, "POST", "/accounts", map[string]string{"user": "author", "password": "original-author-password"}); w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	author := login("author", "original-author-password")
	other := login("author", "original-author-password")
	if w := request(author, "POST", "/accounts/reset-password", map[string]string{}); w.Code != 403 {
		t.Fatal("non-admin reset allowed")
	}
	if w := request(author, "POST", "/password", map[string]string{"current": "incorrect-password", "password": "replacement-author-password"}); w.Code != 401 {
		t.Fatal("wrong current password accepted")
	}
	if w := request(author, "POST", "/password", map[string]string{"current": "original-author-password", "password": "replacement-author-password"}); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for _, c := range []*http.Cookie{author, other} {
		if w := request(c, "GET", "/projects", nil); w.Code != 401 {
			t.Fatal("session survived password change")
		}
	}
	if w := request(nil, "POST", "/login", map[string]string{"user": "author", "password": "original-author-password"}); w.Code != 401 {
		t.Fatal("old password accepted")
	}
	author = login("author", "replacement-author-password")
	if w := request(admin, "POST", "/accounts/reset-password", map[string]string{"user": "author", "current": "original-admin-password", "password": "reset-author-password"}); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := request(author, "GET", "/projects", nil); w.Code != 401 {
		t.Fatal("reset did not revoke target sessions")
	}
	if w := request(admin, "GET", "/projects", nil); w.Code != 200 {
		t.Fatal("reset revoked administrator")
	}
	server.Close()
	server, e = New(dir, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer server.Close()
	if e = server.EnableHosting("https://world.test", ""); e != nil {
		t.Fatal(e)
	}
	login("author", "reset-author-password")
}

func TestAccountWriterLockAndFailedPasswordSave(t *testing.T) {
	dir := t.TempDir()
	s, e := New(dir, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.EnableHosting("https://world.test", "original-admin-password"); e != nil {
		t.Fatal(e)
	}
	second, e := New(dir, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer second.Close()
	if e = second.EnableHosting("https://world.test", ""); e == nil {
		t.Fatal("two hosted writers accepted")
	}
	a := s.access
	a.sessions["existing-session"] = loginSession{User: "admin"}
	// An existing directory cannot be replaced by the atomic account-file write.
	invalid := filepath.Join(dir, "not-a-file")
	if e = os.Mkdir(invalid, 0700); e != nil {
		t.Fatal(e)
	}
	originalPath := a.path
	a.path = invalid
	before := a.data.Users["admin"]
	body := bytes.NewBufferString(`{"current":"original-admin-password","password":"replacement-admin-password"}`)
	r := httptest.NewRequest("POST", "https://world.test/api/v1/password", body)
	w := httptest.NewRecorder()
	s.changePassword(w, r, "test-cookie", "admin")
	if w.Code == 200 || a.data.Users["admin"] != before || len(a.sessions) != 1 {
		t.Fatal("failed save changed credentials/sessions")
	}
	a.path = originalPath
	s.Close()
	if e = second.EnableHosting("https://world.test", ""); e != nil {
		t.Fatal("lock was not released", e)
	}
}
