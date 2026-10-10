package terrain

import (
	"fmt"
	"math"
)

func (e *Environment) naturalRegions(labels []string) [][]int {
	seen := make([]bool, len(labels))
	regions := [][]int{}
	for i, label := range labels {
		if label == "" || seen[i] {
			continue
		}
		q := []int{i}
		seen[i] = true
		for head := 0; head < len(q); head++ {
			for _, j := range e.neighbors(q[head]) {
				if !seen[j] && labels[j] == label {
					seen[j] = true
					q = append(q, j)
				}
			}
		}
		regions = append(regions, q)
	}
	return regions
}

// A historical inference stage. It never changes today's heights, water or
// climate, nor overwrites the preliminary permeability used by hydrology.
func (e *Environment) BuildGeologicalHistory() {
	if e.Hydrology == nil {
		return
	}
	n, w, h := len(e.Mask), e.Options.Columns, e.Options.Rows
	g := &GeologicalState{Version: 1, Coordinates: "parent-grid x east, y south; ages in Ma before present; depth in metres below surface", Cells: make([]GeologicalCell, n)}
	e.Geology = g
	defer e.attachFluvialHistory()
	e.Resources = nil
	f := e.get
	sample := noise(Seed(e.Options.Seed) ^ 0x6417ca21)
	labels := make([]string, n)
	settings := make([]string, n)
	islandSettings := e.islandProvinceSettings()
	for i := range labels {
		x, y := float64(i%w)/float64(w), float64(i/w)/float64(h)
		setting := "stable-interior"
		switch {
		case f("ocean", i) > 0:
			setting = "oceanic-crust"
		case f("volcano", i) > .4:
			setting = "volcanic-arc"
		case (f("boundary", i) == 2 || f("boundary", i) == 3) && f("tectonicStress", i) > .22:
			setting = "rift"
		case f("mountainCore", i) > .16:
			setting = "orogenic-belt"
		case sample(x*5+13, y*5+39) > .6:
			setting = "ancient-orogen"
		}
		if f("ocean", i) == 0 && islandSettings[i] != "" {
			setting = islandSettings[i]
		}
		settings[i] = setting
		labels[i] = fmt.Sprintf("%d/%s", int(f("plate", i)), setting)
		g.Cells[i] = GeologicalCell{Province: -1, Basement: -1, Cover: -1, Intrusion: -1, Fault: -1}
	}
	for _, cells := range e.naturalRegions(labels) {
		i := cells[0]
		setting := settings[i]
		r := detailHash(Seed(e.Options.Seed), i%w, i/w, 111)
		age, rock, process := 900+2400*r, "granite", "crustal crystallization and stabilization"
		switch setting {
		case "oceanic-crust":
			age, rock, process = 5+175*r, "basalt", "oceanic spreading and submarine volcanism"
		case "hotspot-province":
			age, rock, process = 2+70*r, "basalt", "mantle-plume volcanism, plate motion and subsequent subsidence"
		case "volcanic-arc":
			age, rock, process = 2+110*r, "basalt", "arc magmatism"
		case "rift":
			age, rock, process = 30+240*r, "gabbro", "extension and mantle-derived magmatism"
		case "orogenic-belt":
			age, rock, process = 90+650*r, "schist", "compression and regional metamorphism"
			if r < .3 {
				rock = "slate"
			}
		case "ancient-orogen":
			age, rock, process = 600+1800*r, "gneiss", "ancient orogeny followed by erosion"
		}
		if (setting == "rift" || setting == "oceanic-crust") && r > .68 {
			rock = "peridotite"
		}
		paleo := detailHash(Seed(e.Options.Seed), i%w, i/w, 127)
		marine := setting == "oceanic-crust" || f("shelf", i) > .25 || paleo > .35
		wetland := setting != "oceanic-crust" && paleo > .28 && paleo < .65
		evap := marine && paleo > .74
		burial := 400 + 6200*detailHash(Seed(e.Options.Seed), i%w, i/w, 131)
		history := GeologicalHistory{FormerMarine: marine, AncientWetland: wetland, Evaporative: evap, TropicalWeathering: age > 150 && paleo < .5, MaxBurial: rounded(burial), MaxTemperature: rounded(18 + burial*.027), Paleoclimate: "temperate"}
		if history.TropicalWeathering {
			history.Paleoclimate = "warm-humid"
		}
		if evap {
			history.Paleoclimate = "arid-restricted-basin"
		}
		if marine || wetland {
			history.OrganicPreservation = rounded(.2 + .7*r)
		}
		if setting == "orogenic-belt" || setting == "ancient-orogen" {
			history.Trap = "fold closure"
		} else if evap {
			history.Trap = "salt-related closure"
		} else if paleo > .3 {
			history.Trap = "stratigraphic pinch-out"
		}
		p := GeologicalProvince{ID: e.naturalID("province", i), Setting: setting, AgeMa: rounded(age), Rock: rock, Process: process, Geometry: NaturalGeometry{Kind: "region", Cells: cells}, History: history}
		depositionAge := age * .6
		if wetland {
			depositionAge = math.Min(depositionAge, 320)
		}
		p.Events = []GeologicalEvent{{"formation", rounded(age), setting}, {"deposition", rounded(depositionAge), func() string {
			if marine {
				return "shallow-marine"
			}
			return "continental-basin"
		}()}, {"burial", rounded(depositionAge * .58), history.Paleoclimate}}
		if setting == "orogenic-belt" || setting == "ancient-orogen" {
			p.Events = append(p.Events, GeologicalEvent{"deformation", rounded(depositionAge * .33), "compressional"})
		}
		p.Events = append(p.Events, GeologicalEvent{"uplift", rounded(depositionAge * .13), "exhumation"}, GeologicalEvent{"erosion", 0, "present-day surface"})
		pi := len(g.Provinces)
		g.Provinces = append(g.Provinces, p)
		b := GeologicalFormation{ID: e.naturalID("basement", i), ProvinceID: p.ID, Kind: "basement", Rock: rock, AgeMa: p.AgeMa, Process: process, Geometry: p.Geometry}
		b.Strata = []RockStratum{{ID: b.ID + "/rock", Rock: rock, AgeMa: p.AgeMa, Top: 0, Bottom: 12000, Environment: setting, Permeability: .08}}
		if age > 1500 && (rock == "gneiss" || rock == "schist") && marine {
			b.Strata[0].Bottom = 500
			b.Strata = append(b.Strata, RockStratum{ID: b.ID + "/iron-formation", Rock: "banded-iron", AgeMa: p.AgeMa, Top: 500, Bottom: 650, Environment: "ancient-iron-rich-marine", Permeability: .04})
		}
		bi := len(g.Formations)
		g.Formations = append(g.Formations, b)
		for _, j := range cells {
			g.Cells[j].Province = pi
			g.Cells[j].Basement = bi
		}
	}
	// Cover and intrusive bodies are independent overlapping regions. A
	// sedimentary mountain and a plain over granite are both representable.
	for pass := 0; pass < 2; pass++ {
		for i := range labels {
			labels[i] = ""
			c := g.Cells[i]
			p := g.Provinces[c.Province]
			x, y := float64(i%w)/float64(w), float64(i/w)/float64(h)
			r := sample(x*9+51, y*9+87)
			if pass == 0 {
				env := ""
				switch {
				case f("lake", i) > 0:
					env = "lacustrine"
				case f("ocean", i) > 0:
					if f("shelf", i) > .25 {
						env = "shallow-marine"
					} else {
						env = "deep-marine"
					}
				case f("coastType", i) == 4:
					env = "delta"
				case f("accumulation", i) > 3 && f("slope", i) < .08:
					env = "alluvial"
				case f("dune", i) > .15:
					env = "aeolian"
				case p.History.FormerMarine && r > .38:
					env = "former-shallow-sea"
				case f("slope", i) < .06 && r > .3:
					env = "continental-basin"
				}
				if env != "" {
					labels[i] = fmt.Sprintf("%d/%s", c.Province, env)
				}
			} else if r > .63 && (p.Setting != "stable-interior" || p.AgeMa > 1200) {
				labels[i] = fmt.Sprintf("%d/intrusion", c.Province)
			}
		}
		for _, cells := range e.naturalRegions(labels) {
			i := cells[0]
			p := g.Provinces[g.Cells[i].Province]
			kind, rock, process := "sedimentary-basin", "sandstone", "sediment deposition, burial and lithification"
			age := p.AgeMa * .6
			if p.History.AncientWetland {
				age = math.Min(age, 320)
			}
			env := labels[i][len(fmt.Sprint(g.Cells[i].Province))+1:]
			if pass == 1 {
				kind = "intrusion"
				rock = "granite"
				process = "intrusive crystallization and contact heating"
				age = p.AgeMa * .25
				if p.Setting == "oceanic-crust" || p.Setting == "rift" || p.Setting == "hotspot-province" {
					rock = "gabbro"
					if detailHash(Seed(e.Options.Seed), i%w, i/w, 149) > .65 {
						rock = "peridotite"
					}
				}
			} else {
				switch env {
				case "lacustrine", "deep-marine":
					rock = "mudstone"
				case "alluvial", "delta":
					rock = "alluvium"
				case "shallow-marine", "former-shallow-sea":
					rock = "limestone"
				}
				if p.History.Evaporative {
					rock = "evaporite"
				} else if rock == "limestone" && p.Setting == "orogenic-belt" && f("tectonicStress", i) > .4 {
					rock = "marble"
					process = "marine carbonate deposition followed by compressional metamorphism"
				}
			}
			fm := GeologicalFormation{ID: e.naturalID(kind, i), ProvinceID: p.ID, Kind: kind, Rock: rock, AgeMa: rounded(age), Process: process, Geometry: NaturalGeometry{Kind: "region", Cells: cells}, References: []string{g.Formations[g.Cells[i].Basement].ID}}
			add := func(rock, environment string, thickness, perm, organic, relativeAge float64) {
				top := 0.
				if len(fm.Strata) > 0 {
					top = fm.Strata[len(fm.Strata)-1].Bottom
				}
				fm.Strata = append(fm.Strata, RockStratum{ID: fmt.Sprintf("%s/stratum/%d", fm.ID, len(fm.Strata)), Rock: rock, AgeMa: rounded(relativeAge), Top: top, Bottom: rounded(top + thickness), Environment: environment, Permeability: perm, Organic: organic})
			}
			if pass == 1 {
				add(rock, "intrusive", 3000, .08, 0, age)
				if rock == "peridotite" {
					add("chromitite", "ultramafic-cumulate", 150, .02, 0, age)
				}
				if rock == "gabbro" {
					add("cumulate-gabbro", "layered-mafic-intrusion", 500, .03, 0, age)
				}
			} else {
				// Top-down strata have increasing ages; seals sit above reservoirs
				// and organic source rocks. Coal and salts need recorded history.
				add(rock, env, 20+f("sediment", i)*100, .45, 0, age*.1)
				if p.History.Evaporative {
					add("evaporite", "restricted-evaporative", 200, .01, 0, age*.3)
				} else {
					add("shale", env, 150, .03, 0, age*.3)
				}
				add("sandstone", env, 450, .7, 0, age*.5)
				if p.History.AncientWetland {
					add("coal-bearing-shale", "ancient-wetland", 80, .08, p.History.OrganicPreservation, age*.7)
				}
				if p.History.FormerMarine {
					add("limestone", "former-shallow-sea", 600, .5, 0, age*.8)
					add("organic-shale", "anoxic-marine", 200, .02, p.History.OrganicPreservation, age)
				}
			}
			fi := len(g.Formations)
			g.Formations = append(g.Formations, fm)
			for _, j := range cells {
				if pass == 0 {
					g.Cells[j].Cover = fi
				} else {
					g.Cells[j].Intrusion = fi
				}
			}
		}
	}
	// Fault traces follow connected stressed crust, not independently placed
	// line segments. Their one-cell fracture halos intersect all strata.
	seen := make([]bool, n)
	for i := range seen {
		if seen[i] || f("tectonicStress", i) < .38 {
			continue
		}
		path := []int{i}
		seen[i] = true
		for len(path) < 64 {
			best := -1
			score := 0.
			for _, j := range e.neighbors(path[len(path)-1]) {
				if !seen[j] && f("tectonicStress", j) > .3 && f("tectonicStress", j) > score {
					best = j
					score = f("tectonicStress", j)
				}
			}
			if best < 0 {
				break
			}
			seen[best] = true
			path = append(path, best)
		}
		if len(path) < 3 {
			continue
		}
		p := g.Provinces[g.Cells[i].Province]
		s := GeologicalStructure{ID: e.naturalID("fault", i), Kind: "fault", AgeMa: rounded(p.AgeMa * .2), References: []string{p.ID}, Geometry: NaturalGeometry{Kind: "line"}}
		for _, j := range path {
			s.Geometry.Path = append(s.Geometry.Path, [2]float64{float64(j % w), float64(j / w)})
			for _, k := range append(e.neighbors(j), j) {
				g.Cells[k].Fault = len(g.Structures)
			}
		}
		g.Structures = append(g.Structures, s)
	}
	// Local weathering feeds a downstream sediment transport pass. All fields
	// remain separate from the terrain/hydrology inputs they were derived from.
	flux := make([]float64, n)
	for i := range g.Cells {
		c := &g.Cells[i]
		warm := clamp((f("temperature", i)+5)/35, 0, 1)
		wet := clamp(f("precipitation", i)/1600, 0, 1)
		c.Weathering = rounded(warm * wet * (.35 + .65*f("permeability", i)))
		c.Erosion = rounded(clamp(f("slope", i)*1.6+math.Log1p(f("accumulation", i))*.06, 0, 1))
		flux[i] = c.Weathering * c.Erosion
	}
	for _, i := range e.naturalFlowOrder() {
		c := &g.Cells[i]
		c.Deposition = rounded(clamp(flux[i]*(1-smooth(f("slope", i)/.2)), 0, 1))
		if j := int(f("flow", i)); j >= 0 {
			flux[j] += flux[i] * (1 - c.Deposition*.35)
		}
		c.Soil = rounded(clamp(c.Weathering*.45+c.Deposition*.35+f("moisture", i)*.2, 0, 1))
	}
}

// Read-only topological order: generation of geology/resources must never
// invoke accumulateWater and alter an already established hydrological graph.
func (e *Environment) naturalFlowOrder() []int {
	n := len(e.Mask)
	degree := make([]int, n)
	order := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if j := int(e.get("flow", i)); j >= 0 && j < n {
			degree[j]++
		}
	}
	for i, d := range degree {
		if d == 0 {
			order = append(order, i)
		}
	}
	for head := 0; head < len(order); head++ {
		j := int(e.get("flow", order[head]))
		if j >= 0 && j < n {
			degree[j]--
			if degree[j] == 0 {
				order = append(order, j)
			}
		}
	}
	return order
}
