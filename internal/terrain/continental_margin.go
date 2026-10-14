package terrain

import (
	"fmt"
	"math"
)

// Amplitudes multiply independent large, medium and fine domain warps. Nil
// options preserve defaults when opening older projects.
type LandShapeOptions struct {
	Large  float64 `json:"large"`
	Medium float64 `json:"medium"`
	Small  float64 `json:"small"`
}

func (o EnvironmentOptions) shapeOptions() LandShapeOptions {
	if o.LandShape != nil {
		return *o.LandShape
	}
	return LandShapeOptions{1, 1, 1}
}

// A variable-width spine is a continuous swept region, not an ellipse. Cuts
// use the same geometry to represent rifts, gulfs and enclosed depressions.
type LandSpinePoint struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Width float64 `json:"width"`
}
type LandmassStructure struct {
	Envelope   []LandEnvelope     `json:"envelope,omitempty"`
	Version    string             `json:"version"`
	ID         string             `json:"id"`
	Formation  string             `json:"formation"`
	Process    string             `json:"process"`
	Plate      int                `json:"plate"`
	OtherPlate int                `json:"otherPlate"`
	Origin     [2]float64         `json:"origin"`
	Center     [2]float64         `json:"center"`
	Scale      float64            `json:"scale"`
	Seed       uint32             `json:"seed"`
	Warp       LandShapeOptions   `json:"warp"`
	Spines     [][]LandSpinePoint `json:"spines"`
	Cuts       [][]LandSpinePoint `json:"cuts,omitempty"`
}

// Filled continental cores and accreted lobes. Axes and coordinates are stored
// in world units so reload and detail evaluation share the same geometry.
type LandEnvelope struct {
	Center  [2]float64 `json:"center"`
	Axes    [2]float64 `json:"axes"`
	Bearing float64    `json:"bearing"`
}

func spineDistance(x, y float64, path []LandSpinePoint) float64 {
	best := math.Inf(-1)
	for k := 1; k < len(path); k++ {
		a, b := path[k-1], path[k]
		dx, dy := b.X-a.X, b.Y-a.Y
		t := clamp(((x-a.X)*dx+(y-a.Y)*dy)/math.Max(1e-12, dx*dx+dy*dy), 0, 1)
		best = math.Max(best, a.Width+(b.Width-a.Width)*t-math.Hypot(x-a.X-t*dx, y-a.Y-t*dy))
	}
	return best
}

func (s *LandmassStructure) rescale(factor float64) {
	s.Scale *= factor
	for k := range s.Envelope {
		l := &s.Envelope[k]
		for d := 0; d < 2; d++ {
			l.Center[d] = s.Center[d] + (l.Center[d]-s.Center[d])*factor
			l.Axes[d] *= factor
		}
	}
	for _, paths := range [][][]LandSpinePoint{s.Spines, s.Cuts} {
		for _, p := range paths {
			for k := range p {
				p[k].X = s.Center[0] + (p[k].X-s.Center[0])*factor
				p[k].Y = s.Center[1] + (p[k].Y-s.Center[1])*factor
				p[k].Width *= factor
			}
		}
	}
}

func spineField(x, y float64, path []LandSpinePoint) float64 {
	best := math.Inf(-1)
	for k := 1; k < len(path); k++ {
		a, b := path[k-1], path[k]
		dx, dy := b.X-a.X, b.Y-a.Y
		t := clamp(((x-a.X)*dx+(y-a.Y)*dy)/math.Max(1e-12, dx*dx+dy*dy), 0, 1)
		radius := math.Max(.05, a.Width+(b.Width-a.Width)*t)
		best = math.Max(best, 1-math.Hypot(x-a.X-t*dx, y-a.Y-t*dy)/radius)
	}
	return best
}

