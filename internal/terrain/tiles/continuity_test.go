package tiles

import (
	"bytes"
	"context"
	"image/png"
	"kriemhild/internal/terrain/model"
	"math"
	"testing"
)

func patternedPlanet() *model.Planet {
	const w, h = 64, 32
	p := &model.Planet{Spec: model.DefaultSpec(), Grid: model.Grid{Width: w, Height: h, Fields: map[string][]float64{}}, Surface: &model.Surface{Width: w, Height: h}}
	p.Surface.Bed = make([]float32, w*h)
	p.Surface.Coast = make([]float32, w*h)
	p.Surface.Roughness = make([]float32, w*h)
	p.Surface.LakeLevel = make([]float32, w*h)
	for _, key := range []string{"bed", "temperature", "precipitation", "pet", "coast", "soil"} {
		p.Grid.Fields[key] = make([]float64, w*h)
	}
	for i := range p.Surface.Bed {
		pt := p.Grid.Point(i%w, i/w)
		bed := 400 + 1000*math.Exp(-8*(pt[0]*pt[0]+pt[1]*pt[1]))
		coast := 100 * (math.Sin(pt[0]) + pt[1])
		p.Surface.Bed[i] = float32(bed)
		p.Surface.Coast[i] = float32(coast)
		for k, v := range map[string]float64{"bed": bed, "temperature": 14 + 4*math.Sin(pt[1]), "precipitation": 900 + 300*math.Sin(pt[0]), "pet": 900, "coast": coast, "soil": 50} {
			p.Grid.Fields[k][i] = v
		}
	}
	p.Rivers = []model.River{{Path: []model.Point{{-1, .5}, {1, -.5}}, Bed: []float64{50, 40}, Surface: []float64{60, 50}, Width: []float64{20000, 20000}}}
	return p
}

func TestIndependentHalosAgreeIncludingLongitudeSeam(t *testing.T) {
	p := patternedPlanet()
	ctx := context.Background()
	const size = 256
	const stride = size + 2
	for _, pair := range [][6]int{{2, 3, 1, 2, 4, 1}, {2, 3, 1, 2, 3, 2}, {2, 7, 1, 2, 0, 1}} {
		a, err := physicalSamples(ctx, p, pair[0], pair[1], pair[2], size)
		if err != nil {
			t.Fatal(err)
		}
		b, err := physicalSamples(ctx, p, pair[3], pair[4], pair[5], size)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < size; i++ {
			ia, ib := (i+1)*stride+size+1, (i+1)*stride+1
			if pair[2] != pair[5] {
				ia, ib = (size+1)*stride+i+1, stride+i+1
			}
			// Samples include independent elevation, biome, water and climate fields.
			x, y := a[ia], b[ib]
			if math.Abs(x.Bed-y.Bed) > 1e-8 || math.Abs(x.Temperature-y.Temperature) > 1e-8 || math.Abs(x.Precipitation-y.Precipitation) > 1e-8 || x.Water != y.Water || x.Biome != y.Biome {
				t.Fatalf("shared sample disagrees: %v at %d: %+v %+v", pair, i, x, y)
			}
		}
	}
}

func TestParentIsExactlyFourChildrenAtEqualSampleDensity(t *testing.T) {
	p := patternedPlanet()
	for _, style := range []string{"natural", "biome", "mask", "elevation", "hillshade", "shore-ink"} {
		parent, err := Render(context.Background(), p, 1, 1, 0, 512, style, nil)
		if err != nil {
			t.Fatal(err)
		}
		large, err := png.Decode(bytes.NewReader(parent))
		if err != nil {
			t.Fatal(err)
		}
		for y := 0; y < 2; y++ {
			for x := 0; x < 2; x++ {
				b, err := Render(context.Background(), p, 2, 2+x, y, 256, style, nil)
				if err != nil {
					t.Fatal(err)
				}
				small, err := png.Decode(bytes.NewReader(b))
				if err != nil {
					t.Fatal(err)
				}
				for j := 0; j < 256; j++ {
					for i := 0; i < 256; i++ {
						if small.At(i, j) != large.At(x*256+i, y*256+j) {
							t.Fatalf("%s parent/child mismatch at %d,%d child %d,%d", style, i, j, x, y)
						}
					}
				}
			}
		}
	}
}
