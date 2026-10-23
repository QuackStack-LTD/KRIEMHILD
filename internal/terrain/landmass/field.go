// Package landmass evaluates persisted spherical landform grammar independently
// of the simulation grid, tile level, request order, or cache state.
package landmass

import (
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/terrain/tectonics"
	"math"
)

func Hash(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	return x ^ (x >> 31)
}
func noise(seed uint64, x, y, z float64) float64 {
	ix, iy, iz := int64(math.Floor(x)), int64(math.Floor(y)), int64(math.Floor(z))
	x -= float64(ix)
	y -= float64(iy)
	z -= float64(iz)
	fx, fy, fz := x, y, z
	smooth := func(t float64) float64 { return t * t * t * (t*(t*6-15) + 10) }
	x = smooth(x)
	y = smooth(y)
	z = smooth(z)
	out := 0.
	for dz := int64(0); dz < 2; dz++ {
		for dy := int64(0); dy < 2; dy++ {
			for dx := int64(0); dx < 2; dx++ {
				h := Hash(seed ^ uint64(ix+dx)*0x9e3779b97f4a7c15 ^ uint64(iy+dy)*0x632be59bd9b4e019 ^ uint64(iz+dz)*0x85157af5)
				w := func(t float64, i int64) float64 {
					if i == 0 {
						return 1 - t
					}
					return t
				}
				gx, gy, gz := fx-float64(dx), fy-float64(dy), fz-float64(dz)
				gradients := [12][3]float64{{1, 1, 0}, {-1, 1, 0}, {1, -1, 0}, {-1, -1, 0}, {1, 0, 1}, {-1, 0, 1}, {1, 0, -1}, {-1, 0, -1}, {0, 1, 1}, {0, -1, 1}, {0, 1, -1}, {0, -1, -1}}
				g := gradients[h%12]
				out += (gx*g[0] + gy*g[1] + gz*g[2]) * 1.6 * w(x, dx) * w(y, dy) * w(z, dz)
			}
		}
	}
	return out
}
func Noise(seed uint64, lon, lat, freq float64) float64 {
	c := math.Cos(lat)
	return noise(seed, c*math.Cos(lon)*freq, c*math.Sin(lon)*freq, math.Sin(lat)*freq)
}
func strokeDistance(s model.Stroke, x, y float64) float64 {
	best := -10.
	for i, k := range s.Knots {
		if i == 0 {
			best = math.Max(best, k[2]-math.Hypot(x-k[0], y-k[1]))
			continue
		}
		a := s.Knots[i-1]
		dx, dy := k[0]-a[0], k[1]-a[1]
		t := model.Clamp(((x-a[0])*dx+(y-a[1])*dy)/math.Max(1e-12, dx*dx+dy*dy), 0, 1)
		best = math.Max(best, a[2]+(k[2]-a[2])*t-math.Hypot(x-a[0]-t*dx, y-a[1]-t*dy))
	}
	return best
}

// Potential uses azimuthal-equidistant local coordinates. At the poles longitude
// is immaterial; all deformation is sampled on the 3D unit sphere.
func Potential(p *model.Planet, lon, lat float64) float64 {
	return potential(p, lon, lat, nil)
}

type massAngles struct{ sinLat, cosLat, sinRotation, cosRotation, limit float64 }

// Field prepares constants once per sampling request. It is immutable and its
// evaluation is bit-identical to the unprepared path used during generation.
type Field struct {
	planet *model.Planet
	angles []massAngles
}

