package languages

import (
	"kriemhild/internal/languages/spike"
	"strings"
)

type inverseSpellingPath struct {
	position int
	units    []OrthoUnit
}

func unitText(units []OrthoUnit) string {
	var b strings.Builder
	for _, u := range units {
		b.WriteString(u.Value)
	}
	return b.String()
}
func unitsEqual(a, b []OrthoUnit) bool { return string(Canonical(a)) == string(Canonical(b)) }

// Reverse transitions are derived only from canonical forward rules. UTF-8 byte
// prefix traversal is internal: adjacent authored fragments may form a cluster.
// Public rule spans and accepted preimages are replayed on whole typed clusters.
func inverseStage(b *spellingBudget, stage SpellingStage, observed []OrthoUnit) ([][]OrthoUnit, error) {
	textual := stage.OutputLayer == "grapheme"
	text := unitText(observed)
	end := len(observed)
	if textual {
		end = len(text)
	}
	pending := []inverseSpellingPath{{0, []OrthoUnit{}}}
	valid := [][]OrthoUnit{}
	seen := map[string]bool{}
	for len(pending) > 0 {
		if err := b.use(1, 0); err != nil {
			return nil, err
		}
		path := pending[0]
		pending = pending[1:]
		if path.position == end {
			input := path.units
			if stage.InputLayer == "grapheme" {
				v, _, err := resegment(b, annotate(input))
				if err != nil {
					return nil, err
				}
				input = valuesUnits(v)
			}
			trace, err := rewriteStage(b, stage, annotate(input))
			if err != nil {
				if f, ok := err.(*productiveFailure); ok && (f.diagnostic.Code == "cancelled" || f.diagnostic.Code == "execution_limit") {
					return nil, err
				}
				continue
			}
			if !unitsEqual(valuesUnits(trace.After), observed) {
				continue
			}
			key := Hash(input)
			if !seen[key] {
				seen[key] = true
				valid = append(valid, input)
			}
			if len(valid) > 128 {
				return nil, spellingError("execution_limit", stage.ID, "inverse", "Inverse exceeds 128 valid preimages; no partial set is returned.")
			}
			continue
		}
		appendPath := func(input []OrthoUnit, consume int) error {
			u := append(append([]OrthoUnit{}, path.units...), input...)
			if len(u) > 512 || len(pending) >= 128 {
				return spellingError("execution_limit", stage.ID, "inverse", "Inverse exceeds 512 units or 128 live segmentation paths.")
			}
			if err := b.use(1, len(Canonical(u))); err != nil {
				return err
			}
			pending = append(pending, inverseSpellingPath{path.position + consume, u})
			return nil
		}
		for _, rule := range stage.Rules {
			if err := b.use(1, 0); err != nil {
				return nil, err
			}
			consume := len(rule.Output)
			if textual {
				piece := unitText(rule.Output)
				consume = len(piece)
				if consume == 0 || !strings.HasPrefix(text[path.position:], piece) {
					continue
				}
			} else {
				if consume == 0 || path.position+consume > end || !unitsEqual(rule.Output, observed[path.position:path.position+consume]) {
					continue
				}
			}
			if err := appendPath(rule.Input, consume); err != nil {
				return nil, err
			}
		}
		if stage.Unmatched == "identity" {
			if textual {
				clusters, err := SegmentGraphemes(b.ctx, text[path.position:])
				if err != nil {
					return nil, err
				}
				if len(clusters) > 0 {
					if err = appendPath([]OrthoUnit{{Kind: "grapheme", Value: clusters[0].Text}}, clusters[0].ByteEnd); err != nil {
						return nil, err
					}
				}
			} else {
				if err := appendPath(observed[path.position:path.position+1], 1); err != nil {
					return nil, err
				}
			}
		}
	}
	return valid, nil
}
func (a *Artifact) unspell(b *spellingBudget, q Request, out Result, fail func(error) Result) Result {
	r := out.Spelling
	w := a.definition.SequenceWriting
	input := []OrthoUnit{}
	if q.Spelling != nil {
		input = Clone(q.Spelling.Units)
	}
	if input == nil {
		input = []OrthoUnit{}
	}
	if len(input) > 512 {
		return fail(spellingError("execution_limit", "", "inverse", "Inverse input exceeds 512 units."))
	}
	if q.Text != "" && len(input) > 0 {
		return fail(spellingError("invalid_input", "", "inverse", "Choose exact written text or structured output units, not both."))
	}
	if r.OutputLayer == "grapheme" {
		if q.Text != "" {
			clusters, err := SegmentGraphemes(b.ctx, q.Text)
			if err != nil {
				return fail(err)
			}
			for _, g := range clusters {
				input = append(input, OrthoUnit{Kind: "grapheme", Value: g.Text})
			}
		}
		for _, u := range input {
			if u.Kind != "grapheme" || u.Sign != nil {
				return fail(spellingError("invalid_unit", "", "inverse", "Use exact grapheme units for the final layer."))
			}
		}
		v, _, err := resegment(b, annotate(input))
		if err != nil {
			return fail(err)
		}
		input = valuesUnits(v)
	} else {
		if q.Text != "" {
			return fail(spellingError("invalid_input", "", "inverse", "Sign inverse requires explicit structured signs; no Unicode-to-glyph guess is made."))
		}
		for _, u := range input {
			if u.Kind != "sign" || u.Sign == nil || u.Value != "" {
				return fail(spellingError("invalid_unit", "", "inverse", "Use explicit sign references for the final layer."))
			}
		}
		items := []SignUse{}
		for _, u := range input {
			items = append(items, *u.Sign)
		}
		validation := RenderScript(b.ctx, *a.spellingScript, ScriptRenderRequest{Revision: w.Script.Revision, Engine: w.Renderer, Adapter: w.RendererAdapter, Compiler: w.RendererCompiler, Sequence: &SignSequence{ID: "inverse-input", Items: items}})
		if validation.Status != "success" {
			return fail(spellingError(validation.Status, "", "inverse.signs", "Inverse signs must resolve under the pinned Script, roles, readings and context."))
		}
	}
	preimages := [][]OrthoUnit{input}
	for i := len(w.Stages) - 1; i >= 0; i-- {
		next := [][]OrthoUnit{}
		seen := map[string]bool{}
		for _, observed := range preimages {
			items, err := inverseStage(b, w.Stages[i], observed)
			if err != nil {
				return fail(err)
			}
			for _, units := range items {
				key := Hash(units)
				if !seen[key] {
					seen[key] = true
					next = append(next, units)
				}
				if len(next) > 128 {
					return fail(spellingError("execution_limit", w.Stages[i].ID, "inverse", "Inverse exceeds 128 preimages across stages."))
				}
			}
		}
		preimages = next
	}
	if len(preimages) > q.Limit {
		return fail(spellingError("candidate_limit", "", "inverse", "All valid preimages must fit the requested result bound; no incomplete set is returned."))
	}
	total := 0
	for _, units := range preimages {
		run, err := a.spellRun(b, units)
		if err != nil {
			return fail(err)
		}
		if !unitsEqual(valuesUnits(run.Output), input) {
			continue
		}
		p := SpellingPreimage{ID: run.ID, Units: units, Forward: run, Analyses: []spike.Candidate{}, Derivations: []ProductiveDerivation{}}
		if q.Spelling != nil && q.Spelling.Analyze {
			inner := Clone(q)
			inner.Operation = "analyze"
			inner.Spelling = nil
			inner.Text = ""
			inner.Lexeme = ""
			inner.Segments = []string{}
			inner.Limit = 256
			for _, u := range units {
				if u.Kind == "segment" {
					inner.Segments = append(inner.Segments, u.Value)
				}
			}
			analysis := a.Execute(b.ctx, inner)
			if analysis.Status == "execution_limit" || analysis.Status == "cancelled" || analysis.Status == "candidate_limit" {
				return fail(spellingError(analysis.Status, "", "inverse.morphology", "A2 did not return a complete morphological set."))
			}
			p.MorphologyStatus = analysis.Status
			p.Inverse = analysis.Inverse
			if analysis.Inverse != nil {
				if err = b.use(analysis.Inverse.Steps, 0); err != nil {
					return fail(err)
				}
			}
			for i, c := range analysis.Candidates {
				if w.InputLayer == "surface-boundaries" && !unitsEqual(derivationUnits(c, analysis.Derivations[i], w.InputLayer), units) {
					continue
				}
				p.Analyses = append(p.Analyses, c)
				p.Derivations = append(p.Derivations, analysis.Derivations[i])
			}
			if len(p.Analyses) == 0 {
				continue
			}
			total += len(p.Analyses)
		} else {
			total++
		}
		if total > 128 || total > q.Limit {
			return fail(spellingError("candidate_limit", "", "inverse", "Combined orthographic/morphological analyses exceed the complete-result bound."))
		}
		if err = b.use(1, len(Canonical(p))); err != nil {
			return fail(err)
		}
		r.Preimages = append(r.Preimages, p)
	}
	if q.Spelling != nil && q.Spelling.Choice != "" {
		found := false
		for _, p := range r.Preimages {
			if p.ID == q.Spelling.Choice {
				found = true
			}
		}
		if !found {
			return fail(spellingError("invalid_selection", "", "spelling.choice", "Choose an ID in the complete pinned preimage set."))
		}
		r.Choice = q.Spelling.Choice
	}
	out.KnownValid = total
	out.Status = "success"
	if total == 0 {
		out.Status = "no_result"
	} else if total > 1 {
		out.Status = "ambiguous"
	}
	if len(Canonical(out)) > 8<<20 {
		return fail(spellingError("execution_limit", "", "result", "Result exceeds 8 MiB."))
	}
	return out
}
