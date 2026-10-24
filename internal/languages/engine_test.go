package languages

import (
	"context"
	"encoding/json"
	"kriemhild/internal/languages/spike"
	"os"
	"reflect"
	"testing"
)

func reference(t *testing.T) (Definition, map[ScriptRef]ScriptDefinition) {
	t.Helper()
	raw, err := os.ReadFile("spike/testdata/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := spike.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	d := Definition{Schema: 1, LanguageID: "11111111111111111111111111111111", Name: v.Name, Phonology: v.Phonology, Lexicon: v.Lexicon, Morphology: v.Morphology}
	s := ScriptDefinition{Schema: 1, ScriptID: "22222222222222222222222222222222", Name: "Reference script", Direction: "ltr"}
	for _, m := range v.Orthography.Mappings {
		s.Graphemes = append(s.Graphemes, Grapheme{ID: m.ID, Text: m.Grapheme})
	}
	r := ScriptRef{ID: s.ScriptID, Revision: Hash(s)}
	d.Orthography = &Orthography{Script: r, Normalization: v.Orthography.Normalization, Mappings: v.Orthography.Mappings}
	return d, map[ScriptRef]ScriptDefinition{r: s}
}
func request(a *Artifact, op string) Request {
	return Request{Request: spike.Request{Revision: a.Report().Revision, Engine: EngineVersion, Operation: op, Policy: "all", Limit: 64}, Adapter: AdapterVersion}
}
func TestProductCompileDependenciesAndImmutableArtifacts(t *testing.T) {
	d, deps := reference(t)
	before := Canonical(d)
	a := Compile(context.Background(), d, deps)
	report := a.Report()
	for _, op := range []string{"generate", "validate", "inflect", "analyze", "analyze-written", "write"} {
		if !report.Ready[op] {
			t.Fatalf("%s unavailable: %+v", op, report)
		}
	}
	q := request(a, "inflect")
	q.Lexeme = "stone"
	q.Features = Features{"number": "many"}
	result := a.Execute(context.Background(), q)
	if result.Status != "success" || len(result.Candidates) != 1 || result.Candidates[0].Written != "pad̪a" {
		t.Fatalf("incorrect realization %+v", result)
	}
	if result.Revision != Hash(d) || result.Engine != EngineVersion || result.Adapter != AdapterVersion || result.Kernel != spike.EngineVersion || result.Input.Engine != EngineVersion {
		t.Fatal("lost public provenance")
	}
	for i := 0; i < 20; i++ {
		fresh := Compile(context.Background(), d, deps)
		if !reflect.DeepEqual(result, fresh.Execute(context.Background(), q)) {
			t.Fatal("fresh compile changed result")
		}
	}
	d.Phonology.Segments[0].Label = "mutated"
	report.Ready["inflect"] = false
	result.Candidates[0].Segments[0] = "mutated"
	if string(before) == string(Canonical(d)) || a.Execute(context.Background(), q).Candidates[0].Segments[0] == "mutated" || !a.Report().Ready["inflect"] {
		t.Fatal("artifact aliases author or caller data")
	}
}
func TestOperationReadinessIsIndependent(t *testing.T) {
	d, deps := reference(t)
	d.Morphology.Paradigms[0].Cells[0].Operations[0].Kind = "unimplemented"
	a := Compile(context.Background(), d, deps)
	if !a.Report().Ready["generate"] || a.Report().Ready["inflect"] {
		t.Fatal("invalid morphology disabled phonology or executed unsupported rule")
	}
	q := request(a, "generate")
	q.Template = "vowel-consonant-vowel"
	if r := a.Execute(context.Background(), q); r.Status != "ambiguous" {
		t.Fatalf("phonology operation failed: %+v", r)
	}
	d, deps = reference(t)
	for key := range deps {
		delete(deps, key)
	}
	a = Compile(context.Background(), d, deps)
	if !a.Report().Ready["inflect"] || a.Report().Ready["analyze-written"] {
		t.Fatal("missing script disabled independent inflection")
	}
	q = request(a, "inflect")
	q.Lexeme = "stone"
	q.Features = Features{"number": "many"}
	r := a.Execute(context.Background(), q)
	if r.Status != "success" || r.Candidates[0].Written != "" {
		t.Fatal("script fallback fabricated output")
	}
}
func TestGrammarAgreementAndUnsupportedStructures(t *testing.T) {
	d, deps := reference(t)
	d.Syntax = &Syntax{Constructions: []Construction{{ID: "pair", Meaning: "authored pair", Separator: " / ", Roles: []Role{{ID: "left", Category: "noun", Features: Features{"number": "many"}}, {ID: "right", Category: "noun", Features: Features{}}}, Order: []Constituent{{Role: "left"}, {Role: "right"}}, Agreements: []Agreement{{ID: "agree-number", LeftRole: "left", RightRole: "right", Features: []string{"number"}}}}}}
	a := Compile(context.Background(), d, deps)
	q := request(a, "realize")
	q.Policy = "explicit"
	q.Construction = "pair"
	q.Roles = map[string]RoleInput{"left": {Lexeme: "stone", Features: Features{}}, "right": {Lexeme: "person", Features: Features{}}}
	r := a.Execute(context.Background(), q)
	if r.Status != "success" || r.Sentences[0].Written != "pad̪a / nin" || r.Sentences[0].Roles["right"].Features["number"] != "many" {
		t.Fatalf("agreement failed %+v %+v", r, a.Report())
	}
	q.Roles["right"] = RoleInput{Lexeme: "person", Features: Features{"number": "one"}}
	if r = a.Execute(context.Background(), q); r.Status != "unification_failure" {
		t.Fatal("conflict was ignored")
	}
	d.Syntax.Structures = []FeatureStructure{{Type: "recursive", Fields: map[string]Value{}}}
	a = Compile(context.Background(), d, deps)
	if a.Report().Ready["realize"] || !a.Report().Ready["inflect"] {
		t.Fatal("unsupported structures escaped readiness isolation")
	}
}
func TestProductStrictDecodeAndExplicitCompatibility(t *testing.T) {
	d, deps := reference(t)
	raw := Canonical(d)
	var keys map[string]json.RawMessage
	_ = json.Unmarshal(raw, &keys)
	keys["unknown"] = json.RawMessage(`true`)
	if _, err := Decode(Canonical(keys)); err == nil {
		t.Fatal("unknown field accepted")
	}
	if _, err := Decode([]byte(`{"schema":1,"schema":1}`)); err == nil {
		t.Fatal("duplicate key accepted")
	}
	a := Compile(context.Background(), d, deps)
	q := request(a, "translation")
	if r := a.Execute(context.Background(), q); r.Status != "unsupported_operation" || len(r.Candidates) > 0 {
		t.Fatal("fallback on unsupported request")
	}
	q.Adapter = "old"
	if r := a.Execute(context.Background(), q); r.Status != "incompatible_engine" {
		t.Fatal("adapter fallback")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r := Compile(ctx, d, deps).Report(); r.Status != "cancelled" {
		t.Fatal("compile ignored cancellation")
	}
}
