package languages

import (
	"context"
	"fmt"
	"kriemhild/internal/languages/spike"
	"os"
	"strings"
	"testing"
)

func reviewedSequenceCorpus(t testing.TB) (SequenceCorpus, CorpusResolver) {
	t.Helper()
	raw, err := os.ReadFile("testdata/sequence.corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := DecodeSequenceCorpus(raw)
	if err != nil {
		t.Fatal(err)
	}
	bodies := map[string][]byte{}
	for _, name := range []string{"sequence.json", "sequence-normalized.json"} {
		raw, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		d, err := Decode(raw)
		if err != nil {
			t.Fatal(err)
		}
		bodies["language"+d.LanguageID+Hash(d)] = Canonical(d)
	}
	raw, err = os.ReadFile("testdata/sequence-script.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := DecodeScript(raw)
	if err != nil {
		t.Fatal(err)
	}
	bodies["script"+s.ScriptID+Hash(s)] = Canonical(s)
	return c, func(_ context.Context, kind, id, revision string) ([]byte, error) {
		if raw, ok := bodies[kind+id+revision]; ok {
			return append([]byte{}, raw...), nil
		}
		return nil, fmt.Errorf("missing pin")
	}
}
func TestSequenceCorpusReviewedGoldensProductionAndDeterminism(t *testing.T) {
	c, resolve := reviewedSequenceCorpus(t)
	for _, test := range c.Cases {
		raw, err := os.ReadFile("testdata/" + test.ID + ".golden.json")
		if err != nil {
			t.Fatal(err)
		}
		var expected Result
		if err := spike.DecodeJSON(raw, &expected); err != nil {
			t.Fatal(err)
		}
		if string(Canonical(expected)) != string(Canonical(test.Expected)) {
			t.Fatal("reviewed expectation was regenerated", test.ID)
		}
	}
	validated, err := RunSequenceCorpus(context.Background(), c, resolve, true)
	if err != nil || validated.Status != "success" || validated.Calls != 0 || len(validated.Cases) != 0 {
		t.Fatal(validated, err)
	}
	first, err := RunSequenceCorpus(context.Background(), c, resolve, false)
	if err != nil || first.Status != "success" || first.Calls != 18 || first.Cases[0].DomainInputs != 7 {
		t.Fatal(first, err)
	}
	if first.Cases[1].Actual.KnownValid != 2 || len(first.Cases[1].Actual.Spelling.Preimages[0].Analyses) != 2 {
		t.Fatal("homophones collapsed")
	}
	expected := string(Canonical(first))
	first.Cases[0].Actual.Spelling.Spellings[0].Output[0].Unit.Sign.Sign = "caller mutation"
	for i := 0; i < 3; i++ {
		next, err := RunSequenceCorpus(context.Background(), c, resolve, false)
		if err != nil || string(Canonical(next)) != expected {
			t.Fatal("restart/rebuild/caller mutation changed result", err)
		}
	}
	backwards := Clone(c)
	for i, j := 0, len(c.Cases)-1; i < j; i, j = i+1, j-1 {
		backwards.Cases[i], backwards.Cases[j] = backwards.Cases[j], backwards.Cases[i]
	}
	r, err := RunSequenceCorpus(context.Background(), backwards, resolve, false)
	if err != nil || r.Status != "success" {
		t.Fatal("case order changed pinned output", err, r.Status)
	}
}
func TestSequenceCorpusStrictDecodeAndExpectationValidation(t *testing.T) {
	c, _ := reviewedSequenceCorpus(t)
	raw := Canonical(c)
	for name, bad := range map[string][]byte{
		"malformed": []byte(`{"version":`), "unknown": []byte(strings.Replace(string(raw), `"version":`, `"unknown":true,"version":`, 1)),
		"duplicate-field": []byte(strings.Replace(string(raw), `"version":`, `"version":"bad","version":`, 1)),
		"invalid-utf8":    append([]byte{'"'}, 0xff, '"'), "surrogate": []byte(`{"id":"\ud800"}`), "too-large": make([]byte, (2<<20)+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeSequenceCorpus(bad); err == nil {
				t.Fatal("accepted malformed corpus")
			}
		})
	}
	for _, test := range []struct {
		name string
		edit func(*SequenceCorpus)
	}{
		{"schema", func(c *SequenceCorpus) { c.Version = "sequence-corpus/2" }},
		{"typed-invalid-utf8", func(c *SequenceCorpus) { c.Cases[0].Description = string([]byte{0xff}) }},
		{"oversized-string", func(c *SequenceCorpus) {
			c.Cases[0].Expected.Diagnostics = append(c.Cases[0].Expected.Diagnostics, Diagnostic{Message: strings.Repeat("x", 65537)})
		}},
		{"duplicate-case", func(c *SequenceCorpus) { c.Cases[1].ID = c.Cases[0].ID }},
		{"missing-pin", func(c *SequenceCorpus) { c.Cases[0].Pin.Revision = "" }},
		{"compiler", func(c *SequenceCorpus) { c.Cases[0].Pin.Compiler = "latest" }},
		{"reviewer", func(c *SequenceCorpus) { c.Cases[0].Review.Reviewer = "" }},
		{"layer", func(c *SequenceCorpus) { c.Cases[0].InputLayer = "pixel" }},
		{"status", func(c *SequenceCorpus) { c.Cases[0].Outcome = "no_result" }},
		{"raw-status", func(c *SequenceCorpus) { c.Cases[0].Expected.Status = "made-up"; c.Cases[0].Outcome = "rejected" }},
		{"script", func(c *SequenceCorpus) { c.Cases[0].Expected.Spelling.Script.Revision = strings.Repeat("a", 64) }},
		{"byte-span", func(c *SequenceCorpus) { c.Cases[2].Expected.Spelling.Clusters[1].ByteEnd++ }},
		{"utf16-span", func(c *SequenceCorpus) { c.Cases[2].Expected.Spelling.Clusters[1].UTF16End++ }},
		{"code-points", func(c *SequenceCorpus) { c.Cases[2].Expected.Spelling.Clusters[1].CodePoints[0] = "U+0000" }},
		{"cluster-span", func(c *SequenceCorpus) { c.Cases[2].Expected.Spelling.Clusters[1].Index++ }},
		{"rule-span", func(c *SequenceCorpus) { c.Cases[0].Expected.Spelling.Spellings[0].Stages[0].Matches[0].End = 999 }},
		{"provenance", func(c *SequenceCorpus) {
			c.Cases[0].Expected.Spelling.Spellings[0].Stages[0].After[0].Sources = []int{-1}
		}},
		{"boundary", func(c *SequenceCorpus) {
			c.Cases[0].Request.Spelling = &SpellingRequest{Units: []OrthoUnit{{Kind: "boundary", Value: "morpheme"}}}
		}},
		{"limit-partial", func(c *SequenceCorpus) {
			c.Cases[0].Outcome = "limit_reached"
			c.Cases[0].Expected.Status = "execution_limit"
		}},
		{"false-domain", func(c *SequenceCorpus) { c.Cases[0].Domain.MaxLength = 4 }},
		{"domain-duplicates", func(c *SequenceCorpus) {
			c.Cases[0].Domain.Alphabet = append(c.Cases[0].Domain.Alphabet, c.Cases[0].Domain.Alphabet[0])
		}},
		{"case-count", func(c *SequenceCorpus) {
			for len(c.Cases) < 33 {
				c.Cases = append(c.Cases, c.Cases[0])
			}
		}},
		{"output-budget", func(c *SequenceCorpus) { c.Bounds.ReportBytes = 1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			bad := Clone(c)
			test.edit(&bad)
			if ValidateSequenceCorpus(bad) == nil {
				t.Fatal("accepted invalid corpus")
			}
		})
	}
}
func TestSequenceCorpusPreflightMissingPinsFailClosed(t *testing.T) {
	c, resolve := reviewedSequenceCorpus(t)
	for _, kind := range []string{"language", "script"} {
		calls := 0
		bad := func(ctx context.Context, k, id, revision string) ([]byte, error) {
			calls++
			if k == kind {
				return nil, fmt.Errorf("missing")
			}
			return resolve(ctx, k, id, revision)
		}
		r, err := RunSequenceCorpus(context.Background(), c, bad, false)
		if err == nil || r.Calls != 0 || len(r.Cases) > 0 || calls == 0 {
			t.Fatal("executed before resolving complete corpus")
		}
		bad = func(ctx context.Context, k, id, revision string) ([]byte, error) {
			raw, err := resolve(ctx, k, id, revision)
			if k == kind {
				raw = []byte(strings.Replace(string(raw), `"name":`, `"name":"changed", "notes":`, 1))
			}
			return raw, err
		}
		if _, err := RunSequenceCorpus(context.Background(), c, bad, true); err == nil {
			t.Fatal("accepted corrupt pin")
		}
	}
	late := Clone(c)
	late.Cases[3].Pin.Revision = strings.Repeat("a", 64)
	late.Cases[3].Request.Revision = late.Cases[3].Pin.Revision
	late.Cases[3].Expected.Input = late.Cases[3].Request
	late.Cases[3].Expected.Revision = late.Cases[3].Pin.Revision
	r, err := RunSequenceCorpus(context.Background(), late, resolve, false)
	if err == nil || r.Calls != 0 {
		t.Fatal("late invalid pin allowed partial execution")
	}
}
func TestSequenceCorpusDiffReviewCancellationAndBudgets(t *testing.T) {
	c, resolve := reviewedSequenceCorpus(t)
	wrong := Clone(c)
	wrong.Cases[0].Expected.Spelling.Spellings[0].Stages[0].Matches[0].Rule = "wrong-reviewed-rule"
	r, err := RunSequenceCorpus(context.Background(), wrong, resolve, false)
	if err != nil || r.Status != "failed" || !strings.Contains(r.Cases[0].Diffs[0].Path, "rule") || !strings.Contains(CorpusSummary(r), "wrong-reviewed-rule") {
		t.Fatal("no actionable diff", err, r.Status)
	}
	pending := Clone(c)
	pending.Cases[0].Review.Status = "pending"
	pending.Cases[0].Review.Reviewer = ""
	r, err = RunSequenceCorpus(context.Background(), pending, resolve, false)
	if err != nil || r.Status == "success" || r.Cases[0].Status != "review_required" {
		t.Fatal("unreviewed expectation became golden", err)
	}
	for _, edit := range []func(*SequenceCorpus){func(c *SequenceCorpus) { c.Bounds.Calls = 15 }, func(c *SequenceCorpus) { c.Bounds.Work = 100000 }, func(c *SequenceCorpus) { c.Bounds.ReportBytes = 8192 }} {
		limited := Clone(c)
		edit(&limited)
		r, err = RunSequenceCorpus(context.Background(), limited, resolve, false)
		if err != nil || r.Status != "limit_reached" || r.Calls > limited.Bounds.Calls || r.ChargedWork > limited.Bounds.Work || len(Canonical(r)) > limited.Bounds.ReportBytes {
			t.Fatal("budget not enforced", err, r.Status, len(Canonical(r)))
		}
		for _, v := range r.Cases {
			if !v.Complete && v.Passed {
				t.Fatal("incomplete case passed")
			}
		}
	}
	// Trigger cancellation inside production work after complete preflight,
	// independently of wall-clock timing or runtime scheduling.
	ctx := &sequenceCancelContext{Context: context.Background(), remaining: 1200}
	r, err = RunSequenceCorpus(ctx, c, resolve, false)
	if err != nil || r.Status != "cancelled" {
		t.Fatal("inner cancellation lost", err, r.Status)
	}
	for _, v := range r.Cases {
		if v.Status == "cancelled" && v.Passed {
			t.Fatal("cancelled case passed")
		}
	}
	ctx2, cancel := context.WithCancel(context.Background())
	cancel()
	r, err = RunSequenceCorpus(ctx2, c, resolve, false)
	if err == nil || r.Calls != 0 {
		t.Fatal("cancelled preflight executed")
	}
}
func TestSequenceCorpusUnorderedPreimagesPreservesMultiplicity(t *testing.T) {
	c, _ := reviewedSequenceCorpus(t)
	e := Clone(*c.Cases[1].Expected)
	a := Clone(e)
	// A semantic set comparison sorts whole preimages, never surface strings,
	// so duplicate multiplicity and distinct lexical analyses remain visible.
	a.Spelling.Preimages = append(a.Spelling.Preimages, a.Spelling.Preimages[0])
	if len(corpusDiffs(e, a, "unordered-preimages")) == 0 {
		t.Fatal("duplicate analysis silently discarded")
	}
}

func TestSequenceCorpusProductionInverseLimitsAndLossyReadiness(t *testing.T) {
	c, resolve := reviewedSequenceCorpus(t)
	// An intentionally too-small existing request limit exposes A2's real
	// candidate cap. The retained expected ambiguity must now fail, not become
	// a newly generated golden or silently pass with a partial candidate set.
	c.Cases = c.Cases[1:2]
	c.Cases[0].Request.Limit = 1
	c.Cases[0].Expected.Input = c.Cases[0].Request
	r, err := RunSequenceCorpus(context.Background(), c, resolve, false)
	if err != nil || r.Status != "failed" || r.Cases[0].ActualOutcome != "limit_reached" || len(r.Cases[0].Actual.Spelling.Preimages) != 0 {
		t.Fatal("production A2 limit lost", err, r.Status, r.Cases[0].ActualOutcome)
	}
	c, resolve = reviewedSequenceCorpus(t)
	c.Cases = c.Cases[3:]
	c.Cases[0].Domain = &CorpusDomain{Alphabet: []OrthoUnit{{Kind: "segment", Value: "p"}}, MaxLength: 1}
	if _, err := RunSequenceCorpus(context.Background(), c, resolve, false); err == nil {
		t.Fatal("lossy normalization was labelled exhaustive")
	}
}
