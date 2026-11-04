package terrain

import (
	"fmt"
	"math"
	"sort"
)

func resourceSystemKind(id, variant string) string {
	if variant == "placer" {
		return "erosion-transport-placer"
	}
	if variant == "sediment-hosted" || variant == "carbonate-hosted" || variant == "sandstone-redox" {
		return "basin-brine-redox"
	}
	if variant == "closed-basin-brine" {
		return "restricted-evaporite-basin"
	}
	if variant == "lateritic" || variant == "residual-laterite" {
		return "preserved-residual-weathering"
	}
	switch id {
	case "crude-oil", "natural-gas":
		if variant == "biogenic-methane" {
			return "waterlogged-organic-methane"
		}
		if variant == "coalbed-methane" {
			return "organic-coal-basin"
		}
		return "petroleum-source-reservoir-seal-trap"
	case "coal":
		return "organic-coal-basin"
	case "halite", "potash", "gypsum":
		return "restricted-evaporite-basin"
	case "copper", "gold", "silver", "molybdenum", "sulfur", "amethyst":
		return "magmatic-hydrothermal"
	case "lead", "zinc":
		return "basin-brine-carbonate"
	case "nickel", "cobalt", "chromium", "platinum-group-metals":
		return "mafic-ultramafic"
	case "tin", "tungsten", "lithium", "uranium":
		return "evolved-continental-mineralization"
	case "diamonds":
		return "cratonic-mantle-transport"
	case "rare-earth-elements":
		return "specialized-alkaline-carbonatite"
	case "emerald", "ruby-sapphire", "garnet", "gemstones":
		return "metamorphic-contact-gemstones"
	case "geothermal-energy", "hot-spring", "geyser", "mineral-spring", "groundwater-aquifer":
		return "groundwater-rock-circulation"
	case "iron-ore", "manganese", "phosphate-rock":
		return "sedimentary-magmatic-enrichment"
	default:
		return "surface-weathering-" + id
	}
}