func NewField(p *model.Planet) *Field {
	f := &Field{planet: p, angles: make([]massAngles, len(p.Masses))}
	for i, m := range p.Masses {
		a := &f.angles[i]
		a.sinLat, a.cosLat = math.Sincos(m.Center[1])
		a.sinRotation, a.cosRotation = math.Sincos(m.Rotation)
		a.limit = math.Cos(m.Extent + .15)
	}
	return f
}
func (f *Field) Bed(lon, lat float64) (float64, float64, float64) {
	return bedAt(f.planet, lon, lat, potential(f.planet, lon, lat, f.angles))
}
func potential(p *model.Planet, lon, lat float64, prepared []massAngles) float64 {
	lon = model.Wrap(lon)
	lat = model.Clamp(lat, -math.Pi/2, math.Pi/2)
	sl, cl := math.Sincos(lat)
	warpX := Noise(p.Seed+1, lon, lat, 5) * .045 * p.Spec.Complexity
	warpY := Noise(p.Seed+2, lon, lat, 7) * .035 * p.Spec.Complexity
	land, cut := -10., -10.
	for i, m := range p.Masses {
		dlon := model.Wrap(lon - m.Center[0])
		var s0, c0, sn, cs, limit float64
		if prepared != nil {
			a := prepared[i]
			s0, c0, sn, cs, limit = a.sinLat, a.cosLat, a.sinRotation, a.cosRotation, a.limit
		} else {
			s0, c0 = math.Sincos(m.Center[1])
			limit = math.Cos(m.Extent + .15)
		}
		sd, cd := math.Sincos(dlon)
		dot := model.Clamp(s0*sl+c0*cl*cd, -1, 1)
		if dot < limit {
			continue
		}
		angle := math.Acos(dot)
		k := 1.
		if angle > 1e-8 {
			k = angle / math.Sin(angle)
		}
		xx, yy := k*cl*sd, k*(c0*sl-s0*cl*cd)
		if prepared == nil {
			sn, cs = math.Sincos(m.Rotation)
		}
		x, y := xx*cs+yy*sn+warpX, -xx*sn+yy*cs+warpY
		for _, s := range m.Strokes {
			land = math.Max(land, strokeDistance(s, x, y))
		}
		for _, s := range m.Cuts {
			cut = math.Max(cut, strokeDistance(s, x, y))
		}
	}
	if land < -.3 {
		return land
	}
	coast := 0.
	amp := .018 * p.Spec.Complexity
	for oct := 0; oct < 14; oct++ {
		coast += Noise(p.Seed+uint64(80+oct), lon, lat, 18*math.Pow(2.15, float64(oct))) * amp
		amp *= .46
	}
	// All global cuts apply AFTER the union. Sea adjustment cannot refill them.
	return math.Min(land+coast+p.SeaOffset, -cut+coast*.35)
}

func Bed(p *model.Planet, lon, lat float64) (bed, coast, rough float64) {
	return bedAt(p, lon, lat, Potential(p, lon, lat))
}
func bedAt(p *model.Planet, lon, lat, coast float64) (bed, distance, rough float64) {
	rough = math.Abs(Noise(p.Seed+301, lon, lat, 9))
	plate := tectonics.At(p, lon, lat)
	if coast <= 0 {
		d := -coast * p.Spec.RadiusKM
		shelf := -30 - d*1.8
		if p.Spec.Version != "continuous-1" {
			shelf = -d * 1.8
		}
		deep := -4200 - 1000*Noise(p.Seed+303, lon, lat, 6) - 2800*plate.Convergence + 2200*plate.Divergence
		t := model.Clamp((d-100)/190, 0, 1)
		t = t * t * (3 - 2*t)
		return shelf*(1-t) + deep*t, coast, rough
	}
	base := 90 + 1300*(1-math.Exp(-coast*5))
	ridge := .3*math.Pow(math.Max(0, 1-math.Abs(Noise(p.Seed+302, lon, lat, 6)*5)), 3) + plate.Convergence*1.7
	if p.Spec.Version == model.Version && p.Spec.ReliefVersion >= 1 {
		// Broad uplift supports connected subsidiary ridges. Vary crest height
		// along each belt and allow old, subdued interior ranges as well as active
		// boundaries. These fields are materialized before climate and drainage.
		wl := lon + .07*Noise(p.Seed+1800, lon, lat, 5)
		wp := model.Clamp(lat+.055*Noise(p.Seed+1801, lon, lat, 6), -math.Pi/2, math.Pi/2)
		oldBelt := math.Pow(math.Max(0, 1-math.Abs(Noise(p.Seed+302, wl, wp, 6)*3.5)), 3)
		crest := .65 + .65*model.Clamp(.5+Noise(p.Seed+1802, wl, wp, 13), 0, 1)
		ridge = (.32*oldBelt + plate.Convergence*1.55) * crest
	}
	ridge *= model.Clamp(coast*45, 0, 1)
	local := 0.
	amplitude := 220.
	for o := 0; o < 14; o++ {
		local += Noise(p.Seed+uint64(310+o), lon, lat, 35*math.Pow(2, float64(o))) * amplitude
		amplitude *= .48
	}
	bed = (base + ridge*(2100+1700*rough) + local) * p.Spec.Relief * model.Clamp(coast*300, 0, 1)
	if p.Spec.Version != "continuous-1" {
		// Broad plateaus and rift basins are structural. Smaller ridges/gullies
		// follow roughness and distance inland; all frequencies have world addresses.
		plateau := model.Clamp((Noise(p.Seed+900, lon, lat, 5)-.12)*5, 0, 1)
		plateau = plateau * plateau * (3 - 2*plateau)
		basin := math.Pow(model.Clamp((Noise(p.Seed+901, lon, lat, 7)-.25)*3, 0, 1), 2)
		interior := model.Clamp(coast*18, 0, 1)
		bed += (plateau*1200 - basin*1500) * interior * p.Spec.Relief
		bed -= math.Pow(1-math.Abs(Noise(p.Seed+902, lon, lat, 110)), 8) * 100 * rough * interior
		return bed, coast, rough
	}
	return math.Max(1, bed), coast, rough
}
