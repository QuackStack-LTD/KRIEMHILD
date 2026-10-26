package languages

import (
	"context"
	"fmt"
	"kriemhild/internal/languages/spike"
	"sort"
)

type productiveIndex struct {
	inverse   *inverseIndex
	features  map[string]map[string]bool
	paradigms map[string]ProductiveParadigm
	lexemes   map[string]spike.Lexeme
	segments  map[string]spike.Segment
	classes   map[string]spike.NaturalClass
}

// Reject oversized nested inventories before persistence/compilation. Invalid
// authored rules within these limits remain saveable and receive readiness errors.
func productiveShape(m *ProductiveModel) error {
	if m == nil {
		return nil
	}
	limit := func(path string) error {
		return fmt.Errorf("%s: productive definition exceeds the documented inventory bound", path)
	}
	if len(m.Features) > 64 || len(m.Paradigms) > 64 || len(m.Stratum.Rules) > 32 {
		return limit("productive")
	}
	for _, f := range m.Features {
		if len(f.Values) > 64 {
			return limit("productive.features.values")
		}
	}
	for _, p := range m.Paradigms {
		if len(p.Cells) > 64 {
			return limit("productive.paradigms.cells")
		}
		for _, c := range p.Cells {
			if len(c.Features) > 64 || len(c.Operations) > 16 {
				return limit("productive.paradigms.cells")
			}
			for _, op := range c.Operations {
				if len(op.Allomorphs) > 16 {
					return limit("productive.allomorphs")
				}
				for _, a := range op.Allomorphs {
					if len(a.Prefix) > 256 || len(a.Suffix) > 256 {
						return limit("productive.allomorphs.affixes")
					}
					if a.When != nil && (len(a.When.Features) > 64 || len(a.When.Left) > 16 || len(a.When.Right) > 16) {
						return limit("productive.allomorphs.when")
					}
				}
			}
		}
	}
	for _, r := range m.Stratum.Rules {
		if len(r.Input) > 16 || len(r.Left) > 16 || len(r.Right) > 16 || len(r.Output) > 256 {
			return limit("productive.stratum.rules")
		}
	}
	return nil
}

