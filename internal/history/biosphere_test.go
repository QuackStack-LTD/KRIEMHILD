package history

import (
	"encoding/json"
	"kriemhild/internal/biosphere"
	"reflect"
	"testing"
)

func TestBiosphereRetainsConfirmedGeometryAndIndependentInheritance(t *testing.T) {
	d := New("Ecology")
	age := d.World.CurrentAge
	d.Terrains = []Terrain{{ID: "terrain", AgeID: age, Name: "Earth", Width: 16, Height: 16}}
	b := biosphere.Catalogue()[0].Biology
	b.Traits = map[string]string{"physiology": "deciduous"}
	h := biosphere.Habitat{ID: "region-v1", RegionID: "region", Name: "Forest", Kind: "temperate-forest", Cells: []int{1, 2, 3}, Conditions: biosphere.Conditions{Medium: "terrestrial", Temperature: biosphere.Range{Min: 5, Max: 25}, Moisture: biosphere.Range{Min: .4, Max: .8}, Elevation: biosphere.Range{Min: 20, Max: 30}, Light: 1, Fertility: .6}}
	st := biosphere.State{ID: "terrain", AgeID: age, TerrainID: "terrain", Source: "surface-1", Version: 1, Width: 16, Height: 16, Habitats: []biosphere.Habitat{h}, Settings: biosphere.Settings{Mode: "guided", Richness: "moderate", Coherence: "realistic", Scope: "entire", Familiarity: .5, Seed: "seed"}, Occurrences: []biosphere.Occurrence{{ID: "population-1", SpeciesID: "tree", TerrainID: "terrain", HabitatID: h.ID, Status: "native", Abundance: .8}}}
	p := biosphere.Preview{State: st, Species: []biosphere.Species{{ID: "tree", Name: "Old tree", Biology: b}}}
	// An independently selected consumer cannot be saved without its required food.
	isolated := biosphere.Clone(*d)
	unsupported := biosphere.Clone(p)
	grazer := biosphere.Clone(b)
	grazer.Group = "animal"
	grazer.Roles = []string{"herbivore"}
	grazer.Dependencies = []biosphere.Dependency{{Group: "plant", Kind: "feeds-on"}}
	unsupported.Species = append(unsupported.Species, biosphere.Species{ID: "grazer", Name: "Grazer", Biology: grazer})
	unsupported.State.Occurrences = append(unsupported.State.Occurrences, biosphere.Occurrence{ID: "consumer", SpeciesID: "grazer", TerrainID: "terrain", HabitatID: h.ID, Status: "native", Abundance: .5})
	unsupported.State.Interactions = []biosphere.Interaction{{ID: "food", Source: "grazer", Destination: "tree", HabitatID: h.ID, Kind: "feeds-on"}}
	if err := isolated.AcceptBiosphere(unsupported, []string{"consumer"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(isolated.Biospheres[0].Occurrences) != 0 || len(isolated.Biospheres[0].Diagnostics) == 0 {
		t.Fatal("unsupported consumer saved without diagnostic")
	}
	if isolated.Biospheres[0].AcceptedGeneration != "" {
		t.Fatal("partial acceptance marked the entire preview complete")
	}
	if err := d.AcceptBiosphere(p, []string{"population-1"}, []string{"population-1"}, nil); err != nil {
		t.Fatal(err)
	}
	if d.Biosphere(age, "terrain").AcceptedGeneration != biosphere.GenerationKey(st.Source, st.Settings) {
		t.Fatal("complete acceptance did not retain its generation identity")
	}
	p.State.Source = "surface-2"
	p.State.Habitats[0].ID = "region-v2"
	p.State.Habitats[0].Cells = []int{1, 2, 3, 4}
	p.State.Occurrences[0].HabitatID = "region-v2"
	p.State.Occurrences[0].ID = "population-2"
	if err := d.AcceptBiosphere(p, []string{"population-2"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	saved := d.Biosphere(age, "terrain")
	if !saved.Stale || len(saved.Habitats) != 2 || len(saved.Occurrences) != 2 {
		t.Fatal("environment refresh discarded protected populations")
	}
	for _, region := range saved.Habitats {
		if region.ID == "region-v1" && !reflect.DeepEqual(region.Cells, []int{1, 2, 3}) {
			t.Fatal("confirmed range moved")
		}
	}
	if err := d.Apply(Command{Kind: "age", AgeID: age, Name: "Later", Mode: "after"}); err != nil {
		t.Fatal(err)
	}
	later := d.World.CurrentAge
	d.Biosphere(later, "terrain").Settings.Seed = "later"
	d.Entity(later, "tree").Biology.Traits["reproduction"] = "Later adaptation"
	if d.Biosphere(age, "terrain").Settings.Seed != "seed" || d.Entity(age, "tree").Biology.Traits["reproduction"] != "" {
		t.Fatal("Age state shared memory")
	}
	if err := d.Apply(Command{Kind: "age", AgeID: age, Name: "Without tree", Mode: "branch", ExcludeIDs: []string{"tree"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(d.Biosphere(d.World.CurrentAge, "terrain").Occurrences) != 0 || len(d.Biosphere(age, "terrain").Occurrences) != 2 {
		t.Fatal("inheritance exclusion changed source")
	}
	copy := biosphere.Clone(*d)
	copy.Ages = copy.Ages[:1]
	copy.World.CurrentAge = age
	before, _ := json.Marshal(copy.Biospheres)
	if err := copy.Apply(Command{Kind: "delete-age", AgeID: age}); err == nil {
		t.Fatal("deleted last Age")
	}
	after, _ := json.Marshal(copy.Biospheres)
	if string(before) != string(after) {
		t.Fatal("failed delete altered Biosphere")
	}
}
