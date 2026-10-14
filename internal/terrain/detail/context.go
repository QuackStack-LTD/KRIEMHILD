package detail

import (
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
)

func conditioned(p *model.Planet, lon, lat, bed, coast float64) (float64, float64) {
	return landmass.Conditioned(p, lon, lat, bed, coast)
}