// Adds a hierarchy and independently dimensioned endowment to already eligible
// occurrences. No resource is inserted here, and no geological predicate bypassed.
func (e *Environment) describeResourceSystems() {
	r := e.Resources
	r.Version = 2
	r.Source = e.NaturalSource
	r.Generator = "continuous-resources/1"
	r.Units = "procedural regional endowment, not surveyed/economic reserves; extent km2; quantity and grade carry separate units; groundwater quantities are recharge proxies, not pump yield"
	byKey := map[string]int{}
	systemCells := map[string]map[int]bool{}
	occSystems := map[string][]int{}
	for k := range r.Occurrences {
		o := &r.Occurrences[k]
		cells := o.Geometry.Cells
		if o.Geometry.Point != nil {
			cells = []int{int(math.Round(o.Geometry.Point[1]))*e.Options.Columns + int(math.Round(o.Geometry.Point[0]))}
		}
		if len(cells) == 0 {
			continue
		}
		c := e.Geology.Cells[cells[0]]
		p := e.Geology.Provinces[c.Province]
		kind := resourceSystemKind(o.Type, o.Variant)
		key := fmt.Sprintf("%d/%d/%d/%s", c.Province, c.Cover, c.Intrusion, kind)
		idx, ok := byKey[key]
		if !ok {
			idx = len(r.Systems)
			byKey[key] = idx
			host := []string{e.Geology.Formations[c.Basement].ID}
			if c.Cover >= 0 {
				host = append(host, e.Geology.Formations[c.Cover].ID)
			}
			if c.Intrusion >= 0 {
				host = append(host, e.Geology.Formations[c.Intrusion].ID)
			}
			r.Systems = append(r.Systems, ResourceSystem{ID: e.naturalID("system", idx), Kind: kind, ProvinceID: p.ID, FormationIDs: host, Commodities: []string{}, ReasonCodes: []string{"procedural-geological-history", "eligible-host-formation", kind}})
		}
		sys := &r.Systems[idx]
		found := false
		for _, id := range sys.Commodities {
			found = found || id == o.Type
		}
		if !found {
			sys.Commodities = append(sys.Commodities, o.Type)
		}
		o.SystemID = sys.ID
		o.Provenance = "procedural/inferred"
		o.ReasonCodes = append([]string{}, sys.ReasonCodes...)
		o.ReasonCodes = append(o.ReasonCodes, "setting:"+p.Setting, "process:"+o.Variant)
		if c.Intrusion >= 0 {
			o.ReasonCodes = append(o.ReasonCodes, "recorded-intrusion")
		}
		if c.Fault >= 0 {
			o.ReasonCodes = append(o.ReasonCodes, "mapped-fracture-pathway")
		}
		switch kind {
		case "petroleum-source-reservoir-seal-trap":
			o.ReasonCodes = append(o.ReasonCodes, "organic-source-rock", "permeable-reservoir", "overlying-seal", "preserved-trap", "migration-pathway", "thermal:"+p.History.Maturity)
		case "organic-coal-basin":
			o.ReasonCodes = append(o.ReasonCodes, "ancient-peat-environment", "organic-preservation", "sedimentary-burial", "coalification")
		case "restricted-evaporite-basin":
			o.ReasonCodes = append(o.ReasonCodes, "restricted-basin-history", "high-evaporation-history", "evaporite-strata")
		case "cratonic-mantle-transport":
			o.ReasonCodes = append(o.ReasonCodes, "ancient-stable-crust", "kimberlite-mantle-transport")
		case "specialized-alkaline-carbonatite":
			o.ReasonCodes = append(o.ReasonCodes, "carbonatite-intrusion")
		case "magmatic-hydrothermal":
			o.ReasonCodes = append(o.ReasonCodes, "hydrothermal-host-and-fluid-system")
		case "mafic-ultramafic":
			o.ReasonCodes = append(o.ReasonCodes, "mantle-derived-mafic-ultramafic-host")
		case "preserved-residual-weathering":
			o.ReasonCodes = append(o.ReasonCodes, "suitable-parent-rock", "past-warm-wet-weathering", "low-relief-preservation")
		case "groundwater-rock-circulation":
			o.ReasonCodes = append(o.ReasonCodes, "infiltration-recharge", "permeable-or-fractured-host")
			if o.Type == "geothermal-energy" || o.Type == "hot-spring" || o.Type == "geyser" {
				o.ReasonCodes = append(o.ReasonCodes, "elevated-heat-flow")
			}
		case "erosion-transport-placer":
			o.ReasonCodes = append(o.ReasonCodes, "recorded-upstream-primary", "saved-downstream-receivers", "sediment-concentration")
		}
		if systemCells[sys.ID] == nil {
			systemCells[sys.ID] = map[int]bool{}
		}
		for _, i := range cells {
			systemCells[sys.ID][i] = true
		}
		occSystems[sys.ID] = append(occSystems[sys.ID], k)
		for _, i := range cells {
			if e.Geography != nil {
				o.ExtentKM2 += e.GeographicCellAreaKM2(i)
			}
		}
		u := detailHash(Seed(e.Options.Seed), cells[0], int(Seed(o.ID)%10000), 719)
		heavy := math.Min(35, math.Pow(math.Max(.004, 1-u), -1/1.45))
		preservation := clamp(1-e.Geology.Cells[cells[0]].Erosion*.65, .1, 1)
		mineralized := o.ExtentKM2 * (.00001 + .0003*o.Abundance)
		o.Quantity = mineralized * 1e6 * (4 + math.Min(60, o.Depth*.025)) * 2.65 * heavy * preservation
		o.QuantityUnit = "tonnes host material"
		o.Grade = o.Quality
		o.GradeUnit = "concentration index (0–1)"
		indexed := false
		switch o.Type {
		case "solar-energy", "wind-energy", "geothermal-energy", "mineral-spring", "hot-spring", "geyser", "hydrothermal-vent", "freshwater", "timber", "marine-fish", "freshwater-fish", "wildlife", "coral", "fertile-soil", "peat":
			// Ecological/renewable/water potential is not tonnes of ore or a
			// calibrated extraction yield. Keep a dimensioned potential measure.
			o.Quantity = o.ExtentKM2 * o.Abundance
			o.QuantityUnit = "potential-index km2"
			o.Grade = o.Abundance
			o.GradeUnit = "suitability index (0–1)"
			indexed = true
		case "gold":
			o.Grade = .2 + 12*u
			o.GradeUnit = "g/t"
		case "silver":
			o.Grade = 5 + 180*u
			o.GradeUnit = "g/t"
		case "copper", "lead", "zinc", "nickel", "tin", "chromium", "manganese", "bauxite", "iron-ore":
			o.Grade = .1 + 4*u
			if o.Type == "iron-ore" || o.Type == "bauxite" {
				o.Grade = 25 + 40*u
			}
			o.GradeUnit = "mass %"
		case "diamonds":
			o.Grade = .01 + u*.8
			o.GradeUnit = "carats/t"
		case "crude-oil":
			o.Quantity = mineralized * 1e6 * 12 * heavy * preservation
			o.QuantityUnit = "m3 generated oil"
		case "natural-gas":
			o.Quantity = mineralized * 1e6 * 900 * heavy * preservation
			o.QuantityUnit = "standard m3 generated gas"
		case "groundwater-aquifer":
			o.Quantity = 0
			for _, i := range cells {
				o.Quantity += e.Fields["recharge"][i] * e.GeographicCellAreaKM2(i) * 1e6
			}
			o.QuantityUnit = "m3/year recharge proxy"
		}
		if o.Variant != "placer" {
			o.Depth = math.Max(0, o.Depth-e.Geology.Cells[cells[0]].Erosion*180)
		}
		o.Availability = "buried"
		if o.Depth < 30 {
			o.Availability = "near-surface"
		}
		if o.Depth > 1000 {
			o.Availability = "deep-buried"
		}
		if e.Fields["ocean"][cells[0]] > 0 {
			o.Availability = "submarine"
		}
		if indexed {
			o.Quantity = rounded(o.Quantity)
		} else {
			o.Quantity = math.Round(o.Quantity)
		}
		o.ExtentKM2 = rounded(o.ExtentKM2)
		o.Grade = rounded(o.Grade)
	}
	for si := range r.Systems {
		sys := &r.Systems[si]
		sort.Strings(sys.Commodities)
		labels := make([]string, len(e.Mask))
		for i := range systemCells[sys.ID] {
			labels[i] = sys.ID
		}
		for _, cells := range e.naturalRegions(labels) {
			district := ResourceDistrict{ID: e.naturalID("district", len(r.Districts)), SystemID: sys.ID, Geometry: NaturalGeometry{Kind: "region", Cells: cells}, Occurrences: []string{}}
			membership := map[int]bool{}
			for _, i := range cells {
				membership[i] = true
			}
			for _, k := range occSystems[sys.ID] {
				o := &r.Occurrences[k]
				i := -1
				if len(o.Geometry.Cells) > 0 {
					i = o.Geometry.Cells[0]
				} else if o.Geometry.Point != nil {
					i = int(math.Round(o.Geometry.Point[1]))*e.Options.Columns + int(math.Round(o.Geometry.Point[0]))
				}
				if membership[i] {
					o.DistrictID = district.ID
					district.Occurrences = append(district.Occurrences, o.ID)
				}
			}
			r.Districts = append(r.Districts, district)
		}
	}
}
