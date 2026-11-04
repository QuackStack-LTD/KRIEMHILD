package languages

import "fmt"

const SequenceEngine = "languages/4"
const SequenceAdapter = "sequence-compose/1"
const SequenceCompiler = "languages-compiler/4"
const SequenceVersion = "sequence-orthography/1"
const GraphemeVersion = "uniseg/0.4.7-unicode/15.0.0"
const NormalizationVersion = "x-text/0.29.0-unicode/15.0.0"

type OrthoUnit struct {
	Kind  string   `json:"kind"`
	Value string   `json:"value,omitempty"`
	Sign  *SignUse `json:"sign,omitempty"`
}
type SpellingRule struct {
	ID       string      `json:"id"`
	Priority int         `json:"priority"`
	Input    []OrthoUnit `json:"input"`
	Left     []OrthoUnit `json:"left"`
	Right    []OrthoUnit `json:"right"`
	Output   []OrthoUnit `json:"output"`
}
type SpellingStage struct {
	ID            string         `json:"id"`
	Kind          string         `json:"kind"`
	InputLayer    string         `json:"inputLayer"`
	OutputLayer   string         `json:"outputLayer"`
	Unmatched     string         `json:"unmatched"`
	Normalization string         `json:"normalization"`
	Rules         []SpellingRule `json:"rules"`
}
type SequenceWriting struct {
	Version          string          `json:"version"`
	InputLayer       string          `json:"inputLayer"`
	Script           ScriptRef       `json:"script"`
	Renderer         string          `json:"renderer"`
	RendererAdapter  string          `json:"rendererAdapter"`
	RendererCompiler string          `json:"rendererCompiler"`
	Segmentation     string          `json:"segmentation"`
	Normalizer       string          `json:"normalizer"`
	Inverse          string          `json:"inverse"`
	Stages           []SpellingStage `json:"stages"`
}
type ClusterSelection struct {
	Start int `json:"start"`
	End   int `json:"end"`
}
type SpellingRequest struct {
	Units     []OrthoUnit       `json:"units,omitempty"`
	Analyze   bool              `json:"analyze,omitempty"`
	Choice    string            `json:"choice,omitempty"`
	Selection *ClusterSelection `json:"selection,omitempty"`
}

func ScriptDependencies(d Definition) []ScriptRef {
	out := []ScriptRef{}
	if d.Orthography != nil {
		out = append(out, d.Orthography.Script)
	}
	if d.SequenceWriting != nil {
		ref := d.SequenceWriting.Script
		if len(out) == 0 || out[0] != ref {
			out = append(out, ref)
		}
	}
	return out
}
func spellingShape(w *SequenceWriting) error {
	if w == nil {
		return nil
	}
	if len(w.Stages) > 8 {
		return fmt.Errorf("sequenceWriting: at most 8 stages")
	}
	rules := 0
	for _, s := range w.Stages {
		rules += len(s.Rules)
		for _, r := range s.Rules {
			if len(r.Input) > 16 || len(r.Output) > 16 || len(r.Left) > 4 || len(r.Right) > 4 {
				return fmt.Errorf("sequenceWriting: rule pattern/output/context exceeds 16/16/4/4 units")
			}
			for _, list := range [][]OrthoUnit{r.Input, r.Output, r.Left, r.Right} {
				for _, u := range list {
					if len(u.Value) > 4096 {
						return fmt.Errorf("sequenceWriting: unit exceeds 4 KiB")
					}
					if u.Sign != nil {
						if len(u.Sign.Children) > 0 || len(u.Sign.Context) > 32 || len(u.Sign.Roles) > 6 {
							return fmt.Errorf("sequenceWriting: signs must be bounded flat references")
						}
					}
				}
			}
		}
	}
	if rules > 128 {
		return fmt.Errorf("sequenceWriting: at most 128 rules")
	}
	return nil
}
