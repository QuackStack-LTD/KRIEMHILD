package terrain

import (
	"fmt"
	"math"
	"sort"
)

type ResourceDefinition struct {
	ID, Name, Category string
	Continuous         bool
}

var ResourceCatalog = []ResourceDefinition{
	{"coal", "Coal", "energy", false}, {"crude-oil", "Crude oil", "energy", false}, {"natural-gas", "Natural gas", "energy", false}, {"geothermal-energy", "Geothermal energy", "energy", true},
	{"iron-ore", "Iron ore", "metal", false}, {"copper", "Copper", "metal", false}, {"gold", "Gold", "metal", false}, {"bauxite", "Bauxite", "metal", false}, {"silver", "Silver", "metal", false}, {"lead", "Lead", "metal", false}, {"zinc", "Zinc", "metal", false}, {"tin", "Tin", "metal", false}, {"nickel", "Nickel", "metal", false}, {"cobalt", "Cobalt", "metal", false}, {"lithium", "Lithium", "metal", false}, {"chromium", "Chromium", "metal", false}, {"platinum-group-metals", "Platinum-group metals", "metal", false}, {"uranium", "Uranium", "energy", false},
	{"halite", "Mined salt", "industrial", false}, {"limestone", "Limestone", "industrial", false}, {"gypsum", "Gypsum", "industrial", false}, {"quartz-silica", "Quartz and silica", "industrial", false}, {"industrial-clay", "Industrial clay", "industrial", false}, {"phosphate-rock", "Phosphate rock", "industrial", false}, {"sulfur", "Sulfur", "industrial", false}, {"potash", "Potash", "industrial", false}, {"gemstones", "Gemstones", "industrial", false}, {"construction-stone", "Construction stone", "industrial", false},
	{"groundwater-aquifer", "Groundwater aquifer", "water", true}, {"mineral-spring", "Mineral spring", "water", false}, {"hot-spring", "Hot spring", "water", false}, {"geyser", "Geyser", "water", false}, {"hydrothermal-vent", "Hydrothermal vent", "water", false}, {"freshwater", "Freshwater", "water", true},
	{"coral", "Reef-building coral", "ecological", true}, {"timber", "Timber", "ecological", true}, {"marine-fish", "Marine fisheries", "ecological", true}, {"freshwater-fish", "Freshwater fisheries", "ecological", true}, {"wildlife", "Wildlife and game", "ecological", true}, {"fertile-soil", "Fertile soil", "ecological", true}, {"peat", "Peat", "ecological", false}, {"wind-energy", "Wind energy", "renewable", true}, {"solar-energy", "Solar energy", "renewable", true},
}

type resourceContext struct {
	e                     *Environment
	i                     int
	c                     GeologicalCell
	p                     GeologicalProvince
	bed, cover, intrusion *GeologicalFormation
	spring                *HydroSpring
	wetland               *HydroWetland
	refs                  []NaturalReference
}
type resourceCandidate struct {
	Variant, Cause string
	Score, Depth   float64
}