func sortedFeatureKeys(f Features) []string {
	keys := make([]string, 0, len(f))
	for key := range f {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func (p *productiveIndex) invalidFeatures(f Features) string {
	if len(f) > 64 {
		return "Feature bundle exceeds 64 keys."
	}
	for _, key := range sortedFeatureKeys(f) {
		if !p.features[key][f[key]] {
			return "Declare the grammatical feature/value before use: " + key + "=" + f[key]
		}
	}
	return ""
}
func (p *productiveIndex) validMatch(m spike.Match, context bool) bool {
	n := 0
	if m.Segment != "" {
		n++
		if _, ok := p.segments[m.Segment]; !ok {
			return false
		}
	}
	if m.Class != "" {
		n++
		if _, ok := p.classes[m.Class]; !ok {
			return false
		}
	}
	if m.Boundary != "" {
		n++
		if !context || m.Boundary != "word" && m.Boundary != "morpheme" {
			return false
		}
	}
	return n == 1
}

// compileProductive indexes authored plans only. It never materializes the
// lexeme × cell × allomorph surface relation. Independent phone/script artifacts
// remain available when this module is incomplete or invalid.
func (a *Artifact) compileProductive(ctx context.Context) []Diagnostic {
	d := a.definition
	m := d.Productive
	if m == nil {
		return []Diagnostic{{Code: "missing_dependency", Severity: "info", Module: "productive", Path: "productive", Message: "Author a productive module to enable schema-2 inflection and analysis."}}
	}
	p := &productiveIndex{features: map[string]map[string]bool{}, paradigms: map[string]ProductiveParadigm{}, lexemes: map[string]spike.Lexeme{}, segments: map[string]spike.Segment{}, classes: map[string]spike.NaturalClass{}}
	diagnostics := []Diagnostic{}
	bad := func(code, id, path, message string) {
		if len(diagnostics) < 256 {
			diagnostics = append(diagnostics, Diagnostic{Code: code, Severity: "error", Module: "productive", RuleID: id, Path: path, Message: message})
		} else {
			a.report.DiagnosticsOmitted++
		}
	}
	if m.Version != ProductiveVersion || len(m.Features) > 64 || len(m.Paradigms) > 64 || len(m.Stratum.Rules) > 32 {
		bad("invalid_productive", "", "productive", "Use productive/1 and at most 64 features, 64 paradigms and 32 boundary rules.")
		return diagnostics
	}
	if d.Phonology != nil {
		for _, s := range d.Phonology.Segments {
			p.segments[s.ID] = s
		}
		for _, c := range d.Phonology.Classes {
			p.classes[c.ID] = c
		}
		if len(d.Phonology.Rules) > 0 {
			bad("unsupported_stratum", "", "phonology.rules", "Schema 2 uses the explicit productive boundary stratum. Move word rules there; A1 does not implicitly compose two rule pipelines.")
		}
	}
	seen := map[string]bool{}
	id := func(namespace, value string) {
		if value == "" || len(value) > 80 || seen[namespace+"/"+value] {
			bad("invalid_id", value, "productive."+namespace, "Use a nonempty, unique stable ID of at most 80 bytes in this scope.")
		}
		seen[namespace+"/"+value] = true
	}
	segments := func(tokens []string, rule, path string) {
		if len(tokens) > 256 {
			bad("definition_limit", rule, path, "Reduce the form to at most 256 segments.")
			return
		}
		for _, token := range tokens {
			if _, ok := p.segments[token]; !ok {
				bad("unknown_segment", rule, path, "Declare segment "+token+" before referencing it.")
			}
		}
	}
	matches := func(items []spike.Match, context bool, rule, path string) {
		if len(items) > 16 {
			bad("definition_limit", rule, path, "Use at most 16 selectors per pattern/context.")
			return
		}
		for _, item := range items {
			if !p.validMatch(item, context) {
				bad("invalid_selector", rule, path, "Choose exactly one declared segment/class, or an explicit word/morpheme boundary in context only.")
			}
		}
	}
	for _, f := range m.Features {
		id("features", f.ID)
		if len(f.Values) == 0 || len(f.Values) > 64 {
			bad("invalid_features", f.ID, "productive.features", "Declare 1–64 explicit values.")
		}
		values := map[string]bool{}
		for _, v := range f.Values {
			if v == "" || len(v) > 80 || values[v] {
				bad("invalid_features", f.ID, "productive.features", "Feature values must be unique nonempty atoms of at most 80 bytes.")
			}
			values[v] = true
		}
		p.features[f.ID] = values
	}
	for _, paradigm := range m.Paradigms {
		if ctx.Err() != nil {
			return []Diagnostic{{Code: "cancelled", Severity: "error", Module: "productive", Path: "productive", Message: ctx.Err().Error()}}
		}
		id("paradigms", paradigm.ID)
		if len(paradigm.Cells) == 0 || len(paradigm.Cells) > 64 {
			bad("definition_limit", paradigm.ID, "productive.paradigms", "Author 1–64 cells per paradigm.")
			continue
		}
		p.paradigms[paradigm.ID] = paradigm
		for _, cell := range paradigm.Cells {
			id("cells/"+paradigm.ID, cell.ID)
			if message := p.invalidFeatures(cell.Features); message != "" {
				bad("invalid_features", cell.ID, "productive.paradigms.cells.features", message)
			}
			if len(cell.Operations) > 16 {
				bad("definition_limit", cell.ID, "productive.paradigms.cells.operations", "Use at most 16 operations per cell.")
				continue
			}
			previous := ""
			for _, op := range cell.Operations {
				if ctx.Err() != nil {
					return []Diagnostic{{Code: "cancelled", Severity: "error", Module: "productive", Path: "productive", Message: ctx.Err().Error()}}
				}
				id("operations/"+paradigm.ID+"/"+cell.ID, op.ID)
				if op.Kind != "prefix" && op.Kind != "suffix" && op.Kind != "zero" && op.Kind != "circumfix" {
					bad("unsupported_rule", op.ID, "productive.paradigms.cells.operations.kind", "Choose prefix, suffix, zero or circumfix. Infixation, reduplication and interdigitation are not implemented.")
				}
				if op.Policy != "first" && op.Policy != "all" {
					bad("invalid_policy", op.ID, "productive.paradigms.cells.operations.policy", "Explicitly select first or all; there is no inferred allomorph precedence.")
				}
				if len(op.Allomorphs) == 0 || len(op.Allomorphs) > 16 {
					bad("definition_limit", op.ID, "productive.paradigms.cells.operations.allomorphs", "Author 1–16 alternatives; there is no fallback allomorph.")
					continue
				}
				for _, alt := range op.Allomorphs {
					id("allomorphs/"+paradigm.ID+"/"+cell.ID+"/"+op.ID, alt.ID)
					segments(alt.Prefix, alt.ID, "productive.allomorphs.prefix")
					segments(alt.Suffix, alt.ID, "productive.allomorphs.suffix")
					if alt.When == nil {
						bad("missing_condition", alt.ID, "productive.allomorphs.when", "Author an explicit condition object; an empty object explicitly means unconditional.")
					} else {
						if message := p.invalidFeatures(alt.When.Features); message != "" {
							bad("invalid_features", alt.ID, "productive.allomorphs.when.features", message)
						}
						if len(alt.When.Category) > 80 || alt.When.Previous != "" && alt.When.Previous != previous {
							bad("invalid_context", alt.ID, "productive.allomorphs.when", "Category must be bounded; previous must name the immediately preceding authored operation.")
						}
						matches(alt.When.Left, true, alt.ID, "productive.allomorphs.when.left")
						matches(alt.When.Right, true, alt.ID, "productive.allomorphs.when.right")
					}
					valid := op.Kind == "prefix" && len(alt.Prefix) > 0 && len(alt.Suffix) == 0 || op.Kind == "suffix" && len(alt.Suffix) > 0 && len(alt.Prefix) == 0 || op.Kind == "circumfix" && len(alt.Prefix) > 0 && len(alt.Suffix) > 0 || op.Kind == "zero" && len(alt.Prefix) == 0 && len(alt.Suffix) == 0
					if !valid {
						bad("invalid_affix", alt.ID, "productive.allomorphs", "Prefix/suffix use one nonempty side; circumfix requires both sides; zero requires both sides empty.")
					}
				}
				previous = op.ID
			}
		}
	}
	id("stratum", m.Stratum.ID)
	if m.Stratum.BoundaryPolicy != "retain-until-surface" {
		bad("unsupported_boundary_policy", m.Stratum.ID, "productive.stratum.boundaryPolicy", "Choose retain-until-surface; boundary-consuming/implicit-erasure strata are unsupported.")
	}
	for _, rule := range m.Stratum.Rules {
		id("rules", rule.ID)
		if rule.Domain != "morpheme" {
			bad("unsupported_stratum", rule.ID, "productive.stratum.rules.domain", "Choose the explicit morpheme domain.")
		}
		matches(rule.Input, false, rule.ID, "productive.stratum.rules.input")
		matches(rule.Left, true, rule.ID, "productive.stratum.rules.left")
		matches(rule.Right, true, rule.ID, "productive.stratum.rules.right")
		segments(rule.Output, rule.ID, "productive.stratum.rules.output")
		if len(rule.Input) == 0 && len(rule.Output) == 0 {
			bad("invalid_rule", rule.ID, "productive.stratum.rules", "An empty-input rule must insert at least one segment.")
		}
	}
	for _, lex := range d.Lexicon {
		if ctx.Err() != nil {
			return []Diagnostic{{Code: "cancelled", Severity: "error", Module: "productive", Path: "lexicon", Message: ctx.Err().Error()}}
		}
		id("lexemes", lex.ID)
		p.lexemes[lex.ID] = lex
		if lex.Category == "" || len(lex.Category) > 80 || len(lex.Senses) > 32 || len(lex.Overrides) > 64 {
			bad("invalid_lexeme", lex.ID, "lexicon", "Use a category of 1–80 bytes and bounded senses/overrides.")
		}
		for _, sense := range lex.Senses {
			id("senses", sense.ID)
			if len(sense.Meaning) > 2000 {
				bad("definition_limit", sense.ID, "lexicon.senses", "Reduce this authored meaning to 2,000 bytes.")
			}
		}
		segments(lex.Underlying, lex.ID, "lexicon.underlying")
		if message := p.invalidFeatures(lex.Features); message != "" {
			bad("invalid_features", lex.ID, "lexicon.features", message)
		}
		paradigm, ok := p.paradigms[lex.Paradigm]
		if !ok {
			bad("unknown_paradigm", lex.ID, "lexicon.paradigm", "Reference an authored productive paradigm.")
		}
		for _, override := range lex.Overrides {
			id("overrides/"+lex.ID, override.Cell)
			found := false
			for _, c := range paradigm.Cells {
				if c.ID == override.Cell {
					found = true
				}
			}
			if !found || override.RuleID == "" || len(override.RuleID) > 80 {
				bad("invalid_override", lex.ID, "lexicon.overrides", "Reference one existing cell per override and a stable bounded suppletion rule ID.")
			}
			segments(override.Segments, override.RuleID, "lexicon.overrides.segments")
		}
	}
	if len(diagnostics) == 0 && a.report.Ready["validate"] {
		a.productive = p
		ready := len(p.lexemes) > 0 && len(p.paradigms) > 0
		for _, op := range []string{"inflect", "paradigm", "analyze"} {
			a.report.Ready[op] = ready
		}
		a.report.Ready["analyze-written"] = ready && a.report.Ready["write"]
		if ready && d.Schema == IndexedSchemaVersion {
			if err := a.buildInverse(ctx); err != nil {
				a.report.Ready["analyze"] = false
				a.report.Ready["analyze-written"] = false
				if f, ok := err.(*productiveFailure); ok {
					diagnostics = append(diagnostics, f.diagnostic)
				}
			}
		}
	}
	return diagnostics
}

func productiveError(code, id, path, message string) error {
	return &productiveFailure{Diagnostic{Code: code, Severity: "error", Module: "productive", RuleID: id, Path: path, Message: message}}
}

type productiveFailure struct{ diagnostic Diagnostic }

func (e *productiveFailure) Error() string {
	return fmt.Sprintf("%s: %s", e.diagnostic.Code, e.diagnostic.Message)
}
