package detail

import (
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
	"math"
	"sync"
)

type segment struct {
	water                                          string
	a, b                                           model.Point
	bedA, bedB, surfaceA, surfaceB, widthA, widthB float64
	west, east, south, north, padding              float64
}

// Sampler's spatial index is derived, disposable and never changes any sample.
// Each request owns its index; there are no global mutable generation caches.
type Sampler struct {
	field  *landmass.Field
	planet *model.Planet
	bins   [128 * 64][]segment
}

var indexes struct {
	sync.Mutex
	entries []*Sampler
}

func New(p *model.Planet) *Sampler {
	if p.Surface == nil {
		return newSampler(p)
	}
	indexes.Lock()
	defer indexes.Unlock()
	for i, s := range indexes.entries {
		if s.planet == p {
			indexes.entries = append(append(indexes.entries[:i:i], indexes.entries[i+1:]...), s)
			return s
		}
	}
	s := newSampler(p)
	indexes.entries = append(indexes.entries, s)
	if len(indexes.entries) > 2 {
		indexes.entries = indexes.entries[1:]
	}
	return s
}
func newSampler(p *model.Planet) *Sampler {
	s := &Sampler{planet: p, field: landmass.NewField(p)}
	rivers := append([]model.River(nil), p.Rivers...)
	for _, lake := range p.Lakes {
		if len(lake.Path) < 2 {
			continue
		}
		r := model.River{ID: "oxbow", Path: lake.Path}
		for range lake.Path {
			r.Bed = append(r.Bed, lake.Level-2)
			r.Surface = append(r.Surface, lake.Level)
			r.Width = append(r.Width, lake.Width)
		}
		rivers = append(rivers, r)
	}
	for _, r := range rivers {
		water := "river"
		if r.ID == "oxbow" {
			water = "lake"
		}
		for i := 1; i < len(r.Path); i++ {
			a, b := r.Path[i-1], r.Path[i]
			b[0] = a[0] + model.Wrap(b[0]-a[0])
			seg := segment{water: water, a: a, b: b, bedA: r.Bed[i-1], bedB: r.Bed[i], surfaceA: r.Surface[i-1], surfaceB: r.Surface[i], widthA: r.Width[i-1], widthB: r.Width[i]}
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
	seg.west, seg.east = math.Min(a[0], b[0]), math.Max(a[0], b[0])
	seg.south, seg.north = math.Min(a[1], b[1]), math.Max(a[1], b[1])
	seg.padding = 3*math.Max(seg.widthA, seg.widthB)/math.Max(1, s.planet.Spec.RadiusKM*1000) + 1e-12
	bx := func(lon float64) int { return int((lon + math.Pi) / model.Tau * 128) }
	by := func(lat float64) int { return int((.5 - lat/math.Pi) * 64) }
	polar := math.Min(math.Pi/2, math.Max(math.Abs(a[1]), math.Abs(b[1]))+seg.padding)
	lonPad := seg.padding / math.Max(1e-12, math.Cos(polar))
	// Refine the old conservative bin cover, never broaden it: results for saved
	// worlds remain identical. Most reaches fit in one cell, not a 3x3 halo.
	for y := max(0, min(by(a[1]), by(b[1]))-1, by(seg.north+seg.padding)); y <= min(63, max(by(a[1]), by(b[1]))+1, by(seg.south-seg.padding)); y++ {
		for x := max(0, min(bx(a[0]), bx(b[0]))-1, bx(seg.west-lonPad)); x <= min(127, max(bx(a[0]), bx(b[0]))+1, bx(seg.east+lonPad)); x++ {
			s.bins[y*128+x] = append(s.bins[y*128+x], seg)
		}
	}
}
func (s *Sampler) Sample(lon, lat float64) model.Sample {
	lon = model.Wrap(lon)
	lat = model.Clamp(lat, -math.Pi/2, math.Pi/2)
	a := sampleWith(s.planet, lon, lat, s.field.Bed)
	return s.channels(a, lon, lat)
}

// Bed omits climate/biome work for relief normals. It shares the exact channel
// evaluation with Sample; it is not a separate approximation of the surface.
func (s *Sampler) Bed(lon, lat float64) float64 {
	if s.planet.Surface == nil {
		return s.Sample(lon, lat).Bed
	}
	lon = model.Wrap(lon)
	lat = model.Clamp(lat, -math.Pi/2, math.Pi/2)
	f := s.planet.Surface
	bed := f.At(f.Bed, lon, lat)
	if f.At(f.Coast, lon, lat) <= 0 {
		bed = math.Min(0, bed)
	}
	return s.channels(model.Sample{Bed: bed, Water: "dry"}, lon, lat).Bed
}

func (s *Sampler) channels(a model.Sample, lon, lat float64) model.Sample {
	x, y := int((lon+math.Pi)/model.Tau*128), int((.5-lat/math.Pi)*64)
	c := math.Cos(lat)
	scale := s.planet.Spec.RadiusKM * 1000
	for _, seg := range s.bins[min(63, max(0, y))*128+min(127, max(0, x))] {
		// A conservative geographic envelope rejects distant reaches before
		// projection and hypot. Profiling a saved world put 93% of raster CPU in
		// this scan. The exact channel calculation below is unchanged.
		if lat < seg.south-seg.padding || lat > seg.north+seg.padding {
			continue
		}
		if c > 1e-12 {
			pad := seg.padding / c
			if lon < seg.west-pad || lon > seg.east+pad {
				continue
			}
		}
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
			a.Water = seg.water
			a.WaterSurface = level
			a.WaterDepth = level - a.Bed
			a.Moisture = 1
		}
	}
	if a.Water != "dry" {
		a.WaterDepth = math.Max(0, a.WaterSurface-a.Bed)
		a.Biome = "water"
	}
	return a
}
