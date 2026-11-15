package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"kriemhild/internal/biosphere"
	"kriemhild/internal/history"
	"sort"
	"strings"
)

// Read models are deliberately separate from editable full history. Terrain
// display must never deserialize every species and habitat in every Age.
func readViewRows[T any](ctx context.Context, q historyQuery, table, condition string, args ...any) ([]T, error) {
	rows, err := q.QueryContext(ctx, "SELECT data FROM history_"+table+" WHERE "+condition+" ORDER BY id", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []T{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var value T
		if err = json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func readViewWorld(ctx context.Context, q historyQuery, id string) (history.World, error) {
	var w history.World
	var raw []byte
	if err := q.QueryRowContext(ctx, "SELECT data FROM history_worlds WHERE id=$1", id).Scan(&raw); err != nil {
		return w, err
	}
	err := json.Unmarshal(raw, &w)
	return w, err
}

func readMapHeader(ctx context.Context, q historyQuery, project string) (*history.Document, *history.Map, error) {
	var worldRaw, mapRaw []byte
	if err := q.QueryRowContext(ctx, "SELECT w.data,m.data FROM history_maps m JOIN history_worlds w ON w.id=m.world_id WHERE m.project_id=$1", project).Scan(&worldRaw, &mapRaw); err != nil {
		return nil, nil, err
	}
	d, m := &history.Document{}, &history.Map{}
	if err := json.Unmarshal(worldRaw, &d.World); err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(mapRaw, m); err != nil {
		return nil, nil, err
	}
	return d, m, nil
}

func (s *Store) MapHeader(ctx context.Context, project string) (*history.Document, *history.Map, error) {
	return readMapHeader(ctx, s.db, project)
}

func (s *Store) HistoryMetadata(ctx context.Context, id string) (*history.Document, error) {
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d := &history.Document{}
	if d.World, err = readViewWorld(ctx, tx, id); err != nil {
		return nil, err
	}
	if d.Ages, err = readViewRows[history.Age](ctx, tx, "ages", "world_id=$1", id); err != nil {
		return nil, err
	}
	if d.Terrains, err = readViewRows[history.Terrain](ctx, tx, "terrains", "world_id=$1", id); err != nil {
		return nil, err
	}
	if d.Maps, err = readViewRows[history.Map](ctx, tx, "maps", "world_id=$1", id); err != nil {
		return nil, err
	}
	return d, tx.Commit()
}

// MapProjection includes canonical names and illustrated-map designs, but no
// biological catalogue, populations, languages or content from other Ages.
func (s *Store) MapProjection(ctx context.Context, project string) (*history.Document, *history.Map, error) {
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	d, m, err := readMapHeader(ctx, tx, project)
	if err != nil {
		return nil, nil, err
	}
	if err = readMapProjection(ctx, tx, d, m.AgeID); err != nil {
		return nil, nil, err
	}
	return d, m, tx.Commit()
}

func readMapProjection(ctx context.Context, q historyQuery, d *history.Document, age string) (err error) {
	args := []any{d.World.ID, age}
	where := "world_id=$1 AND age_id=$2"
	if d.Terrains, err = readViewRows[history.Terrain](ctx, q, "terrains", where, args...); err != nil {
		return err
	}
	if d.Maps, err = readViewRows[history.Map](ctx, q, "maps", where, args...); err != nil {
		return err
	}
	if d.Representations, err = readViewRows[history.Representation](ctx, q, "representations", where, args...); err != nil {
		return err
	}
	if d.Relations, err = readViewRows[history.Relation](ctx, q, "relations", where, args...); err != nil {
		return err
	}
	// Avoid decoding language, content and biological profiles for a name lookup.
	type displayEntity struct {
		ID         string          `json:"id"`
		AgeID      string          `json:"ageId"`
		TerrainID  string          `json:"terrainId"`
		HomeMap    string          `json:"homeMap"`
		Name       string          `json:"name"`
		Kind       string          `json:"kind"`
		SourceID   string          `json:"sourceId"`
		Deleted    bool            `json:"deleted"`
		Properties json.RawMessage `json:"properties"`
	}
	entities, err := readViewRows[displayEntity](ctx, q, "entities", where+" AND kind='illustrated-map'", args...)
	if err != nil {
		return err
	}
	ids := map[string]bool{}
	for _, r := range d.Representations {
		ids[r.EntityID] = true
	}
	for _, m := range d.Maps {
		if m.EntityID != "" {
			ids[m.EntityID] = true
		}
	}
	keys := make([]string, 0, len(ids))
	for id := range ids {
		keys = append(keys, age+"/"+id)
	}
	sort.Strings(keys)
	// Fetch represented identities through the primary key, instead of scanning
	// all the non-spatial content in a large Age. Keep parameter counts bounded.
	for start := 0; start < len(keys); start += 256 {
		parameters := []any{d.World.ID}
		places := []string{}
		for _, key := range keys[start:min(start+256, len(keys))] {
			parameters = append(parameters, key)
			places = append(places, fmt.Sprintf("$%d", len(parameters)))
		}
		batch, err := readViewRows[displayEntity](ctx, q, "entities", "world_id=$1 AND id IN ("+strings.Join(places, ",")+")", parameters...)
		if err != nil {
			return err
		}
		entities = append(entities, batch...)
	}
	seen := map[string]bool{}
	for _, e := range entities {
		if e.AgeID != age {
			continue
		}
		if seen[e.ID] {
			continue
		}
		seen[e.ID] = true
		d.Entities = append(d.Entities, history.Entity{ID: e.ID, AgeID: e.AgeID, TerrainID: e.TerrainID, HomeMap: e.HomeMap, Name: e.Name, Kind: e.Kind, SourceID: e.SourceID, Deleted: e.Deleted, Properties: e.Properties})
	}
	sort.Slice(d.Entities, func(i, j int) bool { return d.Entities[i].ID < d.Entities[j].ID })

	return nil
}

func (s *Store) AgeMapProjection(ctx context.Context, id, age string) (*history.Document, error) {
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d := &history.Document{}
	if d.World, err = readViewWorld(ctx, tx, id); err != nil {
		return nil, err
	}
	if err = readMapProjection(ctx, tx, d, age); err != nil {
		return nil, err
	}
	return d, tx.Commit()
}

// EcologyView reads one terrain's saved populations and the selected Age's
// species labels. Per-cell suitability profiles and trait catalogues are not
// needed for geographic inspection and must not be retained by its cache.
func (s *Store) EcologyView(ctx context.Context, id, age, terrainID string) (*history.Document, error) {
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d := &history.Document{}
	if d.World, err = readViewWorld(ctx, tx, id); err != nil {
		return nil, err
	}
	if d.Maps, err = readViewRows[history.Map](ctx, tx, "maps", "world_id=$1 AND age_id=$2", id, age); err != nil {
		return nil, err
	}
	var raw []byte
	err = tx.QueryRowContext(ctx, "SELECT data FROM history_biospheres WHERE world_id=$1 AND id=$2", id, age+"/"+terrainID).Scan(&raw)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err == nil {
		var state ecologyState
		if err = json.Unmarshal(raw, &state); err != nil {
			return nil, err
		}
		d.Biospheres = []biosphere.State{state.state()}
		type speciesLabel struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Deleted bool   `json:"deleted"`
			Biology *struct {
				Group string   `json:"group"`
				Roles []string `json:"roles"`
			} `json:"biology"`
		}
		labels, err := readViewRows[speciesLabel](ctx, tx, "entities", "world_id=$1 AND age_id=$2 AND kind='lifeform'", id, age)
		if err != nil {
			return nil, err
		}
		for _, e := range labels {
			if e.Biology != nil {
				d.Entities = append(d.Entities, history.Entity{ID: e.ID, AgeID: age, Name: e.Name, Deleted: e.Deleted, Biology: &biosphere.Lifeform{Group: e.Biology.Group, Roles: e.Biology.Roles}})
			}
		}
	}
	return d, tx.Commit()
}

type ecologyState struct {
	SourceSnapshot string `json:"sourceSnapshot"`
	ID             string `json:"id"`
	AgeID          string `json:"ageId"`
	TerrainID      string `json:"terrainId"`
	Source         string `json:"source"`
	Stale          bool   `json:"stale"`
	Habitats       []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Kind        string `json:"kind"`
		Description string `json:"description"`
		Cells       []int  `json:"cells"`
	} `json:"habitats"`
	Occurrences []struct {
		SpeciesID string   `json:"speciesId"`
		HabitatID string   `json:"habitatId"`
		Abundance float64  `json:"abundance"`
		Status    string   `json:"status"`
		Seasons   []string `json:"seasons"`
	} `json:"occurrences"`
}

func (v ecologyState) state() biosphere.State {
	s := biosphere.State{ID: v.ID, AgeID: v.AgeID, TerrainID: v.TerrainID, Source: v.Source, Stale: v.Stale}
	s.SourceSnapshot = v.SourceSnapshot
	for _, h := range v.Habitats {
		s.Habitats = append(s.Habitats, biosphere.Habitat{ID: h.ID, Name: h.Name, Kind: h.Kind, Description: h.Description, Cells: h.Cells})
	}
	for _, o := range v.Occurrences {
		s.Occurrences = append(s.Occurrences, biosphere.Occurrence{SpeciesID: o.SpeciesID, HabitatID: o.HabitatID, Abundance: o.Abundance, Status: o.Status, Seasons: o.Seasons})
	}
	return s
}

type discardedJSON struct{}

func (*discardedJSON) UnmarshalJSON([]byte) error { return nil }

func decodeWorkspaceBiosphere(raw []byte, state *biosphere.State) error {
	type stateFields biosphere.State
	type habitatWithoutProfiles struct {
		biosphere.Habitat
		Profiles discardedJSON `json:"profiles"`
	}
	view := struct {
		*stateFields
		Habitats []habitatWithoutProfiles `json:"habitats"`
	}{stateFields: (*stateFields)(state)}
	if err := json.Unmarshal(raw, &view); err != nil {
		return err
	}
	state.Habitats = make([]biosphere.Habitat, len(view.Habitats))
	for i, h := range view.Habitats {
		state.Habitats[i] = h.Habitat
	}
	return nil
}

func (s *Store) HistoryWorkspace(ctx context.Context, id string) (*history.Document, error) {
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d, err := readHistoryProjected(ctx, tx, id, nil, true)
	if err != nil {
		return nil, err
	}
	return d, tx.Commit()
}

// Atlas/graph navigation does not need the biological catalogue or generation
// profiles. Full workspace records are loaded when an entity editor needs them.
func (s *Store) HistoryAtlas(ctx context.Context, id string) (*history.Document, error) {
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d, err := readHistoryProjected(ctx, tx, id, nil, true, true)
	if err != nil {
		return nil, err
	}
	return d, tx.Commit()
}

func readBiosphereContext(ctx context.Context, q historyQuery, id, age, terrainID string, workspace bool) (*history.Document, error) {
	d := &history.Document{}
	var err error
	if d.World, err = readViewWorld(ctx, q, id); err != nil {
		return nil, err
	}
	if d.Maps, err = readViewRows[history.Map](ctx, q, "maps", "world_id=$1 AND age_id=$2", id, age); err != nil {
		return nil, err
	}
	var raw []byte
	err = q.QueryRowContext(ctx, "SELECT data FROM history_biospheres WHERE world_id=$1 AND id=$2", id, age+"/"+terrainID).Scan(&raw)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err == nil {
		var state biosphere.State
		if workspace {
			err = decodeWorkspaceBiosphere(raw, &state)
		} else {
			err = json.Unmarshal(raw, &state)
		}
		if err != nil {
			return nil, err
		}
		d.Biospheres = []biosphere.State{state}
	}
	if !workspace {
		if d.Entities, err = readViewRows[history.Entity](ctx, q, "entities", "world_id=$1 AND age_id=$2 AND kind='lifeform'", id, age); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func (s *Store) BiosphereContext(ctx context.Context, id, age, terrainID string, workspace bool) (*history.Document, error) {
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d, err := readBiosphereContext(ctx, tx, id, age, terrainID, workspace)
	if err != nil {
		return nil, err
	}
	return d, tx.Commit()
}
