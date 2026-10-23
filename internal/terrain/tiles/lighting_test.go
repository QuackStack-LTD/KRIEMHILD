package tiles

import (
	"bytes"
	"context"
	"image/png"
	"kriemhild/internal/terrain/model"
	"math"
	"testing"
)

func TestOverheadRasterAnd3DHaveSameMaterialAndLighting(t *testing.T) {
	s := model.DefaultSpec()
	p := &model.Planet{Spec: s, Grid: model.Grid{Width: 4, Height: 2, Fields: map[string][]float64{}}, Surface: &model.Surface{Width: 16, Height: 8}}
	for _, key := range []string{"temperature", "precipitation", "pet"} {
		p.Grid.Fields[key] = make([]float64, 8)
		for i := range p.Grid.Fields[key] {
			p.Grid.Fields[key][i] = map[string]float64{"temperature": 18, "precipitation": 850, "pet": 1100}[key]
		}
	}
	for i := 0; i < 128; i++ {
		p.Surface.Bed = append(p.Surface.Bed, float32(300+i%16*60))
		p.Surface.Coast = append(p.Surface.Coast, 100)
		p.Surface.Roughness = append(p.Surface.Roughness, .3)
		p.Surface.LakeLevel = append(p.Surface.LakeLevel, 0)
	}
	data, err := Render(context.Background(), p, 3, 8, 3, 256, "terrain", nil)
	if err != nil {
		t.Fatal(err)
	}
	raster, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	w, n := Coordinate(3, 8, 3, 256, 0, 0)
	e, south := Coordinate(3, 8, 3, 256, 255, 255)
	m, err := SampleMesh(context.Background(), p, [4]float64{w, south, e, n}, 256, 256, nil)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 256; y += 7 {
		for x := 0; x < 256; x += 7 {
			r, g, b, _ := raster.At(x, y).RGBA()
			c := m.LitColors[y*256+x]
			for k, v := range []uint32{r >> 8, g >> 8, b >> 8} {
				if math.Abs(float64(v)-float64(c[k])) > 1 {
					t.Fatalf("2D/3D material mismatch at %d,%d: %v versus %d,%d,%d", x, y, c, r>>8, g>>8, b>>8)
				}
			}
		}
	}
}
