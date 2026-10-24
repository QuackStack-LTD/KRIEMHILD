package languages

import (
	"context"
	"kriemhild/internal/languages/spike"
	"strings"
	"testing"
)

// Cancellation occurs after traversal starts, rather than only before Execute.
type productiveCancelContext struct {
	context.Context
	checks int
}

func (c *productiveCancelContext) Err() error {
	c.checks++
	if c.checks >= 20 {
		return context.Canceled
	}
	return nil
}

func TestProductiveInnerCancellationAndIntermediateLimits(t *testing.T) {
	d := productiveFixture(t)
	a := Compile(context.Background(), d, nil)
	q := productiveRequest(a, "inflect")
	q.Lexeme = "stone"
	q.Features = Features{"number": "many", "shape": "framed"}
	ctx := &productiveCancelContext{Context: context.Background()}
	if r := a.Execute(ctx, q); r.Status != "cancelled" || len(r.Candidates) != 0 || len(r.Derivations) != 0 || ctx.checks < 20 {
		t.Fatal("inner cancellation returned partial output", r, ctx.checks)
	}
	for _, mutate := range []func(*ProductiveModel){
		func(m *ProductiveModel) { m.Features = make([]spike.Feature, 65) },
		func(m *ProductiveModel) { m.Features[0].Values = make([]string, 65) },
		func(m *ProductiveModel) { m.Paradigms = make([]ProductiveParadigm, 65) },
		func(m *ProductiveModel) { m.Paradigms[0].Cells = make([]ProductiveCell, 65) },
		func(m *ProductiveModel) { m.Paradigms[0].Cells[0].Operations = make([]ProductiveOperation, 17) },
		func(m *ProductiveModel) { m.Paradigms[0].Cells[0].Operations[0].Allomorphs = make([]Allomorph, 17) },
		func(m *ProductiveModel) {
			m.Paradigms[0].Cells[0].Operations[0].Allomorphs[0].When.Left = make([]spike.Match, 17)
		},
		func(m *ProductiveModel) {
			m.Paradigms[0].Cells[0].Operations[0].Allomorphs[0].Prefix = make([]string, 257)
		},
		func(m *ProductiveModel) { m.Stratum.Rules = make([]spike.SoundRule, 33) },
		func(m *ProductiveModel) { m.Stratum.Rules[0].Input = make([]spike.Match, 17) },
	} {
		invalid := Clone(d)
		mutate(invalid.Productive)
		if _, err := Decode(Canonical(invalid)); err == nil {
			t.Fatal("oversized source inventory accepted")
		}
	}
	alt := &d.Productive.Paradigms[0].Cells[1].Operations[0].Allomorphs[0]
	alt.Suffix = make([]string, 256)
	for i := range alt.Suffix {
		alt.Suffix[i] = "a"
	}
	a = Compile(context.Background(), d, nil)
	q = productiveRequest(a, "inflect")
	q.Lexeme = "stone"
	q.Features = Features{"number": "many", "shape": "plain"}
	if r := a.Execute(context.Background(), q); r.Status != "execution_limit" || len(r.Candidates) != 0 {
		t.Fatal("oversized intermediate form escaped limit", r)
	}
	branch := productiveBranch{}
	if err := branch.appendStep(&productiveBudget{ctx: context.Background()}, ProductiveStep{After: make([]ProductiveToken, 513)}); err == nil {
		t.Fatal("typed token bound not enforced")
	}
	raw := string(Canonical(productiveFixture(t)))
	raw = strings.Replace(raw, `"version":"productive/1"`, `"version":"productive/1","version":"productive/1"`, 1)
	if _, err := Decode([]byte(raw)); err == nil {
		t.Fatal("duplicate productive key accepted")
	}
	raw = string(Canonical(productiveFixture(t)))
	raw = strings.Replace(raw, `"version":"productive/1"`, `"version":"`+string([]byte{0xff})+`"`, 1)
	if _, err := Decode([]byte(raw)); err == nil {
		t.Fatal("invalid UTF-8 in productive source accepted")
	}
	old, _ := productFixture(t)
	raw = string(Canonical(old))
	raw = strings.Replace(raw, `"schema":1`, `"productive":null,"schema":1`, 1)
	if _, err := Decode([]byte(raw)); err == nil {
		t.Fatal("null schema-2 field silently discarded in schema 1")
	}
}

func TestProductivePrefixAndClassConditionsRoundTrip(t *testing.T) {
	d := productiveFixture(t)
	op := &d.Productive.Paradigms[0].Cells[1].Operations[0]
	op.Kind = "prefix"
	op.Allomorphs = op.Allomorphs[:1]
	op.Allomorphs[0].Prefix = []string{"a"}
	op.Allomorphs[0].Suffix = nil
	op.Allomorphs[0].When.Right = []spike.Match{{Class: "C"}}
	d.Phonology.Allowed = append(d.Phonology.Allowed, spike.Pattern{ID: "VCVC", Slots: []spike.Match{{Class: "V"}, {Class: "C"}, {Class: "V"}, {Class: "C"}}})
	a := Compile(context.Background(), d, nil)
	q := productiveRequest(a, "inflect")
	q.Lexeme = "stone"
	q.Features = Features{"number": "many", "shape": "plain"}
	r := a.Execute(context.Background(), q)
	if r.Status != "success" || strings.Join(r.Candidates[0].Segments, " ") != "a p a t" {
		t.Fatal("authored prefix/class environment failed", r)
	}
	q = productiveRequest(a, "analyze")
	q.Segments = r.Candidates[0].Segments
	if r = a.Execute(context.Background(), q); r.Status != "ambiguous" || r.KnownValid != 2 {
		t.Fatal("prefix inversion lost homophones", r)
	}
}
