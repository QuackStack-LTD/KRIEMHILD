package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"kriemhild/internal/terrain"
	"net/http"
)

type authoredClimate struct {
	Signature string                `json:"signature"`
	State     *terrain.ClimateState `json:"state"`
}

func (s *Server) worldClimateOverlay(w http.ResponseWriter, r *http.Request) {
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	c := v.climateLayer()
	if c == nil {
		respond(w, 200, map[string]any{"available": false})
		return
	}
	e := *v.solver.Environment
	e.Climate = c
	display := renderEnvironment(&e)
	respond(w, 200, map[string]any{"available": true, "revision": v.climateKey(), "seasonalZones": display.SeasonalZones, "fields": map[string][]float64{"seasonalClimate": display.Fields["seasonalClimate"], "seasonCount": display.Fields["seasonCount"]}})
}

func (v *session) climateKey() string {
	b, _ := json.Marshal(v.authored().Operations)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
func (v *session) climateLayer() *terrain.ClimateState {
	base := v.solver.Environment
	if base == nil || base.Climate == nil {
		return nil
	}
	if len(v.authored().Operations) == 0 {
		return base.Climate
	}
	key := v.climateKey()
	if v.climate.State != nil && v.climate.Signature == key {
		return v.climate.State
	}
	edited := v.authored().Environment(base)
	edited.RebuildClimateForSurface(base)
	v.climate = authoredClimate{key, edited.Climate}
	return v.climate.State
}

type climateChunk struct {
	X    int    `json:"x"`
	Y    int    `json:"y"`
	File string `json:"file"`
}
type climateArchive struct {
	Metadata  string         `json:"metadata"`
	Chunks    []climateChunk `json:"chunks"`
	Signature string         `json:"signature,omitempty"`
}

func writeClimateArchive(state *terrain.ClimateState, prefix string, add func(string, any) error) (*climateArchive, error) {
	if state == nil {
		return nil, nil
	}
	w, h := state.Scale.GridWidth, state.Scale.GridHeight
	if err := state.Validate(w, h); err != nil {
		return nil, err
	}
	a := &climateArchive{Metadata: prefix + "/metadata.json", Chunks: []climateChunk{}}
	meta := *state
	meta.Cells = nil
	if err := add(a.Metadata, meta); err != nil {
		return nil, err
	}
	for y := 0; y < h; y += 32 {
		for x := 0; x < w; x += 32 {
			cells := []terrain.ClimateCell{}
			for yy := y; yy < min(y+32, h); yy++ {
				cells = append(cells, state.Cells[yy*w+x:yy*w+min(x+32, w)]...)
			}
			file := fmt.Sprintf("%s/cells/%d/%d.json", prefix, x/32, y/32)
			if err := add(file, cells); err != nil {
				return nil, err
			}
			a.Chunks = append(a.Chunks, climateChunk{x / 32, y / 32, file})
		}
	}
	return a, nil
}

func readClimateArchive(a *climateArchive, prefix string, w, h int, data map[string][]byte, used map[string]bool) (*terrain.ClimateState, error) {
	if a == nil {
		return nil, nil
	}
	if a.Metadata != prefix+"/metadata.json" || len(a.Chunks) != ((w+31)/32)*((h+31)/32) {
		return nil, fmt.Errorf("invalid climate layer manifest")
	}
	var c terrain.ClimateState
	if err := json.Unmarshal(data[a.Metadata], &c); err != nil {
		return nil, err
	}
	used[a.Metadata] = true
	if len(c.Cells) != 0 {
		return nil, fmt.Errorf("climate cells must use the referenced chunks")
	}
	c.Cells = make([]terrain.ClimateCell, w*h)
	for _, chunk := range a.Chunks {
		file := fmt.Sprintf("%s/cells/%d/%d.json", prefix, chunk.X, chunk.Y)
		x, y := chunk.X*32, chunk.Y*32
		if x < 0 || y < 0 || x >= w || y >= h || chunk.File != file || used[file] {
			return nil, fmt.Errorf("invalid or duplicate climate chunk")
		}
		var cells []terrain.ClimateCell
		if err := json.Unmarshal(data[file], &cells); err != nil {
			return nil, err
		}
		if len(cells) != min(32, w-x)*min(32, h-y) {
			return nil, fmt.Errorf("climate chunk dimensions disagree")
		}
		at := 0
		for yy := y; yy < min(y+32, h); yy++ {
			n := min(32, w-x)
			copy(c.Cells[yy*w+x:yy*w+x+n], cells[at:at+n])
			at += n
		}
		used[file] = true
	}
	return &c, c.Validate(w, h)
}
