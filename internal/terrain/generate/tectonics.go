package generate

import (
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
	"math"
	"strings"
)

func plates(p *model.Planet) {
	r := stream(p.Spec, "tectonics")
	for i := 0; i < 14; i++ {
		lon, lat := r.Float64()*model.Tau-math.Pi, math.Asin(r.Float64()*2-1)
		c := math.Cos(lat)
		center := [3]float64{c * math.Cos(lon), c * math.Sin(lon), math.Sin(lat)}
		axis := [3]float64{r.Float64()*2 - 1, r.Float64()*2 - 1, r.Float64()*2 - 1}
		motion := [3]float64{axis[1]*center[2] - axis[2]*center[1], axis[2]*center[0] - axis[0]*center[2], axis[0]*center[1] - axis[1]*center[0]}
		p.Plates = append(p.Plates, model.Plate{Center: center, Motion: motion, Continental: landmass.Potential(p, lon, lat) > 0})
	}
	if p.Spec.ReliefVersion >= 2 {
		// Continental collision pairs place active orogens within assembled land,
		// rather than hoping random oceanic plates intersect every continent.
		// Their boundaries share the spherical, smoothly warped plate field.
		for _, m := range p.Masses {
			if !strings.HasPrefix(m.ID, "continent-") {
				continue
			}
			lon, lat := m.Center[0], m.Center[1]
			c := [3]float64{math.Cos(lat) * math.Cos(lon), math.Cos(lat) * math.Sin(lon), math.Sin(lat)}
			east := [3]float64{-math.Sin(lon), math.Cos(lon), 0}
			north := [3]float64{-math.Sin(lat) * math.Cos(lon), -math.Sin(lat) * math.Sin(lon), math.Cos(lat)}
			angle := m.Rotation + .35
			distance := math.Min(.28, math.Max(.09, m.Extent*.32))
			for _, sign := range []float64{-1, 1} {
				var center, motion [3]float64
				speed := .45 + .15*r.Float64()
				for k := range center {
					direction := east[k]*math.Cos(angle) + north[k]*math.Sin(angle)
					center[k] = c[k]*math.Cos(distance) + sign*direction*math.Sin(distance)
					motion[k] = -sign * direction * speed
				}
				p.Plates = append(p.Plates, model.Plate{Center: center, Motion: motion, Continental: true})
			}
		}
	}
}
