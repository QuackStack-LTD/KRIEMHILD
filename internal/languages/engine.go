package languages

import (
	"context"
	"fmt"
	"kriemhild/internal/languages/spike"
	"sort"
	"strings"
)

// Artifact is immutable after construction. Each operation uses only validated
// dependencies; a broken script or grammar cannot disable phonological work.
type Artifact struct {
	spellingScript                 *ScriptDefinition
	definition                     Definition
	report                         Report
	phonology, morphology, written *spike.Compiled
	orthographic                   *spike.Compiled
	grammar                        map[string]Construction
	productive                     *productiveIndex
}

func (a *Artifact) Report() Report { return Clone(a.report) }
func Compile(ctx context.Context, source Definition, scripts map[ScriptRef]ScriptDefinition) *Artifact {
	d := Clone(source)
	a := &Artifact{definition: d, grammar: map[string]Construction{}, report: Report{Revision: Hash(d), Engine: EngineVersion, Adapter: AdapterVersion, Kernel: spike.EngineVersion, Status: "incomplete", Ready: map[string]bool{}, Diagnostics: []Diagnostic{}}}
	a.report.Engine, a.report.Adapter, a.report.Compiler = Versions(d)
	if ProductiveSchema(d.Schema) {
		a.report.Schema = d.Schema
		a.report.Capabilities = Capabilities()[d.Schema-1].Operations
		for _, c := range a.report.Capabilities {
			a.report.Ready[c.Operation] = false
		}
	}
	add := func(items []Diagnostic) {
		for _, v := range items {
			if len(a.report.Diagnostics) < 256 {
				a.report.Diagnostics = append(a.report.Diagnostics, v)
			} else {
				a.report.DiagnosticsOmitted++
			}
		}
	}
	if _, err := Decode(Canonical(d)); err != nil {
		add([]Diagnostic{{Code: "invalid_definition", Severity: "error", Module: "definition", Path: "definition", Message: err.Error()}})
		a.report.Status = "invalid"
		return a
	}
	base := spike.Definition{Schema: 1, LanguageID: d.LanguageID, Name: d.Name, Notes: d.Notes, Phonology: d.Phonology}
	p, pr := spike.Compile(ctx, base, spike.TokenTrieBackend{})
	a.report.DiagnosticsOmitted += pr.DiagnosticsOmitted
	a.phonology = p
	for _, op := range []string{"generate", "validate"} {
		a.report.Ready[op] = pr.Ready[op]
	}
	for _, v := range pr.Diagnostics {
		if v.Module != "morphology" && v.Module != "orthography" && v.Module != "lexicon" {
			add([]Diagnostic{v})
		}
	}
	if d.Schema == 1 {
		base.Lexicon = d.Lexicon
		base.Morphology = d.Morphology
	}
	m, mr := spike.Compile(ctx, base, spike.TokenTrieBackend{})
	a.report.DiagnosticsOmitted += mr.DiagnosticsOmitted
	a.morphology = m
	for _, op := range []string{"inflect", "analyze"} {
		a.report.Ready[op] = mr.Ready[op]
	}
	a.report.Ready["paradigm"] = mr.Ready["inflect"]
	// The full report already contains phonological diagnostics. Retain each once.
	seen := map[string]bool{}
	for _, v := range a.report.Diagnostics {
		seen[Hash(v)] = true
	}
	for _, v := range mr.Diagnostics {
		if ProductiveSchema(d.Schema) && (v.Module == "morphology" || v.Module == "lexicon") {
			continue
		}
		if v.Module == "orthography" {
			continue
		}
		if !seen[Hash(v)] {
			add([]Diagnostic{v})
			seen[Hash(v)] = true
		}
	}
	a.report.Forms = mr.Forms
	a.report.States = mr.States + pr.States
	if d.Orthography != nil {
		o := d.Orthography
		a.report.Dependencies = []ScriptRef{o.Script}
		s, ok := scripts[o.Script]
		valid := ok && s.ScriptID == o.Script.ID && Hash(s) == o.Script.Revision
		if valid {
			_, err := DecodeScript(Canonical(s))
			valid = err == nil
		}
		if !valid {
			add([]Diagnostic{{Code: "missing_dependency", Severity: "error", Module: "orthography", Path: "orthography.script", Message: "Load the exact retained script revision before compiling written operations."}})
		} else {
			allowed := map[string]bool{}
			for _, g := range s.Graphemes {
				if g.Text != "" {
					allowed[g.Text] = true
				}
			}
			for _, v := range o.Mappings {
				if !allowed[v.Grapheme] {
					valid = false
					add([]Diagnostic{{Code: "invalid_grapheme", Severity: "error", Module: "orthography", RuleID: v.ID, Path: "orthography.mappings", Message: "Mapping output must be an authored Unicode grapheme in the pinned script."}})
				}
			}
			mapped := map[string]bool{}
			for _, m := range o.Mappings {
				mapped[m.Segment] = true
			}
			if d.Phonology != nil {
				for _, segment := range d.Phonology.Segments {
					if !mapped[segment.ID] {
						valid = false
						add([]Diagnostic{{Code: "missing_mapping", Severity: "error", Module: "orthography", RuleID: segment.ID, Path: "orthography.mappings", Message: "Map every declared segment before enabling writing; independent phonological generation remains available."}})
					}
				}
			}
			if valid {
				base.Orthography = &spike.Orthography{ID: o.Script.ID, Normalization: o.Normalization, Mappings: o.Mappings}
				orthographicBase := Clone(base)
				orthographicBase.Lexicon = nil
				orthographicBase.Morphology = nil
				writtenPhonology, orthographicReport := spike.Compile(ctx, orthographicBase, spike.TokenTrieBackend{})
				a.report.DiagnosticsOmitted += orthographicReport.DiagnosticsOmitted
				a.orthographic = writtenPhonology
				a.report.Ready["write"] = orthographicReport.Ready["validate"]
				a.report.States += orthographicReport.States
				w, wr := spike.Compile(ctx, base, spike.TokenTrieBackend{})
				a.report.DiagnosticsOmitted += wr.DiagnosticsOmitted
				a.written = w
				a.report.Ready["analyze-written"] = wr.Ready["analyze-written"]
				a.report.States += wr.States
				for _, v := range wr.Diagnostics {
					if !seen[Hash(v)] {
						add([]Diagnostic{v})
						seen[Hash(v)] = true
					}
				}
			}
		}
	}
	if ProductiveSchema(d.Schema) {
		add(a.compileProductive(ctx))
		if d.Syntax != nil || d.History != nil {
			add([]Diagnostic{{Code: "unsupported_module", Severity: "error", Module: "productive", Path: "syntax/history", Message: "A1 does not execute schema-2 grammar or historical changes. Keep using schema 1 for those operations."}})
		}
	} else {
		add(a.compileGrammar())
	}
	add(a.compileSpelling(ctx, scripts))
	add(a.checkLexical())
	add(a.checkCorpus())
	if d.History != nil && d.Schema == 1 {
		add([]Diagnostic{{Code: "preview_required", Severity: "info", Module: "history", Path: "history", Message: "Historical changes require a separately validated source and an explicit preview/apply operation."}})
	}
	for _, ready := range a.report.Ready {
		if ready {
			a.report.Status = "compiled"
			break
		}
	}
	for _, d := range a.report.Diagnostics {
		if d.Severity == "error" {
			if a.report.Status == "compiled" {
				a.report.Status = "partial"
			} else if a.report.Status == "incomplete" {
				a.report.Status = "invalid"
			}
		}
	}
	if ctx.Err() != nil {
		a.report.Status = "cancelled"
		for k := range a.report.Ready {
			a.report.Ready[k] = false
		}
	}
	return a
}

