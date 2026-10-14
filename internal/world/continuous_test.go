package world

import (
	"encoding/json"
	"kriemhild/internal/terrain"
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
