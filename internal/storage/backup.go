package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// The map cache is process-local. Hosted PostgreSQL permits one writer process.
func (s *Store) LeaseHosted(ctx context.Context) error {
	if s.driver != "pgx" {
		return nil
	}
	conn, e := s.db.Conn(ctx)
	if e != nil {
		return e
	}
	var locked bool
	if e = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(1263683918)`).Scan(&locked); e != nil || !locked {
		conn.Close()
		return fmt.Errorf("another hosted server holds the map-engine lease")
	}
	s.lease = conn
	return nil
}

// BackupSQLite creates a consistent online snapshot, including WAL contents,
// users, revisions and collaborative update logs. It never overwrites a file.
func (s *Store) BackupSQLite(ctx context.Context, destination string) error {
	if s.driver != "sqlite" {
		return fmt.Errorf("use pg_dump --format=custom for PostgreSQL backups")
	}
	path, e := filepath.Abs(destination)
	if e != nil {
		return e
	}
	if _, e = os.Stat(path); e == nil {
		return fmt.Errorf("backup destination already exists")
	} else if !os.IsNotExist(e) {
		return e
	}
	_, e = s.db.ExecContext(ctx, `VACUUM INTO $1`, path)
	if e != nil {
		return e
	}
	return os.Chmod(path, 0600)
}
func remapWriterWorld(raw json.RawMessage, old, next string) json.RawMessage {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return raw
	}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if x["worldId"] == old {
				x["worldId"] = next
			}
			for _, child := range x {
				walk(child)
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(value)
	out, e := json.Marshal(value)
	if e != nil {
		return raw
	}
	return out
}