func (a *Artifact) Execute(ctx context.Context, q Request) Result {
	out := Result{Result: spike.Result{Status: "invalid_input", LanguageID: a.definition.LanguageID, Revision: a.report.Revision, Engine: EngineVersion, Canonicalizer: CanonicalVersion, Selection: spike.SelectionVersion, Request: q.Request, Candidates: []spike.Candidate{}, Diagnostics: []Diagnostic{}}, Adapter: AdapterVersion, Kernel: spike.EngineVersion, Input: Clone(q), Dependencies: Clone(a.report.Dependencies)}
	out.Engine, out.Adapter, out.Compiler, out.Schema = a.report.Engine, a.report.Adapter, a.report.Compiler, a.report.Schema
	fail := func(status, message string) Result {
		out.Status = status
		out.Diagnostics = append(out.Diagnostics, Diagnostic{Code: status, Severity: "error", Module: "execution", Path: "request", Message: message})
		return out
	}
	if q.Engine != a.report.Engine || q.Adapter != a.report.Adapter || q.Compiler != a.report.Compiler {
		message := "Select the exact supported engine and adapter; no fallback is applied."
		if ProductiveSchema(a.definition.Schema) {
			message = "Select the exact supported engine, adapter and compiler; no fallback is applied."
		}
		return fail("incompatible_engine", message)
	}
	if q.Revision != a.report.Revision {
		return fail("uncompiled_revision", "The request must name this exact immutable definition revision.")
	}
	if q.Limit < 1 || q.Limit > 256 || len(q.Seed) > 1024 || len(q.Features) > 64 || len(q.Segments) > 256 || len(q.Text) > 4096 || len(q.Roles) > 16 || len(q.Lexeme) > 80 || len(q.Template) > 80 || len(q.Construction) > 80 {
		return fail("invalid_input", "Request exceeds the documented feature, role, text, seed, segment or candidate bounds.")
	}
	if q.Operation == "spell" || q.Operation == "unspell" || q.Operation == "graphemes" {
		if a.definition.Schema != 4 {
			return fail("unsupported_operation", "Sequence spelling requires explicit language schema 4.")
		}
		return a.executeSpelling(ctx, q, out)
	}
	if q.Spelling != nil {
		return fail("invalid_input", "Spelling input belongs only to sequence spelling operations.")
	}
	if q.Operation == "regression" {
		return a.runCorpus(ctx, q, out)
	}
	if ProductiveSchema(a.definition.Schema) && (q.Operation == "inflect" || q.Operation == "paradigm" || q.Operation == "analyze" || q.Operation == "analyze-written") {
		return a.executeProductive(ctx, q, out)
	}
	if ProductiveSchema(a.definition.Schema) && (q.Operation == "realize" || q.Operation == "parse") {
		return fail("unsupported_operation", "Schema-2 grammar is not implemented by A1; schema-1 grammar remains available.")
	}
	if q.Operation == "realize" {
		return a.realize(ctx, q, out)
	}
	if q.Operation == "parse" {
		return a.parse(ctx, q, out)
	}
	if q.Operation == "write" {
		return a.write(ctx, q, out)
	}
	if q.Construction != "" || len(q.Roles) > 0 {
		return fail("invalid_input", "Construction and role input belong only to sentence realization.")
	}
	if q.Operation == "inflect" || q.Operation == "paradigm" {
		return a.paradigm(ctx, q, out)
	}
	var kernel *spike.Compiled
	switch q.Operation {
	case "generate", "validate":
		kernel = a.phonology
	case "inflect", "analyze":
		kernel = a.morphology
	case "analyze-written":
		kernel = a.written
	default:
		return fail("unsupported_operation", "This engine does not implement the requested operation.")
	}
	if !a.report.Ready[q.Operation] || kernel == nil {
		return fail("missing_dependency", "Compile and correct the dependencies listed for this operation.")
	}
	// Written realization is opt-in via an authored script. Invalid orthography
	// leaves the independently usable phonological/morphological operation intact.
	if q.Operation == "generate" && a.report.Ready["write"] {
		kernel = a.orthographic
	}
	in := Clone(q.Request)
	in.Revision = kernel.Report().Revision
	in.Engine = spike.EngineVersion
	out.Result = kernel.Execute(ctx, in)
	out.Revision = a.report.Revision
	out.Engine = a.report.Engine
	out.Request = q.Request
	return out
}

