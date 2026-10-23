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

func TestBedQueryMatchesFullSampleAcrossChannelsAndWater(t *testing.T) {
	p := &model.Planet{Spec: model.DefaultSpec(), Surface: &model.Surface{Width: 16, Height: 8}, Grid: model.Grid{Width: 4, Height: 2, Fields: map[string][]float64{}}, Rivers: []model.River{{Path: []model.Point{{-.5, 0}, {.5, 0}}, Bed: []float64{200, 100}, Surface: []float64{201, 101}, Width: []float64{100000, 100000}}}}
	for i := 0; i < 128; i++ {
		p.Surface.Bed = append(p.Surface.Bed, float32(150+i))
		p.Surface.Coast = append(p.Surface.Coast, float32(i%16-8))
		p.Surface.Roughness = append(p.Surface.Roughness, .3)
		p.Surface.LakeLevel = append(p.Surface.LakeLevel, 180)
	}
	s := New(p)
	for j := -20; j <= 20; j++ {
		for i := -40; i <= 40; i++ {
			lon, lat := float64(i)*.1, float64(j)*.04
			if a, b := s.Bed(lon, lat), s.Sample(lon, lat).Bed; a != b {
				t.Fatalf("relief-only sample differs at %.4f,%.4f: %v vs %v", lon, lat, a, b)
			}
		}
	}
}

func TestChannelEnvelopeNeverRejectsInfluentialReaches(t *testing.T) {
	// Compare the optimized query to the same sampler with all envelopes opened.
	// This exercises varying widths, seam copies, polar geometry and channels
	// near their influence boundary without prescribing invented river routes.
	p := &model.Planet{Spec: model.DefaultSpec(), Surface: &model.Surface{Width: 16, Height: 8}, Grid: model.Grid{Width: 4, Height: 2, Fields: map[string][]float64{}}, Rivers: []model.River{
		{Path: []model.Point{{3.1, .7}, {-3.1, .75}}, Bed: []float64{20, 10}, Surface: []float64{21, 11}, Width: []float64{500, 50000}},
		{Path: []model.Point{{-.7, 1.55}, {.7, 1.56}}, Bed: []float64{40, 30}, Surface: []float64{41, 31}, Width: []float64{30000, 1000}},
		{Path: []model.Point{{-.5, 0}, {.5, .1}}, Bed: []float64{200, 100}, Surface: []float64{201, 101}, Width: []float64{100000, 1000}},
	}}
	for i := 0; i < 128; i++ {
		p.Surface.Bed = append(p.Surface.Bed, 500)
		p.Surface.Coast = append(p.Surface.Coast, 100)
		p.Surface.Roughness = append(p.Surface.Roughness, .3)
		p.Surface.LakeLevel = append(p.Surface.LakeLevel, 0)
	}
	fast := newSampler(p)
	reference := newSampler(p)
	unique := map[segment]bool{}
	for _, bin := range fast.bins {
		for _, seg := range bin {
			unique[seg] = true
		}
	}
	for i := range reference.bins {
		reference.bins[i] = nil
	}
	bx := func(lon float64) int { return int((lon + math.Pi) / model.Tau * 128) }
	by := func(lat float64) int { return int((.5 - lat/math.Pi) * 64) }
	for seg := range unique {
		seg.padding = math.Inf(1)
		a, b := seg.a, seg.b
		for y := max(0, min(by(a[1]), by(b[1]))-1); y <= min(63, max(by(a[1]), by(b[1]))+1); y++ {
			for x := max(0, min(bx(a[0]), bx(b[0]))-1); x <= min(127, max(bx(a[0]), bx(b[0]))+1); x++ {
				reference.bins[y*128+x] = append(reference.bins[y*128+x], seg)
			}
		}
	}
	for _, r := range p.Rivers {
		for k := 0; k <= 40; k++ {
			tt := float64(k) / 40
			lon := model.Wrap(r.Path[0][0] + model.Wrap(r.Path[1][0]-r.Path[0][0])*tt)
			lat := r.Path[0][1] + (r.Path[1][1]-r.Path[0][1])*tt
			for _, delta := range []float64{-.03, -.005, -.00001, 0, .00001, .005, .03} {
				for _, dx := range []float64{-.03, 0, .03} {
					a, b := fast.Sample(lon+dx, lat+delta), reference.Sample(lon+dx, lat+delta)
					if a != b {
						t.Fatalf("envelope changed source data at %.8f,%.8f", lon+dx, lat+delta)
					}
				}
			}
		}
	}
}
