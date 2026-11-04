package generate

import (
	"context"
	"kriemhild/internal/terrain/model"
	"math"
	"reflect"
	"testing"
)

func TestVersionedReliefAcrossRepresentativeWorlds(t *testing.T) {
	var oldMountainArea, newMountainArea, landArea float64
	for _, seed := range []string{"Kriemhild", "gallery-001", "gallery-002"} {
		var previous *model.Planet
		for _, version := range []int{0, 1} {
			spec := model.DefaultSpec()
			spec.SurfaceWidth = 1024
			spec.Seed = seed
			spec.ReliefVersion = version
			p, err := Generate(context.Background(), spec, nil)
			if err != nil {
				t.Fatal(err)
			}
			if version == 0 {
				previous = p
			} else if len(p.Surface.Bed) != len(previous.Surface.Bed) {
				t.Fatal("preview change altered source resolution")
			}
			for i, h := range p.Surface.Bed {
				if !model.Finite(float64(h)) {
					t.Fatal("non-finite mountain")
				}
				if p.Surface.Coast[i] > 0 {
					weight := math.Cos(math.Pi * (.5 - (float64(i/p.Surface.Width)+.5)/float64(p.Surface.Height)))
					if version == 0 {
						landArea += weight
						if h > 1500 {
							oldMountainArea += weight
						}
					} else if h > 1500 {
						newMountainArea += weight
					}
				}
			}
			if p.Spec.ClimateVersion != previous.Spec.ClimateVersion {
				t.Fatal("relief update selected another climate model")
			}
		}
		meta, fields, err := previous.Encode()
		if err != nil {
			t.Fatal(err)
		}
		restored, err := model.Decode(meta, fields)
		if err != nil {
			t.Fatal(err)
		}
		if restored.Spec.ReliefVersion != 0 || !reflect.DeepEqual(restored.Surface, previous.Surface) {
			t.Fatal("old saved terrain was upgraded or regenerated")
		}
	}
	t.Logf("area-weighted land above 1500m: old %.3f, new %.3f", oldMountainArea/landArea, newMountainArea/landArea)
	if newMountainArea <= oldMountainArea {
		t.Fatal("wider uplift belts did not produce more mountain terrain in representative worlds")
	}
}
