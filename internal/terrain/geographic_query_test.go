package terrain

import (
	"math"
	"testing"
)

func TestNaturalQueryKilometresAndSphericalGeometry(t *testing.T) {
	e := BuildEnvironment(realismOptions(t, `{"columns":32,"rows":16,"seed":"metric-inspection"}`))
	point := [2]float64{1, 7.5}
	e.Resources.Occurrences = []ResourceOccurrence{{ID: "test-point", Type: "gold", Abundance: .5, Geometry: NaturalGeometry{Point: &point}}}
	index := NewNaturalIndex(e)
	x, y := 0., 7.5
	distance := e.PointDistanceKM(x, y, point[0], point[1])
	expected := 2 * math.Pi * 6371 / 32
	if math.Abs(distance-expected) > 1e-6 {
		t.Fatal("equatorial grid scale lost", distance, expected)
	}
	if q := index.QueryKM(x, y, distance-1); len(q.Goods) != 0 {
		t.Fatal("query radius treated as cells", q)
	}
	q := index.QueryKM(x, y, distance+1)
	if q.DistanceUnits != "km" || len(q.Goods) != 1 || math.Abs(q.Goods[0].Distance-distance) > .01 {
		t.Fatal("point distance not returned in kilometres", q)
	}
	if e.PointDistanceKM(0, 1, 1, 1) >= distance*.35 {
		t.Fatal("polar longitude shrinkage ignored")
	}
	if math.Abs(e.PointDistanceKM(31, y, 0, y)-distance) > 1e-6 {
		t.Fatal("planet seam distance takes the long way")
	}
	e.Resources.Occurrences[0].Geometry = NaturalGeometry{Cells: []int{8 * 32}}
	index = NewNaturalIndex(e)
	if q = index.QueryKM(.2, 8.1, 0); len(q.Goods) != 1 || q.Goods[0].Location != "here" {
		t.Fatal("region incorrectly measured to cell centre", q)
	}
	// The adjacent cell across the seam is near; its centre is not one full
	// planet away, and the distance is to the region edge, not its centre.
	if q = index.QueryKM(31, 8, distance*.6); len(q.Goods) != 1 || q.Goods[0].Distance >= distance*.6 {
		t.Fatal("wrapped region omitted", q)
	}
}
