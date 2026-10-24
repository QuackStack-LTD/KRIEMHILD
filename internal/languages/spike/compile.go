package spike

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

type Report struct {
	Revision           string          `json:"revision"`
	Engine             string          `json:"engine"`
	Status             string          `json:"status"`
	Ready              map[string]bool `json:"ready"`
	Diagnostics        []Diagnostic    `json:"diagnostics"`
	DiagnosticsOmitted int             `json:"diagnosticsOmitted"`
	Forms              int             `json:"forms"`
	States             int             `json:"states"`
}
type Compiled struct {
	definition        Definition
	report            Report
	segments          map[string]Segment
	classes           map[string]NaturalClass
	templates         map[string]Pattern
	grammatical       map[string]map[string]bool
	mappings          map[string]Mapping
	lexemes           map[string]Lexeme
	forms             []Candidate
	phonetic, written Relation
}

func (c *Compiled) Report() Report { return copyJSON(c.report) }
func validID(s string) bool {
	if len(s) < 1 || len(s) > 80 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._:-", r)) {
			return false
		}
	}
	return true
}
func Compile(ctx context.Context, def Definition, backend RelationBackend) (*Compiled, Report) {
	c := &Compiled{definition: copyJSON(def), segments: map[string]Segment{}, classes: map[string]NaturalClass{}, templates: map[string]Pattern{}, grammatical: map[string]map[string]bool{}, mappings: map[string]Mapping{}, lexemes: map[string]Lexeme{}}
	def = c.definition
	c.report = Report{Revision: Hash(def), Engine: EngineVersion, Status: "incomplete", Ready: map[string]bool{}, Diagnostics: []Diagnostic{}}
	add := func(code, module, id, path, message string) {
		if len(c.report.Diagnostics) >= 256 {
			c.report.DiagnosticsOmitted++
			return
		}
		c.report.Diagnostics = append(c.report.Diagnostics, Diagnostic{code, "error", module, id, path, message})
	}
	missing := func(module, message string) {
		c.report.Diagnostics = append(c.report.Diagnostics, Diagnostic{"missing_dependency", "warning", module, "", module, message})
	}
	seen := map[string]bool{}
	id := func(namespace, value, path string) {
		key := namespace + "/" + value
		if !validID(value) || seen[key] {
			add("invalid_identity", namespace, value, path, "Use a unique stable ID with letters, digits, dot, colon, hyphen or underscore.")
		}
		seen[key] = true
	}
	if len(Canonical(def)) > 2<<20 {
		add("definition_limit", "definition", "", "definition", "The spike accepts definitions up to 2 MiB.")
	}
	if def.Schema != SchemaVersion || !validID(def.LanguageID) || strings.TrimSpace(def.Name) == "" || len(def.Name) > 160 || len(def.Notes) > 8192 {
		add("invalid_definition", "definition", "", "definition", "Choose schema 1, a stable identity and a nonempty name of at most 160 bytes.")
	}
	featureInventory := func(values []Feature, module string) map[string]map[string]bool {
		inventory := map[string]map[string]bool{}
		if len(values) > 64 {
			add("definition_limit", module, "", module+".features", "At most 64 features are supported in the spike.")
		}
		for i, f := range values {
			path := fmt.Sprintf("%s.features[%d]", module, i)
			id(module+"-feature", f.ID, path)
			allowed := map[string]bool{}
			if len(f.Values) == 0 || len(f.Values) > 64 {
				add("invalid_feature", module, f.ID, path, "Define 1–64 explicit values.")
			}
			for _, v := range f.Values {
				if v == "" || len(v) > 80 || allowed[v] {
					add("invalid_feature", module, f.ID, path, "Feature values must be nonempty and unique.")
				}
				allowed[v] = true
			}
			inventory[f.ID] = allowed
		}
		return inventory
	}
	validateFeatures := func(features Features, inventory map[string]map[string]bool, module, owner, path string) {
		keys := make([]string, 0, len(features))
		for k := range features {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if !inventory[key][features[key]] {
				add("unknown_feature", module, owner, path+"."+key, "Declare this feature and value in the module inventory.")
			}
		}
	}
	var inventory map[string]map[string]bool
	if p := def.Phonology; p != nil {
		inventory = featureInventory(p.Features, "phonology")
		if len(p.Segments) > 128 || len(p.Classes) > 64 || len(p.Rules) > 32 || len(p.Templates) > 32 || len(p.Allowed) > 64 || len(p.Forbidden) > 64 {
			add("definition_limit", "phonology", "", "phonology", "Inventory/rule/template counts exceed spike limits.")
		}
		for i, s := range p.Segments {
			id("segment", s.ID, fmt.Sprintf("phonology.segments[%d]", i))
			validateFeatures(s.Features, inventory, "phonology", s.ID, "phonology.segments."+s.ID+".features")
			if len(s.Label) > 160 {
				add("definition_limit", "phonology", s.ID, "phonology.segments."+s.ID, "Segment label exceeds 160 bytes.")
			}
			c.segments[s.ID] = s
		}
		for i, n := range p.Classes {
			id("class", n.ID, fmt.Sprintf("phonology.classes[%d]", i))
			if len(n.Features) == 0 {
				add("empty_class", "phonology", n.ID, "phonology.classes."+n.ID, "Define at least one feature predicate.")
			}
			validateFeatures(n.Features, inventory, "phonology", n.ID, "phonology.classes."+n.ID)
			c.classes[n.ID] = n
		}
		validatePattern := func(slots []Match, owner, path string, allowBoundary bool) {
			if len(slots) > 16 {
				add("pattern_limit", "phonology", owner, path, "Patterns have at most 16 positions in the spike.")
			}
			for i, m := range slots {
				count := 0
				if m.Segment != "" {
					count++
				}
				if m.Class != "" {
					count++
				}
				if m.Boundary != "" {
					count++
				}
				ok := count == 1
				if m.Segment != "" {
					_, exists := c.segments[m.Segment]
					ok = ok && exists
				}
				if m.Class != "" {
					_, exists := c.classes[m.Class]
					ok = ok && exists
				}
				if m.Boundary != "" {
					ok = ok && allowBoundary && m.Boundary == "word"
				}
				if !ok {
					add("invalid_pattern", "phonology", owner, fmt.Sprintf("%s[%d]", path, i), "Select one defined segment/class, or an allowed word boundary.")
				}
			}
		}
		for _, group := range []struct {
			name   string
			values []Pattern
		}{{"templates", p.Templates}, {"allowed", p.Allowed}, {"forbidden", p.Forbidden}} {
			for i, pattern := range group.values {
				path := fmt.Sprintf("phonology.%s[%d]", group.name, i)
				id("pattern", pattern.ID, path)
				if len(pattern.Slots) == 0 {
					add("empty_pattern", "phonology", pattern.ID, path, "Define at least one position.")
				}
				validatePattern(pattern.Slots, pattern.ID, path+".slots", group.name == "forbidden")
				if group.name == "templates" {
					c.templates[pattern.ID] = pattern
				}
			}
		}
		for i, rule := range p.Rules {
			path := fmt.Sprintf("phonology.rules[%d]", i)
			id("rule", rule.ID, path)
			if rule.Domain != "word" {
				add("unsupported_rule", "phonology", rule.ID, path+".domain", "The spike implements a single ordered word stratum only.")
			}
			validatePattern(rule.Input, rule.ID, path+".input", false)
			validatePattern(rule.Left, rule.ID, path+".left", true)
			validatePattern(rule.Right, rule.ID, path+".right", true)
			if len(rule.Input) == 0 && len(rule.Output) == 0 {
				add("empty_rule", "phonology", rule.ID, path, "An insertion needs an explicit output.")
			}
			if len(rule.Output) > 16 {
				add("pattern_limit", "phonology", rule.ID, path+".output", "At most 16 output segments are supported.")
			}
			for _, segment := range rule.Output {
				if _, ok := c.segments[segment]; !ok {
					add("unknown_segment", "phonology", rule.ID, path+".output", "Declare every output segment.")
				}
			}
		}
		if len(p.Segments) == 0 {
			missing("phonology", "Define a segment inventory before executing phonology.")
		}
		if len(p.Allowed) == 0 {
			missing("phonotactics", "Define allowed complete-word patterns before validating or generating forms.")
		}
	} else {
		missing("phonology", "No phonology is defined; phonological operations are unavailable.")
	}
	validateSegments := func(values []string, phonemic bool, module, owner, path string) {
		if len(values) > 64 {
			add("form_limit", module, owner, path, "Authored sequences have at most 64 segments.")
		}
		for _, s := range values {
			segment, ok := c.segments[s]
			if !ok || phonemic && !segment.Phonemic {
				add("unknown_segment", module, owner, path, "Use declared phonemic segment IDs in underlying forms and morphemes.")
			}
		}
	}
	paradigms := map[string]Paradigm{}
	if m := def.Morphology; m != nil {
		c.grammatical = featureInventory(m.Features, "morphology")
		if len(m.Paradigms) > 64 {
			add("definition_limit", "morphology", "", "morphology.paradigms", "At most 64 paradigms are supported.")
		}
		for i, p := range m.Paradigms {
			path := fmt.Sprintf("morphology.paradigms[%d]", i)
			id("paradigm", p.ID, path)
			if len(p.Cells) == 0 || len(p.Cells) > 64 {
				add("paradigm_limit", "morphology", p.ID, path, "Define 1–64 explicit cells.")
			}
			for j, cell := range p.Cells {
				cellPath := fmt.Sprintf("%s.cells[%d]", path, j)
				id("cell", cell.ID, cellPath)
				validateFeatures(cell.Features, c.grammatical, "morphology", cell.ID, cellPath+".features")
				if len(cell.Operations) > 16 {
					add("definition_limit", "morphology", cell.ID, cellPath+".operations", "At most 16 operations are supported per cell.")
				}
				for k, op := range cell.Operations {
					opPath := fmt.Sprintf("%s.operations[%d]", cellPath, k)
					id("rule", op.ID, opPath)
					if op.Kind != "prefix" && op.Kind != "suffix" && op.Kind != "zero" {
						add("unsupported_rule", "morphology", op.ID, opPath+".kind", "The spike supports prefix, suffix, zero marking and explicit lexical suppletion only.")
					}
					if op.Kind == "zero" && len(op.Segments) > 0 || op.Kind != "zero" && len(op.Segments) == 0 {
						add("invalid_operation", "morphology", op.ID, opPath, "Affixes need segments; zero realization has none.")
					}
					validateSegments(op.Segments, true, "morphology", op.ID, opPath+".segments")
				}
			}
			paradigms[p.ID] = p
		}
	} else {
		missing("morphology", "No paradigms are defined; inflection and morphological analysis are unavailable.")
	}
	if len(def.Lexicon) > 1000 {
		add("definition_limit", "lexicon", "", "lexicon", "At most 1000 fixture lexemes are supported.")
	}
	for i, lexeme := range def.Lexicon {
		path := fmt.Sprintf("lexicon[%d]", i)
		id("lexeme", lexeme.ID, path)
		if len(lexeme.Underlying) == 0 || len(lexeme.Category) > 80 || len(lexeme.Senses) == 0 || len(lexeme.Senses) > 32 {
			add("incomplete_lexeme", "lexicon", lexeme.ID, path, "Define underlying segments, senses and bounded category metadata.")
		}
		validateSegments(lexeme.Underlying, true, "lexicon", lexeme.ID, path+".underlying")
		validateFeatures(lexeme.Features, c.grammatical, "lexicon", lexeme.ID, path+".features")
		for j, sense := range lexeme.Senses {
			id("sense", sense.ID, fmt.Sprintf("%s.senses[%d]", path, j))
			if strings.TrimSpace(sense.Meaning) == "" || len(sense.Meaning) > 1024 {
				add("invalid_sense", "lexicon", lexeme.ID, path+".senses", "Give each sense a nonempty meaning up to 1024 bytes.")
			}
		}
		p, exists := paradigms[lexeme.Paradigm]
		if !exists {
			add("unknown_paradigm", "lexicon", lexeme.ID, path+".paradigm", "Reference a defined paradigm.")
		}
		cells := map[string]bool{}
		for _, cell := range p.Cells {
			cells[cell.ID] = true
		}
		overrides := map[string]bool{}
		for j, override := range lexeme.Overrides {
			overridePath := fmt.Sprintf("%s.overrides[%d]", path, j)
			id("rule", override.RuleID, overridePath)
			if !cells[override.Cell] || overrides[override.Cell] {
				add("invalid_override", "morphology", override.RuleID, overridePath, "Choose a unique existing paradigm cell.")
			}
			overrides[override.Cell] = true
			validateSegments(override.Segments, true, "lexicon", lexeme.ID, overridePath+".segments")
		}
		c.lexemes[lexeme.ID] = lexeme
	}
	if o := def.Orthography; o != nil {
		id("orthography", o.ID, "orthography.id")
		if o.Normalization != "none" && o.Normalization != "NFC" && o.Normalization != "NFD" {
			add("unsupported_normalization", "orthography", o.ID, "orthography.normalization", "Choose none, NFC or NFD explicitly.")
		}
		if len(o.Mappings) > 128 {
			add("definition_limit", "orthography", o.ID, "orthography.mappings", "At most 128 mappings are supported.")
		}
		for i, m := range o.Mappings {
			path := fmt.Sprintf("orthography.mappings[%d]", i)
			id("rule", m.ID, path)
			_, known := c.segments[m.Segment]
			_, duplicate := c.mappings[m.Segment]
			if !known || duplicate || m.Grapheme == "" || len(m.Grapheme) > 128 || !utf8.ValidString(m.Grapheme) {
				add("invalid_mapping", "orthography", m.ID, path, "Map one defined segment once to a nonempty UTF-8 string of at most 128 bytes.")
			}
			c.mappings[m.Segment] = m
		}
	} else {
		missing("orthography", "No orthography is defined; written rendering and written analysis are unavailable.")
	}
	failed := func() bool {
		for _, d := range c.report.Diagnostics {
			if d.Severity == "error" {
				return true
			}
		}
		return false
	}
	finish := func() (*Compiled, Report) {
		if failed() {
			c.report.Status = "invalid"
			c.report.Ready = map[string]bool{}
		}
		return c, c.Report()
	}
	if err := ctx.Err(); err != nil {
		add("cancelled", "compiler", "", "definition", err.Error())
		return finish()
	}
	if failed() {
		return finish()
	}
	p := def.Phonology
	phonologyReady := p != nil && len(p.Segments) > 0 && len(p.Allowed) > 0
	c.report.Ready["generate"] = phonologyReady && len(p.Templates) > 0
	c.report.Ready["validate"] = phonologyReady
	c.report.Ready["inflect"] = phonologyReady && def.Morphology != nil && len(def.Lexicon) > 0
	c.report.Ready["analyze"] = c.report.Ready["inflect"]
	c.report.Ready["analyze-written"] = c.report.Ready["analyze"] && def.Orthography != nil
	if !c.report.Ready["inflect"] {
		return finish()
	}
	if backend == nil {
		add("missing_backend", "compiler", "", "compiler", "Choose an explicit relation backend.")
		return finish()
	}
	pairs, written := []Pair{}, []Pair{}
	formBytes := 0
	for _, lexeme := range def.Lexicon {
		for _, cell := range paradigms[lexeme.Paradigm].Cells {
			if err := ctx.Err(); err != nil {
				add("cancelled", "compiler", "", "definition", err.Error())
				return finish()
			}
			features, ok := Unify(lexeme.Features, cell.Features)
			if !ok {
				continue
			}
			if len(pairs) >= 10000 {
				add("relation_limit", "morphology", "", "morphology", "The spike compiles at most 10000 lexical forms.")
				return finish()
			}
			form := Candidate{Lexeme: lexeme.ID, Cell: cell.ID, Features: features, Underlying: append([]string{}, lexeme.Underlying...), Trace: []Trace{{Stage: "lexicon", RuleID: lexeme.ID, Before: []string{}, After: append([]string{}, lexeme.Underlying...)}}}
			input := append([]string{}, lexeme.Underlying...)
			suppletive := false
			for _, override := range lexeme.Overrides {
				if override.Cell == cell.ID {
					form.Trace = append(form.Trace, Trace{"suppletion", override.RuleID, append([]string{}, input...), append([]string{}, override.Segments...)})
					input = append([]string{}, override.Segments...)
					suppletive = true
					break
				}
			}
			if !suppletive {
				for _, op := range cell.Operations {
					before := append([]string{}, input...)
					switch op.Kind {
					case "prefix":
						input = append(append([]string{}, op.Segments...), input...)
					case "suffix":
						input = append(input, op.Segments...)
					}
					form.Trace = append(form.Trace, Trace{"morphology", op.ID, before, append([]string{}, input...)})
				}
			}
			if len(input) > 256 {
				add("form_limit", "morphology", cell.ID, "lexicon."+lexeme.ID, "Realized form exceeds 256 segments.")
				continue
			}
			out, trace, diagnostic := c.phonology(ctx, input)
			form.Trace = append(form.Trace, trace...)
			if diagnostic != nil {
				add(diagnostic.Code, diagnostic.Module, lexeme.ID, "lexicon."+lexeme.ID, diagnostic.Message)
				continue
			}
			form.Segments = out
			if d := c.validateForm(out); d != nil {
				add(d.Code, d.Module, lexeme.ID, "lexicon."+lexeme.ID, d.Message)
				continue
			}
			chunks, orthoTrace, diagnostic := c.render(out)
			if diagnostic != nil {
				add(diagnostic.Code, diagnostic.Module, lexeme.ID, "lexicon."+lexeme.ID, diagnostic.Message)
				continue
			}
			form.Written = strings.Join(chunks, "")
			form.Trace = append(form.Trace, orthoTrace...)
			formBytes += len(Canonical(form))
			if formBytes > 8<<20 {
				add("artifact_limit", "compiler", "", "compiler", "Compiled form/trace payload exceeds the 8 MiB spike budget.")
				return finish()
			}
			index := len(c.forms)
			c.forms = append(c.forms, form)
			upper := analysisTape(lexeme.ID, features)
			pairs = append(pairs, Pair{upper, form.Segments, index})
			if def.Orthography != nil {
				written = append(written, Pair{upper, chunks, index})
			}
		}
	}
	if failed() {
		return finish()
	}
	var err error
	c.phonetic, err = backend.Build(ctx, pairs, 100000)
	if err != nil {
		add("relation_limit", "compiler", "", "compiler", err.Error())
		return finish()
	}
	if def.Orthography != nil {
		c.written, err = backend.Build(ctx, written, 100000)
		if err != nil {
			add("relation_limit", "compiler", "", "compiler", err.Error())
			return finish()
		}
	}
	c.report.Forms = len(c.forms)
	c.report.States = c.phonetic.States()
	if c.written != nil {
		c.report.States += c.written.States()
	}
	c.report.Status = "compiled"
	return finish()
}

func Unify(a, b Features) (Features, bool) {
	out := Features{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if prior, ok := out[k]; ok && prior != v {
			return nil, false
		}
		out[k] = v
	}
	return out, true
}
func analysisTape(lexeme string, features Features) []string {
	return []string{"lexeme:" + lexeme, string(Canonical(features))}
}
func normalize(s, policy string) string {
	switch policy {
	case "NFC":
		return norm.NFC.String(s)
	case "NFD":
		return norm.NFD.String(s)
	}
	return s
}
