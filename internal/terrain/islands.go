package terrain

import (
	"fmt"
	"math"
	"sort"
)

// These are geological foundations, including drowned members, not decorative
// map objects. Actual shorelines and water connectivity come from the surface.
type IslandFoundation struct {
	ID      string     `json:"id"`
	Center  [2]float64 `json:"center"`
	Radius  float64    `json:"radius"`
	Aspect  float64    `json:"aspect"`
	Bearing float64    `json:"bearing"`
	Summit  float64    `json:"summitMetres"`
	Age     float64    `json:"ageMa"`
	Stage   string     `json:"stage"`
}
type Archipelago struct {
	ID           string             `json:"id"`
	Process      string             `json:"process"`
	Setting      string             `json:"setting"`
	Cause        string             `json:"cause"`
	Plate        int                `json:"plate"`
	OtherPlate   int                `json:"otherPlate"`
	Source       [2]float64         `json:"source"`
	SeaLevelRise float64            `json:"relativeSeaLevelRiseMetres,omitempty"`
	Path         [][2]float64       `json:"path"`
	Foundations  []IslandFoundation `json:"foundations"`
}
type islandSite struct {
	cell, plate, other int
	process, setting   string
	score, bearing     float64
}

func (e *Environment) buildArchipelagos(hotspots [][2]float64) {
	e.Archipelagos = nil
	if e.Options.Heights != nil || e.Options.IslandFrequency <= 0 {
		return
	}
	w, h := e.Options.Columns, e.Options.Rows
	n := w * h
	scale := math.Max(.65, float64(min(w, h))/100)
	seed := Seed(e.Options.Seed) ^ 0xa874cf31
	hash := func(i, k int) float64 { return detailHash(seed, i%w, i/w, k) }
	patch := noise(seed)
	landDistance := distance(e.Mask, w, h, 1)
	original := append([]int(nil), e.Mask...)
	sites := []islandSite{}
	// Rank sites only within supported settings. Coherent productivity provinces
	// and irregular exclusion distances leave substantial empty ocean regions.
	for i := range e.Mask {
		x, y := i%w, i/w
		if original[i] != 0 || min(x, y, w-1-x, h-1-y) <= e.Margin+3 || landDistance[i] < 1.5 {
			continue
		}
		plate := int(e.get("plate", i))
		p := e.Plates[plate]
		other := -1
		nearest := math.Inf(1)
		for k, q := range e.Plates {
			if k == plate {
				continue
			}
			d := math.Hypot(float64(x)-q.X, float64(y)-q.Y)
			if d < nearest {
				nearest = d
				other = k
			}
		}
		if other < 0 {
			continue
		}
		q := e.Plates[other]
		bearing := math.Atan2(q.Y-p.Y, q.X-p.X) + math.Pi/2
		process, setting := "", "open-ocean"
		coastal := landDistance[i] < 8*scale && e.get("shelf", i) > .35
		switch {
		case e.get("boundary", i) == 1 && e.get("tectonicStress", i) > .2 && (!p.Continental || !q.Continental) && (p.Continental || !q.Continental && plate < other):
			process = "subduction-arc"
		case e.get("boundary", i) == 2 && !p.Continental:
			process = "spreading-chain"
		case coastal:
			process = "shelf-remnants"
			if landDistance[i] <= 3*scale {
				process = "continental-fragments"
			}
			// Isobath tangent keeps continental groups parallel to their coast.
			gx := landDistance[y*w+min(w-1, x+1)] - landDistance[y*w+max(0, x-1)]
			gy := landDistance[min(h-1, y+1)*w+x] - landDistance[max(0, y-1)*w+x]
			bearing = math.Atan2(gy, gx) + math.Pi/2
		case landDistance[i] > 8*scale && !p.Continental && patch(float64(x)/float64(w)*6+71, float64(y)/float64(h)*6+49) > .58:
			process = "hotspot-track"
			bearing = math.Atan2(p.VY, p.VX)
		}
		if process == "" {
			continue
		}
		if coastal {
			setting = "continental-margin"
		}
		// Several surrounding shores identify marginal/enclosed marine settings.
		shores := 0
		for k := 0; k < 8; k++ {
			a := float64(k) * math.Pi / 4
			xx := int(math.Round(float64(x) + math.Cos(a)*10*scale))
			yy := int(math.Round(float64(y) + math.Sin(a)*10*scale))
			if xx >= 0 && xx < w && yy >= 0 && yy < h && original[yy*w+xx] != 0 {
				shores++
			}
		}
		if shores >= 3 {
			setting = "marginal-sea"
		}
		score := hash(i, 17)*.55 + patch(float64(x)/float64(w)*8, float64(y)/float64(h)*8)*.45
		// Existing hotspot locations seed tracks too, rather than starting a new
		// independent random landmass system beside the tectonic model.
		if process == "hotspot-track" {
			for _, hot := range hotspots {
				if math.Hypot(float64(x)-hot[0], float64(y)-hot[1]) < 2*scale {
					score += .35
				}
			}
		}
		sites = append(sites, islandSite{i, plate, other, process, setting, score, bearing})
	}
	sort.Slice(sites, func(i, j int) bool {
		if sites[i].score == sites[j].score {
			return sites[i].cell < sites[j].cell
		}
		return sites[i].score > sites[j].score
	})
	budget := max(1, int(math.Round(float64(n)/900*e.Options.IslandFrequency)))
	coastalBudget := int(math.Round(float64(budget) * e.Options.IslandCoastalShare))
	counts := [2]int{}
	limits := [2]int{budget - coastalBudget, coastalBudget}
	landBudget := max(4, int(float64(n)*math.Min(.06, .023*e.Options.IslandFrequency)))
	added := 0
	for _, site := range sites {
		category := 0
		if site.setting != "open-ocean" {
			category = 1
		}
		if counts[category] >= limits[category] || added >= landBudget {
			continue
		}
		i := site.cell
		x, y := float64(i%w), float64(i/w)
		clear := true
		for _, g := range e.Archipelagos {
			if math.Hypot(x-g.Source[0], y-g.Source[1]) < scale*(8+hash(i, 27)*7) {
				clear = false
				break
			}
		}
		if !clear {
			continue
		}
		g := Archipelago{ID: e.naturalID("archipelago/"+site.process, i), Process: site.process, Setting: site.setting, Plate: site.plate, OtherPlate: site.other, Source: [2]float64{x, y}}
		volcanic := site.process == "subduction-arc" || site.process == "spreading-chain" || site.process == "hotspot-track"
		switch site.process {
		case "subduction-arc":
			g.Cause = "Convergence and subduction built a curved volcanic ridge on the overriding margin."
		case "spreading-chain":
			g.Cause = "Mantle-derived volcanism raised an oceanic spreading ridge above sea level in separated segments."
		case "hotspot-track":
			g.Cause = "Plate motion over a mantle plume produced successively older, eroded and subsiding volcanic edifices."
		case "continental-fragments":
			g.Cause = "Relative sea-level rise flooded low saddles between resistant continental ridges."
			g.SeaLevelRise = 60 + 120*hash(i, 31)
		case "shelf-remnants":
			g.Cause = "Erosion left resistant bedrock highs and drowned banks on a submerged continental shelf."
			g.SeaLevelRise = 30 + 70*hash(i, 31)
		}
		members := 3 + int(hash(i, 35)*4)
		if site.process == "hotspot-track" && hash(i, 36) > .7 {
			members = 2
		} // isolated island with older submerged companion
		spacing := scale * (2.5 + hash(i, 37)*2.2)
		bend := (hash(i, 38) - .5) * 1.2
		if site.process == "subduction-arc" {
			bend = .65 + hash(i, 38)*.65
		}
		for k := 0; k < members; k++ {
			u := float64(k) - float64(members-1)/2
			if site.process == "hotspot-track" {
				u = float64(k)
			}
			along := u*spacing + (hash(i, 160+k)-.5)*spacing*.65
			across := bend*u*u*scale + (hash(i, 50+k)-.5)*scale
			if site.process == "shelf-remnants" {
				across += (hash(i, 170+k) - .5) * spacing * 2.5
			}
			cx := math.Round(x + math.Cos(site.bearing)*along - math.Sin(site.bearing)*across)
			cy := math.Round(y + math.Sin(site.bearing)*along + math.Cos(site.bearing)*across)
			if cx < float64(e.Margin+2) || cy < float64(e.Margin+2) || cx > float64(w-e.Margin-3) || cy > float64(h-e.Margin-3) {
				continue
			}
			j := int(cy)*w + int(cx)
			if original[j] != 0 || landDistance[j] < 1.5 {
				continue
			}
			if (site.process == "subduction-arc" || site.process == "hotspot-track") && int(e.get("plate", j)) != site.plate {
				continue
			}
			if !volcanic && (e.get("shelf", j) < .35 || landDistance[j] > 10*scale) {
				continue
			}
			radius := scale * (.85 + hash(i, 70+k)*.75)
			if k == members/2 {
				radius *= 1.3
			}
			f := IslandFoundation{ID: fmt.Sprintf("%s/%d", g.ID, k), Center: [2]float64{cx, cy}, Radius: radius, Aspect: .65 + hash(i, 90+k)*.7, Bearing: site.bearing + (hash(i, 100+k)-.5)*.8, Summit: 60 + hash(i, 110+k)*260, Age: 10 + hash(i, 120+k)*150, Stage: "eroded-island"}
			if !volcanic {
				before := f.Summit
				f.Summit -= g.SeaLevelRise
				f.Radius *= math.Sqrt(math.Max(.2, f.Summit/before))
				if f.Summit <= 0 {
					f.Stage = "submerged-bank"
				}
			}
			if volcanic {
				f.Summit = 250 + hash(i, 110+k)*1000
				f.Age = 1 + float64(k)*3
				f.Stage = "volcanic-island"
			}
			if site.process == "hotspot-track" {
				f.Summit *= math.Exp(-float64(k) * .4)
				if k == members-1 {
					f.Stage = "subsiding-atoll"
					f.Summit = -18
				}
			}
			if k != members/2 && hash(i, 130+k) > .72 {
				f.Stage = "submerged-bank"
				f.Summit = -15 - hash(i, 140+k)*100
			}
			if e.applyIslandFoundation(f, volcanic, original, landBudget-added, patch) {
				g.Foundations = append(g.Foundations, f)
				g.Path = append(g.Path, f.Center)
			}
			// Count emerged cells after each member; never truncate a landform to
			// satisfy a pixel quota. Whole candidates exceeding the budget are skipped.
			added = 0
			for at, land := range original {
				if land == 0 && e.Mask[at] != 0 {
					added++
				}
			}
		}
		if len(g.Foundations) > 0 {
			e.Archipelagos = append(e.Archipelagos, g)
			counts[category]++
		}
	}
}