func (s LandmassStructure) field(x, y float64) float64 {
	land, cut := s.fieldParts(x, y)
	strength := 1.8
	if s.Version == "filled-sdf-2" {
		strength = 8
	}
	return math.Min(land, -cut*strength)
}
func (s LandmassStructure) fieldParts(x, y float64) (float64, float64) {
	r := s.Scale
	u, v := (x-s.Center[0])/r, (y-s.Center[1])/r
	// Warps act on the whole formation, before evaluating its spines and cuts.
	// Different frequencies/seeds avoid repeated concentric coast textures.
	large, medium, fine := noise(s.Seed^0x192abc31), noise(s.Seed^0x587ae913), noise(s.Seed^0x769ed112)
	u0, v0 := u, v
	u += (large(u0*1.1+17, v0*1.1+23) - .5) * .65 * s.Warp.Large
	v += (large(u0*1.1+73, v0*1.1+89) - .5) * .65 * s.Warp.Large
	u0, v0 = u, v
	u += (medium(u0*3.8+19, v0*3.8+37) - .5) * .22 * s.Warp.Medium
	v += (medium(u0*3.8+83, v0*3.8+53) - .5) * .22 * s.Warp.Medium
	u0, v0 = u, v
	u += (fine(u0*13+29, v0*13+47) - .5) * .065 * s.Warp.Small
	v += (fine(u0*13+61, v0*13+97) - .5) * .065 * s.Warp.Small
	x, y = s.Center[0]+u*r, s.Center[1]+v*r
	land := math.Inf(-1)
	for _, l := range s.Envelope {
		dx, dy := x-l.Center[0], y-l.Center[1]
		cs, sn := math.Cos(l.Bearing), math.Sin(l.Bearing)
		u, v := (dx*cs+dy*sn)/l.Axes[0], (-dx*sn+dy*cs)/l.Axes[1]
		land = math.Max(land, (1-math.Hypot(u, v))*math.Min(l.Axes[0], l.Axes[1])/r)
	}
	for _, p := range s.Spines {
		v := spineField(x, y, p)
		if s.Version == "filled-sdf-2" {
			v = spineDistance(x, y, p) / r
		}
		land = math.Max(land, v)
	}
	cut := math.Inf(-1)
	for _, p := range s.Cuts {
		v := spineField(x, y, p)
		if s.Version == "filled-sdf-2" {
			v = spineDistance(x, y, p) / r
		}
		cut = math.Max(cut, v)
	}
	return land, cut
}

