package storage

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"kriemhild/internal/history"
	"kriemhild/internal/world"
	"reflect"
	"strings"
	"time"
)

// Terrain transactions operate on geographic records in one Age. In particular,
// they never read habitat profiles, populations or the generated species list.
func readMapEdit(ctx context.Context, tx *sql.Tx, project string) (*history.Document, *history.Map, error) {
	d, m, err := readMapHeader(ctx, tx, project)
	if err != nil {
		return nil, nil, err
	}
	where := "world_id=$1 AND age_id=$2"
	args := []any{d.World.ID, m.AgeID}
	if d.Terrains, err = readViewRows[history.Terrain](ctx, tx, "terrains", where, args...); err != nil {
		return nil, nil, err
	}
	if d.Maps, err = readViewRows[history.Map](ctx, tx, "maps", where, args...); err != nil {
		return nil, nil, err
	}
	if d.Entities, err = readViewRows[history.Entity](ctx, tx, "entities", where+" AND kind<>'lifeform'", args...); err != nil {
		return nil, nil, err
	}
	if d.Representations, err = readViewRows[history.Representation](ctx, tx, "representations", where, args...); err != nil {
		return nil, nil, err
	}
	if d.Relations, err = readViewRows[history.Relation](ctx, tx, "relations", where, args...); err != nil {
		return nil, nil, err
	}
	return d, d.Project(project), nil
}

func prepareMapReferences(ctx context.Context, tx *sql.Tx, d *history.Document, m *history.Map, changes *world.State) error {
	keys := []string{}
	for _, shape := range changes.Entities {
		keys = append(keys, m.AgeID+"/"+shape.ID)
	}
	for start := 0; start < len(keys); start += 256 {
		args := []any{d.World.ID}
		places := []string{}
		for _, key := range keys[start:min(start+256, len(keys))] {
			args = append(args, key)
			places = append(places, fmt.Sprintf("$%d", len(args)))
		}
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM history_entities WHERE world_id=$1 AND kind='lifeform' AND id IN ("+strings.Join(places, ",")+")", args...).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("map object cannot replace a biological record")
		}
	}
	// Deleting a named place may invalidate a species' local-name context.
	// Load only records mentioning that identity, not the species catalogue.
	for _, shape := range changes.Entities {
		id := shape.ID
		if shape.LogicalID != "" {
			id = shape.LogicalID
		}
		if !shape.Deleted && shape.RecordStatus != "draft" || d.Entity(m.AgeID, id) == nil {
			continue
		}
		quoted, _ := json.Marshal(id)
		pattern := "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(string(quoted)) + "%"
		owners, err := readViewRows[history.Entity](ctx, tx, "entities", "world_id=$1 AND age_id=$2 AND kind='lifeform' AND data LIKE $3 ESCAPE '!'", d.World.ID, m.AgeID, pattern)
		if err != nil {
			return err
		}
		for _, e := range owners {
			if d.Entity(e.AgeID, e.ID) == nil {
				d.Entities = append(d.Entities, e)
			}
		}
	}
	return nil
}

func mapEditRows(d *history.Document) map[string]map[string]json.RawMessage {
	groups := map[string]any{"maps": d.Maps, "entities": d.Entities, "representations": d.Representations, "relations": d.Relations}
	out := map[string]map[string]json.RawMessage{}
	for table, group := range groups {
		out[table] = map[string]json.RawMessage{}
		records := reflect.ValueOf(group)
		for i := 0; i < records.Len(); i++ {
			record := records.Index(i)
			raw, _ := json.Marshal(record.Interface())
			out[table][historyRecordKey(record)] = raw
		}
	}
	return out
}

type mapRevisionPatch struct {
	Format   string                                `json:"format"`
	Base     int                                   `json:"base"`
	World    history.World                         `json:"world"`
	Rows     map[string]map[string]json.RawMessage `json:"rows"`
	NameOnly bool                                  `json:"nameOnly,omitempty"`
}

