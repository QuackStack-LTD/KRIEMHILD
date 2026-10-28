package languages

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type CorpusDiff struct {
	Path     string `json:"path"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}
type CorpusCaseReport struct {
	ID                    string       `json:"id"`
	Status                string       `json:"status"`
	ExpectedOutcome       string       `json:"expectedOutcome"`
	ActualOutcome         string       `json:"actualOutcome"`
	Passed                bool         `json:"passed"`
	Reviewed              bool         `json:"reviewed"`
	Complete              bool         `json:"complete"`
	Calls                 int          `json:"calls"`
	ChargedWork           int          `json:"chargedWork"`
	ObservedSpellingSteps int          `json:"observedSpellingSteps"`
	DomainInputs          int          `json:"domainInputs"`
	Actual                *Result      `json:"actual,omitempty"`
	Diffs                 []CorpusDiff `json:"diffs"`
}
type CorpusReport struct {
	Version     string             `json:"version"`
	Corpus      string             `json:"corpus"`
	SourceHash  string             `json:"sourceHash"`
	Mode        string             `json:"mode"`
	Status      string             `json:"status"`
	Calls       int                `json:"calls"`
	ChargedWork int                `json:"chargedWork"`
	Cases       []CorpusCaseReport `json:"cases"`
}
type preparedCorpus struct {
	definition Definition
	scripts    map[ScriptRef]ScriptDefinition
}

// Resolve every case before executing any. This also snapshots caller-owned
// inputs and checks dependency ownership through the supplied resolver.
func prepareSequenceCorpus(ctx context.Context, c SequenceCorpus, resolve CorpusResolver) ([]preparedCorpus, error) {
	if err := ValidateSequenceCorpus(c); err != nil {
		return nil, err
	}
	if resolve == nil {
		return nil, corpusError(c.ID, "an explicit immutable resolver is required")
	}
	out := make([]preparedCorpus, len(c.Cases))
	for i, test := range c.Cases {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		p := test.Pin
		raw, err := resolve(ctx, "language", p.LanguageID, p.Revision)
		if err != nil {
			return nil, corpusError(test.ID, "pinned language unavailable; supply its exact retained revision")
		}
		d, err := Decode(raw)
		if err != nil {
			return nil, corpusError(test.ID, "invalid pinned source: "+err.Error())
		}
		if d.LanguageID != p.LanguageID || Hash(d) != p.Revision || d.Schema != p.Schema || d.SequenceWriting == nil {
			return nil, corpusError(test.ID, "language identity/hash/schema mismatch")
		}
		w := d.SequenceWriting
		if w.Script != p.Script || w.Segmentation != p.Segmentation || w.Normalizer != p.Normalizer || w.Renderer != p.Renderer || w.RendererAdapter != p.RendererAdapter || w.RendererCompiler != p.RendererCompiler {
			return nil, corpusError(test.ID, "source contradicts Unicode/Script/renderer pins")
		}
		if test.Request.Operation == "spell" && test.InputLayer != w.InputLayer || test.Request.Operation == "unspell" && (len(w.Stages) == 0 || test.InputLayer != w.Stages[len(w.Stages)-1].OutputLayer) {
			return nil, corpusError(test.ID, "input layer contradicts the pinned pipeline")
		}
		deps := map[ScriptRef]ScriptDefinition{}
		for _, ref := range ScriptDependencies(d) {
			body, err := resolve(ctx, "script", ref.ID, ref.Revision)
			if err != nil {
				return nil, corpusError(test.ID, "pinned Script unavailable; no active-revision fallback")
			}
			s, err := DecodeScript(body)
			if err != nil || s.ScriptID != ref.ID || Hash(s) != ref.Revision {
				return nil, corpusError(test.ID, "Script identity/hash/schema mismatch")
			}
			deps[ref] = s
		}
		if deps[p.Script].Schema != 2 {
			return nil, corpusError(test.ID, "sequence corpus requires the pinned Script 2 renderer")
		}
		if test.Expected.Spelling != nil {
			runs := append([]SpellingRun{}, test.Expected.Spelling.Spellings...)
			for _, preimage := range test.Expected.Spelling.Preimages {
				runs = append(runs, preimage.Forward)
			}
			for _, run := range runs {
				if len(run.Stages) != len(w.Stages) {
					return nil, corpusError(test.ID, "expectation omits required pipeline stages")
				}
				for j, trace := range run.Stages {
					stage := w.Stages[j]
					if trace.Stage != stage.ID || trace.Kind != stage.Kind || trace.InputLayer != stage.InputLayer || trace.OutputLayer != stage.OutputLayer {
						return nil, corpusError(test.ID, "expectation contradicts pinned stages")
					}
				}
			}
		}
		if test.Domain != nil {
			if w.Inverse != "all" {
				return nil, corpusError(test.ID, "bounded inverse cross-check requires eligible inverse")
			}
			for _, stage := range w.Stages {
				if stage.Kind != "rewrite" {
					return nil, corpusError(test.ID, "normalization is lossy; cannot claim exhaustive inverse")
				}
				for _, r := range stage.Rules {
					if len(r.Output) == 0 {
						return nil, corpusError(test.ID, "deletion is lossy; cannot claim exhaustive inverse")
					}
				}
			}
		}
		out[i] = preparedCorpus{d, deps}
	}
	return out, nil
}

// RunSequenceCorpus never saves, publishes, activates or records a compile.
// validateOnly resolves and validates pins but never compiles or executes.
func RunSequenceCorpus(ctx context.Context, corpus SequenceCorpus, resolve CorpusResolver, validateOnly bool) (CorpusReport, error) {
	if err := ValidateSequenceCorpus(corpus); err != nil {
		return CorpusReport{Version: SequenceCorpusVersion, Status: "invalid_corpus", Cases: []CorpusCaseReport{}}, err
	}
	c := Clone(corpus)
	out := CorpusReport{Version: SequenceCorpusVersion, Corpus: c.ID, SourceHash: Hash(c), Mode: "execute", Status: "success", Cases: []CorpusCaseReport{}}
	if validateOnly {
		out.Mode = "validate"
	}
	prepared, err := prepareSequenceCorpus(ctx, c, resolve)
	if err != nil {
		out.Status = "invalid_corpus"
		if ctx.Err() != nil {
			out.Status = "cancelled"
			if ctx.Err() == context.DeadlineExceeded {
				out.Status = "limit_reached"
			}
		}
		return out, err
	}
	if validateOnly {
		return out, nil
	}
	artifacts := map[string]*Artifact{}
	stopped := ""
	for i, test := range c.Cases {
		r := CorpusCaseReport{ID: test.ID, Status: "completed", ExpectedOutcome: test.Outcome, Reviewed: test.Review.Status == "reviewed", Diffs: []CorpusDiff{}}
		charge := func(q Request) bool {
			if ctx.Err() != nil {
				stopped = "cancelled"
				return false
			}
			work := 100000
			if q.Spelling != nil && q.Spelling.Analyze {
				work += 128 * 100000
			}
			if out.Calls+1 > c.Bounds.Calls || out.ChargedWork+work > c.Bounds.Work {
				stopped = "limit_reached"
				return false
			}
			out.Calls++
			out.ChargedWork += work
			r.Calls++
			r.ChargedWork += work
			return true
		}
		if stopped == "" && charge(test.Request) {
			a := artifacts[test.Pin.Revision]
			if a == nil {
				a = Compile(ctx, prepared[i].definition, prepared[i].scripts)
				artifacts[test.Pin.Revision] = a
			}
			actual := a.Execute(ctx, Clone(test.Request))
			r.Actual = &actual
			r.ActualOutcome = CorpusOutcome(actual.Status)
			if actual.Spelling != nil {
				r.ObservedSpellingSteps += actual.Spelling.Steps
			}
			r.Diffs = corpusDiffs(*test.Expected, actual, test.Comparison)
			r.Complete = true
			if test.Domain != nil && len(r.Diffs) == 0 {
				r.Status = checkCorpusDomain(ctx, a, test, *test.Domain, charge, &r)
				if r.Status == "cancelled" || r.Status == "limit_reached" {
					stopped = r.Status
					r.Complete = false
				}
			}
			if actual.Status == "cancelled" {
				stopped = "cancelled"
				r.Status = "cancelled"
				r.Complete = false
			}
			r.Passed = r.Complete && len(r.Diffs) == 0 && r.Reviewed && r.Status == "completed"
			if !r.Reviewed && r.Status == "completed" {
				r.Status = "review_required"
			}
			if len(r.Diffs) > 0 && r.Status != "cancelled" {
				r.Status = "mismatch"
			}
		} else {
			r.Status = stopped
			r.ActualOutcome = stopped
		}
		if len(Canonical(out))+len(Canonical(r)) > c.Bounds.ReportBytes-1024-(len(c.Cases)-i-1)*1024 {
			r.Actual = nil
			r.Diffs = []CorpusDiff{{Path: "report", Expected: "within reportBytes", Actual: "limit_reached"}}
			r.Status = "limit_reached"
			r.ActualOutcome = "limit_reached"
			r.Complete = false
			r.Passed = false
			stopped = "limit_reached"
		}
		out.Cases = append(out.Cases, r)
		if !r.Passed && out.Status == "success" {
			out.Status = "failed"
		}
	}
	if stopped != "" {
		out.Status = stopped
	}
	return out, nil
}

func checkCorpusDomain(ctx context.Context, a *Artifact, test SequenceCase, d CorpusDomain, charge func(Request) bool, r *CorpusCaseReport) string {
	domain := [][]OrthoUnit{{}}
	level := [][]OrthoUnit{{}}
	for n := 0; n < d.MaxLength; n++ {
		next := [][]OrthoUnit{}
		for _, prefix := range level {
			for _, u := range d.Alphabet {
				v := append(append([]OrthoUnit{}, prefix...), u)
				next = append(next, v)
				domain = append(domain, v)
			}
		}
		level = next
	}
	for _, units := range domain {
		if ctx.Err() != nil {
			return "cancelled"
		}
		q := Clone(test.Request)
		q.Lexeme = ""
		q.Features = nil
		q.Segments = nil
		q.Text = ""
		q.Spelling = &SpellingRequest{Units: units}
		if !charge(q) {
			if ctx.Err() != nil {
				return "cancelled"
			}
			return "limit_reached"
		}
		forward := a.Execute(ctx, q)
		if forward.Spelling != nil {
			r.ObservedSpellingSteps += forward.Spelling.Steps
		}
		if forward.Status != "success" || forward.Spelling == nil || len(forward.Spelling.Spellings) != 1 {
			r.Diffs = append(r.Diffs, CorpusDiff{"domain.forward", "success with one complete spelling", forward.Status})
			return "mismatch"
		}
		run := forward.Spelling.Spellings[0]
		q = Clone(test.Request)
		q.Lexeme = ""
		q.Features = nil
		q.Segments = nil
		q.Text = ""
		q.Spelling = nil
		q.Operation = "unspell"
		if forward.Spelling.OutputLayer == "sign" {
			q.Spelling = &SpellingRequest{Units: valuesUnits(run.Output)}
		} else {
			q.Text = run.Text
		}
		if !charge(q) {
			if ctx.Err() != nil {
				return "cancelled"
			}
			return "limit_reached"
		}
		inverse := a.Execute(ctx, q)
		if inverse.Spelling != nil {
			r.ObservedSpellingSteps += inverse.Spelling.Steps
		}
		if inverse.Status != "success" && inverse.Status != "ambiguous" {
			r.Diffs = append(r.Diffs, CorpusDiff{"domain.inverse", "complete success/ambiguous", inverse.Status})
			if CorpusOutcome(inverse.Status) == "limit_reached" || inverse.Status == "cancelled" {
				return CorpusOutcome(inverse.Status)
			}
			return "mismatch"
		}
		found := false
		for _, p := range inverse.Spelling.Preimages {
			if string(Canonical(p.Units)) == string(Canonical(units)) {
				found = true
			}
		}
		if !found {
			r.Diffs = append(r.Diffs, CorpusDiff{"domain.preimage", string(Canonical(units)), "missing"})
			return "mismatch"
		}
		r.DomainInputs++
	}
	return "completed"
}

func corpusDiffs(expected, actual Result, policy string) []CorpusDiff {
	if policy == "unordered-preimages" {
		expected = Clone(expected)
		actual = Clone(actual)
		for _, r := range []*Result{&expected, &actual} {
			if r.Spelling != nil {
				sort.SliceStable(r.Spelling.Preimages, func(i, j int) bool {
					return string(Canonical(r.Spelling.Preimages[i])) < string(Canonical(r.Spelling.Preimages[j]))
				})
			}
		}
	}
	var e, a any
	_ = json.Unmarshal(Canonical(expected), &e)
	_ = json.Unmarshal(Canonical(actual), &a)
	diffs := []CorpusDiff{}
	summary := func(v any) string {
		s := string(Canonical(v))
		runes := []rune(s)
		if len(runes) > 160 {
			s = string(runes[:160]) + "…"
		}
		return s
	}
	var visit func(string, any, any)
	visit = func(path string, e, a any) {
		if len(diffs) >= 20 {
			return
		}
		if string(Canonical(e)) == string(Canonical(a)) {
			return
		}
		if em, ok := e.(map[string]any); ok {
			if am, ok := a.(map[string]any); ok {
				keys := []string{}
				seen := map[string]bool{}
				for k := range em {
					seen[k] = true
					keys = append(keys, k)
				}
				for k := range am {
					if !seen[k] {
						keys = append(keys, k)
					}
				}
				sort.Strings(keys)
				for _, k := range keys {
					visit(path+"."+k, em[k], am[k])
				}
				return
			}
		}
		if es, ok := e.([]any); ok {
			if as, ok := a.([]any); ok && len(es) == len(as) {
				for i := range es {
					visit(fmt.Sprintf("%s[%d]", path, i), es[i], as[i])
				}
				return
			}
		}
		diffs = append(diffs, CorpusDiff{path, summary(e), summary(a)})
	}
	visit("result", e, a)
	return diffs
}
func CorpusSummary(r CorpusReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s: %s (%d cases, %d calls, %d charged work)\n", r.Version, r.Corpus, r.Status, len(r.Cases), r.Calls, r.ChargedWork)
	for _, c := range r.Cases {
		fmt.Fprintf(&b, "%s: %s expected=%s actual=%s reviewed=%t complete=%t\n", c.ID, c.Status, c.ExpectedOutcome, c.ActualOutcome, c.Reviewed, c.Complete)
		for _, d := range c.Diffs {
			fmt.Fprintf(&b, "  %s: expected %s; actual %s\n", d.Path, d.Expected, d.Actual)
		}
	}
	return b.String()
}
