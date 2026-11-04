package tectonics

import (
	"kriemhild/internal/terrain/model"
	"math"
	"testing"
)

func TestNewOrogensCurveAndOldBoundaryIsStable(t *testing.T) {
	p := &model.Planet{Seed: 912345, Spec: model.DefaultSpec(), Plates: []model.Plate{{Center: [3]float64{-1, 0, 0}, Motion: [3]float64{1, 0, 0}}, {Center: [3]float64{1, 0, 0}, Motion: [3]float64{-1, 0, 0}}}}
	minOffset, maxOffset := 10., -10.
	for _, version := range []int{0, 1} {
		p.Spec.ReliefVersion = version
		for _, lat := range []float64{-.9, -.6, -.3, 0, .3, .6, .9} {
			best, offset := -1., 0.
			for i := -200; i <= 200; i++ {
				d := float64(i) * .001
				a := At(p, math.Pi/2+d, lat)
				if a.Boundary > best {
					best = a.Boundary
					offset = d
				}
			}
			if version == 0 && math.Abs(offset) > 1e-12 {
				t.Fatal("legacy plate boundary changed")
			}
			if version == 1 {
				minOffset = math.Min(minOffset, offset)
				maxOffset = math.Max(maxOffset, offset)
			}
		}
	}
	if maxOffset-minOffset < .02 {
		t.Fatal("new belt still follows one straight great circle")
	}
	for _, lat := range []float64{-1.2, 0, 1.2} {
		a, b := At(p, -math.Pi, lat), At(p, math.Pi, lat)
		if math.Abs(a.Convergence-b.Convergence) > 1e-12 {
			t.Fatal("plate curvature has a longitude seam")
		}
	}
}