func (c resourceContext) f(name string) float64 { return c.e.get(name, c.i) }
func (c resourceContext) rock(rock string) bool {
	for _, f := range []*GeologicalFormation{c.bed, c.cover, c.intrusion} {
		if f == nil {
			continue
		}
		if f.Rock == rock {
			return true
		}
		for _, s := range f.Strata {
			if s.Rock == rock {
				return true
			}
		}
	}
	return false
}
func (c resourceContext) layer(rock string) *RockStratum {
	if c.cover != nil {
		for i := range c.cover.Strata {
			s := &c.cover.Strata[i]
			if s.Rock == rock {
				return s
			}
		}
	}
	return nil
}
func (c resourceContext) petroleum() bool {
	return c.cover != nil && c.layer("organic-shale") != nil && c.p.History.OrganicPreservation > .35 && c.p.History.MaxBurial > 1800 && c.p.History.Trap != "" && c.layer("sandstone") != nil && (c.layer("shale") != nil || c.layer("evaporite") != nil)
}
func (e *Environment) resourceContexts() []resourceContext {
	g := e.Geology
	out := make([]resourceContext, len(e.Mask))
	for i, c := range g.Cells {
		x := resourceContext{e: e, i: i, c: c, p: g.Provinces[c.Province], bed: &g.Formations[c.Basement]}
		x.refs = []NaturalReference{{"province", x.p.ID}, {"formation", x.bed.ID}}
		if c.Cover >= 0 {
			x.cover = &g.Formations[c.Cover]
			x.refs = append(x.refs, NaturalReference{"formation", x.cover.ID})
		}
		if c.Intrusion >= 0 {
			x.intrusion = &g.Formations[c.Intrusion]
			x.refs = append(x.refs, NaturalReference{"formation", x.intrusion.ID})
		}
		if c.Fault >= 0 {
			x.refs = append(x.refs, NaturalReference{"structure", g.Structures[c.Fault].ID})
		}
		out[i] = x
	}
	for i := range e.Hydrology.Springs {
		s := &e.Hydrology.Springs[i]
		out[s.Cell].spring = s
		out[s.Cell].refs = append(out[s.Cell].refs, NaturalReference{"spring", s.ID})
	}
	for i := range e.Hydrology.Wetlands {
		wet := &e.Hydrology.Wetlands[i]
		for _, cell := range wet.Cells {
			out[cell].wetland = wet
		}
	}
	return out
}

