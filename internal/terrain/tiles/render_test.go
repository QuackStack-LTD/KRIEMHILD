package tiles_test

import (
	"bytes"
	"context"
	"image/png"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/terrain/generate"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/terrain/tiles"
	"math"
	"sync"
	"testing"
)

func TestTilesOrderHalosAndParentCoordinates(t *testing.T) {
	spec := model.DefaultSpec()
	spec.SurfaceWidth = 1024
	p, err := generate.Generate(context.Background(), spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The final right halo of one tile is the first interior pixel of its neighbor.
	lon, lat := tiles.Coordinate(2, 3, 1, 256, 256, 17)
	a, b := tiles.Coordinate(2, 4, 1, 256, 0, 17)
	if lon != a || lat != b {
		t.Fatal("tile halo coordinate mismatch")
	}
	s := detail.New(p)
	if s.Sample(lon, lat) != s.Sample(a, b) {
		t.Fatal("seam samples differ")
	}
	// Parent anchors are geographic coordinates, independent of pixel centres.
	for j := 0; j < 9; j++ {
		x := float64(j)/8*math.Pi - 1
		coarse := s.Sample(x, .2)
		fine := detail.New(p).Sample(x, .2)
		if coarse != fine {
			t.Fatal("parent anchor changed")
		}
	}
	want, err := tiles.Render(context.Background(), p, 0, 0, 0, 256, "natural", nil)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			other, err := tiles.Render(context.Background(), p, 1, i%4, 0, 256, "mask", nil)
			if err != nil || len(other) == 0 {
				t.Error(err)
			}
			got, err := tiles.Render(context.Background(), p, 0, 0, 0, 256, "natural", nil)
			if err != nil || !bytes.Equal(want, got) {
				t.Error("tile changed with request order")
			}
		}(i)
	}
	wg.Wait()
	image, err := png.Decode(bytes.NewReader(want))
	if err != nil || image.Bounds().Dx() != 256 || image.Bounds().Dy() != 256 {
		t.Fatal("invalid tile resolution", err)
	}
	// Authored terrain replaces the original water boundary in illustrator ink.
	// A synthetic half-tile shore must leave no ink from the saved base coast.
	ink, err := tiles.Render(context.Background(), p, 0, 0, 0, 256, "shore-ink", func(s model.Sample, lon, lat float64) model.Sample {
		s.Water = "dry"
		if lon < -math.Pi/2 {
			s.Water = "lake"
		}
		return s
	})
	if err != nil {
		t.Fatal(err)
	}
	shore, err := png.Decode(bytes.NewReader(ink))
	if err != nil {
		t.Fatal(err)
	}
	for y := 1; y < 255; y++ {
		for x := 1; x < 255; x++ {
			_, _, _, alpha := shore.At(x, y).RGBA()
			if (alpha != 0) != (x == 127 || x == 128) {
				t.Fatalf("edited shoreline disagrees with water boundary at %d,%d", x, y)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = tiles.Render(ctx, p, 0, 0, 0, 256, "natural", nil); err != context.Canceled {
		t.Fatal("tile ignored cancellation", err)
	}
}