// Geometry is generated in local coordinates, then rotated and translated.
// Formation choice is varied within tectonic settings, never a copied outline.
func makeLandmass(seed uint32, formation string, center [2]float64, radius, bearing float64, warp LandShapeOptions) LandmassStructure {
	rng := RNG(seed)
	s := LandmassStructure{Version: "filled-sdf-2", Formation: formation, Origin: center, Center: center, Scale: radius, Seed: seed, Warp: warp}
	add := func(path []LandSpinePoint, cut bool) {
		if cut {
			s.Cuts = append(s.Cuts, path)
		} else {
			s.Spines = append(s.Spines, path)
		}
	}
	curve := func(a, b, c, d [2]float64, wa, wb float64, cut bool) {
		p := []LandSpinePoint{}
		for k := 0; k <= 14; k++ {
			t := float64(k) / 14
			q := 1 - t
			width := wa + (wb-wa)*smooth(t)
			width *= 1 + .17*math.Sin(t*math.Pi*3+rng.Next()*.3)
			p = append(p, LandSpinePoint{q*q*q*a[0] + 3*q*q*t*b[0] + 3*q*t*t*c[0] + t*t*t*d[0], q*q*q*a[1] + 3*q*q*t*b[1] + 3*q*t*t*c[1] + t*t*t*d[1], width})
		}
		add(p, cut)
	}
	bend := (rng.Next() - .5) * 1.1
	switch formation {
	case "compact":
		curve([2]float64{-.5, -.1}, [2]float64{-.2, bend}, [2]float64{.3, -bend}, [2]float64{.5, .1}, .85, .7, false)
	case "elongated":
		curve([2]float64{-1, -.1}, [2]float64{-.4, bend}, [2]float64{.4, -bend}, [2]float64{1, .1}, .55, .65, false)
	case "crescent":
		arc := math.Pi * (1.2 + rng.Next()*.55)
		p := []LandSpinePoint{}
		for k := 0; k <= 22; k++ {
			t := float64(k) / 22
			a := -arc/2 + t*arc
			p = append(p, LandSpinePoint{.8 * math.Cos(a), .95 * math.Sin(a), .18 + .22*math.Sin(t*math.Pi)})
		}
		add(p, false)
	case "hook":
		curve([2]float64{-1, -.3}, [2]float64{-.2, -.6}, [2]float64{.9, -.3}, [2]float64{.8, .5}, .48, .2, false)
		curve([2]float64{.8, .5}, [2]float64{.8, .9}, [2]float64{.2, 1}, [2]float64{-.15, .7}, .2, .28, false)
	case "isthmus":
		p := []LandSpinePoint{}
		for k := 0; k <= 18; k++ {
			t := float64(k) / 18
			x := -1.1 + t*2.2
			p = append(p, LandSpinePoint{x, bend * math.Sin(t*math.Pi*2), .1 + .43*math.Pow(math.Abs(2*t-1), .65)})
		}
		add(p, false)
	case "rifted":
		curve([2]float64{-1, -.3}, [2]float64{-.3, -.4}, [2]float64{.5, .5}, [2]float64{1, .2}, .55, .5, false)
		curve([2]float64{-.8, -1}, [2]float64{-.25, -.5}, [2]float64{.05, .3}, [2]float64{.5, 1.2}, .09, .14, true)
	case "enclosed-basin":
		p := []LandSpinePoint{}
		// Complete asymmetric ring; rainfall/evaporation decide whether its basin
		// becomes a lake, a salt pan or a dry below-sea-level depression later.
		for k := 0; k <= 30; k++ {
			a := float64(k) * math.Pi * 2 / 30
			p = append(p, LandSpinePoint{.8 * math.Cos(a), .65*math.Sin(a) + .13*math.Sin(2*a), .24 + .12*(1+math.Sin(a+.5))})
		}
		add(p, false)
	case "branched":
		curve([2]float64{-1, -.2}, [2]float64{-.4, bend}, [2]float64{.5, -bend}, [2]float64{1.1, .1}, .43, .24, false)
		for k := 0; k < 3; k++ {
			x := -.65 + float64(k)*.6
			sign := 1.
			if k%2 == 0 {
				sign = -1
			}
			curve([2]float64{x, 0}, [2]float64{x + .3, sign * .3}, [2]float64{x - .4, sign * .8}, [2]float64{x - .15, sign * (.85 + rng.Next()*.35)}, .25, .1+rng.Next()*.16, false)
		}
	default: // curved and teardrop bodies have independently tapered ends.
		curve([2]float64{-1.2, -.2}, [2]float64{-.4, bend - 1}, [2]float64{.5, bend + .7}, [2]float64{1.05, .2}, .14+rng.Next()*.12, .38+rng.Next()*.17, false)
	}
	// Secondary peninsulas and collision accretions change large-scale shape,
	// not merely its perimeter texture. Leave ring interiors intact.
	if formation != "enclosed-basin" && formation != "crescent" && rng.Next() < .75 {
		sign := 1.
		if rng.Next() < .5 {
			sign = -1
		}
		curve([2]float64{.25, 0}, [2]float64{.25, sign * .5}, [2]float64{.8, sign * .8}, [2]float64{1.2, sign * .8}, .27, .13, false)
	}
	if formation == "indented" {
		// Long, narrowing inlet troughs enter a broad coastal body. They are
		// negative terrain geometry, subsequently flooded by marine connectivity.
		for k := 0; k < 4; k++ {
			x := -.65 + float64(k)*.4
			curve([2]float64{x, 1.1}, [2]float64{x + .1, .65}, [2]float64{x - .15, .1}, [2]float64{x - .2, -.15}, .11, .035, true)
		}
	}
	if formation != "enclosed-basin" && formation != "crescent" && formation != "isthmus" {
		// Unequal, independently oriented drowned arms cut into the filled core.
		// Their depth is macro geometry, independent of perimeter roughness.
		for k := 0; k < 1+int(rng.Next()*3); k++ {
			a := rng.Next() * math.Pi * 2
			cs, sn := math.Cos(a), math.Sin(a)
			turn := (rng.Next() - .5) * .65
			end := .15 + rng.Next()*.5
			curve([2]float64{cs * 1.65, sn * 1.65}, [2]float64{cs*1.05 - sn*turn, sn*1.05 + cs*turn}, [2]float64{cs*.65 + sn*turn, sn*.65 - cs*turn}, [2]float64{cs * end, sn * end}, .13+rng.Next()*.15, .035+rng.Next()*.065, true)
		}
	}
	cs, sn := math.Cos(bearing), math.Sin(bearing)
	core := func(x, y, a, b float64) {
		s.Envelope = append(s.Envelope, LandEnvelope{Center: [2]float64{center[0] + radius*(cs*x-sn*y), center[1] + radius*(sn*x+cs*y)}, Axes: [2]float64{radius * a, radius * b}, Bearing: bearing + (rng.Next()-.5)*.35})
	}
	switch formation {
	case "enclosed-basin": // Preserve the explicitly annular formation.
	case "crescent":
		core(.7, 0, .37, .65)
	case "isthmus":
		core(-.85, 0, .64, .56)
		core(.85, 0, .58, .64)
	case "hook":
		core(-.45, -.25, .82, .55)
	case "elongated":
		core(0, 0, 1.15, .62)
	default:
		core(-.12, 0, .92+rng.Next()*.25, .62+rng.Next()*.24)
		for k := 0; k < 2+int(rng.Next()*3); k++ {
			a := rng.Next() * math.Pi * 2
			d := .5 + rng.Next()*.4
			core(math.Cos(a)*d, math.Sin(a)*d, .3+rng.Next()*.4, .3+rng.Next()*.35)
		}
	}
	for _, paths := range [][][]LandSpinePoint{s.Spines, s.Cuts} {
		for _, p := range paths {
			for k := range p {
				x, y := p[k].X*radius, p[k].Y*radius
				p[k].X = center[0] + cs*x - sn*y
				p[k].Y = center[1] + sn*x + cs*y
				p[k].Width *= radius
			}
		}
	}
	return s
}

