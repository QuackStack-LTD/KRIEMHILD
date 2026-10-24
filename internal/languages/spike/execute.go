package spike

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
)

func (c *Compiled) match(m Match, token string) bool {
	if m.Boundary != "" {
		return m.Boundary == "word" && token == "#"
	}
	if m.Segment != "" {
		return m.Segment == token
	}
	n, ok := c.classes[m.Class]
	if !ok {
		return false
	}
	s, ok := c.segments[token]
	if !ok {
		return false
	}
	for key, value := range n.Features {
		if s.Features[key] != value {
			return false
		}
	}
	return true
}
func (c *Compiled) at(pattern []Match, input []string, start int) bool {
	for i, m := range pattern {
		index := start + i
		token := "#"
		if index >= 0 && index < len(input) {
			token = input[index]
		} else if index < -1 || index > len(input) {
			return false
		}
		if !c.match(m, token) {
			return false
		}
	}
	return true
}
func (c *Compiled) phonology(ctx context.Context, input []string) ([]string, []Trace, *Diagnostic) {
	current := append([]string{}, input...)
	trace := []Trace{}
	if c.definition.Phonology == nil {
		return nil, trace, &Diagnostic{Code: "missing_dependency", Module: "phonology", Message: "Define phonology before executing sound rules."}
	}
	for _, rule := range c.definition.Phonology.Rules {
		if err := ctx.Err(); err != nil {
			return nil, trace, &Diagnostic{Code: "cancelled", Module: "phonology", RuleID: rule.ID, Message: err.Error()}
		}
		before := append([]string{}, current...)
		out := []string{}
		applications := 0
		// Each rule sees its own input snapshot and makes one consuming pass.
		// Epenthesis runs once at a gap; outputs are not recursively rewritten.
		for i := 0; i <= len(current); {
			matches := i+len(rule.Input) <= len(current) && c.at(rule.Input, current, i) && c.at(rule.Left, current, i-len(rule.Left)) && c.at(rule.Right, current, i+len(rule.Input))
			if matches {
				out = append(out, rule.Output...)
				applications++
				if len(rule.Input) > 0 {
					i += len(rule.Input)
					if len(out) > 256 {
						return nil, trace, &Diagnostic{Code: "form_limit", Module: "phonology", RuleID: rule.ID, Message: "Sound rule output exceeds 256 segments."}
					}
					continue
				}
			}
			if i == len(current) {
				break
			}
			out = append(out, current[i])
			i++
			if len(out) > 256 {
				return nil, trace, &Diagnostic{Code: "form_limit", Module: "phonology", RuleID: rule.ID, Message: "Sound rule output exceeds 256 segments."}
			}
		}
		if len(out) > 256 {
			return nil, trace, &Diagnostic{Code: "form_limit", Module: "phonology", RuleID: rule.ID, Message: "Sound rule output exceeds 256 segments."}
		}
		if applications > 0 {
			trace = append(trace, Trace{"phonology", rule.ID, before, append([]string{}, out...)})
		}
		current = out
	}
	return current, trace, nil
}
func (c *Compiled) validateForm(input []string) *Diagnostic {
	if len(input) > 256 {
		return &Diagnostic{Code: "form_limit", Module: "phonotactics", Message: "A form has at most 256 segments."}
	}
	for _, token := range input {
		if _, ok := c.segments[token]; !ok {
			return &Diagnostic{Code: "unknown_segment", Module: "phonotactics", Message: "Undeclared segment " + token}
		}
	}
	if c.definition.Phonology == nil {
		return &Diagnostic{Code: "missing_dependency", Module: "phonotactics", Message: "No phonology is defined."}
	}
	for _, pattern := range c.definition.Phonology.Forbidden {
		for start := -1; start <= len(input); start++ {
			if c.at(pattern.Slots, input, start) {
				return &Diagnostic{Code: "forbidden_sequence", Module: "phonotactics", RuleID: pattern.ID, Message: "The form matches forbidden sequence " + pattern.ID + "."}
			}
		}
	}
	for _, pattern := range c.definition.Phonology.Allowed {
		if len(pattern.Slots) == len(input) && c.at(pattern.Slots, input, 0) {
			return nil
		}
	}
	return &Diagnostic{Code: "word_shape", Module: "phonotactics", Message: "The complete form matches none of the authored allowed word patterns."}
}
func (c *Compiled) render(segments []string) ([]string, []Trace, *Diagnostic) {
	chunks := []string{}
	trace := []Trace{}
	if c.definition.Orthography == nil {
		return chunks, trace, nil
	}
	for _, token := range segments {
		mapping, ok := c.mappings[token]
		if !ok {
			return nil, trace, &Diagnostic{Code: "unmapped_segment", Module: "orthography", Message: "Define an orthographic mapping for segment " + token + "."}
		}
		written := normalize(mapping.Grapheme, c.definition.Orthography.Normalization)
		chunks = append(chunks, written)
		trace = append(trace, Trace{"orthography", mapping.ID, []string{token}, []string{written}})
	}
	joined := strings.Join(chunks, "")
	normalized := normalize(joined, c.definition.Orthography.Normalization)
	if joined != normalized {
		trace = append(trace, Trace{"normalization", c.definition.Orthography.ID, append([]string{}, chunks...), []string{normalized}})
		chunks = []string{normalized}
	}
	return chunks, trace, nil
}
func (c *Compiled) Execute(ctx context.Context, request Request) Result {
	r := Result{Status: "invalid_input", LanguageID: c.definition.LanguageID, Revision: c.report.Revision, Engine: EngineVersion, Canonicalizer: CanonicalVersion, Selection: SelectionVersion, Request: copyJSON(request), Candidates: []Candidate{}, Diagnostics: []Diagnostic{}}
	fail := func(status, code, module, message string) Result {
		r.Status = status
		r.KnownValid = len(r.Candidates)
		r.Candidates = []Candidate{}
		r.Diagnostics = append(r.Diagnostics, Diagnostic{Code: code, Severity: "error", Module: module, Path: "request", Message: message})
		return r
	}
	if err := ctx.Err(); err != nil {
		return fail("cancelled", "cancelled", "engine", err.Error())
	}
	if request.Revision != c.report.Revision {
		return fail("invalid_input", "revision_mismatch", "engine", "Choose the exact immutable definition revision compiled by this artifact.")
	}
	if request.Engine != EngineVersion {
		return fail("incompatible_engine", "engine_mismatch", "engine", "Choose the compiled artifact's exact engine version.")
	}
	if request.Limit < 1 || request.Limit > 256 || len(request.Segments) > 256 || len(request.Text) > 4096 || len(request.Features) > 64 || len(request.Seed) > 256 {
		return fail("invalid_input", "request_limit", "engine", "Use a result limit of 1–256 and bounded input, feature and seed sizes.")
	}
	if request.Policy != "all" && request.Policy != "seeded" && request.Policy != "explicit" {
		return fail("invalid_input", "unknown_policy", "engine", "Choose all, seeded or explicit selection.")
	}
	if request.Policy == "seeded" && request.Seed == "" {
		return fail("invalid_input", "seed_required", "engine", "Seeded selection requires a recorded nonempty seed.")
	}
	if request.Policy != "seeded" && request.Seed != "" {
		return fail("invalid_input", "irrelevant_seed", "engine", "A seed is used only with the seeded policy.")
	}
	invalidFields := false
	switch request.Operation {
	case "generate":
		invalidFields = request.Lexeme != "" || len(request.Features) > 0 || len(request.Segments) > 0 || request.Text != ""
	case "inflect":
		invalidFields = request.Template != "" || len(request.Segments) > 0 || request.Text != ""
	case "analyze", "validate":
		invalidFields = request.Lexeme != "" || len(request.Features) > 0 || request.Template != "" || request.Text != ""
	case "analyze-written":
		invalidFields = request.Lexeme != "" || len(request.Features) > 0 || request.Template != "" || len(request.Segments) > 0
	}
	if invalidFields {
		return fail("invalid_input", "irrelevant_input", "engine", "This operation cannot use the supplied fields; no input was silently ignored.")
	}
	if c.report.Status == "invalid" {
		return fail("uncompiled_revision", "invalid_definition", "compiler", "Correct compilation diagnostics before executing this revision.")
	}
	if !c.report.Ready[request.Operation] {
		switch request.Operation {
		case "generate", "inflect", "analyze", "analyze-written", "validate":
			return fail("missing_dependency", "missing_dependency", request.Operation, "The required module is not defined/compiled for this operation.")
		default:
			return fail("unsupported_operation", "unsupported_operation", "engine", "The spike does not implement this operation; no substitute was generated.")
		}
	}
	switch request.Operation {
	case "inflect", "analyze", "analyze-written":
		var ids []int
		var err error
		var truncated bool
		switch request.Operation {
		case "inflect":
			lexeme, ok := c.lexemes[request.Lexeme]
			if !ok {
				return fail("invalid_input", "unknown_lexeme", "lexicon", "Choose a lexical entry from this revision.")
			}
			keys := make([]string, 0, len(request.Features))
			for k := range request.Features {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if !c.grammatical[k][request.Features[k]] {
					return fail("invalid_input", "unknown_feature", "morphology", "The requested grammatical feature/value is not defined: "+k)
				}
			}
			features, ok := Unify(lexeme.Features, request.Features)
			if !ok {
				return fail("no_valid_candidate", "feature_conflict", "morphology", "Requested features conflict with the lexical entry.")
			}
			ids, truncated, err = c.phonetic.Lookup(ctx, false, analysisTape(lexeme.ID, features), 10000)
		case "analyze":
			ids, truncated, err = c.phonetic.Lookup(ctx, true, request.Segments, 10000)
		case "analyze-written":
			ids, truncated, err = c.written.InverseText(ctx, normalize(request.Text, c.definition.Orthography.Normalization), 10000)
		}
		if err != nil {
			if ctx.Err() != nil {
				return fail("cancelled", "cancelled", "engine", ctx.Err().Error())
			}
			return fail("execution_limit", "traversal_limit", "engine", err.Error())
		}
		if truncated {
			return fail("execution_limit", "relation_limit", "engine", "Analysis exceeded the complete relation traversal limit.")
		}
		for _, id := range ids {
			r.Candidates = append(r.Candidates, copyJSON(c.forms[id]))
		}
		r.Examined = len(ids)
	case "validate":
		if diagnostic := c.validateForm(request.Segments); diagnostic != nil {
			diagnostic.Severity = "error"
			diagnostic.Path = "request.segments"
			r.Status = "no_valid_candidate"
			r.Diagnostics = append(r.Diagnostics, *diagnostic)
			return r
		}
		r.Candidates = append(r.Candidates, Candidate{Underlying: append([]string{}, request.Segments...), Segments: append([]string{}, request.Segments...), Trace: []Trace{}})
		r.Examined = 1
	case "generate":
		template, ok := c.templates[request.Template]
		if !ok {
			return fail("invalid_input", "unknown_template", "phonology", "Choose a defined generation template.")
		}
		options := [][]string{}
		segmentIDs := make([]string, 0, len(c.segments))
		for id := range c.segments {
			segmentIDs = append(segmentIDs, id)
		}
		sort.Strings(segmentIDs)
		for _, selector := range template.Slots {
			choices := []string{}
			for _, id := range segmentIDs {
				if c.segments[id].Phonemic && c.match(selector, id) {
					choices = append(choices, id)
				}
			}
			if len(choices) == 0 {
				return fail("missing_dependency", "empty_class", "phonology", "A template position contains no declared phonemic segments.")
			}
			options = append(options, choices)
		}
		const candidateBudget = 10000
		positions := make([]int, len(options))
		done := false
		payloadBytes := 0
		for !done {
			if err := ctx.Err(); err != nil {
				return fail("cancelled", "cancelled", "engine", err.Error())
			}
			if r.Examined >= candidateBudget {
				return fail("execution_limit", "enumeration_limit", "phonology", "The complete candidate set exceeds 10000. Narrow the template; partial enumeration is never used for seeded choice.")
			}
			input := make([]string, len(options))
			for i, values := range options {
				input[i] = values[positions[i]]
			}
			r.Examined++
			out, trace, diagnostic := c.phonology(ctx, input)
			if diagnostic == nil {
				diagnostic = c.validateForm(out)
			}
			if diagnostic != nil {
				r.Rejected++
				if len(r.Diagnostics) < 64 {
					diagnostic.Severity = "information"
					diagnostic.Path = "candidates." + strings.Join(input, ".")
					r.Diagnostics = append(r.Diagnostics, *diagnostic)
				} else {
					r.DiagnosticsOmitted++
				}
			} else {
				chunks, orthoTrace, diagnostic := c.render(out)
				if diagnostic != nil {
					return fail("missing_dependency", diagnostic.Code, diagnostic.Module, diagnostic.Message)
				}
				trace = append([]Trace{{"template", template.ID, []string{}, append([]string{}, input...)}}, trace...)
				trace = append(trace, orthoTrace...)
				candidate := Candidate{Underlying: input, Segments: out, Written: strings.Join(chunks, ""), Trace: trace}
				payloadBytes += len(Canonical(candidate))
				if payloadBytes > 8<<20 {
					return fail("execution_limit", "result_limit", "engine", "The complete candidate/trace payload exceeds the 8 MiB spike budget.")
				}
				r.Candidates = append(r.Candidates, candidate)
			}
			for i := len(positions) - 1; i >= 0; i-- {
				positions[i]++
				if positions[i] < len(options[i]) {
					break
				}
				positions[i] = 0
				if i == 0 {
					done = true
				}
			}
		}
	}
	sort.SliceStable(r.Candidates, func(i, j int) bool { return string(Canonical(r.Candidates[i])) < string(Canonical(r.Candidates[j])) })
	r.KnownValid = len(r.Candidates)
	if len(r.Candidates) == 0 {
		return fail("no_valid_candidate", "no_valid_candidate", request.Operation, "No form satisfies the authored rules and exact requested features.")
	}
	if request.Policy == "seeded" {
		// Versioned deterministic choice over the COMPLETE canonical candidate
		// sequence. No wall clock, global RNG, locale or map order participates.
		digest := sha256.Sum256(Canonical(struct {
			Version, Seed, Revision, Operation string
			Candidates                         []Candidate
		}{SelectionVersion, request.Seed, request.Revision, request.Operation, r.Candidates}))
		chosen := binary.BigEndian.Uint64(digest[:8]) % uint64(len(r.Candidates))
		r.Candidates = []Candidate{r.Candidates[chosen]}
	}
	r.Status = "success"
	if len(r.Candidates) > 1 {
		r.Status = "ambiguous"
	}
	if request.Policy == "explicit" && len(r.Candidates) > 1 {
		r.Status = "choice_required"
	}
	if len(r.Candidates) > request.Limit {
		r.Candidates = r.Candidates[:request.Limit]
		r.Truncated = true
		r.Status = "candidate_limit"
	}
	return r
}

func (r Result) String() string {
	return fmt.Sprintf("%s: %d candidates, %d examined", r.Status, len(r.Candidates), r.Examined)
}
