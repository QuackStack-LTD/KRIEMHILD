package httpapi

import (
	"kriemhild/internal/biosphere"
	"kriemhild/internal/history"
	"testing"
)

func TestBiosphereSpotUsesSavedRangesAndAge(t *testing.T) {
	d := history.New("Habitats")
	age := d.World.CurrentAge
	d.Entities = []history.Entity{{ID: "tree", AgeID: age, Name: "Silver tree", Biology: &biosphere.Lifeform{Group: "plant", Roles: []string{"producer"}}}, {ID: "fox", AgeID: age, Name: "Red fox", Biology: &biosphere.Lifeform{Group: "animal"}}, {ID: "tree", AgeID: "older", Name: "Old tree", Biology: &biosphere.Lifeform{Group: "plant"}}}
	d.Biospheres = []biosphere.State{{ID: "s", AgeID: age, TerrainID: "terrain", Stale: true, Habitats: []biosphere.Habitat{{ID: "forest", Cells: []int{5, 6}, Name: "Forest canopy"}, {ID: "understory", Cells: []int{5}, Name: "Forest floor"}}, Occurrences: []biosphere.Occurrence{{HabitatID: "forest", SpeciesID: "tree", Abundance: .7}, {HabitatID: "understory", SpeciesID: "tree", Abundance: .4}, {HabitatID: "understory", SpeciesID: "fox", Abundance: .2, Status: "locally-extinct"}}}}
	spots := biosphereSpots(d, age, "terrain")
	if len(spots) != 2 || len(spots[5].Habitats) != 2 || len(spots[5].Lifeforms) != 1 || spots[5].Lifeforms[0].Name != "Silver tree" || spots[5].Lifeforms[0].Abundance != .7 || !spots[5].Stale {
		t.Fatalf("incorrect spot: %+v", spots[5])
	}
	if spots[7].Available || len(biosphereSpots(d, age, "other")) > 0 || len(biosphereSpots(d, "older", "terrain")) > 0 {
		t.Fatal("inferred a population outside saved range, terrain or Age")
	}
}
