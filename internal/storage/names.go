package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/history"
)

// NameChange contains only affected identities and their map labels. It can be
// applied to a loaded view without replacing geometry or biological data.
type NameChange struct {
	Base            int                      `json:"base"`
	World           history.World            `json:"world"`
	Entities        []history.Entity         `json:"entities,omitempty"`
	Representations []history.Representation `json:"representations,omitempty"`
	Maps            []history.Map            `json:"maps,omitempty"`
	Terrains        []history.Terrain        `json:"terrains,omitempty"`
}

func (s *Store) RenameHistory(ctx context.Context, id string, c history.Command) (*NameChange, error) {
	if c.Kind != "rename-entity" && c.Kind != "rename-terrain" {
		return nil, fmt.Errorf("invalid rename operation")
	}
	if Actor(ctx) != "" && c.Revision < 0 {
		return nil, fmt.Errorf("expected revision is required")
	}
	tx, err := s.beginHistory(ctx, false)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d := &history.Document{}
	if d.World, err = readViewWorld(ctx, tx, id); err != nil {
		return nil, err
	}
	if c.Revision >= 0 && c.Revision != d.World.Revision {
		return nil, fmt.Errorf("World changed; reload before retrying")
	}
	where := "world_id=$1 AND age_id=$2"
	args := []any{id, c.AgeID}
	field := func(name string) string {
		if s.driver == "pgx" {
			return "data::jsonb->>'" + name + "'"
		}
		return "json_extract(data,'$." + name + "')"
	}
	if c.Kind == "rename-entity" {
		d.Entities, err = readViewRows[history.Entity](ctx, tx, "entities", "world_id=$1 AND id=$2", id, c.AgeID+"/"+c.EntityID)
		if err != nil {
			return nil, err
		}
		d.Representations, err = readViewRows[history.Representation](ctx, tx, "representations", where+" AND "+field("entityId")+"=$3", id, c.AgeID, c.EntityID)
		if err != nil {
			return nil, err
		}
		d.Maps, err = readViewRows[history.Map](ctx, tx, "maps", where+" AND "+field("entityId")+"=$3", id, c.AgeID, c.EntityID)
	} else {
		d.Terrains, err = readViewRows[history.Terrain](ctx, tx, "terrains", "world_id=$1 AND id=$2", id, c.AgeID+"/"+c.TerrainID)
		if err != nil {
			return nil, err
		}
		d.Maps, err = readViewRows[history.Map](ctx, tx, "maps", where+" AND "+field("terrainId")+"=$3", append(args, c.TerrainID)...)
	}
	if err != nil {
		return nil, err
	}
	rows := func() map[string]map[string]json.RawMessage {
		r := mapEditRows(d)
		delete(r, "relations")
		r["terrains"] = map[string]json.RawMessage{}
		for _, t := range d.Terrains {
			r["terrains"][t.AgeID+"/"+t.ID], _ = json.Marshal(t)
		}
		return r
	}
	before := rows()
	if c.Kind == "rename-entity" {
		err = d.RenameEntity(c.AgeID, c.EntityID, c.Name)
	} else {
		err = d.RenameTerrain(c.AgeID, c.TerrainID, c.Name)
	}
	if err != nil {
		return nil, err
	}
	base := d.World.Revision
	after := rows()
	if err = writeRecordPatch(ctx, tx, d, before, after, true); err != nil {
		return nil, err
	}
	change := &NameChange{Base: base, World: d.World}
	for table, records := range after {
		for key, raw := range records {
			if string(before[table][key]) != string(raw) {
				if err = change.add(table, raw); err != nil {
					return nil, err
				}
			}
		}
	}
	return change, tx.Commit()
}

func (c *NameChange) add(table string, raw json.RawMessage) error {
	switch table {
	case "entities":
		var v history.Entity
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		c.Entities = append(c.Entities, v)
	case "representations":
		var v history.Representation
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		c.Representations = append(c.Representations, v)
	case "maps":
		var v history.Map
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		c.Maps = append(c.Maps, v)
	case "terrains":
		var v history.Terrain
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		c.Terrains = append(c.Terrains, v)
	default:
		return fmt.Errorf("invalid name change")
	}
	return nil
}

// Fall back to the caller's regular read model for mixed, missing or long audit
// chains. Small consecutive name edits need no World or Biosphere reload.
func (s *Store) NameChangesSince(ctx context.Context, id string, from, to int) (*NameChange, error) {
	if from < 0 || to <= from || to-from > 128 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT revision,CASE WHEN snapshot LIKE '{"format":"KRIEMHILD-MAP-PATCH-1",%' THEN snapshot ELSE '' END FROM world_revisions WHERE world_id=$1 AND revision>$2 AND revision<=$3 ORDER BY revision`, id, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	next := from + 1
	latest := map[string]map[string]json.RawMessage{}
	c := &NameChange{Base: from}
	for rows.Next() {
		var revision int
		var raw []byte
		var p mapRevisionPatch
		if err = rows.Scan(&revision, &raw); err != nil {
			return nil, err
		}
		if revision != next || len(raw) == 0 || json.Unmarshal(raw, &p) != nil || !p.NameOnly || p.Base != revision-1 {
			return nil, nil
		}
		next++
		c.World = p.World
		for table, records := range p.Rows {
			if latest[table] == nil {
				latest[table] = map[string]json.RawMessage{}
			}
			for key, value := range records {
				latest[table][key] = value
			}
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if next != to+1 {
		return nil, nil
	}
	for table, records := range latest {
		for _, raw := range records {
			if err = c.add(table, raw); err != nil {
				return nil, err
			}
		}
	}
	return c, nil
}