func (a *Artifact) compileGrammar() []Diagnostic {
	s := a.definition.Syntax
	if s == nil {
		return nil
	}
	diagnostics := []Diagnostic{}
	bad := func(id, message string) {
		diagnostics = append(diagnostics, Diagnostic{Code: "invalid_grammar", Severity: "error", Module: "syntax", RuleID: id, Path: "syntax.constructions", Message: message})
	}
	if len(s.Types) > 64 || len(s.Structures) > 128 || len(s.Constructions) > 64 {
		bad("", "Grammar exceeds its bounded type, structure or construction limits.")
		return diagnostics
	}
	if len(s.Types) > 0 || len(s.Structures) > 0 {
		bad("", "Recursive typed structures are retained as draft data; this adapter currently executes flat role features only.")
	}
	for _, c := range s.Constructions {
		if c.ID == "" || len(c.ID) > 80 || len(c.Meaning) > 1000 || len(c.Roles) > 16 || len(c.Order) > 32 || len(c.Separator) > 16 || len(c.Agreements) > 32 {
			bad(c.ID, "Invalid construction identity or limits.")
			continue
		}
		if _, exists := a.grammar[c.ID]; exists {
			bad(c.ID, "Construction IDs must be unique.")
			continue
		}
		roles := map[string]bool{}
		for _, r := range c.Roles {
			if r.ID == "" || roles[r.ID] || len(r.Features) > 64 || r.Category == "" {
				bad(c.ID, "Roles need unique IDs, explicit categories and bounded feature requirements.")
			}
			roles[r.ID] = true
		}
		used := map[string]bool{}
		for _, v := range c.Order {
			if v.Construction != "" {
				bad(c.ID, "Nested constructions require a future adapter version; no flattening fallback is applied.")
			}
			if v.Role != "" {
				if !roles[v.Role] || v.Literal != "" {
					bad(c.ID, "Order must reference a declared role or an explicit literal.")
				}
				used[v.Role] = true
			} else if v.Literal == "" || len(v.Literal) > 256 {
				bad(c.ID, "Constituents need a bounded role or literal.")
			}
		}
		for id := range roles {
			if !used[id] {
				bad(c.ID, "Every declared role must appear in the constituent order.")
			}
		}
		for _, v := range c.Agreements {
			if v.ID == "" || !roles[v.LeftRole] || !roles[v.RightRole] || len(v.Features) == 0 || len(v.Features) > 64 {
				bad(c.ID, "Agreement requires two declared roles and explicit feature names.")
			}
		}
		a.grammar[c.ID] = c
	}
	a.report.Ready["realize"] = len(diagnostics) == 0 && len(a.grammar) > 0 && a.report.Ready["inflect"]
	a.report.Ready["parse"] = a.report.Ready["realize"]
	return diagnostics
}

