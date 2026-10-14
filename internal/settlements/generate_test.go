package settlements

import (
	"context"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/terrain/generate"
	"kriemhild/internal/terrain/model"
	"math"
	"reflect"
	"testing"
)

func TestSitingDeterministicFootprints(t *testing.T) {
	p, err := generate.Generate(context.Background(), model.DefaultSpec(), nil)
	if err != nil {
		t.Fatal(err)
	}
	spec := Spec{Name: "Market town", Kind: "town", Archetype: "market", Seed: 12}
	var preview Preview
	for i, b := range p.Grid.Fields["bed"] {
		if b < 100 || b > 600 {
			continue
		}
		pt := p.Grid.Point(i%p.Grid.Width, i/p.Grid.Width)
		spec.Longitude = pt[0] * 180 / math.Pi
		spec.Latitude = pt[1] * 180 / math.Pi
		preview, err = Generate(p, spec)
		if err == nil {
			break
		}
	}
	if err != nil || len(preview.Features) < 20 {
		t.Fatal("no feasible site", err)
	}
	again, err := Generate(p, spec)
	if err != nil || !reflect.DeepEqual(preview, again) {
		t.Fatal("settlement is nondeterministic")
	}
	ids := map[string]bool{}
	for _, f := range preview.Features {
		if ids[f.ID] {
			t.Fatal("duplicate feature id")
		}
		if f.Parent != "" && !ids[f.Parent] {
			t.Fatal("missing parent")
		}
		ids[f.ID] = true
		if f.Kind == "house" {
			if f.Geometry != "polygon" || len(f.Points) != 4 {
				t.Fatal("building is not a footprint")
			}
			for _, pt := range f.Points {
				if detail.Sample(p, pt[0], pt[1]).Water != "dry" {
					t.Fatal("submerged building")
				}
			}
		}
	}
	for i, b := range p.Grid.Fields["bed"] {
		if b > 0 {
			continue
		}
		pt := p.Grid.Point(i%p.Grid.Width, i/p.Grid.Width)
		spec.Longitude = pt[0] * 180 / math.Pi
		spec.Latitude = pt[1] * 180 / math.Pi
		if math.Abs(spec.Latitude) > 80 {
			continue
		}
		if _, err = Generate(p, spec); err == nil {
			t.Fatal("underwater settlement accepted")
		}
		break
	}
}
