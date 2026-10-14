package terrain

import "testing"

func TestLegacyVillagePaletteCannotGenerateSettlements(t *testing.T) {
	c, err := DecodeConfig([]byte(`{"continents":[],"climates":[],"types":[{"id":"grass","neighbors":["grass","village"]},{"id":"village","weight":1000,"neighbors":["grass","village"]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	r, err := Compile(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, physical := range []bool{false, true} {
		var masks []uint32
		if physical {
			masks = make([]uint32, 256)
			for i := range masks {
				masks[i] = 3
			}
		}
		s := NewSolver(r, Options{Width: 16, Height: 16, Seed: 42}, masks)
		for k := 0; k < 1000 && s.Status == "running"; k++ {
			s.Step()
		}
		s.Cleanup()
		if s.Status != "done" {
			t.Fatal(s.Status)
		}
		for i := range s.Dom {
			if s.TypeAt(i) != 0 {
				t.Fatal("generated village", physical, i)
			}
		}
	}
}
