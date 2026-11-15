package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/storage"
	"math"
	"net/http/httptest"
	"os"
	"testing"
)

// Optional integration check against a consistent, expendable database copy.
// No private World data is embedded in the regression fixture.
func TestResourceReportSavedWorld(t *testing.T) {
	dir, project := os.Getenv("KRIEMHILD_RESOURCE_TEST_DATA"), os.Getenv("KRIEMHILD_RESOURCE_TEST_PROJECT")
	if dir == "" || project == "" {
		t.Skip("requires an expendable database copy and terrain project")
	}
	db, err := storage.Open(context.Background(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	opened := autoRequest(t, s, "POST", "/api/projects/"+project+"/open?view=terrain", `{}`)
	id := opened["id"].(string)
	v := s.sessions[id]
	doc, m, err := db.MapProjection(context.Background(), project)
	if err != nil {
		t.Fatal(err)
	}
	name := os.Getenv("KRIEMHILD_RESOURCE_TEST_SETTLEMENT")
	for _, r := range doc.Representations {
		if r.MapID != m.ID || r.Shape.Name != name || len(r.Shape.Points) == 0 {
			continue
		}
		var x, y float64
		for _, p := range r.Shape.Points {
			x += p[0]
			y += p[1]
		}
		x /= float64(len(r.Shape.Points))
		y /= float64(len(r.Shape.Points))
		lon, lat := x/float64(v.solver.W-1)*360-180, 90-y/float64(v.solver.H-1)*180
		radius := 2 * v.planet.Spec.RadiusKM * math.Asin(math.Sqrt(314/(4*math.Pi*v.planet.Spec.RadiusKM*v.planet.Spec.RadiusKM)))
		for _, center := range [][2]float64{{lon, lat}, {0, 0}} {
			path := fmt.Sprintf("/api/sessions/%s/terrain/resources?longitude=%.12g&latitude=%.12g&radiusKm=%.12g", id, center[0], center[1], radius)
			response := httptest.NewRecorder()
			s.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
			if response.Code != 200 {
				t.Fatal(response.Code, response.Body.String())
			}
			var report terrainResourceReport
			if err := json.Unmarshal(response.Body.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if math.Abs(report.AreaKM2-314) > .0001 {
				t.Fatal("wrong observed area", report.AreaKM2)
			}
			t.Logf("314 km² circle at %.6f, %.6f: %d bytes", center[0], center[1], response.Body.Len())
		}
		return
	}
	t.Fatal("settlement fixture not found")
}
