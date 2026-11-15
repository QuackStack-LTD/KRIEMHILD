package httpapi

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIdleSessionCleanupReleasesTerrainAndSkipsBusySessions(t *testing.T) {
	now := time.Now()
	dir := t.TempDir()
	cache := filepath.Join(dir, "expired-preview")
	if err := os.Mkdir(cache, 0700); err != nil {
		t.Fatal(err)
	}
	expired := &session{last: now.Add(-31 * time.Minute), dir: cache}
	active := &session{last: now.Add(-time.Minute)}
	busy := &session{last: now.Add(-time.Hour)}
	busy.mu.Lock()
	s := &Server{sessions: map[string]*session{"expired": expired, "active": active, "busy": busy}}
	s.reapIdleSessions(now)
	if len(s.sessions) != 2 || !expired.retired || active.retired || busy.retired {
		t.Fatal("cleanup discarded active terrain or retained expired terrain")
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatal("expired preview cache was retained", err)
	}
	busy.mu.Unlock()
	s.reapIdleSessions(now.Add(2 * time.Minute))
	if !busy.retired || len(s.sessions) != 1 {
		t.Fatal("busy session was never revisited")
	}
}