// Eligibility is evaluated before any seeded variation. The same predicates
// validate imported occurrences; a possible host never implies a deposit.
func resourceEligibility(id string, c resourceContext) resourceCandidate {
	f, h := c.f, c.p.History
	dry := f("waterBody") == 0
	marine := f("ocean") > 0
	fractured := c.c.Fault >= 0
	intruded := c.intrusion != nil
	granite := intruded && c.intrusion.Rock == "granite"
	mafic := c.rock("gabbro") || c.rock("peridotite")
	ultra := c.rock("peridotite")
	hydrothermal := fractured && (intruded || f("geothermal") > 65 || c.p.Setting == "ancient-orogen")
	stable := f("slope") < .1 && c.p.AgeMa > 150
	laterite := stable && h.TropicalWeathering && c.c.Erosion < .4 && dry
	result := func(v, why string, score, depth float64) resourceCandidate {
		return resourceCandidate{v, why, rounded(clamp(score, 0, 1)), rounded(depth)}
	}
	switch id {
	case "coal":
		if s := c.layer("coal-bearing-shale"); s != nil && h.AncientWetland && h.MaxBurial > 400 && h.MaxTemperature < 200 && s.Organic > .3 {
			return result("buried-coal", "Preserved ancient wetland vegetation was buried in a sedimentary basin.", s.Organic, s.Top)
		}
	case "crude-oil":
		if c.petroleum() && h.MaxTemperature >= 60 && h.MaxTemperature <= 150 {
			return result("trapped-petroleum", "Mature organic source rock, sandstone migration/reservoir pathways and a sealed geological trap form a petroleum system.", h.OrganicPreservation, h.MaxBurial*.45)
		}
	case "natural-gas":
		if c.petroleum() && h.MaxTemperature > 100 && h.MaxTemperature < 220 {
			return result("thermogenic", "Buried organic source rocks generated gas beneath a reservoir seal.", h.OrganicPreservation, h.MaxBurial*.5)
		}
		if c.layer("coal-bearing-shale") != nil && h.MaxBurial > 500 && h.MaxTemperature < 200 {
			return result("coalbed-methane", "Buried coal-bearing strata retain adsorbed methane.", .5, h.MaxBurial*.25)
		}
		if c.cover != nil && c.wetland != nil && f("temperature") > 5 && f("permeability") < .3 {
			return result("biogenic-methane", "Shallow waterlogged fine sediments preserve locally generated biogenic methane.", .25, 15)
		}
	case "geothermal-energy":
		if fractured && f("groundFlow") > .04 && f("geothermal") > 55 {
			return result("deep-circulation-potential", "Crustal heat and supplied fracture circulation provide geothermal potential; this does not imply a surface spring.", f("geothermal")/210, 1500)
		}
	case "iron-ore":
		if c.p.AgeMa > 1500 && c.rock("banded-iron") {
			return result("ancient-iron-formation", "Ancient sedimentary iron enrichment was preserved or metamorphosed.", .7, 150)
		}
		if intruded && mafic {
			return result("magmatic-magnetite", "Iron oxides segregated within a mafic intrusion.", .5, 350)
		}
	case "copper":
		if hydrothermal && (intruded || c.p.Setting == "volcanic-arc") {
			return result("hydrothermal", "Fault-controlled fluids mineralized an intrusive or volcanic-arc host.", .6, 300)
		}
		if fractured && c.layer("sandstone") != nil && c.layer("organic-shale") != nil {
			return result("sediment-hosted", "Basin fluids encountered reducing organic sediments and permeable sandstone.", .4, 500)
		}
	case "gold":
		if hydrothermal && (c.p.Setting == "ancient-orogen" || c.p.Setting == "orogenic-belt" || granite) {
			return result("primary-vein", "Hydrothermal fluids deposited gold along fractures in an orogenic or intrusive host.", .55, 200)
		}
	case "bauxite":
		if laterite && (c.rock("granite") || c.rock("gneiss") || c.rock("basalt")) && f("slope") > .005 && f("wetland") < .4 {
			return result("residual-laterite", "Long-lived warm humid weathering concentrated aluminum on a stable, drained surface.", .65, 5)
		}
	case "silver":
		if hydrothermal && (granite || c.p.Setting == "volcanic-arc" || c.rock("limestone")) {
			return result("hydrothermal-vein", "Mineralized fractures in a volcanic, intrusive or carbonate host carry silver-bearing veins.", .45, 250)
		}
	case "lead", "zinc":
		if fractured && c.layer("limestone") != nil && h.MaxBurial > 1000 {
			return result("carbonate-hosted", "Warm basin fluids mineralized fractured carbonate strata.", .6, 400)
		}
	case "tin":
		if granite && hydrothermal {
			return result("granitic-vein", "An evolved granitic intrusion and hydrothermal fractures host resistant tin minerals.", .55, 150)
		}
	case "nickel", "cobalt":
		if ultra && laterite {
			return result("lateritic", "Intense tropical weathering concentrated metals above ultramafic parent rock.", .7, 12)
		}
		if intruded && mafic {
			return result("magmatic-sulfide", "Mantle-derived mafic or ultramafic intrusion hosts sulfide segregation.", .55, 500)
		}
	case "lithium":
		if granite && fractured {
			return result("pegmatite", "Evolved granitic melt and fracture pathways formed lithium-bearing pegmatites.", .5, 180)
		}
		if h.Evaporative && c.layer("evaporite") != nil && f("terminalBasin") > 0 && f("aridity") > .5 && c.rock("basalt") {
			return result("closed-basin-brine", "Volcanic-derived solutes accumulated in an evaporative closed basin.", .6, 80)
		}
		if c.cover != nil && c.cover.Rock == "mudstone" && c.p.Setting == "volcanic-arc" {
			return result("volcanic-clay", "Volcanic material altered in a fine-grained lake basin.", .4, 30)
		}
	case "chromium", "platinum-group-metals":
		if ultra && c.p.AgeMa > 100 {
			return result("ultramafic-cumulate", "Mantle-derived ultramafic cumulates concentrated refractory metals.", .55, 500)
		}
		if intruded && c.intrusion.Rock == "gabbro" && c.p.AgeMa > 500 {
			return result("layered-mafic", "An ancient layered mafic intrusion concentrated magmatic minerals.", .4, 600)
		}
	case "uranium":
		if granite && fractured {
			return result("granitic-hydrothermal", "Uranium-bearing evolved granite supplied fracture-hosted mineralization.", .45, 200)
		}
		if c.layer("sandstone") != nil && c.layer("organic-shale") != nil && f("groundFlow") > .06 {
			return result("sandstone-redox", "Supplied groundwater moved through sandstone into reducing organic strata.", .4, 350)
		}
	case "halite", "gypsum", "potash":
		if s := c.layer("evaporite"); s != nil && h.Evaporative && (id != "potash" || h.Paleoclimate == "arid-restricted-basin" && h.MaxBurial > 1500) {
			return result("evaporite-sequence", "A restricted ancient sea or saline basin repeatedly evaporated, leaving buried salt-bearing strata.", .65, s.Top)
		}
	case "limestone":
		if s := c.layer("limestone"); s != nil && h.FormerMarine {
			return result("carbonate-platform", "Ancient shallow marine carbonate deposition formed limestone, including subsequently uplifted rock.", .85, s.Top)
		}
		if c.cover != nil && c.cover.Rock == "limestone" {
			return result("carbonate", "Carbonate accumulation formed the mapped limestone unit.", .7, 0)
		}
	case "quartz-silica":
		if fractured && (c.rock("granite") || c.rock("gneiss") || c.rock("schist")) {
			return result("quartz-veins", "Silica-rich fluids mineralized fractures in granitic or metamorphic rock.", .7, 40)
		}
		if c.layer("sandstone") != nil && c.c.Deposition > .15 {
			return result("silica-sand", "Weathering and transport concentrated quartz-rich sandstone-derived sediment.", .55, 5)
		}
	case "industrial-clay":
		if c.cover != nil && (c.cover.Rock == "mudstone" || c.cover.Rock == "alluvium") && c.c.Deposition > .1 {
			return result("depositional-clay", "Fine weathered sediment settled in a lake, floodplain, delta or marine basin.", .7, 8)
		}
		if c.rock("granite") && c.c.Weathering > .25 {
			return result("weathered-kaolin", "Chemical breakdown of feldspar-rich rock produced clay.", c.c.Weathering, 10)
		}
	case "phosphate-rock":
		if c.layer("limestone") != nil && h.FormerMarine && h.OrganicPreservation > .55 {
			return result("marine-phosphorite", "Productive former marine shelves accumulated phosphate-bearing organic sediment.", .55, 180)
		}
	case "sulfur":
		if hydrothermal && f("volcano") > .3 {
			return result("volcanic-hydrothermal", "Volcanic fluids concentrated sulfur-bearing minerals.", .6, 20)
		}
		if c.layer("evaporite") != nil && c.petroleum() {
			return result("sedimentary-sulfur", "Evaporite and organic-rich basin reactions concentrated sulfur.", .45, 300)
		}
	case "gemstones":
		if c.rock("schist") && fractured {
			return result("garnet", "Regional metamorphism and mineralized fractures formed garnet-bearing rock.", .4, 70)
		}
		if granite && fractured {
			return result("pegmatite-gems", "Late granitic fluids crystallized gemstones in pegmatitic fractures.", .4, 90)
		}
	case "construction-stone":
		if dry && f("slope") < .5 {
			rock := c.bed.Rock
			if c.cover != nil && c.cover.Strata[0].Bottom > 30 {
				rock = c.cover.Rock
			}
			if intruded {
				rock = c.intrusion.Rock
			}
			if rock != "alluvium" && rock != "evaporite" && rock != "mudstone" {
				return result(rock, "An accessible mapped rock formation supplies construction stone of this lithology.", .75, 10)
			}
		}
	case "groundwater-aquifer":
		if !marine && f("recharge") > .015 && ((c.layer("sandstone") != nil && c.layer("shale") != nil) || c.rock("alluvium") || fractured) {
			return result("recharged-aquifer", "Recharge enters permeable or fractured rock; fine strata provide confinement or a lower storage boundary.", clamp(f("groundFlow")/1.5+f("recharge"), 0, 1), 20+f("waterTableDepth"))
		}
	case "mineral-spring":
		if c.spring != nil && (fractured || c.rock("limestone") || c.rock("evaporite") || c.spring.Origin == "aquifer-contact") {
			return result(c.spring.Origin, "Supplied groundwater emerges at a geological contact or fracture; water chemistry is stored with the spring.", math.Min(1, c.spring.Discharge), 0)
		}
	case "hot-spring":
		if c.spring != nil && c.spring.Temperature >= 35 && (fractured || f("volcano") > .4) && f("geothermal") > 55 {
			return result("geothermal-spring", "Geothermal heat, a supplied aquifer and circulation pathways produce this hot spring.", math.Min(1, c.spring.Discharge), 0)
		}
	case "geyser":
		if c.spring != nil && c.spring.Temperature >= 75 && f("geothermal") > 160 && fractured && c.layer("shale") != nil && f("groundFlow") > .2 {
			return result("confined-geothermal-plumbing", "A hot supplied fracture system beneath a confining layer supports pressure cycling.", .3, 100)
		}
	case "hydrothermal-vent":
		if marine && f("waterDepth") > 100 && fractured && (c.p.Setting == "oceanic-crust") && (f("volcano") > .35 || f("tectonicStress") > .55) {
			return result("submarine-vent", "Seawater circulates through fractured, heated oceanic volcanic crust.", .6, f("waterDepth"))
		}
	case "freshwater":
		if !marine && f("salinity") < 5 {
			water := f("dryDischarge")*.08 + f("groundFlow")*.3 + f("glacierMelt")*.4
			if f("lake") > 0 {
				water += math.Min(1, f("waterDepth")/80) * .4
			}
			if water > .05 {
				return result("seasonal-water-supply", "Stored water, upstream dry-season discharge and recharge determine local availability.", water, 0)
			}
		}
	case "coral":
		if marine && f("reef") > .2 && f("waterDepth") >= 2 && f("waterDepth") <= 70 && f("temperature") >= 20 && f("temperature") <= 31 && f("clarity") >= .6 && f("salinity") >= 30 {
			return result("reef-building-coral", "This generated reef has warm, clear, sunlit marine water and suitable substrate.", f("reef"), f("waterDepth"))
		}
	case "timber":
		if dry && f("temperature") > 0 && f("temperature") < 34 && f("moisture") > .55 && f("glacier") < .1 {
			biomass := f("moisture") * clamp((f("temperature")+4)/24, 0, 1) * (1 - f("slope")) * (.4 + .6*c.c.Soil)
			if biomass > .2 {
				v := "productive-forest"
				if biomass < .55 {
					v = "sparse-woodland"
				}
				return result(v, "Climate, soil, moisture and forest biomass support timber potential.", biomass, 0)
			}
		}
	case "marine-fish":
		if marine {
			productivity := f("shelf")*.4 + math.Abs(f("current"))*.08 + f("reef")*.25 + f("coastalSediment")*.1
			if productivity > .12 {
				return result("marine-productivity", "Shelf habitat, currents, reef habitat and nutrient-bearing coastal sediment support fisheries.", productivity, 0)
			}
		}
	case "freshwater-fish":
		if !marine && f("salinity") < 5 && f("summer") > 2 && (f("lake") > 0 || f("river") > 0) && f("dryDischarge") > .08 {
			return result("freshwater-productivity", "Connected freshwater habitat and sustained seasonal flow support fish populations.", .3+math.Log1p(f("dryDischarge"))*.15, 0)
		}
	case "wildlife":
		if dry && f("summer") > 0 && f("moisture") > .25 && f("glacier") < .1 {
			return result("habitat-potential", "Vegetation, soil productivity and catchment water support connected terrestrial habitat.", f("moisture")*(.4+c.c.Soil*.4+math.Min(.2, f("groundFlow")))*(1-f("slope")*.5), 0)
		}
	case "fertile-soil":
		if dry && f("summer") > 4 && f("aridity") < .65 && f("slope") < .18 && c.c.Soil > .3 {
			return result("productive-soil", "Parent-rock weathering, organic input and sediment deposition sustain soil fertility.", c.c.Soil*(1-f("slope")*2), 0)
		}
	case "peat":
		if c.wetland != nil && f("wetland") > .65 && f("moisture") > .7 && f("temperature") < 16 && f("temperature") > -8 && f("dryDischarge") >= f("accumulation")*.3 {
			return result("waterlogged-organic", "Persistent waterlogging and cool conditions preserve vegetation faster than it decomposes.", f("wetland")*.6, 1)
		}
	case "wind-energy":
		if f("windStrength") > .35 {
			return result("renewable-potential", "Prevailing winds and terrain exposure determine this renewable energy potential.", math.Pow(f("windStrength"), 3)*(.6+f("slope")*.4), 0)
		}
	case "solar-energy":
		insolation := math.Max(0, math.Cos(f("latitude")*math.Pi/180)) * (1 - .55*clamp(f("precipitation")/2400, 0, 1))
		if insolation > .12 {
			return result("renewable-potential", "Latitude, seasonal insolation and rainfall-related cloud cover determine solar potential.", insolation, 0)
		}
	}
	return resourceCandidate{}
}

