package terrain

import (
	"fmt"
	"math"
	"sort"
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
	for _, p := range s.Spines {
		land = math.Max(land, spineField(x, y, p))
	}
	for _, p := range s.Cuts {
		land = math.Min(land, -spineField(x, y, p)*1.8)
	}
	return land
}

// Geometry is generated in local coordinates, then rotated and translated.
// Formation choice is varied within tectonic settings, never a copied outline.
func makeLandmass(seed uint32, formation string, center [2]float64, radius, bearing float64, warp LandShapeOptions) LandmassStructure {
	rng := RNG(seed)
	s := LandmassStructure{Version: "organic-spines-1", Formation: formation, Origin: center, Center: center, Scale: radius, Seed: seed, Warp: warp}
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
	cs, sn := math.Cos(bearing), math.Sin(bearing)
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

func (e *Environment) continentalNuclei(points []Point) []LandmassStructure {
	count := 0
	for _, p := range points {
		if p.K == 1 {
			count++
		}
	}
	w, h := float64(e.Options.Columns), float64(e.Options.Rows)
	radius := math.Sqrt(w*h/math.Max(2, float64(count))) * .45
	shapes := []LandmassStructure{}
	seed := Seed(e.Options.Seed)
	for k, p := range points {
		if p.K != 1 {
			continue
		}
		rng := RNG(seed ^ uint32(k+1)*0x738a913)
		center := [2]float64{p.X, p.Y}
		a, b, _, _, convergence, bearing := e.platesAt(center[0], center[1])
		names := []string{"curved", "branched", "hook", "isthmus", "crescent", "enclosed-basin", "indented"}
		formation := names[int(rng.Next()*float64(len(names)))]
		process := "erosion-and-shelf-flooding"
		if convergence < -.3 {
			process = "continental-rifting"
			if rng.Next() < .65 {
				formation = "rifted"
			}
		}
		if convergence > .3 {
			process = "continental-collision"
			if !e.Plates[a].Continental || !e.Plates[b].Continental {
				process = "subduction-margin"
				if rng.Next() < .5 {
					formation = "crescent"
				}
			} else if rng.Next() < .5 {
				formation = "branched"
			}
		}
		// High-latitude uplifted margins preserve drowned glacial troughs.
		latitude := e.Options.LatitudeNorth + (e.Options.LatitudeSouth-e.Options.LatitudeNorth)*center[1]/h
		if math.Abs(latitude) > 55 && convergence > .2 && rng.Next() < .6 {
			formation = "indented"
			process = "glacially-eroded-margin"
		}
		warp := e.Options.shapeOptions()
		warp.Large *= .6 + rng.Next()*.9
		warp.Medium *= .5 + rng.Next()
		warp.Small *= .3 + rng.Next()*1.3
		s := makeLandmass(seed^uint32(k+1)*0x471cab31, formation, center, radius*(.55+rng.Next()*.6), bearing+(rng.Next()-.5)*1.2, warp)
		s.ID = fmt.Sprintf("land-structure/%08x/%d", seed, k)
		s.Process = process
		s.Plate = a
		s.OtherPlate = b
		// Place complete foundations inside the overview, rather than slicing
		// their land at a rectangular mask. Partial maps can continue off frame.
		if e.Geography == nil || e.Geography.Coverage == "planet" {
			minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
			for _, path := range s.Spines {
				for _, v := range path {
					pad := v.Width*1.15 + s.Scale*.12
					minX = math.Min(minX, v.X-pad)
					maxX = math.Max(maxX, v.X+pad)
					minY = math.Min(minY, v.Y-pad)
					maxY = math.Max(maxY, v.Y+pad)
				}
			}
			margin := float64(e.Margin) + 1
			scale := math.Min(1, math.Min((w-2*margin)/(maxX-minX), (h-2*margin)/(maxY-minY)))
			cx, cy := (minX+maxX)/2, (minY+maxY)/2
			tx := clamp(cx, margin+(maxX-minX)*scale/2, w-margin-(maxX-minX)*scale/2)
			ty := clamp(cy, margin+(maxY-minY)*scale/2, h-margin-(maxY-minY)*scale/2)
			// Evaluate actual fitted candidates: choosing corners before fitting
			// would repeatedly push unrelated masses onto the same central position.
			if len(shapes) > 0 {
				best := math.Inf(-1)
				for trial := 0; trial < 64; trial++ {
					px := margin + (maxX-minX)*scale/2 + rng.Next()*(w-2*margin-(maxX-minX)*scale)
					py := margin + (maxY-minY)*scale/2 + rng.Next()*(h-2*margin-(maxY-minY)*scale)
					score := 0.
					for _, path := range s.Spines {
						for j := 0; j < len(path); j += 3 {
							x, y := px+(path[j].X-cx)*scale, py+(path[j].Y-cy)*scale
							nearest := math.Inf(-1)
							for _, old := range shapes {
								nearest = math.Max(nearest, old.field(x, y))
							}
							score += clamp(-nearest, -3, 2)
						}
					}
					score -= .008 * math.Hypot(px-p.X, py-p.Y)
					if score > best {
						best = score
						tx, ty = px, py
					}
				}
			}
			s.Center = [2]float64{tx + (s.Center[0]-cx)*scale, ty + (s.Center[1]-cy)*scale}
			s.Scale *= scale
			for _, paths := range [][][]LandSpinePoint{s.Spines, s.Cuts} {
				for _, path := range paths {
					for j := range path {
						path[j].X = tx + (path[j].X-cx)*scale
						path[j].Y = ty + (path[j].Y-cy)*scale
						path[j].Width *= scale
					}
				}
			}
		}
		shapes = append(shapes, s)
	}
	return shapes
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

func (e *Environment) generateContinentalMask(points []Point, sample func(float64, float64) float64) {
	w, h := e.Options.Columns, e.Options.Rows
	n := w * h
	e.ContinentalStructures = e.continentalNuclei(points)
	scores := make([]float64, n)
	eligible := make([]int, 0, n)
	owners := make([]int, n)
	clearances := make([]float64, n)
	for i := range scores {
		x, y := float64(i%w), float64(i/w)
		scores[i], owners[i] = continentalMass(x, y, e.ContinentalStructures)
		clearance := continentalClearance(x, y, w, h, e.Margin, clamp((100-e.Options.LandPercent)/50, .2, 1), sample)
		if e.Geography != nil && e.Geography.Coverage != "planet" {
			clearance = float64(min(w, h))
		}
		// This offshore guard is normally outside the generated structure. A broad
		// deformation prevents high-coverage worlds from following a straight rim.
		scores[i] -= (1 - smooth(clearance/(float64(min(w, h))*.09))) * .7
		clearances[i] = clearance
	}
	// Submerged sutures have resolved width; sub-cell competitive boundaries
	// must not become diagonal land bridges or one-pixel cracks. Collision can
	// weld related bodies, whereas other overlaps are drowned continental seams.
	seam := make([]int, n)
	for i := range seam {
		seam[i] = 1
		for _, j := range e.neighbors(i) {
			if owners[i] != owners[j] {
				a, b := e.ContinentalStructures[owners[i]], e.ContinentalStructures[owners[j]]
				if a.Plate == b.Plate && a.Process == "continental-collision" && b.Process == "continental-collision" {
					continue
				}
				seam[i] = 0
				break
			}
		}
	}
	distance := e.gridDistance(seam, 0)
	for i := range scores {
		width := 1.2 + 1.5*sample(float64(i%w)/17+911, float64(i/w)/17+953)
		scores[i] *= smooth((distance[i] - .5) / width)
		if clearances[i] > 0 && distance[i] >= 1 {
			eligible = append(eligible, i)
		}
	}
	// Relative spine distance gives narrow and broad cores the same priority:
	// coverage changes widen shores rather than erase the axes of peninsulas.
	sort.SliceStable(eligible, func(a, b int) bool { return scores[eligible[a]] > scores[eligible[b]] })
	count := min(len(eligible), int(round(float64(n)*e.Options.LandPercent/100)))
	for _, i := range eligible[:count] {
		e.Mask[i] = 1
	}
	e.Fields["continentalShape"] = scores
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
	if s.ID == "" || s.Version != "organic-spines-1" || !finite(s.Scale) || s.Scale <= 0 || s.Scale > 1024 || len(s.Spines) == 0 || len(s.Spines) > 16 || len(s.Cuts) > 16 {
		return fmt.Errorf("invalid land structure %s", s.ID)
	}
	switch s.Formation {
	case "curved", "branched", "hook", "isthmus", "crescent", "enclosed-basin", "rifted", "indented":
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
