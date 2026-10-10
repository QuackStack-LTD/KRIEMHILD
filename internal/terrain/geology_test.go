package terrain

import (
	"math"
	"testing"
)

func TestPassiveContinentalCrossSection(t *testing.T) {
	o, _ := DecodeEnvironment([]byte(`{"columns":80,"rows":48}`))
	e := &Environment{Options: o, Mask: make([]int, 80*48), Heights: make([]int, 80*48), Fields: map[string][]float64{}}
	for _, name := range append(append([]string{}, FieldNames...), SurfaceFields...) {
		e.Fields[name] = make([]float64, len(e.Mask))
	}
	for i := range e.Mask {
		x := float64(i % 80)
		if x >= 24 {
			e.Mask[i] = 1
		}
		e.set("highland", i, math.Exp(-math.Pow((x-55)/14, 2)))
		e.set("mountainCore", i, math.Exp(-math.Pow((x-55)/7, 2)))
	}
	e.buildGeologicalSurface(func(x, y float64) float64 { return .5 }, nil)
	e.limitSurfaceGradients()
	z := func(x int) float64 { return e.get("elevation", 24*80+x) }
	// Deep basin -> slope -> gentle shelf -> coast -> lowland -> mountain mass.
	if !(z(10) < -3000 && z(19) < -150 && z(22) > -100 && z(23) > -40 && z(24) < 100 && z(27) < 500 && z(55) > 3000) {
		t.Fatalf("missing continental profile: %v", []float64{z(10), z(19), z(22), z(23), z(24), z(27), z(55)})
	}
	if z(48) < 1800 || z(62) < 1800 {
		t.Fatal("mountain lacks a broad structural base")
	}
	if math.Abs(z(22)-z(23)) > 80 {
		t.Fatal("shelf is a cliff")
	}
}

func TestGeneratedSurfaceHasSupportedGradients(t *testing.T) {
	for _, seed := range []string{"KRIEMHILD", "1", "2", "3", "islands"} {
		o, _ := DecodeEnvironment([]byte(`{"columns":160,"rows":100}`))
		o.Seed = seed
		e := BuildEnvironment(o)
		coasts, gentle, lowlands, land := 0, 0, 0, 0
		for i, z := range e.Fields["elevation"] {
			if e.Mask[i] != 0 {
				land++
				if z < 600 {
					lowlands++
				}
			}
			for _, j := range nb(i, o.Columns, o.Rows) {
				gi, gj := e.get("geology", i), e.get("geology", j)
				limit := 654.
				if (gi == 2 || gi == 6) && (gj == 2 || gj == 6) {
					limit = 1104
				}
				if math.Abs(z-e.get("elevation", j)) > limit {
					t.Fatalf("%s unsupported step at %d -> %d: z=%g/%g geology=%g/%g lake=%g/%g", seed, i, j, z, e.get("elevation", j), gi, gj, e.get("lake", i), e.get("lake", j))
				}
				if e.Mask[i] != 0 && e.get("ocean", j) == 1 && gi != 2 && gi != 6 {
					coasts++
					if z-e.get("elevation", j) < 200 {
						gentle++
					}
				}
			}
		}
		if gentle*10 < coasts*9 {
			t.Fatalf("%s passive coasts too steep: %d/%d gentle", seed, gentle, coasts)
		}
		if lowlands*3 < land {
			t.Fatalf("%s lacks substantial lowlands", seed)
		}
	}
}
