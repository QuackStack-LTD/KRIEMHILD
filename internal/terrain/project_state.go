package terrain

import (
	"fmt"
	"math"
	"math/bits"
)

const DetailEngineVersion = "kriemhild-detail-2"

type DetailState struct {
	Seed      uint32          `json:"seed"`
	Foothills []float64       `json:"foothills"`
	Features  []DetailFeature `json:"features"`
}

func (m *DetailModel) ProjectState() DetailState { return DetailState{m.Seed, m.foothills, m.Features} }
func RestoreDetailModel(e *Environment, s DetailState) (*DetailModel, error) {
	if e == nil || len(s.Foothills) != len(e.Mask) {
		return nil, fmt.Errorf("invalid detail model dimensions")
	}
	for _, v := range s.Foothills {
		if !finite(v) {
			return nil, fmt.Errorf("invalid foothill field")
		}
	}
	ids := map[string]bool{}
	for _, f := range s.Features {
		if f.ID == "" || ids[f.ID] || f.ParentID == "" || (f.Kind != "river" && f.Kind != "canal") || len(f.Path) < 2 || len(f.Path) > 4096 || (len(f.Widths) != 0 && len(f.Widths) != len(f.Path)) {
			return nil, fmt.Errorf("invalid detail feature")
		}
		ids[f.ID] = true
		for _, p := range f.Path {
			if !finite(p[0]) || !finite(p[1]) || !finite(p[2]) || p[0] < 0 || p[1] < 0 || p[0] > float64(e.Options.Columns-1) || p[1] > float64(e.Options.Rows-1) {
				return nil, fmt.Errorf("feature outside world")
			}
		}
	}
	m := &DetailModel{World: e, Seed: s.Seed, foothills: s.Foothills, Features: s.Features}
	m.indexDrainage()
	return m, nil
}

