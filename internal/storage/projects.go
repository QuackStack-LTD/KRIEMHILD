// Package storage keeps world checkpoints, explored tiles and legacy portable
// archives in PostgreSQL or embedded SQLite without working-cache dependencies.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type Project struct {
	ID          string `json:"id"`
	Seed        string `json:"seed"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	DetailTiles int    `json:"detailTiles"`
	Bytes       int    `json:"bytes"`
	UpdatedAt   int64  `json:"updatedAt"`
}
type Store struct {
	db     *sql.DB
	driver string
}

func Open(ctx context.Context, dataDir, databaseURL string) (*Store, error) {
	driver, dsn := "sqlite", ""
	if databaseURL != "" {
		u, err := url.Parse(databaseURL)
		if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" {
			return nil, errors.New("DATABASE_URL must be a PostgreSQL connection URL")
		}
		driver, dsn = "pgx", databaseURL
	} else {
		if err := os.MkdirAll(dataDir, 0700); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
		filename, err := filepath.Abs(filepath.Join(dataDir, "kriemhild.sqlite"))
		if err != nil {
			return nil, err
		}
		// URL-escape paths so Windows drive letters/spaces and Unix paths both work.
		p := filepath.ToSlash(filename)
		if len(p) > 1 && p[1] == ':' {
			p = "/" + p
		}
		u := url.URL{Scheme: "file", Path: p}
		q := url.Values{}
		q.Add("_pragma", "busy_timeout(10000)")
		q.Add("_pragma", "journal_mode(WAL)")
		q.Add("_pragma", "foreign_keys(1)")
		u.RawQuery = q.Encode()
		dsn = u.String()
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, errors.New("open project database failed")
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)
	if driver == "sqlite" {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
	}
	s := &Store{db, driver}
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("%s project database is unavailable; check connectivity and credentials", s.Driver())
	}
	if err = s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize %s project schema: %w", s.Driver(), err)
	}
	return s, nil
}
func (s *Store) Driver() string {
	if s.driver == "pgx" {
		return "postgresql"
	}
	return "sqlite"
}
func (s *Store) Close() error                   { return s.db.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if s.driver == "pgx" {
		if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(1263683917)"); err != nil {
			return err
		}
	}
	statements := []string{
		"CREATE TABLE IF NOT EXISTS kriemhild_schema (version INTEGER PRIMARY KEY)",
		"CREATE TABLE IF NOT EXISTS kriemhild_projects (id TEXT PRIMARY KEY, seed TEXT NOT NULL, width INTEGER NOT NULL, height INTEGER NOT NULL, detail_tiles INTEGER NOT NULL, bytes BIGINT NOT NULL, updated_at BIGINT NOT NULL, archive BYTEA NOT NULL)",
	}
	// SQLite accepts BLOB rather than PostgreSQL's bytea storage class.
	if s.driver == "sqlite" {
		statements[1] = "CREATE TABLE IF NOT EXISTS kriemhild_projects (id TEXT PRIMARY KEY, seed TEXT NOT NULL, width INTEGER NOT NULL, height INTEGER NOT NULL, detail_tiles INTEGER NOT NULL, bytes BIGINT NOT NULL, updated_at BIGINT NOT NULL, archive BLOB NOT NULL)"
	}
	if _, err = tx.ExecContext(ctx, statements[0]); err != nil {
		return err
	}
	var version int
	if err = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(version),0) FROM kriemhild_schema").Scan(&version); err != nil {
		return err
	}
	if version > 2 {
		return errors.New("database schema is newer than this server")
	}
	if _, err = tx.ExecContext(ctx, statements[1]); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS kriemhild_world_parts (world_id TEXT NOT NULL REFERENCES kriemhild_projects(id) ON DELETE CASCADE, path TEXT NOT NULL, data BYTEA NOT NULL, PRIMARY KEY(world_id,path))"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO kriemhild_schema(version) VALUES(2) ON CONFLICT(version) DO NOTHING"); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) Save(ctx context.Context, p Project, archive []byte) error {
	if p.ID == "" || len(archive) == 0 {
		return errors.New("project ID and archive are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO kriemhild_projects(id,seed,width,height,detail_tiles,bytes,updated_at,archive)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8)
 ON CONFLICT(id) DO UPDATE SET seed=excluded.seed,width=excluded.width,height=excluded.height,detail_tiles=excluded.detail_tiles,bytes=excluded.bytes,updated_at=excluded.updated_at,archive=excluded.archive`, p.ID, p.Seed, p.Width, p.Height, p.DetailTiles, len(archive), time.Now().UTC().UnixMilli(), archive)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM kriemhild_world_parts WHERE world_id=$1", p.ID); err != nil {
		return err
	}
	return tx.Commit()
}

// SaveParts atomically commits only the changed checkpoint, view or detail tiles.
// Complete ZIP archives remain a backward-compatible import/export format.
func (s *Store) SaveParts(ctx context.Context, p Project, parts map[string][]byte) error {
	if p.ID == "" || len(parts) == 0 {
		return errors.New("world identity and parts required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO kriemhild_projects(id,seed,width,height,detail_tiles,bytes,updated_at,archive)
 VALUES($1,$2,$3,$4,$5,0,$6,$7) ON CONFLICT(id) DO UPDATE SET seed=excluded.seed,width=excluded.width,height=excluded.height,detail_tiles=excluded.detail_tiles,updated_at=excluded.updated_at,archive=excluded.archive`, p.ID, p.Seed, p.Width, p.Height, p.DetailTiles, time.Now().UTC().UnixMilli(), []byte{})
	if err != nil {
		return err
	}
	for path, data := range parts {
		if _, err = tx.ExecContext(ctx, `INSERT INTO kriemhild_world_parts(world_id,path,data) VALUES($1,$2,$3) ON CONFLICT(world_id,path) DO UPDATE SET data=excluded.data`, p.ID, path, data); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE kriemhild_projects SET bytes=(SELECT COALESCE(SUM(LENGTH(data)),0) FROM kriemhild_world_parts WHERE world_id=$1) WHERE id=$1", p.ID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) LoadParts(ctx context.Context, id string) (map[string][]byte, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT path,data FROM kriemhild_world_parts WHERE world_id=$1", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	parts := map[string][]byte{}
	for rows.Next() {
		var path string
		var data []byte
		if err = rows.Scan(&path, &data); err != nil {
			return nil, err
		}
		parts[path] = data
	}
	return parts, rows.Err()
}
func (s *Store) List(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,seed,width,height,detail_tiles,bytes,updated_at FROM kriemhild_projects ORDER BY updated_at DESC,id LIMIT 1000")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Project{}
	for rows.Next() {
		var p Project
		if err = rows.Scan(&p.ID, &p.Seed, &p.Width, &p.Height, &p.DetailTiles, &p.Bytes, &p.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
func (s *Store) Load(ctx context.Context, id string) ([]byte, error) {
	var archive []byte
	err := s.db.QueryRowContext(ctx, "SELECT archive FROM kriemhild_projects WHERE id=$1", id).Scan(&archive)
	return archive, err
}
