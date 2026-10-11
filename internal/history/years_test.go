package history

import (
	"encoding/json"
	"reflect"
	"testing"
)

func datedEntity(t *testing.T, d *Document, age, id, props string) {
	t.Helper()
	e := Entity{ID: id, Name: id, Kind: "character", Properties: json.RawMessage(props)}
	if err := d.Apply(Command{Kind: "entity", AgeID: age, Entity: &e}); err != nil {
		t.Fatal(err)
	}
}

func TestYearsReconcileDatesAndEvents(t *testing.T) {
	d := New("Years")
	a := d.World.CurrentAge
	datedEntity(t, d, a, "a", `{"born":"0042","died":"80","summary":"A navigator"}`)
	datedEntity(t, d, a, "b", `{"born":42,"died":"0"}`)
	if len(d.Years) != 3 || len(d.Year(a, "year/42").Events) != 2 {
		t.Fatal("years not shared by value", d.Years)
	}
	original := d.Year(a, "year/42").ID
	datedEntity(t, d, a, "a", `{"born":"-12","died":"","biography":"A revised history"}`)
	if len(d.Year(a, original).Events) != 1 || len(d.Year(a, "year/80").Events) != 0 || d.Year(a, "year/-12").Events[0].Description != "A revised history" {
		t.Fatal("date edits left stale events")
	}
	before, _ := json.Marshal(d.Years)
	d.SyncYears()
	after, _ := json.Marshal(d.Years)
	if string(before) != string(after) {
		t.Fatal("reconciliation not idempotent")
	}
	if err := d.Apply(Command{Kind: "delete-entities", AgeID: a, EntityIDs: []string{"b"}}); err != nil {
		t.Fatal(err)
	}
	if len(d.Year(a, "year/42").Events) != 0 || len(d.Year(a, "year/0").Events) != 0 {
		t.Fatal("deleted entity remains on timeline")
	}
}

func TestAgeStartedFromYearKeepsIndependentHistory(t *testing.T) {
	d := New("Years")
	a := d.World.CurrentAge
	datedEntity(t, d, a, "a", `{"born":"12","died":"90"}`)
	if err := d.Apply(Command{Kind: "age", AgeID: a, Name: "Invalid", Mode: "branch", SourceYearID: "missing"}); err == nil {
		t.Fatal("accepted missing year")
	}
	if len(d.Ages) != 1 {
		t.Fatal("invalid year partially created Age")
	}
	if err := d.Apply(Command{Kind: "age", AgeID: a, Name: "Divergence", Mode: "branch", SourceYearID: "year/12"}); err != nil {
		t.Fatal(err)
	}
	b := d.World.CurrentAge
	if !reflect.DeepEqual(d.Age(b).StartedFromYear, &YearOrigin{AgeID: a, YearID: "year/12", Number: 12}) {
		t.Fatal("lost year origin")
	}
	datedEntity(t, d, b, "a", `{"born":"15","died":""}`)
	if len(d.Year(a, "year/12").Events) != 1 || len(d.Year(b, "year/12").Events) != 0 {
		t.Fatal("Age states alias")
	}
	if err := d.Apply(Command{Kind: "delete-age", AgeID: a}); err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(); err != nil {
		t.Fatal("deleted source lost provenance", err)
	}
}

func TestYearValidationAndLegacyDates(t *testing.T) {
	d := New("Years")
	a := d.World.CurrentAge
	for _, props := range []string{`{"born":"1.5"}`, `{"born":"tomorrow"}`, `{"born":{}}`, `{"born":"9007199254740992"}`} {
		e := Entity{ID: "bad", Name: "bad", Kind: "character", Properties: json.RawMessage(props)}
		if d.Apply(Command{Kind: "entity", AgeID: a, Entity: &e}) == nil {
			t.Fatal("accepted invalid year", props)
		}
	}
	d.Entities = append(d.Entities, Entity{ID: "legacy", Name: "Old", AgeID: a, Kind: "character", Properties: json.RawMessage(`{"born":"Spring of the old era","died":"12"}`)})
	d.SyncYears()
	if len(d.Years) != 1 || d.Years[0].Number != 12 {
		t.Fatal("legacy numeric date not migrated")
	}
	e := *d.Entity(a, "legacy")
	e.Name = "Renamed"
	if err := d.Apply(Command{Kind: "entity", AgeID: a, Entity: &e}); err != nil {
		t.Fatal("legacy text blocks unrelated edit", err)
	}
}
