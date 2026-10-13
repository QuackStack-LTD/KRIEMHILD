// Package auth delegates identity lifecycle to OIDC and stores only revocable
// application sessions. Provider credentials never reach browser JavaScript.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"kriemhild/internal/storage"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Config struct{ Mode, Issuer, ClientID, ClientSecret, PublicURL, LegacySubject string }
type flow struct {
	Nonce, Verifier string
	Until           time.Time
}
type Service struct {
	Config   Config
	Store    *storage.Store
	Local    storage.User
	provider *oidc.Provider
	oauth    oauth2.Config
	logout   string
	mu       sync.Mutex
	flows    map[string]flow
}
type identityKey struct{}

func Identity(ctx context.Context) storage.User {
	u, _ := ctx.Value(identityKey{}).(storage.User)
	return u
}
func Context(ctx context.Context, u storage.User) context.Context {
	return context.WithValue(storage.WithActor(ctx, u.ID), identityKey{}, u)
}
func random() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func New(ctx context.Context, c Config, store *storage.Store, listen string) (*Service, error) {
	if c.Mode == "" {
		c.Mode = "local"
	}
	s := &Service{Config: c, Store: store, flows: map[string]flow{}}
	if c.Mode == "local" {
		host, _, e := net.SplitHostPort(listen)
		ip := net.ParseIP(host)
		if e != nil || ip == nil || !ip.IsLoopback() {
			return nil, fmt.Errorf("local authentication mode requires a loopback listen address; configure OIDC for hosted access")
		}
		u, e := store.ResolveUser(ctx, "kriemhild:local", "local", "Local author", "", false)
		if e != nil {
			return nil, e
		}
		s.Local = u
		return s, store.AssignLegacy(ctx, u.ID)
	}
	if c.Mode != "oidc" {
		return nil, fmt.Errorf("KRIEMHILD_AUTH_MODE must be local or oidc")
	}
	for _, v := range []string{c.PublicURL, c.Issuer} {
		u, e := url.Parse(v)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("OIDC issuer and public URL must be HTTPS URLs")
		}
	}
	u, _ := url.Parse(c.PublicURL)
	if u.Path != "" && u.Path != "/" {
		return nil, fmt.Errorf("public URL must be an origin without a path")
	}
	if c.ClientID == "" {
		return nil, fmt.Errorf("OIDC client ID is required")
	}
	p, e := oidc.NewProvider(ctx, c.Issuer)
	if e != nil {
		return nil, fmt.Errorf("OIDC discovery failed: %w", e)
	}
	s.provider = p
	if e = store.LeaseHosted(ctx); e != nil {
		return nil, e
	}
	s.oauth = oauth2.Config{ClientID: c.ClientID, ClientSecret: c.ClientSecret, Endpoint: p.Endpoint(), RedirectURL: strings.TrimRight(c.PublicURL, "/") + "/auth/callback", Scopes: []string{oidc.ScopeOpenID, "profile", "email"}}
	var meta struct {
		Logout string `json:"end_session_endpoint"`
	}
	if e = p.Claims(&meta); e != nil {
		return nil, e
	}
	s.logout = meta.Logout
	return s, nil
}
func (s *Service) cookie(w http.ResponseWriter, name, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: s.Config.Mode == "oidc", SameSite: http.SameSiteLaxMode, MaxAge: age})
}
func (s *Service) User(r *http.Request) (storage.User, string, error) {
	if s.Config.Mode == "local" {
		return s.Local, "", nil
	}
	c, e := r.Cookie("__Host-kriemhild")
	if e != nil {
		return storage.User{}, "", e
	}
	return s.Store.Session(r.Context(), c.Value)
}
func (s *Service) ValidCSRF(r *http.Request, csrf string) bool {
	return s.Config.Mode == "local" || csrf != "" && subtle.ConstantTimeCompare([]byte(csrf), []byte(r.Header.Get("X-CSRF-Token"))) == 1
}
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if s.Config.Mode == "local" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	switch r.URL.Path {
	case "/auth/login":
		if r.Method != "GET" {
			http.Error(w, "method not allowed", 405)
			return
		}
		state, nonce, verifier := random(), random(), oauth2.GenerateVerifier()
		s.mu.Lock()
		for k, v := range s.flows {
			if time.Now().After(v.Until) {
				delete(s.flows, k)
			}
		}
		if len(s.flows) >= 1000 {
			s.mu.Unlock()
			http.Error(w, "login busy", 429)
			return
		}
		s.flows[storage.TokenHash(state)] = flow{nonce, verifier, time.Now().Add(5 * time.Minute)}
		s.mu.Unlock()
		s.cookie(w, "__Host-kriemhild-login", state, 300)
		http.Redirect(w, r, s.oauth.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), 302)
	case "/auth/callback":
		if r.Method != "GET" {
			http.Error(w, "method not allowed", 405)
			return
		}
		state := r.URL.Query().Get("state")
		c, e := r.Cookie("__Host-kriemhild-login")
		if e != nil || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(c.Value)) != 1 {
			http.Error(w, "invalid login state", 400)
			return
		}
		s.mu.Lock()
		f, ok := s.flows[storage.TokenHash(state)]
		delete(s.flows, storage.TokenHash(state))
		s.mu.Unlock()
		s.cookie(w, "__Host-kriemhild-login", "", -1)
		if !ok || time.Now().After(f.Until) {
			http.Error(w, "login expired", 400)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		t, e := s.oauth.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(f.Verifier))
		if e != nil {
			http.Error(w, "login exchange failed", 401)
			return
		}
		raw, _ := t.Extra("id_token").(string)
		id, e := s.provider.Verifier(&oidc.Config{ClientID: s.Config.ClientID}).Verify(ctx, raw)
		if e != nil || id.Nonce != f.Nonce || id.Subject == "" {
			http.Error(w, "invalid identity", 401)
			return
		}
		var claims struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Verified bool   `json:"email_verified"`
		}
		if e = id.Claims(&claims); e != nil {
			http.Error(w, "invalid identity claims", 401)
			return
		}
		if claims.Name == "" {
			claims.Name = "Author"
		}
		u, e := s.Store.ResolveUser(ctx, id.Issuer, id.Subject, claims.Name, claims.Email, claims.Verified)
		if errors.Is(e, storage.ErrAccountClosed) {
			http.Error(w, "This KRIEMHILD account is closed. Contact the administrator to discuss recovery.", 403)
			return
		}
		if e != nil {
			http.Error(w, "account unavailable", 503)
			return
		}
		if s.Config.LegacySubject != "" && id.Subject == s.Config.LegacySubject {
			if e = s.Store.AssignLegacy(ctx, u.ID); e != nil {
				http.Error(w, "legacy migration failed", 503)
				return
			}
		}
		secret, _, e := s.Store.NewSession(ctx, u.ID)
		if e != nil {
			http.Error(w, "session unavailable", 503)
			return
		}
		s.cookie(w, "__Host-kriemhild", secret, 12*60*60)
		http.Redirect(w, r, "/", 303)
	case "/auth/logout":
		_, csrf, e := s.User(r)
		if r.Method != "POST" || e != nil || !s.ValidCSRF(r, csrf) {
			http.Error(w, "invalid sign-out request", 403)
			return
		}
		c, _ := r.Cookie("__Host-kriemhild")
		if e = s.Store.RevokeSession(r.Context(), c.Value); e != nil {
			http.Error(w, "sign-out unavailable", 503)
			return
		}
		s.cookie(w, "__Host-kriemhild", "", -1)
		target := "/"
		if s.logout != "" {
			u, e := url.Parse(s.logout)
			if e == nil && u.Scheme == "https" {
				q := u.Query()
				q.Set("client_id", s.Config.ClientID)
				q.Set("post_logout_redirect_uri", strings.TrimRight(s.Config.PublicURL, "/")+"/")
				u.RawQuery = q.Encode()
				target = u.String()
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"redirect": target})
	default:
		http.NotFound(w, r)
	}
}
