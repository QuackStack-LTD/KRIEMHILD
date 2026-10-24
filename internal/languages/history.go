package languages

import (
	"context"
	"kriemhild/internal/languages/spike"
	"sort"
)

type HistoricalChange struct {
	Lexeme string        `json:"lexeme"`
	Before []string      `json:"before"`
	After  []string      `json:"after"`
	Trace  []spike.Trace `json:"trace"`
}
type Evolution struct {
	Status         string             `json:"status"`
	Engine         string             `json:"engine"`
	Adapter        string             `json:"adapter"`
	SourceRevision string             `json:"sourceRevision"`
	TargetRevision string             `json:"targetRevision"`
	PreviewHash    string             `json:"previewHash"`
	Changes        []HistoricalChange `json:"changes"`
	Diagnostics    []Diagnostic       `json:"diagnostics"`
	Target         *Definition        `json:"target,omitempty"`
}

// PreviewEvolution is a pure batch. Changes use the explicitly authored target
// inventory/classes and phonotactics. Source lexical metadata is copied;
// target-only entries remain intact. Publication requires a separate transaction.
func PreviewEvolution(ctx context.Context, source, target Definition) Evolution {
	out := Evolution{Status: "invalid_input", Engine: EngineVersion, Adapter: AdapterVersion, SourceRevision: Hash(source), TargetRevision: Hash(target), Changes: []HistoricalChange{}, Diagnostics: []Diagnostic{}}
	fail := func(code, path, message string) Evolution {
		out.Status = code
		out.Target = nil
		out.Diagnostics = append(out.Diagnostics, Diagnostic{Code: code, Severity: "error", Module: "history", Path: path, Message: message})
		return out
	}
	h := target.History
	if h == nil || h.SourceLanguage != source.LanguageID || h.SourceRevision != out.SourceRevision {
		return fail("missing_dependency", "history.sourceRevision", "Pin the exact immutable source language revision.")
	}
	if target.Phonology == nil || len(source.Lexicon) == 0 || len(source.Lexicon) > 1000 || len(h.Changes) > 32 || len(h.Exceptions) > 1000 {
		return fail("missing_dependency", "history", "Define a target sound inventory/phonotactics and a bounded source lexicon and change list.")
	}
	probe := spike.Definition{Schema: 1, LanguageID: target.LanguageID, Name: target.Name, Phonology: Clone(target.Phonology), Morphology: &spike.Morphology{Features: []spike.Feature{}, Paradigms: []spike.Paradigm{{ID: "historical-probe", Cells: []spike.Cell{{ID: "historical-base", Features: Features{}, Operations: []spike.Operation{{ID: "historical-zero", Kind: "zero"}}}}}}}}
	probe.Phonology.Rules = Clone(h.Changes)
	exceptions := map[string]HistoricalException{}
	sourceIDs := map[string]bool{}
	for _, lex := range source.Lexicon {
		sourceIDs[lex.ID] = true
	}
	for _, e := range h.Exceptions {
		if !sourceIDs[e.Lexeme] || e.RuleID == "" || exceptions[e.Lexeme].RuleID != "" {
			return fail("invalid_input", "history.exceptions", "Exceptions need a unique source lexeme and a stable rule ID.")
		}
		exceptions[e.Lexeme] = e
	}
	for _, lex := range source.Lexicon {
		if _, ok := exceptions[lex.ID]; !ok {
			probe.Lexicon = append(probe.Lexicon, spike.Lexeme{ID: lex.ID, Category: lex.Category, Senses: Clone(lex.Senses), Underlying: Clone(lex.Underlying), Features: Features{}, Paradigm: "historical-probe"})
		}
	}
	compiled, report := spike.Compile(ctx, probe, spike.TokenTrieBackend{})
	if ctx.Err() != nil {
		return fail("cancelled", "history", ctx.Err().Error())
	}
	if len(probe.Lexicon) > 0 && !report.Ready["inflect"] {
		out.Diagnostics = report.Diagnostics
		out.Status = "invalid_definition"
		return out
	}
	validatorDef := Clone(probe)
	validatorDef.Lexicon = nil
	validatorDef.Morphology = nil
	validatorDef.Phonology.Rules = nil
	validator, validationReport := spike.Compile(ctx, validatorDef, spike.TokenTrieBackend{})
	if !validationReport.Ready["validate"] {
		out.Diagnostics = validationReport.Diagnostics
		out.Status = "invalid_definition"
		return out
	}
	next := Clone(target)
	merged := map[string]spike.Lexeme{}
	for _, lex := range next.Lexicon {
		merged[lex.ID] = lex
	}
	for _, lex := range source.Lexicon {
		if ctx.Err() != nil {
			return fail("cancelled", "history", ctx.Err().Error())
		}
		var form spike.Candidate
		if e, ok := exceptions[lex.ID]; ok {
			q := spike.Request{Revision: validationReport.Revision, Engine: spike.EngineVersion, Operation: "validate", Segments: e.Segments, Policy: "explicit", Limit: 1}
			r := validator.Execute(ctx, q)
			if r.Status != "success" {
				return fail("invalid_exception", "history.exceptions", "An exceptional target form violates the target inventory or phonotactics.")
			}
			form = spike.Candidate{Segments: Clone(e.Segments), Trace: []spike.Trace{{Stage: "historical-exception", RuleID: e.RuleID, Before: Clone(lex.Underlying), After: Clone(e.Segments)}}}
		} else {
			q := spike.Request{Revision: report.Revision, Engine: spike.EngineVersion, Operation: "inflect", Lexeme: lex.ID, Features: Features{}, Policy: "explicit", Limit: 1}
			r := compiled.Execute(ctx, q)
			if r.Status != "success" || len(r.Candidates) != 1 {
				return fail("invalid_derivation", "history.changes", "A source lexeme has no unique valid target form; correct the changes or author an exception.")
			}
			form = r.Candidates[0]
		}
		out.Changes = append(out.Changes, HistoricalChange{Lexeme: lex.ID, Before: Clone(lex.Underlying), After: Clone(form.Segments), Trace: Clone(form.Trace)})
		updated := Clone(lex)
		updated.Underlying = Clone(form.Segments)
		merged[lex.ID] = updated
	}
	ids := make([]string, 0, len(merged))
	for id := range merged {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	next.Lexicon = nil
	for _, id := range ids {
		next.Lexicon = append(next.Lexicon, merged[id])
	}
	if _, err := Decode(Canonical(next)); err != nil {
		return fail("definition_limit", "history", err.Error())
	}
	out.Status = "success"
	out.Target = &next
	out.PreviewHash = Hash(out)
	return out
}
