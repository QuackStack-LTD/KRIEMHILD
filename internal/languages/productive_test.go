package languages

import (
	"context"
	"encoding/json"
	"kriemhild/internal/languages/spike"
	"os"
	"reflect"
	"strings"
	"testing"
)

func productiveFixture(t testing.TB) Definition {
	t.Helper()
	raw, err := os.ReadFile("testdata/productive.json")
	if err != nil {
		t.Fatal(err)
	}
	d, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func productiveRequest(a *Artifact, operation string) Request {
	return Request{Request: spike.Request{Revision: a.report.Revision, Engine: a.report.Engine, Operation: operation, Policy: "all", Limit: 64}, Adapter: a.report.Adapter, Compiler: a.report.Compiler}
}
func TestProductiveCompositionAndCompleteInverse(t *testing.T) {
	d := productiveFixture(t)
	before := string(Canonical(d))
	a := Compile(context.Background(), d, nil)
	if !a.report.Ready["inflect"] || !a.report.Ready["analyze"] || a.report.Forms != 0 || a.morphology.Report().Forms != 0 {
		t.Fatalf("productive compilation must not expand surface forms: %+v", a.report)
	}
	q := productiveRequest(a, "inflect")
	q.Lexeme = "stone"
	q.Features = Features{"number": "many", "shape": "plain"}
	r := a.Execute(context.Background(), q)
	if r.Status != "success" || len(r.Candidates) != 1 || !reflect.DeepEqual(r.Candidates[0].Segments, []string{"p", "a", "d", "a"}) {
		t.Fatalf("regular authored suffix/stratum failed: %+v", r)
	}
	if r.Schema != 2 || r.Compiler != ProductiveCompiler || len(r.Derivations) != 1 || len(r.Derivations[0].Steps) != 5 {
		t.Fatal("missing compiler/boundary provenance", r)
	}
	steps := r.Derivations[0].Steps
	if steps[2].Allomorph != "after-t" || steps[2].Alternatives[1].Decision != "precedence-skipped" || steps[3].After[3].Boundary != "morpheme" || steps[4].Stage != "erase-boundaries" {
		t.Fatal("allomorph precedence or boundary retention failed", steps)
	}
	q.Features["shape"] = "framed"
	r = a.Execute(context.Background(), q)
	if r.Status != "success" || !reflect.DeepEqual(r.Candidates[0].Segments, []string{"i", "p", "a", "d", "a"}) || r.Derivations[0].Steps[3].Allomorph != "frame-confirm" {
		t.Fatal("circumfix edges or ordered epsilon stage failed", r)
	}
	q.Lexeme = "person"
	q.Features["shape"] = "plain"
	r = a.Execute(context.Background(), q)
	if r.Status != "success" || !reflect.DeepEqual(r.Candidates[0].Segments, []string{"n", "i", "n"}) || r.Derivations[0].Steps[2].Stage != "suppletion" {
		t.Fatal("irregular override did not bypass regular plan", r)
	}
	// Every forward form in this declared domain must be recovered by inverse
	// composition, including homophones and cell identity.
	for _, lex := range d.Lexicon {
		q = productiveRequest(a, "paradigm")
		q.Lexeme = lex.ID
		forward := a.Execute(context.Background(), q)
		for _, candidate := range forward.Candidates {
			inverse := productiveRequest(a, "analyze")
			inverse.Segments = candidate.Segments
			analysis := a.Execute(context.Background(), inverse)
			found := false
			for _, c := range analysis.Candidates {
				if string(Canonical(c)) == string(Canonical(candidate)) {
					found = true
				}
			}
			if !found {
				t.Fatalf("inverse lost %+v in %+v", candidate, analysis)
			}
		}
	}
	q = productiveRequest(a, "analyze")
	q.Segments = []string{"p", "a", "d", "a"}
	r = a.Execute(context.Background(), q)
	if r.Status != "ambiguous" || r.KnownValid != 2 || len(r.Derivations) != 2 {
		t.Fatal("homophonic analyses collapsed", r)
	}
	q.Policy = "explicit"
	if r = a.Execute(context.Background(), q); r.Status != "choice_required" {
		t.Fatal("ambiguity hidden", r)
	}
	q.Policy = "all"
	q.Limit = 1
	if r = a.Execute(context.Background(), q); r.Status != "candidate_limit" || r.KnownValid != 2 || !r.Truncated {
		t.Fatal("display cap confused with complete domain", r)
	}
	q.Policy = "seeded"
	q.Seed = "author-choice"
	expected := string(Canonical(a.Execute(context.Background(), q)))
	for i := 0; i < 20; i++ {
		if string(Canonical(a.Execute(context.Background(), q))) != expected {
			t.Fatal("seeded replay differed")
		}
	}
	if string(Canonical(d)) != before {
		t.Fatal("execution changed authored input")
	}
}

func TestProductiveValidationPartialReadinessAndVersions(t *testing.T) {
	d := productiveFixture(t)
	for _, mutate := range []func(*Definition){
		func(d *Definition) { d.Productive.Paradigms[0].Cells[1].Operations[0].Kind = "reduplication" },
		func(d *Definition) { d.Productive.Stratum.Rules[0].Input = []spike.Match{{Boundary: "morpheme"}} },
		func(d *Definition) { d.Productive.Paradigms[0].Cells[0].Operations[0].Allomorphs[0].When = nil },
		func(d *Definition) { d.Productive.Stratum.BoundaryPolicy = "erase" },
		func(d *Definition) { d.Lexicon[0].Paradigm = "missing" },
		func(d *Definition) { d.Productive.Features[0].Values = []string{"one", "one"} },
	} {
		invalid := Clone(d)
		mutate(&invalid)
		a := Compile(context.Background(), invalid, nil)
		if a.report.Ready["inflect"] || !a.report.Ready["validate"] || a.report.Status != "partial" {
			t.Fatal("invalid productive module escaped readiness or disabled independent phonology", a.report)
		}
	}
	old, _ := productFixture(t)
	if !CompatibleVersions(1, EngineVersion, AdapterVersion, "") || CompatibleVersions(1, ProductiveEngine, ProductiveAdapter, ProductiveCompiler) {
		t.Fatal("version dispatch reinterprets old source")
	}
	old.Productive = d.Productive
	if _, err := Decode(Canonical(old)); err == nil {
		t.Fatal("schema 1 accepted new rules")
	}
	d.Morphology = &spike.Morphology{}
	if _, err := Decode(Canonical(d)); err == nil {
		t.Fatal("two morphological authorities accepted")
	}
	raw := string(Canonical(productiveFixture(t)))
	raw = strings.Replace(raw, `"version":"productive/1"`, `"version":"productive/1","unknown":true`, 1)
	if _, err := Decode([]byte(raw)); err == nil {
		t.Fatal("unknown field discarded")
	}
	a := Compile(context.Background(), productiveFixture(t), nil)
	q := productiveRequest(a, "inflect")
	q.Engine = EngineVersion
	if r := a.Execute(context.Background(), q); r.Status != "incompatible_engine" {
		t.Fatal("engine fallback", r)
	}
	q = productiveRequest(a, "inflect")
	q.Lexeme = "stone"
	q.Features = Features{"z": "bad", "a": "bad"}
	expected := string(Canonical(a.Execute(context.Background(), q)))
	for i := 0; i < 40; i++ {
		if string(Canonical(a.Execute(context.Background(), q))) != expected {
			t.Fatal("map iteration changed invalid-input diagnostic")
		}
	}
	if !strings.Contains(expected, "a=bad") {
		t.Fatal("invalid keys not sorted")
	}
}

func TestProductiveBoundsCancellationAndNoFallback(t *testing.T) {
	d := productiveFixture(t)
	d.Productive.Paradigms[0].Cells[1].Operations[0].Allomorphs = d.Productive.Paradigms[0].Cells[1].Operations[0].Allomorphs[:1]
	a := Compile(context.Background(), d, nil)
	q := productiveRequest(a, "inflect")
	q.Lexeme = "tree"
	q.Features = Features{"number": "many", "shape": "plain"}
	if r := a.Execute(context.Background(), q); r.Status != "no_valid_candidate" || len(r.Candidates) != 0 {
		t.Fatal("invented fallback allomorph", r)
	}
	r := a.Execute(context.Background(), q)
	if len(r.RejectedDerivations) != 1 || r.RejectedDerivations[0].Failure.Code != "no_matching_allomorph" || r.RejectedDerivations[0].Steps[2].Alternatives[0].Decision != "condition-failed" {
		t.Fatal("rejected path provenance missing", r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	q.Lexeme = "stone"
	if r := a.Execute(ctx, q); r.Status != "cancelled" || len(r.Candidates) > 0 {
		t.Fatal("ignored cancellation", r)
	}
	// Nine binary choices exceed the documented live-branch bound before any
	// display or seeded choice can select from an incomplete prefix.
	d = productiveFixture(t)
	op := d.Productive.Paradigms[0].Cells[0].Operations[0]
	op.Policy = "all"
	op.Allomorphs = append(op.Allomorphs, Clone(op.Allomorphs[0]))
	op.Allomorphs[1].ID = "other-zero"
	for i := 1; i < 10; i++ {
		next := Clone(op)
		next.ID = string(rune('a' + i))
		d.Productive.Paradigms[0].Cells[0].Operations = append(d.Productive.Paradigms[0].Cells[0].Operations, next)
	}
	a = Compile(context.Background(), d, nil)
	q = productiveRequest(a, "inflect")
	q.Lexeme = "stone"
	q.Features = Features{"number": "one"}
	q.Policy = "seeded"
	q.Seed = "no-prefix-choice"
	if r := a.Execute(context.Background(), q); r.Status != "execution_limit" || len(r.Candidates) > 0 || r.KnownValid != 0 {
		t.Fatal("work limit concealed as empty/partial success", r)
	}
	budget := &productiveBudget{ctx: context.Background(), steps: productiveSteps}
	if err := budget.charge(1, 0); err == nil {
		t.Fatal("step limit unenforced")
	}
	budget = &productiveBudget{ctx: context.Background()}
	if err := budget.charge(1, productiveBytes+1); err == nil {
		t.Fatal("payload limit unenforced")
	}
	if r := PreviewEvolution(context.Background(), d, d); r.Status != "unsupported_operation" {
		t.Fatal("historical flow silently reinterpreted productive plans")
	}
}

func TestProductiveOrderedRulesAllAnalysesAndPinnedWriting(t *testing.T) {
	d := productiveFixture(t)
	// Equivalent surface strings remain distinct allomorph derivations.
	op := &d.Productive.Paradigms[0].Cells[0].Operations[0]
	op.Policy = "all"
	op.Allomorphs = append(op.Allomorphs, Clone(op.Allomorphs[0]))
	op.Allomorphs[1].ID = "second-zero"
	a := Compile(context.Background(), d, nil)
	q := productiveRequest(a, "inflect")
	q.Lexeme = "stone"
	q.Features = Features{"number": "one"}
	r := a.Execute(context.Background(), q)
	if r.Status != "ambiguous" || r.KnownValid != 2 || r.Derivations[0].Steps[2].Allomorph == r.Derivations[1].Steps[2].Allomorph {
		t.Fatal("distinct epsilon derivations collapsed", r)
	}
	inverseZero := productiveRequest(a, "analyze")
	inverseZero.Segments = r.Candidates[0].Segments
	if inverse := a.Execute(context.Background(), inverseZero); inverse.Status != "ambiguous" || inverse.KnownValid != 4 {
		t.Fatal("inverse collapsed two lexical identities and two allomorph derivations", inverse)
	}
	d = productiveFixture(t)
	// Insertion and deletion see separate snapshots; reversing their authored
	// order changes the form. A segment rule cannot consume across a boundary.
	d.Productive.Stratum.Rules = append(d.Productive.Stratum.Rules,
		spike.SoundRule{ID: "delete-initial-p", Domain: "morpheme", Input: []spike.Match{{Segment: "p"}}, Left: []spike.Match{{Boundary: "word"}}},
		spike.SoundRule{ID: "restore-initial-p", Domain: "morpheme", Output: []string{"p"}, Left: []spike.Match{{Boundary: "word"}}, Right: []spike.Match{{Segment: "a"}}})
	a = Compile(context.Background(), d, nil)
	q = productiveRequest(a, "inflect")
	q.Lexeme = "stone"
	q.Features = Features{"number": "many", "shape": "plain"}
	r = a.Execute(context.Background(), q)
	if r.Status != "success" || !reflect.DeepEqual(r.Candidates[0].Segments, []string{"p", "a", "d", "a"}) {
		t.Fatal("ordered insertion/deletion failed", r)
	}
	d.Productive.Stratum.Rules[1], d.Productive.Stratum.Rules[2] = d.Productive.Stratum.Rules[2], d.Productive.Stratum.Rules[1]
	a = Compile(context.Background(), d, nil)
	q.Revision = a.report.Revision
	r = a.Execute(context.Background(), q)
	if r.Status != "success" || !reflect.DeepEqual(r.Candidates[0].Segments, []string{"a", "d", "a"}) {
		t.Fatal("rule order ignored", r)
	}
	d = productiveFixture(t)
	d.Productive.Stratum.Rules[0].Input = append(d.Productive.Stratum.Rules[0].Input, spike.Match{Segment: "a"})
	d.Productive.Stratum.Rules[0].Right = nil
	a = Compile(context.Background(), d, nil)
	q.Revision = a.report.Revision
	r = a.Execute(context.Background(), q)
	if r.Status != "success" || !reflect.DeepEqual(r.Candidates[0].Segments, []string{"p", "a", "t", "a"}) {
		t.Fatal("segment match consumed a morpheme boundary", r)
	}
	d = productiveFixture(t)
	d.Lexicon[2].Overrides = nil
	a = Compile(context.Background(), d, nil)
	q.Revision = a.report.Revision
	q.Lexeme = "person"
	r = a.Execute(context.Background(), q)
	if r.Status != "success" || !reflect.DeepEqual(r.Candidates[0].Segments, []string{"n", "a", "n", "i"}) || r.Derivations[0].Steps[2].Allomorph != "after-n" {
		t.Fatal("conditioned alternate after n failed", r)
	}
	_, deps := productFixture(t)
	legacy, _ := productFixture(t)
	d.Orthography = legacy.Orthography
	a = Compile(context.Background(), d, deps)
	q = productiveRequest(a, "inflect")
	q.Lexeme = "stone"
	q.Features = Features{"number": "many", "shape": "plain"}
	r = a.Execute(context.Background(), q)
	if r.Status != "success" || r.Candidates[0].Written != "pad̪a" {
		t.Fatal("pinned writing did not compose after surface validation", r)
	}
	inverse := productiveRequest(a, "analyze-written")
	inverse.Text = r.Candidates[0].Written
	if result := a.Execute(context.Background(), inverse); result.Status != "ambiguous" || result.KnownValid != 2 {
		t.Fatal("written inverse lost ambiguity", result)
	}
	for ref, s := range deps {
		s.Name = "changed draft"
		deps[ref] = s
	}
	if string(Canonical(a.Execute(context.Background(), q))) != string(Canonical(r)) {
		t.Fatal("mutating script draft changed immutable artifact")
	}
	a2 := Compile(context.Background(), d, deps)
	if a2.report.Ready["write"] || !a2.report.Ready["inflect"] {
		t.Fatal("unpinned script was accepted or disabled independent morphology", a2.report)
	}
	// Corpus expectations include the typed states, not just final strings.
	q.Revision = ""
	d.Tests = []RegressionCase{{ID: "typed-stone-many", Request: q, ExpectedStatus: r.Status, Expected: r.Candidates, ExpectedDerivations: r.Derivations}}
	_, deps = productFixture(t)
	a = Compile(context.Background(), d, deps)
	corpus := productiveRequest(a, "regression")
	corpus.Policy = "explicit"
	if result := a.Execute(context.Background(), corpus); result.Status != "success" {
		t.Fatal("typed corpus did not replay", result)
	}
	d.Tests[0].ExpectedDerivations[0].Steps[0].RuleID = "tampered"
	a = Compile(context.Background(), d, deps)
	corpus.Revision = a.report.Revision
	if result := a.Execute(context.Background(), corpus); result.Status != "regression_failure" {
		t.Fatal("typed trace mismatch hidden", result)
	}
	d.Tests[0].Request.Policy = "seeded"
	d.Tests[0].Request.Seed = "revision-sensitive"
	a = Compile(context.Background(), d, deps)
	if a.report.Ready["regression"] || !a.report.Ready["inflect"] {
		t.Fatal("revision-sensitive corpus silently advertised as executable", a.report)
	}
}

func TestProductiveGoldenReplay(t *testing.T) {
	d := productiveFixture(t)
	a := Compile(context.Background(), d, nil)
	for _, name := range []string{"productive-inflect", "productive-inverse"} {
		raw, err := os.ReadFile("testdata/" + name + ".request.json")
		if err != nil {
			t.Fatal(err)
		}
		var q Request
		if err = json.Unmarshal(raw, &q); err != nil {
			t.Fatal(err)
		}
		raw, err = os.ReadFile("testdata/" + name + ".golden.json")
		if err != nil {
			t.Fatal(err)
		}
		var expected Result
		if err = json.Unmarshal(raw, &expected); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 10; i++ {
			if string(Canonical(a.Execute(context.Background(), q))) != string(Canonical(expected)) {
				t.Fatalf("%s full derivation golden changed", name)
			}
		}
	}
}
