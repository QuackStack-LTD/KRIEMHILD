package biosphere

import "testing"

func TestContinuousWaterBodyIDsUsePhysicalOceanMask(t *testing.T) {
	e := fixture()
	n := len(e.Mask)
	e.Fields["waterBody"] = make([]float64, n)
	for i := range e.Mask {
		e.Mask[i] = 1
		e.Fields["ocean"][i] = 0
		e.Fields["waterDepth"][i] = 0
		e.Fields["river"][i] = 0
		e.Fields["lake"][i] = 0
	}
	// Body 1 can be an inland lake. Several disconnected oceans can have any ID.
	e.Fields["waterBody"][0] = 1
	e.Fields["waterDepth"][0] = 12
	e.Mask[0] = 0
	e.Fields["waterBody"][1] = 4
	e.Fields["waterDepth"][1] = 12
	e.Fields["ocean"][1] = 1
	e.Mask[1] = 0
	habitats, _, err := Habitats(e, nil, "continuous")
	if err != nil {
		t.Fatal(err)
	}
	fresh, marine := false, false
	for _, h := range habitats {
		for _, i := range h.Cells {
			if i == 0 {
				if h.Conditions.Medium == "marine" {
					t.Fatal("inland lake classified as marine")
				}
				fresh = fresh || h.Conditions.Medium == "freshwater"
			}
			if i == 1 {
				marine = marine || h.Conditions.Medium == "marine"
			}
		}
	}
	if !fresh || !marine {
		t.Fatal("fresh and marine habitats not retained")
	}
}
