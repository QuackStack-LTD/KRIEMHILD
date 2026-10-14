package biosphere

import (
	"encoding/json"
	"kriemhild/internal/legacyterrain"
	"reflect"
	"slices"
	"testing"
)

func TestVerticalHabitatsAndPhoticRequirement(t *testing.T) {
	habitats, _, err := Habitats(fixture(), nil, "ocean")
	if err != nil {
		t.Fatal(err)
	}
	var photic, deep Habitat
	for _, h := range habitats {
		if h.Kind == "open-ocean" {
			if h.Stratum == "photic" {
				photic = h
			}
			if h.Stratum == "benthic" {
				deep = h
			}
		}
	}
	if photic.ID == "" || deep.ID == "" || photic.Conditions.Depth.Max > 200 || photic.Conditions.Bathymetry.Min < 2000 {
		t.Fatal("missing independent water column", photic, deep)
	}
	var plankton Lifeform
	for _, s := range Catalogue() {
		if s.ID == "phytoplankton" {
			plankton = s.Biology
		}
	}
	if score, _ := Suitability(plankton, photic); score == 0 {
		t.Fatal("photic producer excluded")
	}
	plankton.Requirements.MinLight = 0
	plankton.Requirements.Strata = nil
	plankton.Requirements.Depth.Max = 12000
	if score, _ := Suitability(plankton, deep); score != 0 {
		t.Fatal("producer accepted in dark abyss")
	}
}

func TestCanonicalDrylandsAndAuthoredVegetation(t *testing.T) {
	e := fixture()
	n := len(e.Mask)
	e.Fields["biome"] = make([]float64, n)
	e.Fields["vegetation"] = make([]float64, n)
	index := func(id string) float64 {
		for i, b := range terrain.Biomes {
			if b.ID == id {
				return float64(i)
			}
		}
		t.Fatal(id)
		return 0
	}
	for i := range e.Mask {
		e.Fields["biome"][i] = index("steppe")
		e.Fields["vegetation"][i] = .5
	}
	h, source, _ := Habitats(e, nil, "t")
	if h[0].Kind != "steppe" {
		t.Fatal("dry climate overridden", h[0].Kind)
	}
	for i := range e.Mask {
		e.Fields["biome"][i] = index("forest")
		e.Fields["vegetation"][i] = 0
	}
	next, nextSource, _ := Habitats(e, nil, "t")
	if source == nextSource || next[0].Kind != "grassland" {
		t.Fatal("authored clearing absent from snapshot")
	}
}

func TestSuitableFractionAndSeasonalNiche(t *testing.T) {
	p := generated(t, settings())
	h := p.State.Habitats[0]
	b := p.Species[0].Biology
	for _, s := range p.Species {
		if score, _ := Suitability(s.Biology, h); score > 0 {
			b = s.Biology
			break
		}
	}
	score, _ := Suitability(b, h)
	if score == 0 {
		t.Fatal("fixture has no compatible lifeform")
	}
	h.Profiles = append([]Conditions(nil), h.Profiles...)
	for i := 0; i < len(h.Profiles)/2; i++ {
		h.Profiles[i].Temperature.Max = 1000
	}
	mixed, reason := Suitability(b, h)
	if mixed <= 0 || mixed >= score {
		t.Fatal("whole patch rejected or unsuitable fraction ignored", score, mixed, reason)
	}
	b.Requirements.DroughtMonths = &Range{11, 12}
	if score, _ := Suitability(b, h); score != 0 {
		t.Fatal("drought niche ignored")
	}
}

