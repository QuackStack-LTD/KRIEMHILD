package storage

import (
	"context"
	"kriemhild/internal/history"
	"kriemhild/internal/world"
)

func (s *Store) AttachTerrain(ctx context.Context, id, age, project, name string, w, h int, parts map[string][]byte, state *world.State) (*history.Document, history.Map, error) {
	tx, err := s.beginHistory(ctx, false)
	if err != nil {
		return nil, history.Map{}, err
	}
	defer tx.Rollback()
	d, err := readHistory(ctx, tx, id)
	if err != nil {
		return nil, history.Map{}, err
	}
	if m := d.Project(project); m != nil {
		return d, *m, nil
	}
	old := d.World.Revision
	snapshot, err := saveSnapshot(ctx, tx, "", parts)
	if err != nil {
		return nil, history.Map{}, err
	}
	m, err := d.AddTerrain(age, name, project, w, h)
	if err != nil {
		return nil, m, err
	}
	d.Map(age, m.ID).Snapshot = snapshot
	m.Snapshot = snapshot
	d.Sync(m, state)
	if err = writeHistory(ctx, tx, d, old, false); err != nil {
		return nil, m, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM kriemhild_projects WHERE id=$1", project); err != nil {
		return nil, m, err
	}
	return d, m, tx.Commit()
}
