package world

import (
	"encoding/json"
	"testing"
)

func TestRecoverMissingBatchPartsFromSavedCursorWithoutOverwritingExistingData(t *testing.T) {
	s := New(100, 100)
	s.Header.Version = 2
	root := Entity{ID: "town", Name: "Town", Kind: "town", Layer: "settlements", Geometry: "polygon", Points: [][2]float64{{10, 10}, {12, 10}, {12, 12}}, Color: "#aabbcc"}
	house := Entity{ID: "house", Name: "House", Kind: "house", Layer: "buildings", ParentID: "town", Geometry: "point", Points: [][2]float64{{10.1, 10.1}}, Color: "#aabbcc"}
	if err := s.PutEntities([]Entity{root, house}); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveFamily("town", 1, 2); err != nil {
		t.Fatal(err)
	}
	for _, undone := range []bool{false, true} {
		data, _ := json.Marshal(s)
		var next State
		_ = json.Unmarshal(data, &next)
		if undone {
			next.Reindex(100, 100)
			if _, err := next.Undo(false); err != nil {
				t.Fatal(err)
			}
		}
		expected := next.Entities[house.ID]
		delete(next.Entities, house.ID)
		original := next.Entities[root.ID]
		original.Name = "Newer shared profile"
		next.Entities[root.ID] = original
		if err := next.Validate(100, 100); err != nil {
			t.Fatal(err)
		}
		if next.Entities[house.ID].Points[0] != expected.Points[0] || next.Entities[root.ID].Name != original.Name {
			t.Fatal("recovery moved geometry or overwrote an existing part")
		}
		next.MarkRecoveredParts()
		if next.Parts(false)["builder/entity/house"] == nil {
			t.Fatal("recovered object will be lost on the next save")
		}
	}
	// Missing current map representations become tombstones, retaining undo data.
	s.Saved()
	s.RefreshRepresentations(map[string]Entity{})
	if !s.Entities[house.ID].Deleted {
		t.Fatal("removed representation stayed visible")
	}
	if err := s.Validate(100, 100); err != nil {
		t.Fatal("refresh lost undo references", err)
	}
}
