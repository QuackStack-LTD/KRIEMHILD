package generate

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"kriemhild/internal/terrain/model"
	"math"
	"os"
	"testing"
)

func TestHistoricalClimateFieldGoldens(t *testing.T) {
	for version := 0; version <= 3; version++ {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			b, err := os.ReadFile(fmt.Sprintf("testdata/climate-compatibility/v%d.json", version))
			if err != nil {
				t.Fatal(err)
			}
			var golden struct {
				Hashes          map[string]string
				BiomeLandShares map[string]float64
			}
			if err = json.Unmarshal(b, &golden); err != nil {
				t.Fatal(err)
			}
			spec := model.DefaultSpec()
			spec.ClimateVersion = version
			spec.SurfaceWidth = 1024
			p, err := Generate(context.Background(), spec, nil)
			if err != nil {
				t.Fatal(err)
			}
			for key, want := range golden.Hashes {
				h := sha256.New()
				if len(key) > 5 && key[:5] == "grid/" {
					a, ok := p.Grid.Fields[key[5:]]
					if !ok {
						t.Fatal("missing historical field", key)
					}
					var word [8]byte
					for _, v := range a {
						binary.LittleEndian.PutUint64(word[:], math.Float64bits(v))
						h.Write(word[:])
					}
				} else {
					a := map[string][]float32{"surface/bed": p.Surface.Bed, "surface/coast": p.Surface.Coast, "surface/roughness": p.Surface.Roughness, "surface/lakeLevel": p.Surface.LakeLevel}[key]
					if a == nil {
						t.Fatal("unknown golden field", key)
					}
					var word [4]byte
					for _, v := range a {
						binary.LittleEndian.PutUint32(word[:], math.Float32bits(v))
						h.Write(word[:])
					}
				}
				if got := fmt.Sprintf("%x", h.Sum(nil)); got != want {
					t.Errorf("historical field changed: %s", key)
				}
			}
			shares := ClimateMeasurements(p)["all"].Biomes
			for key, want := range golden.BiomeLandShares {
				if math.Abs(shares[key]-want) > 1e-12 {
					t.Errorf("historical biome %s changed: %g -> %g", key, want, shares[key])
				}
			}
		})
	}
}