func TestNativeRangeUsesBarriersWrappingAndDistance(t *testing.T) {
	b := Catalogue()[0].Biology
	b.Requirements.Habitats = nil
	b.Requirements.DispersalKM = 10000
	c := Conditions{Medium: "terrestrial", Temperature: Range{10, 20}, Moisture: Range{.5, .7}, Elevation: Range{0, 100}, Light: 1, Substrates: []string{"soil"}}
	hs := []Habitat{}
	for i := 0; i < 7; i++ {
		local := c
		if i == 3 {
			local.Medium = "marine"
		}
		hs = append(hs, Habitat{ID: string(rune('a' + i)), Kind: "grassland", Cells: []int{i}, Conditions: local})
	}
	s := Species{ID: "seeded", Biology: b}
	state := State{Width: 7, Height: 1, CellKM: 100, Habitats: hs}
	r := connectedNativeRange(s, state, "seed")
	if len(r) != 3 {
		t.Fatal("crossed ocean barrier", r)
	}
	local := connectedNativeRange(s, state, "seed", "g")
	if !local["g"] || local["a"] || len(local) != 3 {
		t.Fatal("local adaptation originated on a disconnected continent", local)
	}
	state.WrapX = true
	r = connectedNativeRange(s, state, "seed")
	if len(r) != 6 {
		t.Fatal("longitude seam blocked terrestrial path", r)
	}
	s.Biology.Requirements.DispersalKM = 50
	r = connectedNativeRange(s, state, "seed")
	if len(r) != 1 {
		t.Fatal("distance not scaled", r)
	}
	state.CellEastWestKM, state.LatitudeNorth, state.LatitudeSouth = 100, 80, 80
	r = connectedNativeRange(s, state, "seed")
	if len(r) <= 1 {
		t.Fatal("east-west distances did not contract at high latitude")
	}
}

func TestPollinationRequiresFloweringAndColdResidentsStayNative(t *testing.T) {
	plant := Catalogue()[0].Biology
	dep := Dependency{Kind: "pollinates", Group: "plant"}
	if !compatibleDependency(Lifeform{}, dep, plant) {
		t.Fatal("flowering plant rejected")
	}
	plant.Traits = map[string]string{"reproduction": "spores"}
	if compatibleDependency(Lifeform{}, dep, plant) {
		t.Fatal("spores treated as flowers")
	}
	p := generated(t, settings())
	byID := map[string]Lifeform{}
	for _, s := range p.Species {
		byID[s.ID] = s.Biology
	}
	for _, o := range p.State.Occurrences {
		if byID[o.SpeciesID].Group == "animal" && o.Status != "native" {
			t.Fatal("temperature invents migration")
		}
	}
}

func TestVersionOneRemainsReadableAndLockedPreviewPreserved(t *testing.T) {
	p := generated(t, settings())
	old := p.Species[0].Biology
	old.Version = 1
	if err := ValidateLifeform(&old); err != nil {
		t.Fatal(err)
	}
	legacy := Lifeform{Version: 1, Group: "algae", Roles: []string{"producer"}, Requirements: Requirements{Medium: "marine", Temperature: Range{-5, 40}, Moisture: Range{0, 1}, Depth: Range{0, 12000}, Elevation: Range{-12000, 0}, Salinity: Range{0, 50}}}
	legacyHabitat := Habitat{Kind: "open-ocean", Stratum: "surface", Conditions: Conditions{Medium: "marine", Temperature: Range{2, 10}, Moisture: Range{1, 1}, Depth: Range{2500, 2500}, Elevation: Range{-2500, -2500}, Salinity: Range{35, 35}, Light: 0}}
	if score, _ := Suitability(legacy, legacyHabitat); score == 0 {
		t.Fatal("stored v1 pelagic-over-seabed semantics reinterpreted")
	}
	legacyHabitat.Stratum = "benthic"
	if score, _ := Suitability(legacy, legacyHabitat); score != 0 {
		t.Fatal("legacy tolerance bypassed new benthic darkness")
	}
	o := p.State.Occurrences[0]
	o.Locked = true
	o.Confirmed = true
	o.Abundance = .123
	p.State.Occurrences[0] = o
	next, err := Generate(p.State, settings(), p.Species)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, v := range next.State.Occurrences {
		if v.ID == o.ID {
			found = true
			if !reflect.DeepEqual(o, v) {
				t.Fatal("locked occurrence rewritten")
			}
		}
	}
	if !found {
		t.Fatal("locked occurrence lost")
	}
	data, _ := json.Marshal(next)
	var restored Preview
	if err = json.Unmarshal(data, &restored); err != nil || !reflect.DeepEqual(next, restored) {
		t.Fatal("v2 round trip changed niches")
	}
	for _, s := range next.Species {
		if slices.Contains(s.Biology.Roles, "producer") && s.Biology.Group == "algae" && s.Biology.Requirements.MinLight == 0 {
			t.Fatal("new producer bypasses light")
		}
	}
}

