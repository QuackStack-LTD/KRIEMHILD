package landmass

import (
	"kriemhild/internal/terrain/model"
	"math"
)

// Conditioning is sampled from immutable climate/tectonic fields; no visited-tile
// history enters it. The envelope is zero at macro grid anchors. All child and
// neighboring tile anchors sample this same function, never tile-local noise.
func Conditioned(p *model.Planet, lon, lat, bed, coast float64) (float64, float64) {
	if p.Spec.Version == "continuous-1" {
		return bed, coast
	}
	g := p.Grid
	x := (model.Wrap(lon) + math.Pi) / model.Tau * float64(g.Width)
	y := (.5-lat/math.Pi)*float64(g.Height) - .5
	envelope := math.Pow(math.Sin(math.Pi*x)*math.Sin(math.Pi*y), 2)
	wet := g.At("moisture", lon, lat)
	cold := model.Clamp((5-g.At("temperature", lon, lat))/15, 0, 1)
	rough := g.At("roughness", lon, lat)
	if math.Abs(coast*p.Spec.RadiusKM) < 8 {
		// Narrow flooded inlets on cold rugged margins; quieter bars on warm coasts.
		notch := math.Pow(1-math.Abs(Noise(p.Seed+1501, lon, lat, 460)), 12)
		delta := (.00015*(1-cold)*(1-rough) - .00055*cold*rough*notch) * envelope * math.Exp(-math.Abs(coast)*p.Spec.RadiusKM/3)
		old := coast
		coast += delta
		if old*coast < 0 {
			bed = coast * p.Spec.RadiusKM * 1000 * .025
		} else {
			bed += delta * p.Spec.RadiusKM * 1000 * .025
		}
	}
	if coast > 0 {
		// Wind-aligned dune crests in dry low relief; gullies in wet terrain, and
		// sharper glacial ridges on cold high ground. Features taper at shores.
		inland := model.Clamp(coast*p.Spec.RadiusKM/2, 0, 1) * envelope
		if p.Spec.Version == model.Version && p.Spec.ReliefVersion >= 1 {
			// A materialized surface needs no zero-at-climate-grid envelope.
			// That envelope embossed regular straight troughs into every orogen.
			inland = model.Clamp(coast*p.Spec.RadiusKM/2, 0, 1)
		}
		wind := g.At("windX", lon, lat)
		dune := math.Sin((math.Cos(lon)*math.Cos(lat)*wind+math.Sin(lat)*.4)*13000 + Noise(p.Seed+1502, lon, lat, 900)*5)
		bed += inland * ((1-wet)*model.Clamp(1-bed/1100, 0, 1)*dune*7 - wet*rough*math.Pow(1-math.Abs(Noise(p.Seed+1503, lon, lat, 1300)), 8)*35 + cold*rough*math.Abs(Noise(p.Seed+1504, lon, lat, 700))*30)
		// Resolve actual mountain-scale relief inside the parent orogen. The old
		// 220 m octave series began at ~1,000 km wavelengths, so even its "local"
		// slopes were almost planar in regional 3D. These ridges span kilometres,
		// carry hundreds of metres of relief and remain zero at parent anchors.
		mountain := model.Clamp((bed-700)/1800, 0, 1) * (.25 + .75*rough)
		amplitude, frequency := 1100., 1800.
		if p.Spec.Version == model.Version {
			amplitude, frequency = 450, 128
			if p.Spec.ReliefVersion >= 1 {
				amplitude, frequency = 950, 45
			}
		}
		for octave := 0; octave < 6; octave++ {
			if p.Spec.Version == model.Version && frequency > float64(max(1024, p.Spec.SurfaceWidth))*.25 {
				break
			}
			wl, wp := lon, lat
			if p.Spec.Version == model.Version && p.Spec.ReliefVersion >= 1 {
				wl += .014 * Noise(p.Seed+1700, lon, lat, 18)
				wp = model.Clamp(wp+.012*Noise(p.Seed+1701, lon, lat, 19), -math.Pi/2, math.Pi/2)
			}
			ridge := math.Pow(1-math.Abs(Noise(p.Seed+uint64(1600+octave), wl, wp, frequency)), 3) - .4
			bed += inland * mountain * ridge * amplitude * p.Spec.Relief
			amplitude *= .48
			frequency *= 2.2
		}
	}
	return bed, coast
}
