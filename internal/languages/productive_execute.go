package languages

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"kriemhild/internal/languages/spike"
	"sort"
)

const productiveSteps = 100000
const productiveBytes = 8 << 20

type productiveBudget struct {
	ctx          context.Context
	steps, bytes int
}

func (b *productiveBudget) charge(steps, bytes int) error {
	if err := b.ctx.Err(); err != nil {
		return productiveError("cancelled", "", "request", err.Error())
	}
	b.steps += steps
	b.bytes += bytes
	if b.steps > productiveSteps || b.bytes > productiveBytes {
		return productiveError("execution_limit", "", "request", "Productive work exceeded 100,000 steps or 8 MiB of intermediate/result payload. Narrow the authored domain or request.")
	}
	return nil
}

type productiveBranch struct {
	failure  *Diagnostic
	tokens   []ProductiveToken
	steps    []ProductiveStep
	previous string
}

func segmentTokens(segments []string) []ProductiveToken {
	out := make([]ProductiveToken, 0, len(segments))
	for _, s := range segments {
		out = append(out, ProductiveToken{Segment: s})
	}
	return out
}
func tokenLabels(tokens []ProductiveToken) []string {
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if t.Boundary != "" {
			out = append(out, "<"+t.Boundary+">")
		} else {
			out = append(out, t.Segment)
		}
	}
	return out
}
func (b *productiveBranch) appendStep(budget *productiveBudget, step ProductiveStep) error {
	if len(step.After) > 512 {
		return productiveError("execution_limit", step.RuleID, "productive", "An intermediate form exceeded 512 typed tokens.")
	}
	segments := 0
	for _, t := range step.After {
		if t.Boundary == "" {
			segments++
		}
	}
	if segments > 256 {
		return productiveError("execution_limit", step.RuleID, "productive", "An intermediate form exceeded 256 surface segments.")
	}
	if err := budget.charge(1, len(Canonical(step))); err != nil {
		return err
	}
	b.steps = append(append([]ProductiveStep{}, b.steps...), step)
	b.tokens = step.After
	return nil
}
func (p *productiveIndex) match(m spike.Match, token ProductiveToken) bool {
	if m.Boundary != "" {
		return m.Boundary == token.Boundary
	}
	if token.Boundary != "" {
		return false
	}
	if m.Segment != "" {
		return m.Segment == token.Segment
	}
	class, ok := p.classes[m.Class]
	if !ok {
		return false
	}
	segment, ok := p.segments[token.Segment]
	if !ok {
		return false
	}
	for key, value := range class.Features {
		if segment.Features[key] != value {
			return false
		}
	}
	return true
}
func (p *productiveIndex) at(budget *productiveBudget, pattern []spike.Match, tokens []ProductiveToken, start int) (bool, error) {
	for i, m := range pattern {
		if err := budget.charge(1, 0); err != nil {
			return false, err
		}
		pos := start + i
		token := ProductiveToken{Boundary: "word"}
		if pos >= 0 && pos < len(tokens) {
			token = tokens[pos]
		} else if pos < -1 || pos > len(tokens) {
			return false, nil
		}
		if !p.match(m, token) {
			return false, nil
		}
	}
	return true, nil
}
func (p *productiveIndex) condition(budget *productiveBudget, condition *AllomorphCondition, branch productiveBranch, lex spike.Lexeme, features Features) (bool, error) {
	if err := budget.charge(1, 0); err != nil {
		return false, err
	}
	for _, key := range sortedFeatureKeys(condition.Features) {
		if features[key] != condition.Features[key] {
			return false, nil
		}
	}
	if condition.Category != "" && condition.Category != lex.Category || condition.Previous != "" && condition.Previous != branch.previous {
		return false, nil
	}
	start := 0
	if len(condition.Left) > 0 && condition.Left[0].Boundary == "word" {
		start = -1
	}
	match, err := p.at(budget, condition.Left, branch.tokens, start)
	if err != nil || !match {
		return match, err
	}
	start = len(branch.tokens) - len(condition.Right)
	if len(condition.Right) > 0 && condition.Right[len(condition.Right)-1].Boundary == "word" {
		start++
	}
	return p.at(budget, condition.Right, branch.tokens, start)
}
func (p *productiveIndex) affix(budget *productiveBudget, branches []productiveBranch, op ProductiveOperation, lex spike.Lexeme, features Features) ([]productiveBranch, error) {
	out := []productiveBranch{}
	for _, branch := range branches {
		if err := budget.charge(1, 0); err != nil {
			return nil, err
		}
		if branch.failure != nil {
			out = append(out, branch)
			continue
		}
		decisions := []AlternativeDecision{}
		selected := []Allomorph{}
		for _, alt := range op.Allomorphs {
			if err := budget.charge(1, 0); err != nil {
				return nil, err
			}
			decision := "precedence-skipped"
			if op.Policy != "first" || len(selected) == 0 {
				ok, err := p.condition(budget, alt.When, branch, lex, features)
				if err != nil {
					return nil, err
				}
				decision = "condition-failed"
				if ok {
					selected = append(selected, alt)
					decision = "selected"
				}
			}
			decisions = append(decisions, AlternativeDecision{ID: alt.ID, Decision: decision})
		}
		if len(selected) == 0 {
			branch.failure = &Diagnostic{Code: "no_matching_allomorph", Severity: "info", Module: "productive", RuleID: op.ID, Path: "productive.allomorphs", Message: "No authored allomorph condition matched this lexical cell. Add an explicit alternative or adjust the requested features."}
			if err := branch.appendStep(budget, ProductiveStep{Stage: "allomorph-rejected", RuleID: op.ID, Before: branch.tokens, After: branch.tokens, Alternatives: decisions}); err != nil {
				return nil, err
			}
			out = append(out, branch)
		}
		for _, alt := range selected {
			if len(out) >= 256 {
				return nil, productiveError("execution_limit", op.ID, "productive.operations", "Composition exceeded 256 live branches; no partial choice was made.")
			}
			tokens := []ProductiveToken{}
			if len(alt.Prefix) > 0 {
				tokens = append(tokens, segmentTokens(alt.Prefix)...)
				tokens = append(tokens, ProductiveToken{Boundary: "morpheme"})
			}
			tokens = append(tokens, branch.tokens...)
			if len(alt.Suffix) > 0 {
				tokens = append(tokens, ProductiveToken{Boundary: "morpheme"})
				tokens = append(tokens, segmentTokens(alt.Suffix)...)
			}
			next := productiveBranch{tokens: branch.tokens, steps: branch.steps, previous: op.ID}
			if err := next.appendStep(budget, ProductiveStep{Stage: "allomorph", RuleID: op.ID, Allomorph: alt.ID, Before: branch.tokens, After: tokens, Alternatives: decisions}); err != nil {
				return nil, err
			}
			out = append(out, next)
		}
	}
	return out, nil
}
func (p *productiveIndex) applyRule(budget *productiveBudget, branch *productiveBranch, rule spike.SoundRule) error {
	before := branch.tokens
	out := []ProductiveToken{}
	for i := 0; i <= len(before); {
		if err := budget.charge(1, 0); err != nil {
			return err
		}
		match := false
		if i+len(rule.Input) <= len(before) {
			ok, err := p.at(budget, rule.Input, before, i)
			if err != nil {
				return err
			}
			if ok {
				ok, err = p.at(budget, rule.Left, before, i-len(rule.Left))
				if err != nil {
					return err
				}
			}
			if ok {
				ok, err = p.at(budget, rule.Right, before, i+len(rule.Input))
				if err != nil {
					return err
				}
			}
			match = ok
		}
		if match {
			out = append(out, segmentTokens(rule.Output)...)
			if len(out) > 512 {
				return productiveError("execution_limit", rule.ID, "productive.stratum", "Rule output exceeded 512 typed tokens.")
			}
			if len(rule.Input) > 0 {
				i += len(rule.Input)
				continue
			}
		}
		if i == len(before) {
			break
		}
		out = append(out, before[i])
		i++
	}
	return branch.appendStep(budget, ProductiveStep{Stage: "boundary-rule", RuleID: rule.ID, Before: before, After: out})
}