func (e *Environment) BuildNaturalResources() {
	if e.Geology == nil || e.Hydrology == nil {
		return
	}
	r := &ResourceState{Version: 1, GeologyVersion: e.Geology.Version, Units: "normalized abundance/potential 0..1; inferred occurrences, not surveyed reserves", Potential: map[string][]float64{}}
	e.Resources = r
	contexts := e.resourceContexts()
	n, w, h := len(e.Mask), e.Options.Columns, e.Options.Rows
	sample := noise(Seed(e.Options.Seed) ^ 0xb8713165)
	for k, definition := range ResourceCatalog {
		labels := make([]string, n)
		candidates := make([]resourceCandidate, n)
		var values []float64
		if definition.Continuous {
			values = make([]float64, n)
		}
		for i, c := range contexts {
			candidate := resourceEligibility(definition.ID, c)
			if candidate.Score <= .02 {
				continue
			}
			// Coherent mineralization districts, constrained by eligibility.
			// Ecological/potential fields are spatial yields, not random deposits.
			variation := sample(float64(i%w)/float64(w)*18+float64(k)*31, float64(i/w)/float64(h)*18+float64(k)*17)
			threshold := .61
			if definition.ID == "limestone" || definition.ID == "construction-stone" {
				threshold = .3
			}
			if definition.ID == "geyser" {
				threshold = .86
			}
			if !definition.Continuous && variation < threshold {
				continue
			}
			candidate.Score = rounded(candidate.Score * (.65 + .35*variation))
			candidates[i] = candidate
			if values != nil {
				values[i] = candidate.Score
			}
			labels[i] = fmt.Sprintf("%s/%d/%d/%d", candidate.Variant, c.c.Province, c.c.Cover, c.c.Intrusion)
			if c.spring != nil && (definition.ID == "mineral-spring" || definition.ID == "hot-spring" || definition.ID == "geyser") {
				labels[i] += fmt.Sprint("/point/", i)
			}
		}
		if values != nil {
			r.Potential[definition.ID] = values
		}
		for _, cells := range e.naturalRegions(labels) {
			i := cells[0]
			c := contexts[i]
			candidate := candidates[i]
			o := ResourceOccurrence{ID: e.naturalID("resource/"+definition.ID, i), Type: definition.ID, Variant: candidate.Variant, Cause: candidate.Cause, Depth: candidate.Depth, Confidence: .55, Geometry: NaturalGeometry{Kind: "region", Cells: cells}}
			refs := map[NaturalReference]bool{}
			for _, j := range cells {
				o.Abundance += candidates[j].Score / float64(len(cells))
				o.Accessibility += (1 - e.get("slope", j)) / (1 + candidate.Depth/1000) / float64(len(cells))
				for _, ref := range contexts[j].refs {
					refs[ref] = true
				}
				if contexts[j].wetland != nil {
					refs[NaturalReference{"wetland", contexts[j].wetland.ID}] = true
				}
				if e.get("waterBody", j) > 0 {
					refs[NaturalReference{"water", e.hydroID("water", int(e.get("waterBody", j)))}] = true
				}
			}
			for _, field := range []string{"temperature", "precipitation", "flow"} {
				refs[NaturalReference{"field", field}] = true
			}
			for ref := range refs {
				o.References = append(o.References, ref)
			}
			sort.Slice(o.References, func(a, b int) bool {
				if o.References[a].Kind == o.References[b].Kind {
					return o.References[a].ID < o.References[b].ID
				}
				return o.References[a].Kind < o.References[b].Kind
			})
			o.Abundance = rounded(o.Abundance)
			o.Quality = rounded(.3 + o.Abundance*.6)
			o.Accessibility = rounded(o.Accessibility)
			if definition.ID == "hydrothermal-vent" || c.spring != nil && (definition.ID == "mineral-spring" || definition.ID == "hot-spring" || definition.ID == "geyser") {
				o.Geometry = NaturalGeometry{Kind: "point", Point: &[2]float64{float64(i % w), float64(i / w)}}
			}
			if c.spring != nil && (definition.ID == "mineral-spring" || definition.ID == "hot-spring" || definition.ID == "geyser") {
				o.Properties = map[string]float64{"temperatureC": c.spring.Temperature}
				for mineral, amount := range c.spring.Minerals {
					o.Properties[mineral+"MgL"] = amount
				}
				if c.rock("limestone") || c.rock("marble") {
					o.Properties["calciumMgL"] = 80
					o.Properties["magnesiumMgL"] = 25
				}
				if c.rock("evaporite") {
					o.Properties["sodiumMgL"] = 180
					o.Properties["sulfateMgL"] = 120
				}
			}
			r.Occurrences = append(r.Occurrences, o)
		}
	}
	for _, id := range []string{"gold", "tin", "gemstones"} {
		e.buildPlacerResources(id)
	}
}