func (e *Environment) applyIslandFoundation(f IslandFoundation, volcanic bool, original []int, remaining int, patch func(float64, float64) float64) bool {
	w, h := e.Options.Columns, e.Options.Rows
	base := f.Radius*6 + 2
	type sample struct {
		i      int
		z, rim float64
	}
	changes := []sample{}
	emerged := 0
	for y := max(e.Margin+1, int(f.Center[1]-base)); y <= min(h-e.Margin-2, int(f.Center[1]+base)); y++ {
		for x := max(e.Margin+1, int(f.Center[0]-base)); x <= min(w-e.Margin-2, int(f.Center[0]+base)); x++ {
			i := y*w + x
			if original[i] != 0 {
				continue
			} // existing continental mountain systems are untouched
			dx, dy := float64(x)-f.Center[0], float64(y)-f.Center[1]
			u := (dx*math.Cos(f.Bearing) + dy*math.Sin(f.Bearing)) / f.Aspect
			v := -dx*math.Sin(f.Bearing) + dy*math.Cos(f.Bearing)
			r := math.Hypot(u, v) / f.Radius
			roughness := .18 + .1*clamp(f.Age/30, 0, 1)
			if !volcanic {
				roughness += .28
			}
			r *= 1 - roughness/2 + roughness*patch(float64(x)*.53+73, float64(y)*.53+89)
			if r > 6 {
				continue
			}
			old := e.get("elevation", i)
			z, rim := 0., 0.
			if r <= 1 {
				z = f.Summit * math.Pow(math.Max(0, 1-r*r), 1.6)
			} else {
				z = -8 - 130*smooth((r-1)/2)
			}
			if volcanic && r > 1 {
				z = -8 - 650*smooth((r-1)/2.5)
			}
			if f.Stage == "submerged-bank" {
				z = -15 - math.Abs(f.Summit) - 180*smooth(r/3)
			}
			if f.Stage == "subsiding-atoll" {
				rim = math.Exp(-math.Pow((r-1.25)/.42, 2))
				z = -100 + 86*rim
				if r > 2 {
					z -= 600 * smooth((r-2)/2)
				}
			}
			blend := 1 - smooth((r-2.5)/3.5)
			z = old*(1-blend) + math.Max(old, z)*blend
			if z <= old+.1 {
				continue
			}
			if e.Mask[i] == 0 && z >= 4 {
				// Retain an actual strait, including diagonals: island growth must not
				// bridge existing continents and change their drainage-area hierarchy.
				for yy := max(0, y-1); yy <= min(h-1, y+1); yy++ {
					for xx := max(0, x-1); xx <= min(w-1, x+1); xx++ {
						if original[yy*w+xx] != 0 {
							return false
						}
					}
				}
				emerged++
			}
			changes = append(changes, sample{i, z, rim})
		}
	}
	if emerged > remaining || len(changes) == 0 {
		return false
	}
	for _, s := range changes {
		e.set("elevation", s.i, s.z)
		if s.z >= 4 {
			e.Mask[s.i] = 1
		}
		if volcanic {
			e.set("geology", s.i, 6)
			if s.z > 0 {
				e.set("volcano", s.i, math.Max(e.get("volcano", s.i), .45+.4*math.Exp(-f.Age/12)))
			}
			e.set("seamount", s.i, math.Max(e.get("seamount", s.i), math.Max(s.rim, .2)))
		} else {
			e.set("geology", s.i, 1)
			e.set("shelf", s.i, math.Max(e.get("shelf", s.i), .6))
		}
	}
	return true
}

