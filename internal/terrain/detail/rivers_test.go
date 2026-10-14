package detail

import (
	"kriemhild/internal/terrain/model"
	"math"
	"testing"
)

func TestRiverIndexCrossesLongitudeSeam(t *testing.T) {
	p := &model.Planet{Rivers: []model.River{{Path: []model.Point{{math.Pi - .01, 0}, {-math.Pi + .01, 0}}, Bed: []float64{10, 9}, Surface: []float64{11, 10}, Width: []float64{100, 100}}}}
	s := New(p)
	for _, x := range []int{0, 127} {
		segments := s.bins[32*128+x]
		if len(segments) != 1 {
			t.Fatalf("seam bin %d has %d segments", x, len(segments))
		}
		if math.Abs(segments[0].b[0]-segments[0].a[0]-.02) > 1e-9 {
			t.Fatal("river crosses the long way round")
		}
	}
	if len(s.bins[32*128+64]) != 0 {
		t.Fatal("seam river polluted opposite hemisphere")
	}
}
