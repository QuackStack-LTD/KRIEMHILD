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