// Both shape grammar and relief consume the same plate geometry. Wrapping
// distances avoids a false tectonic boundary at the planetary longitude seam.
func (e *Environment) platesAt(x, y float64) (a, b int, da, db, convergence, bearing float64) {
	if x == math.Floor(x) && y == math.Floor(y) && x >= 0 && y >= 0 && x < float64(e.Options.Columns) && y < float64(e.Options.Rows) && len(e.plateCache) > 0 {
		p := e.plateCache[int(y)*e.Options.Columns+int(x)]
		return p.a, p.b, p.da, p.db, p.convergence, p.bearing
	}
	a, b, da, db = 0, 1, math.Inf(1), math.Inf(1)
	deltaX := func(dx float64) float64 {
		if e.Geography != nil && e.Geography.WrapX {
			dx = math.Remainder(dx, float64(e.Options.Columns))
		}
		return dx
	}
	for k, p := range e.Plates {
		dx, dy := deltaX(x-p.X), y-p.Y
		d := dx*dx + dy*dy
		if d < da {
			b, db, a, da = a, da, k, d
		} else if d < db {
			b, db = k, d
		}
	}
	pa, pb := e.Plates[a], e.Plates[b]
	dx, dy := deltaX(pb.X-pa.X), pb.Y-pa.Y
	convergence = ((pa.VX-pb.VX)*dx + (pa.VY-pb.VY)*dy) / math.Max(.001, math.Hypot(dx, dy))
	bearing = math.Atan2(dy, dx) + math.Pi/2
	return
}

type plateSample struct {
	a, b                         int
	da, db, convergence, bearing float64
}

func (e *Environment) cachePlates() {
	values := make([]plateSample, len(e.Mask))
	for i := range values {
		a, b, da, db, c, r := e.platesAt(float64(i%e.Options.Columns), float64(i/e.Options.Columns))
		values[i] = plateSample{a, b, da, db, c, r}
	}
	e.plateCache = values
}

func (e *Environment) continentalNuclei(_ []Point) []LandmassStructure {
	return e.layoutNuclei()
}

