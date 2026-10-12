package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"kriemhild/internal/history"
	"kriemhild/internal/world"
	"sort"
	"strings"
	"time"
)

var historyTables = []string{"ages", "timelines", "edges", "terrains", "maps", "entities", "representations", "relations", "years", "biospheres", "library_files"}

// A World spans multiple tables. PostgreSQL readers must see one consistent
// version across all queries, including while another editor saves a map.
func (s *Store) beginHistory(ctx context.Context, readOnly bool) (*sql.Tx, error) {
	options := &sql.TxOptions{ReadOnly: readOnly}
	if s.driver == "pgx" {
		options.Isolation = sql.LevelRepeatableRead
	}
	return s.db.BeginTx(ctx, options)
}

func (s *Store) migrateHistory(ctx context.Context) error {
	statements := []string{"CREATE TABLE IF NOT EXISTS writing_documents (id TEXT PRIMARY KEY,name TEXT NOT NULL,body TEXT NOT NULL,revision INTEGER NOT NULL,updated_at BIGINT NOT NULL)", "CREATE TABLE IF NOT EXISTS history_worlds (id TEXT PRIMARY KEY, name TEXT NOT NULL, revision INTEGER NOT NULL, updated_at BIGINT NOT NULL, data TEXT NOT NULL)", "CREATE TABLE IF NOT EXISTS history_blobs (hash TEXT PRIMARY KEY, data BYTEA NOT NULL)", "CREATE TABLE IF NOT EXISTS history_snapshots (hash TEXT PRIMARY KEY, data TEXT NOT NULL)"}
	for _, table := range historyTables {
		statements = append(statements, "CREATE TABLE IF NOT EXISTS history_"+table+" (world_id TEXT NOT NULL REFERENCES history_worlds(id) ON DELETE CASCADE, id TEXT NOT NULL, age_id TEXT NOT NULL, project_id TEXT NOT NULL, source_id TEXT NOT NULL, destination_id TEXT NOT NULL, kind TEXT NOT NULL, data TEXT NOT NULL, PRIMARY KEY(world_id,id))")
	}
	statements = append(statements, writerSchema...)
	statements = append(statements, "CREATE UNIQUE INDEX IF NOT EXISTS history_map_project ON history_maps(project_id)")
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
	for _, q := range statements {
		if _, err = tx.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	if err = migrateWriter(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

type historyQuery interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readHistory(ctx context.Context, q historyQuery, id string) (*history.Document, error) {
	var raw string
	d := &history.Document{}
	if err := q.QueryRowContext(ctx, "SELECT data FROM history_worlds WHERE id=$1", id).Scan(&raw); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(raw), &d.World); err != nil {
		return nil, err
	}
	dest := []any{&d.Ages, &d.Timelines, &d.Edges, &d.Terrains, &d.Maps, &d.Entities, &d.Representations, &d.Relations, &d.Years, &d.Biospheres, &d.LibraryFiles}
	for i, table := range historyTables {
		rows, err := q.QueryContext(ctx, "SELECT data FROM history_"+table+" WHERE world_id=$1 ORDER BY id", id)
		if err != nil {
			return nil, err
		}
		items := []json.RawMessage{}
		for rows.Next() {
			var raw string
			if err = rows.Scan(&raw); err != nil {
				rows.Close()
				return nil, err
			}
			items = append(items, json.RawMessage(raw))
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		b, _ := json.Marshal(items)
		if err = json.Unmarshal(b, dest[i]); err != nil {
			return nil, err
		}
	}
	d.MigrateFamilyViews()
	d.SyncYears()
	sort.SliceStable(d.Timelines, func(i, j int) bool { return d.Timelines[i].Order < d.Timelines[j].Order })
	return d, nil
}
func (s *Store) History(ctx context.Context, id string) (*history.Document, error) {
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d, err := readHistory(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	return d, tx.Commit()
}
func writeHistory(ctx context.Context, tx *sql.Tx, d *history.Document, expected int, create bool) error {
	d.MigrateFamilyViews()
	d.SyncYears()
	if err := d.Validate(); err != nil {
		return err
	}
	d.World.Revision = expected + 1
	d.World.UpdatedAt = time.Now().UnixMilli()
	b, _ := json.Marshal(d.World)
	if create {
		if _, err := tx.ExecContext(ctx, "INSERT INTO history_worlds(id,name,revision,updated_at,data) VALUES($1,$2,$3,$4,$5)", d.World.ID, d.World.Name, d.World.Revision, d.World.UpdatedAt, string(b)); err != nil {
			return err
		}
	} else {
		res, err := tx.ExecContext(ctx, "UPDATE history_worlds SET name=$2,revision=$3,updated_at=$4,data=$5 WHERE id=$1 AND revision=$6", d.World.ID, d.World.Name, d.World.Revision, d.World.UpdatedAt, string(b), expected)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			return fmt.Errorf("World changed; reload before retrying")
		}
	}
	groups := []any{d.Ages, d.Timelines, d.Edges, d.Terrains, d.Maps, d.Entities, d.Representations, d.Relations, d.Years, d.Biospheres, d.LibraryFiles}
	for i, table := range historyTables {
		if _, err := tx.ExecContext(ctx, "DELETE FROM history_"+table+" WHERE world_id=$1", d.World.ID); err != nil {
			return err
		}
		data, _ := json.Marshal(groups[i])
		var records []map[string]json.RawMessage
		json.Unmarshal(data, &records)
		for _, record := range records {
			value := func(key string) string { var v string; json.Unmarshal(record[key], &v); return v }
			id, age := value("id"), value("ageId")
			key := id
			if age != "" {
				key = age + "/" + id
			}
			raw, _ := json.Marshal(record)
			if _, err := tx.ExecContext(ctx, "INSERT INTO history_"+table+"(world_id,id,age_id,project_id,source_id,destination_id,kind,data) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", d.World.ID, key, age, value("projectId"), value("source"), value("destination"), value("kind"), string(raw)); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *Store) CreateHistory(ctx context.Context, d *history.Document) error {
	tx, err := s.beginHistory(ctx, false)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = writeHistory(ctx, tx, d, 0, true); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) UpdateHistory(ctx context.Context, id string, revision int, fn func(*history.Document) error) (*history.Document, error) {
	tx, err := s.beginHistory(ctx, false)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d, err := readHistory(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if revision >= 0 && revision != d.World.Revision {
		return nil, fmt.Errorf("World changed; reload before retrying")
	}
	old := d.World.Revision
	if err = fn(d); err != nil {
		return nil, err
	}
	if err = writeHistory(ctx, tx, d, old, false); err != nil {
		return nil, err
	}
	return d, tx.Commit()
}
func (s *Store) Histories(ctx context.Context) ([]history.World, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT data FROM history_worlds ORDER BY updated_at DESC,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []history.World{}
	for rows.Next() {
		var raw string
		var w history.World
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &w); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
func (s *Store) DeleteHistory(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM history_worlds WHERE id=$1", id)
	return err
}
func (s *Store) MapContext(ctx context.Context, project string) (*history.Document, *history.Map, error) {
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	var id string
	if err := tx.QueryRowContext(ctx, "SELECT world_id FROM history_maps WHERE project_id=$1", project).Scan(&id); err != nil {
		return nil, nil, err
	}
	d, err := readHistory(ctx, tx, id)
	if err != nil {
		return nil, nil, err
	}
	m := d.Project(project)
	if m == nil {
		return nil, nil, sql.ErrNoRows
	}
	return d, m, tx.Commit()
}
func hash(data []byte) string { b := sha256.Sum256(data); return hex.EncodeToString(b[:]) }
func snapshotManifest(ctx context.Context, q historyQuery, id string) (map[string]string, error) {
	out := map[string]string{}
	if id == "" {
		return out, nil
	}
	var raw string
	if err := q.QueryRowContext(ctx, "SELECT data FROM history_snapshots WHERE hash=$1", id).Scan(&raw); err != nil {
		return nil, err
	}
	err := json.Unmarshal([]byte(raw), &out)
	return out, err
}
func (s *Store) SnapshotParts(ctx context.Context, id string) (map[string][]byte, error) {
	return s.SnapshotPartsMatching(ctx, id, nil)
}

// SnapshotPartsMatching avoids reading explored tiles when only environmental
// layers and authored surface operations are needed.
func (s *Store) SnapshotPartsMatching(ctx context.Context, id string, prefixes []string) (map[string][]byte, error) {
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	manifest, err := snapshotManifest(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	parts := map[string][]byte{}
	for path, h := range manifest {
		matched := len(prefixes) == 0
		for _, prefix := range prefixes {
			matched = matched || strings.HasPrefix(path, prefix)
		}
		if !matched {
			continue
		}
		var data []byte
		if err = tx.QueryRowContext(ctx, "SELECT data FROM history_blobs WHERE hash=$1", h).Scan(&data); err != nil {
			return nil, err
		}
		parts[path] = data
	}
	return parts, tx.Commit()
}
func saveSnapshot(ctx context.Context, tx *sql.Tx, previous string, parts map[string][]byte) (string, error) {
	manifest, err := snapshotManifest(ctx, tx, previous)
	if err != nil {
		return "", err
	}
	for path, data := range parts {
		h := hash(data)
		if _, err = tx.ExecContext(ctx, "INSERT INTO history_blobs(hash,data) VALUES($1,$2) ON CONFLICT(hash) DO NOTHING", h, data); err != nil {
			return "", err
		}
		manifest[path] = h
	}
	raw, _ := json.Marshal(manifest)
	id := hash(raw)
	_, err = tx.ExecContext(ctx, "INSERT INTO history_snapshots(hash,data) VALUES($1,$2) ON CONFLICT(hash) DO NOTHING", id, string(raw))
	return id, err
}
func (s *Store) CommitHistoricalMap(ctx context.Context, project string, parts map[string][]byte, state *world.State) (bool, error) {
	tx, err := s.beginHistory(ctx, false)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(ctx, "SELECT world_id FROM history_maps WHERE project_id=$1", project).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	d, err := readHistory(ctx, tx, id)
	if err != nil {
		return true, err
	}
	m := d.Project(project)
	old := d.World.Revision
	snapshot, err := saveSnapshot(ctx, tx, m.Snapshot, parts)
	if err != nil {
		return true, err
	}
	m.Snapshot = snapshot
	m.Name = state.Header.Name
	// Only changed drawables can update shared logical identity. An older
	// open map must not overwrite a footprint just saved from its child map.
	changed := world.New(int(state.W)+1, int(state.H)+1)
	for id, shape := range state.Entities {
		if _, ok := parts["builder/entity/"+id]; ok {
			if shape.LogicalID != "" {
				e := d.Entity(m.AgeID, shape.LogicalID)
				if e == nil || e.Deleted || e.Kind != shape.Kind {
					return true, fmt.Errorf("additional map region must reference an active entity of the same type")
				}
			}
			changed.Entities[id] = shape
		}
	}
	d.Sync(*m, changed)
	if err = writeHistory(ctx, tx, d, old, false); err != nil {
		return true, err
	}
	return true, tx.Commit()
}

// Portable bundles contain each immutable blob once, even across many Ages.
type Bundle struct {
	Format    string                       `json:"format"`
	Schema    int                          `json:"schema"`
	Document  *history.Document            `json:"document"`
	Snapshots map[string]map[string]string `json:"snapshots"`
	Blobs     map[string][]byte            `json:"-"`
}

func (s *Store) HistoryBundle(ctx context.Context, id string) (Bundle, error) {
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return Bundle{}, err
	}
	defer tx.Rollback()
	d, err := readHistory(ctx, tx, id)
	b := Bundle{Format: "KRIEMHILD-WORLD", Schema: 1, Document: d, Snapshots: map[string]map[string]string{}, Blobs: map[string][]byte{}}
	if err != nil {
		return b, err
	}
	for _, m := range d.Maps {
		if m.Snapshot == "" {
			return b, fmt.Errorf("map has not been saved")
		}
		if b.Snapshots[m.Snapshot] != nil {
			continue
		}
		manifest, err := snapshotManifest(ctx, tx, m.Snapshot)
		if err != nil {
			return b, err
		}
		b.Snapshots[m.Snapshot] = manifest
		for _, h := range manifest {
			if b.Blobs[h] != nil {
				continue
			}
			var data []byte
			if err = tx.QueryRowContext(ctx, "SELECT data FROM history_blobs WHERE hash=$1", h).Scan(&data); err != nil {
				return b, err
			}
			b.Blobs[h] = data
		}
	}
	for _, f := range d.LibraryFiles {
		for _, v := range f.Versions {
			if _, ok := b.Blobs[v.Hash]; ok {
				continue
			}
			var data []byte
			if err = tx.QueryRowContext(ctx, "SELECT data FROM history_blobs WHERE hash=$1", v.Hash).Scan(&data); err != nil {
				return b, err
			}
			b.Blobs[v.Hash] = data
		}
	}
	return b, tx.Commit()
}
func (b Bundle) Validate() error {
	if b.Format != "KRIEMHILD-WORLD" || b.Schema != 1 || b.Document == nil {
		return fmt.Errorf("unsupported World archive")
	}
	b.Document.MigrateFamilyViews()
	b.Document.SyncYears()
	if err := b.Document.Validate(); err != nil {
		return err
	}
	for _, m := range b.Document.Maps {
		if b.Snapshots[m.Snapshot] == nil {
			return fmt.Errorf("missing map snapshot")
		}
	}
	for _, f := range b.Document.LibraryFiles {
		for _, v := range f.Versions {
			data, ok := b.Blobs[v.Hash]
			if !ok || hash(data) != v.Hash || int64(len(data)) != v.Size {
				return fmt.Errorf("missing or corrupt Library file: %s", v.Name)
			}
		}
	}
	for id, manifest := range b.Snapshots {
		raw, _ := json.Marshal(manifest)
		if hash(raw) != id {
			return fmt.Errorf("snapshot checksum mismatch")
		}
		for _, h := range manifest {
			data, ok := b.Blobs[h]
			if !ok || hash(data) != h {
				return fmt.Errorf("missing or corrupt terrain blob")
			}
		}
	}
	return nil
}
func (s *Store) ImportHistory(ctx context.Context, b Bundle) error {
	if err := b.Validate(); err != nil {
		return err
	}
	tx, err := s.beginHistory(ctx, false)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for h, data := range b.Blobs {
		if _, err = tx.ExecContext(ctx, "INSERT INTO history_blobs(hash,data) VALUES($1,$2) ON CONFLICT(hash) DO NOTHING", h, data); err != nil {
			return err
		}
	}
	for h, manifest := range b.Snapshots {
		raw, _ := json.Marshal(manifest)
		if _, err = tx.ExecContext(ctx, "INSERT INTO history_snapshots(hash,data) VALUES($1,$2) ON CONFLICT(hash) DO NOTHING", h, string(raw)); err != nil {
			return err
		}
	}
	if err = writeHistory(ctx, tx, b.Document, 0, true); err != nil {
		return err
	}
	return tx.Commit()
}

// SnapshotPart reads one immutable layer without loading explored terrain chunks.
func (s *Store) SnapshotPart(ctx context.Context, id, name string) ([]byte, error) {
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	manifest, err := snapshotManifest(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	hash, ok := manifest[name]
	if !ok {
		return nil, sql.ErrNoRows
	}
	var data []byte
	if err = tx.QueryRowContext(ctx, "SELECT data FROM history_blobs WHERE hash=$1", hash).Scan(&data); err != nil {
		return nil, err
	}
	return data, tx.Commit()
}

func (s *Store) HistoryRevision(ctx context.Context, id string) (int, error) {
	var revision int
	err := s.db.QueryRowContext(ctx, "SELECT revision FROM history_worlds WHERE id=$1", id).Scan(&revision)
	return revision, err
}
