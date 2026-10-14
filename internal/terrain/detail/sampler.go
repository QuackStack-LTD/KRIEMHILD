package detail

import (
	"kriemhild/internal/terrain/generate"
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
	"math"
)

func Sample(p *model.Planet, lon, lat float64) model.Sample {
	if p.Surface != nil {
		return New(p).Sample(lon, lat)
	}
	return sampleWith(p, lon, lat, func(lon, lat float64) (float64, float64, float64) { return landmass.Bed(p, lon, lat) })
}
func sampleWith(p *model.Planet, lon, lat float64, surface func(float64, float64) (float64, float64, float64)) model.Sample {
	lon = model.Wrap(lon)
	lat = model.Clamp(lat, -math.Pi/2, math.Pi/2)
	var bed, coast, rough float64
	if f := p.Surface; f != nil {
		bed, coast, rough = f.At(f.Bed, lon, lat), f.At(f.Coast, lon, lat)/p.Spec.RadiusKM, f.At(f.Roughness, lon, lat)
		if coast <= 0 {
			bed = math.Min(0, bed)
		}
	} else {
		bed, coast, rough = surface(lon, lat)
		bed, coast = conditioned(p, lon, lat, bed, coast)
	}
	g := p.Grid
	temp := g.At("temperature", lon, lat) - (bed-g.At("bed", lon, lat))*.0065
	rain, pet := g.At("precipitation", lon, lat), g.At("pet", lon, lat)
	s := model.Sample{Bed: bed, CoastDistance: coast * p.Spec.RadiusKM, Temperature: temp, Precipitation: rain, PET: pet, Moisture: model.Clamp(rain/math.Max(100, pet)*.6, 0, 1), Roughness: rough, Water: "dry"}
	if coast <= 0 {
		s.Water = "ocean"
		s.WaterSurface = 0
		s.WaterDepth = -bed
	} else if p.Surface != nil {
		level := p.Surface.At(p.Surface.LakeLevel, lon, lat)
		if level > 0 && bed < level {
			s.Water = "lake"
			s.WaterSurface = level
			s.WaterDepth = level - bed
		}
	} else {
		for _, lake := range p.Lakes {
			if math.Abs(lat-lake.Center[1]) > lake.Radius*1.2 {
				continue
			}
			d := math.Hypot(model.Wrap(lon-lake.Center[0])*math.Cos(lat), lat-lake.Center[1])
			boundary := lake.Radius
			if p.Surface == nil {
				boundary *= 1 + .18*landmass.Noise(p.Seed+811, lon, lat, 210)
			}
			if d < boundary && bed < lake.Level {
				s.Water = "lake"
				s.WaterSurface = lake.Level
				s.WaterDepth = lake.Level - bed
				break
			}
		}
	}
	s.Biome = generate.BiomeAt(p, lon, lat, bed, s.Water != "dry")
	return s
}
