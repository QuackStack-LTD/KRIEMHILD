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
	width := .035
	if p.Spec.Version == model.Version && p.Spec.ReliefVersion >= 1 {
		// Deform spherical plate boundaries continuously before measuring their
		// distance. This bends entire orogens, rather than adding pixel roughness
		// to perfectly straight Voronoi boundaries. No latitude/longitude seam.
		u := v
		for k := range v {
			phase := float64((p.Seed>>(k*13))&8191) / 8191 * model.Tau
			v[k] += .075*math.Sin(4*u[(k+1)%3]+phase)*math.Sin(3*u[(k+2)%3]-phase) +
				.022*math.Sin(11*u[(k+1)%3]+7*u[(k+2)%3]+phase)
		}
		length := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
		for k := range v {
			v[k] /= length
		}
		width = .065 + .02*math.Sin(5*v[0]+4*v[1]-3*v[2])
	}
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
	boundary := math.Exp(-(d2 - d1) / width)
	return Influence{Plate: first, Boundary: boundary, Convergence: model.Clamp(-relative, 0, 1) * boundary, Divergence: model.Clamp(relative, 0, 1) * boundary, Continental: a.Continental}
}
