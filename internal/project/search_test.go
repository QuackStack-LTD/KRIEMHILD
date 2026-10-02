package project

import (
	"errors"
	"fmt"
	"testing"
)

func TestSearchPagesStayBoundToQueryAndAgeSnapshot(t *testing.T) {
	s, st := fixture(t)
	records := []Record{}
	for i := 0; i < 205; i++ {
		records = append(records, Record{ID: NewID(), Kind: "entity", Type: "Place", Name: fmt.Sprintf("Harbor %03d", i), Properties: map[string]any{"description": "shoreline", "_internal": "hidden-search-sentinel"}})
	}
	st = apply(t, s, st, Command{Action: "put-many", Records: records})
	first, e := s.SearchPage(st.Age.ID, " HARBOR ", "", 100)
	if e != nil || len(first.Records) != 100 || first.Next == "" {
		t.Fatalf("first page: %+v %v", first, e)
	}
	second, e := s.SearchPage(st.Age.ID, "harbor", first.Next, 100)
	if e != nil || len(second.Records) != 100 || second.Next == "" {
		t.Fatalf("second page: %+v %v", second, e)
	}
	third, e := s.SearchPage(st.Age.ID, "harbor", second.Next, 100)
	if e != nil || len(third.Records) != 5 || third.Next != "" {
		t.Fatalf("last page: %+v %v", third, e)
	}
	seen := map[string]bool{}
	n := 0
	for _, page := range []SearchPage{first, second, third} {
		for _, r := range page.Records {
			if seen[r.ID] || r.Name != fmt.Sprintf("Harbor %03d", n) {
				t.Fatal("duplicate or out-of-order result", r)
			}
			seen[r.ID] = true
			n++
		}
	}
	if _, e = s.SearchPage(st.Age.ID, "shoreline", first.Next, 100); !errors.Is(e, ErrConflict) {
		t.Fatal("cursor crossed query", e)
	}
	hidden, e := s.SearchPage(st.Age.ID, "hidden-search-sentinel", "", 100)
	if e != nil || len(hidden.Records) != 0 {
		t.Fatal("internal properties were indexed", e)
	}
	first.Records[0].Properties["description"] = "caller mutation"
	unchanged, e := s.SearchPage(st.Age.ID, "harbor", "", 1)
	if e != nil || unchanged.Records[0].Properties["description"] != "shoreline" {
		t.Fatal("cached mutable record escaped", e)
	}
	sourceAge := st.Age.ID
	st = apply(t, s, st, Command{Action: "copy-age", Name: "Later"})
	changed := records[0]
	changed.Name = "Mountain"
	st = put(t, s, st, changed)
	if _, e = s.SearchPage(st.Age.ID, "harbor", first.Next, 100); !errors.Is(e, ErrConflict) {
		t.Fatal("cursor crossed changed snapshot", e)
	}
	current, e := s.SearchPage(st.Age.ID, "mountain", "", 100)
	if e != nil || len(current.Records) != 1 {
		t.Fatal("new snapshot not indexed", e)
	}
	historical, e := s.SearchPage(sourceAge, "mountain", "", 100)
	if e != nil || len(historical.Records) != 0 {
		t.Fatal("index crossed Age history", e)
	}
	original, e := s.SearchPage(sourceAge, "harbor", first.Next, 100)
	if e != nil || len(original.Records) != 100 {
		t.Fatal("unchanged Age cursor invalidated", e)
	}
	for _, cursor := range []string{"!bad", "e30"} {
		if _, e = s.SearchPage(sourceAge, "harbor", cursor, 100); e == nil {
			t.Fatal("invalid cursor accepted")
		}
	}
	for _, limit := range []int{0, 201} {
		if _, e = s.SearchPage(sourceAge, "harbor", "", limit); e == nil {
			t.Fatal("unbounded page accepted")
		}
	}
	if len(s.searchIndexes) > 2 || s.searchMemory() > 128<<20 {
		t.Fatal("search cache exceeded budget")
	}
}
