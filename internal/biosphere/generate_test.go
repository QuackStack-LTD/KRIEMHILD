package biosphere

import (
	"encoding/json"
	"kriemhild/internal/terrain"
	"reflect"
	"slices"
	"testing"
)

func fixture() *terrain.Environment {
	w, h := 32, 16
	e := &terrain.Environment{Options: terrain.EnvironmentOptions{Columns: w, Rows: h, Seed: "biosphere-test"}, Fields: map[string][]float64{}, Mask: make([]int, w*h)}
	for _, k := range []string{"elevation", "temperature", "winter", "summer", "moisture", "precipitation", "waterDepth", "ocean", "salinity", "river", "lake", "wetland", "geothermal", "spring", "aridity", "fertility", "bathymetry"} {
		e.Fields[k] = make([]float64, w*h)
	}
	for i := range e.Mask {
		e.Mask[i] = 1
		e.Fields["elevation"][i] = 200
		e.Fields["temperature"][i] = 15
		e.Fields["winter"][i] = 5
		e.Fields["summer"][i] = 25
		e.Fields["moisture"][i] = .7
		e.Fields["precipitation"][i] = 900
		e.Fields["fertility"][i] = .6
		switch i % w / 4 {
		case 0:
			e.Fields["temperature"][i] = 26
			e.Fields["winter"][i] = 22
			e.Fields["summer"][i] = 30
			e.Fields["precipitation"][i] = 2200
		case 1: // temperate forest
		case 2:
			e.Fields["aridity"][i] = .9
			e.Fields["moisture"][i] = .1
			e.Fields["precipitation"][i] = 80
		case 3:
			e.Fields["elevation"][i] = 3200
			e.Fields["temperature"][i] = -3
			e.Fields["winter"][i] = -22
			e.Fields["summer"][i] = 8
		case 4:
			e.Fields["wetland"][i] = .8
			e.Fields["moisture"][i] = 1
		case 5:
			e.Fields["river"][i] = 1
			e.Fields["waterDepth"][i] = 2
		case 6:
			e.Mask[i] = 0
			e.Fields["ocean"][i] = 1
			e.Fields["salinity"][i] = 35
			e.Fields["waterDepth"][i] = 30
			e.Fields["elevation"][i] = -30
		case 7:
			e.Mask[i] = 0
			e.Fields["ocean"][i] = 1
			e.Fields["salinity"][i] = 35
			e.Fields["waterDepth"][i] = 2500
			e.Fields["elevation"][i] = -2500
		}
	}
	return e
}
func generated(t *testing.T, settings Settings) Preview {
	t.Helper()
	e := fixture()
	h, source, err := Habitats(e, nil, "terrain")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Generate(State{ID: "terrain", TerrainID: "terrain", AgeID: "a", Version: 1, Source: source, Width: 32, Height: 16, Habitats: h}, settings, nil)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func settings() Settings {
	return Settings{Mode: "guided", Preset: "earth-like", Familiarity: .5, Richness: "diverse", Coherence: "realistic", Scope: "entire", Seed: "ecosystem", Rules: []Rule{}}
}
func TestConnectedHabitatsAndDeterministicCommunities(t *testing.T) {
	p := generated(t, settings())
	other := generated(t, settings())
	if !reflect.DeepEqual(p, other) {
		t.Fatal("generation not deterministic")
	}
	kinds := map[string]bool{}
	for _, h := range p.State.Habitats {
		kinds[h.Kind] = true
		seen := map[int]bool{h.Cells[0]: true}
		for change := true; change; {
			change = false
			for _, c := range h.Cells {
				if seen[c] {
					continue
				}
				for _, offset := range []int{-1, 1, -32, 32} {
					if seen[c+offset] && ((offset != 1 && offset != -1) || c/32 == (c+offset)/32) {
						seen[c] = true
						change = true
					}
				}
			}
		}
		if len(seen) != len(h.Cells) {
			t.Fatal("scattered habitat cells")
		}
	}
	for _, k := range []string{"tropical-forest", "temperate-forest", "desert", "alpine", "wetland", "river", "shallow-marine", "open-ocean"} {
		if !kinds[k] {
			t.Fatal("missing habitat", k)
		}
	}
	species := map[string]Lifeform{}
	groups := map[string]bool{}
	origins := map[string]bool{}
	for _, s := range p.Species {
		species[s.ID] = s.Biology
		groups[s.Biology.Group] = true
		origins[s.Biology.Origin] = true
		if err := ValidateLifeform(&s.Biology); err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidateState(p.State, species); err != nil {
		t.Fatal(err)
	}
	for _, g := range []string{"plant", "animal", "fungus", "bacterium", "algae", "virus"} {
		if !groups[g] {
			t.Fatal("missing biological group", g)
		}
	}
	if !origins["earth-reference"] || !origins["fictional"] {
		t.Fatal("balanced mode lost origin diversity")
	}
	for _, h := range p.State.Habitats {
		count := 0
		for _, o := range p.State.Occurrences {
			if o.HabitatID == h.ID {
				count++
			}
		}
		if count < 2 {
			t.Fatal("incomplete base community", h.Name)
		}
	}
	b, _ := json.Marshal(p)
	var restored Preview
	if err := json.Unmarshal(b, &restored); err != nil || !reflect.DeepEqual(p, restored) {
		t.Fatal("structured data not portable", err)
	}
}
func TestHardConstraintsHostsAndMissingScope(t *testing.T) {
	p := generated(t, settings())
	species := map[string]Lifeform{}
	for _, s := range p.Species {
		species[s.ID] = s.Biology
	}
	var forest, ocean Habitat
	for _, h := range p.State.Habitats {
		if h.Kind == "temperate-forest" {
			forest = h
		}
		if h.Kind == "open-ocean" {
			ocean = h
		}
	}
	fox := Catalogue()[16].Biology
	for _, s := range Catalogue() {
		if s.ID == "red-fox" {
			fox = s.Biology
		}
	}
	if score, _ := Suitability(fox, ocean); score != 0 {
		t.Fatal("terrestrial fox accepted in open ocean")
	}
	fox.Requirements.Substrates = []string{"impossible-substrate"}
	if score, _ := Suitability(fox, forest); score != 0 {
		t.Fatal("required substrate ignored")
	}
	broken := Clone(p.State)
	for i, r := range broken.Interactions {
		if r.Kind == "infects" {
			broken.Interactions = append(broken.Interactions[:i], broken.Interactions[i+1:]...)
			break
		}
	}
	if err := ValidateState(broken, species); err == nil {
		t.Fatal("hostless virus accepted")
	}
	opts := settings()
	opts.Scope = "missing"
	next, err := Generate(p.State, opts, p.Species)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.State.Occurrences) != 0 {
		t.Fatal("missing-only generation replaced existing ecological roles", len(next.State.Occurrences))
	}
}
func TestFictionalRulesAreExecutableAndScoped(t *testing.T) {
	e := fixture()
	for i := range e.Mask {
		if i%32 < 4 {
			e.Fields["geothermal"][i] = 90
			e.Fields["spring"][i] = 1
			e.Fields["temperature"][i] = 18
			e.Fields["winter"][i] = 5
			e.Fields["summer"][i] = 30
		}
	}
	habitats, source, _ := Habitats(e, nil, "t")
	opts := settings()
	opts.Rules = []Rule{{ID: "glow", Name: "Luminous fungi", TargetGroup: "fungus", Habitats: []string{"geothermal"}, Traits: map[string]string{"bioluminescence": "night light"}, Abundance: 1, Conflict: "replace", Enabled: true, Strict: true}, {ID: "food", Name: "Fungivory", TargetGroup: "animal", TargetRoles: []string{"herbivore"}, Habitats: []string{"geothermal"}, FoodGroup: "fungus", Abundance: 1, Conflict: "merge", Enabled: true}}
	p, err := Generate(State{ID: "t", AgeID: "a", TerrainID: "t", Width: 32, Height: 16, Source: source, Version: 1, Habitats: habitats}, opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	microbe := false
	species := map[string]Lifeform{}
	for _, s := range p.Species {
		species[s.ID] = s.Biology
		if s.Biology.Traits["bioluminescence"] != "" {
			found = true
			if !slices.Contains(s.Biology.Requirements.Habitats, "geothermal") {
				t.Fatal("rule lost habitat restriction")
			}
		}
		microbe = microbe || s.Biology.Group == "archaea"
	}
	if !found || !microbe {
		t.Fatal("geothermal rule or archaea absent")
	}
	if err := ValidateState(p.State, species); err != nil {
		t.Fatal(err)
	}
	rule := opts.Rules[0]
	rule.ID = "conflict"
	rule.Conflict = "reject"
	rule.Traits = map[string]string{"bioluminescence": "different"}
	b := Catalogue()[9].Biology
	b.Group = "fungus"
	h := habitats[0]
	h.Kind = "geothermal"
	if err := ApplyRules(&b, h, []Rule{opts.Rules[0], rule}); err == nil {
		t.Fatal("conflict ignored")
	}
}
func TestQuickAndGenerationVariants(t *testing.T) {
	opts := settings()
	opts.Mode = "quick"
	opts.Richness = "sparse"
	p := generated(t, opts)
	placeholder := false
	for _, s := range p.Species {
		placeholder = placeholder || s.Biology.Origin == "placeholder"
	}
	if !placeholder {
		t.Fatal("quick mode does not provide expandable placeholders")
	}
	opts = settings()
	a := generated(t, opts)
	h := a.State.Habitats[0]
	opts.Variants = map[string]int{h.ID + "/producer": 1}
	b := generated(t, opts)
	if reflect.DeepEqual(a.Species, b.Species) {
		t.Fatal("regeneration variant has no effect")
	}
	for _, o := range a.State.Occurrences {
		if o.HabitatID == h.ID {
			continue
		}
		if !slices.ContainsFunc(b.State.Occurrences, func(other Occurrence) bool { return reflect.DeepEqual(o, other) }) {
			t.Fatal("regenerating one region changed another")
		}
	}
}

func TestGeothermalPotentialIsNotSurfaceHabitat(t *testing.T) {
	e := fixture()
	for i := range e.Mask {
		e.Fields["geothermal"][i] = 70
	}
	hs, _, err := Habitats(e, nil, "earth")
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hs {
		if h.Kind == "geothermal" {
			t.Fatal("deep heat potential cannot replace ordinary surface ecosystems")
		}
	}
}

func TestHabitatRevisionsAndSharedSpecies(t *testing.T) {
	e := fixture()
	hs, _, _ := Habitats(e, nil, "terrain")
	first := hs[0]
	e.Fields["elevation"][first.Cells[0]] += 1
	updated, _, _ := Habitats(e, nil, "terrain")
	var revision Habitat
	for _, h := range updated {
		if h.RegionID == first.RegionID {
			revision = h
			break
		}
	}
	if revision.ID == first.ID || revision.ID == "" {
		t.Fatal("changed habitat did not receive independent environmental revision")
	}
	unchanged := false
	for _, h := range hs {
		if h.RegionID != first.RegionID && slices.ContainsFunc(updated, func(v Habitat) bool { return reflect.DeepEqual(h, v) }) {
			unchanged = true
		}
	}
	if !unchanged {
		t.Fatal("environment change relocated unrelated habitats")
	}
	// Comparable forest strata share species identities rather than cloning each species per habitat.
	p := generated(t, settings())
	regions := map[string]map[string]bool{}
	for _, o := range p.State.Occurrences {
		if regions[o.SpeciesID] == nil {
			regions[o.SpeciesID] = map[string]bool{}
		}
		regions[o.SpeciesID][o.HabitatID] = true
	}
	shared := false
	for _, rs := range regions {
		shared = shared || len(rs) > 1
	}
	if !shared {
		t.Fatal("lifeforms duplicated per habitat instead of shared range records")
	}
}

func TestLiquidWaterAndAuthoredLand(t *testing.T) {
	e := fixture()
	i := 6 * 4
	e.Fields["winter"][i] = -45
	e.Fields["summer"][i] = -12
	hs, _, _ := Habitats(e, nil, "water")
	for _, h := range hs {
		if slices.Contains(h.Cells, i) && h.Conditions.Temperature.Min < -3 {
			t.Fatal("aquatic habitat used freezing air as liquid water")
		}
	}
	// Authored land raised out of the ocean overrides the old generation mask.
	e.Fields["waterBody"] = make([]float64, len(e.Mask))
	e.Fields["elevation"][i] = 50
	e.Fields["waterDepth"][i] = 0
	e.Fields["river"][i] = 0
	hs, _, _ = Habitats(e, nil, "water")
	for _, h := range hs {
		if slices.Contains(h.Cells, i) && h.Conditions.Medium != "terrestrial" {
			t.Fatal("authored elevation/water state ignored")
		}
	}
}
