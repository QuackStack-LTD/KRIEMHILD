package tiles

import (
	"context"
	"fmt"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/terrain/model"
)

type Mesh struct {
	Columns int        `json:"columns"`
	Rows    int        `json:"rows"`
	Bounds  [4]float64 `json:"bounds"` // west, south, east, north, radians
	Bed     []float64  `json:"bed"`
	Surface []float64  `json:"surface"`
	Wet     []bool     `json:"wet"`
	Colors  [][3]uint8 `json:"colors"`
}

func SampleMesh(ctx context.Context, p *model.Planet, bounds [4]float64, columns, rows int, transform Transform) (*Mesh, error) {
	for _, v := range bounds {
		if !model.Finite(v) {
			return nil, fmt.Errorf("invalid mesh bounds")
		}
	}
	if columns < 8 || columns > 256 || rows < 8 || rows > 256 || columns*rows > 65536 || bounds[0] >= bounds[2] || bounds[1] >= bounds[3] || bounds[2]-bounds[0] > model.Tau+.00001 || bounds[1] < -model.Tau/4 || bounds[3] > model.Tau/4 {
		return nil, fmt.Errorf("invalid mesh extent or resolution")
	}
	n := columns * rows
	m := &Mesh{Columns: columns, Rows: rows, Bounds: bounds, Bed: make([]float64, n), Surface: make([]float64, n), Wet: make([]bool, n), Colors: make([][3]uint8, n)}
	sampler := detail.New(p)
	for y := 0; y < rows; y++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		lat := bounds[3] - (bounds[3]-bounds[1])*float64(y)/float64(rows-1)
		for x := 0; x < columns; x++ {
			lon := bounds[0] + (bounds[2]-bounds[0])*float64(x)/float64(columns-1)
			s := sampler.Sample(lon, lat)
			if transform != nil {
				s = transform(s, lon, lat)
			}
			i := y*columns + x
			m.Bed[i], m.Surface[i], m.Wet[i] = s.Bed, s.WaterSurface, s.Water != "dry"
			c := palette(s, "natural", 1)
			m.Colors[i] = [3]uint8{c.R, c.G, c.B}
		}
	}
	return m, nil
}
