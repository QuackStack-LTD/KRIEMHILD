// Package tectonics evaluates a reduced-order moving plate framework on a sphere.
package tectonics

import (
	"kriemhild/internal/terrain/model"
	"math"
)

type Influence struct {
	Plate                             int
	Boundary, Convergence, Divergence float64
	Continental                       bool
}

func At(p *model.Planet, lon, lat float64) Influence {
	if len(p.Plates) < 2 {
		return Influence{}
	}
	cl := math.Cos(lat)
	v := [3]float64{cl * math.Cos(lon), cl * math.Sin(lon), math.Sin(lat)}
	first, second := 0, 0
	d1, d2 := 10., 10.
	for i, plate := range p.Plates {
		d := 0.
		for k := 0; k < 3; k++ {
			x := v[k] - plate.Center[k]
			d += x * x
		}
		if d < d1 {
			second, d2 = first, d1
			first, d1 = i, d
		} else if d < d2 {
			second, d2 = i, d
		}
	}
	a, b := p.Plates[first], p.Plates[second]
	relative, norm := 0., 0.
	for k := 0; k < 3; k++ {
		direction := b.Center[k] - a.Center[k]
		relative += (b.Motion[k] - a.Motion[k]) * direction
		norm += direction * direction
	}
	relative /= math.Sqrt(math.Max(norm, 1e-9))
	boundary := math.Exp(-(d2 - d1) / .035)
	return Influence{Plate: first, Boundary: boundary, Convergence: model.Clamp(-relative, 0, 1) * boundary, Divergence: model.Clamp(relative, 0, 1) * boundary, Continental: a.Continental}
}
