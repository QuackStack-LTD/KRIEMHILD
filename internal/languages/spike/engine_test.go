package spike

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func fixture(t testing.TB) Definition {
	t.Helper()
	raw, err := os.ReadFile("testdata/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	d, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func compileFixture(t testing.TB, d Definition) *Compiled {
	t.Helper()
	c, report := Compile(context.Background(), d, TokenTrieBackend{})
	if report.Status != "compiled" {
		t.Fatalf("unexpected compilation: %s", Canonical(report))
	}
	return c
}
func request(c *Compiled, operation string) Request {
	return Request{Revision: c.report.Revision, Engine: EngineVersion, Operation: operation, Policy: "all", Limit: 256}
}
func inflect(c *Compiled, lexeme, number string) Result {
	r := request(c, "inflect")
	r.Lexeme = lexeme
	r.Features = Features{"number": number}
	return c.Execute(context.Background(), r)
}
func TestAuthoredParadigmContextAndSuppletion(t *testing.T) {
	d := fixture(t)
	before := Canonical(d)
	c := compileFixture(t, d)
	if c.Report().Forms != 8 || c.Report().States == 0 {
		t.Fatal(c.Report())
	}
	for _, test := range []struct {
		lexeme, number, written string
		segments, ruleIDs       []string
	}{
		{"stone", "one", "pat", []string{"p", "a", "t"}, []string{"stone", "count-one-zero", "spell-p", "spell-a", "spell-t"}},
		{"stone", "many", "pad̪a", []string{"p", "a", "d", "a"}, []string{"stone", "count-many-suffix", "intervocalic-voicing", "spell-p", "spell-a", "spell-d", "spell-a"}},
		{"person", "many", "nin", []string{"n", "i", "n"}, []string{"person", "person-many-suppletion", "spell-n", "spell-i", "spell-n"}},
		{"tree", "one", "ad̪a", []string{"a", "d", "a"}, []string{"tree", "count-one-zero", "intervocalic-voicing", "spell-a", "spell-d", "spell-a"}},
	} {
		r := inflect(c, test.lexeme, test.number)
		if r.Status != "success" || len(r.Candidates) != 1 {
			t.Fatal(r)
		}
		form := r.Candidates[0]
		if form.Written != test.written || !reflect.DeepEqual(form.Segments, test.segments) {
			t.Fatalf("%s: %s", test.lexeme, Canonical(form))
		}
		ids := []string{}
		for _, step := range form.Trace {
			ids = append(ids, step.RuleID)
		}
		if !reflect.DeepEqual(ids, test.ruleIDs) {
			t.Fatalf("incorrect rule derivation: %v", ids)
		}
	}
	if !bytes.Equal(before, Canonical(d)) {
		t.Fatal("compilation mutated authored definition")
	}
	d.Lexicon[0].Underlying[0] = "n"
	r := inflect(c, "stone", "one")
	if r.Candidates[0].Written != "pat" {
		t.Fatal("artifact borrowed mutable definition storage")
	}
	r.Candidates[0].Segments[0] = "n"
	r.Candidates[0].Trace[0].After[0] = "n"
	if inflect(c, "stone", "one").Candidates[0].Segments[0] != "p" {
		t.Fatal("result mutated immutable artifact")
	}
}
func TestInverseAnalysisAmbiguityLimitsAndNoGuesses(t *testing.T) {
	c := compileFixture(t, fixture(t))
	r := request(c, "analyze")
	r.Segments = []string{"p", "a", "d", "a"}
	got := c.Execute(context.Background(), r)
	if got.Status != "ambiguous" || len(got.Candidates) != 2 || got.Candidates[0].Lexeme != "rock" || got.Candidates[1].Lexeme != "stone" {
		t.Fatalf("homophones lost: %s", Canonical(got))
	}
	r.Operation = "analyze-written"
	r.Segments = nil
	r.Text = "pad̪a"
	got = c.Execute(context.Background(), r)
	if got.Status != "ambiguous" || len(got.Candidates) != 2 {
		t.Fatal(got)
	}
	r.Policy = "explicit"
	got = c.Execute(context.Background(), r)
	if got.Status != "choice_required" {
		t.Fatal(got)
	}
	r.Policy = "all"
	r.Limit = 1
	got = c.Execute(context.Background(), r)
	if got.Status != "candidate_limit" || !got.Truncated || got.Examined != 2 {
		t.Fatal(got)
	}
	r.Limit = 256
	r.Text = "invented"
	got = c.Execute(context.Background(), r)
	if got.Status != "no_valid_candidate" || len(got.Candidates) != 0 {
		t.Fatal("unapproved fallback", got)
	}
	r = request(c, "inflect")
	r.Lexeme = "person"
	r.Features = Features{"gender": "invented"}
	got = c.Execute(context.Background(), r)
	if got.Status != "invalid_input" || got.Diagnostics[0].Code != "unknown_feature" {
		t.Fatal(got)
	}
	r = request(c, "realize-sentence")
	if got := c.Execute(context.Background(), r); got.Status != "unsupported_operation" || len(got.Candidates) != 0 {
		t.Fatal(got)
	}
}

func TestExplicitInputFieldsAndDiagnosticReferences(t *testing.T) {
	c := compileFixture(t, fixture(t))
	r := request(c, "generate")
	r.Template = "vowel-consonant-vowel"
	r.Features = Features{"number": "many"}
	if got := c.Execute(context.Background(), r); got.Status != "invalid_input" || got.Diagnostics[0].Code != "irrelevant_input" {
		t.Fatal("features silently ignored", got)
	}
	r = request(c, "validate")
	r.Segments = []string{"n", "n"}
	got := c.Execute(context.Background(), r)
	if got.Status != "no_valid_candidate" || got.Diagnostics[0].RuleID != "no-double-n" || got.Diagnostics[0].Path != "request.segments" {
		t.Fatal("failed constraint has no reference", got)
	}
	a, b := Features{"author-feature": "first"}, Features{"custom-category": "second"}
	merged, ok := Unify(a, b)
	if !ok || len(merged) != 2 || len(a) != 1 || len(b) != 1 {
		t.Fatal("feature unification changed inputs")
	}
	if _, ok := Unify(a, Features{"author-feature": "conflicting"}); ok {
		t.Fatal("conflicting features unified")
	}
}
func TestNaturalClassesGenerationAndDeterministicTrace(t *testing.T) {
	c := compileFixture(t, fixture(t))
	r := request(c, "generate")
	r.Template = "vowel-consonant-vowel"
	got := c.Execute(context.Background(), r)
	if got.Examined != 12 || len(got.Candidates) != 11 || got.Status != "ambiguous" {
		t.Fatalf("unexpected enumeration: %s", Canonical(got))
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].RuleID != "no-ipi" {
		t.Fatal("missing rejection reason", got.Diagnostics)
	}
	for _, form := range got.Candidates {
		if form.Underlying[1] == "d" {
			t.Fatal("phonetic realization was generated as a phoneme")
		}
	}
	r.Policy = "seeded"
	r.Seed = "fixed-author-seed"
	expected := Canonical(c.Execute(context.Background(), r))
	if !strings.Contains(string(expected), `"status":"success"`) {
		t.Fatal(string(expected))
	}
	for i := 0; i < 50; i++ {
		if !bytes.Equal(expected, Canonical(c.Execute(context.Background(), r))) {
			t.Fatal("same revision/engine/request changed bytes")
		}
	}
	second := compileFixture(t, fixture(t))
	if !bytes.Equal(expected, Canonical(second.Execute(context.Background(), r))) {
		t.Fatal("fresh compilation changed output or trace")
	}
	r.Seed = ""
	if c.Execute(context.Background(), r).Diagnostics[0].Code != "seed_required" {
		t.Fatal("seed inferred without authorization")
	}
	before := Canonical(c.definition)
	_ = c.Execute(context.Background(), request(c, "generate"))
	if !bytes.Equal(before, Canonical(c.definition)) {
		t.Fatal("generation changed canonical lexicon")
	}
}
func TestRuleOrderAndWordBoundaryAreExplicit(t *testing.T) {
	d := fixture(t)
	rule := SoundRule{ID: "feed-voiced-to-n", Domain: "word", Input: []Match{{Segment: "d"}}, Output: []string{"n"}, Left: []Match{{Class: "V"}}, Right: []Match{{Class: "V"}}}
	d.Phonology.Rules = append(d.Phonology.Rules, rule)
	first := compileFixture(t, d)
	if inflect(first, "stone", "many").Candidates[0].Written != "pana" {
		t.Fatal("feeding order ignored")
	}
	d.Phonology.Rules[0], d.Phonology.Rules[1] = d.Phonology.Rules[1], d.Phonology.Rules[0]
	second := compileFixture(t, d)
	if inflect(second, "stone", "many").Candidates[0].Written != "pad̪a" {
		t.Fatal("rule output recursively reapplied")
	}
	if first.Report().Revision == second.Report().Revision {
		t.Fatal("ordering not included in revision hash")
	}
	d = fixture(t)
	d.Phonology.Rules = append(d.Phonology.Rules, SoundRule{ID: "word-final-t", Domain: "word", Input: []Match{{Segment: "t"}}, Output: []string{"p"}, Right: []Match{{Boundary: "word"}}})
	c := compileFixture(t, d)
	if inflect(c, "stone", "one").Candidates[0].Written != "pap" || inflect(c, "stone", "many").Candidates[0].Written != "pad̪a" {
		t.Fatal("word boundary incorrectly matched an interior segment")
	}
}
func TestIncompleteDefinitionsDoNotInventModules(t *testing.T) {
	d := Definition{Schema: 1, LanguageID: "draft-language", Name: "Incomplete author draft"}
	c, report := Compile(context.Background(), d, TokenTrieBackend{})
	if report.Status != "incomplete" || report.Ready["generate"] {
		t.Fatal(report)
	}
	r := request(c, "generate")
	if got := c.Execute(context.Background(), r); got.Status != "missing_dependency" || len(got.Candidates) != 0 {
		t.Fatal(got)
	}
	d = fixture(t)
	d.Orthography = nil
	c = compileFixture(t, d)
	if c.Report().Ready["analyze-written"] || !c.Report().Ready["inflect"] {
		t.Fatal(c.Report())
	}
	if form := inflect(c, "stone", "many").Candidates[0]; form.Written != "" || !reflect.DeepEqual(form.Segments, []string{"p", "a", "d", "a"}) {
		t.Fatal("script invented", form)
	}
	// This language does not distinguish grammatical number. Nothing adds it.
	d.Morphology.Features = nil
	d.Morphology.Paradigms[0].Cells = d.Morphology.Paradigms[0].Cells[:1]
	d.Morphology.Paradigms[0].Cells[0].Features = Features{}
	for i := range d.Lexicon {
		d.Lexicon[i].Overrides = nil
	}
	c = compileFixture(t, d)
	r = request(c, "inflect")
	r.Lexeme = "stone"
	r.Features = Features{}
	if got := c.Execute(context.Background(), r); got.Status != "success" || len(got.Candidates[0].Features) != 0 {
		t.Fatal(got)
	}
}
func TestStrictDecodingAndCanonicalMapOrder(t *testing.T) {
	for _, raw := range []string{
		`{"schema":1,"languageId":"l","name":"A","name":"B"}`,
		`{"schema":1,"languageId":"l","name":"A","na\u006de":"B"}`,
		`{"schema":1,"languageId":"l","name":"A","invented":true}`,
		`{"schema":1,"languageId":"l","name":"A"} {}`,
		`{"schema":1,"languageId":"l","name":"\ud800"}`,
		`{"schema":1,"languageId":"l","name":"\udc00"}`,
		strings.Repeat("[", 34) + strings.Repeat("]", 34),
	} {
		if _, err := Decode([]byte(raw)); err == nil {
			t.Fatal("silently accepted invalid/ambiguous source", raw)
		}
	}
	if _, err := Decode([]byte(`{"schema":1,"languageId":"l","name":"\ud83d\ude00"}`)); err != nil {
		t.Fatal("valid surrogate pair rejected", err)
	}
	if _, err := Decode([]byte("{\"schema\":1,\"languageId\":\"l\",\"name\":\"\xff\"}")); err == nil {
		t.Fatal("invalid UTF-8 silently replaced")
	}
	var a, b Features
	_ = json.Unmarshal([]byte(`{"a":"one","b":"two"}`), &a)
	_ = json.Unmarshal([]byte(`{"b":"two","a":"one"}`), &b)
	if Hash(a) != Hash(b) {
		t.Fatal("map insertion order affected canonical hash")
	}
}
func TestLimitsCancellationAndImmutableRequestIdentity(t *testing.T) {
	c := compileFixture(t, fixture(t))
	r := request(c, "generate")
	r.Template = "vowel-consonant-vowel"
	r.Revision = "wrong"
	if got := c.Execute(context.Background(), r); got.Diagnostics[0].Code != "revision_mismatch" {
		t.Fatal(got)
	}
	r.Revision = c.report.Revision
	r.Engine = "future/2"
	if c.Execute(context.Background(), r).Status != "incompatible_engine" {
		t.Fatal("engine silently substituted")
	}
	r.Engine = EngineVersion
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if c.Execute(ctx, r).Status != "cancelled" {
		t.Fatal("ignored cancellation")
	}
	_, report := Compile(ctx, fixture(t), TokenTrieBackend{})
	if report.Status != "invalid" || report.Diagnostics[len(report.Diagnostics)-1].Code != "cancelled" {
		t.Fatal(report)
	}
	d := fixture(t)
	d.Phonology.Templates[0].Slots = make([]Match, 16)
	for i := range d.Phonology.Templates[0].Slots {
		d.Phonology.Templates[0].Slots[i] = Match{Class: "V"}
	}
	c = compileFixture(t, d)
	r = request(c, "generate")
	r.Template = d.Phonology.Templates[0].ID
	r.Policy = "seeded"
	r.Seed = "repeatable"
	got := c.Execute(context.Background(), r)
	if got.Status != "execution_limit" || got.Diagnostics[len(got.Diagnostics)-1].Code != "enumeration_limit" || len(got.Candidates) != 0 || len(got.Diagnostics) > 65 {
		t.Fatal("truncated valid set used as seed population", got)
	}
	d = fixture(t)
	d.Morphology.Paradigms[0].Cells[0].Operations[0].Kind = "arbitrary-script"
	_, report = Compile(context.Background(), d, TokenTrieBackend{})
	if report.Status != "invalid" {
		t.Fatal("unsupported mechanism silently accepted")
	}
}
func TestTransducerBothTapesEpsilonAndAmbiguousTextSegmentation(t *testing.T) {
	pairs := []Pair{{[]string{"left"}, []string{"a", "bc"}, 0}, {[]string{"right"}, []string{"ab", "c"}, 1}, {[]string{"zero", "feature"}, []string{}, 2}, {[]string{"unicode"}, []string{"d̪"}, 3}}
	relation, err := (TokenTrieBackend{}).Build(context.Background(), pairs, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pairs {
		ids, truncated, err := relation.Lookup(context.Background(), false, p.Upper, 10)
		if err != nil || truncated || !reflect.DeepEqual(ids, []int{p.Payload}) {
			t.Fatal(ids, truncated, err)
		}
		ids, truncated, err = relation.Lookup(context.Background(), true, p.Lower, 10)
		if err != nil || truncated || !reflect.DeepEqual(ids, []int{p.Payload}) {
			t.Fatal(ids, truncated, err)
		}
	}
	ids, truncated, err := relation.InverseText(context.Background(), "abc", 10)
	if err != nil || truncated || !reflect.DeepEqual(ids, []int{0, 1}) {
		t.Fatal("ambiguous grapheme segmentation lost", ids, truncated, err)
	}
	ids, truncated, err = relation.InverseText(context.Background(), "d̪", 10)
	if err != nil || truncated || !reflect.DeepEqual(ids, []int{3}) {
		t.Fatal("combining grapheme split", ids, truncated, err)
	}
	if _, err = (TokenTrieBackend{}).Build(context.Background(), pairs, 2); err == nil {
		t.Fatal("state budget ignored")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err = relation.Lookup(ctx, false, pairs[0].Upper, 10); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestWholeFormNormalizationAndAnalysis(t *testing.T) {
	d := fixture(t)
	d.Orthography.Mappings[0].Grapheme = "A"
	d.Orthography.Mappings[2].Grapheme = "\u030a" // Combining ring follows A in tree's surface.
	// Make t's realization p, so the authored sequence A + ring crosses mappings.
	d.Phonology.Rules[0].Output = []string{"p"}
	c := compileFixture(t, d)
	form := inflect(c, "tree", "one").Candidates[0]
	if form.Written != "ÅA" {
		t.Fatalf("whole-form NFC not respected: %q", form.Written)
	}
	r := request(c, "analyze-written")
	r.Text = "A\u030aA"
	got := c.Execute(context.Background(), r)
	if got.Status != "success" || got.Candidates[0].Lexeme != "tree" {
		t.Fatal("NFC reverse analysis failed", got)
	}
}
