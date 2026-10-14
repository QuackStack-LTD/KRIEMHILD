package world

import (
	"kriemhild/internal/terrain"
	"testing"
)

func TestAuthoredCoverSurvivesClimateReconciliation(t *testing.T) {
	o, err := terrain.DecodeEnvironment([]byte(`{"columns":16,"rows":16,"seed":"authored-cover","realism":true}`))
	if err != nil {
		t.Fatal(err)
	}
	base := terrain.BuildEnvironment(o)
	i := 0
	for cell, v := range base.Mask {
		if v != 0 {
			i = cell
			break
		}
	}
	forest := 1.
	for index, b := range terrain.Biomes {
		if b.ID == "forest" {
			forest = float64(index)
		}
	}
	base.Fields["biome"][i] = forest
	base.Fields["vegetation"][i] = 1
	s := New(16, 16)
	op := Operation{ID: "clear", Kind: "clear", Layer: "vegetation", Radius: 2, Center: [2]float64{float64(i % 16), float64(i / 16)}}
	s.Operations[op.ID] = op
	s.Reindex(16, 16)
	edited := s.Environment(base)
	edited.RebuildClimateForSurface(base)
	if edited.BiomeAt(i) != "grass" || base.BiomeAt(i) != "forest" {
		t.Fatal("authored cover lost or base modified", edited.BiomeAt(i), base.BiomeAt(i))
	}
	p := Apply(terrain.DetailPoint{Biome: "forest", Vegetation: 1}, float64(i%16), float64(i/16), []Operation{op})
	if p.Biome != edited.BiomeAt(i) {
		t.Fatal("detail and snapshot cover disagree")
	}
}