func TestDependencyIdentityPhenologyAndObligateCycles(t *testing.T) {
	source, target := Catalogue()[0].Biology, Catalogue()[0].Biology
	source.Traits = map[string]string{"activeSeasons": "spring, summer"}
	target.Traits = map[string]string{"reproduction": "flowering", "activeSeasons": "summer", "lifeStages": "adult", "massKg": "1"}
	dep := Dependency{Kind: "pollinates", SpeciesIDs: []string{"flower"}, Seasons: []string{"summer"}, LifeStage: "adult"}
	if !matchesDependency(source, dep, Species{ID: "flower", Biology: target}) {
		t.Fatal("compatible flowering host rejected")
	}
	if matchesDependency(source, dep, Species{ID: "other", Biology: target}) {
		t.Fatal("host identity ignored")
	}
	target.Traits["activeSeasons"] = "winter"
	if matchesDependency(source, dep, Species{ID: "flower", Biology: target}) {
		t.Fatal("pollination without overlapping phenology")
	}
	links := []Interaction{{HabitatID: "h", Source: "a", Destination: "b", Kind: "depends-on"}, {HabitatID: "h", Source: "b", Destination: "a", Kind: "depends-on"}}
	if validateObligateCycles(links) == nil {
		t.Fatal("unexplained circular obligate dependency accepted")
	}
	links[0].Kind, links[1].Kind = "symbiotic-with", "symbiotic-with"
	if err := validateObligateCycles(links); err != nil {
		t.Fatal("valid mutualistic cycle rejected", err)
	}
}

func TestRiverbanksOverlapWaterWithoutBlockingLandRanges(t *testing.T) {
	e := &terrain.Environment{Options: terrain.EnvironmentOptions{Columns: 5, Rows: 3}, Mask: make([]int, 15), Fields: map[string][]float64{}}
	for key, value := range map[string]float64{"elevation": 100, "temperature": 18, "winter": 10, "summer": 24, "moisture": .6, "waterBody": 0, "waterDepth": 0, "river": 0} {
		e.Fields[key] = make([]float64, 15)
		for i := range e.Mask {
			e.Mask[i] = 1
			e.Fields[key][i] = value
		}
	}
	for y := 0; y < 3; y++ {
		e.Fields["river"][y*5+2] = .8
	}
	hs, _, err := Habitats(e, nil, "river")
	if err != nil {
		t.Fatal(err)
	}
	land, water := false, false
	for _, h := range hs {
		if slices.Contains(h.Cells, 7) {
			land = land || h.Conditions.Medium == "terrestrial" && slices.Contains(h.Overlays, "riparian")
			water = water || h.Conditions.Medium == "freshwater"
		}
	}
	if !land || !water || e.Fields["river"][7] != .8 {
		t.Fatal("river erased its banks or derivation mutated terrain")
	}
	b := Catalogue()[0].Biology
	b.Requirements.Habitats = nil
	b.Requirements.DispersalKM = 1000
	state := State{Width: 5, Height: 3, CellKM: 10, Habitats: hs}
	ranges := connectedNativeRange(Species{ID: "grazer", Biology: b}, state, "seed")
	left, right := false, false
	for _, h := range hs {
		if ranges[h.ID] {
			for _, cell := range h.Cells {
				left = left || cell%5 == 0
				right = right || cell%5 == 4
			}
		}
	}
	if !left || !right {
		t.Fatal("river centreline blocked terrestrial banks", ranges)
	}
}

func TestNativeOriginUsesSurvivableCellsWithinMixedHabitat(t *testing.T) {
	b := Catalogue()[0].Biology
	b.Requirements.Habitats = nil
	b.Requirements.DispersalKM = 1.1
	c := Conditions{Medium: "terrestrial", Temperature: Range{10, 20}, Moisture: Range{.5, .7}, Elevation: Range{0, 100}, Light: 1, Substrates: []string{"soil"}}
	profiles := []Conditions{c, c, c, c, c}
	for i := 0; i < 4; i++ {
		profiles[i].Temperature.Max = 1000
	}
	state := State{Width: 6, Height: 1, CellKM: 1, Habitats: []Habitat{
		{ID: "mixed", Kind: "grassland", Cells: []int{0, 1, 2, 3, 4}, Conditions: c, Profiles: profiles},
		{ID: "neighbor", Kind: "grassland", Cells: []int{5}, Conditions: c, Profiles: []Conditions{c}},
	}}
	for _, seed := range []string{"one", "two", "three", "four", "five"} {
		r := connectedNativeRange(Species{ID: "native", Biology: b}, state, seed, "mixed")
		if !r["neighbor"] {
			t.Fatal("range originated in unsurvivable part of its habitat", seed, r)
		}
	}
}
