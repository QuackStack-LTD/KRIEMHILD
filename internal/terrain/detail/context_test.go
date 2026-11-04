package detail

import (
	"kriemhild/internal/terrain/model"
	"math"
	"testing"
)

func TestMountainDetailHasRegionalReliefAndKeepsParentAnchors(t *testing.T) {
	p := &model.Planet{Spec: model.DefaultSpec(), Seed: 37, Grid: model.Grid{Width: 192, Height: 96, Fields: map[string][]float64{}}}
	// The old procedural refinement used climate anchors. New materialized
	// relief has its own version and deliberately removes that grid envelope.
	p.Spec.ReliefVersion = 0
	for k, v := range map[string]float64{"roughness": .8, "temperature": 0, "moisture": .5, "windX": 1} {
		a := make([]float64, 192*96)
		for i := range a {
			a[i] = v
		}
		p.Grid.Fields[k] = a
	}
	for x := 10; x < 20; x++ {
		pt := p.Grid.Point(x, 40)
		b, c := conditioned(p, pt[0], pt[1], 3000, .1)
		if math.Abs(b-3000) > 1e-9 || c != .1 {
			t.Fatal("parent anchor moved")
		}
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			b, _ := conditioned(p, .115+float64(x)*.0001, .205+float64(y)*.0001, 3000, .1)
			lo = math.Min(lo, b)
			hi = math.Max(hi, b)
		}
	}
	if hi-lo < 150 {
		t.Fatalf("regional mountain field is too flat: %.1f metres", hi-lo)
	}
}
