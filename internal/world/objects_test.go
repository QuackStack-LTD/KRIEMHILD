package world

import "testing"

func TestObjectOnlyUndoKeepsTerrainAndLayerChanges(t *testing.T) {
	s := New(32, 32)
	if _, err := s.PutEntity(entity("house", 8, 8)); err != nil {
		t.Fatal(err)
	}
	op := Operation{ID: "crater", Kind: "crater", Layer: "elevation", Center: [2]float64{8, 8}, Radius: 2, Amount: 100}
	if _, err := s.AddOperation(op); err != nil {
		t.Fatal(err)
	}
	layers := append([]Layer(nil), s.Header.Layers...)
	layers[0].Visible = false
	s.SetLayers(layers)
	if len(s.Header.History) != 1 {
		t.Fatal("non-object action entered undo history")
	}
	if _, err := s.Undo(false); err != nil {
		t.Fatal(err)
	}
	if !s.Entities["house"].Deleted || s.Operations["crater"].Deleted || s.Header.Layers[0].Visible {
		t.Fatal("undo changed terrain/layers")
	}
	op.ID = "raise"
	op.Kind = "raise"
	s.AddOperation(op)
	if _, err := s.Undo(true); err != nil {
		t.Fatal("terrain action discarded object redo", err)
	}
	if s.Entities["house"].Deleted {
		t.Fatal("redo failed")
	}
	s.PutEntity(s.Entities["house"])
	if len(s.Header.History) != 1 {
		t.Fatal("unchanged entity created a redundant undo entry")
	}
}

func TestObjectGeometryEnforcedAndDefaultBlack(t *testing.T) {
	s := New(32, 32)
	for _, tc := range []struct {
		layer, kind string
		allowed     map[string]bool
	}{
		{"settlements", "village", map[string]bool{"polygon": true}},
		{"roads", "road", map[string]bool{"line": true}},
		{"buildings", "house", map[string]bool{"point": true, "polygon": true}},
		{"buildings", "factory", map[string]bool{"point": true}},
		{"buildings", "landmark", map[string]bool{"point": true}},
		{"buildings", "custom-structure", map[string]bool{"point": true, "line": true, "polygon": true}},
	} {
		for i, shape := range []string{"point", "line", "polygon"} {
			e := entity(tc.kind+shape, 3, 3)
			e.Layer = tc.layer
			e.Kind = tc.kind
			e.Geometry = shape
			e.Color = ""
			e.Points = [][2]float64{{3, 3}, {4, 3}, {4, 4}}[:i+1]
			_, err := s.PutEntity(e)
			if (err == nil) != tc.allowed[shape] {
				t.Fatalf("%s %s: %v", tc.kind, shape, err)
			}
			if err == nil && s.Entities[e.ID].Color != "#000000" {
				t.Fatal("missing black default")
			}
		}
	}
}

func TestLegacyMixedHistoryMigratesWithoutChangingLandscape(t *testing.T) {
	s := New(32, 32)
	s.PutEntity(entity("house", 8, 8))
	o := Operation{ID: "crater", Kind: "crater", Layer: "elevation", Center: [2]float64{8, 8}, Radius: 2, Amount: 100}
	s.AddOperation(o)
	o = s.Operations[o.ID]
	before := o
	before.Deleted = true
	b := o.Bounds()
	s.Header.History = append(s.Header.History, Change{"operation/" + o.ID, raw(before), raw(o), &b})
	s.Header.Cursor = 2
	if err := s.Validate(32, 32); err != nil {
		t.Fatal(err)
	}
	if len(s.Header.History) != 1 || s.Header.Cursor != 1 || s.Operations[o.ID].Deleted {
		t.Fatal("history migration changed existing terrain")
	}
}
