package languages

import (
	"context"
	"testing"
)

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
