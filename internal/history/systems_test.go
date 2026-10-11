package history

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func systemFixture(t *testing.T) *Document {
	t.Helper()
	d := New("Mechanics")
	age := d.World.CurrentAge
	for _, row := range []struct{ id, kind, root string }{{"magic", "system", ""}, {"engineering", "system", ""}, {"law", "system-rule", "magic"}, {"mineral", "system-resource", "magic"}, {"teleport", "system-capability", "magic"}, {"gate", "system-implementation", "engineering"}} {
		e := Entity{ID: row.id, Kind: row.kind, Name: row.id, System: &SystemRecord{Version: 1, SystemID: row.root, Category: "hybrid"}}
		if err := d.Apply(Command{Kind: "entity", AgeID: age, Entity: &e}); err != nil {
			t.Fatal(err)
		}
	}
	for _, pair := range [][2]string{{"teleport", "law"}, {"gate", "teleport"}} {
		r := Relation{Source: pair[0], Destination: pair[1], Kind: "system-requires"}
		if err := d.Apply(Command{Kind: "relation", AgeID: age, Relation: &r}); err != nil {
			t.Fatal(err)
		}
	}
	return d
}
func hasSystemIssue(a SystemAnalysis, id, code string) bool {
	for _, i := range a.Issues {
		if i.EntityID == id && i.Code == code {
			return true
		}
	}
	return false
}
func TestSystemImpactExceptionsAndConstraints(t *testing.T) {
	d := systemFixture(t)
	age := d.World.CurrentAge
	r := Relation{Source: "law", Destination: "mineral", Kind: "system-consumes"}
	if err := d.Apply(Command{Kind: "relation", AgeID: age, Relation: &r}); err != nil {
		t.Fatal(err)
	}
	d.Entity(age, "magic").System.Constraints = []SystemConstraint{{Key: "resurrection", Mode: "prohibit"}}
	d.Entity(age, "teleport").System.Effects = []string{"resurrection"}
	d.Entity(age, "gate").System.Effects = []string{"resurrection"}
	before, _ := json.Marshal(d)
	a := d.AnalyzeSystems(age, "magic", "law")
	after, _ := json.Marshal(d)
	if string(before) != string(after) {
		t.Fatal("advisory check mutated data")
	}
	if len(a.Affected) != 2 || a.Affected[1].EntityID != "gate" || !slices.Equal(a.Affected[1].Path, []string{"law", "teleport", "gate"}) {
		t.Fatalf("incorrect cross-system impact: %+v", a.Affected)
	}
	if !hasSystemIssue(a, "teleport", "prohibited-effect") || !hasSystemIssue(a, "gate", "prohibited-effect") {
		t.Fatalf("inherited constraints ignored: %+v", a.Issues)
	}
	r = Relation{Source: "teleport", Destination: "magic", Kind: "system-exception-to", Description: "An intentional exceptional effect"}
	if err := d.Apply(Command{Kind: "relation", AgeID: age, Relation: &r}); err != nil {
		t.Fatal(err)
	}
	a = d.AnalyzeSystems(age, "magic", "law")
	if hasSystemIssue(a, "teleport", "prohibited-effect") {
		t.Fatal("exception not honored")
	}
	// Separate prerequisites can impose mutually impossible numeric limits.
	min, max := 5., 3.
	d.Entity(age, "law").System.Constraints = []SystemConstraint{{Key: "fuel", Mode: "minimum", Value: &min, Unit: "kg"}}
	d.Entity(age, "mineral").System.Constraints = []SystemConstraint{{Key: "fuel", Mode: "maximum", Value: &max, Unit: "kg"}}
	a = d.AnalyzeSystems(age, "magic", "law")
	if !hasSystemIssue(a, "gate", "dependency-limits") {
		t.Fatal("transitive numeric contradiction missed")
	}
	d.Entity(age, "gate").System.Effects = nil
	d.Entity(age, "gate").System.Operation = "Investigate resurrection"
	a = d.AnalyzeSystems(age, "magic", "law")
	if !hasSystemIssue(a, "gate", "possible-prose-conflict") {
		t.Fatal("prose should be flagged as possible only")
	}
	d.Entity(age, "mineral").System.Constraints[0].Unit = "tonnes"
	a = d.AnalyzeSystems(age, "magic", "law")
	if hasSystemIssue(a, "gate", "dependency-limits") {
		t.Fatal("incomparable units treated as a hard conflict")
	}
}
func TestSystemCyclesAndIndependentHistoricalState(t *testing.T) {
	d := systemFixture(t)
	age := d.World.CurrentAge
	r := Relation{Source: "law", Destination: "teleport", Kind: "system-requires"}
	if err := d.Apply(Command{Kind: "relation", AgeID: age, Relation: &r}); err != nil {
		t.Fatal(err)
	}
	if !hasSystemIssue(d.AnalyzeSystems(age, "magic", "law"), "law", "startup-cycle") {
		t.Fatal("startup cycle missed")
	}
	d.Entity(age, "law").System.SeedAvailable = true
	if hasSystemIssue(d.AnalyzeSystems(age, "magic", "law"), "law", "startup-cycle") {
		t.Fatal("explicit bootstrap was ignored")
	}
	d.Entity(age, "law").System.Fields = []TypedField{{Definition: FieldDefinition{ID: "formula", Name: "Formula", Type: "text"}, Value: json.RawMessage(`"E=mc2"`)}}
	original := cloneSystem(d.Entity(age, "law").System)
	if err := d.Apply(Command{Kind: "age", AgeID: age, Mode: "branch", Name: "Later"}); err != nil {
		t.Fatal(err)
	}
	next := d.World.CurrentAge
	e := *d.Entity(next, "law")
	e.System.Costs = "New cost"
	e.System.Fields[0].Value = json.RawMessage(`"Revised"`)
	e.Properties = json.RawMessage(`{"startYear":123,"endYear":130}`)
	if err := d.Apply(Command{Kind: "entity", AgeID: next, Entity: &e}); err != nil {
		t.Fatal(err)
	}
	e.System.Costs = "External mutation"
	if d.Entity(next, "law").System.Costs == e.System.Costs {
		t.Fatal("caller aliases saved system")
	}
	if !reflect.DeepEqual(original, d.Entity(age, "law").System) {
		t.Fatal("Age inheritance aliased system state")
	}
	if len(d.Years) != 2 {
		t.Fatalf("system dates not synchronized: %+v", d.Years)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := d.Apply(Command{Kind: "age", AgeID: age, Mode: "branch", Name: "Missing law", ExcludeIDs: []string{"law"}}); err != nil {
		t.Fatal(err)
	}
	if !hasSystemIssue(d.AnalyzeSystems(d.World.CurrentAge, "magic", "teleport"), "teleport", "missing-prerequisite") {
		t.Fatal("excluded prerequisite was silently forgotten")
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestSystemValidationAndKnowledgeSeparation(t *testing.T) {
	d := systemFixture(t)
	age := d.World.CurrentAge
	e := Entity{ID: "person", Name: "Researcher", Kind: "character"}
	if err := d.Apply(Command{Kind: "entity", AgeID: age, Entity: &e}); err != nil {
		t.Fatal(err)
	}
	r := Relation{Source: "person", Destination: "teleport", Kind: "system-knowledge", Status: "incorrect-theory"}
	if err := d.Apply(Command{Kind: "relation", AgeID: age, Relation: &r}); err != nil {
		t.Fatal(err)
	}
	if d.Entity(age, "teleport").System.Possibility != "" || d.Entity(age, "teleport").System.Development != "" {
		t.Fatal("knowledge changed reality")
	}
	r.Status = "invented-status"
	if err := d.Apply(Command{Kind: "relation", AgeID: age, Relation: &r}); err == nil {
		t.Fatal("invalid knowledge state accepted")
	}
	r.Status = "understood"
	r.Source, r.Destination = r.Destination, r.Source
	if err := d.Apply(Command{Kind: "relation", AgeID: age, Relation: &r}); err == nil {
		t.Fatal("reversed knowledge endpoints accepted")
	}
	bad := *d.Entity(age, "law")
	bad.System = cloneSystem(bad.System)
	bad.System.Version = 99
	if err := d.Apply(Command{Kind: "entity", AgeID: age, Entity: &bad}); err == nil {
		t.Fatal("unknown version accepted")
	}
	bad.System.Version = 1
	bad.System.Sources = []SystemSource{{EntityID: "missing"}}
	if err := d.Apply(Command{Kind: "entity", AgeID: age, Entity: &bad}); err == nil {
		t.Fatal("dangling source accepted")
	}
	bad.System.Sources = nil
	bad.System.Constraints = []SystemConstraint{{Key: "power", Mode: "maximum"}}
	if err := d.Apply(Command{Kind: "entity", AgeID: age, Entity: &bad}); err == nil {
		t.Fatal("numeric constraint without value accepted")
	}
	if err := d.Apply(Command{Kind: "delete-entities", AgeID: age, EntityIDs: []string{"magic"}}); err == nil {
		t.Fatal("orphaned system sections")
	}
	if err := d.Apply(Command{Kind: "age", AgeID: age, Name: "No magic", Mode: "branch", ExcludeIDs: []string{"magic"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(); err != nil {
		t.Fatal("invalid historical exclusion:", err)
	}
}

func TestSystemRestrictsAndProhibitsAreNotStartupRequirements(t *testing.T) {
	d := systemFixture(t)
	age := d.World.CurrentAge
	limit := 2.
	d.Entity(age, "law").System.Constraints = []SystemConstraint{{Key: "range", Mode: "maximum", Value: &limit, Unit: "km"}}
	wanted := 10.
	d.Entity(age, "gate").System.Constraints = []SystemConstraint{{Key: "range", Mode: "value", Value: &wanted, Unit: "km"}}
	r := Relation{Source: "mineral", Destination: "gate", Kind: "system-prohibits"}
	if err := d.Apply(Command{Kind: "relation", AgeID: age, Relation: &r}); err != nil {
		t.Fatal(err)
	}
	a := d.AnalyzeSystems(age, "magic", "mineral")
	if len(a.Affected) == 0 || !hasSystemIssue(a, "gate", "incompatible-dependencies") {
		t.Fatal("explicit prohibition missed")
	}
	if slices.Contains(a.Requirements["gate"], "mineral") {
		t.Fatal("prohibition became consumption")
	}
}