// Only mature, stable portions of a subsiding volcanic rim get recruitment.
// buildReefs still checks temperature, salinity, light, depth and turbidity.
func (e *Environment) archipelagoAtollRims() []bool {
	w, h := e.Options.Columns, e.Options.Rows
	out := make([]bool, w*h)
	for _, g := range e.Archipelagos {
		for _, f := range g.Foundations {
			if f.Stage != "subsiding-atoll" || f.Age < 4 {
				continue
			}
			x, y := int(f.Center[0]), int(f.Center[1])
			if detailHash(Seed(e.Options.Seed), x, y, 361) < .35 {
				continue
			}
			r := int(math.Ceil(f.Radius * 3))
			for yy := max(0, y-r); yy <= min(h-1, y+r); yy++ {
				for xx := max(0, x-r); xx <= min(w-1, x+r); xx++ {
					i := yy*w + xx
					if e.get("seamount", i) > .55 && math.Hypot(float64(xx-x), float64(yy-y)) < f.Radius*2.5 {
						out[i] = true
					}
				}
			}
		}
	}
	return out
}

// Carry volcanic origin into the subsequent lithology stage. A hotspot shield
// is basaltic plume crust, not a subduction arc simply because it is volcanic.
func (e *Environment) islandProvinceSettings() []string {
	out := make([]string, e.Options.Columns*e.Options.Rows)
	for i, source := range e.islandVolcanoSources() {
		switch source.Origin {
		case "hotspot":
			out[i] = "hotspot-province"
		case "rift":
			out[i] = "rift"
		case "subduction":
			out[i] = "volcanic-arc"
		}
	}
	return out
}

