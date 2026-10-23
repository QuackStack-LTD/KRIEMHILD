package tiles

import (
	"image/color"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/terrain/model"
	"math"
)

// Both views use the same material, world-space illumination and fixed physical
// normal stencil. Camera movement and display LOD cannot rotate the light.
func terrainColor(s model.Sample, east, north float64) color.NRGBA {
	c := palette(s, "natural", 1)
	if s.Water != "dry" {
		return c
	}
	light := (3*east + 5 + 4*north) / math.Sqrt(50*(1+east*east+north*north))
	factor := .55 + .75*math.Max(0, light)
	channel := func(v uint8) uint8 {
		x := float64(v) / 255
		if x <= .04045 {
			x /= 12.92
		} else {
			x = math.Pow((x+.055)/1.055, 2.4)
		}
		x *= factor
		if x <= .0031308 {
			x *= 12.92
		} else {
			x = 1.055*math.Pow(x, 1/2.4) - .055
		}
		return uint8(math.Round(model.Clamp(x*255, 0, 255)))
	}
	return color.NRGBA{channel(c.R), channel(c.G), channel(c.B), 255}
}

func gradients(sampler *detail.Sampler, p *model.Planet, lon, lat float64, transform Transform) (float64, float64) {
	dlat := 10 / (p.Spec.RadiusKM * 1000)
	dlon := dlat / math.Max(.01, math.Cos(lat))
	height := func(x, y float64) float64 {
		if transform == nil {
			return sampler.Bed(x, y)
		}
		s := sampler.Sample(x, y)
		if transform != nil {
			s = transform(s, x, y)
		}
		return s.Bed
	}
	return (height(lon+dlon, lat) - height(lon-dlon, lat)) / 20,
		(height(lon, math.Min(math.Pi/2, lat+dlat)) - height(lon, math.Max(-math.Pi/2, lat-dlat))) / 20
}
