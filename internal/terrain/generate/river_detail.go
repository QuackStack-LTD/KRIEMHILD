package generate

import (
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
	"math"
)

// Subdivide saved reaches inside their valley corridor. Endpoints/confluences and
// monotonic water profiles are exact; the same geometry feeds rendering and carving.
func refineRiver(p *model.Planet, r model.River) model.River {
	out := r
	out.Path = nil
	out.Bed = nil
	out.Surface = nil
	out.Discharge = nil
	out.Width = nil
	for i := 1; i < len(r.Path); i++ {
		a, b := r.Path[i-1], r.Path[i]
		dx, dy := model.Wrap(b[0]-a[0]), b[1]-a[1]
		phase := float64(landmass.Hash(p.Seed+uint64(i)+uint64(len(r.ID)))%1000) / 1000 * model.Tau
		for j := 0; j < 8; j++ {
			t := float64(j) / 8
			wave := math.Sin(t*math.Pi) * math.Sin(t*model.Tau+phase) * .09
			lon, lat := a[0]+dx*t-dy*wave, a[1]+dy*t+dx*wave
			if p.Surface != nil {
				// Shared Catmull-Rom tangents remove the old right-angle corners.
				// Meanders grow only on low-gradient reaches, inside this corridor.
				before, after := r.Path[max(0, i-2)], r.Path[min(len(r.Path)-1, i+1)]
				ax, ay := model.Wrap(b[0]-before[0])*.5, (b[1]-before[1])*.5
				bx, by := model.Wrap(after[0]-a[0])*.5, (after[1]-a[1])*.5
				t2, t3 := t*t, t*t*t
				lon = a[0] + (-2*t3+3*t2)*dx + (t3-2*t2+t)*ax + (t3-t2)*bx
				lat = a[1] + (-2*t3+3*t2)*dy + (t3-2*t2+t)*ay + (t3-t2)*by
				slope := (r.Surface[i-1] - r.Surface[i]) / math.Max(1, distanceKM(a, b, p.Spec.RadiusKM)*1000)
				bend := math.Sin(t*math.Pi) * math.Sin((float64(i-1)+t)*1.7) * .12 * model.Clamp(1-slope/.002, 0, 1)
				lon -= dy * bend
				lat += dx * bend
				// Do not let curve overshoot turn an inland reach into a sea crossing
				// or cut across a ridge higher than its original drainage corridor.
				linearLon, linearLat := a[0]+dx*t, a[1]+dy*t
				candidate := p.Surface.At(p.Surface.Bed, lon, lat)
				linear := p.Surface.At(p.Surface.Bed, linearLon, linearLat)
				if candidate > linear+10 || p.Surface.At(p.Surface.Coast, lon, lat) < 0 && p.Surface.At(p.Surface.Coast, linearLon, linearLat) > 0 {
					lon, lat = linearLon, linearLat
				}
			}
			out.Path = append(out.Path, model.Point{model.Wrap(lon), model.Clamp(lat, -math.Pi/2, math.Pi/2)})
			interp := func(v []float64) float64 { return v[i-1] + (v[i]-v[i-1])*t }
			out.Bed = append(out.Bed, interp(r.Bed))
			out.Surface = append(out.Surface, interp(r.Surface))
			out.Discharge = append(out.Discharge, interp(r.Discharge))
			out.Width = append(out.Width, interp(r.Width))
		}
	}
	last := len(r.Path) - 1
	out.Path = append(out.Path, r.Path[last])
	out.Bed = append(out.Bed, r.Bed[last])
	out.Surface = append(out.Surface, r.Surface[last])
	out.Discharge = append(out.Discharge, r.Discharge[last])
	out.Width = append(out.Width, r.Width[last])
	return out
}
