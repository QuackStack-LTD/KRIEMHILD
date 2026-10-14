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
	p, err := generate.Generate(context.Background(), model.DefaultSpec(), nil)
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
	cancel()
	if _, err := tiles.SampleMesh(ctx, p, b, 16, 8, nil); err != context.Canceled {
		t.Fatal("mesh ignored cancellation")
	}
	if _, err := tiles.SampleMesh(context.Background(), p, b, 1024, 1024, nil); err == nil {
		t.Fatal("unbounded mesh accepted")
	}
}
