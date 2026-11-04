package generate

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"kriemhild/internal/terrain/model"
)

// Explicit, offline diagnostic, never a saved-world migration or API path.
// Re-simulate weather on identical saved relief and retain its rivers/lakes.
func TestFixedSurfaceClimateAudit(t *testing.T) {
	source, out := os.Getenv("KRIEMHILD_FIXED_SOURCE"), os.Getenv("KRIEMHILD_FIXED_OUT")
	if source == "" || out == "" {
		t.Skip("opt-in read-only fixed-surface comparison")
	}
	meta, err := os.ReadFile(filepath.Join(source, "metadata"))
	if err != nil {
		t.Fatal(err)
	}
	fields, err := os.ReadFile(filepath.Join(source, "fields"))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := model.Decode(meta, fields)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Surface == nil {
		t.Fatal("saved authoritative surface required")
	}
	hash := func(a []float32) string {
		h := sha256.New()
		var b [4]byte
		for _, v := range a {
			binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
			h.Write(b[:])
		}
		return fmt.Sprintf("%x", h.Sum(nil))
	}
	bedHash, coastHash, lakeHash := hash(saved.Surface.Bed), hash(saved.Surface.Coast), hash(saved.Surface.LakeLevel)
	for _, version := range []int{2, 3} {
		p, err := model.Decode(meta, fields)
		if err != nil {
			t.Fatal(err)
		}
		p.Spec.ClimateVersion = version
		weather := *p
		weather.Surface = nil
		weather.Grid = model.Grid{Width: 192, Height: 96, Fields: map[string][]float64{}}
		for _, key := range []string{"bed", "coast", "roughness", "windX", "windY", "temperature", "precipitation", "pet", "moisture"} {
			weather.Grid.Fields[key] = make([]float64, 192*96)
		}
		for i := range weather.Grid.Fields["bed"] {
			pt := weather.Grid.Point(i%192, i/192)
			weather.Grid.Fields["bed"][i] = p.Surface.At(p.Surface.Bed, pt[0], pt[1])
			weather.Grid.Fields["coast"][i] = p.Surface.At(p.Surface.Coast, pt[0], pt[1])
			weather.Grid.Fields["roughness"][i] = p.Surface.At(p.Surface.Roughness, pt[0], pt[1])
		}
		weather.Diagnostics.ClimateBudget = nil
		if err = climate(context.Background(), &weather); err != nil {
			t.Fatal(err)
		}
		sea := seaLevelClimate(weather.Grid)
		for key := range weather.Grid.Fields {
			if key == "bed" || key == "coast" || key == "roughness" {
				continue
			}
			a := make([]float64, p.Grid.Width*p.Grid.Height)
			for i := range a {
				pt := p.Grid.Point(i%p.Grid.Width, i/p.Grid.Width)
				a[i] = weather.Grid.At(key, pt[0], pt[1])
			}
			p.Grid.Fields[key] = a
		}
		for i, bed := range p.Grid.Fields["bed"] {
			pt := p.Grid.Point(i%p.Grid.Width, i/p.Grid.Width)
			lapse := math.Max(0, bed) * .0065
			p.Grid.Fields["temperature"][i] = sea.At("temperature", pt[0], pt[1]) - lapse
			for _, key := range temperatureMonths {
				p.Grid.Fields[key][i] = sea.At(key, pt[0], pt[1]) - lapse
			}
		}
		materializedMoisture(&p.Grid, p.Spec)
		summarizeSeasons(&p.Grid)
		p.Diagnostics.ClimateBudget = weather.Diagnostics.ClimateBudget
		if hash(p.Surface.Bed) != bedHash || hash(p.Surface.Coast) != coastHash || hash(p.Surface.LakeLevel) != lakeHash || !reflect.DeepEqual(p.Rivers, saved.Rivers) {
			t.Fatal("diagnostic changed authoritative geography")
		}
		for _, key := range []string{"bed", "coast", "lake", "wetland", "flow", "drainage", "discharge"} {
			if !reflect.DeepEqual(p.Grid.Fields[key], saved.Grid.Fields[key]) {
				t.Fatal("diagnostic changed saved mask or hydrology", key)
			}
		}
		dir := filepath.Join(out, fmt.Sprintf("v%d", version))
		if err = os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		m, f, err := p.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "metadata"), m, 0644); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "fields"), f, 0644); err != nil {
			t.Fatal(err)
		}
		proof, _ := json.MarshalIndent(map[string]any{"diagnosticOnly": true, "sourceSeed": saved.Spec.Seed, "sourceClimateVersion": saved.Spec.ClimateVersion, "candidateClimateVersion": version, "surfaceBedSHA256": bedHash, "surfaceCoastSHA256": coastHash, "surfaceLakeSHA256": lakeHash, "riversUnchanged": true, "gridWaterMasksUnchanged": true}, "", "  ")
		if err = os.WriteFile(filepath.Join(dir, "proof.json"), proof, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
