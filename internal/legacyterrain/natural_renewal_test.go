package terrain

import (
	"strings"
	"testing"
)

// Independently specified causal fixtures. Positive hosts and each missing
// prerequisite are asserted directly, rather than relying on a lucky seed.
func TestRenewedResourceCausalRequirements(t *testing.T) {
	e := &Environment{NaturalSource: strings.Repeat("a", 64), Fields: map[string][]float64{}}
	for _, k := range []string{"waterBody", "ocean", "slope", "geothermal", "groundFlow", "temperature", "aridity", "wetland", "terminalBasin", "glacier", "summer", "permeability"} {
		e.Fields[k] = []float64{0}
	}
	e.Fields["geothermal"][0] = 90
	e.Fields["groundFlow"][0] = .2
	history := GeologicalHistory{AncientWetland: true, FormerMarine: true, Evaporative: true, OrganicPreservation: .8, MaxBurial: 3000, MaxTemperature: 120, Trap: "fold closure", Migration: true, Preservation: .8, BurialDurationMa: 100, Paleoclimate: "arid-restricted-basin"}
	base := resourceContext{e: e, p: GeologicalProvince{Type: "ancient-craton", AgeMa: 2500, History: history}, bed: &GeologicalFormation{Rock: "gneiss"}, cover: &GeologicalFormation{Strata: []RockStratum{{Rock: "coal-bearing-shale", Organic: .7, Top: 100}, {Rock: "organic-shale"}, {Rock: "sandstone"}, {Rock: "shale"}, {Rock: "evaporite"}}}, intrusion: &GeologicalFormation{Rock: "granite"}, c: GeologicalCell{Fault: 0}}
	checks := []struct {
		name, id string
		remove   func(*resourceContext)
	}{
		{"coal needs past peat", "coal", func(c *resourceContext) { c.p.History.AncientWetland = false }},
		{"coal needs burial", "coal", func(c *resourceContext) { c.p.History.MaxBurial = 0 }},
		{"oil needs trap", "crude-oil", func(c *resourceContext) { c.p.History.Trap = "" }},
		{"oil needs migration", "crude-oil", func(c *resourceContext) { c.p.History.Migration = false }},
		{"oil needs time", "crude-oil", func(c *resourceContext) { c.p.History.BurialDurationMa = 0 }},
		{"oil needs preservation", "crude-oil", func(c *resourceContext) { c.p.History.Preservation = 0 }},
		{"oil needs thermal window", "crude-oil", func(c *resourceContext) { c.p.History.MaxTemperature = 240 }},
		{"evaporite history", "halite", func(c *resourceContext) { c.p.History.Evaporative = false }},
		{"potash history", "potash", func(c *resourceContext) { c.p.History.Evaporative = false }},
		{"gypsum history", "gypsum", func(c *resourceContext) { c.p.History.Evaporative = false }},
		{"copper fluid pathway", "copper", func(c *resourceContext) { c.c.Fault = -1 }},
	}
	for _, test := range checks {
		t.Run(test.name, func(t *testing.T) {
			c := base
			if resourceEligibility(test.id, c).Score <= 0 {
				t.Fatal("valid host rejected")
			}
			test.remove(&c)
			if resourceEligibility(test.id, c).Score != 0 {
				t.Fatal("missing causal prerequisite accepted")
			}
		})
	}
	// A surface forest is not ancient organic source history.
	c := base
	c.p.History.AncientWetland = false
	if resourceEligibility("coal", c).Score != 0 {
		t.Fatal("modern forest made coal")
	}
	c = base
	c.cover = nil
	if resourceEligibility("crude-oil", c).Score != 0 {
		t.Fatal("petroleum without source/reservoir/seal")
	}
	c = base
	c.intrusion = &GeologicalFormation{Rock: "kimberlite"}
	if resourceEligibility("diamonds", c).Score <= 0 {
		t.Fatal("cratonic transport host rejected")
	}
	c.p.AgeMa = 100
	if resourceEligibility("diamonds", c).Score != 0 {
		t.Fatal("young-crust diamonds")
	}
	c = base
	c.intrusion = &GeologicalFormation{Rock: "carbonatite"}
	if resourceEligibility("rare-earth-elements", c).Score <= 0 {
		t.Fatal("carbonatite rejected")
	}
	c.intrusion = base.intrusion
	if resourceEligibility("rare-earth-elements", c).Score != 0 {
		t.Fatal("ordinary granite made rare-earth carbonatite")
	}
	if resourceEligibility("geothermal-energy", base).Score <= 0 {
		t.Fatal("supplied heated fracture rejected")
	}
	e.Fields["geothermal"][0] = 45
	if resourceEligibility("geothermal-energy", base).Score != 0 {
		t.Fatal("geothermal without heat")
	}
	e.Fields["geothermal"][0] = 90
	e.Fields["groundFlow"][0] = 0
	if resourceEligibility("geothermal-energy", base).Score != 0 {
		t.Fatal("geothermal without circulation")
	}
	e.Fields["groundFlow"][0] = .2
	for _, k := range []string{"recharge", "waterTableDepth", "elevation", "groundwaterHead"} {
		e.Fields[k] = []float64{0}
	}
	e.Fields["recharge"][0] = .1
	c = base
	if got := resourceEligibility("groundwater-aquifer", c).Variant; got != "confined-sedimentary" {
		t.Fatalf("confining strata lost: %s", got)
	}
	e.Fields["groundwaterHead"][0] = 20
	if got := resourceEligibility("groundwater-aquifer", c).Variant; got != "artesian-potential" {
		t.Fatalf("positive confined head not distinguished: %s", got)
	}
	c.cover = &GeologicalFormation{Rock: "alluvium"}
	if got := resourceEligibility("groundwater-aquifer", c).Variant; got != "shallow-alluvial" {
		t.Fatalf("alluvial water %s", got)
	}
	c.cover = &GeologicalFormation{Rock: "limestone"}
	if got := resourceEligibility("groundwater-aquifer", c).Variant; got != "karst-carbonate" {
		t.Fatalf("carbonate water %s", got)
	}
	c.cover = nil
	if got := resourceEligibility("groundwater-aquifer", c).Variant; got != "regional-fractured-rock" {
		t.Fatalf("fractured water %s", got)
	}
}
