package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"kriemhild/internal/biosphere"
	"kriemhild/internal/history"
	"kriemhild/internal/world"
	"reflect"
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
	statements = append(statements,
		"CREATE INDEX IF NOT EXISTS history_entity_age_kind ON history_entities(world_id,age_id,kind)",
		"CREATE INDEX IF NOT EXISTS history_representation_age ON history_representations(world_id,age_id)",
		"CREATE INDEX IF NOT EXISTS history_relation_age ON history_relations(world_id,age_id)")
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

// Retain small fingerprints, not serialized copies, for differential saves.
type historyFingerprints map[string]map[string][sha256.Size]byte

func readHistory(ctx context.Context, q historyQuery, id string) (*history.Document, error) {
	return readHistoryIndexed(ctx, q, id, nil)
}

func readHistoryIndexed(ctx context.Context, q historyQuery, id string, previous historyFingerprints) (*history.Document, error) {
	return readHistoryProjected(ctx, q, id, previous, false)
}

func readHistoryProjected(ctx context.Context, q historyQuery, id string, previous historyFingerprints, workspace bool, atlas ...bool) (*history.Document, error) {
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
		atlasView := len(atlas) > 0 && atlas[0]
		if atlasView && table == "biospheres" {
			continue
		}
		condition := "world_id=$1"
		if atlasView && table == "entities" {
			condition += " AND kind<>'lifeform'"
		}
		rows, err := q.QueryContext(ctx, "SELECT id,data FROM history_"+table+" WHERE "+condition+" ORDER BY id", id)
		if err != nil {
			return nil, err
		}
		// Decode each row directly into its typed slice. Building a second JSON
		// array here used to duplicate every Biosphere and all of its cell data.
		target := reflect.ValueOf(dest[i]).Elem()
		target.Set(reflect.MakeSlice(target.Type(), 0, 0))
		if previous != nil {
			previous[table] = map[string][sha256.Size]byte{}
		}
		for rows.Next() {
			var key string
			var raw []byte
			if err = rows.Scan(&key, &raw); err == nil {
				if previous != nil {
					previous[table][key] = sha256.Sum256(raw)
				}
				value := reflect.New(target.Type().Elem())
				if workspace && table == "biospheres" {
					err = decodeWorkspaceBiosphere(raw, value.Interface().(*biosphere.State))
				} else {
					err = json.Unmarshal(raw, value.Interface())
				}
				if err == nil {
					target.Set(reflect.Append(target, value.Elem()))
				}
			}
			if err != nil {
				rows.Close()
				return nil, err
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
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
func writeHistory(ctx context.Context, tx *sql.Tx, d *history.Document, expected int, create bool, baselines ...historyFingerprints) error {
	if create {
		if err := ensureOwnership(ctx, tx, d.World.ID); err != nil {
			return err
		}
	}
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
		var previous map[string][sha256.Size]byte
		if len(baselines) > 0 {
			previous = baselines[0][table]
		}
		records := reflect.ValueOf(groups[i])
		if previous == nil {
			if _, err := tx.ExecContext(ctx, "DELETE FROM history_"+table+" WHERE world_id=$1", d.World.ID); err != nil {
				return err
			}
		} else {
			// Remove vanished identities before inserting replacements, including
			// maps with globally unique project IDs. Unchanged rows stay in place.
			present := make(map[string]bool, records.Len())
			for j := 0; j < records.Len(); j++ {
				present[historyRecordKey(records.Index(j))] = true
			}
			for key := range previous {
				if !present[key] {
					if _, err := tx.ExecContext(ctx, "DELETE FROM history_"+table+" WHERE world_id=$1 AND id=$2", d.World.ID, key); err != nil {
						return err
					}
				}
			}
		}
		// Encode typed records once and reuse the insert statement. The former
		// slice -> JSON -> map -> JSON path made several full-size copies.
		statement, err := tx.PrepareContext(ctx, "INSERT INTO history_"+table+"(world_id,id,age_id,project_id,source_id,destination_id,kind,data) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(world_id,id) DO UPDATE SET age_id=excluded.age_id,project_id=excluded.project_id,source_id=excluded.source_id,destination_id=excluded.destination_id,kind=excluded.kind,data=excluded.data")
		if err != nil {
			return err
		}
		for j := 0; j < records.Len(); j++ {
			record := records.Index(j)
			value := func(field string) string { return historyRecordField(record, field) }
			key, age := historyRecordKey(record), value("AgeID")
			raw, err := json.Marshal(record.Interface())
			if err == nil {
				if old, exists := previous[key]; exists && old == sha256.Sum256(raw) {
					continue
				}
				_, err = statement.ExecContext(ctx, d.World.ID, key, age, value("ProjectID"), value("Source"), value("Destination"), value("Kind"), string(raw))
			}
			if err != nil {
				statement.Close()
				return err
			}
		}
		if err = statement.Close(); err != nil {
			return err
		}
	}
	return recordRevision(ctx, tx, d.World.ID, d.World.Revision, d)
}

func historyRecordField(record reflect.Value, field string) string {
	v := record.FieldByName(field)
	if v.IsValid() {
		return v.String()
	}
	return ""
}

func historyRecordKey(record reflect.Value) string {
	id, age := historyRecordField(record, "ID"), historyRecordField(record, "AgeID")
	if age != "" {
		return age + "/" + id
	}
	return id
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
	if Actor(ctx) != "" && revision < 0 {
		return nil, fmt.Errorf("expected revision is required")
	}
	tx, err := s.beginHistory(ctx, false)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	previous := historyFingerprints{}
	d, err := readHistoryIndexed(ctx, tx, id, previous)
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
	if err = writeHistory(ctx, tx, d, old, false, previous); err != nil {
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
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for _, table := range []string{"document_updates", "document_comments", "writer_versions"} {
		if _, e = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE node_id IN (SELECT id FROM writer_nodes WHERE project_id IN (SELECT id FROM writer_projects WHERE world_id=$1))", id); e != nil {
			return e
		}
	}
	for _, q := range []string{`DELETE FROM writer_nodes WHERE project_id IN (SELECT id FROM writer_projects WHERE world_id=$1)`, `DELETE FROM writer_projects WHERE world_id=$1`, `DELETE FROM history_worlds WHERE id=$1`, `DELETE FROM world_access WHERE world_id=$1`} {
		if _, e = tx.ExecContext(ctx, q, id); e != nil {
			return e
		}
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM writer_files WHERE world_id=$1`, id); e != nil {
		return e
	}
	return tx.Commit()
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
	d, m, err := readMapHeader(ctx, tx, project)
	if err != nil {
		return true, err
	}
	changed := world.New(int(state.W)+1, int(state.H)+1)
	for id, shape := range state.Entities {
		if _, ok := parts["builder/entity/"+id]; ok {
			changed.Entities[id] = shape
		}
	}
	if len(changed.Entities) > 0 {
		d, m, err = readMapEdit(ctx, tx, project)
		if err != nil {
			return true, err
		}
		if err = prepareMapReferences(ctx, tx, d, m, changed); err != nil {
			return true, err
		}
	} else {
		d.Maps = []history.Map{*m}
		m = &d.Maps[0]
	}
	before := mapEditRows(d)
	snapshot, err := saveSnapshot(ctx, tx, m.Snapshot, parts)
	if err != nil {
		return true, err
	}
	m.Snapshot = snapshot
	m.Name = state.Header.Name
	if len(changed.Entities) > 0 {
		if err = syncMapEdit(d, m, changed); err != nil {
			return true, err
		}
	}
	if err = writeMapEdit(ctx, tx, d, before); err != nil {
		return true, err
	}
	return true, tx.Commit()
}

// Portable bundles contain each immutable blob once, even across many Ages.
type Bundle struct {
	Writing   []WriterBundle               `json:"writing,omitempty"`
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
	if hasLinguisticRecords(d) {
		b.Schema = 2
	}
	if hasProductiveRevisions(d) {
		b.Schema = 3
	}
	sourceMaps := append([]history.Map(nil), d.Maps...)
	for _, f := range d.LibraryFiles {
		if f.ResourceSurvey != nil && f.ResourceSurvey.SourceSnapshot != "" {
			sourceMaps = append(sourceMaps, history.Map{Snapshot: f.ResourceSurvey.SourceSnapshot})
		}
	}
	for _, m := range sourceMaps {
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
			if v.Banner != nil {
				h := v.Banner.Hash
				if b.Blobs[h] == nil {
					var data []byte
					if err = tx.QueryRowContext(ctx, "SELECT data FROM history_blobs WHERE hash=$1", h).Scan(&data); err != nil {
						return b, err
					}
					b.Blobs[h] = data
				}
			}
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
	linguisticBlobs := map[string][]byte{}
	if err = collectResourceSurveyBlobs(ctx, tx, d, b.Blobs); err != nil {
		return b, err
	}
	for _, f := range d.LibraryFiles {
		if f.ResourceSurvey != nil {
			b.Schema = max(b.Schema, 6)
			break
		}
	}
	if err = collectLinguisticBodies(ctx, tx, d, linguisticBlobs); err != nil {
		return b, err
	}
	for h, raw := range linguisticBlobs {
		b.Blobs[h] = raw
		if required := linguisticArchiveSchema(raw); required > b.Schema {
			b.Schema = required
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM writer_projects WHERE world_id=$1`, id)
	if err != nil {
		return b, err
	}
	ids := []string{}
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			rows.Close()
			return b, err
		}
		ids = append(ids, key)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return b, err
	}
	for _, key := range ids {
		writing, e := writerBundleTx(ctx, tx, key, true)
		if e != nil {
			return b, e
		}
		b.Writing = append(b.Writing, writing)
	}
	return b, tx.Commit()
}
func (b Bundle) Validate() error {
	if b.Format != "KRIEMHILD-WORLD" || b.Schema < 1 || b.Schema > 6 || b.Document == nil {
		return fmt.Errorf("unsupported World archive")
	}
	if b.Schema == 1 && hasLinguisticRecords(b.Document) {
		return fmt.Errorf("Languages require World archive schema 2; a schema 1 reader would discard their definitions")
	}
	if b.Schema < 3 && hasProductiveRevisions(b.Document) {
		return fmt.Errorf("Productive language definitions require World archive schema 3; an older reader cannot preserve their semantics")
	}
	b.Document.MigrateFamilyViews()
	b.Document.SyncYears()
	if err := b.Document.Validate(); err != nil {
		return err
	}
	if err := validateLinguisticBundle(b.Document, b.Blobs, b.Schema); err != nil {
		return err
	}
	if err := validateResourceSurveyBlobs(b.Document, b.Blobs); err != nil {
		return err
	}
	for _, f := range b.Document.LibraryFiles {
		if f.ResourceSurvey != nil && b.Schema < 6 {
			return fmt.Errorf("Resource observations require World archive schema 6")
		}
	}
	for _, m := range b.Document.Maps {
		if b.Snapshots[m.Snapshot] == nil {
			return fmt.Errorf("missing map snapshot")
		}
	}
	for _, f := range b.Document.LibraryFiles {
		if f.ResourceSurvey != nil && f.ResourceSurvey.SourceSnapshot != "" && b.Snapshots[f.ResourceSurvey.SourceSnapshot] == nil {
			return fmt.Errorf("missing observed terrain snapshot")
		}
		for _, v := range f.Versions {
			if v.Banner != nil {
				data, ok := b.Blobs[v.Banner.Hash]
				if !ok || hash(data) != v.Banner.Hash || int64(len(data)) != v.Banner.Size {
					return fmt.Errorf("missing or corrupt Library banner")
				}
			}
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
	for _, writing := range b.Writing {
		old := writing.Project.WorldID
		writing.Project.ImportedWorldID = old
		writing.Project.WorldID = b.Document.World.ID
		remap := func(n *WritingNode) {
			for i := range n.References {
				if n.References[i].WorldID == old {
					n.References[i].WorldID = b.Document.World.ID
				}
			}
			n.Content = remapWriterWorld(n.Content, old, b.Document.World.ID)
		}
		for i := range writing.Nodes {
			remap(&writing.Nodes[i])
		}
		for i := range writing.Versions {
			remap(&writing.Versions[i].Node)
		}
		if _, err = importWriterTx(ctx, tx, writing, b.Document.World.ID); err != nil {
			return err
		}
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
