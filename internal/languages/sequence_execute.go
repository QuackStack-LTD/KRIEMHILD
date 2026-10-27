package languages

import (
	"context"
	"golang.org/x/text/unicode/norm"
	"kriemhild/internal/languages/spike"
	"sort"
	"strings"
)

type SpellingValue struct {
	Unit    OrthoUnit `json:"unit"`
	Sources []int     `json:"sources"`
	Rules   []string  `json:"rules"`
}
type SpellingMatch struct {
	Rule   string `json:"rule"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Action string `json:"action"`
}
type SpellingTrace struct {
	Stage       string          `json:"stage"`
	Kind        string          `json:"kind"`
	InputLayer  string          `json:"inputLayer"`
	OutputLayer string          `json:"outputLayer"`
	OffsetUnit  string          `json:"offsetUnit"`
	Before      []SpellingValue `json:"before"`
	After       []SpellingValue `json:"after"`
	Matches     []SpellingMatch `json:"matches"`
	Original    string          `json:"original,omitempty"`
	Normalized  string          `json:"normalized,omitempty"`
}
type SpellingRun struct {
	ID         string                `json:"id"`
	Form       *spike.Candidate      `json:"form,omitempty"`
	Derivation *ProductiveDerivation `json:"derivation,omitempty"`
	Input      []OrthoUnit           `json:"input"`
	Output     []SpellingValue       `json:"output"`
	Stages     []SpellingTrace       `json:"stages"`
	Text       string                `json:"text"`
	Clusters   []GraphemeCluster     `json:"clusters"`
	Render     *ScriptResult         `json:"render,omitempty"`
}
type SpellingPreimage struct {
	ID               string                 `json:"id"`
	Units            []OrthoUnit            `json:"units"`
	Forward          SpellingRun            `json:"forward"`
	Analyses         []spike.Candidate      `json:"analyses"`
	Derivations      []ProductiveDerivation `json:"derivations"`
	MorphologyStatus string                 `json:"morphologyStatus,omitempty"`
	Inverse          *InverseMetrics        `json:"inverse,omitempty"`
}
type SpellingResult struct {
	Version          string             `json:"version"`
	Segmentation     string             `json:"segmentation"`
	Normalizer       string             `json:"normalizer"`
	InputLayer       string             `json:"inputLayer"`
	OutputLayer      string             `json:"outputLayer"`
	Script           ScriptRef          `json:"script"`
	Renderer         string             `json:"renderer"`
	RendererAdapter  string             `json:"rendererAdapter"`
	RendererCompiler string             `json:"rendererCompiler"`
	Steps            int                `json:"steps"`
	Clusters         []GraphemeCluster  `json:"clusters"`
	Selection        *ClusterSelection  `json:"selection,omitempty"`
	Spellings        []SpellingRun      `json:"spellings"`
	Preimages        []SpellingPreimage `json:"preimages"`
	Choice           string             `json:"choice,omitempty"`
}
type spellingBudget struct {
	ctx                 context.Context
	steps, bytes, trace int
}

func spellingError(code, id, path, msg string) error {
	return &productiveFailure{Diagnostic{Code: code, Severity: "error", Module: "sequenceWriting", RuleID: id, Path: path, Message: msg}}
}
func (b *spellingBudget) use(n, bytes int) error {
	if err := b.ctx.Err(); err != nil {
		return spellingError("cancelled", "", "request", err.Error())
	}
	b.steps += n
	b.bytes += bytes
	if b.steps > 100000 || b.bytes > 8<<20 {
		return spellingError("execution_limit", "", "request", "Spelling exceeded 100,000 steps or 8 MiB; no partial spelling set is complete.")
	}
	return nil
}
func valuesUnits(v []SpellingValue) []OrthoUnit {
	out := []OrthoUnit{}
	for _, x := range v {
		out = append(out, x.Unit)
	}
	return out
}
func annotate(units []OrthoUnit) []SpellingValue {
	out := []SpellingValue{}
	for i, u := range units {
		out = append(out, SpellingValue{u, []int{i}, []string{}})
	}
	return out
}
func provenance(values []SpellingValue, rule string) ([]int, []string) {
	sources := []int{}
	rules := []string{}
	ss := map[int]bool{}
	rr := map[string]bool{}
	for _, v := range values {
		for _, s := range v.Sources {
			if !ss[s] {
				ss[s] = true
				sources = append(sources, s)
			}
		}
		for _, r := range v.Rules {
			if !rr[r] {
				rr[r] = true
				rules = append(rules, r)
			}
		}
	}
	sort.Ints(sources)
	if rule != "" && !rr[rule] {
		rules = append(rules, rule)
	}
	return sources, rules
}
func offsetUnit(layer string) string {
	if layer == "grapheme" {
		return "grapheme-cluster"
	}
	if layer == "sign" {
		return "sign"
	}
	return "source-token"
}
func spellingAt(b *spellingBudget, pattern []OrthoUnit, in []SpellingValue, start int) (bool, error) {
	for i, u := range pattern {
		if err := b.use(1, 0); err != nil {
			return false, err
		}
		pos := start + i
		if u.Kind == "edge" {
			if pos != -1 && pos != len(in) {
				return false, nil
			}
			continue
		}
		if pos < 0 || pos >= len(in) || !sameUnit(u, in[pos].Unit) {
			return false, nil
		}
	}
	return true, nil
}
func resegment(b *spellingBudget, values []SpellingValue) ([]SpellingValue, []GraphemeCluster, error) {
	var text strings.Builder
	ends := []int{}
	for _, v := range values {
		if err := b.use(1, 0); err != nil {
			return nil, nil, err
		}
		text.WriteString(v.Unit.Value)
		ends = append(ends, text.Len())
	}
	clusters, err := SegmentGraphemes(b.ctx, text.String())
	if err != nil {
		return nil, nil, err
	}
	out := []SpellingValue{}
	first := 0
	for _, g := range clusters {
		contributors := []SpellingValue{}
		for first < len(ends) && ends[first] <= g.ByteStart {
			first++
		}
		for i := first; i < len(values); i++ {
			start := 0
			if i > 0 {
				start = ends[i-1]
			}
			if start >= g.ByteEnd {
				break
			}
			if err := b.use(1, 0); err != nil {
				return nil, nil, err
			}
			if ends[i] > g.ByteStart && start < g.ByteEnd {
				contributors = append(contributors, values[i])
			}
		}
		s, r := provenance(contributors, "")
		out = append(out, SpellingValue{OrthoUnit{Kind: "grapheme", Value: g.Text}, s, r})
	}
	return out, clusters, nil
}
func rewriteStage(b *spellingBudget, stage SpellingStage, in []SpellingValue) (SpellingTrace, error) {
	trace := SpellingTrace{Stage: stage.ID, Kind: stage.Kind, InputLayer: stage.InputLayer, OutputLayer: stage.OutputLayer, OffsetUnit: offsetUnit(stage.InputLayer), Before: in, After: []SpellingValue{}, Matches: []SpellingMatch{}}
	if stage.Kind == "normalize" {
		for _, v := range in {
			trace.Original += v.Unit.Value
		}
		if stage.Normalization == "NFC" {
			trace.Normalized = norm.NFC.String(trace.Original)
		} else {
			trace.Normalized = norm.NFD.String(trace.Original)
		}
		clusters, err := SegmentGraphemes(b.ctx, trace.Normalized)
		if err != nil {
			return trace, err
		}
		if len(clusters) != len(in) {
			return trace, spellingError("unsupported_normalization", stage.ID, "stage", "Normalized grapheme alignment changed; provenance is not guessed.")
		}
		for i, g := range clusters {
			if err := b.use(1, 0); err != nil {
				return trace, err
			}
			s, r := provenance(in[i:i+1], stage.ID)
			trace.After = append(trace.After, SpellingValue{OrthoUnit{Kind: "grapheme", Value: g.Text}, s, r})
		}
		action := "replaced"
		if trace.Original == trace.Normalized {
			action = "unchanged"
		}
		trace.Matches = append(trace.Matches, SpellingMatch{stage.ID, 0, len(in), action})
		return trace, nil
	}
	for pos := 0; pos < len(in); {
		if err := b.use(1, 0); err != nil {
			return trace, err
		}
		var selected *SpellingRule
		for ri := range stage.Rules {
			r := &stage.Rules[ri]
			matched, err := spellingAt(b, r.Input, in, pos)
			if err != nil {
				return trace, err
			}
			if !matched {
				continue
			}
			matched, err = spellingAt(b, r.Left, in, pos-len(r.Left))
			if err != nil {
				return trace, err
			}
			if !matched {
				continue
			}
			matched, err = spellingAt(b, r.Right, in, pos+len(r.Input))
			if err != nil {
				return trace, err
			}
			if !matched {
				continue
			}
			if selected == nil || r.Priority > selected.Priority {
				selected = r
			}
		}
		if selected == nil {
			if stage.Unmatched != "identity" {
				return trace, spellingError("no_mapping", stage.ID, "stage", "No authored rule maps input unit at "+offsetUnit(stage.InputLayer)+" offset "+number(float64(pos))+".")
			}
			trace.After = append(trace.After, in[pos])
			trace.Matches = append(trace.Matches, SpellingMatch{"", pos, pos + 1, "unchanged"})
			pos++
			continue
		}
		end := pos + len(selected.Input)
		s, r := provenance(in[pos:end], selected.ID)
		for _, u := range selected.Output {
			trace.After = append(trace.After, SpellingValue{u, s, r})
		}
		action := "replaced"
		if len(selected.Output) == 0 {
			action = "deleted"
		} else if string(Canonical(selected.Input)) == string(Canonical(selected.Output)) {
			action = "unchanged"
		}
		trace.Matches = append(trace.Matches, SpellingMatch{selected.ID, pos, end, action})
		pos = end
		if len(trace.After) > 512 {
			return trace, spellingError("execution_limit", stage.ID, "stage", "Stage exceeds 512 output units.")
		}
	}
	if stage.OutputLayer == "grapheme" {
		var err error
		trace.After, _, err = resegment(b, trace.After)
		if err != nil {
			return trace, err
		}
	}
	return trace, nil
}
func (a *Artifact) spellRun(b *spellingBudget, input []OrthoUnit) (SpellingRun, error) {
	run := SpellingRun{Input: Clone(input), Output: []SpellingValue{}, Stages: []SpellingTrace{}, Clusters: []GraphemeCluster{}}
	if len(input) > 512 {
		return run, spellingError("execution_limit", "", "input", "Input exceeds 512 units.")
	}
	values := annotate(input)
	w := a.definition.SequenceWriting
	for _, stage := range w.Stages {
		trace, err := rewriteStage(b, stage, values)
		if err != nil {
			return run, err
		}
		if stage.OutputLayer == "grapheme" {
			run.Text = ""
			for _, v := range trace.After {
				run.Text += v.Unit.Value
			}
			run.Clusters, err = SegmentGraphemes(b.ctx, run.Text)
			if err != nil {
				return run, err
			}
		}
		size := len(Canonical(trace))
		b.trace += size
		if b.trace > 1<<20 {
			return run, spellingError("execution_limit", stage.ID, "trace", "Spelling trace exceeds 1 MiB.")
		}
		if err := b.use(1, size); err != nil {
			return run, err
		}
		run.Stages = append(run.Stages, trace)
		values = trace.After
	}
	run.Output = values
	if w.Stages[len(w.Stages)-1].OutputLayer == "sign" {
		items := []SignUse{}
		for _, v := range values {
			items = append(items, *v.Unit.Sign)
		}
		r := RenderScript(b.ctx, *a.spellingScript, ScriptRenderRequest{Revision: w.Script.Revision, Engine: w.Renderer, Adapter: w.RendererAdapter, Compiler: w.RendererCompiler, Sequence: &SignSequence{ID: "spelling", Name: "Spelled sign sequence", Items: items}})
		if r.Status != "success" {
			return run, spellingError(r.Status, "", "render", "Pinned Script rendering failed; inspect glyph/reading dependencies.")
		}
		run.Render = &r
	}
	run.ID = Hash(struct {
		Revision string
		Input    []OrthoUnit
	}{a.report.Revision, input})
	if err := b.use(1, len(Canonical(run))); err != nil {
		return run, err
	}
	return run, nil
}
func derivationUnits(c spike.Candidate, d ProductiveDerivation, layer string) []OrthoUnit {
	out := []OrthoUnit{}
	if layer == "surface-boundaries" {
		for _, step := range d.Steps {
			if step.Stage == "erase-boundaries" {
				for _, t := range step.Before {
					if t.Boundary != "" {
						out = append(out, OrthoUnit{Kind: "boundary", Value: t.Boundary})
					} else {
						out = append(out, OrthoUnit{Kind: "segment", Value: t.Segment})
					}
				}
				return out
			}
		}
	}
	for _, s := range c.Segments {
		out = append(out, OrthoUnit{Kind: "segment", Value: s})
	}
	return out
}
func (a *Artifact) executeSpelling(ctx context.Context, q Request, out Result) Result {
	b := &spellingBudget{ctx: ctx}
	r := &SpellingResult{Version: SequenceVersion, Segmentation: GraphemeVersion, Normalizer: NormalizationVersion, Clusters: []GraphemeCluster{}, Spellings: []SpellingRun{}, Preimages: []SpellingPreimage{}}
	out.Spelling = r
	defer func() { r.Steps = b.steps }()
	fail := func(err error) Result {
		d := Diagnostic{Code: "invalid_input", Severity: "error", Module: "sequenceWriting", Path: "request", Message: err.Error()}
		if f, ok := err.(*productiveFailure); ok {
			d = f.diagnostic
		}
		out.Status = d.Code
		out.Diagnostics = []Diagnostic{d}
		r.Spellings = []SpellingRun{}
		r.Preimages = []SpellingPreimage{}
		out.KnownValid = 0
		return out
	}
	if err := b.use(1, 0); err != nil {
		return fail(err)
	}
	if q.Spelling != nil {
		if len(q.Spelling.Units) > 512 || len(q.Spelling.Choice) > 64 {
			return fail(spellingError("execution_limit", "", "spelling", "Request exceeds unit/choice bounds."))
		}
		for _, u := range q.Spelling.Units {
			if len(u.Value) > 4096 {
				return fail(spellingError("execution_limit", "", "spelling.units", "Unit exceeds 4 KiB."))
			}
			if s := u.Sign; s != nil {
				if len(s.Children) > 0 || len(s.Context) > 32 || len(s.Roles) > 6 || len(s.Sign) > 80 || len(s.Reading) > 80 {
					return fail(spellingError("invalid_unit", "", "spelling.units", "Use bounded flat sign references."))
				}
				for k, v := range s.Context {
					if len(k) > 80 || len(v) > 160 {
						return fail(spellingError("invalid_unit", "", "spelling.units", "Context atom exceeds 80/160 bytes."))
					}
				}
			}
		}
	}
	if q.Policy != "all" {
		return fail(spellingError("invalid_policy", "", "policy", "Sequence operations require all; no seeded or implicit choice is supported."))
	}
	if q.Template != "" || q.Construction != "" || len(q.Roles) > 0 || q.Seed != "" {
		return fail(spellingError("invalid_input", "", "request", "Remove fields unrelated to sequence operations; nothing is silently ignored."))
	}
	if q.Spelling != nil && q.Operation != "graphemes" && q.Spelling.Selection != nil {
		return fail(spellingError("invalid_input", "", "selection", "Cluster selection belongs to the graphemes operation."))
	}
	if q.Operation == "graphemes" {
		if q.Lexeme != "" || len(q.Segments) > 0 || len(q.Features) > 0 || q.Spelling != nil && (len(q.Spelling.Units) > 0 || q.Spelling.Analyze || q.Spelling.Choice != "") {
			return fail(spellingError("invalid_input", "", "request", "Grapheme processing takes exact text and optional cluster selection only."))
		}
		var err error
		r.Clusters, err = SegmentGraphemes(ctx, q.Text)
		if err != nil {
			return fail(err)
		}
		if q.Spelling != nil && q.Spelling.Selection != nil {
			s := q.Spelling.Selection
			if s.Start < 0 || s.End < s.Start || s.End > len(r.Clusters) {
				return fail(spellingError("invalid_selection", "", "selection", "Selection uses valid grapheme-cluster boundaries, not bytes/code points."))
			}
			r.Selection = Clone(s)
		}
		out.Status = "success"
		return out
	}
	w := a.definition.SequenceWriting
	if !a.report.Ready[q.Operation] || w == nil || a.spellingScript == nil {
		code := "missing_dependency"
		if q.Operation == "unspell" && a.report.Ready["spell"] {
			code = "unsupported_inverse"
		}
		return fail(spellingError(code, "", "sequenceWriting", "Compile compatible source and pinned dependencies; lossy inverse stages are not executable."))
	}
	r.InputLayer = w.InputLayer
	r.OutputLayer = w.Stages[len(w.Stages)-1].OutputLayer
	r.Script = w.Script
	r.Renderer = w.Renderer
	r.RendererAdapter = w.RendererAdapter
	r.RendererCompiler = w.RendererCompiler
	if q.Operation == "unspell" {
		if q.Lexeme != "" || len(q.Segments) > 0 || len(q.Features) > 0 {
			return fail(spellingError("invalid_input", "", "request", "Inverse accepts written text or signs; lexical analysis follows it explicitly."))
		}
		return a.unspell(b, q, out, fail)
	}
	if q.Text != "" {
		return fail(spellingError("invalid_input", "", "text", "Forward spelling uses source units or a lexeme, not an undeclared text layer."))
	}
	if q.Spelling != nil && q.Spelling.Choice != "" || q.Lexeme == "" && len(q.Features) > 0 {
		return fail(spellingError("invalid_input", "", "request", "Forward selection is explicit through the lexical form; inverse choices and unused feature bundles are not applied."))
	}
	if q.Spelling != nil && q.Spelling.Analyze {
		return fail(spellingError("invalid_input", "", "spelling.analyze", "Morphological analysis follows inverse spelling only."))
	}
	inputs := [][]OrthoUnit{}
	forms := []spike.Candidate{}
	derivations := []ProductiveDerivation{}
	if q.Lexeme != "" {
		if len(q.Segments) > 0 || q.Spelling != nil && len(q.Spelling.Units) > 0 {
			return fail(spellingError("invalid_input", "", "input", "Choose a composed lexeme or explicit source units, not both."))
		}
		inner := Clone(q)
		inner.Operation = "inflect"
		inner.Spelling = nil
		inner.Limit = 256
		composed := a.Execute(ctx, inner)
		if composed.Status != "success" && composed.Status != "ambiguous" {
			return fail(spellingError(composed.Status, "", "composition", "Productive input is unavailable or incomplete."))
		}
		forms = composed.Candidates
		derivations = composed.Derivations
		for i, c := range forms {
			inputs = append(inputs, derivationUnits(c, derivations[i], w.InputLayer))
		}
	} else {
		input := []OrthoUnit{}
		if q.Spelling != nil {
			input = Clone(q.Spelling.Units)
		}
		if input == nil {
			input = []OrthoUnit{}
		}
		if len(input) > 0 && len(q.Segments) > 0 {
			return fail(spellingError("invalid_input", "", "input", "Choose source units or segment IDs, not both."))
		}
		for _, s := range q.Segments {
			input = append(input, OrthoUnit{Kind: "segment", Value: s})
		}
		for _, u := range input {
			if u.Sign != nil || u.Kind != "segment" && !(w.InputLayer == "surface-boundaries" && u.Kind == "boundary" && u.Value == "morpheme") {
				return fail(spellingError("invalid_unit", "", "input", "Input must belong to the declared post-phonological layer."))
			}
			if u.Kind == "segment" {
				found := false
				if a.definition.Phonology != nil {
					for _, segment := range a.definition.Phonology.Segments {
						if segment.ID == u.Value {
							found = true
						}
					}
				}
				if !found {
					return fail(spellingError("unknown_segment", "", "input", "Reference a declared phonological segment."))
				}
			}
		}
		inputs = append(inputs, input)
	}
	if len(inputs) > 128 || len(inputs) > q.Limit {
		return fail(spellingError("candidate_limit", "", "input", "All composed alternatives must fit the requested result bound; no partial spelling is returned."))
	}
	for i, input := range inputs {
		run, err := a.spellRun(b, input)
		if err != nil {
			return fail(err)
		}
		if len(forms) > 0 {
			run.Form = &forms[i]
			run.Derivation = &derivations[i]
			run.ID = Hash(struct{ Revision, Form string }{a.report.Revision, Hash(derivations[i])})
		}
		r.Spellings = append(r.Spellings, run)
	}
	out.KnownValid = len(r.Spellings)
	out.Status = "success"
	if len(r.Spellings) > 1 {
		out.Status = "ambiguous"
	}
	if len(Canonical(out)) > 8<<20 {
		return fail(spellingError("execution_limit", "", "result", "Result exceeds 8 MiB."))
	}
	return out
}
