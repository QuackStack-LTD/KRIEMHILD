package detail

import (
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
	"math"
)

type segment struct {
	a, b                                           model.Point
	bedA, bedB, surfaceA, surfaceB, widthA, widthB float64
}

// Sampler's spatial index is derived, disposable and never changes any sample.
// Each request owns its index; there are no global mutable generation caches.
type Sampler struct {
	field  *landmass.Field
	planet *model.Planet
	bins   [128 * 64][]segment
}

func New(p *model.Planet) *Sampler {
	s := &Sampler{planet: p, field: landmass.NewField(p)}
	for _, r := range p.Rivers {
		for i := 1; i < len(r.Path); i++ {
			a, b := r.Path[i-1], r.Path[i]
			b[0] = a[0] + model.Wrap(b[0]-a[0])
			seg := segment{a, b, r.Bed[i-1], r.Bed[i], r.Surface[i-1], r.Surface[i], r.Width[i-1], r.Width[i]}
			for _, offset := range []float64{-model.Tau, 0, model.Tau} {
				copy := seg
				copy.a[0] += offset
				copy.b[0] += offset
				s.index(copy)
			}
		}
	}
	return s
}
func (s *Sampler) index(seg segment) {
	a, b := seg.a, seg.b
	bx := func(lon float64) int { return int((lon + math.Pi) / model.Tau * 128) }
	by := func(lat float64) int { return int((.5 - lat/math.Pi) * 64) }
	for y := max(0, min(by(a[1]), by(b[1]))-1); y <= min(63, max(by(a[1]), by(b[1]))+1); y++ {
		for x := max(0, min(bx(a[0]), bx(b[0]))-1); x <= min(127, max(bx(a[0]), bx(b[0]))+1); x++ {
			s.bins[y*128+x] = append(s.bins[y*128+x], seg)
		}
	}
}
func (s *Sampler) Sample(lon, lat float64) model.Sample {
	lon = model.Wrap(lon)
	lat = model.Clamp(lat, -math.Pi/2, math.Pi/2)
	a := sampleWith(s.planet, lon, lat, s.field.Bed)
	x, y := int((lon+math.Pi)/model.Tau*128), int((.5-lat/math.Pi)*64)
	c := math.Cos(lat)
	scale := s.planet.Spec.RadiusKM * 1000
	for _, seg := range s.bins[min(63, max(0, y))*128+min(127, max(0, x))] {
		dx, dy := (seg.b[0]-seg.a[0])*c, seg.b[1]-seg.a[1]
		px, py := (lon-seg.a[0])*c, lat-seg.a[1]
		t := model.Clamp((px*dx+py*dy)/math.Max(1e-20, dx*dx+dy*dy), 0, 1)
		distance := math.Hypot(px-dx*t, py-dy*t) * scale
		width := seg.widthA + (seg.widthB-seg.widthA)*t
		if distance > width*3 {
			continue
		}
		bed := seg.bedA + (seg.bedB-seg.bedA)*t
		level := seg.surfaceA + (seg.surfaceB-seg.surfaceA)*t
		f := model.Clamp(1-distance/(width*3), 0, 1)
		f = f * f * (3 - 2*f)
		a.Bed += (math.Min(a.Bed, bed) - a.Bed) * f
		if distance < width*.5 && a.Water == "dry" && level > a.Bed {
			a.Water = "river"
			a.WaterSurface = level
			a.WaterDepth = level - a.Bed
			a.Moisture = 1
		}
	}
	if a.Water != "dry" {
		a.WaterDepth = math.Max(0, a.WaterSurface-a.Bed)
	}
	return a
}