func (a *Artifact) realize(ctx context.Context, q Request, out Result) Result {
	fail := func(status, message string) Result {
		out.Status = status
		out.Diagnostics = []Diagnostic{{Code: status, Severity: "error", Module: "syntax", Path: "request", Message: message}}
		return out
	}
	if !a.report.Ready["realize"] {
		return fail("missing_dependency", "Compile a valid grammar and its morphological dependencies first.")
	}
	c, ok := a.grammar[q.Construction]
	if !ok || len(q.Roles) != len(c.Roles) || q.Lexeme != "" || len(q.Features) > 0 || len(q.Segments) > 0 || q.Template != "" || q.Text != "" || q.Limit < 1 || q.Limit > 256 || q.Policy != "explicit" || q.Seed != "" {
		return fail("invalid_input", "Choose a construction, exactly its declared roles, and explicit selection; sentence input cannot contain word-operation fields.")
	}
	features := map[string]Features{}
	for _, r := range c.Roles {
		in, found := q.Roles[r.ID]
		if !found {
			return fail("invalid_input", "A declared role is missing.")
		}
		var lex *spike.Lexeme
		for i := range a.definition.Lexicon {
			if a.definition.Lexicon[i].ID == in.Lexeme {
				lex = &a.definition.Lexicon[i]
				break
			}
		}
		if lex == nil || lex.Category != r.Category {
			return fail("invalid_input", "Role lexeme is missing or has an incompatible category.")
		}
		f, unified := spike.Unify(r.Features, in.Features)
		if unified {
			f, unified = spike.Unify(f, lex.Features)
		}
		if !unified {
			return fail("unification_failure", "Lexical, role and requested features conflict.")
		}
		features[r.ID] = f
	}
	// Equalities propagate until fixed point. Missing values remain undecided;
	// they are never filled with a language-independent default.
	for pass := 0; pass <= len(c.Roles); pass++ {
		changed := false
		for _, eq := range c.Agreements {
			left, right := features[eq.LeftRole], features[eq.RightRole]
			for _, key := range eq.Features {
				l, lok := left[key]
				r, rok := right[key]
				if lok && rok && l != r {
					return fail("unification_failure", fmt.Sprintf("Agreement %s conflicts on %s.", eq.ID, key))
				}
				if lok && !rok {
					right[key] = l
					changed = true
				}
				if rok && !lok {
					left[key] = r
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	sentence := Sentence{Construction: c.ID, Roles: map[string]spike.Candidate{}, Tokens: []string{}, Trace: []spike.Trace{}}
	for _, r := range c.Roles {
		request := Request{Request: spike.Request{Revision: a.report.Revision, Engine: EngineVersion, Operation: "inflect", Lexeme: q.Roles[r.ID].Lexeme, Features: features[r.ID], Policy: "explicit", Limit: 256}, Adapter: AdapterVersion}
		result := a.Execute(ctx, request)
		if result.Status != "success" || len(result.Candidates) != 1 {
			return fail(result.Status, "A role requires one explicit valid morphological realization; resolve its feature ambiguity first.")
		}
		sentence.Roles[r.ID] = result.Candidates[0]
	}
	for _, v := range c.Order {
		if v.Role != "" {
			form := sentence.Roles[v.Role]
			text := form.Written
			if text == "" {
				text = strings.Join(form.Segments, " ")
			}
			sentence.Tokens = append(sentence.Tokens, text)
		} else {
			sentence.Tokens = append(sentence.Tokens, v.Literal)
		}
	}
	sentence.Written = strings.Join(sentence.Tokens, c.Separator)
	a.gloss(&sentence, c)
	for _, v := range c.Agreements {
		sentence.Trace = append(sentence.Trace, spike.Trace{Stage: "agreement", RuleID: v.ID, Before: []string{v.LeftRole, v.RightRole}, After: append([]string(nil), v.Features...)})
	}
	out.Status = "success"
	out.KnownValid = 1
	out.Examined = len(c.Roles)
	out.Sentences = []Sentence{sentence}
	return out
}

func Operations(report Report) []string {
	ops := []string{}
	for op, ready := range report.Ready {
		if ready {
			ops = append(ops, op)
		}
	}
	sort.Strings(ops)
	return ops
}
