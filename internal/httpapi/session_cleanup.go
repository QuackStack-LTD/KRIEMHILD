package httpapi

import (
	"os"
	"time"
)

const sessionIdleLimit = 30 * time.Minute

// Opportunistic cleanup avoids a permanent goroutine per Server. Busy sessions
// are skipped; the next sweep retries after their requests have finished.
func (s *Server) reapIdleSessions(now time.Time) {
	s.mu.Lock()
	if !s.lastSessionSweep.IsZero() && now.Sub(s.lastSessionSweep) < time.Minute {
		s.mu.Unlock()
		return
	}
	s.lastSessionSweep = now
	dirs := []string{}
	for id, v := range s.sessions {
		if !v.mu.TryLock() {
			continue
		}
		if !v.last.IsZero() && now.Sub(v.last) > sessionIdleLimit {
			v.retired = true
			delete(s.sessions, id)
			if v.dir != "" {
				dirs = append(dirs, v.dir)
			}
		}
		v.mu.Unlock()
	}
	s.mu.Unlock()
	for _, dir := range dirs {
		_ = os.RemoveAll(dir)
	}
}
