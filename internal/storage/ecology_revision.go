package storage

import (
	"context"
	"encoding/json"
	"kriemhild/internal/history"
)

// Only known projected patches can prove that ecology is unchanged. Full
// historical edits continue to invalidate the cache, including species names.
func (s *Store) EcologyUnchangedSince(ctx context.Context, world string, from, to int) bool {
	return s.projectedRecordsUnchanged(ctx, world, from, to, false)
}
func (s *Store) MapUnchangedSince(ctx context.Context, world string, from, to int) bool {
	return s.projectedRecordsUnchanged(ctx, world, from, to, true)
}
func (s *Store) projectedRecordsUnchanged(ctx context.Context, world string, from, to int, geography bool) bool {
	rows, err := s.db.QueryContext(ctx, `SELECT revision,CASE WHEN snapshot LIKE '{"format":"KRIEMHILD-MAP-PATCH-1",%' THEN snapshot ELSE '' END FROM world_revisions WHERE world_id=$1 AND revision>$2 AND revision<=$3 ORDER BY revision`, world, from, to)
	if err != nil {
		return false
	}
	defer rows.Close()
	next := from + 1
	for rows.Next() {
		var rev int
		var raw []byte
		if rows.Scan(&rev, &raw) != nil || rev != next || len(raw) == 0 {
			return false
		}
		next++
		var p mapRevisionPatch
		if json.Unmarshal(raw, &p) != nil {
			return false
		}
		if p.NameOnly && !geography {
			for _, raw := range p.Rows["entities"] {
				var e struct {
					Kind string `json:"kind"`
				}
				if json.Unmarshal(raw, &e) != nil || e.Kind == "lifeform" {
					return false
				}
			}
			continue
		}
		for table, changes := range p.Rows {
			switch table {
			case "library_files":
			case "entities":
				for _, r := range changes {
					var e struct {
						Kind string `json:"kind"`
					}
					if json.Unmarshal(r, &e) != nil || e.Kind == "" || e.Kind == "lifeform" || geography && history.Registry.Types[e.Kind].Category != "records" {
						return false
					}
				}
			default:
				if len(changes) > 0 {
					return false
				}
			}
		}
	}
	return rows.Err() == nil && next == to+1
}