// Restore stored state, rebuilding only disposable solver indexes. This never
// invokes world generation, WFC collapse, climate, drainage or terrain synthesis.
func RestoreProjectSolver(s *Solver, c Config, e *Environment, edit Snapshot) error {
	if len(c.Continents) > 128 || len(c.Climates) > 128 {
		return fmt.Errorf("too many world classifications")
	}
	r, err := Compile(c)
	if err != nil {
		return err
	}
	if s.W < 16 || s.W > 256 || s.H < 16 || s.H > 256 || s.N != s.W*s.H || s.N > 65536 || s.Radius2 < 1 || s.Radius2 > 25 || s.T != len(c.Types) || s.K != len(c.Continents) || s.Z != len(c.Climates) || !finite(s.Stability) {
		return fmt.Errorf("invalid world dimensions or solver configuration")
	}
	if len(s.Dom) != s.N || len(s.Locked) != s.N || len(s.Pinned) != s.N || (len(s.EnvironmentMasks) != 0 && len(s.EnvironmentMasks) != s.N) || (len(s.Prefer) != 0 && len(s.Prefer) != s.N) || (len(s.ContShare) != 0 && len(s.ContShare) != s.N*s.K) || (len(s.ClimShare) != 0 && len(s.ClimShare) != s.N*s.Z) {
		return fmt.Errorf("invalid world array lengths")
	}
	full := uint32((uint64(1) << s.T) - 1)
	validSnapshot := func(v Snapshot) bool {
		if v.Status != "done" || len(v.Dom) != s.N || len(v.Locked) != s.N || len(v.Pinned) != s.N {
			return false
		}
		for i, m := range v.Dom {
			if bits.OnesCount32(m) != 1 || m&full != m || v.Locked[i] < 0 || v.Locked[i] > 1 || v.Pinned[i] < 0 || v.Pinned[i] > 1 {
				return false
			}
		}
		return true
	}
	if !validSnapshot(s.Snapshot()) || !validSnapshot(edit) {
		return fmt.Errorf("project must contain a completed world and valid edit layer")
	}
	for _, m := range s.EnvironmentMasks {
		if m == 0 || m&full != m {
			return fmt.Errorf("invalid environmental mask")
		}
	}
	for _, p := range s.Prefer {
		if p < -1 || p >= s.T {
			return fmt.Errorf("invalid preferred biome")
		}
	}
	if e != nil {
		rawOptions := e.Options
		if rawOptions.Columns != s.W || rawOptions.Rows != s.H || len(e.Mask) != s.N || len(e.Heights) != s.N {
			return fmt.Errorf("environment dimensions do not match world")
		}
		required := append(append([]string{}, FieldNames...), SurfaceFields...)
		if e.Options.Realism {
			required = append(required, RealismFields...)
		}
		for _, name := range required {
			if len(e.Fields[name]) != s.N {
				return fmt.Errorf("missing physical field %s", name)
			}
		}
		for name, f := range e.Fields {
			if len(f) != s.N {
				return fmt.Errorf("invalid field %s", name)
			}
			for _, v := range f {
				if !finite(v) {
					return fmt.Errorf("non-finite field %s", name)
				}
			}
		}
		for i, v := range e.Fields["flow"] {
			j := int(v)
			if v != float64(j) || j < -1 || j >= s.N || j == i {
				return fmt.Errorf("invalid drainage reference")
			}
			if j >= 0 && e.Fields["drainageElevation"][j] > e.Fields["drainageElevation"][i] {
				return fmt.Errorf("uphill drainage reference")
			}
		}
		degree := make([]int, s.N)
		for _, v := range e.Fields["flow"] {
			if v >= 0 {
				degree[int(v)]++
			}
		}
		queue := []int{}
		for i, d := range degree {
			if d == 0 {
				queue = append(queue, i)
			}
		}
		for head := 0; head < len(queue); head++ {
			j := int(e.Fields["flow"][queue[head]])
			if j >= 0 {
				degree[j]--
				if degree[j] == 0 {
					queue = append(queue, j)
				}
			}
		}
		if len(queue) != s.N {
			return fmt.Errorf("cyclic drainage network")
		}
		cellsValid := func(cells []int) bool {
			for _, i := range cells {
				if i < 0 || i >= s.N {
					return false
				}
			}
			return true
		}
		for _, body := range e.WaterBodies {
			if !cellsValid(body.Cells) || body.ID < 1 {
				return fmt.Errorf("invalid water body")
			}
		}
		for _, reef := range e.Reefs {
			if !cellsValid(reef.Cells) {
				return fmt.Errorf("invalid reef region")
			}
		}
		if e.Hydrology != nil {
			if diagnostics := e.ValidateHydrology(); len(diagnostics) > 0 {
				return fmt.Errorf("invalid hydrology %s: %s", diagnostics[0].Object, diagnostics[0].Reason)
			}
		}
		if e.Options.Realism && e.Entities == nil {
			return fmt.Errorf("missing geographic entities")
		}
		if e.Entities != nil {
			for _, river := range e.Entities.Rivers {
				if len(river.Path) < 2 || !cellsValid(river.Path) || river.Source != river.Path[0] || river.Mouth != river.Path[len(river.Path)-1] {
					return fmt.Errorf("invalid river entity")
				}
			}
			for _, volcano := range e.Entities.Volcanoes {
				if !cellsValid(append([]int{volcano.Cell}, volcano.Lava...)) || !cellsValid(volcano.Ash) {
					return fmt.Errorf("invalid volcanic entity")
				}
			}
			for _, region := range e.Entities.DuneFields {
				if !cellsValid(region.Cells) {
					return fmt.Errorf("invalid dune region")
				}
			}
			for _, settlement := range e.Entities.Settlements {
				if !cellsValid(append([]int{settlement.Cell}, settlement.Farms...)) {
					return fmt.Errorf("invalid settlement")
				}
			}
		}
	}
	s.Rules = r
	s.Environment = e
	s.Full = full
	s.Dom = edit.Dom
	s.Locked = edit.Locked
	s.Pinned = edit.Pinned
	s.Status = "done"
	s.Open = 0
	s.Queue = nil
	s.InQueue = make([]bool, s.N)
	s.Pos = make([]int, s.N)
	s.Buckets = make([][]int, s.T+1)
	s.Compat = map[uint32]uint32{}
	s.TrailCell = nil
	s.TrailMask = nil
	s.Recording = false
	s.Offsets = nil
	rad := int(math.Sqrt(float64(s.Radius2)))
	for y := -rad; y <= rad; y++ {
		for x := -rad; x <= rad; x++ {
			if (x != 0 || y != 0) && x*x+y*y <= s.Radius2 {
				s.Offsets = append(s.Offsets, [2]int{x, y})
			}
		}
	}
	return nil
}

func ValidateDetailTile(t DetailTile, e *Environment) error {
	if e == nil || t.Level < 0 || t.Level > DetailLevels || t.X < 0 || t.Y < 0 || t.Size != 33 || len(t.Points) != 1089 || t.Step != math.Exp2(-float64(t.Level)) || float64(t.X)*32*t.Step >= float64(e.Options.Columns-1) || float64(t.Y)*32*t.Step >= float64(e.Options.Rows-1) {
		return fmt.Errorf("invalid detail tile coordinates or dimensions")
	}
	for _, p := range t.Points {
		if p.Cell < 0 || p.Cell >= len(e.Mask) || !finite(p.Elevation) || !finite(p.Parent) || !finite(p.WaterDepth) || p.WaterBody < 0 || p.WaterDepth < 0 {
			return fmt.Errorf("invalid detail sample")
		}
	}
	return nil
}
