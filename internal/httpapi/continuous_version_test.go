package httpapi

import (
	"fmt"
	"kriemhild/internal/terrain/model"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTerrainRequestDefaultsPreserveExplicitClimateVersions(t *testing.T) {
	for _, version := range []int{0, 1, 2, 3, 4} {
		body := fmt.Sprintf(`{"spec":{"seed":"compatibility","climateVersion":%d}}`, version)
		spec, err := decodeTerrainJobSpec(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(body)))
		if err != nil || spec.ClimateVersion != version {
			t.Fatalf("explicit version %d: %+v %v", version, spec, err)
		}
	}
	spec, err := decodeTerrainJobSpec(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(`{"spec":{"seed":"new-world"}}`)))
	if err != nil || spec.ClimateVersion != model.ClimateVersion || spec.Rainfall != 1.65 {
		t.Fatalf("new defaults: %+v %v", spec, err)
	}
	for _, version := range []string{"continuous-1", "continuous-2"} {
		body := fmt.Sprintf(`{"spec":{"seed":"historical","version":%q,"climateVersion":0}}`, version)
		spec, err := decodeTerrainJobSpec(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(body)))
		if err != nil || spec.Version != version {
			t.Fatal("terrain generator silently upgraded", spec, err)
		}
	}
}
