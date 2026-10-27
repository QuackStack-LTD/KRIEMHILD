package languages

import (
	"context"
	"fmt"
)

func sameUnit(a, b OrthoUnit) bool { return string(Canonical(a)) == string(Canonical(b)) }
func (a *Artifact) compileSpelling(ctx context.Context, scripts map[ScriptRef]ScriptDefinition) []Diagnostic {
	if a.definition.Schema != 4 {
		return nil
	}
	a.report.Ready["graphemes"] = true
	w := a.definition.SequenceWriting
	out := []Diagnostic{}
	bad := func(code, id, path, msg string) {
		if len(out) < 256 {
			out = append(out, Diagnostic{Code: code, Module: "sequenceWriting", Severity: "error", RuleID: id, Path: path, Message: msg})
		}
	}
	if w == nil {
		bad("missing_dependency", "", "sequenceWriting", "Author sequenceWriting before enabling spelling.")
		return out
	}
	if err := spellingShape(w); err != nil {
		bad("definition_limit", "", "sequenceWriting", err.Error())
		return out
	}
	if w.Version != SequenceVersion || w.Segmentation != GraphemeVersion || w.Normalizer != NormalizationVersion {
		bad("incompatible_engine", "", "sequenceWriting", "Pin sequence-orthography/1, uniseg/0.4.7-unicode/15.0.0 and x-text/0.29.0-unicode/15.0.0.")
	}
	if w.InputLayer != "surface" && w.InputLayer != "surface-boundaries" {
		bad("unsupported_layer", "", "sequenceWriting.inputLayer", "Choose post-phonological surface or surface-boundaries.")
	}
	if w.Inverse != "all" && w.Inverse != "unsupported" {
		bad("invalid_policy", "", "sequenceWriting.inverse", "Choose all or unsupported; no inverse policy is inferred.")
	}
	script, ok := scripts[w.Script]
	if !ok || script.ScriptID != w.Script.ID || Hash(script) != w.Script.Revision || !CompatibleScriptVersions(script.Schema, w.Renderer, w.RendererAdapter, w.RendererCompiler) {
		bad("missing_dependency", "", "sequenceWriting.script", "Pin a retained Script 2 revision and exact renderer versions.")
	} else {
		r := CompileScript(ctx, script)
		if !r.Ready["render"] {
			bad("missing_dependency", "", "sequenceWriting.script", "Correct the pinned Script glyphs/readings/layout before spelling.")
			out = append(out, r.Diagnostics...)
		} else {
			a.spellingScript = &script
		}
	}
	a.report.Dependencies = ScriptDependencies(a.definition)
	seen := map[string]bool{}
	budget := &spellingBudget{ctx: ctx}
	id := func(value, path string) {
		if value == "" || len(value) > 80 || seen[value] {
			bad("invalid_id", value, path, "Use unique stable nonempty stage/rule IDs of at most 80 bytes.")
		}
		seen[value] = true
	}
	segments := map[string]bool{}
	if a.definition.Phonology != nil {
		for _, s := range a.definition.Phonology.Segments {
			segments[s.ID] = true
		}
	}
	unit := func(u OrthoUnit, layer, id, path string, context bool) {
		if u.Kind == "edge" && context && u.Value == "word" && u.Sign == nil {
			return
		}
		valid := false
		switch layer {
		case "surface", "surface-boundaries":
			valid = u.Kind == "segment" && segments[u.Value] || layer == "surface-boundaries" && u.Kind == "boundary" && u.Value == "morpheme"
		case "grapheme":
			if u.Kind == "grapheme" && u.Value != "" {
				g, e := SegmentGraphemes(ctx, u.Value)
				valid = e == nil && len(g) == 1
			}
		case "sign":
			valid = u.Kind == "sign" && u.Value == "" && u.Sign != nil && u.Sign.Kind == "sign"
		}
		if u.Kind != "sign" && u.Sign != nil {
			valid = false
		}
		if !valid {
			bad("invalid_unit", id, path, "Use exact typed units belonging to the declared layer; graphemes must each be one extended cluster.")
		}
		if valid && u.Kind == "sign" && a.spellingScript != nil {
			q := ScriptRenderRequest{Revision: w.Script.Revision, Engine: w.Renderer, Adapter: w.RendererAdapter, Compiler: w.RendererCompiler, Sequence: &SignSequence{ID: "validate-sign", Items: []SignUse{*u.Sign}}}
			r := RenderScript(ctx, *a.spellingScript, q)
			if r.Status != "success" {
				bad(r.Status, id, path, "Choose existing sign/glyph IDs, explicit roles and a context-compatible reading: "+r.Status)
			}
		}
	}
	layer := w.InputLayer
	invertible := w.Inverse == "all"
	if len(w.Stages) == 0 {
		bad("missing_dependency", "", "sequenceWriting.stages", "Author at least one spelling stage.")
	}
	for si, s := range w.Stages {
		if ctx.Err() != nil {
			bad("cancelled", s.ID, "sequenceWriting", ctx.Err().Error())
			break
		}
		path := fmt.Sprintf("sequenceWriting.stages.%d", si)
		id(s.ID, path)
		if s.InputLayer != layer {
			bad("incompatible_layer", s.ID, path, "Each stage must consume the preceding stage's output layer.")
		}
		if s.Kind == "normalize" {
			if layer != "grapheme" || s.OutputLayer != "grapheme" || len(s.Rules) > 0 || s.Unmatched != "" || (s.Normalization != "NFC" && s.Normalization != "NFD") {
				bad("invalid_normalization", s.ID, path, "A normalization stage consumes/emits graphemes with explicit NFC/NFD and no rules/unmatched policy.")
			}
			invertible = false
			layer = s.OutputLayer
			continue
		}
		if s.Kind != "rewrite" || s.Normalization != "" {
			bad("unsupported_rule", s.ID, path, "Only finite snapshot rewrite and explicit normalize stages execute.")
		}
		if !((layer == "surface" || layer == "surface-boundaries") && s.OutputLayer == "grapheme" || layer == "grapheme" && (s.OutputLayer == "grapheme" || s.OutputLayer == "sign")) {
			bad("unsupported_layer", s.ID, path, "Use surface-to-grapheme, grapheme-to-grapheme or final grapheme-to-sign stages.")
		}
		if s.Unmatched != "reject" && (s.Unmatched != "identity" || s.InputLayer != s.OutputLayer) {
			bad("invalid_policy", s.ID, path, "Unmatched units require reject or same-layer identity.")
		}
		for ri, r := range s.Rules {
			if err := budget.use(1, 0); err != nil {
				bad("cancelled", r.ID, path, err.Error())
				return out
			}
			id(r.ID, path+".rules")
			if r.Priority < -100000 || r.Priority > 100000 {
				bad("invalid_priority", r.ID, path, "Priority must be an integer within ±100000.")
			}
			if len(r.Input) == 0 {
				bad("empty_pattern", r.ID, path, "Empty-input rules are unsupported; author a finite consuming pattern.")
			}
			if len(r.Output) == 0 {
				invertible = false
			}
			for _, u := range r.Input {
				unit(u, layer, r.ID, path+".input", false)
			}
			for _, u := range append(append([]OrthoUnit{}, r.Left...), r.Right...) {
				unit(u, layer, r.ID, path+".context", true)
			}
			for _, u := range r.Output {
				unit(u, s.OutputLayer, r.ID, path+".output", false)
			}
			for _, prev := range s.Rules[:ri] {
				if err := budget.use(1, 0); err != nil {
					code := "execution_limit"
					if ctx.Err() != nil {
						code = "cancelled"
					}
					bad(code, r.ID, path, err.Error())
					return out
				}
				if prev.Priority != r.Priority {
					continue
				}
				overlap := len(r.Input) > 0 && len(prev.Input) > 0
				for j := 0; j < len(r.Input) && j < len(prev.Input); j++ {
					if !sameUnit(r.Input[j], prev.Input[j]) {
						overlap = false
						break
					}
				}
				if overlap {
					bad("rule_conflict", r.ID, path, "Equal-priority overlapping patterns require different explicit priorities (including context-dependent rules).")
				}
			}
		}
		layer = s.OutputLayer
	}
	if layer != "grapheme" && layer != "sign" {
		bad("unsupported_layer", "", "sequenceWriting", "Final output must be graphemes or signs.")
	}
	if len(out) == 0 {
		a.report.Ready["spell"] = true
		a.report.Ready["unspell"] = invertible
	}
	if !invertible {
		out = append(out, Diagnostic{Code: "unsupported_inverse", Severity: "warning", Module: "sequenceWriting", Path: "sequenceWriting.inverse", Message: "Inverse is disabled by authored policy, deletion or normalization. Forward spelling remains independent."})
	}
	return out
}
