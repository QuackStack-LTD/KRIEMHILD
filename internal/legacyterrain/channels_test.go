package terrain

import (
	"encoding/json"
	"testing"
)

func TestVisibleDrainageIsSelectiveAndConnected(t *testing.T) {
	for _, seed := range []string{"KRIEMHILD", "mountain-drainage", "dry-world"} {
		raw, _ := json.Marshal(map[string]any{"columns": 96, "rows": 64, "realism": true, "seed": seed})
		opts, _ := DecodeEnvironment(raw)
		e := BuildEnvironment(opts)
		visible := e.VisibleChannels()
		old, total, mountainOld, mountainNew, mountainCells := 0, 0, 0, 0, 0
		for i := range e.Mask {
			eligible := e.Mask[i] != 0 && e.get("waterBody", i) == 0 && e.get("flow", i) >= 0
			if eligible && e.get("mountainCore", i) > .2 {
				mountainCells++
			}
			if eligible && e.get("accumulation", i) >= .5 {
				old++
				if e.get("mountainCore", i) > .2 {
					mountainOld++
				}
			}
			if !visible[i] {
				continue
			}
			total++
			if e.get("mountainCore", i) > .2 {
				mountainNew++
			}
			j := int(e.get("flow", i))
			if j >= 0 && e.get("waterBody", j) == 0 && !visible[j] {
				t.Fatal("channel terminates before its receiving water", seed, i)
			}
		}
		// The denser network can retain more than half the legacy runoff
		// paths, but must still exclude at least a third of that blanket.
		if total == 0 || total*3 >= old*2 {
			t.Fatalf("excessive river coverage %s: %d / %d", seed, total, old)
		}
		// Most mountain slopes must remain free of visible channels. Measure
		// actual mountain area, not an obsolete discharge-only selection.
		if mountainCells > 10 && mountainNew*2 >= mountainCells {
			t.Fatal("mountain slopes still over-covered", seed, mountainNew, mountainCells)
		}
		if mountainOld > 100 && mountainNew*50 < mountainOld {
			t.Fatal("mountain tributaries were suppressed too heavily", seed, mountainNew, mountainOld)
		}
		model := NewDetailModel(e)
		for _, f := range model.Features {
			if f.Kind == "river" && (f.Discharge <= 0 || f.NetworkID == "") {
				t.Fatal("river geometry has no supplied hydrological identity")
			}
		}
		t.Logf("%s: visible reaches %d -> %d; mountain reaches %d -> %d", seed, old, total, mountainOld, mountainNew)
	}
}
