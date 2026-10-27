package languages

import (
	"context"
	"strings"
	"testing"
)

type sequenceCancelContext struct {
	context.Context
	remaining int
}

func (c *sequenceCancelContext) Err() error {
	c.remaining--
	if c.remaining <= 0 {
		return context.Canceled
	}
	return nil
}

func TestSequenceBoundaryInverseAndInnerCancellationAndTraceLimit(t *testing.T) {
	d, s := sequenceFixture(t)
	d.SequenceWriting.Stages = d.SequenceWriting.Stages[:1]
	d.SequenceWriting.InputLayer = "surface-boundaries"
	d.SequenceWriting.Stages[0].InputLayer = "surface-boundaries"
	d.SequenceWriting.Stages[0].Rules = append(d.SequenceWriting.Stages[0].Rules, SpellingRule{ID: "visible-boundary", Input: []OrthoUnit{{Kind: "boundary", Value: "morpheme"}}, Output: []OrthoUnit{{Kind: "grapheme", Value: "="}}})
	a := sequenceArtifact(t, d, s)
	q := sequenceQuery(a, "spell")
	q.Lexeme = "stone"
	q.Features = Features{"number": "many", "shape": "plain"}
	r := a.Execute(context.Background(), q)
	if r.Status != "success" {
		t.Fatal(r)
	}
	text := r.Spelling.Spellings[0].Text
	q = sequenceQuery(a, "unspell")
	q.Text = text
	q.Spelling = &SpellingRequest{Analyze: true}
	r = a.Execute(context.Background(), q)
	if r.Status != "ambiguous" || r.KnownValid != 2 || r.Spelling.Preimages[0].Units[3].Kind != "boundary" {
		t.Fatal("A2 boundary stream was not reproduced", r)
	}
	q = sequenceQuery(a, "spell")
	q.Segments = []string{"p", "a", "d", "a"}
	ctx := &sequenceCancelContext{Context: context.Background(), remaining: 35}
	r = a.Execute(ctx, q)
	if r.Status != "cancelled" || len(r.Spelling.Spellings) > 0 {
		t.Fatal("inner matching ignored cancellation", r.Status)
	}
	d, s = sequenceFixture(t)
	d.SequenceWriting.Stages = []SpellingStage{}
	d.SequenceWriting.Inverse = "unsupported"
	for i := 0; i < 8; i++ {
		layer := "grapheme"
		kind := "grapheme"
		if i == 0 {
			layer = "surface"
			kind = "segment"
		}
		id := string(rune('a'+i)) + strings.Repeat("r", 79)
		d.SequenceWriting.Stages = append(d.SequenceWriting.Stages, SpellingStage{ID: string(rune('a'+i)) + "-stage", Kind: "rewrite", InputLayer: layer, OutputLayer: "grapheme", Unmatched: "reject", Rules: []SpellingRule{{ID: id, Input: []OrthoUnit{{Kind: kind, Value: "p"}}, Output: []OrthoUnit{{Kind: "grapheme", Value: "p"}}}}})
	}
	a = sequenceArtifact(t, d, s)
	q = sequenceQuery(a, "spell")
	q.Spelling = &SpellingRequest{Units: []OrthoUnit{}}
	for i := 0; i < 512; i++ {
		q.Spelling.Units = append(q.Spelling.Units, OrthoUnit{Kind: "segment", Value: "p"})
	}
	r = a.Execute(context.Background(), q)
	if r.Status != "execution_limit" || len(r.Spelling.Spellings) > 0 || r.Diagnostics[0].Path != "trace" {
		t.Fatal("trace limit returned partial or wrong status", r.Status, r.Diagnostics)
	}
}

func TestSequenceInverseExhaustiveFiniteDomainAndClusterMerging(t *testing.T) {
	d, s := sequenceFixture(t)
	d.SequenceWriting.Stages = d.SequenceWriting.Stages[:1]
	a := sequenceArtifact(t, d, s)
	// Every stream through length three over the entire authored inventory,
	// including the empty stream, supplies the independent forward oracle.
	domain := [][]string{{}}
	level := [][]string{{}}
	for n := 0; n < 3; n++ {
		next := [][]string{}
		for _, p := range level {
			for _, segment := range d.Phonology.Segments {
				v := append(append([]string{}, p...), segment.ID)
				next = append(next, v)
				domain = append(domain, v)
			}
		}
		level = next
	}
	expected := map[string]map[string]bool{}
	for _, segments := range domain {
		q := sequenceQuery(a, "spell")
		q.Segments = segments
		r := a.Execute(context.Background(), q)
		if r.Status != "success" {
			t.Fatal(r.Status)
		}
		run := r.Spelling.Spellings[0]
		if expected[run.Text] == nil {
			expected[run.Text] = map[string]bool{}
		}
		expected[run.Text][Hash(run.Input)] = true
	}
	for text, inputs := range expected {
		q := sequenceQuery(a, "unspell")
		q.Text = text
		r := a.Execute(context.Background(), q)
		if r.Status != "success" && r.Status != "ambiguous" {
			t.Fatal(text, r.Status)
		}
		if len(r.Spelling.Preimages) != len(inputs) {
			t.Fatalf("%q: inverse=%d forward=%d", text, len(r.Spelling.Preimages), len(inputs))
		}
		for _, p := range r.Spelling.Preimages {
			if !inputs[Hash(p.Units)] {
				t.Fatal("inverse invented a source sequence", p.Units)
			}
		}
	}
	for i := range d.SequenceWriting.Stages[0].Rules {
		r := &d.SequenceWriting.Stages[0].Rules[i]
		if r.ID == "map-p" {
			r.Output = []OrthoUnit{{Kind: "grapheme", Value: "e"}}
		}
		if r.ID == "map-a" {
			r.Output = []OrthoUnit{{Kind: "grapheme", Value: "\u0301"}}
		}
	}
	a = sequenceArtifact(t, d, s)
	q := sequenceQuery(a, "spell")
	q.Segments = []string{"p", "a"}
	r := a.Execute(context.Background(), q)
	if r.Status != "success" {
		t.Fatal(r)
	}
	run := r.Spelling.Spellings[0]
	if len(run.Output) != 1 || len(run.Output[0].Sources) != 2 || run.Text != "e\u0301" {
		t.Fatal("cross-fragment cluster provenance lost", run)
	}
	q = sequenceQuery(a, "unspell")
	q.Text = run.Text
	r = a.Execute(context.Background(), q)
	found := false
	for _, p := range r.Spelling.Preimages {
		if len(p.Units) == 2 && p.Units[0].Value == "p" && p.Units[1].Value == "a" {
			found = true
		}
	}
	if !found {
		t.Fatal("inverse split/ignored a combining cluster", r)
	}
}
