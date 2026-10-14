package generate

import (
	"fmt"
	"hash/fnv"
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
	"math"
	"math/rand/v2"
)

func seed(s string) uint64 { h := fnv.New64a(); h.Write([]byte(s)); return h.Sum64() }
func stream(s model.Spec, name string) *rand.Rand {
	v := seed(s.Seed + "/" + name)
	return rand.New(rand.NewPCG(v, landmass.Hash(v)))
}
func layout(p *model.Planet) {
	r := stream(p.Spec, fmt.Sprintf("layout/%d", p.Spec.LayoutVariant))
	shape := stream(p.Spec, "morphology")
	p.Seed = seed(p.Spec.Seed + "/surface")
	count := p.Spec.Continents
	scale := math.Sqrt(p.Spec.LandCoverage/.32) * math.Sqrt(5/float64(count))
	if p.Spec.Preset == "archipelago" {
		scale *= .55
		count += 4
	}
	if p.Spec.Preset == "supercontinent" {
		count = 3
		scale = 1.35
	}
	families := []string{"shield", "hook", "ribbon", "branch", "fractured", "crescent", "accreted"}
	if p.Spec.Version != "continuous-1" {
		families = []string{"broad-shield", "hook", "ribbon", "branch", "fractured", "crescent", "compact", "island-heavy", "indented", "split"}
	}
	for i := 0; i < count; i++ {
		if p.Spec.Version != "continuous-1" {
			shape = stream(p.Spec, fmt.Sprintf("morphology/%d", i))
		}
		center := model.Point{r.Float64()*model.Tau - math.Pi, math.Asin(r.Float64()*1.65 - .825)}
		radius := (.34 + shape.Float64()*.19) * scale
		// Rejection prevents accidental total overlap, without farthest-point spacing.
		for attempt := 0; attempt < 80; attempt++ {
			clear := true
			for _, m := range p.Masses {
				d := math.Acos(model.Clamp(math.Sin(center[1])*math.Sin(m.Center[1])+math.Cos(center[1])*math.Cos(m.Center[1])*math.Cos(center[0]-m.Center[0]), -1, 1))
				if d < radius+m.Extent*.48+.12 {
					clear = false
					break
				}
			}
			if clear || p.Spec.Preset == "supercontinent" {
				break
			}
			center = model.Point{r.Float64()*model.Tau - math.Pi, math.Asin(r.Float64()*1.65 - .825)}
		}
		if p.Spec.Preset == "supercontinent" && i > 0 {
			center = model.Point{p.Masses[0].Center[0] + float64(i)*.38, p.Masses[0].Center[1] + .15*float64(i)}
		}
		family := families[(i+int(shape.Uint64()%uint64(len(families))))%len(families)]
		m := model.Mass{ID: fmt.Sprintf("continent-%d", i), Family: family, Center: center, Rotation: shape.Float64() * model.Tau}
		stretchX, stretchY, shear := 1., 1., 0.
		if p.Spec.Version != "continuous-1" {
			stretchX = .7 + shape.Float64()*.7
			stretchY = .7 + shape.Float64()*.7
			shear = (shape.Float64() - .5) * .7
		}
		add := func(cut bool, ks ...model.Knot) {
			for j := range ks {
				ks[j][0], ks[j][1] = ks[j][0]*stretchX+ks[j][1]*shear, ks[j][1]*stretchY
				for k := range ks[j] {
					ks[j][k] *= radius
				}
				m.Extent = math.Max(m.Extent, math.Hypot(ks[j][0], ks[j][1])+ks[j][2])
			}
			s := model.Stroke{Knots: ks}
			if cut {
				m.Cuts = append(m.Cuts, s)
			} else {
				m.Strokes = append(m.Strokes, s)
			}
		}
		switch family {
		case "compact":
			add(false, model.Knot{-.6, -.3, .65}, model.Knot{.45, .2, .8}, model.Knot{.1, .65, .4})
			add(true, model.Knot{-1.3, .6, .27}, model.Knot{-.5, .25, .13})
		case "split":
			add(false, model.Knot{-.55, -.5, .68}, model.Knot{.5, .4, .7}, model.Knot{.65, 1, .3})
			add(true, model.Knot{.9, -1.1, .3}, model.Knot{.15, -.2, .23}, model.Knot{-.45, .35, .17})
		case "island-heavy":
			add(false, model.Knot{-.6, -.2, .55}, model.Knot{.15, .4, .55}, model.Knot{.65, .2, .25})
			for j := 0; j < 9; j++ {
				a := float64(j) * .65
				d := .9 + shape.Float64()*.35
				add(false, model.Knot{math.Cos(a) * d, math.Sin(a) * d, .07 + shape.Float64()*.12})
			}
		case "indented":
			add(false, model.Knot{-.4, -.7, .5}, model.Knot{.2, .1, .75}, model.Knot{-.1, .8, .6})
			for j := 0; j < 8; j++ {
				a := float64(j) * model.Tau / 8
				add(true, model.Knot{math.Cos(a) * 1.4, math.Sin(a) * 1.4, .12}, model.Knot{math.Cos(a+.15) * .55, math.Sin(a+.15) * .55, .025})
			}
		case "broad-shield":
			add(false, model.Knot{-.65, .4, .65}, model.Knot{.55, .45, .75}, model.Knot{.2, -.5, .65})
			add(true, model.Knot{0, 1.6, .38}, model.Knot{-.15, .65, .19})
			add(true, model.Knot{1.6, -.5, .3}, model.Knot{.55, -.3, .16})
		case "hook", "crescent":
			add(false, model.Knot{-.55, -.8, .35}, model.Knot{-.9, -.1, .6}, model.Knot{-.45, .8, .65}, model.Knot{.4, 1.1, .35}, model.Knot{1, .55, .17}, model.Knot{.65, .15, .04})
			add(true, model.Knot{.1, -1.2, .28}, model.Knot{.25, -.1, .32}, model.Knot{.25, .5, .15})
		case "ribbon":
			add(false, model.Knot{-.3, -1.5, .09}, model.Knot{-.4, -.6, .32}, model.Knot{.2, .2, .45}, model.Knot{.1, 1, .34}, model.Knot{.6, 1.7, .05})
		case "branch":
			add(false, model.Knot{0, -.7, .45}, model.Knot{-.2, .15, .65}, model.Knot{.1, 1, .28}, model.Knot{-.1, 1.5, .06})
			add(false, model.Knot{-.1, .2, .35}, model.Knot{-1, .55, .2}, model.Knot{-1.4, .2, .04})
			add(false, model.Knot{0, -.2, .35}, model.Knot{.7, -.4, .18}, model.Knot{.8, -1.1, .04})
		default:
			add(false, model.Knot{-.45, -.4, .6}, model.Knot{.2, .1, .85}, model.Knot{.45, .9, .4})
			add(false, model.Knot{-.4, .2, .4}, model.Knot{-1, .8, .24}, model.Knot{-1.2, 1.1, .05})
			add(false, model.Knot{.35, -.35, .3}, model.Knot{.8, -.9, .17}, model.Knot{1.1, -.65, .045})
			add(true, model.Knot{1.3, .35, .27}, model.Knot{.45, .35, .22}, model.Knot{.1, .1, .08})
			if family == "fractured" {
				add(true, model.Knot{-.75, -1, .11}, model.Knot{-.4, 0, .07}, model.Knot{-.5, 1.4, .08})
			}
		}
		// Asymmetric secondary lobes vary proportions within each family.
		lobes := 3
		if family == "ribbon" || family == "hook" || family == "crescent" {
			lobes = 1
		}
		for j := 0; j < lobes; j++ {
			a := shape.Float64() * model.Tau
			d := .5 + shape.Float64()*.2
			add(false, model.Knot{math.Cos(a) * d, math.Sin(a) * d, .18 + shape.Float64()*.18})
		}
		// Regional embayments cut across the assembled lobes instead of merely
		// perturbing a round coastline. Their taper protects inland connectivity.
		for j := 0; j < 3; j++ {
			a := shape.Float64() * model.Tau
			add(true, model.Knot{math.Cos(a) * 1.55, math.Sin(a) * 1.55, .14}, model.Knot{math.Cos(a+.15) * .83, math.Sin(a+.15) * .83, .045}, model.Knot{math.Cos(a+.35) * .6, math.Sin(a+.35) * .6, .012})
		}
		p.Masses = append(p.Masses, m)
	}
	// Curved, interrupted island systems, generated by their own stream.
	r = stream(p.Spec, "islands")
	groups := int(12 * p.Spec.Islands)
	if p.Spec.Preset == "archipelago" {
		groups *= 3
	}
	for i := 0; i < groups; i++ {
		c := model.Point{r.Float64()*model.Tau - math.Pi, math.Asin(r.Float64()*1.8 - .9)}
		if i%2 == 0 && len(p.Masses) > 0 {
			parent := p.Masses[i%count]
			c = model.Point{parent.Center[0] + parent.Extent*(.7+r.Float64()*.4), model.Clamp(parent.Center[1]+(r.Float64()-.5)*.7, -1.3, 1.3)}
		}
		a := r.Float64() * model.Tau
		arc := .12 + r.Float64()*.3
		n := 5 + r.IntN(10)
		for j := 0; j < n; j++ {
			if r.Float64() < .18 {
				continue
			}
			t := float64(j) / float64(n-1)
			theta := a + t*2.2
			size := (.012 + r.Float64()*.035) * (1 - t*.6)
			m := model.Mass{ID: fmt.Sprintf("arc-%d-island-%d", i, j), Family: "island-arc", Center: model.Point{model.Wrap(c[0] + math.Cos(theta)*arc), model.Clamp(c[1]+math.Sin(theta)*arc, -1.5, 1.5)}, Rotation: theta + .8, Extent: size * 2.8}
			m.Strokes = []model.Stroke{{Knots: []model.Knot{{0, -size, size}, {size * .25, size * .4, size * .6}, {0, size * 1.8, size * .05}}}}
			p.Masses = append(p.Masses, m)
		}
	}
}
