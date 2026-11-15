package history

import (
	"encoding/json"
	"fmt"
	"kriemhild/internal/biosphere"
	"testing"
)

// Model the catalogue and populations saved by a large terrain Biosphere.
func largeInheritanceWorld(n int) *Document {
	d := New("Large Biosphere")
	age := d.World.CurrentAge
	d.Terrains = []Terrain{{ID: "terrain", AgeID: age, Name: "Earth", Width: 16, Height: 16}}
	b := biosphere.Catalogue()[0].Biology
	b.Dependencies = nil
	h := biosphere.Habitat{ID: "forest", Name: "Forest", Kind: "temperate-forest", Cells: []int{1, 2, 3}, Conditions: biosphere.Conditions{Medium: "terrestrial"}}
	s := biosphere.State{ID: "terrain", AgeID: age, TerrainID: "terrain", Version: 1, Width: 16, Height: 16, Stale: true, Habitats: []biosphere.Habitat{h}, Settings: biosphere.Settings{Mode: "guided", Richness: "moderate", Coherence: "realistic", Scope: "entire", Familiarity: .5, Seed: "seed"}}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("species-%08d", i)
		biology := b
		if i > 0 {
			biology.ParentID = "species-00000000"
		}
		d.Entities = append(d.Entities, Entity{ID: id, AgeID: age, TerrainID: "terrain", Kind: "lifeform", Name: id, Properties: json.RawMessage(`{}`), Biology: &biology})
		for j := 0; j < 3; j++ {
			s.Occurrences = append(s.Occurrences, biosphere.Occurrence{ID: fmt.Sprintf("%s/%d", id, j), SpeciesID: id, TerrainID: "terrain", HabitatID: h.ID, Status: "native", Abundance: .5})
		}
	}
	d.Biospheres = []biosphere.State{s}
	return d
}

func BenchmarkLargeBiosphereAge(b *testing.B) {
	for _, n := range []int{2000, 8000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				d := largeInheritanceWorld(n)
				b.StartTimer()
				if err := d.Apply(Command{Kind: "age", AgeID: d.World.CurrentAge, Name: "Later", Mode: "after"}); err != nil {
					b.Fatal(err)
				}
				if err := d.Validate(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestLargeBiosphereInheritanceExclusionsAndIndexLifetime(t *testing.T) {
	d := largeInheritanceWorld(2000)
	first := d.World.CurrentAge
	before, _ := json.Marshal(struct {
		Entities   []Entity
		Biospheres []biosphere.State
	}{d.Entities, d.Biospheres})
	if err := d.Apply(Command{Kind: "age", AgeID: first, Name: "Later", Mode: "after", ExcludeIDs: []string{"species-00000000"}, EntityNames: map[string]string{"species-00001999": "Later species"}}); err != nil {
		t.Fatal(err)
	}
	later := d.World.CurrentAge
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	if d.entityIndex != nil {
		t.Fatal("temporary lookup retained after operation")
	}
	if len(d.Entities) != 4000 || len(d.Biosphere(later, "terrain").Occurrences) != 5997 {
		t.Fatal("inheritance lost entities or kept excluded populations")
	}
	if e := d.Entity(later, "species-00001999"); e.Name != "Later species" || e.Biology.ParentID != "" {
		t.Fatal("rename or excluded taxon reconciliation lost")
	}
	d.Entity(later, "species-00001999").Biology.Requirements.Habitats = []string{"adapted"}
	d.Biosphere(later, "terrain").Habitats[0].Cells[0] = 4
	after, _ := json.Marshal(struct {
		Entities   []Entity
		Biospheres []biosphere.State
	}{d.Entities[:2000], d.Biospheres[:1]})
	if string(before) != string(after) {
		t.Fatal("new Age changed the source Age")
	}
	// Neither a successful nor a failed validation may leave a stale lookup when
	// later commands append entities or correct an invalid record.
	d.Entities = append(d.Entities, Entity{ID: "new", AgeID: later, Name: "New", Kind: "person", Properties: json.RawMessage(`{}`)})
	if d.Entity(later, "new") == nil {
		t.Fatal("stale entity lookup after append")
	}
	d.Entities[len(d.Entities)-1].Name = ""
	if d.Validate() == nil || d.entityIndex != nil {
		t.Fatal("failed validation retained lookup")
	}
	d.Entities[len(d.Entities)-1].Name = "Corrected"
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}
