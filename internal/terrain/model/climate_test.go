package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestClimateVersionCompatibility(t *testing.T) {
	for _, generator := range []string{"continuous-1", "continuous-2", Version} {
		for _, version := range []int{0, 1, 2, 3, 4, 5} {
			spec := DefaultSpec()
			spec.Version = generator
			spec.ClimateVersion = version
			valid := version <= MaxClimateVersion && (version < 2 || generator == Version)
			if (spec.Validate() == nil) != valid {
				t.Fatalf("generator %s climate %d compatibility", generator, version)
			}
		}
	}
}

func TestHistoricalVersionZeroSerializationIsExplicit(t *testing.T) {
	spec := DefaultSpec()
	spec.ClimateVersion = 0
	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"climateVersion":0`) {
		t.Fatal("explicit zero lost during request/export serialization")
	}
	var old Spec
	if err := json.Unmarshal([]byte(`{"version":"continuous-3","seed":"historical"}`), &old); err != nil || old.ClimateVersion != 0 {
		t.Fatal("historical omitted version became new default")
	}
}
