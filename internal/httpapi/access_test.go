package httpapi

import (
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"kriemhild/internal/auth"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type accountFixture struct {
	u            storage.User
	secret, csrf string
}

func secureFixture(t *testing.T) (*Server, *storage.Store, accountFixture, accountFixture, *history.Document) {
	t.Helper()
	ctx := context.Background()
	db, e := storage.Open(ctx, t.TempDir(), "")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	newUser := func(sub string) accountFixture {
		u, e := db.ResolveUser(ctx, "https://issuer.example", sub, sub, sub+"@example.test", true)
		if e != nil {
			t.Fatal(e)
		}
		secret, csrf, e := db.NewSession(ctx, u.ID)
		if e != nil {
			t.Fatal(e)
		}
		return accountFixture{u, secret, csrf}
	}
	a, b := newUser("alice"), newUser("bob")
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir(), Auth: &auth.Service{Config: auth.Config{Mode: "oidc", PublicURL: "https://app.example"}, Store: db}})
	d := history.New("Private world")
	if e = db.CreateHistory(storage.WithActor(ctx, a.u.ID), d); e != nil {
		t.Fatal(e)
	}
	return s, db, a, b, d
}
func secured(s *Server, a accountFixture, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if a.secret != "" {
		r.AddCookie(&http.Cookie{Name: "__Host-kriemhild", Value: a.secret})
		r.Header.Set("X-CSRF-Token", a.csrf)
	}
	r.Header.Set("If-Match", "0")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func share(t *testing.T, db *storage.Store, a, b accountFixture, world, role string) {
	t.Helper()
	token, e := db.Invite(storage.WithActor(context.Background(), a.u.ID), world, b.u.Email, role)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.AcceptInvitation(storage.WithActor(context.Background(), b.u.ID), b.u, token); e != nil {
		t.Fatal(e)
	}
}
func TestPrivateWorldRouteIsolation(t *testing.T) {
	s, db, a, b, d := secureFixture(t)
	ctx := storage.WithActor(context.Background(), a.u.ID)
	p := storage.WritingProject{ID: "private-writer", WorldID: d.World.ID, Name: "Novel"}
	if e := db.SaveWriterProject(ctx, &p); e != nil {
		t.Fatal(e)
	}
	s.sessions["private-session"] = &session{creatorID: a.u.ID}
	paths := []struct{ method, path string }{{"GET", "/api/worlds"}, {"GET", "/api/projects"}, {"GET", "/api/worlds/" + d.World.ID}, {"POST", "/api/worlds/" + d.World.ID}, {"DELETE", "/api/worlds/" + d.World.ID}, {"GET", "/api/worlds/" + d.World.ID + "/export"}, {"GET", "/api/worlds/" + d.World.ID + "/members"}, {"GET", "/api/worlds/" + d.World.ID + "/revisions"}, {"GET", "/api/writer/projects/private-writer"}, {"POST", "/api/writer/projects/private-writer/nodes"}, {"GET", "/api/sessions/private-session/world"}, {"DELETE", "/api/sessions/private-session"}, {"GET", "/api/map-context/" + d.World.ID}}
	for _, p := range paths {
		if w := secured(s, accountFixture{}, p.method, p.path, `{}`); w.Code != 401 {
			t.Errorf("anonymous %s %s: %d", p.method, p.path, w.Code)
		}
		w := secured(s, b, p.method, p.path, `{}`)
		if p.path == "/api/worlds" || p.path == "/api/projects" {
			if w.Code != 200 || strings.Contains(w.Body.String(), d.World.Name) {
				t.Errorf("list leaked: %s", w.Body)
			}
		} else if w.Code != 404 {
			t.Errorf("IDOR %s %s: %d %s", p.method, p.path, w.Code, w.Body)
		}
	}
	if w := secured(s, b, "GET", "/api/writer/projects", ""); w.Code != 200 || strings.Contains(w.Body.String(), "Novel") {
		t.Fatal("writer list leaked", w.Body)
	}
	noCSRF := a
	noCSRF.csrf = ""
	if w := secured(s, noCSRF, "POST", "/api/worlds", `{"name":"No"}`); w.Code != 403 {
		t.Fatal("CSRF bypass", w.Code)
	}
	if w := secured(s, b, "POST", "/api/writer/projects", `{"id":"stolen","name":"Stolen","worldId":"`+d.World.ID+`"}`); w.Code != 404 {
		t.Fatal("writer destination bypass", w.Code)
	}
}
func TestRolesInvitationsRevisionsAndRevocation(t *testing.T) {
	s, db, a, b, d := secureFixture(t)
	share(t, db, a, b, d.World.ID, "viewer")
	path := "/api/worlds/" + d.World.ID
	if w := secured(s, b, "GET", path, ""); w.Code != 200 {
		t.Fatal(w.Body)
	}
	for _, p := range []string{path, path + "/library", path + "/child", path + "/invitations"} {
		if w := secured(s, b, "POST", p, `{}`); w.Code != 403 {
			t.Errorf("viewer mutation %s: %d", p, w.Code)
		}
	}
	if e := db.SetMember(storage.WithActor(context.Background(), a.u.ID), d.World.ID, b.u.ID, "editor"); e != nil {
		t.Fatal(e)
	}
	body := `{"kind":"rename-world","name":"Shared world","revision":1}`
	w := secured(s, b, "POST", path, body)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if w = secured(s, a, "POST", path, body); w.Code != 409 {
		t.Fatal("stale operation accepted", w.Code)
	}
	revisions, e := db.Revisions(context.Background(), d.World.ID, 0)
	if e != nil || len(revisions) != 2 || revisions[0].Author != b.u.ID {
		t.Fatal("incorrect revision attribution", revisions, e)
	}
	if w = secured(s, b, "DELETE", path, ""); w.Code != 403 {
		t.Fatal("editor deleted world")
	}
	if e = db.SetMember(storage.WithActor(context.Background(), a.u.ID), d.World.ID, b.u.ID, ""); e != nil {
		t.Fatal(e)
	}
	if w = secured(s, b, "GET", path, ""); w.Code != 404 {
		t.Fatal("revocation did not apply")
	}
	if e = db.SetMember(storage.WithActor(context.Background(), a.u.ID), d.World.ID, a.u.ID, ""); e == nil {
		t.Fatal("last owner removed")
	}
}
func TestInvitationRecipientAndReuse(t *testing.T) {
	_, db, a, b, d := secureFixture(t)
	ctx := storage.WithActor(context.Background(), a.u.ID)
	token, e := db.Invite(ctx, d.World.ID, b.u.Email, "editor")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.AcceptInvitation(ctx, a.u, token); e == nil {
		t.Fatal("wrong recipient accepted")
	}
	unverified := b.u
	unverified.Verified = false
	if _, e = db.AcceptInvitation(ctx, unverified, token); e == nil {
		t.Fatal("unverified recipient accepted")
	}
	ctx = storage.WithActor(context.Background(), b.u.ID)
	if _, e = db.AcceptInvitation(ctx, b.u, token); e != nil {
		t.Fatal(e)
	}
	if _, e = db.AcceptInvitation(ctx, b.u, token); e == nil {
		t.Fatal("token reused")
	}
}
func TestLiveAccessIsRevoked(t *testing.T) {
	s, db, a, b, d := secureFixture(t)
	share(t, db, a, b, d.World.ID, "editor")
	server := httptest.NewServer(s)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	headers := http.Header{}
	headers.Set("Cookie", "__Host-kriemhild="+b.secret)
	conn, _, e := websocket.Dial(ctx, strings.Replace(server.URL, "http:", "ws:", 1)+"/api/collaboration/worlds/"+d.World.ID, &websocket.DialOptions{HTTPHeader: headers})
	if e != nil {
		t.Fatal(e)
	}
	defer conn.CloseNow()
	var event map[string]any
	if e = wsjson.Read(ctx, conn, &event); e != nil {
		t.Fatal(e)
	}
	w := secured(s, a, "DELETE", "/api/worlds/"+d.World.ID+"/members/"+b.u.ID, "")
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	for i := 0; i < 4; i++ {
		if e = wsjson.Read(ctx, conn, &event); e != nil {
			return
		}
	}
	t.Fatal("revoked socket remained open")
}
func TestMapCommandsRequireFreshRevision(t *testing.T) {
	s, db, a, b, d := secureFixture(t)
	v := projectFixture(t)
	v.worldID = history.ID()
	v.creatorID = a.u.ID
	v.actorID = a.u.ID
	s.sessions["map"] = v
	if e := s.saveWorld(v, true); e != nil {
		t.Fatal(e)
	}
	share(t, db, a, b, v.worldID, "editor")
	path := "/api/sessions/map/world"
	payload, _ := json.Marshal(map[string]any{"kind": "metadata", "revision": v.authored().Header.Revision, "name": "Map edit"})
	w := secured(s, a, "POST", path, string(payload))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	w = secured(s, b, "POST", path, string(payload))
	if w.Code != 409 {
		t.Fatal("stale map accepted", w.Code, w.Body)
	}
	if v.collabRevision != 1 {
		t.Fatal(v.collabRevision)
	}
	_ = d
}
