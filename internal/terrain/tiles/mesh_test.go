package tiles_test

import (
	"context"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/terrain/generate"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/terrain/tiles"
	"math"
	"testing"
)

func TestMeshUsesSameBedAndWaterSampler(t *testing.T) {
	spec := model.DefaultSpec()
	spec.SurfaceWidth = 1024
	p, err := generate.Generate(context.Background(), spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	b := [4]float64{-math.Pi, -math.Pi / 2, math.Pi, math.Pi / 2}
	m, err := tiles.SampleMesh(context.Background(), p, b, 16, 8, nil)
	if err != nil {
		t.Fatal(err)
	}
	sampler := detail.New(p)
	for y := 0; y < m.Rows; y++ {
		for x := 0; x < m.Columns; x++ {
			i := y*m.Columns + x
			lon := b[0] + (b[2]-b[0])*float64(x)/float64(m.Columns-1)
			lat := b[3] - (b[3]-b[1])*float64(y)/float64(m.Rows-1)
			s := sampler.Sample(lon, lat)
			if m.Bed[i] != s.Bed || m.Surface[i] != s.WaterSurface || m.Wet[i] != (s.Water != "dry") {
				t.Fatal("3D source differs from map/inspection")
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	left, err := tiles.SampleMesh(context.Background(), p, [4]float64{0, 0, .2, .2}, 17, 17, nil)
	if err != nil {
		t.Fatal(err)
	}
	right, err := tiles.SampleMesh(context.Background(), p, [4]float64{.2, 0, .4, .2}, 17, 17, nil)
	if err != nil {
		t.Fatal(err)
	}
	child, err := tiles.SampleMesh(context.Background(), p, [4]float64{0, .1, .1, .2}, 17, 17, nil)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 17; y++ {
		if left.Bed[y*17+16] != right.Bed[y*17] || left.Surface[y*17+16] != right.Surface[y*17] {
			t.Fatal("adjacent mesh edge differs")
		}
		if left.Gradients[y*17+16] != right.Gradients[y*17] {
			t.Fatal("adjacent mesh normal source differs")
		}
	}
	for y := 0; y < 9; y++ {
		for x := 0; x < 9; x++ {
			if math.Abs(left.Bed[y*17+x]-child.Bed[y*2*17+x*2]) > 1e-8 {
				t.Fatal("parent/child shared anchor differs")
			}
			for axis := 0; axis < 2; axis++ {
				if math.Abs(left.Gradients[y*17+x][axis]-child.Gradients[y*2*17+x*2][axis]) > 1e-8 {
					t.Fatal("parent/child normal source differs")
				}
			}
		}
	}
	cancel()
	if _, err := tiles.SampleMesh(ctx, p, b, 16, 8, nil); err != context.Canceled {
		t.Fatal("mesh ignored cancellation")
	}
	if _, err := tiles.SampleMesh(context.Background(), p, b, 1024, 1024, nil); err == nil {
		t.Fatal("unbounded mesh accepted")
	}
}
