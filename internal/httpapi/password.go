package httpapi

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"time"
)

func passwordMatches(acct account, password string) bool {
	if len(password) > 512 {
		return false
	}
	salt, e := base64.StdEncoding.DecodeString(acct.Salt)
	if e != nil {
		return false
	}
	want, e := base64.StdEncoding.DecodeString(acct.Hash)
	if e != nil {
		return false
	}
	key, e := pbkdf2.Key(sha256.New, password, salt, 600000, 32)
	return e == nil && subtle.ConstantTimeCompare(key, want) == 1
}

// Both password operations require reauthentication. Saving precedes session
// revocation; failed disk writes leave credentials and sessions unchanged.
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request, cookieName, user string) bool {
	reset := r.URL.Path == "/api/v1/accounts/reset-password"
	if reset && user != "admin" {
		send(w, 403, map[string]string{"error": "Administrator access required"})
		return true
	}
	var input struct {
		User     string `json:"user"`
		Current  string `json:"current"`
		Password string `json:"password"`
	}
	if e := decode(w, r, &input); e != nil {
		failure(w, e)
		return true
	}
	target := user
	if reset {
		target = input.User
	} else if input.User != "" && input.User != user {
		send(w, 403, map[string]string{"error": "You can change only your own password"})
		return true
	}
	a := s.access
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	a.mu.Lock()
	rate := a.attempts[host]
	if time.Since(rate.Since) > time.Minute {
		rate = attempt{Since: time.Now()}
	}
	rate.Count++
	a.attempts[host] = rate
	current := a.data.Users[user]
	before, exists := a.data.Users[target]
	a.mu.Unlock()
	if rate.Count > 10 {
		send(w, 429, map[string]string{"error": "Too many authentication attempts; try again in a minute"})
		return true
	}
	if !passwordMatches(current, input.Current) {
		send(w, 401, map[string]string{"error": "Current password is incorrect"})
		return true
	}
	if !exists {
		failure(w, fmt.Errorf("account not found"))
		return true
	}
	next, e := passwordAccount(input.Password)
	if e != nil {
		failure(w, e)
		return true
	}
	a.mu.Lock()
	if a.data.Users[user] != current || a.data.Users[target] != before {
		a.mu.Unlock()
		send(w, 409, map[string]string{"error": "Credentials changed; sign in again"})
		return true
	}
	a.data.Users[target] = next
	if e = a.save(); e != nil {
		a.data.Users[target] = before
		a.mu.Unlock()
		failure(w, e)
		return true
	}
	for token, session := range a.sessions {
		if session.User == target {
			delete(a.sessions, token)
		}
	}
	a.mu.Unlock()
	if target == user {
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", HttpOnly: true, Secure: a.origin.Scheme == "https", SameSite: http.SameSiteStrictMode, MaxAge: -1})
	}
	send(w, 200, map[string]bool{"ok": true, "signInRequired": target == user})
	return true
}
