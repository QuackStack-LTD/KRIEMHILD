package languages

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func sequenceFixture(t testing.TB) (Definition, ScriptDefinition) {
	t.Helper()
	raw, e := os.ReadFile("testdata/sequence.json")
	if e != nil {
		t.Fatal(e)
	}
	d, e := Decode(raw)
	if e != nil {
		t.Fatal(e)
	}
	raw, e = os.ReadFile("testdata/sequence-script.json")
	if e != nil {
		t.Fatal(e)
	}
	s, e := DecodeScript(raw)
	if e != nil {
		t.Fatal(e)
	}
	if d.SequenceWriting.Script.Revision != Hash(s) {
		t.Fatal("fixture script pin differs", Hash(s))
	}
	return d, s
}
func sequenceArtifact(t testing.TB, d Definition, s ScriptDefinition) *Artifact {
	t.Helper()
	return Compile(context.Background(), d, map[ScriptRef]ScriptDefinition{{s.ScriptID, Hash(s)}: s})
}
func sequenceQuery(a *Artifact, op string) Request {
	q := productiveRequest(a, op)
	q.Limit = 128
	return q
}
func TestSequenceForwardProvenanceGoldens(t *testing.T) {
	d, s := sequenceFixture(t)
	a := sequenceArtifact(t, d, s)
	if !a.report.Ready["spell"] || !a.report.Ready["unspell"] {
		t.Fatal(a.report)
	}
	for _, name := range []string{"sequence-forward", "sequence-inverse"} {
		raw, e := os.ReadFile("testdata/" + name + ".request.json")
		if e != nil {
			t.Fatal(e)
		}
		var q Request
		if e = json.Unmarshal(raw, &q); e != nil {
			t.Fatal(e)
		}
		r := a.Execute(context.Background(), q)
		raw, e = os.ReadFile("testdata/" + name + ".golden.json")
		if e != nil {
			t.Fatal(e)
		}
		var expected Result
		if e = json.Unmarshal(raw, &expected); e != nil {
			t.Fatal(e)
		}
		if string(Canonical(r)) != string(Canonical(expected)) {
			t.Fatalf("%s complete reviewed golden differs: status=%s steps=%d", name, r.Status, r.Spelling.Steps)
		}
		fresh := sequenceArtifact(t, Clone(d), Clone(s)).Execute(context.Background(), q)
		if string(Canonical(r)) != string(Canonical(fresh)) {
			t.Fatal("rebuild changed bytes")
		}
	}
	q := sequenceQuery(a, "spell")
	q.Segments = []string{"n", "i", "n"}
	r := a.Execute(context.Background(), q)
	if r.Status != "success" || len(r.Spelling.Spellings[0].Output) != 2 || r.Spelling.Spellings[0].Render.Signs[1].Roles[0] != "classifier" {
		t.Fatal("logogram/classifier path missing", r)
	}
	q.Segments = nil
	q.Lexeme = "stone"
	q.Features = Features{"number": "many", "shape": "plain"}
	r = a.Execute(context.Background(), q)
	run := r.Spelling.Spellings[0]
	if run.Text != "pe\u0301de\u0301" || len(run.Clusters) != 4 || run.Clusters[1].UTF16End != 3 || run.Output[1].Sources[0] != 1 || run.Output[1].Rules[0] != "map-a" || run.Output[1].Rules[1] != "sign-acute" || run.Derivation.Steps[4].Before[3].Boundary != "morpheme" {
		t.Fatal("representation/provenance layers conflated", run)
	}
}
func TestSequenceValidationCompatibilityAndReadiness(t *testing.T) {
	d, s := sequenceFixture(t)
	for _, schema := range []int{1, 2, 3} {
		old := Clone(d)
		old.Schema = schema
		old.SequenceWriting = nil
		if schema == 1 {
			old.Productive = nil
		}
		var body map[string]any
		json.Unmarshal(Canonical(old), &body)
		body["sequenceWriting"] = nil
		if _, err := Decode(Canonical(body)); err == nil {
			t.Fatal("old source silently accepted spelling", schema)
		}
	}
	for name, mutate := range map[string]func(*Definition){
		"empty": func(d *Definition) { d.SequenceWriting.Stages[0].Rules[0].Input = nil },
		"tie": func(d *Definition) {
			r := Clone(d.SequenceWriting.Stages[0].Rules[0])
			r.ID = "conflict"
			d.SequenceWriting.Stages[0].Rules = append(d.SequenceWriting.Stages[0].Rules, r)
		},
		"layer":    func(d *Definition) { d.SequenceWriting.Stages[1].InputLayer = "surface" },
		"unicode":  func(d *Definition) { d.SequenceWriting.Segmentation = "latest" },
		"identity": func(d *Definition) { d.SequenceWriting.Stages[0].Unmatched = "identity" },
		"glyph":    func(d *Definition) { d.SequenceWriting.Stages[1].Rules[0].Output[0].Sign.Sign = "missing" },
		"unit":     func(d *Definition) { d.SequenceWriting.Stages[0].Rules[0].Output[0].Value = "two clusters" },
		"pin":      func(d *Definition) { d.SequenceWriting.Renderer = "latest" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := Clone(d)
			mutate(&bad)
			a := sequenceArtifact(t, bad, s)
			if a.report.Ready["spell"] || !a.report.Ready["inflect"] || len(a.report.Diagnostics) == 0 {
				t.Fatal("partial readiness dishonest", a.report)
			}
		})
	}
	noScript := Compile(context.Background(), d, nil)
	if noScript.report.Ready["spell"] || !noScript.report.Ready["graphemes"] || !noScript.report.Ready["analyze"] {
		t.Fatal(noScript.report)
	}
	if ArtifactKey("w", "a", d) == ArtifactKey("w", "b", d) {
		t.Fatal("Age omitted from cache key")
	}
}
func TestSequenceBoundariesNormalizationAndSnapshotOrder(t *testing.T) {
	d, s := sequenceFixture(t)
	d.SequenceWriting.InputLayer = "surface-boundaries"
	d.SequenceWriting.Stages = d.SequenceWriting.Stages[:1]
	d.SequenceWriting.Stages[0].InputLayer = "surface-boundaries"
	d.SequenceWriting.Stages[0].Rules = append(d.SequenceWriting.Stages[0].Rules, SpellingRule{ID: "erase-morpheme", Input: []OrthoUnit{{Kind: "boundary", Value: "morpheme"}}, Output: []OrthoUnit{}})
	a := sequenceArtifact(t, d, s)
	if !a.report.Ready["spell"] || a.report.Ready["unspell"] {
		t.Fatal(a.report)
	}
	q := sequenceQuery(a, "spell")
	q.Lexeme = "stone"
	q.Features = Features{"number": "many", "shape": "plain"}
	r := a.Execute(context.Background(), q)
	if r.Status != "success" || len(r.Spelling.Spellings[0].Input) != 5 || r.Spelling.Spellings[0].Stages[0].Matches[3].Action != "deleted" {
		t.Fatal("boundaries not explicit", r)
	}
	d.SequenceWriting.Stages = append(d.SequenceWriting.Stages, SpellingStage{ID: "explicit-nfc", Kind: "normalize", InputLayer: "grapheme", OutputLayer: "grapheme", Normalization: "NFC"})
	a = sequenceArtifact(t, d, s)
	q = sequenceQuery(a, "spell")
	q.Segments = []string{"a"}
	r = a.Execute(context.Background(), q)
	run := r.Spelling.Spellings[0]
	if run.Text != "\u00e9" || run.Stages[1].Original != "e\u0301" || run.Stages[1].Normalized != "\u00e9" || a.report.Ready["unspell"] {
		t.Fatal("normalization hidden or inverse guessed", r)
	}
	q = sequenceQuery(a, "unspell")
	if r = a.Execute(context.Background(), q); r.Status != "unsupported_inverse" {
		t.Fatal(r.Status)
	}
	d, s = sequenceFixture(t)
	d.SequenceWriting.Stages = d.SequenceWriting.Stages[:1]
	for i, pair := range [][2]string{{"p", "b"}, {"b", "p"}} {
		d.SequenceWriting.Stages = append(d.SequenceWriting.Stages, SpellingStage{ID: []string{"p-to-b", "b-to-p"}[i], Kind: "rewrite", InputLayer: "grapheme", OutputLayer: "grapheme", Unmatched: "identity", Rules: []SpellingRule{{ID: []string{"to-b", "to-p"}[i], Input: []OrthoUnit{{Kind: "grapheme", Value: pair[0]}}, Output: []OrthoUnit{{Kind: "grapheme", Value: pair[1]}}}}})
	}
	a = sequenceArtifact(t, d, s)
	q = sequenceQuery(a, "spell")
	q.Segments = []string{"p"}
	r = a.Execute(context.Background(), q)
	if r.Status != "success" || r.Spelling.Spellings[0].Text != "p" || len(r.Spelling.Spellings[0].Stages) != 3 {
		t.Fatal("finite cycles did not execute once", r)
	}
}
func TestSequenceInverseContextsOverlapsEpsilonAndLimits(t *testing.T) {
	d, s := sequenceFixture(t)
	d.SequenceWriting.Stages = d.SequenceWriting.Stages[:1]
	// Context/priorities choose the voiced mapping only at a word edge.
	rule := Clone(d.SequenceWriting.Stages[0].Rules[0])
	rule.ID = "priority-edge"
	rule.Priority = 10
	rule.Left = []OrthoUnit{{Kind: "edge", Value: "word"}}
	rule.Output = []OrthoUnit{{Kind: "grapheme", Value: "X"}}
	d.SequenceWriting.Stages[0].Rules = append(d.SequenceWriting.Stages[0].Rules, rule)
	a := sequenceArtifact(t, d, s)
	q := sequenceQuery(a, "unspell")
	q.Text = "X"
	r := a.Execute(context.Background(), q)
	if r.Status != "success" || len(r.Spelling.Preimages) != 1 {
		t.Fatal("context inverse failed", r)
	}
	q.Text = "de\u0301"
	r = a.Execute(context.Background(), q)
	if r.Status != "ambiguous" || len(r.Spelling.Preimages) != 2 {
		t.Fatal("many-to-one collapsed", r)
	}
	q.Spelling = &SpellingRequest{Choice: r.Spelling.Preimages[1].ID}
	chosen := a.Execute(context.Background(), q)
	if len(chosen.Spelling.Preimages) != 2 || chosen.Spelling.Choice != q.Spelling.Choice {
		t.Fatal("explicit choice discarded alternatives")
	}
	q.Text = "unmapped"
	q.Spelling = nil
	r = a.Execute(context.Background(), q)
	if r.Status != "no_result" {
		t.Fatal(r.Status)
	}
	q.Text = ""
	r = a.Execute(context.Background(), q)
	if r.Status != "success" || len(r.Spelling.Preimages) != 1 || len(r.Spelling.Preimages[0].Units) != 0 {
		t.Fatal("empty relation lost", r)
	}
	q.Text = strings.Repeat("d", 12)
	r = a.Execute(context.Background(), q)
	if r.Status != "execution_limit" || len(r.Spelling.Preimages) != 0 {
		t.Fatal("high ambiguity returned partial spelling", r.Status)
	}
	q.Text = "d"
	q.Limit = 1
	r = a.Execute(context.Background(), q)
	if r.Status != "candidate_limit" || len(r.Spelling.Preimages) != 0 {
		t.Fatal("display cap silently lost ambiguity", r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	q.Limit = 128
	r = a.Execute(ctx, q)
	if r.Status != "cancelled" {
		t.Fatal(r.Status)
	}
	q = sequenceQuery(a, "spell")
	q.Spelling = &SpellingRequest{Units: make([]OrthoUnit, 513)}
	r = a.Execute(context.Background(), q)
	if r.Status != "execution_limit" {
		t.Fatal(r.Status)
	}
	q = sequenceQuery(a, "graphemes")
	q.Text = "e\u0301"
	q.Spelling = &SpellingRequest{Selection: &ClusterSelection{Start: 0, End: 2}}
	r = a.Execute(context.Background(), q)
	if r.Status != "invalid_selection" {
		t.Fatal(r.Status)
	}
}
