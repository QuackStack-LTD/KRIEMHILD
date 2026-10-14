package generate

import (
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
	"math"
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
}