// Secondary occurrences transport actual primary mineralization along the
// stored downhill graph. The strongest contributing source is retained by ID.
func (e *Environment) buildPlacerResources(resource string) {
	n := len(e.Mask)
	mass := make([]float64, n)
	source := make([]int, n)
	for i := range source {
		source[i] = -1
	}
	for k, o := range e.Resources.Occurrences {
		if o.Type != resource || o.Variant == "placer" {
			continue
		}
		for _, i := range o.Geometry.Cells {
			mass[i] = o.Abundance * e.Geology.Cells[i].Erosion
			source[i] = k
		}
	}
	labels := make([]string, n)
	grades := make([]float64, n)
	for _, i := range e.naturalFlowOrder() {
		if source[i] < 0 {
			continue
		}
		j := int(e.get("flow", i))
		if j < 0 || e.get("ocean", i) > 0 {
			continue
		}
		if e.get("river", i) > 0 && e.get("slope", i) < .08 && e.get("catchmentArea", i) >= 4 && mass[i] > .08 {
			primary := e.Resources.Occurrences[source[i]]
			isPrimary := false
			for _, cell := range primary.Geometry.Cells {
				if cell == i {
					isPrimary = true
					break
				}
			}
			if !isPrimary {
				labels[i] = primary.ID
				grades[i] = rounded(math.Min(.85, mass[i]*.25))
			}
		}
		if mass[i]*.9 > mass[j] {
			source[j] = source[i]
		}
		mass[j] += mass[i] * .9
	}
	for _, cells := range e.naturalRegions(labels) {
		i := cells[0]
		grade := 0.
		for _, j := range cells {
			grade += grades[j] / float64(len(cells))
		}
		if grade < .02 {
			continue
		}
		e.Resources.Occurrences = append(e.Resources.Occurrences, ResourceOccurrence{ID: e.naturalID("resource/"+resource+"/placer", i), Type: resource, Variant: "placer", Cause: "Erosion of a generated upstream mineral deposit and river transport concentrated resistant minerals in lower-gradient channel sediment.", Geometry: NaturalGeometry{Kind: "region", Cells: cells}, Abundance: rounded(grade), Quality: .4, Confidence: .5, Accessibility: .8, Depth: 2, References: []NaturalReference{{"resource", labels[i]}, {"field", "flow"}, {"formation", e.Geology.Formations[e.Geology.Cells[i].Basement].ID}}})
	}
}
