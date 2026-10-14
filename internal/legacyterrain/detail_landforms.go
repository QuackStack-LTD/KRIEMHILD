package terrain

import "math"

// Smooth, coordinate-addressed fields describe landforms, rather than a fresh
// random displacement at each zoom. All wavelengths are in parent-world units.
func (m *DetailModel) detailNoise(x, y float64, salt int) float64 {
	ix, iy := int(math.Floor(x)), int(math.Floor(y))
	a, b := smooth(x-float64(ix)), smooth(y-float64(iy))
	n := func(i, j int) float64 { return detailHash(m.Seed, i, j, salt)*2 - 1 }
	return (n(ix, iy)*(1-a)+n(ix+1, iy)*a)*(1-b) + (n(ix, iy+1)*(1-a)+n(ix+1, iy+1)*a)*b
}

func (m *DetailModel) buildFoothills() {
	e := m.World
	w, h := e.Options.Columns, e.Options.Rows
	m.foothills = make([]float64, w*h)
	// The surrounding apron reaches several parent cells beyond a range. This
	// changes sub-cell relief only: it cannot replace the successful world ranges.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sum, weight := 0., 0.
			for dy := -5; dy <= 5; dy++ {
				for dx := -5; dx <= 5; dx++ {
					xx, yy := max(0, min(w-1, x+dx)), max(0, min(h-1, y+dy))
					g := math.Exp(-float64(dx*dx+dy*dy) / 12)
					sum += g * math.Max(e.get("highland", yy*w+xx), e.get("mountainCore", yy*w+xx))
					weight += g
				}
			}
			m.foothills[y*w+x] = sum / weight
		}
	}
}

func (m *DetailModel) apron(x, y float64) float64 {
	w, h := m.World.Options.Columns, m.World.Options.Rows
	x, y = clamp(x, 0, float64(w-1)), clamp(y, 0, float64(h-1))
	ix, iy := int(x), int(y)
	a, b := x-float64(ix), y-float64(iy)
	at := func(dx, dy int) float64 { return m.foothills[min(h-1, iy+dy)*w+min(w-1, ix+dx)] }
	return (at(0, 0)*(1-a)+at(1, 0)*a)*(1-b) + (at(0, 1)*(1-a)+at(1, 1)*a)*b
}

func (m *DetailModel) landformPotential(x, y float64) float64 {
	core := m.field("mountainCore", x, y)
	wetland := clamp(m.field("wetland", x, y), 0, 1)
	dune := clamp(m.field("dune", x, y), 0, 1)
	moisture := clamp(m.field("moisture", x, y), 0, 1)
	// Warped bands form connected ridges and their intervening valleys. Their
	// orientation follows the parent slope, with a slowly varying geological grain.
	gx := m.field("elevation", x+.5, y) - m.field("elevation", x-.5, y)
	gy := m.field("elevation", x, y+.5) - m.field("elevation", x, y-.5)
	angle := math.Atan2(gy, gx) + .35*m.detailNoise(x*.12, y*.12, 11)
	// Avoid an orientation field rotating around the world origin: blend fixed
	// directional fields instead of evaluating a position-dependent rotated frame.
	nx := m.detailNoise(x*1.3, y*.42, 13)
	ny := m.detailNoise(x*.42, y*1.3, 13)
	grain := nx*math.Cos(angle)*math.Cos(angle) + ny*math.Sin(angle)*math.Sin(angle)
	hills := m.detailNoise(x*1.05, y*1.05, 19)
	basin := m.detailNoise(x*.62, y*.62, 23)
	basin = -math.Pow(math.Max(0, -basin), 2)
	ridge := 1 - 2*math.Abs(grain)
	broad := .72*hills + .55*ridge + .8*basin
	plateau := math.Tanh(m.detailNoise(x*.9, y*.9, 31)*3) * .28
	geology := m.field("geology", x, y)
	broad += plateau * math.Exp(-math.Pow((geology-5)*2, 2))
	amp := (180 + 420*m.apron(x, y)) * (1 - .7*clamp(core, 0, 1)) * (1 - .93*wetland) * (1 - .7*dune)
	// Fine gullies are nested in the same grain; dry terrain carries wind-aligned
	// dune trains, wet floodplains have low hummocks, exposed ground has small ribs.
	fine := m.detailNoise(x*4, y*4, 37)*16 + m.detailNoise(x*13, y*13, 43)*3
	gullies := -math.Pow(1-math.Abs(m.detailNoise(x*5, y*2.2, 47)), 8) * 18 * moisture
	wind := m.field("duneOrientation", x, y)
	dunePhase := (x*math.Cos(wind)+y*math.Sin(wind))*28 + 2*m.detailNoise(x*2, y*2, 53)
	dunes := dune * (18*math.Sin(dunePhase) + 4*math.Sin(dunePhase*2))
	land := amp*broad + fine*(1-.8*wetland) + gullies + dunes
	if depth := m.field("waterDepth", x, y); depth > 0 {
		shelf := m.field("shelf", x, y)
		// Submerged ridges, channels and sediment bars belong to the same bed.
		bars := math.Sin(x*12+3*m.detailNoise(x, y, 61)) * math.Sin(y*9) * 12 * shelf
		reef := m.field("reef", x, y) * math.Max(0, ridge) * 18
		bed := broad*(65+80*shelf) + fine*.6 + bars + reef
		t := smooth(depth / 150)
		return land*(1-t) + bed*t
	}
	return land
}

