package landmass

import (
	"context"
	"testing"
)

func TestConnectedLandIncludesSeamAndSeparatesIslands(t *testing.T) {
	w, h := 12, 7
	v := make([]float32, w*h)
	for i := range v {
		v[i] = -1
	}
	for _, p := range [][2]int{{0, 2}, {11, 2}, {0, 3}, {11, 3}, {5, 2}, {5, 3}, {6, 3}} {
		v[p[1]*w+p[0]] = 1
	}
	c, err := Components(context.Background(), w, h, v)
	if err != nil {
		t.Fatal(err)
	}
	if len(c) != 2 {
		t.Fatal("seam or island connectivity wrong", len(c))
	}
	cells := 0
	for _, land := range c {
		cells += land.Cells
		if len(land.Paths) == 0 {
			t.Fatal("missing coastline")
		}
		for _, ring := range land.Paths {
			if ring[0] != ring[len(ring)-1] {
				t.Fatal("unclosed coastline")
			}
		}
	}
	if cells != 7 {
		t.Fatal("lost land cells")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = Components(ctx, w, h, v); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestConnectedLandClosesPolarAndEntireWorldAreas(t *testing.T) {
	for _, all := range []bool{false, true} {
		const w, h = 12, 8
		v := make([]float32, w*h)
		for i := range v {
			v[i] = -1
			if all || i/w < 3 {
				v[i] = 1
			}
		}
		c, err := Components(context.Background(), w, h, v)
		if err != nil || len(c) != 1 {
			t.Fatalf("all=%v: %v %v", all, c, err)
		}
		inside := func(x, y float64) bool {
			in := false
			for _, ring := range c[0].Paths {
				for i := 1; i < len(ring); i++ {
					a, b := ring[i-1], ring[i]
					if (a[1] > y) != (b[1] > y) && x < (b[0]-a[0])*(y-a[1])/(b[1]-a[1])+a[0] {
						in = !in
					}
				}
			}
			return in
		}
		for _, x := range []float64{.001, 1, 1.999} {
			if !inside(x, .001) || inside(x, .99) != all {
				t.Fatalf("polar area lost at %v, all=%v", x, all)
			}
		}
	}
}
