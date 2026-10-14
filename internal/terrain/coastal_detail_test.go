package terrain

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestMoistureRecyclingCannotCreateAtmosphericWater(t *testing.T) {
	for _, input := range []float64{0, .05, .5, 1} {
		for _, fraction := range []float64{0, .2, .8, 1} {
			for _, temp := range []float64{-15, 5, 25, 45} {
				remaining := input * fraction
				r := recycleVapor(input, remaining, temp, 2000)
				if r < 0 || remaining+r > input+1e-12 {
					t.Fatal("recycling creates vapor", input, remaining, r)
				}
			}
		}
	}
	if recycleVapor(1, .5, 25, 2000) <= recycleVapor(1, .5, 25, 150) || recycleVapor(1, .5, -10, 2000) != 0 {
		t.Fatal("recycling ignores energy or moisture limitation")
	}
}

func TestEuclideanTransformMatchesBruteForce(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		w, h := 19, 13
		mask := make([]bool, w*h)
		for _, i := range []int{0, 73, 188} {
			mask[i] = true
		}
		got := euclideanDistance(mask, w, h, wrap)
		for i, d := range got {
			want := math.Inf(1)
			for j, v := range mask {
				if !v {
					continue
				}
				dx := math.Abs(float64(i%w - j%w))
				if wrap {
					dx = math.Min(dx, float64(w)-dx)
				}
				want = math.Min(want, math.Hypot(dx, float64(i/w-j/w)))
			}
			if math.Abs(d-want) > 1e-10 {
				t.Fatalf("distance %d: %g != %g", i, d, want)
			}
		}
	}
}

func TestCoastRefinementAddsTopologyAndPersists(t *testing.T) {
	m := detailFixture(t)
	old := *m.World
	old.Coastline = nil
	legacy := NewDetailModel(&old)
	changed, emerged := 0, 0
	for y := 3.; y < float64(old.Options.Rows-3); y += .25 {
		for x := 3.; x < float64(old.Options.Columns-3); x += .25 {
			d := m.values(m.coastDistance, x, y)
			if math.Abs(d) > 2.5 {
				continue
			}
			a, b := m.Sample(x, y, 3), legacy.Sample(x, y, 3)
			if (a.WaterBody > 0) != (b.WaterBody > 0) {
				changed++
				if a.WaterBody == 0 {
					emerged++
					if a.Biome == "water" {
						t.Fatal("emerged coast inherits water biome")
					}
				}
				if math.Abs(d) >= m.World.Coastline.Band {
					t.Fatal("refinement escaped coastal band")
				}
			}
		}
	}
	if changed < 20 || emerged < 5 {
		t.Fatal("zoom only interpolates the parent footprint", changed, emerged)
	}
	raw, err := json.Marshal(m.World)
	if err != nil {
		t.Fatal(err)
	}
	var restored Environment
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	again := NewDetailModel(&restored)
	for k := 0; k < 100; k++ {
		x, y := 3+float64(k%25)*1.75, 3+float64(k%17)*1.75
		if !reflect.DeepEqual(m.Sample(x, y, 7), again.Sample(x, y, 7)) {
			t.Fatal("reload changed coastal field", x, y)
		}
	}
	t.Logf("new coastal samples=%d, emerged=%d", changed, emerged)
}

func TestSubcellIslandsAndDryInteriors(t *testing.T) {
	// A controlled straight marine coast makes newly emerged fragments and coves
	// distinguishable from interpolation of an already irregular parent raster.
	o := realismOptions(t, `{"columns":32,"rows":24,"seed":"subcell-coasts"}`)
	e := BuildEnvironment(o)
	for i := range e.Mask {
		land := i%32 >= 16
		e.Mask[i] = 0
		e.Fields["elevation"][i] = -30
		e.Fields["waterBody"][i] = 1
		e.Fields["waterLevel"][i] = 0
		e.Fields["waterDepth"][i] = 30
		e.Fields["flow"][i] = -1
		if land {
			e.Mask[i] = 1
			e.Fields["elevation"][i] = 30
			e.Fields["waterBody"][i] = 0
			e.Fields["waterDepth"][i] = 0
		}
	}
	m := NewDetailModel(e)
	islands, coves := 0, 0
	for y := 3.; y < 21; y += .0625 {
		for x := 14.; x < 18; x += .0625 {
			p := m.Sample(x, y, 4)
			if x < 15 && p.WaterBody == 0 {
				islands++
			}
			if x > 16 && p.WaterBody == 1 {
				coves++
			}
		}
	}
	if islands == 0 || coves == 0 {
		t.Fatal("no new subcell topology", islands, coves)
	}
	for _, x := range []float64{8.375, 23.625} {
		p := m.Sample(x, 12.25, 8)
		if (p.WaterBody == 0) != (x > 16) {
			t.Fatal("major basin/interior changed")
		}
	}
}
