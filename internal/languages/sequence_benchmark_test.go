package languages

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkSequenceSpelling(b *testing.B) {
	for _, size := range []struct{ units, rules int }{{32, 16}, {128, 64}, {512, 128}} {
		d, s := sequenceFixture(b)
		d.SequenceWriting.Stages = d.SequenceWriting.Stages[:1]
		for i := len(d.SequenceWriting.Stages[0].Rules); i < size.rules; i++ {
			symbols := []string{"a", "i", "p", "t", "d", "n"}
			x := i
			input := []OrthoUnit{}
			for j := 0; j < 3; j++ {
				input = append(input, OrthoUnit{Kind: "segment", Value: symbols[x%len(symbols)]})
				x /= len(symbols)
			}
			d.SequenceWriting.Stages[0].Rules = append(d.SequenceWriting.Stages[0].Rules, SpellingRule{ID: fmt.Sprintf("authored-%03d", i), Priority: 10, Input: input, Output: []OrthoUnit{{Kind: "grapheme", Value: "x"}}})
		}
		a := sequenceArtifact(b, d, s)
		q := sequenceQuery(a, "spell")
		for i := 0; i < size.units; i++ {
			q.Spelling = &SpellingRequest{}
			q.Segments = append(q.Segments, []string{"p", "a", "d", "a"}[i%4])
		}
		// Raw source units allow the full 512-unit orthographic domain; ordinary
		// segment-list requests preserve their older 256-segment transport bound.
		q.Spelling.Units = []OrthoUnit{}
		for _, segment := range q.Segments {
			q.Spelling.Units = append(q.Spelling.Units, OrthoUnit{Kind: "segment", Value: segment})
		}
		q.Segments = nil
		for _, cold := range []bool{false, true} {
			name := fmt.Sprintf("units-%d/rules-%d/cold-%v", size.units, size.rules, cold)
			b.Run(name, func(b *testing.B) {
				r := a.Execute(context.Background(), q)
				if r.Status != "success" && r.Status != "execution_limit" {
					b.Fatal(r.Status, a.report)
				}
				b.Logf("status=%s steps=%d spellings=%d", r.Status, r.Spelling.Steps, len(r.Spelling.Spellings))
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					current := a
					if cold {
						current = sequenceArtifact(b, d, s)
					}
					current.Execute(context.Background(), q)
				}
			})
		}
	}
	d, s := sequenceFixture(b)
	d.SequenceWriting.Stages = d.SequenceWriting.Stages[:1]
	a := sequenceArtifact(b, d, s)
	b.Run("inverse-high-ambiguity", func(b *testing.B) {
		q := sequenceQuery(a, "unspell")
		q.Text = "dddddddddddd"
		r := a.Execute(context.Background(), q)
		if r.Status != "execution_limit" {
			b.Fatal(r.Status)
		}
		b.Logf("status=%s steps=%d preimages=%d", r.Status, r.Spelling.Steps, len(r.Spelling.Preimages))
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			a.Execute(context.Background(), q)
		}
	})
}
