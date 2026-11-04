package hydrology

import (
	"encoding/json"
	"kriemhild/internal/terrain/model"
	"math"
	"testing"
)

func river(id, destination string, points ...model.Point) model.River {
	r := model.River{ID: id, Destination: destination, Path: points, Basin: 1}
	for i := range points {
		r.Surface = append(r.Surface, 100-float64(i))
		r.Bed = append(r.Bed, 95-float64(i))
		r.Discharge = append(r.Discharge, float64(i+1))
		r.Width = append(r.Width, 5)
	}
	return r
}
func TestConnectedTopologySplitsTrunksAtConfluencesAndKeepsClosedBasins(t *testing.T) {
	a, b, c := model.Point{0, 0}, model.Point{.1, 0}, model.Point{.2, 0}
	p := &model.Planet{Rivers: []model.River{river("main", "ocean", a, b, c), river("tributary", "main", model.Point{.1, .1}, b), river("closed", "basin", model.Point{-.3, 0}, model.Point{-.2, 0})}}
	before, _ := json.Marshal(p)
	g := Build(p)
	if len(g.Issues) > 0 || len(g.Reaches) != 4 || g.Counts["confluences"] != 1 || g.Counts["sources"] != 3 {
		t.Fatalf("bad graph: %+v", g)
	}
	after, _ := json.Marshal(p)
	if string(before) != string(after) {
		t.Fatal("derivation mutated source")
	}
	if Build(p).Revision != g.Revision {
		t.Fatal("unstable graph revision")
	}
}
func TestLakesAndLongitudeSeamAreGeographicConnections(t *testing.T) {
	points := []model.Point{{math.Pi - .03, 0}, {math.Pi - .01, 0}, {-math.Pi + .01, 0}, {-math.Pi + .03, 0}}
	p := &model.Planet{Rivers: []model.River{river("main", "ocean", points...)}, Lakes: []model.Lake{{Center: model.Point{math.Pi, 0}, Radius: .02, Level: 99}}}
	g := Build(p)
	if len(g.Issues) > 0 || g.Counts["lake-nodes"] == 0 || len(g.Reaches) < 2 {
		t.Fatalf("lost lake/seam connection: %+v", g)
	}
	for _, r := range g.Reaches {
		if r.From == r.To {
			t.Fatal("degenerate reach")
		}
	}
}
func TestDiagnosticsDoNotSilentlyJoinDisconnectedOrUphillGeometry(t *testing.T) {
	main := river("main", "ocean", model.Point{0, 0}, model.Point{.2, 0})
	bad := river("bad", "main", model.Point{.1, .1}, model.Point{.1, 0}) // Not a stored junction.
	bad.Surface[1] = 200
	p := &model.Planet{Rivers: []model.River{main, bad}}
	g := Build(p)
	if g.Counts["mismatched-confluence"] != 1 || g.Counts["uphill-flow"] != 1 {
		t.Fatal(g.Counts)
	}
	bad.Destination = "missing"
	p.Rivers[1] = bad
	if Build(p).Counts["disconnected-reach"] != 1 {
		t.Fatal("missing destination accepted")
	}
}
