package httpapi

import (
	"bufio"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateBucket struct {
	tokens  float64
	updated time.Time
}
type requestLimits struct {
	mu      sync.Mutex
	buckets map[string]rateBucket
}

func (l *requestLimits) allow(key string, capacity int, period time.Duration, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.buckets == nil {
		l.buckets = map[string]rateBucket{}
	}
	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= 10000 {
			for k, v := range l.buckets {
				if now.Sub(v.updated) > time.Hour {
					delete(l.buckets, k)
				}
			}
			if len(l.buckets) >= 10000 {
				return false
			}
		}
		b = rateBucket{tokens: float64(capacity), updated: now}
	}
	b.tokens = min(float64(capacity), b.tokens+now.Sub(b.updated).Seconds()*float64(capacity)/period.Seconds())
	b.updated = now
	allowed := b.tokens >= 1
	if allowed {
		b.tokens--
	}
	l.buckets[key] = b
	return allowed
}
func requestClass(r *http.Request) string {
	p := r.URL.Path
	if strings.HasPrefix(p, "/auth/") {
		return "authentication"
	}
	if strings.Contains(p, "/invitations") {
		return "invitations"
	}
	if strings.Contains(p, "/import") {
		return "imports"
	}
	if p == "/api/sessions" && r.Method == "POST" || strings.HasSuffix(p, "/child") {
		return "generation"
	}
	if strings.HasPrefix(p, "/api/collaboration/") {
		return "collaboration"
	}
	return "api"
}
func (s *Server) limitRequest(w http.ResponseWriter, r *http.Request, identity string) bool {
	if s.auth.Config.Mode != "oidc" {
		return true
	}
	class := requestClass(r)
	capacity, period := 600, time.Minute
	switch class {
	case "authentication":
		capacity, period = 60, time.Minute
	case "invitations":
		if mutating(r) {
			capacity, period = 20, 10*time.Minute
		}
	case "imports":
		capacity, period = 10, time.Minute
	case "generation":
		capacity, period = 4, time.Minute
	case "collaboration":
		capacity, period = 60, time.Minute
	}
	if identity == "" {
		identity, _, _ = net.SplitHostPort(r.RemoteAddr)
	}
	// Forwarded headers are intentionally not trusted. Proxy-side limits can
	// additionally distinguish unauthenticated users behind a shared proxy.
	if !s.limits.allow(class+":"+identity, capacity, period, time.Now()) {
		w.Header().Set("Retry-After", "30")
		fail(w, 429, "Too many requests; wait before retrying")
		return false
	}
	return true
}

type observedResponse struct {
	http.ResponseWriter
	status int
}

func (w *observedResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *observedResponse) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}
func (w *observedResponse) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (w *observedResponse) Flush() {
	if w.status == 0 {
		w.status = 200
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}
func (w *observedResponse) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}
func observeRequest(w *observedResponse, r *http.Request) {
	if w.status >= 400 {
		// No query strings, cookies, bodies, email addresses or invitation tokens.
		slog.Warn("request_rejected", "class", requestClass(r), "method", r.Method, "status", w.status)
	}
}
