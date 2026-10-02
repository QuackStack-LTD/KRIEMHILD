package httpapi

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"kriemhild/internal/project"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

type account struct {
	Salt string `json:"salt"`
	Hash string `json:"hash"`
}
type accessData struct {
	Users map[string]account           `json:"users"`
	Roles map[string]map[string]string `json:"roles"`
}
type loginSession struct {
	User  string
	Until time.Time
}
type attempt struct {
	Count int
	Since time.Time
}
type Access struct {
	mu       sync.Mutex
	lock     *flock.Flock
	path     string
	origin   *url.URL
	data     accessData
	sessions map[string]loginSession
	attempts map[string]attempt
}

func passwordAccount(password string) (account, error) {
	if len(password) < 12 || len(password) > 512 {
		return account{}, fmt.Errorf("passwords must contain 12–512 bytes")
	}
	salt := make([]byte, 24)
	if _, e := rand.Read(salt); e != nil {
		return account{}, e
	}
	key, e := pbkdf2.Key(sha256.New, password, salt, 600000, 32)
	return account{base64.StdEncoding.EncodeToString(salt), base64.StdEncoding.EncodeToString(key)}, e
}
func (a *Access) save() error {
	b, e := json.MarshalIndent(a.data, "", "  ")
	if e != nil {
		return e
	}
	return project.WriteAtomic(a.path, b)
}
func (s *Server) EnableHosting(origin, password string) error {
	if s.access != nil {
		return fmt.Errorf("hosting is already enabled")
	}
	u, e := url.Parse(origin)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "https" && !(u.Scheme == "http" && localHost(u.Host))) {
		return fmt.Errorf("hosted origin must be an HTTPS origin; HTTP is allowed only for loopback testing")
	}
	u.Path = ""
	a := &Access{path: filepath.Join(s.Data, ".access.json"), origin: u, sessions: map[string]loginSession{}, attempts: map[string]attempt{}}
	a.lock = flock.New(filepath.Join(s.Data, ".access.lock"))
	locked, e := a.lock.TryLock()
	if e != nil {
		return e
	}
	if !locked {
		return fmt.Errorf("another hosted server owns this account store")
	}
	defer func() {
		if s.access != a {
			a.lock.Unlock()
		}
	}()
	b, e := os.ReadFile(a.path)
	if os.IsNotExist(e) {
		acct, e := passwordAccount(password)
		if e != nil {
			return fmt.Errorf("initial administrator password: %w", e)
		}
		a.data = accessData{Users: map[string]account{"admin": acct}, Roles: map[string]map[string]string{}}
		if e = a.save(); e != nil {
			return e
		}
	} else if e != nil {
		return e
	} else if e = json.Unmarshal(b, &a.data); e != nil {
		return e
	}
	if a.data.Users["admin"].Hash == "" || a.data.Roles == nil {
		return fmt.Errorf("invalid account store")
	}
	s.access = a
	return nil
}
func (a *Access) user(r *http.Request, cookieName string) string {
	cookie, e := r.Cookie(cookieName)
	if e != nil {
		return ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	session, ok := a.sessions[cookie.Value]
	if !ok || time.Now().After(session.Until) {
		delete(a.sessions, cookie.Value)
		return ""
	}
	return session.User
}
func (a *Access) role(user, world string) string {
	if user == "admin" {
		return "owner"
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.data.Roles[world][user]
}
func (a *Access) grant(world, user, role string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.data.Users[user]; !ok {
		return fmt.Errorf("account not found")
	}
	if role != "owner" && role != "editor" && role != "viewer" && role != "remove" {
		return fmt.Errorf("invalid role")
	}
	if a.data.Roles[world] == nil {
		a.data.Roles[world] = map[string]string{}
	}
	before := a.data.Roles[world][user]
	if role == "remove" {
		delete(a.data.Roles[world], user)
	} else {
		a.data.Roles[world][user] = role
	}
	if e := a.save(); e != nil {
		if before == "" {
			delete(a.data.Roles[world], user)
		} else {
			a.data.Roles[world][user] = before
		}
		return e
	}
	return nil
}
func (s *Server) authentication(w http.ResponseWriter, r *http.Request, cookieName string) (string, bool) {
	if s.access == nil {
		return "Local author", false
	}
	a := s.access
	if r.URL.Path == "/api/v1/login" && r.Method == "POST" {
		var input struct {
			User     string `json:"user"`
			Password string `json:"password"`
		}
		if e := decode(w, r, &input); e != nil {
			failure(w, e)
			return "", true
		}
		if len(input.Password) > 512 {
			send(w, 401, map[string]string{"error": "Invalid credentials"})
			return "", true
		}
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		a.mu.Lock()
		rate := a.attempts[host]
		if time.Since(rate.Since) > time.Minute {
			rate = attempt{Since: time.Now()}
		}
		rate.Count++
		a.attempts[host] = rate
		if rate.Count > 10 {
			a.mu.Unlock()
			send(w, 429, map[string]string{"error": "Too many login attempts; try again in a minute"})
			return "", true
		}
		acct, ok := a.data.Users[input.User]
		if !ok {
			acct = a.data.Users["admin"]
		}
		a.mu.Unlock()
		salt, _ := base64.StdEncoding.DecodeString(acct.Salt)
		want, _ := base64.StdEncoding.DecodeString(acct.Hash)
		key, e := pbkdf2.Key(sha256.New, input.Password, salt, 600000, 32)
		if e != nil || subtle.ConstantTimeCompare(key, want) != 1 || !ok {
			send(w, 401, map[string]string{"error": "Invalid credentials"})
			return "", true
		}
		token := project.NewID() + project.NewID()
		a.mu.Lock()
		// A password change may have completed while the old hash was verified.
		if a.data.Users[input.User] != acct {
			a.mu.Unlock()
			send(w, 401, map[string]string{"error": "Credentials changed; sign in again"})
			return "", true
		}
		for id, session := range a.sessions {
			if time.Now().After(session.Until) {
				delete(a.sessions, id)
			}
		}
		if len(a.sessions) >= 1000 {
			a.mu.Unlock()
			send(w, 503, map[string]string{"error": "Session capacity reached"})
			return "", true
		}
		a.sessions[token] = loginSession{input.User, time.Now().Add(12 * time.Hour)}
		a.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", HttpOnly: true, Secure: a.origin.Scheme == "https", SameSite: http.SameSiteStrictMode, MaxAge: 43200})
		send(w, 200, map[string]bool{"ok": true})
		return "", true
	}
	user := a.user(r, cookieName)
	if r.URL.Path == "/api/v1/session" && r.Method == "GET" {
		send(w, 200, map[string]any{"hosted": true, "authenticated": user != "", "user": user, "version": "development", "library": "Hosted workspace"})
		return user, true
	}
	if user == "" {
		send(w, 401, map[string]string{"error": "Sign in to this workspace"})
		return "", true
	}
	if (r.URL.Path == "/api/v1/password" || r.URL.Path == "/api/v1/accounts/reset-password") && r.Method == "POST" {
		return user, s.changePassword(w, r, cookieName, user)
	}
	if r.URL.Path == "/api/v1/logout" && r.Method == "POST" {
		cookie, _ := r.Cookie(cookieName)
		a.mu.Lock()
		delete(a.sessions, cookie.Value)
		a.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", HttpOnly: true, Secure: a.origin.Scheme == "https", SameSite: http.SameSiteStrictMode, MaxAge: -1})
		send(w, 200, map[string]bool{"ok": true})
		return user, true
	}
	if r.URL.Path == "/api/v1/accounts" && r.Method == "POST" {
		if user != "admin" {
			send(w, 403, map[string]string{"error": "Administrator access required"})
			return user, true
		}
		var input struct {
			User     string `json:"user"`
			Password string `json:"password"`
		}
		if e := decode(w, r, &input); e != nil {
			failure(w, e)
			return user, true
		}
		if input.User == "" || len(input.User) > 100 || strings.ContainsAny(input.User, "\r\n\t") {
			failure(w, fmt.Errorf("invalid account name"))
			return user, true
		}
		acct, e := passwordAccount(input.Password)
		if e != nil {
			failure(w, e)
			return user, true
		}
		a.mu.Lock()
		if _, ok := a.data.Users[input.User]; ok {
			a.mu.Unlock()
			failure(w, fmt.Errorf("account already exists"))
			return user, true
		}
		a.data.Users[input.User] = acct
		e = a.save()
		if e != nil {
			delete(a.data.Users, input.User)
		}
		a.mu.Unlock()
		if e != nil {
			failure(w, e)
			return user, true
		}
		send(w, 201, map[string]bool{"ok": true})
		return user, true
	}
	return user, false
}