type islandVolcanoSource struct {
	Group, Origin string
	Age           float64
}

func (e *Environment) islandVolcanoSources() []islandVolcanoSource {
	w, h := e.Options.Columns, e.Options.Rows
	out := make([]islandVolcanoSource, w*h)
	for _, g := range e.Archipelagos {
		origin := ""
		switch g.Process {
		case "hotspot-track":
			origin = "hotspot"
		case "subduction-arc":
			origin = "subduction"
		case "spreading-chain":
			origin = "rift"
		}
		if origin == "" {
			continue
		}
		for _, f := range g.Foundations {
			x, y := int(f.Center[0]), int(f.Center[1])
			r := int(math.Ceil(f.Radius * 1.5))
			for yy := max(0, y-r); yy <= min(h-1, y+r); yy++ {
				for xx := max(0, x-r); xx <= min(w-1, x+r); xx++ {
					i := yy*w + xx
					if e.get("geology", i) == 6 && math.Hypot(float64(xx-x), float64(yy-y)) <= f.Radius*1.5 {
						out[i] = islandVolcanoSource{g.ID, origin, f.Age}
					}
				}
			}
		}
	}
	return out
}

// Optional for legacy worlds; imported foundations are loaded, never replanted.
func (e *Environment) ValidateArchipelagos() error {
	w, h := e.Options.Columns, e.Options.Rows
	if len(e.Archipelagos) > w*h/10 {
		return fmt.Errorf("too many archipelagos")
	}
	ids := map[string]bool{}
	point := func(p [2]float64) bool {
		return finite(p[0]) && finite(p[1]) && p[0] >= 0 && p[1] >= 0 && p[0] <= float64(w-1) && p[1] <= float64(h-1)
	}
	register := func(id string) bool {
		if id == "" || ids[id] {
			return false
		}
		ids[id] = true
		return true
	}
	for _, g := range e.Archipelagos {
		if !register(g.ID) || !point(g.Source) || g.Plate < 0 || g.Plate >= len(e.Plates) || g.OtherPlate < 0 || g.OtherPlate >= len(e.Plates) || g.OtherPlate == g.Plate || len(g.Foundations) == 0 || len(g.Foundations) > 8 || len(g.Path) != len(g.Foundations) || !finite(g.SeaLevelRise) || g.SeaLevelRise < 0 || g.SeaLevelRise > 500 {
			return fmt.Errorf("invalid archipelago %s", g.ID)
		}
		switch g.Setting {
		case "continental-margin", "marginal-sea", "open-ocean":
		default:
			return fmt.Errorf("invalid island setting %s", g.ID)
		}
		switch g.Process {
		case "subduction-arc", "spreading-chain", "hotspot-track", "continental-fragments", "shelf-remnants":
		default:
			return fmt.Errorf("invalid island process %s", g.ID)
		}
		if g.Process == "subduction-arc" {
			p, q := e.Plates[g.Plate], e.Plates[g.OtherPlate]
			convergence := (p.VX-q.VX)*(q.X-p.X) + (p.VY-q.VY)*(q.Y-p.Y)
			if convergence <= 0 || p.Continental && q.Continental || !p.Continental && (q.Continental || g.Plate > g.OtherPlate) {
				return fmt.Errorf("unsupported subduction arc %s", g.ID)
			}
		}
		if g.Process == "spreading-chain" {
			p, q := e.Plates[g.Plate], e.Plates[g.OtherPlate]
			if p.Continental || (p.VX-q.VX)*(q.X-p.X)+(p.VY-q.VY)*(q.Y-p.Y) >= 0 {
				return fmt.Errorf("unsupported spreading chain %s", g.ID)
			}
		}
		for k, f := range g.Foundations {
			if !register(f.ID) || !point(f.Center) || g.Path[k] != f.Center || !finite(f.Radius) || f.Radius <= 0 || f.Radius > 20 || !finite(f.Aspect) || f.Aspect < .3 || f.Aspect > 2 || !finite(f.Bearing) || !finite(f.Summit) || f.Summit < -1000 || f.Summit > 4000 || !finite(f.Age) || f.Age < 0 || f.Age > 4600 {
				return fmt.Errorf("invalid island foundation %s", f.ID)
			}
			switch f.Stage {
			case "volcanic-island", "eroded-island", "submerged-bank", "subsiding-atoll":
			default:
				return fmt.Errorf("invalid island stage %s", f.ID)
			}
		}
	}
	if e.Entities != nil {
		sources := e.islandVolcanoSources()
		for _, v := range e.Entities.Volcanoes {
			if v.Archipelago != "" && (v.Cell < 0 || v.Cell >= len(sources) || sources[v.Cell].Group != v.Archipelago || sources[v.Cell].Origin != v.Origin) {
				return fmt.Errorf("unsupported island volcano %d", v.ID)
			}
		}
	}
	return nil
}