func continentalMass(x, y float64, shapes []LandmassStructure) (float64, int) {
	best := math.Inf(-1)
	owner := 0
	for k, s := range shapes {
		v := s.field(x, y)
		if v > best {
			best = v
			owner = k
		}
	}
	return .5 + .5*math.Tanh(best*1.7), owner
}

func (e *Environment) generateContinentalMask(_ []Point, sample func(float64, float64) float64) {
	e.separatedContinentalMask(sample)
}

// Reserve connected offshore water without imprinting the rectangular canvas
// on the continent. Each margin has independent, multiscale coastal geometry.
func continentalClearance(x, y float64, w, h, margin int, oceanRoom float64, sample func(float64, float64) float64) float64 {
	scale := float64(min(w, h)) * oceanRoom
	inset := func(t, side float64) float64 {
		warp := t*6 + (sample(t*3+side, side+31)-.5)*1.4
		return float64(margin) + scale*(.01+.04*sample(warp+side, side+71)+.03*sample(t*23+side, side+113))
	}
	return math.Min(math.Min(x-inset(y/float64(h), 157), float64(w-1)-x-inset(y/float64(h), 281)),
		math.Min(y-inset(x/float64(w), 419), float64(h-1)-y-inset(x/float64(w), 557)))
}

func validateLandmassStructure(s LandmassStructure) error {
	if s.ID == "" || (s.Version != "organic-spines-1" && s.Version != "filled-sdf-2") || !finite(s.Scale) || s.Scale <= 0 || s.Scale > 1024 || len(s.Spines) == 0 || len(s.Spines) > 16 || len(s.Cuts) > 16 || len(s.Envelope) > 16 {
		return fmt.Errorf("invalid land structure %s", s.ID)
	}
	for _, l := range s.Envelope {
		if !finite(l.Center[0]) || !finite(l.Center[1]) || !finite(l.Axes[0]) || !finite(l.Axes[1]) || !finite(l.Bearing) || l.Axes[0] <= 0 || l.Axes[1] <= 0 || l.Axes[0] > 2048 || l.Axes[1] > 2048 {
			return fmt.Errorf("invalid land envelope %s", s.ID)
		}
	}
	switch s.Formation {
	case "compact", "elongated", "curved", "branched", "hook", "isthmus", "crescent", "enclosed-basin", "rifted", "indented":
	default:
		return fmt.Errorf("unknown land formation %s", s.ID)
	}
	for _, v := range []float64{s.Center[0], s.Center[1], s.Origin[0], s.Origin[1], s.Warp.Large, s.Warp.Medium, s.Warp.Small} {
		if !finite(v) {
			return fmt.Errorf("non-finite land structure %s", s.ID)
		}
	}
	for _, v := range []float64{s.Warp.Large, s.Warp.Medium, s.Warp.Small} {
		if v < 0 || v > 6 {
			return fmt.Errorf("invalid land deformation %s", s.ID)
		}
	}
	for _, paths := range [][][]LandSpinePoint{s.Spines, s.Cuts} {
		for _, path := range paths {
			if len(path) < 2 || len(path) > 128 {
				return fmt.Errorf("invalid land spine %s", s.ID)
			}
			for _, p := range path {
				if !finite(p.X) || !finite(p.Y) || !finite(p.Width) || math.Abs(p.X) > 4096 || math.Abs(p.Y) > 4096 || p.Width <= 0 || p.Width > 1024 {
					return fmt.Errorf("invalid land spine geometry %s", s.ID)
				}
			}
		}
	}
	return nil
}
func (e *Environment) ValidateLandStructures() error {
	if len(e.ContinentalStructures) > 40 {
		return fmt.Errorf("too many continental structures")
	}
	ids := map[string]bool{}
	for _, s := range e.ContinentalStructures {
		if err := validateLandmassStructure(s); err != nil {
			return err
		}
		if ids[s.ID] || s.Plate < 0 || s.Plate >= len(e.Plates) || s.OtherPlate < 0 || s.OtherPlate >= len(e.Plates) || s.Plate == s.OtherPlate {
			return fmt.Errorf("invalid continental provenance %s", s.ID)
		}
		ids[s.ID] = true
	}
	return nil
}