type riverSample struct {
	regime                                              string
	depth                                               float64
	valid                                               bool
	distance, width, valley, level, discharge, grade, t float64
}

func (m *DetailModel) indexDrainage() {
	m.channels = map[int][]int{}
	w, h := m.World.Options.Columns, m.World.Options.Rows
	for index, f := range m.Features {
		x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		for _, p := range f.Path {
			x0 = math.Min(x0, p[0])
			y0 = math.Min(y0, p[1])
			x1 = math.Max(x1, p[0])
			y1 = math.Max(y1, p[1])
		}
		for y := max(0, int(math.Floor(y0-1))); y <= min(h-1, int(math.Ceil(y1+1))); y++ {
			for x := max(0, int(math.Floor(x0-1))); x <= min(w-1, int(math.Ceil(x1+1))); x++ {
				m.channels[y*w+x] = append(m.channels[y*w+x], index)
			}
		}
	}
}

func (m *DetailModel) riverAt(x, y float64) riverSample {
	best := riverSample{distance: math.Inf(1)}
	score := math.Inf(1)
	for _, index := range m.channels[m.cell(x, y)] {
		f := m.Features[index]
		if f.Discharge < 2 && f.Kind != "canal" {
			continue
		} // Small surface streams do not carve major valleys.
		r := SampleChannel(f, x, y)
		if r.Valid && r.Distance/r.Valley < score {
			score = r.Distance / r.Valley
			best = riverSample{valid: true, distance: r.Distance, width: r.Width, valley: r.Valley, level: r.Level, discharge: r.Discharge, grade: r.Grade, t: r.T, regime: f.Regime, depth: f.Depth}
		}
	}
	return best
}

func (m *DetailModel) sourceDistance(x, y float64) float64 {
	d := 1.
	for _, index := range m.channels[m.cell(x, y)] {
		f := m.Features[index]
		for _, p := range [][3]float64{f.Path[0], f.Path[len(f.Path)-1]} {
			d = math.Min(d, math.Hypot(x-p[0], y-p[1]))
		}
	}
	return d
}

func (m *DetailModel) residual(x, y float64) float64 {
	ix, iy := math.Floor(x), math.Floor(y)
	a, b := x-ix, y-iy
	if a == 0 && b == 0 {
		return 0
	}
	f := m.landformPotential
	anchor := (f(ix, iy)*(1-a)+f(ix+1, iy)*a)*(1-b) + (f(ix, iy+1)*(1-a)+f(ix+1, iy+1)*a)*b
	relief := f(x, y) - anchor
	p := m.base(x, y)
	r := m.riverAt(x, y)
	if r.valid {
		// Existing source and confluence anchors survive subdivision as well.
		relief *= smooth(math.Hypot(r.distance, math.Min(r.t, 1-r.t)) / .025)
	}
	if r.valid && (r.discharge >= 2 || r.depth > 0) && p.WaterBody == 0 {
		target, valley := ChannelBed(ChannelSample{Valid: true, Distance: r.distance, Width: r.width, Valley: r.valley, Level: r.level, Discharge: r.discharge, Grade: r.grade, Depth: r.depth})
		// Preserve root samples while incising the intervening reach. The water
		// profile remains the inherited downhill hydraulic grade, not the local bed.
		anchorFade := smooth(math.Hypot(math.Min(a, 1-a), math.Min(b, 1-b)) / .16)

		// Erosion lowers the channel; alluvial deposition levels adjacent local
		// hollows into its floodplain, avoiding perched water beside a lower bank.
		relief += (target - p.Elevation - relief) * valley * anchorFade
	}
	return relief
}

func (m *DetailModel) refinement(x, y float64, level int) float64 {
	if level <= 0 {
		return 0
	}
	s := math.Exp2(float64(min(DetailLevels, level)))
	u, v := x*s, y*s
	ix, iy := math.Floor(u), math.Floor(v)
	a, b := u-ix, v-iy
	if a == 0 && b == 0 {
		return m.residual(x, y)
	}
	return (m.residual(ix/s, iy/s)*(1-a)+m.residual((ix+1)/s, iy/s)*a)*(1-b) + (m.residual(ix/s, (iy+1)/s)*(1-a)+m.residual((ix+1)/s, (iy+1)/s)*a)*b
}
