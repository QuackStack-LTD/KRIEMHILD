package httpapi

import (
	"encoding/json"
	"fmt"
	"kriemhild/internal/legacyterrain"
	"math"
	"os"
	"path/filepath"
)

func (v *session) fixtureDetail(l, x, y int) ([]byte, error) {
	if l < 0 || l > terrain.DetailLevels || x < 0 || y < 0 || float64(x)*32/math.Exp2(float64(l)) >= float64(v.solver.W-1) || float64(y)*32/math.Exp2(float64(l)) >= float64(v.solver.H-1) {
		return nil, fmt.Errorf("detail tile outside world or supported levels")
	}
	key := detailKey(l, x, y)
	if v.tiles[key] {
		return os.ReadFile(filepath.Join(v.dir, filepath.FromSlash(key+".json")))
	}
	if v.detail == nil {
		v.detail = terrain.NewDetailModel(v.solver.Environment)
	}
	// Persist ancestors as well, so every region reference resolves in the file.
	if l > 0 {
		if _, err := v.fixtureDetail(l-1, x/2, y/2); err != nil {
			return nil, err
		}
	}
	tile, err := v.detail.Tile(l, x, y)
	if err != nil {
		return nil, err
	}
	if l > 0 {
		parentBytes, err := v.fixtureDetail(l-1, x/2, y/2)
		if err != nil {
			return nil, err
		}
		var parent terrain.DetailTile
		if err = json.Unmarshal(parentBytes, &parent); err != nil {
			return nil, err
		}
		inheritStoredParent(&tile, parent)
	}
	data, err := json.Marshal(tile)
	if err != nil {
		return nil, err
	}
	if err = v.putDetail(key, data); err != nil {
		return nil, err
	}
	return data, nil
}