// A geographic revision stores a patch against its predecessor. Restoring a
// revision expands patches on demand; autosaving never serializes the biosphere.
func writeMapEdit(ctx context.Context, tx *sql.Tx, d *history.Document, before map[string]map[string]json.RawMessage) error {
	return writeRecordPatch(ctx, tx, d, before, mapEditRows(d))
}

// A projected writer explicitly supplies its record groups. Missing groups are
// untouched; saving a report never loads or rewrites generated biological data.
func writeRecordPatch(ctx context.Context, tx *sql.Tx, d *history.Document, before, after map[string]map[string]json.RawMessage, nameOnly ...bool) error {
	old := d.World.Revision
	d.World.Revision++
	d.World.UpdatedAt = time.Now().UnixMilli()
	raw, err := json.Marshal(d.World)
	if err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, "UPDATE history_worlds SET name=$2,revision=$3,updated_at=$4,data=$5 WHERE id=$1 AND revision=$6", d.World.ID, d.World.Name, d.World.Revision, d.World.UpdatedAt, string(raw), old)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("World changed; reload before retrying")
	}
	patch := mapRevisionPatch{Format: "KRIEMHILD-MAP-PATCH-1", Base: old, World: d.World, Rows: map[string]map[string]json.RawMessage{}}
	patch.NameOnly = len(nameOnly) > 0 && nameOnly[0]
	for table, records := range after {
		if table != "entities" && table != "library_files" && table != "maps" && table != "representations" && table != "relations" && table != "terrains" {
			return fmt.Errorf("invalid projected record table")
		}
		patch.Rows[table] = map[string]json.RawMessage{}
		for key := range before[table] {
			if _, exists := records[key]; !exists {
				if _, err = tx.ExecContext(ctx, "DELETE FROM history_"+table+" WHERE world_id=$1 AND id=$2", d.World.ID, key); err != nil {
					return err
				}
				patch.Rows[table][key] = json.RawMessage("null")
			}
		}
		for key, raw := range records {
			if bytes.Equal(before[table][key], raw) {
				continue
			}
			var fields struct {
				AgeID       string `json:"ageId"`
				ProjectID   string `json:"projectId"`
				Source      string `json:"source"`
				Destination string `json:"destination"`
				Kind        string `json:"kind"`
			}
			if err = json.Unmarshal(raw, &fields); err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, "INSERT INTO history_"+table+"(world_id,id,age_id,project_id,source_id,destination_id,kind,data) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(world_id,id) DO UPDATE SET age_id=excluded.age_id,project_id=excluded.project_id,source_id=excluded.source_id,destination_id=excluded.destination_id,kind=excluded.kind,data=excluded.data", d.World.ID, key, fields.AgeID, fields.ProjectID, fields.Source, fields.Destination, fields.Kind, string(raw))
			if err != nil {
				return err
			}
			patch.Rows[table][key] = raw
		}
	}
	if Actor(ctx) == "" {
		return nil
	}
	var exists int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM world_revisions WHERE world_id=$1 AND revision=$2", d.World.ID, old).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		// The first authenticated edit of a locally created World needs a base.
		full, err := readHistory(ctx, tx, d.World.ID)
		if err != nil {
			return err
		}
		return recordRevision(ctx, tx, d.World.ID, d.World.Revision, full)
	}
	return recordRevision(ctx, tx, d.World.ID, d.World.Revision, patch)
}

func syncMapEdit(d *history.Document, m *history.Map, state *world.State) error {
	for _, shape := range state.Entities {
		if shape.LogicalID != "" && !shape.Deleted {
			e := d.Entity(m.AgeID, shape.LogicalID)
			if e == nil || e.Deleted || !mapKindCompatible(e.Kind, shape.Kind) {
				return fmt.Errorf("invalid logical entity reference")
			}
		}
	}
	d.SyncMapShapes(*m, state)
	return nil
}

func mapKindCompatible(a, b string) bool {
	family := func(k string) int {
		switch k {
		case "country", "state", "kingdom", "empire":
			return 1
		case "capital", "city", "town", "village", "hamlet", "custom-settlement":
			return 2
		}
		return 0
	}
	return a == b || family(a) != 0 && family(a) == family(b)
}

