package project

import (
	"fmt"
	"testing"
)

func TestSearchAcrossAgesPreservesIdentityContextAndBoundedPages(t *testing.T) {
	s, st := fixture(t)
	city := Record{ID: NewID(), Kind: "entity", Type: "Place", Name: "Harbor founding"}
	st = put(t, s, st, city)
	names := map[string]string{st.Age.ID: city.Name}
	for i := 1; i < 7; i++ {
		st = apply(t, s, st, Command{Action: "copy-age", Name: fmt.Sprintf("Age %d", i)})
		city.Name = fmt.Sprintf("Harbor %d", i)
		st = put(t, s, st, city)
		names[st.Age.ID] = city.Name
	}
	seen := map[string]bool{}
	cursor := ""
	firstCursor := ""
	for pages := 0; ; pages++ {
		if pages > 10 {
			t.Fatal("search did not terminate")
		}
		result, e := s.SearchAcrossAges("HARBOR", cursor, 2)
		if e != nil || len(result.Hits) > 2 || result.AgesScanned > 3 {
			t.Fatalf("page %+v %v", result, e)
		}
		for _, hit := range result.Hits {
			if seen[hit.Age] || hit.Record.ID != city.ID || hit.Record.Name != names[hit.Age] || hit.Snapshot != st.Root.Ages[hit.Age].Snapshot {
				t.Fatal("lost Age context", hit)
			}
			seen[hit.Age] = true
		}
		cursor = result.Next
		if firstCursor == "" {
			firstCursor = cursor
		}
		if cursor == "" {
			break
		}
	}
	if len(seen) != len(names) {
		t.Fatal("missing historical matches", seen)
	}
	if _, e := s.SearchAcrossAges("different", firstCursor, 2); e != ErrConflict {
		t.Fatal("cross-query cursor accepted", e)
	}
	empty, e := s.SearchAcrossAges("absent", "", 100)
	if e != nil || len(empty.Hits) != 0 || empty.AgesScanned != 3 || empty.Next == "" {
		t.Fatal("absent query scanned every Age", e, empty)
	}
	st = put(t, s, st, Record{ID: NewID(), Kind: "note", Name: "Later update"})
	if _, e = s.SearchAcrossAges("harbor", firstCursor, 2); e != ErrConflict {
		t.Fatal("stale history cursor accepted", e)
	}
}
