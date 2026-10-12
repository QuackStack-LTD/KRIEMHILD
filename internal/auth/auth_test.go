package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/coreos/go-oidc/v3/oidc"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"kriemhild/internal/storage"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestOIDCCodePKCESessionAndLogout(t *testing.T) {
	db, e := storage.Open(context.Background(), t.TempDir(), "")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	var issuer, nonce, challenge string
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys", "end_session_endpoint": issuer + "/logout", "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/keys":
			json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "key", Algorithm: "RS256", Use: "sig"}}})
		case "/token":
			r.ParseForm()
			hash := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if r.Form.Get("code") != "valid-code" || base64.RawURLEncoding.EncodeToString(hash[:]) != challenge {
				http.Error(w, "invalid code or PKCE", 400)
				return
			}
			signer, _ := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key, KeyID: "key"}}, nil)
			token, _ := jwt.Signed(signer).Claims(map[string]any{"iss": issuer, "sub": "immutable-subject", "aud": "client", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": nonce, "name": "Writer", "email": "writer@example.test", "email_verified": true}).Serialize()
			json.NewEncoder(w).Encode(map[string]any{"access_token": "secret-provider-token", "token_type": "Bearer", "id_token": token})
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()
	issuer = provider.URL
	ctx := oidc.ClientContext(context.Background(), provider.Client())
	s, e := New(ctx, Config{Mode: "oidc", Issuer: issuer, ClientID: "client", PublicURL: "https://app.example"}, db, "0.0.0.0:8124")
	if e != nil {
		t.Fatal(e)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("GET", "https://app.example/auth/login", nil))
	location, e := url.Parse(w.Header().Get("Location"))
	if e != nil {
		t.Fatal(e)
	}
	q := location.Query()
	nonce = q.Get("nonce")
	challenge = q.Get("code_challenge")
	if q.Get("code_challenge_method") != "S256" || nonce == "" {
		t.Fatal("PKCE or nonce missing")
	}
	flowCookie := w.Result().Cookies()[0]
	if !flowCookie.Secure || !flowCookie.HttpOnly {
		t.Fatal("insecure login cookie")
	}
	callback := httptest.NewRequest("GET", "https://app.example/auth/callback?state="+q.Get("state")+"&code=valid-code", nil).WithContext(ctx)
	callback.AddCookie(flowCookie)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, callback)
	if w.Code != 303 {
		t.Fatal(w.Code, w.Body)
	}
	var session *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "__Host-kriemhild" {
			session = c
		}
	}
	if session == nil || !session.Secure || !session.HttpOnly || session.SameSite != http.SameSiteLaxMode {
		t.Fatal("session cookie missing or insecure")
	}
	u, csrf, e := db.Session(ctx, session.Value)
	if e != nil || u.Subject != "immutable-subject" || !u.Verified {
		t.Fatal(u, e)
	}
	w = httptest.NewRecorder()
	s.ServeHTTP(w, callback)
	if w.Code != 400 {
		t.Fatal("callback state replay accepted")
	}
	logout := httptest.NewRequest("POST", "https://app.example/auth/logout", nil)
	logout.AddCookie(session)
	logout.Header.Set("X-CSRF-Token", csrf)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, logout)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if _, _, e = db.Session(ctx, session.Value); e == nil {
		t.Fatal("logout did not revoke session")
	}
}
func TestLocalModeCannotListenPublicly(t *testing.T) {
	db, e := storage.Open(context.Background(), t.TempDir(), "")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	for _, addr := range []string{"0.0.0.0:8124", ":8124", "[::]:8124"} {
		if _, e = New(context.Background(), Config{Mode: "local"}, db, addr); e == nil {
			t.Fatal("public local mode allowed", addr)
		}
	}
	if _, e = New(context.Background(), Config{Mode: "local"}, db, "127.0.0.1:8124"); e != nil {
		t.Fatal(e)
	}
}
