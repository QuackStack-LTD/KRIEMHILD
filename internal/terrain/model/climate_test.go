package model

import "testing"

func TestClimateVersionCompatibility(t *testing.T) {
	for _, generator := range []string{"continuous-1", "continuous-2", Version} {
		for _, version := range []int{0, 1, 2, 3, 4} {
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
