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
		old, total, mountainOld, mountainNew := 0, 0, 0, 0
		for i := range e.Mask {
			eligible := e.Mask[i] != 0 && e.get("waterBody", i) == 0 && e.get("flow", i) >= 0
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
		if total == 0 || total*2 >= old {
			t.Fatalf("insufficient river reduction %s: %d / %d", seed, total, old)
		}
		if mountainOld > 10 && mountainNew*2 >= mountainOld {
			t.Fatal("mountain slopes still over-covered", seed, mountainNew, mountainOld)
		}
		model := NewDetailModel(e)
		for _, f := range model.Features {
			if f.Discharge < 2 {
				t.Fatal("synthetic runoff tributary survived")
			}
		}
		t.Logf("%s: visible reaches %d -> %d; mountain reaches %d -> %d", seed, old, total, mountainOld, mountainNew)
	}
}
