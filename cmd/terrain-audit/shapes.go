package main

import (
	"kriemhild/internal/terrain/model"
	"math"
)

type shapeMetric struct {
	Cells                 int     `json:"cells"`
	Area                  float64 `json:"areaKm2"`
	Perimeter             float64 `json:"coastKm"`
	Compactness           float64 `json:"compactness"`
	Orientation           float64 `json:"orientationDegrees"`
	Aspect                float64 `json:"aspectRatio"`
	sx, sy, sxx, syy, sxy float64
	origin                int
}

func shapeMetrics(p *model.Planet, labels []int) map[int]*shapeMetric {
	out := map[int]*shapeMetric{}
	g := p.Grid
	dx, dy := model.Tau/float64(g.Width), math.Pi/float64(g.Height)
	for i, id := range labels {
		if id == 0 {
			continue
		}
		s := out[id]
		if s == nil {
			s = &shapeMetric{origin: i % g.Width}
			out[id] = s
		}
		x, y := i%g.Width, i/g.Width
		lat := g.Point(x, y)[1]
		area := dx * dy * math.Cos(lat) * p.Spec.RadiusKM * p.Spec.RadiusKM
		s.Area += area
		s.Cells++
		xx := float64(x - s.origin)
		if xx > float64(g.Width)/2 {
			xx -= float64(g.Width)
		}
		if xx < -float64(g.Width)/2 {
			xx += float64(g.Width)
		}
		xx *= math.Cos(lat)
		yy := float64(y)
		s.sx += xx
		s.sy += yy
		s.sxx += xx * xx
		s.syy += yy * yy
		s.sxy += xx * yy
		for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			ny := y + d[1]
			if ny < 0 || ny >= g.Height {
				continue
			}
			nx := (x + d[0] + g.Width) % g.Width
			if labels[ny*g.Width+nx] != id {
				edge := dy * p.Spec.RadiusKM
				if d[1] != 0 {
					edge = dx * math.Cos(lat) * p.Spec.RadiusKM
				}
				s.Perimeter += edge
			}
		}
	}
	for _, s := range out {
		n := float64(s.Cells)
		a, b, c := s.sxx/n-s.sx*s.sx/(n*n), s.sxy/n-s.sx*s.sy/(n*n), s.syy/n-s.sy*s.sy/(n*n)
		s.Orientation = .5 * math.Atan2(2*b, a-c) * 180 / math.Pi
		disc := math.Sqrt((a-c)*(a-c) + 4*b*b)
		s.Aspect = math.Sqrt(math.Max(.001, (a+c+disc)) / math.Max(.001, a+c-disc))
		s.Compactness = 4 * math.Pi * s.Area / math.Max(1, s.Perimeter*s.Perimeter)
	}
	return out
}
