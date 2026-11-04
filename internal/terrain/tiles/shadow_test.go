package tiles

import (
	"bytes"
	"context"
	"image/png"
	"kriemhild/internal/terrain/model"
	"math"
	"testing"
)

func TestReliefShadowEqualsMultiplyAndInkTracksWaterAtEveryZoom(t *testing.T) {
	p := &model.Planet{Spec: model.DefaultSpec(), Grid: model.Grid{Width: 4, Height: 2, Fields: map[string][]float64{}}, Surface: &model.Surface{Width: 16, Height: 8}}
	for i := 0; i < 128; i++ {
		p.Surface.Bed = append(p.Surface.Bed, float32(i%16*150))
		p.Surface.Coast = append(p.Surface.Coast, 100)
		p.Surface.Roughness = append(p.Surface.Roughness, .2)
		p.Surface.LakeLevel = append(p.Surface.LakeLevel, 0)
	}
	for _, z := range []int{0, 4, 12, 18} {
		x, y := 1<<z, 0
		grayscale, err := Render(context.Background(), p, z, x, y, 256, "hillshade", nil)
		if err != nil {
			t.Fatal(err)
		}
		shadow, err := Render(context.Background(), p, z, x, y, 256, "relief-shadow", nil)
		if err != nil {
			t.Fatal(err)
		}
		a, err := png.Decode(bytes.NewReader(grayscale))
		if err != nil {
			t.Fatal(err)
		}
		b, err := png.Decode(bytes.NewReader(shadow))
		if err != nil {
			t.Fatal(err)
		}
		for j := 0; j < 256; j += 13 {
			for i := 0; i < 256; i += 11 {
				r, _, _, _ := a.At(i, j).RGBA()
				br, bg, bb, alpha := b.At(i, j).RGBA()
				if br != 0 || bg != 0 || bb != 0 || alpha != 65535-r {
					t.Fatalf("shadow differs from multiply at level %d pixel %d,%d", z, i, j)
				}
			}
		}
		lon, _ := Coordinate(z, x, y, 256, 127, 0)
		lon2, _ := Coordinate(z, x, y, 256, 128, 0)
		cut := (lon + lon2) / 2
		ink, err := Render(context.Background(), p, z, x, y, 256, "shore-ink", func(s model.Sample, lon, lat float64) model.Sample {
			s.Water = "dry"
			if lon < cut {
				s.Water = "ocean"
			}
			return s
		})
		if err != nil {
			t.Fatal(err)
		}
		image, err := png.Decode(bytes.NewReader(ink))
		if err != nil {
			t.Fatal(err)
		}
		for j := 0; j < 256; j += 17 {
			for i := 1; i < 255; i++ {
				_, _, _, alpha := image.At(i, j).RGBA()
				if (alpha > 0) != (i == 127 || i == 128) {
					t.Fatalf("ink moved off water boundary at level %d pixel %d,%d (cut %.12f)", z, i, j, cut*180/math.Pi)
				}
			}
		}
	}
}