// Apply a validated geographic metadata operation without loading lifeforms,
// populations, or other Ages. The callback must preserve map/record identities.
func (s *Store) UpdateMapRecords(ctx context.Context, project string, revision int, fn func(*history.Document, *history.Map) error) (*history.Document, error) {
	if Actor(ctx) != "" && revision < 0 {
		return nil, fmt.Errorf("expected revision is required")
	}
	tx, err := s.beginHistory(ctx, false)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d, m, err := readMapEdit(ctx, tx, project)
	if err != nil {
		return nil, err
	}
	if revision >= 0 && revision != d.World.Revision {
		return nil, fmt.Errorf("World changed; reload before retrying")
	}
	before := mapEditRows(d)
	if err = fn(d, m); err != nil {
		return nil, err
	}
	if err = writeMapEdit(ctx, tx, d, before); err != nil {
		return nil, err
	}
	return d, tx.Commit()
}

func (s *Store) SyncMapEntities(ctx context.Context, project string, revision int, state *world.State) (*history.Document, error) {
	if Actor(ctx) != "" && revision < 0 {
		return nil, fmt.Errorf("expected revision is required")
	}
	tx, err := s.beginHistory(ctx, false)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d, m, err := readMapEdit(ctx, tx, project)
	if err != nil {
		return nil, err
	}
	if revision >= 0 && d.World.Revision != revision {
		return nil, fmt.Errorf("World changed; reload before retrying")
	}
	if err = prepareMapReferences(ctx, tx, d, m, state); err != nil {
		return nil, err
	}
	before := mapEditRows(d)
	if err = syncMapEdit(d, m, state); err != nil {
		return nil, err
	}
	if err = writeMapEdit(ctx, tx, d, before); err != nil {
		return nil, err
	}
	return d, tx.Commit()
}

// Expand only when a user inspects/restores an audit revision. Iteration avoids
// a deep recursion after a long editing session with many automatic saves.
func expandMapRevision(ctx context.Context, q historyQuery, id, raw string) (string, error) {
	patches := []mapRevisionPatch{}
	for {
		var patch mapRevisionPatch
		if err := json.Unmarshal([]byte(raw), &patch); err != nil {
			return "", err
		}
		if patch.Format != "KRIEMHILD-MAP-PATCH-1" {
			break
		}
		if patch.World.ID != id || patch.Base >= patch.World.Revision {
			return "", fmt.Errorf("invalid map revision chain")
		}
		patches = append(patches, patch)
		if err := q.QueryRowContext(ctx, "SELECT snapshot FROM world_revisions WHERE world_id=$1 AND revision=$2", id, patch.Base).Scan(&raw); err != nil {
			return "", err
		}
	}
	if len(patches) == 0 {
		return raw, nil
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return "", err
	}
	for i := len(patches) - 1; i >= 0; i-- {
		patch := patches[i]
		doc["world"], _ = json.Marshal(patch.World)
		for table, changes := range patch.Rows {
			if table == "library_files" {
				table = "libraryFiles"
			}
			var records []json.RawMessage
			if data := doc[table]; len(data) > 0 {
				if err := json.Unmarshal(data, &records); err != nil {
					return "", err
				}
			}
			seen := map[string]bool{}
			out := []json.RawMessage{}
			for _, record := range records {
				var key struct {
					ID    string `json:"id"`
					AgeID string `json:"ageId"`
				}
				if err := json.Unmarshal(record, &key); err != nil {
					return "", err
				}
				id := key.ID
				if key.AgeID != "" {
					id = key.AgeID + "/" + id
				}
				seen[id] = true
				if next, exists := changes[id]; exists {
					record = next
				}
				if string(record) != "null" {
					out = append(out, record)
				}
			}
			for id, record := range changes {
				if !seen[id] && string(record) != "null" {
					out = append(out, record)
				}
			}
			doc[table], _ = json.Marshal(out)
		}
	}
	data, err := json.Marshal(doc)
	return string(data), err
}
