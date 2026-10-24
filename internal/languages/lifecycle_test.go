package languages

import (
	"context"
	"fmt"
	"kriemhild/internal/languages/spike"
	"reflect"
	"sync"
	"testing"
)

func TestHistoricalPreviewIsDeterministicAndPreservesSource(t *testing.T) {
	source, _ := reference(t)
	before := string(Canonical(source))
	target := Clone(source)
	target.LanguageID = "33333333333333333333333333333333"
	target.Name = "Descendant"
	target.Lexicon = nil
	target.History = &Historical{SourceLanguage: source.LanguageID, SourceRevision: Hash(source), SourceAge: "44444444444444444444444444444444", Changes: []spike.SoundRule{{ID: "historical-voicing", Domain: "word", Input: []spike.Match{{Segment: "t"}}, Output: []string{"d"}}}, Exceptions: []HistoricalException{{Lexeme: "person", RuleID: "person-exception", Segments: []string{"n", "i", "n"}}}}
	preview := PreviewEvolution(context.Background(), source, target)
	if preview.Status != "success" || preview.Target == nil || len(preview.Changes) != 4 || preview.PreviewHash == "" {
		t.Fatalf("invalid preview %+v", preview)
	}
	forms := map[string][]string{}
	for _, lex := range preview.Target.Lexicon {
		forms[lex.ID] = lex.Underlying
	}
	if !reflect.DeepEqual(forms["stone"], []string{"p", "a", "d"}) || !reflect.DeepEqual(forms["person"], []string{"n", "i", "n"}) {
		t.Fatal("wrong historical forms", forms)
	}
	if string(Canonical(source)) != before || len(target.Lexicon) != 0 {
		t.Fatal("historical preview changed its source or target")
	}
	if string(Canonical(preview)) != string(Canonical(PreviewEvolution(context.Background(), source, target))) {
		t.Fatal("historical preview changed across executions")
	}
	target.History.Exceptions[0].Segments = []string{"n", "n", "n"}
	if r := PreviewEvolution(context.Background(), source, target); r.Status == "success" || r.Target != nil {
		t.Fatal("invalid batch yielded an applicable target")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r := PreviewEvolution(ctx, source, target); r.Status != "cancelled" {
		t.Fatal("historical batch ignored cancellation")
	}
}
func TestSentenceAnalysisAndCorpus(t *testing.T) {
	d, deps := reference(t)
	d.Syntax = &Syntax{Constructions: []Construction{{ID: "pair", Meaning: "pair", Separator: " / ", Roles: []Role{{ID: "left", Category: "noun", Features: Features{"number": "many"}}, {ID: "right", Category: "noun", Features: Features{}}}, Order: []Constituent{{Role: "left"}, {Role: "right"}}, Agreements: []Agreement{{ID: "number-agreement", LeftRole: "left", RightRole: "right", Features: []string{"number"}}}}}}
	a := Compile(context.Background(), d, deps)
	q := request(a, "parse")
	q.Text = "pad̪a / nin"
	r := a.Execute(context.Background(), q)
	if r.Status != "ambiguous" || r.KnownValid != 2 || len(r.Sentences) != 2 {
		t.Fatalf("homophonic analysis lost: %+v", r)
	}
	q.Policy = "seeded"
	q.Seed = "author-seed"
	seeded := a.Execute(context.Background(), q)
	if seeded.Status != "success" || len(seeded.Sentences) != 1 || seeded.KnownValid != 2 {
		t.Fatal("seeded parse ignored complete ambiguity")
	}
	for i := 0; i < 10; i++ {
		if !reflect.DeepEqual(seeded, a.Execute(context.Background(), q)) {
			t.Fatal("sentence selection was unstable")
		}
	}
	q.Policy = "all"
	q.Seed = ""
	q.Limit = 1
	if r = a.Execute(context.Background(), q); r.Status != "candidate_limit" || !r.Truncated || r.KnownValid != 2 {
		t.Fatal("parse cap concealed alternatives")
	}
	q.Text = "pad̪a / nan"
	q.Limit = 64
	if r = a.Execute(context.Background(), q); r.Status != "no_valid_candidate" {
		t.Fatal("parser ignored agreement")
	}
	word := request(a, "inflect")
	word.Lexeme = "stone"
	word.Features = Features{"number": "many"}
	expected := a.Execute(context.Background(), word)
	word.Revision = ""
	d.Tests = []RegressionCase{{ID: "stone-many", Request: word, ExpectedStatus: "success", Expected: expected.Candidates}}
	a = Compile(context.Background(), d, deps)
	q = request(a, "regression")
	q.Policy = "explicit"
	r = a.Execute(context.Background(), q)
	if r.Status != "success" || len(r.Tests) != 1 || !r.Tests[0].Passed {
		t.Fatalf("authored corpus failed %+v", r)
	}
	d.Tests[0].ExpectedStatus = "no_valid_candidate"
	a = Compile(context.Background(), d, deps)
	q = request(a, "regression")
	q.Policy = "explicit"
	if r = a.Execute(context.Background(), q); r.Status != "regression_failure" || r.Rejected != 1 {
		t.Fatal("regression failure hidden")
	}
}
func TestArtifactCacheBudgetsIsolationAndConcurrency(t *testing.T) {
	d, deps := reference(t)
	a := Compile(context.Background(), d, deps)
	cache := NewCache(a.AccountedBytes() * 2)
	key := ArtifactKey("world-a", "age", d)
	if key == ArtifactKey("world-b", "age", d) || key == ArtifactKey("world-a", "other-age", d) {
		t.Fatal("cache key lost resource scope")
	}
	if !cache.Put(key, a) || cache.Get(key) != a {
		t.Fatal("cache did not retain immutable artifact")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				cache.Get(key)
				cache.Put(key, a)
			}
		}()
	}
	wg.Wait()
	for i := 0; i < 10; i++ {
		cache.Put(string(rune('a'+i)), a)
	}
	n, used, budget := cache.Stats()
	if n > 2 || used > budget || cache.Get(key) != nil {
		t.Fatal("LRU exceeded budget or failed to evict")
	}
	if NewCache(1).Put(key, a) {
		t.Fatal("oversized artifact retained")
	}
	changed := Clone(d)
	changed.Notes = "a changed definition"
	if ArtifactKey("world-a", "age", changed) == key {
		t.Fatal("source revision did not invalidate cache key")
	}
}
func TestLexicalStructureRetainsAuthoredFoundations(t *testing.T) {
	d, deps := reference(t)
	d.Lexical = &LexicalModel{Morphemes: []Morpheme{{ID: "stone-root", Kind: "root", Underlying: []string{"p", "a", "t"}, Meanings: []string{"stone material"}, Features: Features{}}}, Entries: []LexicalEntry{{Lexeme: "stone", Morphemes: []string{"stone-root"}}}, Relations: []LexicalRelation{{ID: "stone-rock", Kind: "related-meaning", Source: "stone", Target: "rock"}}, Attestations: []Attestation{{ID: "stone-attested", Lexeme: "stone", Form: "pat", Source: "Authored reference corpus"}}}
	raw := Canonical(d)
	decoded, err := Decode(raw)
	if err != nil || string(Canonical(decoded)) != string(raw) {
		t.Fatal("lexical source data did not round-trip", err)
	}
	a := Compile(context.Background(), d, deps)
	if a.Report().Status != "compiled" || !a.Report().Ready["inflect"] {
		t.Fatal("valid lexical annotations did not compile", a.Report())
	}
	d.Lexical.Derivations = []Derivation{{ID: "stone-cycle", Kind: "compound", Inputs: []string{"stone"}, Target: "stone"}}
	a = Compile(context.Background(), d, deps)
	found := false
	for _, v := range a.Report().Diagnostics {
		if v.Code == "invalid_lexical_structure" {
			found = true
		}
	}
	if !found {
		t.Fatal("cyclic derivation was accepted")
	}
	q := request(a, "derive")
	if r := a.Execute(context.Background(), q); r.Status != "unsupported_operation" || len(r.Candidates) > 0 {
		t.Fatal("annotation was falsely executed as a derivational rule")
	}
}
func TestWritingNeedsNoLexiconAndCannotChangePhonologicalCandidates(t *testing.T) {
	d, deps := reference(t)
	d.Lexicon = nil
	d.Morphology = nil
	a := Compile(context.Background(), d, deps)
	if !a.Report().Ready["write"] || !a.Report().Ready["generate"] || a.Report().Ready["inflect"] {
		t.Fatal("writing incorrectly depends on morphology")
	}
	q := request(a, "write")
	q.Segments = []string{"p", "a", "d", "a"}
	r := a.Execute(context.Background(), q)
	if r.Status != "success" || r.Candidates[0].Written != "pad̪a" {
		t.Fatal("explicit transcription failed", r)
	}
	q = request(a, "generate")
	q.Template = "vowel-consonant-vowel"
	generated := a.Execute(context.Background(), q)
	if generated.KnownValid != 11 || generated.Candidates[0].Written == "" {
		t.Fatal("phonology-only generation lost its valid writing system")
	}
	d.Orthography.Mappings = d.Orthography.Mappings[:1]
	a = Compile(context.Background(), d, deps)
	q = request(a, "generate")
	q.Template = "vowel-consonant-vowel"
	r = a.Execute(context.Background(), q)
	if r.KnownValid != 11 || a.Report().Ready["write"] || r.Candidates[0].Written != "" {
		t.Fatal("incomplete spelling changed valid phonological candidate set")
	}
}
func TestProductDiagnosticsStayBoundedAndCountOmissions(t *testing.T) {
	d, deps := reference(t)
	d.Lexical = &LexicalModel{}
	for i := 0; i < 300; i++ {
		d.Lexical.Morphemes = append(d.Lexical.Morphemes, Morpheme{ID: fmt.Sprintf("m-%d", i), Underlying: []string{"missing"}})
	}
	report := Compile(context.Background(), d, deps).Report()
	if len(report.Diagnostics) != 256 || report.DiagnosticsOmitted != 44 {
		t.Fatalf("diagnostic truncation was not explicit: %d / %d", len(report.Diagnostics), report.DiagnosticsOmitted)
	}
}
