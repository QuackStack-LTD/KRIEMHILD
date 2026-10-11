package history

import (
	"encoding/json"
	"kriemhild/internal/biosphere"
	"reflect"
	"testing"
)

func conceptFixture(t *testing.T) (*Document, string) {
	t.Helper()
	d := New("Concepts")
	a := d.World.CurrentAge
	for _, e := range []Entity{{ID: "place", Kind: "city", Name: "Rivergate"}, {ID: "person", Kind: "character", Name: "Ada"}, {ID: "org", Kind: "organization", Name: "Guild", Properties: json.RawMessage(`{"startDate":42,"endDate":80}`)}} {
		if err := d.Apply(Command{Kind: "entity", AgeID: a, Entity: &e}); err != nil {
			t.Fatal(err)
		}
	}
	year := int64(42)
	e := Entity{ID: "faith", Kind: "religion", Name: "River tradition", Properties: json.RawMessage(`{"startYear":42}`), Content: &ConceptContent{Version: 1, Template: "religion", Tags: []string{"river"}, Fields: []TypedField{{Definition: FieldDefinition{ID: "site", Name: "Place", Type: "entity"}, Value: json.RawMessage(`"place"`)}}, Components: []ConceptComponent{{ID: "belief", Type: "belief", Title: "The river remembers", Description: "A disputed teaching", StartYear: &year, Claim: &Claim{Statement: "Water remembers every journey", WorldTruth: "unknown"}, Contexts: []ConceptVariation{{ID: "variant", ContextID: "place", Stance: "metaphorical", Prevalence: "minority"}}, Fields: []TypedField{{Definition: FieldDefinition{ID: "observance", Name: "Observances", Type: "section", Fields: []FieldDefinition{{ID: "name", Name: "Name", Type: "text"}, {ID: "weight", Name: "Weight", Type: "number"}}}, Value: json.RawMessage(`[{"name":"Dawn ritual","weight":2}]`)}}, Links: []ComponentLink{{ID: "link", TargetID: "person", Kind: "associated-with", Stance: "disputed", Description: "Personal interpretation"}}}}}}
	if err := d.Apply(Command{Kind: "entity", AgeID: a, Entity: &e}); err != nil {
		t.Fatal(err)
	}
	return d, a
}
func TestConceptInheritancePromotionAndYears(t *testing.T) {
	d, a := conceptFixture(t)
	if len(d.Year(a, "year/42").Events) != 3 || len(d.Year(a, "year/80").Events) != 1 {
		t.Fatal("organization and component dates not indexed", d.Years)
	}
	if err := d.Apply(Command{Kind: "age", AgeID: a, Mode: "branch", Name: "Reformation"}); err != nil {
		t.Fatal(err)
	}
	b := d.World.CurrentAge
	inherited := d.Entity(b, "faith")
	inherited.Content.Components[0].Claim.WorldTruth = "undecided"
	inherited.Content.Components[0].Fields[0].Value[3] = 'N'
	if d.Entity(a, "faith").Content.Components[0].Claim.WorldTruth != "unknown" || reflect.DeepEqual(d.Entity(a, "faith").Content.Components[0].Fields[0].Value, inherited.Content.Components[0].Fields[0].Value) {
		t.Fatal("content aliases between Ages")
	}
	// Restore valid structured field before promotion.
	inherited.Content.Components[0].Fields[0].Value = json.RawMessage(`[{"name":"Night ritual","weight":3}]`)
	original := cloneContent(inherited.Content)
	if err := d.Apply(Command{Kind: "promote-component", AgeID: b, EntityID: "faith", ComponentID: "belief"}); err != nil {
		t.Fatal(err)
	}
	ref := d.Entity(b, "faith").Content.Components[0]
	promoted := d.Entity(b, ref.EntityID)
	if promoted == nil || promoted.Kind != "doctrine" || !reflect.DeepEqual(promoted.Content.Fields, original.Components[0].Fields) || promoted.Content.Claim.WorldTruth != "undecided" {
		t.Fatal("promotion lost data")
	}
	if d.Entity(a, "faith").Content.Components[0].EntityID != "" {
		t.Fatal("promotion modified original Age")
	}
	if len(d.Year(b, "year/42").Events) != 3 {
		t.Fatal("promotion duplicated/lost year event")
	}
	found := false
	for _, r := range d.Relations {
		if r.AgeID == b && r.Source == promoted.ID && r.Destination == "person" && r.Stance == "disputed" {
			found = true
		}
	}
	if !found {
		t.Fatal("promotion lost relationship")
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestConceptTypedValidationAndSafeDeletion(t *testing.T) {
	d, a := conceptFixture(t)
	for _, test := range []struct {
		name string
		edit func(*ConceptContent)
	}{
		{"version", func(c *ConceptContent) { c.Version = 900 }},
		{"truth", func(c *ConceptContent) { c.Components[0].Claim.WorldTruth = "believed" }},
		{"reference", func(c *ConceptContent) { c.Fields[0].Value = json.RawMessage(`"missing"`) }},
		{"number", func(c *ConceptContent) { c.Components[0].Fields[0].Value = json.RawMessage(`[{"weight":"many"}]`) }},
		{"unknown column", func(c *ConceptContent) { c.Components[0].Fields[0].Value = json.RawMessage(`[{"unknown":true}]`) }},
		{"context", func(c *ConceptContent) { c.Components[0].Contexts[0].ContextID = "missing" }},
		{"link type", func(c *ConceptContent) { c.Components[0].Links[0].Kind = "biological-parent" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			e := *d.Entity(a, "faith")
			e.Content = cloneContent(e.Content)
			test.edit(e.Content)
			before, _ := json.Marshal(d)
			if err := d.Apply(Command{Kind: "entity", AgeID: a, Entity: &e}); err == nil {
				t.Fatal("invalid content accepted")
			}
			after, _ := json.Marshal(d)
			if string(before) != string(after) {
				t.Fatal("invalid write changed document")
			}
		})
	}
	if err := d.Apply(Command{Kind: "delete-entities", AgeID: a, EntityIDs: []string{"place"}}); err == nil || d.Entity(a, "place").Deleted {
		t.Fatal("referenced entity deletion should be atomic and rejected")
	}
	e := *d.Entity(a, "faith")
	e.Content = nil
	e.Name = "Renamed"
	if err := d.Apply(Command{Kind: "entity", AgeID: a, Entity: &e}); err != nil || d.Entity(a, "faith").Content == nil {
		t.Fatal("legacy profile update lost typed content", err)
	}
}
func TestAllLogicalEntitiesYearAliasesAndEdits(t *testing.T) {
	d := New("Years")
	a := d.World.CurrentAge
	kinds := []string{"system"}
	for kind := range Registry.Types {
		if kind != "system" {
			kinds = append(kinds, kind)
		}
	}
	for _, kind := range kinds {
		if kind == "organization-unit" || kind == "organization-position" {
			continue
		}
		e := Entity{ID: kind, Kind: kind, Name: kind, Properties: json.RawMessage(`{"startYear":0,"endYear":42}`)}
		if Registry.Types[kind].Category == "systems" {
			e.System = &SystemRecord{Version: 1, Category: "custom"}
			if kind != "system" {
				e.System.SystemID = "system"
			}
		}
		if kind == "lifeform" {
			b := biosphere.Catalogue()[0].Biology
			e.Biology = &b
		}
		if err := d.Apply(Command{Kind: "entity", AgeID: a, Entity: &e}); err != nil {
			t.Fatal(kind, err)
		}
	}
	if len(d.Year(a, "year/0").Events) != len(d.Entities) {
		t.Fatal("not all entity kinds indexed")
	}
	org := *d.Entity(a, "organization")
	org.Properties = json.RawMessage(`{"startYear":0,"endYear":42,"startDate":-10,"endDate":80}`)
	if err := d.Apply(Command{Kind: "entity", AgeID: a, Entity: &org}); err != nil {
		t.Fatal(err)
	}
	if d.Year(a, "year/-10") == nil || d.Year(a, "year/80") == nil {
		t.Fatal("date aliases not applied")
	}
	org = *d.Entity(a, "organization")
	org.Properties = json.RawMessage(`{"startYear":-10,"endYear":80,"startDate":"","endDate":""}`)
	if err := d.Apply(Command{Kind: "entity", AgeID: a, Entity: &org}); err != nil {
		t.Fatal(err)
	}
	if len(d.Year(a, "year/-10").Events) != 0 || len(d.Year(a, "year/80").Events) != 0 {
		t.Fatal("cleared dates left stale events")
	}
	for _, kind := range []string{"terrain", "map", "writing-document"} {
		if yearEntityEligible(kind) {
			t.Fatal("non-entity date indexed", kind)
		}
	}
}

func TestConceptFieldTypes(t *testing.T) {
	d, a := conceptFixture(t)
	tests := []struct {
		kind, valid, invalid string
		options              []string
	}{
		{"text", `"text"`, `4`, nil}, {"long-text", `"long text"`, `[]`, nil}, {"number", `3.5`, `"3.5"`, nil}, {"boolean", `true`, `"true"`, nil}, {"date", `-42`, `1.5`, nil}, {"age", `"` + a + `"`, `"other-world-age"`, nil}, {"entity", `"person"`, `"missing"`, nil}, {"select", `"a"`, `"c"`, []string{"a", "b"}}, {"multi-select", `["a","b"]`, `["a","a"]`, []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			f := TypedField{Definition: FieldDefinition{ID: "test", Name: "Test", Type: tt.kind, Options: tt.options}, Value: json.RawMessage(tt.valid)}
			if err := d.validateFields(a, []TypedField{f}); err != nil {
				t.Fatal(err)
			}
			f.Value = json.RawMessage(tt.invalid)
			if err := d.validateFields(a, []TypedField{f}); err == nil {
				t.Fatal("invalid typed value accepted")
			}
		})
	}
}
func TestEndpointTransferRetainsRelationshipAndAge(t *testing.T) {
	d := New("Transfer")
	a := d.World.CurrentAge
	for _, id := range []string{"p1", "p2", "child", "other"} {
		person(t, d, a, id, "character")
	}
	for _, r := range []Relation{{ID: "a", Source: "p1", Destination: "child", Kind: "parent-of", DestinationPort: "n", Description: "First parent"}, {ID: "b", Source: "child", Destination: "p2", Kind: "child-of", SourcePort: "n", Description: "Second parent"}} {
		if err := d.Apply(Command{Kind: "relation", AgeID: a, Relation: &r}); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.Apply(Command{Kind: "age", AgeID: a, Mode: "branch", Name: "Future"}); err != nil {
		t.Fatal(err)
	}
	b := d.World.CurrentAge
	changes := []Relation{}
	for _, r := range d.Relations {
		if r.AgeID != b {
			continue
		}
		if r.Source == "child" {
			r.Source = "other"
			r.SourcePort = "sw"
		} else {
			r.Destination = "other"
			r.DestinationPort = "sw"
		}
		changes = append(changes, r)
	}
	if err := d.Apply(Command{Kind: "relations", AgeID: b, Relations: changes}); err != nil {
		t.Fatal(err)
	}
	if len(d.Relations) != 4 {
		t.Fatal("transfer created duplicate relation")
	}
	for _, r := range d.Relations {
		if r.Description == "" {
			t.Fatal("lost description")
		}
		if r.AgeID == a && r.Source != "child" && r.Destination != "child" {
			t.Fatal("source Age modified")
		}
	}
	before, _ := json.Marshal(d.Relations)
	changes[0].Destination = changes[0].Source
	if err := d.Apply(Command{Kind: "relations", AgeID: b, Relations: changes}); err == nil {
		t.Fatal("self relationship accepted")
	}
	after, _ := json.Marshal(d.Relations)
	if string(before) != string(after) {
		t.Fatal("failed transfer partially applied")
	}
}
