package terrain

import (
	"math"
	"reflect"
	"testing"
)

func detailFixture(t *testing.T) *DetailModel {
	t.Helper()
	o, err := DecodeEnvironment([]byte(`{"columns":64,"rows":48,"realism":true,"seed":"detail-audit"}`))
	if err != nil {
		t.Fatal(err)
	}
	return NewDetailModel(BuildEnvironment(o))
}

func TestDetailPreservesParentsWaterAndWorld(t *testing.T) {
	m := detailFixture(t)
	before := append([]float64{}, m.World.Fields["elevation"]...)
	variation := 0
	for level := 1; level <= DetailLevels; level++ {
		step := math.Exp2(float64(1 - level))
		for k := 0; k < 40; k++ {
			x, y := float64(10+k%10)*step, float64(8+k/10)*step
			parent, child := m.Sample(x, y, level-1), m.Sample(x, y, level)
			if parent.Elevation != child.Elevation || parent.WaterBody != child.WaterBody {
				t.Fatal("child changed a parent node", level, x, y)
			}
			x += step * .5
			y += step * .5
			p := m.Sample(x, y, level)
			base := m.Sample(x, y, 0)
			if p.Elevation != base.Elevation {
				variation++
			}
			if p.WaterBody != base.WaterBody {
				t.Fatal("refinement changed water topology")
			}
			if p.WaterBody > 0 && (p.WaterDepth <= 0 || math.Abs(p.WaterDepth-(p.WaterLevel-p.Elevation)) > 1e-8) {
				t.Fatal("invalid local water surface")
			}
			if !finite(p.Elevation) || math.Abs(p.Elevation-base.Elevation) > 250.01 {
				t.Fatal("detail replaced regional structure")
			}
		}
	}
	if variation < 20 {
		t.Fatal("no real elevation refinement")
	}
	if !reflect.DeepEqual(before, m.World.Fields["elevation"]) {
		t.Fatal("detail mutated world")
	}
	for i := range m.World.Mask {
		x, y := float64(i%m.World.Options.Columns), float64(i/m.World.Options.Columns)
		if m.Sample(x, y, 8).Elevation != before[i] {
			t.Fatal("world anchor changed", i)
		}
	}
}

func TestDetailTileEdgesAndRevisits(t *testing.T) {
	m := detailFixture(t)
	for _, level := range []int{1, 3, 5, 8} {
		a, err := m.Tile(level, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := m.Tile(level, 2, 1)
		c, _ := m.Tile(level, 1, 2)
		for i := 0; i < a.Size; i++ {
			if !reflect.DeepEqual(a.Points[i*a.Size+32], b.Points[i*a.Size]) || !reflect.DeepEqual(a.Points[32*a.Size+i], c.Points[i]) {
				t.Fatal("tile seam", level, i)
			}
		}
		_, _ = m.Tile(8, 40, 50)
		again, _ := m.Tile(level, 1, 1)
		if !reflect.DeepEqual(a, again) {
			t.Fatal("visit order changed world")
		}
		fresh := NewDetailModel(m.World)
		rebuilt, _ := fresh.Tile(level, 1, 1)
		if !reflect.DeepEqual(a, rebuilt) {
			t.Fatal("cache eviction changed geography")
		}
	}
	for _, args := range [][3]int{{-1, 0, 0}, {9, 0, 0}, {0, -1, 0}, {0, 100, 0}} {
		if _, err := m.Tile(args[0], args[1], args[2]); err == nil {
			t.Fatal("invalid tile accepted")
		}
	}
}

func TestDetailDrainageIdentityAndGrade(t *testing.T) {
	m := detailFixture(t)
	ids := map[string]bool{}
	if len(m.Features) == 0 {
		t.Fatal("no drainage")
	}
	for _, f := range m.Features {
		if ids[f.ID] || f.ParentID == "" {
			t.Fatal("unstable feature identity")
		}
		ids[f.ID] = true
		for i := 1; i < len(f.Path); i++ {
			if f.Path[i][2] > f.Path[i-1][2] {
				t.Fatal("water runs uphill")
			}
		}
		p := f.Path[0]
		a, b := m.Sample(p[0], p[1], 0), m.Sample(p[0], p[1], 8)
		if a.Elevation != b.Elevation {
			t.Fatal("drainage junction relocated")
		}
	}
}

func TestDetailShoreIsContinuous(t *testing.T) {
	m := detailFixture(t)
	w := m.World.Options.Columns
	checked := 0
	for i := range m.World.Mask {
		if i%w == w-1 || m.World.Mask[i] == m.World.Mask[i+1] {
			continue
		}
		x, y := float64(i%w)+.5, float64(i/w)
		a, b := m.Sample(x-1e-6, y, 8), m.Sample(x+1e-6, y, 8)
		if math.Abs(a.Elevation-b.Elevation) > .05 {
			t.Fatalf("shoreline cliff introduced by refinement at %d: %v -> %v", i, a.Elevation, b.Elevation)
		}
		checked++
	}
	if checked < 10 {
		t.Fatal("missing shoreline coverage")
	}
}

func TestDetailGeomorphStartsOnParentTriangles(t *testing.T) {
	m := detailFixture(t)
	tile, err := m.Tile(1, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	for y := 1; y < 32; y += 2 {
		for x := 1; x < 32; x += 2 {
			// Odd/odd child nodes sit on the parent triangle's diagonal.
			a := m.Sample(float64(x+1)/2, float64(y-1)/2, 0).Elevation
			b := m.Sample(float64(x-1)/2, float64(y+1)/2, 0).Elevation
			if math.Abs(tile.Points[y*33+x].Parent-(a+b)/2) > 1e-8 {
				t.Fatal("new child appears above/below the existing parent mesh")
			}
		}
	}
}
