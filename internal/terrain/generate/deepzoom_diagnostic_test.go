package generate

import (
	"context"
	"encoding/json"
	"kriemhild/internal/terrain/model"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// Opt-in read-only diagnostic: reconstruct only the original coarse climate of
// a saved world, then compare the materialized temperatures at the SAME sites.
func TestSavedClimateElevationDiagnostic(t *testing.T) {
	path := os.Getenv("KRIEMHILD_AUDIT_PLANET")
	if path == "" {
		t.Skip("set KRIEMHILD_AUDIT_PLANET to a disposable extracted archive")
	}
	meta, err := os.ReadFile(filepath.Join(path, "metadata"))
	if err != nil {
		t.Fatal(err)
	}
	fields, err := os.ReadFile(filepath.Join(path, "fields"))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := model.Decode(meta, fields)
	if err != nil {
		t.Fatal(err)
	}
	p := *saved
	p.Surface = nil
	p.Grid = model.Grid{Width: 192, Height: 96, Fields: map[string][]float64{}}
	if err = surface(context.Background(), &p); err != nil {
		t.Fatal(err)
	}
	if err = climate(context.Background(), &p); err != nil {
		t.Fatal(err)
	}
	sea := model.Grid{Width: 192, Height: 96, Fields: map[string][]float64{"temperature": make([]float64, 192*96)}}
	for i, bed := range p.Grid.Fields["bed"] {
		sea.Fields["temperature"][i] = p.Grid.Fields["temperature"][i] + math.Max(0, bed)*.0065
	}
	land, bad, sum, maxError := 0., 0., 0., 0.
	worst := map[string]any{}
	byBand := map[int][]float64{}
	for i, bed := range saved.Grid.Fields["bed"] {
		if !saved.Emerged(i) {
			continue
		}
		pt := saved.Grid.Point(i%saved.Grid.Width, i/saved.Grid.Width)
		weight := math.Cos(pt[1])
		land += weight
		expected := sea.At("temperature", pt[0], pt[1]) - math.Max(0, bed)*.0065
		actual := saved.Grid.Fields["temperature"][i]
		delta := expected - actual
		sum += delta * weight
		if math.Abs(delta) > 1 {
			bad += weight
		}
		band := int(math.Abs(pt[1])*180/math.Pi) / 15 * 15
		if byBand[band] == nil {
			byBand[band] = make([]float64, 3)
		}
		byBand[band][0] += weight
		byBand[band][1] += actual * weight
		byBand[band][2] += expected * weight
		if math.Abs(delta) > maxError {
			maxError = math.Abs(delta)
			worst = map[string]any{"lon": pt[0], "lat": pt[1], "bed": bed, "actual": actual, "expected": expected, "oldBed": p.Grid.At("bed", pt[0], pt[1])}
		}
	}
	report := map[string]any{"meanSpuriousCoolingC": sum / land, "emergedLandErrorAbove1C": bad / land, "maxAbsoluteErrorC": maxError, "worst": worst, "bandsAreaActualExpectedSums": byBand}
	b, _ := json.MarshalIndent(report, "", "  ")
	t.Log(string(b))
	if dst := os.Getenv("KRIEMHILD_AUDIT_OUT"); dst != "" {
		if err = os.WriteFile(dst, b, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
