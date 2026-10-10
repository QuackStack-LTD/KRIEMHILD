package storage

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func exercise(t *testing.T, dir, dsn string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	s, err := Open(ctx, dir, dsn)
	if err != nil {
		t.Fatal(err)
	}
	id := "storage-test-" + time.Now().Format("20060102150405.000000000")
	p := Project{ID: id, Seed: "portable", Width: 48, Height: 32, DetailTiles: 17}
	payload := []byte{0, 1, 2, 0xff, 'P', 'K'}
	if err = s.Save(ctx, p, payload); err != nil {
		t.Fatal(err)
	}
	// UPSERT must replace atomically, not append a duplicate project identity.
	payload = append(payload, 42)
	p.DetailTiles = 18
	if err = s.Save(ctx, p, payload); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, dir, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Load(ctx, id)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatal("archive changed across database restart", err)
	}
	projects, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, v := range projects {
		if v.ID == id {
			found++
			if v.DetailTiles != 18 || v.Bytes != len(payload) || v.UpdatedAt <= 0 {
				t.Fatal("wrong project metadata")
			}
		}
	}
	if found != 1 {
		t.Fatal("missing or duplicate saved world")
	}
	if s.Driver() == "postgresql" {
		s.db.ExecContext(ctx, "DELETE FROM kriemhild_projects WHERE id=$1", id)
	}
}
func TestEmbeddedProjectDatabasePersists(t *testing.T) { exercise(t, t.TempDir(), "") }
func TestExistingCatalogMigrationPreservesWorlds(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "kriemhild.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		"CREATE TABLE kriemhild_schema (version INTEGER PRIMARY KEY)",
		"INSERT INTO kriemhild_schema VALUES (2)",
		"CREATE TABLE kriemhild_projects (id TEXT PRIMARY KEY, seed TEXT NOT NULL, width INTEGER NOT NULL, height INTEGER NOT NULL, detail_tiles INTEGER NOT NULL, bytes BIGINT NOT NULL, updated_at BIGINT NOT NULL, archive BLOB NOT NULL)",
		"INSERT INTO kriemhild_projects VALUES ('legacy','seed',32,32,5,3,1,X'010203')",
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	store, err := Open(context.Background(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	projects, err := store.List(context.Background())
	if err != nil || len(projects) != 1 || projects[0].Name != "Unnamed World" {
		t.Fatal("catalog migration failed", err, projects)
	}
	data, err := store.Load(context.Background(), "legacy")
	if err != nil || !bytes.Equal(data, []byte{1, 2, 3}) {
		t.Fatal("migration changed saved project", err)
	}
}
func TestPostgresProjectDatabasePersists(t *testing.T) {
	dsn := os.Getenv("KRIEMHILD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set KRIEMHILD_TEST_DATABASE_URL for a PostgreSQL integration database")
	}
	exercise(t, t.TempDir(), dsn)
}
func TestConfiguredPostgresDoesNotFallBackSilently(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	s, err := Open(ctx, dir, "postgres://test:private-password@127.0.0.1:1/test?sslmode=disable")
	if err == nil {
		s.Close()
		t.Fatal("unavailable configured database accepted")
	}
	if _, err = os.Stat(dir + "/kriemhild.sqlite"); !os.IsNotExist(err) {
		t.Fatal("created a separate fallback database for a configured PostgreSQL failure")
	}
}