type productiveForm struct {
	candidate  spike.Candidate
	derivation ProductiveDerivation
	key        string
}

func (a *Artifact) composeCell(budget *productiveBudget, lex spike.Lexeme, cell ProductiveCell, features Features) ([]productiveForm, error) {
	branch := productiveBranch{}
	if err := branch.appendStep(budget, ProductiveStep{Stage: "lexical", RuleID: lex.ID, Before: []ProductiveToken{}, After: segmentTokens(lex.Underlying)}); err != nil {
		return nil, err
	}
	if err := branch.appendStep(budget, ProductiveStep{Stage: "cell", RuleID: cell.ID, Before: branch.tokens, After: branch.tokens}); err != nil {
		return nil, err
	}
	branches := []productiveBranch{branch}
	overridden := false
	for _, override := range lex.Overrides {
		if err := budget.charge(1, 0); err != nil {
			return nil, err
		}
		if override.Cell == cell.ID {
			overridden = true
			if err := branches[0].appendStep(budget, ProductiveStep{Stage: "suppletion", RuleID: override.RuleID, Before: branch.tokens, After: segmentTokens(override.Segments)}); err != nil {
				return nil, err
			}
			break
		}
	}
	if !overridden {
		for _, op := range cell.Operations {
			var err error
			branches, err = a.productive.affix(budget, branches, op, lex, features)
			if err != nil {
				return nil, err
			}
		}
	}
	forms := []productiveForm{}
	for _, branch := range branches {
		if branch.failure != nil {
			d := ProductiveDerivation{Lexeme: lex.ID, Cell: cell.ID, Steps: branch.steps, Failure: branch.failure}
			if err := budget.charge(1, len(Canonical(d))); err != nil {
				return nil, err
			}
			forms = append(forms, productiveForm{derivation: d})
			continue
		}
		for _, rule := range a.definition.Productive.Stratum.Rules {
			if err := a.productive.applyRule(budget, &branch, rule); err != nil {
				return nil, err
			}
		}
		segments := []string{}
		for _, token := range branch.tokens {
			if err := budget.charge(1, 0); err != nil {
				return nil, err
			}
			if token.Boundary == "" {
				segments = append(segments, token.Segment)
			}
		}
		if err := branch.appendStep(budget, ProductiveStep{Stage: "erase-boundaries", RuleID: a.definition.Productive.Stratum.ID, Before: branch.tokens, After: segmentTokens(segments)}); err != nil {
			return nil, err
		}
		q := Request{Request: spike.Request{Revision: a.report.Revision, Engine: a.report.Engine, Operation: "validate", Segments: segments, Policy: "explicit", Limit: 1}, Adapter: a.report.Adapter}
		var surface Result
		if a.report.Ready["write"] {
			q.Operation = "write"
			surface = a.write(budget.ctx, q, Result{Result: spike.Result{Candidates: []spike.Candidate{}, Diagnostics: []Diagnostic{}}})
		} else {
			q.Engine = spike.EngineVersion
			q.Revision = a.phonology.Report().Revision
			surface.Result = a.phonology.Execute(budget.ctx, q.Request)
		}
		if surface.Status == "cancelled" || budget.ctx.Err() != nil {
			return nil, productiveError("cancelled", "", "request", "Productive execution was cancelled.")
		}
		if surface.Status != "success" {
			failure := Diagnostic{Code: "invalid_surface", Severity: "info", Module: "phonotactics", Path: "surface", Message: "The composed form does not satisfy the authored surface constraints."}
			if len(surface.Diagnostics) > 0 {
				failure = surface.Diagnostics[0]
			}
			derivation := ProductiveDerivation{Lexeme: lex.ID, Cell: cell.ID, Steps: branch.steps, Failure: &failure}
			if err := budget.charge(1, len(Canonical(derivation))); err != nil {
				return nil, err
			}
			forms = append(forms, productiveForm{derivation: derivation})
			continue
		}
		candidate := spike.Candidate{Lexeme: lex.ID, Cell: cell.ID, Features: features, Underlying: append([]string{}, lex.Underlying...), Segments: segments, Written: surface.Candidates[0].Written, Trace: []spike.Trace{}}
		for _, step := range branch.steps {
			id := step.RuleID
			if step.Allomorph != "" {
				id += "/" + step.Allomorph
			}
			candidate.Trace = append(candidate.Trace, spike.Trace{Stage: step.Stage, RuleID: id, Before: tokenLabels(step.Before), After: tokenLabels(step.After)})
		}
		candidate.Trace = append(candidate.Trace, surface.Candidates[0].Trace...)
		derivation := ProductiveDerivation{Candidate: Hash(candidate), Lexeme: lex.ID, Cell: cell.ID, Steps: branch.steps}
		if err := budget.charge(1, len(Canonical(candidate))+len(Canonical(derivation))); err != nil {
			return nil, err
		}
		forms = append(forms, productiveForm{candidate: candidate, derivation: derivation, key: string(Canonical(candidate))})
	}
	return forms, nil
}

