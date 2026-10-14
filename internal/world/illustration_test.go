package world

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestIllustrationSceneHistoryAndPersistence(t *testing.T) {
	s := New(192, 96)
	s.Header.Version = 2
	a := Illustration{Version: 1, ID: "country-atlas", Title: "Northern Lands", Scope: "country", Style: "medieval", Bounds: [4]float64{.5, .2, .4, .3}, Layers: []IllustrationLayer{{ID: "labels", Name: "Labels", Visible: true, Opacity: 1}}, Objects: map[string]IllustrationObject{"label-capital": {ID: "label-capital", Source: "capital", Layer: "labels", Kind: "text", Position: &[2]float64{.7, .3}, Color: "#443322"}}}
	if err := s.PutIllustration(a); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(s)
	var restored State
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if err := restored.Validate(192, 96); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.Header.Illustrations[a.ID], a) {
		t.Fatal("scene lost")
	}
	if _, err := restored.Undo(false); err != nil {
		t.Fatal(err)
	}
	if len(restored.Header.Illustrations) != 0 {
		t.Fatal("undo did not remove first scene")
	}
	if _, err := restored.Undo(true); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.Header.Illustrations[a.ID], a) {
		t.Fatal("redo lost source linkage")
	}
	if len(restored.Entities) != 0 || len(restored.Operations) != 0 {
		t.Fatal("illustration changed geography")
	}
	o := a.Objects["label-capital"]
	o.Kind = "unknown"
	a.Objects[o.ID] = o
	if err := a.Validate(); err == nil {
		t.Fatal("unsupported renderer object accepted")
	}
}
