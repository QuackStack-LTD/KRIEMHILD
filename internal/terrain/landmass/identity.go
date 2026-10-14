package landmass

import (
	"kriemhild/internal/terrain/model"
	"math"
)

// Resolve the actual contributing stroke, not the nearest continent centre.
func Identity(p *model.Planet, lon, lat float64) *model.Mass {
	if Potential(p, lon, lat) <= 0 {
		return nil
	}
	sl, cl := math.Sincos(lat)
	wx := Noise(p.Seed+1, lon, lat, 5) * .045 * p.Spec.Complexity
	wy := Noise(p.Seed+2, lon, lat, 7) * .035 * p.Spec.Complexity
	best := -10.
	var found *model.Mass
	for i := range p.Masses {
		m := &p.Masses[i]
		dl := model.Wrap(lon - m.Center[0])
		s0, c0 := math.Sincos(m.Center[1])
		sd, cd := math.Sincos(dl)
		dot := model.Clamp(s0*sl+c0*cl*cd, -1, 1)
		if dot < math.Cos(m.Extent+.15) {
			continue
		}
		angle := math.Acos(dot)
		k := 1.
		if angle > 1e-8 {
			k = angle / math.Sin(angle)
		}
		xx, yy := k*cl*sd, k*(c0*sl-s0*cl*cd)
		sn, cs := math.Sincos(m.Rotation)
		x, y := xx*cs+yy*sn+wx, -xx*sn+yy*cs+wy
		for _, stroke := range m.Strokes {
			v := strokeDistance(stroke, x, y)
			if v > best {
				best = v
				found = m
			}
		}
	}
	return found
}