func (a *Artifact) executeProductive(ctx context.Context, q Request, out Result) (result Result) {
	fail := func(code, id, path, message string) Result {
		out.Status = code
		out.Candidates = []spike.Candidate{}
		out.Derivations = nil
		out.RejectedDerivations = nil
		out.KnownValid = 0
		out.Diagnostics = []Diagnostic{{Code: code, Severity: "error", Module: "productive", RuleID: id, Path: path, Message: message}}
		return out
	}
	if !a.report.Ready[q.Operation] || a.productive == nil {
		return fail("missing_dependency", "", "productive", "Compile a valid productive module and its dependencies before executing this operation.")
	}
	if q.Policy != "all" && q.Policy != "explicit" && q.Policy != "seeded" || q.Policy == "seeded" && q.Seed == "" || q.Policy != "seeded" && q.Seed != "" {
		return fail("invalid_input", "", "request.policy", "Choose all, explicit or seeded with a nonempty seed only for seeded selection.")
	}
	forward := q.Operation == "inflect" || q.Operation == "paradigm"
	if q.Template != "" || q.Construction != "" || len(q.Roles) > 0 || forward && (q.Text != "" || len(q.Segments) > 0) || !forward && (q.Lexeme != "" || len(q.Features) > 0) || q.Operation == "analyze" && q.Text != "" || q.Operation == "analyze-written" && len(q.Segments) > 0 {
		return fail("invalid_input", "", "request", "Remove fields unrelated to this operation; no input is silently ignored.")
	}
	if message := a.productive.invalidFeatures(q.Features); message != "" {
		return fail("invalid_input", "", "request.features", message)
	}
	if forward {
		if _, ok := a.productive.lexemes[q.Lexeme]; !ok {
			return fail("invalid_input", q.Lexeme, "request.lexeme", "Choose a lexical entry from this revision.")
		}
	}
	if q.Operation == "analyze" {
		for _, s := range q.Segments {
			if _, ok := a.productive.segments[s]; !ok {
				return fail("invalid_input", s, "request.segments", "Declare each input segment before analysis.")
			}
		}
	}
	budget := &productiveBudget{ctx: ctx}
	var indexed map[inverseRecord]bool
	if !forward && a.definition.Schema == IndexedSchemaVersion {
		metrics := &InverseMetrics{}
		defer func() {
			metrics.Steps = budget.steps
			metrics.PayloadBytes = budget.bytes
			metrics.Stop = result.Status
			result.Inverse = metrics
		}()
		if a.productive.inverse == nil {
			return fail("missing_dependency", "", "inverse.index", "Recompile this revision to rebuild its derived inverse index.")
		}
		var err error
		indexed, err = a.productive.inverse.possible(q, budget, metrics)
		if err != nil {
			var f *productiveFailure
			if errors.As(err, &f) {
				return fail(f.diagnostic.Code, f.diagnostic.RuleID, f.diagnostic.Path, f.diagnostic.Message)
			}
			return fail("execution_limit", "", "inverse", err.Error())
		}
	}
	forms := []productiveForm{}
	for li, lex := range a.definition.Lexicon {
		if forward && lex.ID != q.Lexeme {
			continue
		}
		for ci, cell := range a.productive.paradigms[lex.Paradigm].Cells {
			if indexed != nil && !indexed[inverseRecord{li, ci}] {
				if ctx.Err() != nil {
					return fail("cancelled", "", "inverse", ctx.Err().Error())
				}
				continue
			}
			if err := budget.charge(1, 0); err != nil {
				var f *productiveFailure
				if errors.As(err, &f) {
					return fail(f.diagnostic.Code, f.diagnostic.RuleID, f.diagnostic.Path, f.diagnostic.Message)
				}
				return fail("execution_limit", "", "request", err.Error())
			}
			out.Examined++
			features, ok := spike.Unify(lex.Features, cell.Features)
			if ok {
				features, ok = spike.Unify(features, q.Features)
			}
			if !ok {
				continue
			}
			generated, err := a.composeCell(budget, lex, cell, features)
			if err != nil {
				var f *productiveFailure
				if errors.As(err, &f) {
					return fail(f.diagnostic.Code, f.diagnostic.RuleID, f.diagnostic.Path, f.diagnostic.Message)
				}
				return fail("execution_limit", "", "productive", err.Error())
			}
			if len(generated) == 0 {
				out.Rejected++
			}
			for _, form := range generated {
				if form.candidate.Lexeme == "" {
					out.Rejected++
					if len(out.RejectedDerivations) < 256 {
						out.RejectedDerivations = append(out.RejectedDerivations, form.derivation)
					} else {
						out.DiagnosticsOmitted++
					}
					continue
				}
				if !forward && (q.Operation == "analyze" && string(Canonical(form.candidate.Segments)) != string(Canonical(q.Segments)) || q.Operation == "analyze-written" && form.candidate.Written != q.Text) {
					continue
				}
				forms = append(forms, form)
			}
		}
	}
	sort.Slice(forms, func(i, j int) bool {
		return forms[i].key < forms[j].key
	})
	if err := budget.charge(1, 0); err != nil {
		var f *productiveFailure
		if errors.As(err, &f) {
			return fail(f.diagnostic.Code, f.diagnostic.RuleID, f.diagnostic.Path, f.diagnostic.Message)
		}
		return fail("execution_limit", "", "request", err.Error())
	}
	out.KnownValid = len(forms)
	out.Status = "success"
	if len(forms) == 0 {
		out.Status = "no_valid_candidate"
		return out
	}
	if q.Policy == "seeded" {
		sum := sha256.Sum256(Canonical(struct {
			Version, Seed, Revision, Operation string
			Candidates                         []spike.Candidate
		}{out.Selection, q.Seed, a.report.Revision, q.Operation, func() []spike.Candidate {
			c := make([]spike.Candidate, 0, len(forms))
			for _, f := range forms {
				c = append(c, f.candidate)
			}
			return c
		}()}))
		index := binary.BigEndian.Uint64(sum[:8]) % uint64(len(forms))
		forms = []productiveForm{forms[index]}
	} else if len(forms) > 1 {
		out.Status = "ambiguous"
		if q.Policy == "explicit" {
			out.Status = "choice_required"
		}
	}
	if len(forms) > q.Limit {
		forms = forms[:q.Limit]
		out.Status = "candidate_limit"
		out.Truncated = true
	}
	for _, form := range forms {
		out.Candidates = append(out.Candidates, form.candidate)
		out.Derivations = append(out.Derivations, form.derivation)
	}
	return out
}
