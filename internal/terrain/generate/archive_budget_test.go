package generate_test

import (
	"context"
	"kriemhild/internal/terrain/generate"
	"kriemhild/internal/terrain/model"
	"testing"
)

func TestLargeWetPlanetArchiveBudget(t *testing.T) {
	spec := model.DefaultSpec()
	spec.SurfaceWidth = 1024 // Functional checks use the smallest complete surface.
	spec.Seed = "large-wet-archive"
	spec.RadiusKM = 100000
	spec.Continents = 12
	spec.LandCoverage = .65
	spec.Rainfall = 3
	spec.Islands = 3
	p, err := generate.Generate(context.Background(), spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	meta, fields, err := p.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = model.Decode(meta, fields); err != nil {
		t.Fatalf("generated metadata (%d bytes, %d rivers) cannot reopen: %v", len(meta), len(p.Rivers), err)
	}
}
