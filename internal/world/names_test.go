package world

import "testing"

func TestCanonicalNamesSurvivePendingGeometryAndUndo(t *testing.T) {
	s := New(48, 32)
	e := Entity{ID: "city", Name: "Old name", Kind: "city", Layer: "settlements", Geometry: "polygon", Points: [][2]float64{{10, 10}, {11, 10}, {10, 11}}, Color: "#112233", RecordStatus: "published"}
	if _, err := s.PutEntity(e); err != nil {
		t.Fatal(err)
	}
	e.Points = [][2]float64{{12, 12}, {13, 12}, {12, 13}}
	if _, err := s.PutEntity(e); err != nil {
		t.Fatal(err)
	}
	named := e
	named.Name = "Canonical name"
	named.Points = [][2]float64{{10, 10}, {11, 10}, {10, 11}}
	s.RefreshRepresentations(map[string]Entity{e.ID: named})
	if s.Entities[e.ID].Name != named.Name || s.Entities[e.ID].Points[0][0] != 12 {
		t.Fatal("name refresh discarded a pending move")
	}
	if _, err := s.Undo(false); err != nil {
		t.Fatal(err)
	}
	if s.Entities[e.ID].Name != named.Name || s.Entities[e.ID].Points[0][0] != 10 {
		t.Fatal("undo resurrected an old canonical name")
	}
}

func TestNameOnlyRefreshKeepsRasterRevisionGeometryAndUndo(t *testing.T) {
	s := New(48, 32)
	e := Entity{ID: "city", Name: "Old", LabelName: "Old", Kind: "city", Layer: "settlements", Geometry: "polygon", Points: [][2]float64{{5, 5}, {6, 5}, {5, 6}}, Color: "#112233", RecordStatus: "published"}
	if _, err := s.PutEntity(e); err != nil {
		t.Fatal(err)
	}
	e.Points = [][2]float64{{8, 8}, {9, 8}, {8, 9}}
	if _, err := s.PutEntity(e); err != nil {
		t.Fatal(err)
	}
	revision := s.Header.Revision
	named := e
	named.Name = "New"
	named.LabelName = "New"
	named.Points = [][2]float64{{99, 99}}
	s.RefreshNames(map[string]Entity{"city": named})
	if s.Header.Revision != revision || s.Entities["city"].Points[0][0] != 8 || s.Entities["city"].Name != "New" {
		t.Fatal("rename replaced geometry or raster revision")
	}
	if _, err := s.Undo(false); err != nil {
		t.Fatal(err)
	}
	if s.Entities["city"].Name != "New" || s.Entities["city"].Points[0][0] != 5 {
		t.Fatal("undo lost the canonical name")
	}
}
