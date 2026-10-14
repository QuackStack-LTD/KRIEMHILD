package generate_test

import (
	"context"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/terrain/generate"
	"kriemhild/internal/terrain/model"
	"math"
	"reflect"
	"testing"
)

func TestDeterminismPersistenceAndSampling(t *testing.T) {
	spec := model.DefaultSpec()
	spec.SurfaceWidth = 1024 // Functional checks use the smallest complete surface.
	p, err := generate.Generate(context.Background(), spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	spec.TileSize = 512
	q, err := generate.Generate(context.Background(), spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Masses, q.Masses) || !reflect.DeepEqual(p.Grid, q.Grid) || !reflect.DeepEqual(p.Rivers, q.Rivers) {
		t.Fatal("render quality changes geography")
	}
	meta, fields, err := p.Encode()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := model.Decode(meta, fields)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		lon := float64(i)*.137 - 13
		lat := float64(i%90-45) * math.Pi / 90
		a := detail.Sample(p, lon, lat)
		b := detail.Sample(restored, lon, lat)
		if !reflect.DeepEqual(a, b) {
			t.Fatal("archive changes sampled terrain")
		}
		if a.WaterDepth < 0 || a.Water != "dry" && math.Abs(a.WaterSurface-a.Bed-a.WaterDepth) > 1e-8 {
			t.Fatal("water invariant")
		}
		x, y := detail.Sample(p, -math.Pi, lat), detail.Sample(p, math.Pi, lat)
		if x != y {
			t.Fatal("longitude seam")
		}
	}
	for _, river := range p.Rivers {
		for i := 1; i < len(river.Path); i++ {
			if river.Surface[i] > river.Surface[i-1] || river.Discharge[i] < river.Discharge[i-1] || river.Bed[i] > river.Surface[i] {
				t.Fatal("river invariant")
			}
		}
	}
	t.Logf("coverage %.3f, major continents %d, rivers %d, timings %v", p.Diagnostics.LandCoverage, p.Diagnostics.MajorContinents, len(p.Rivers), p.Diagnostics.Timings)
}
func TestCancelledGeneration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := generate.Generate(ctx, model.DefaultSpec(), nil); err != context.Canceled {
		t.Fatal(err)
	}
}
func TestMalformedArchive(t *testing.T) {
	spec := model.DefaultSpec()
	spec.SurfaceWidth = 1024
	p, err := generate.Generate(context.Background(), spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	meta, fields, _ := p.Encode()
	if _, err = model.Decode(meta, fields[:100]); err == nil {
		t.Fatal("accepted truncated fields")
	}
}
