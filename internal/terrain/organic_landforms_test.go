package terrain

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
)

func shapeFixture(seed string, w, h int) *Environment {
	o, _ := DecodeEnvironment([]byte(fmt.Sprintf(`{"columns":%d,"rows":%d,"seed":%q}`, w, h, seed)))
	e := &Environment{Options: o, Fields: map[string][]float64{}, Mask: make([]int, w*h), Margin: max(2, int(math.Ceil(float64(min(w, h))*.045)))}
	e.Geography, _ = o.geographicScale()
	rng := RNG(Seed(seed) ^ 0x738ac52)
	for k := 0; k < o.PlateCount; k++ {
		e.Plates = append(e.Plates, Plate{rng.Next() * float64(w), rng.Next() * float64(h), rng.Next()*2 - 1, rng.Next()*2 - 1, rng.Next() > .45})
	}
	macro := &Solver{Rules: &Rules{Config: Config{Continents: []Kind{{Odds: .5}, {Odds: .5}}}}, W: w, H: h, N: w * h, K: 2}
	macro.buildContinents(&ContinentOptions{o.ContinentCount, 5}, Seed(seed))
	e.Points = macro.ContPoints
	e.generateContinentalMask(e.Points, noise(Seed(seed)))
	return e
}

func TestOrganicShapeDiversityAcrossSeeds(t *testing.T) {
	elongated, indented, deepBays := 0, 0, 0
	forms := map[string]bool{}
	var previous []int
	for k := 0; k < 12; k++ {
		e := shapeFixture(fmt.Sprintf("shape-audit-%d", k), 128, 96)
		r := e.LandShapeDiagnostics()
		if math.Abs(r.LandFraction-.42) > .011 && e.Layout.Warning == "" {
			t.Fatal("coverage changed", r.LandFraction)
		}
		if len(r.IslandSizes) == 0 {
			t.Fatal("missing land", k)
		}
		if reflect.DeepEqual(previous, e.Mask) {
			t.Fatal("seed did not change silhouette")
		}
		previous = append([]int(nil), e.Mask...)
		for key := range r.Formations {
			forms[key] = true
		}
		for _, m := range r.Landmasses {
			if m.Area < 100 {
				continue
			}
			if m.Aspect > 2 {
				elongated++
			}
			if m.Solidity < .82 {
				indented++
			}
			if m.BayDepth > 4 {
				deepBays++
			}
		}
		t.Logf("%d components=%d largest=%d bays=%v", k, len(r.IslandSizes), r.IslandSizes[0], r.Landmasses[0])
	}
	// Correctly unwrapped hulls no longer count seam-crossing continents as
	// extremely elongated/concave. Most foundations should be broad; extremes occasional.
	if len(forms) < 6 || elongated < 2 || indented < 5 || deepBays < 8 {
		t.Fatal("insufficient structural diversity", forms, elongated, indented, deepBays)
	}
	a, b := shapeFixture("repeatable-shapes", 96, 64), shapeFixture("repeatable-shapes", 96, 64)
	if !reflect.DeepEqual(a.Mask, b.Mask) || !reflect.DeepEqual(a.ContinentalStructures, b.ContinentalStructures) {
		t.Fatal("shape grammar is not deterministic")
	}
}
func TestShapeGrammarRetainsTopologyWithoutNoise(t *testing.T) {
	makeShape := func(kind string) LandmassStructure {
		return makeLandmass(71, kind, [2]float64{64, 64}, 28, 0, LandShapeOptions{})
	}
	ring := makeShape("enclosed-basin")
	if ring.field(64, 64) >= 0 || ring.field(64+28*.8, 64) <= 0 {
		t.Fatal("basin is not enclosed by land")
	}
	neck := makeShape("isthmus")
	if neck.field(64, 64) <= 0 || neck.field(64, 64+28*.4) >= 0 {
		t.Fatal("isthmus lost its narrow connection")
	}
	crescent := makeShape("crescent")
	if crescent.field(64, 64) >= 0 || crescent.field(64+28*.8, 64) <= 0 {
		t.Fatal("crescent lost gulf")
	}
	branch := makeShape("branched")
	if len(branch.Spines) < 4 {
		t.Fatal("branches absent without coastline noise")
	}
	rift := makeShape("rifted")
	middle := rift.Cuts[0][7]
	if rift.field(middle.X, middle.Y) >= 0 {
		t.Fatal("rift is only a drawing, not negative geometry")
	}
}
func TestShapeSettingsAndStoredGeometry(t *testing.T) {
	for _, raw := range []string{`{"landShape":{"large":-1}}`, `{"landShape":{"medium":4}}`, `{"landShape":{"small":4}}`} {
		if _, err := DecodeEnvironment([]byte(raw)); err == nil {
			t.Fatal("invalid shape settings accepted")
		}
	}
	e := shapeFixture("stored-shapes", 96, 64)
	if err := e.ValidateLandStructures(); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var restored Environment
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(e.ContinentalStructures, restored.ContinentalStructures) || !reflect.DeepEqual(e.Mask, restored.Mask) {
		t.Fatal("stored shapes changed")
	}
	if err = restored.ValidateLandStructures(); err != nil {
		t.Fatal(err)
	}
	restored.ContinentalStructures[0].Spines[0][0].Width = -1
	if restored.ValidateLandStructures() == nil {
		t.Fatal("invalid stored spine accepted")
	}
	restored.ContinentalStructures = nil
	if restored.ValidateLandStructures() != nil {
		t.Fatal("legacy world rejected")
	}
	base := append([]int(nil), e.Mask...)
	e.Options.LandShape = &LandShapeOptions{}
	e.Mask = make([]int, len(e.Mask))
	e.generateContinentalMask(e.Points, noise(Seed(e.Options.Seed)))
	if reflect.DeepEqual(base, e.Mask) {
		t.Fatal("deformation settings have no physical effect")
	}
}

func TestOrganicBasinsRespectMarineConnectivity(t *testing.T) {
	for _, kind := range []string{"enclosed-basin", "crescent"} {
		const size = 96
		e := &Environment{Options: EnvironmentOptions{Columns: size, Rows: size}, Mask: make([]int, size*size), Fields: map[string][]float64{}}
		e.Fields["elevation"] = make([]float64, size*size)
		shape := makeLandmass(71, kind, [2]float64{48, 48}, 22, 0, LandShapeOptions{})
		for i := range e.Mask {
			e.Fields["elevation"][i] = -150
			if shape.field(float64(i%size), float64(i/size)) > 0 {
				e.Mask[i] = 1
				e.Fields["elevation"][i] = 150
			}
		}
		e.connectOcean()
		center := 48*size + 48
		if kind == "enclosed-basin" && (e.get("ocean", center) != 0 || e.get("waterBody", center) != 0 || e.get("elevation", center) >= 0) {
			t.Fatal("enclosed basin automatically flooded as ocean")
		}
		if kind == "crescent" && (e.get("ocean", center) != 1 || e.get("waterDepth", center) <= 0) {
			t.Fatal("open gulf failed to connect to marine water")
		}
	}
}
