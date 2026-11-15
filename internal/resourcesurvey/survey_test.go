package resourcesurvey

import (
	"encoding/json"
	"math"
	"testing"
)

func testCircle(area float64) Circle {
	return Circle{Longitude: -44.654288, Latitude: 34.081332, RadiusKM: 2 * 6371 * math.Asin(math.Sqrt(area/(4*math.Pi*6371*6371))), PlanetRadiusKM: 6371}
}
func testObservation(id string, c Circle, year *int64, value float64, blobs map[string][]byte) Observation {
	m := Manifest{Version: Version, Level: Level(c), Cells: map[string]string{}, Resources: map[string]string{}, Report: Put(blobs, map[string]any{"area": c.Area()})}
	for _, cell := range Cells(c, m.Level) {
		m.Cells[cell.Key()] = Put(blobs, State{Fields: map[string]float64{"ore": value}})
	}
	return Observation{ID: id, Circle: c, Year: year, RecordedAt: int64(len(blobs)), Manifest: m}
}
func TestOverlappingCircleCDCAndBackdatedYears(t *testing.T) {
	blobs := map[string][]byte{}
	y10, y12, y15, y9 := int64(10), int64(12), int64(15), int64(9)
	a := testObservation("300", testCircle(300), &y10, 10, blobs)
	b := testObservation("400", testCircle(400), &y12, 10, blobs)
	shared := 0
	for k, h := range a.Manifest.Cells {
		if b.Manifest.Cells[k] == h {
			shared++
		}
	}
	if shared < 50 {
		t.Fatal("growing a circle did not reuse geographic sample identities", shared)
	}
	c := testObservation("150", testCircle(150), &y15, 30, blobs)
	old := testObservation("backdated", testCircle(400), &y9, 1, blobs)
	old.RecordedAt = 9999
	obs := []Observation{a, b, c, old}
	current := Resolve(b.Circle, obs, nil)
	counts := map[string]int{}
	for _, r := range current {
		counts[r.ReportID]++
		var state State
		if json.Unmarshal(blobs[r.Hash], &state) != nil {
			t.Fatal("bad cell")
		}
		if r.ReportID == "150" && state.Fields["ore"] != 30 {
			t.Fatal("changed state lost")
		}
	}
	if counts["150"] == 0 || counts["400"] == 0 || counts["backdated"] > 0 || counts["300"] > 0 {
		t.Fatal("overlap or chronology wrong", counts)
	}
	for _, r := range Resolve(b.Circle, obs, &y12) {
		if r.ReportID != "400" {
			t.Fatal("historical query used future/newly inserted observation", r.ReportID)
		}
	}
	undated := testObservation("undated", b.Circle, nil, 99, blobs)
	for _, r := range Resolve(b.Circle, append(obs, undated), &y15) {
		if r.ReportID == "undated" {
			t.Fatal("undated observation invented a historical date")
		}
	}
	// Removing a retained report reveals older knowledge instead of a live read.
	for _, r := range Resolve(b.Circle, []Observation{a, b}, nil) {
		if r.ReportID != "400" {
			t.Fatal("removed report remained in knowledge")
		}
	}
}
func TestSphericalGridCoverageBudgetAndArea(t *testing.T) {
	total := 0.
	level := 4
	n := 1 << level
	for f := 0; f < 6; f++ {
		for y := 0; y < n; y++ {
			for x := 0; x < n; x++ {
				c := Cell{level, f, x, y}
				lon, lat := c.Center()
				if At(level, lon, lat) != c {
					t.Fatal("cell roundtrip", c)
				}
				total += c.Area(6371)
			}
		}
	}
	if math.Abs(total-4*math.Pi*6371*6371) > 1e-5 {
		t.Fatal("sphere has missing or double-counted area", total)
	}
	for _, lat := range []float64{-90, -89.999, 0, 34, 89.999, 90} {
		for _, lon := range []float64{-180, -45, 0, 179.999} {
			for _, radius := range []float64{.05, 10, 5000} {
				c := Circle{lon, lat, radius, 6371}
				l := Level(c)
				cells := Cells(c, l)
				if len(cells) == 0 || len(cells) > 8192 {
					t.Fatal("unbounded grid", c, len(cells))
				}
				center := At(l, lon, lat).Key()
				found := false
				for _, cell := range cells {
					if cell.Key() == center {
						found = true
					}
				}
				if !found {
					t.Fatal("circle center omitted at seam or pole", c)
				}
			}
		}
	}
}
func TestManifestRejectsMissingAndForgedCellState(t *testing.T) {
	blobs := map[string][]byte{}
	o := testObservation("a", testCircle(300), nil, 2, blobs)
	if err := o.Manifest.Validate(blobs); err != nil {
		t.Fatal(err)
	}
	for _, h := range o.Manifest.Cells {
		delete(blobs, h)
		break
	}
	if o.Manifest.Validate(blobs) == nil {
		t.Fatal("missing state accepted")
	}
}
