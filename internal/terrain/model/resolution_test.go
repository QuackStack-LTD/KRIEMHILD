package model

import (
	"math"
	"testing"
)

func TestFinerSamplingWithinSavedTriangleAddsNoRelief(t *testing.T) {
	s := &Surface{Width: 4, Height: 2, Bed: []float32{0, 100, 200, 300, 400, 500, 600, 700}}
	point := func(u float64) (float64, float64) { return -math.Pi + Tau*u/4, math.Pi/2 - math.Pi*(.6)/2 }
	loLon, lat := point(.1)
	hiLon, _ := point(.7)
	lo, hi := s.At(s.Bed, loLon, lat), s.At(s.Bed, hiLon, lat)
	for _, density := range []int{256, 512, 1024, 2048} {
		for i := 0; i <= density; i++ {
			f := float64(i) / float64(density)
			got := s.At(s.Bed, loLon+(hiLon-loLon)*f, lat)
			if math.Abs(got-(lo+(hi-lo)*f)) > 1e-10 {
				t.Fatalf("density %d invents intermediate relief", density)
			}
		}
	}
}
