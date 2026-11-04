package languages

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"kriemhild/internal/languages/spike"
	"sort"
)

// Expand only authored cells, unify requested features, and use the compiled
// forward relation. Missing features do not become defaults or an exact empty
// tape lookup. Reverse analyses retain distinct lexeme/cell identities.
func (a *Artifact) paradigm(ctx context.Context, q Request, out Result) Result {
	fail := func(code, message string) Result {
		out.Status = code
		out.Candidates = []spike.Candidate{}
		out.Diagnostics = []Diagnostic{{Code: code, Severity: "error", Module: "morphology", Path: "request", Message: message}}
		return out
	}
	if !a.report.Ready["inflect"] {
		return fail("missing_dependency", "Compile valid phonology, lexical entries and paradigms first.")
	}
	if q.Template != "" || len(q.Segments) > 0 || q.Text != "" || q.Limit < 1 || q.Limit > 256 || (q.Policy != "all" && q.Policy != "explicit" && q.Policy != "seeded") || q.Policy == "seeded" && q.Seed == "" || q.Policy != "seeded" && q.Seed != "" {
		return fail("invalid_input", "Inflection requires a lexical ID, authored feature values and an explicit valid selection policy.")
	}
	for key, value := range q.Features {
		found := false
		for _, f := range a.definition.Morphology.Features {
			if f.ID == key {
				for _, atom := range f.Values {
					if atom == value {
						found = true
					}
				}
			}
		}
		if !found {
			return fail("invalid_input", "Unknown grammatical feature/value: "+key)
		}
	}
	var lex *spike.Lexeme
	for i := range a.definition.Lexicon {
		if a.definition.Lexicon[i].ID == q.Lexeme {
			lex = &a.definition.Lexicon[i]
			break
		}
	}
	if lex == nil {
		return fail("invalid_input", "Choose a lexeme from this definition.")
	}
	kernel := a.morphology
	if a.report.Ready["analyze-written"] {
		kernel = a.written
	}
	seen := map[string]bool{}
	requests := map[string]bool{}
	for _, p := range a.definition.Morphology.Paradigms {
		if p.ID != lex.Paradigm {
			continue
		}
		for _, cell := range p.Cells {
			features, ok := spike.Unify(q.Features, cell.Features)
			if ok {
				features, ok = spike.Unify(features, lex.Features)
			}
			if !ok {
				continue
			}
			key := Hash(features)
			if requests[key] {
				continue
			}
			requests[key] = true
			in := spike.Request{Revision: kernel.Report().Revision, Engine: spike.EngineVersion, Operation: "inflect", Lexeme: lex.ID, Features: features, Policy: "all", Limit: 256}
			r := kernel.Execute(ctx, in)
			out.Examined += r.Examined
			if r.Status != "success" && r.Status != "ambiguous" && r.Status != "no_valid_candidate" {
				return fail(r.Status, "The complete authored paradigm could not be enumerated.")
			}
			for _, form := range r.Candidates {
				key := Hash(form)
				if !seen[key] {
					out.Candidates = append(out.Candidates, form)
					seen[key] = true
				}
			}
		}
	}
	sort.Slice(out.Candidates, func(i, j int) bool {
		return string(Canonical(out.Candidates[i])) < string(Canonical(out.Candidates[j]))
	})
	out.KnownValid = len(out.Candidates)
	out.Request = q.Request
	if len(out.Candidates) == 0 {
		out.Status = "no_valid_candidate"
		return out
	}
	out.Status = "success"
	if q.Policy == "seeded" {
		sum := sha256.Sum256(Canonical(struct {
			Version, Seed, Revision, Operation string
			Candidates                         []spike.Candidate
		}{out.Selection, q.Seed, a.report.Revision, q.Operation, out.Candidates}))
		index := binary.BigEndian.Uint64(sum[:8]) % uint64(len(out.Candidates))
		out.Candidates = []spike.Candidate{out.Candidates[index]}
	} else if len(out.Candidates) > 1 {
		out.Status = "ambiguous"
		if q.Policy == "explicit" {
			out.Status = "choice_required"
		}
	}
	if len(out.Candidates) > q.Limit {
		out.Candidates = out.Candidates[:q.Limit]
		out.Status = "candidate_limit"
		out.Truncated = true
	}
	return out
}
