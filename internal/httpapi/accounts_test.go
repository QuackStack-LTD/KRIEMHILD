package httpapi

import (
	"context"
	"encoding/json"
	"kriemhild/internal/storage"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAccountRoutesEnforceClosureAndSessionRevocation(t *testing.T) {
	s, db, a, b, d := secureFixture(t)
	share(t, db, a, b, d.World.ID, "editor")
	for _, path := range []string{"/api/account", "/api/worlds/" + d.World.ID + "/activity", "/api/worlds/" + d.World.ID + "/invitations"} {
		if w := secured(s, accountFixture{}, "GET", path, ""); w.Code != 401 {
			t.Fatal(path, w.Code)
		}
	}
	for _, path := range []string{"/activity", "/invitations"} {
		if w := secured(s, b, "GET", "/api/worlds/"+d.World.ID+path, ""); w.Code != 403 {
			t.Fatal("editor saw administration data", w.Code)
		}
	}
	if w := secured(s, a, "DELETE", "/api/account", `{"confirmation":"CLOSE"}`); w.Code != 409 {
		t.Fatal("owner closure", w.Code, w.Body)
	}
	if w := secured(s, b, "POST", "/api/worlds/"+d.World.ID+"/visit", `{}`); w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	if w := secured(s, b, "POST", "/api/account/sessions/revoke", `{}`); w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	if w := secured(s, b, "GET", "/api/worlds/"+d.World.ID, ""); w.Code != 401 {
		t.Fatal("revoked session still accepted", w.Code)
	}
	var err error
	b.secret, b.csrf, err = db.NewSession(context.Background(), b.u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if w := secured(s, b, "DELETE", "/api/account", `{"confirmation":"CLOSE"}`); w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	if w := secured(s, a, "GET", "/api/worlds/"+d.World.ID, ""); w.Code != 200 {
		t.Fatal("member closure deleted shared world", w.Code)
	}
	if _, err = db.ResolveUser(context.Background(), b.u.Issuer, b.u.Subject, b.u.Name, b.u.Email, true); err != storage.ErrAccountClosed {
		t.Fatal("account closure undone", err)
	}
}
func TestInvitationAdministrationAndActivity(t *testing.T) {
	s, _, a, b, d := secureFixture(t)
	base := "/api/worlds/" + d.World.ID
	w := secured(s, a, "POST", base+"/invitations", `{"email":"bob@example.test","role":"viewer"}`)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	var created map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	w = secured(s, a, "GET", base+"/invitations", "")
	var pending []storage.Invitation
	if err := json.Unmarshal(w.Body.Bytes(), &pending); err != nil || len(pending) != 1 {
		t.Fatal(w.Body, err)
	}
	if w = secured(s, a, "DELETE", base+"/invitations/"+pending[0].ID, ""); w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	if w = secured(s, b, "POST", "/api/invitations/accept", `{"token":"`+created["token"]+`"}`); w.Code != 400 {
		t.Fatal("cancelled token granted access", w.Code)
	}
	w = secured(s, a, "GET", base+"/activity", "")
	var events []storage.AuditEvent
	if err := json.Unmarshal(w.Body.Bytes(), &events); err != nil || len(events) != 2 {
		t.Fatal(w.Body, err)
	}
}
func TestRateLimitsRefillAndCannotTrustSpoofedForwarding(t *testing.T) {
	var limits requestLimits
	now := time.Now()
	for i := 0; i < 4; i++ {
		if !limits.allow("a", 4, time.Minute, now) {
			t.Fatal("early limit")
		}
	}
	if limits.allow("a", 4, time.Minute, now) {
		t.Fatal("burst not limited")
	}
	if !limits.allow("b", 4, time.Minute, now) || !limits.allow("a", 4, time.Minute, now.Add(15*time.Second)) {
		t.Fatal("independent/refilled limit")
	}
	s, _, _, _, _ := secureFixture(t)
	for i := 0; i < 5; i++ {
		r := httptest.NewRequest(http.MethodPost, "/api/sessions", nil)
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("X-Forwarded-For", time.Now().String())
		w := httptest.NewRecorder()
		allowed := s.limitRequest(w, r, "")
		if (i < 4) != allowed {
			t.Fatal("spoofed forwarding bypassed limit", i)
		}
		if !allowed && (w.Code != 429 || w.Header().Get("Retry-After") == "") {
			t.Fatal("missing rate limit response")
		}
	}
}

func TestQueuedMutationRevalidatesRevokedSession(t *testing.T) {
	s, db, a, _, _ := secureFixture(t)
	s.accessMu.Lock()
	result := make(chan *httptest.ResponseRecorder, 1)
	go func() { result <- secured(s, a, "POST", "/api/worlds", `{"name":"Must not be created"}`) }()
	// The limiter runs after authentication and before the world lock. Wait
	// until this request is authenticated but unable to mutate anything.
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.limits.mu.Lock()
		_, waiting := s.limits.buckets["api:"+a.u.ID]
		s.limits.mu.Unlock()
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			s.accessMu.Unlock()
			t.Fatal("request did not reach lock")
		}
		time.Sleep(time.Millisecond)
	}
	if err := db.RevokeUserSessions(storage.WithActor(context.Background(), a.u.ID)); err != nil {
		s.accessMu.Unlock()
		t.Fatal(err)
	}
	s.accessMu.Unlock()
	select {
	case w := <-result:
		if w.Code != 401 {
			t.Fatal("queued mutation bypassed revocation", w.Code, w.Body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("queued request did not finish")
	}
}
