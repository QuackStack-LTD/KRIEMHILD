package world

import (
	"encoding/json"
	"fmt"
	"kriemhild/internal/terrain"
	"math"
	"reflect"
	"testing"
)

func entity(id string, x, y float64) Entity {
	return Entity{ID: id, Name: id, Kind: "house", Layer: "buildings", Geometry: "point", Points: [][2]float64{{x, y}}, Color: "#aabbcc"}
}
func TestHierarchyHistoryLayersAndIncrementalParts(t *testing.T) {
	s := New(100, 100)
	parent := entity("capital", 10, 10)
	child := entity("castle", 11, 11)
	child.ParentID = parent.ID
	for _, e := range []Entity{parent, child} {
		if _, err := s.PutEntity(e); err != nil {
			t.Fatal(err)
		}
	}
	parent.ParentID = child.ID
	if _, err := s.PutEntity(parent); err == nil {
		t.Fatal("accepted hierarchy cycle")
	}
	parent.ParentID = ""
	parent.Deleted = true
	if _, err := s.PutEntity(parent); err == nil {
		t.Fatal("deleted nonempty parent")
	}
	s.Saved()
	child.Points[0] = [2]float64{80, 80}
	if _, err := s.PutEntity(child); err != nil {
		t.Fatal(err)
	}
	if len(s.Parts(false)) != 2 {
		t.Fatal("rewrote unrelated entities")
	}
	got, _ := s.Query(Bounds{0, 0, 20, 20}, 8)
	if len(got) != 1 {
		t.Fatal("move did not update index", got)
	}
	if _, err := s.Undo(false); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Query(Bounds{0, 0, 20, 20}, 8)
	if len(got) != 2 {
		t.Fatal("undo did not restore spatial position")
	}
	if _, err := s.Undo(true); err != nil {
		t.Fatal(err)
	}
	layers := append([]Layer(nil), s.Header.Layers...)
	for i := range layers {
		if layers[i].ID == "buildings" {
			layers[i].Locked = true
			layers[i].Visible = false
		}
	}
	if dirty, err := s.SetLayers(layers); err != nil || dirty != nil {
		t.Fatal("entity layer edit invalidated terrain", err)
	}
	if _, err := s.PutEntity(child); err == nil {
		t.Fatal("edited locked layer")
	}
	got, _ = s.Query(Bounds{0, 0, 99, 99}, 8)
	if len(got) != 0 {
		t.Fatal("hidden entity rendered")
	}
	b, _ := json.Marshal(s)
	restored := New(100, 100)
	if err := json.Unmarshal(b, restored); err != nil {
		t.Fatal(err)
	}
	if err := restored.Validate(100, 100); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.Header, s.Header) {
		t.Fatal("history changed on round trip")
	}
	layers[0].Order = layers[1].Order
	if _, err := s.SetLayers(layers); err == nil {
		t.Fatal("duplicate order accepted")
	}
}
func TestIndexViewportLODMoveAndLimit(t *testing.T) {
	q := NewIndex(1000, 1000)
	for i := 0; i < 10000; i++ {
		q.Put(fmt.Sprint(i), Bounds{float64(i%100) * 10, float64(i/100) * 10, 1, 1}, 4)
	}
	if ids, _ := q.Query(Bounds{0, 0, 1000, 1000}, 0, 2000); len(ids) != 0 {
		t.Fatal("buildings leaked into world LOD")
	}
	ids, more := q.Query(Bounds{0, 0, 25, 25}, 4, 2000)
	if len(ids) != 9 || more {
		t.Fatal("incorrect viewport", len(ids), more)
	}
	ids, more = q.Query(Bounds{0, 0, 1000, 1000}, 4, 2000)
	if len(ids) != 2000 || !more {
		t.Fatal("query not bounded")
	}
	q.Put("0", Bounds{900, 900, 1, 1}, 0)
	q.Remove("101")
	ids, _ = q.Query(Bounds{0, 0, 15, 15}, 4, 20)
	if len(ids) != 2 {
		t.Fatal("stale index records", ids)
	}
}
func TestSurfaceOperationsPreserveBaseAndUndo(t *testing.T) {
	s := New(65, 65)
	p := terrain.DetailPoint{Elevation: 100, Vegetation: 1}
	o := Operation{ID: "crater", Kind: "crater", Layer: "elevation", Center: [2]float64{16, 16}, Radius: 4, Amount: 80}
	if _, err := s.AddOperation(o); err != nil {
		t.Fatal(err)
	}
	center := Apply(p, 16, 16, s.Ops(Bounds{16, 16, 0, 0}))
	if math.Abs(center.Elevation-20) > 1e-8 || center.Vegetation != 0 || center.Rock != 1 {
		t.Fatal("crater is not physical", center)
	}
	outside := Apply(p, 21, 16, s.Ops(Bounds{21, 16, 0, 0}))
	if outside != p {
		t.Fatal("nonlocal modification")
	}
	if _, err := s.Undo(false); err == nil {
		t.Fatal("terrain was added to object undo history")
	}
	if p.Elevation != 100 {
		t.Fatal("base sample was modified")
	}
	o = Operation{ID: "lake", Kind: "water", Layer: "water", Center: [2]float64{16, 16}, Radius: 2, Target: 50}
	s.AddOperation(o)
	center = Apply(p, 16, 16, s.Ops(Bounds{16, 16, 0, 0}))
	if center.WaterDepth != 30 || center.WaterBody == 0 {
		t.Fatal("water depth ignores edited terrain", center)
	}
	o.ID = "drain"
	o.Kind = "drain"
	s.AddOperation(o)
	center = Apply(p, 16, 16, s.Ops(Bounds{16, 16, 0, 0}))
	if center.WaterDepth != 0 || center.WaterBody != 0 {
		t.Fatal("draining failed")
	}
}
func TestEditedTileParentAnchorsAndNeighborSeams(t *testing.T) {
	s := New(65, 65)
	s.AddOperation(Operation{ID: "raise", Kind: "raise", Layer: "elevation", Center: [2]float64{16, 8}, Radius: 3, Amount: 100})
	makeTile := func(l, x int, step float64) *terrain.DetailTile {
		tile := &terrain.DetailTile{Level: l, X: x, Y: 0, Step: step, Points: make([]terrain.DetailPoint, 1089)}
		for i := range tile.Points {
			tile.Points[i].Elevation = 100
		}
		return tile
	}
	parent, left, right := makeTile(0, 0, 1), makeTile(1, 0, .5), makeTile(1, 1, .5)
	s.ApplyTile(parent, nil)
	s.ApplyTile(left, parent)
	s.ApplyTile(right, parent)
	for y := 0; y < 33; y++ {
		if left.Points[y*33+32].Elevation != right.Points[y*33].Elevation {
			t.Fatal("crack across tile boundary")
		}
		for axis := 0; axis < 2; axis++ {
			if left.Points[y*33+32].Gradient[axis] != right.Points[y*33].Gradient[axis] {
				t.Fatal("normal seam across edited tile boundary")
			}
		}
	}
	for y := 0; y < 33; y += 2 {
		for x := 0; x < 33; x += 2 {
			p := left.Points[y*33+x]
			if p.Parent != p.Elevation || p.Elevation != parent.Points[y/2*33+x/2].Elevation {
				t.Fatal("parent anchor changed")
			}
		}
	}
}
func TestMalformedHistoryRejected(t *testing.T) {
	s := New(32, 32)
	s.PutEntity(entity("city", 2, 2))
	bad := entity("city", 999, 2)
	s.Header.History[0].Before = raw(bad)
	if s.Validate(32, 32) == nil {
		t.Fatal("invalid geometry in undo history accepted")
	}
}
