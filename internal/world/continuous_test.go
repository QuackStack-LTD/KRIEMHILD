package world

import (
	"encoding/json"
	"fmt"
	"kriemhild/internal/legacyterrain"
	"testing"
)

func TestContinuousLandscapeHistoryRoundTrip(t *testing.T) {
	s := New(192, 96)
	s.Header.Version = 2
	o := Operation{ID: "local-edit", Kind: "raise", Layer: "elevation", Center: [2]float64{30, 20}, Radius: 2, Amount: 200}
	if _, err := s.AddOperation(o); err != nil {
		t.Fatal(err)
	}
	p := terrain.DetailPoint{Elevation: 100}
	if Apply(p, 30, 20, s.Ops(Bounds{30, 20, 0, 0})).Elevation != 300 {
		t.Fatal("edit missing")
	}
	outside := Apply(p, 50, 50, s.Ops(Bounds{50, 50, 0, 0}))
	if outside != p {
		t.Fatal("nonlocal effect")
	}
	if _, err := s.Undo(false); err != nil {
		t.Fatal(err)
	}
	if Apply(p, 30, 20, s.Ops(Bounds{30, 20, 0, 0})).Elevation != 100 {
		t.Fatal("undo missing")
	}
	data, _ := json.Marshal(s)
	var restored State
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if err := restored.Validate(192, 96); err != nil {
		t.Fatal(err)
	}
	if _, err := restored.Undo(true); err != nil {
		t.Fatal(err)
	}
	if Apply(p, 30, 20, restored.Ops(Bounds{30, 20, 0, 0})).Elevation != 300 {
		t.Fatal("redo lost on restart")
	}
}

func TestContinuousSettlementBatchAndFamilyMove(t *testing.T) {
	s := New(192, 96)
	s.Header.Version = 2
	root := Entity{ID: "town", Name: "Town", Kind: "town", Layer: "settlements", Geometry: "polygon", Points: [][2]float64{{10, 10}, {12, 10}, {12, 12}}, Color: "#aabbcc"}
	entities := []Entity{root}
	for i := 0; i < 120; i++ {
		entities = append(entities, Entity{ID: fmt.Sprintf("house-%d", i), Name: "House", Kind: "house", Layer: "buildings", ParentID: "town", Geometry: "polygon", Points: [][2]float64{{10.1, 10.1}, {10.2, 10.1}, {10.2, 10.2}}, Color: "#aabbcc"})
	}
	if err := s.PutEntities(entities); err != nil {
		t.Fatal(err)
	}
	if s.Header.Revision != 1 || len(s.Header.History) != 1 {
		t.Fatal("settlement is not one undo action")
	}
	if _, err := s.Undo(false); err != nil {
		t.Fatal(err)
	}
	for _, e := range s.Entities {
		if !e.Deleted {
			t.Fatal("partial settlement undo")
		}
	}
	if _, err := s.Undo(true); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveFamily("town", 2, 3); err != nil {
		t.Fatal(err)
	}
	if s.Entities["house-119"].Points[0] != [2]float64{12.1, 13.1} {
		t.Fatal("child did not move")
	}
	data, _ := json.Marshal(s)
	var restored State
	if json.Unmarshal(data, &restored) != nil {
		t.Fatal("decode failed")
	}
	if err := restored.Validate(192, 96); err != nil {
		t.Fatal(err)
	}
	if _, err := restored.Undo(false); err != nil {
		t.Fatal(err)
	}
	if restored.Entities["house-119"].Points[0] != [2]float64{10.1, 10.1} {
		t.Fatal("family undo lost on restart")
	}
	for i := range restored.Header.Layers {
		if restored.Header.Layers[i].ID == "buildings" {
			restored.Header.Layers[i].Locked = true
		}
	}
	revision := restored.Header.Revision
	if err := restored.MoveFamily("town", 1, 0); err == nil {
		t.Fatal("moved locked child")
	}
	if restored.Header.Revision != revision || restored.Entities["town"].Points[0] != root.Points[0] {
		t.Fatal("failed move mutated state")
	}
}
