package terrain

import "math"

type continentalNucleus struct{ x, y, cosine, sine, radiusX, radiusY float64 }

func continentalNuclei(points []Point, w, h int, sample func(float64, float64) float64) []continentalNucleus {
	count := 0
	for _, p := range points {
		if p.K == 1 {
			count++
		}
	}
	radius := math.Sqrt(float64(w*h)/math.Max(2, float64(count))) * .52
	result := []continentalNucleus{}
	for _, p := range points {
		if p.K != 1 {
			continue
		}
		angle := sample(p.X*.07+731, p.Y*.07+719) * math.Pi * 2
		// Foundations close offshore before the image frame can clip them.
		// Their elongated, overlapping lobes form continental bodies and
		// peninsulas; the seed's water provinces still separate ocean basins.
		x := float64(w)*.12 + p.X*.76
		y := float64(h)*.12 + p.Y*.76
		result = append(result, continentalNucleus{x, y, math.Cos(angle), math.Sin(angle), radius * (.8 + .5*sample(p.X+809, p.Y)), radius * (.48 + .35*sample(p.X, p.Y+821))})
	}
	return result
}

func continentalMass(x, y float64, nuclei []continentalNucleus) float64 {
	best := 0.
	for _, c := range nuclei {
		dx, dy := x-c.x, y-c.y
		u, v := (dx*c.cosine+dy*c.sine)/c.radiusX, (-dx*c.sine+dy*c.cosine)/c.radiusY
		best = math.Max(best, math.Exp(-u*u-v*v))
	}
	return best
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
