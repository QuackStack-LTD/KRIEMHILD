package languages

import (
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/languages/spike"
	"os"
	"reflect"
	"sort"
	"testing"
)

func indexedFixture(t testing.TB, n int) Definition {
	d := productiveInventory(t, n)
	d.Schema = 3
	return d
}
func TestIndexedFullGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/indexed.json")
	if err != nil {
		t.Fatal(err)
	}
	d, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile("testdata/indexed-inverse.request.json")
	if err != nil {
		t.Fatal(err)
	}
	var q Request
	if err = json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile("testdata/indexed-inverse.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden Result
	if err = json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	r := Compile(context.Background(), d, nil).Execute(context.Background(), q)
	if r.KnownValid != 2 || r.Examined != 9 || r.Rejected != 2 || r.Inverse.Pruned != 3 || r.Candidates[0].Lexeme != "rock" || r.Candidates[1].Lexeme != "stone" {
		t.Fatal("reviewed lexical/diagnostic semantics changed", r)
	}
	if string(Canonical(r)) != string(Canonical(golden)) {
		t.Fatal("full indexed request/result/trace golden differs")
	}
}
func TestIndexedInverseThousandPreservesSuppletionAndPrunes(t *testing.T) {
	d := indexedFixture(t, 1000)
	a := Compile(context.Background(), d, nil)
	if !a.report.Ready["analyze"] {
		t.Fatal(a.report)
	}
	q := productiveRequest(a, "analyze")
	q.Segments = []string{"n", "i", "n"}
	q.Limit = 256
	r := a.Execute(context.Background(), q)
	if r.Status != "ambiguous" || r.KnownValid != 53 || r.Inverse == nil || r.Inverse.Pruned < 2900 || r.Examined != 53 {
		t.Fatalf("incomplete suppletive inverse: %+v", r)
	}
	forward := productiveRequest(a, "inflect")
	forward.Lexeme = "lex-0001"
	forward.Features = Features{"number": "many", "shape": "plain"}
	f := a.Execute(context.Background(), forward)
	if f.Status != "success" {
		t.Fatal(f)
	}
	q.Segments = f.Candidates[0].Segments
	r = a.Execute(context.Background(), q)
	if r.Status != "success" && r.Status != "ambiguous" {
		t.Fatal(r)
	}
	if r.Examined >= 100 || r.Inverse.Steps >= productiveSteps {
		t.Fatal("index did not prune expensive composition", r.Inverse, r.Examined)
	}
	t.Logf("1000 entries: ordinary valid=%d examined=%d steps=%d; suppletion valid=53 examined=53", r.KnownValid, r.Examined, r.Inverse.Steps)
	for i := 0; i < 3; i++ {
		fresh := Compile(context.Background(), Clone(d), nil)
		if string(Canonical(fresh.Execute(context.Background(), q))) != string(Canonical(r)) {
			t.Fatal("cold rebuild or warm reuse changed canonical result")
		}
	}
	before := r.Inverse.Identity
	q.Policy = "explicit"
	other := a.Execute(context.Background(), q)
	if other.Inverse.Identity != before || other.Inverse.Request == r.Inverse.Request {
		t.Fatal("policy identity not separated")
	}
	d.Notes = "new authored revision"
	fresh := Compile(context.Background(), d, nil)
	if fresh.productive.inverse.identity == before {
		t.Fatal("stale revision identity")
	}
}

func TestIndexedInverseCompleteAgainstExhaustiveDomain(t *testing.T) {
	for _, mode := range []string{"ordinary", "epsilon-alternatives", "deletion", "insertion"} {
		t.Run(mode, func(t *testing.T) {
			d := productiveFixture(t)
			d.Schema = 3
			switch mode {
			case "epsilon-alternatives":
				op := &d.Productive.Paradigms[0].Cells[0].Operations
				*op = []ProductiveOperation{{ID: "epsilon", Kind: "zero", Policy: "all", Allomorphs: []Allomorph{{ID: "a", When: &AllomorphCondition{}}, {ID: "b", When: &AllomorphCondition{}}}}}
			case "deletion":
				d.Productive.Stratum.Rules = []spike.SoundRule{{ID: "delete", Domain: "morpheme", Input: []spike.Match{{Class: "C"}}, Output: []string{}}}
				for length := 1; length <= 5; length++ {
					slots := []spike.Match{}
					for i := 0; i < length; i++ {
						slots = append(slots, spike.Match{Class: "V"})
					}
					d.Phonology.Allowed = append(d.Phonology.Allowed, spike.Pattern{ID: fmt.Sprintf("vowel-%d", length), Slots: slots})
				}
			case "insertion":
				d.Productive.Stratum.Rules = append(d.Productive.Stratum.Rules, spike.SoundRule{ID: "insert", Domain: "morpheme", Output: []string{"p"}, Left: []spike.Match{{Boundary: "word"}}, Right: []spike.Match{{Segment: "a"}}})
			}
			a := Compile(context.Background(), d, nil)
			if !a.report.Ready["analyze"] {
				t.Fatal(a.report)
			}
			targets := map[string][]spike.Candidate{}
			for _, lex := range d.Lexicon {
				q := productiveRequest(a, "paradigm")
				q.Lexeme = lex.ID
				q.Limit = 256
				r := a.Execute(context.Background(), q)
				for _, c := range r.Candidates {
					key := string(Canonical(c.Segments))
					targets[key] = append(targets[key], c)
				}
			}
			keys := []string{}
			if len(targets) == 0 {
				t.Fatal("fixture has no accepted surface forms")
			}
			for k := range targets {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				expected := targets[k]
				sort.Slice(expected, func(i, j int) bool { return string(Canonical(expected[i])) < string(Canonical(expected[j])) })
				q := productiveRequest(a, "analyze")
				q.Segments = expected[0].Segments
				q.Limit = 256
				r := a.Execute(context.Background(), q)
				if !reflect.DeepEqual(r.Candidates, expected) {
					t.Fatalf("lost analysis mode=%s input=%s expected=%d actual=%+v", mode, k, len(expected), r)
				}
			}
			q := productiveRequest(a, "analyze")
			q.Segments = []string{"d", "d", "d"}
			r := a.Execute(context.Background(), q)
			if r.Status != "no_valid_candidate" || r.Inverse.Stop != r.Status {
				t.Fatal(r)
			}
		})
	}
}

func TestIndexedInverseWrittenNormalizationAndAmbiguousSegmentation(t *testing.T) {
	d := productiveFixture(t)
	d.Schema = 3
	_, scripts := productFixture(t)
	var s ScriptDefinition
	for _, script := range scripts {
		s = script
	}
	d.Orthography = &Orthography{Script: ScriptRef{s.ScriptID, Hash(s)}, Normalization: "NFC", Mappings: []spike.Mapping{}}
	for _, seg := range d.Phonology.Segments {
		d.Orthography.Mappings = append(d.Orthography.Mappings, spike.Mapping{ID: "map-" + seg.ID, Segment: seg.ID, Grapheme: "a"})
	}
	s.Graphemes = append(s.Graphemes, Grapheme{ID: "ambiguous-a", Text: "a"})
	d.Orthography.Script.Revision = Hash(s)
	a := Compile(context.Background(), d, map[ScriptRef]ScriptDefinition{d.Orthography.Script: s})
	if !a.report.Ready["analyze-written"] {
		t.Fatal(a.report)
	}
	q := productiveRequest(a, "analyze-written")
	q.Text = "aaa"
	q.Limit = 256
	r := a.Execute(context.Background(), q)
	if r.KnownValid < 3 || r.Status != "ambiguous" {
		t.Fatal("same written surface lost distinct analyses", r)
	}
	for _, c := range r.Candidates {
		if c.Written != q.Text {
			t.Fatal(c)
		}
	}
	// Combining marks across chunk boundaries may compose; pruning must remain
	// conservative rather than greedily choosing a mapping segmentation.
	for i := range d.Orthography.Mappings {
		if d.Orthography.Mappings[i].Segment == "a" {
			d.Orthography.Mappings[i].Grapheme = "\u0301"
		}
	}
	s.Graphemes = append(s.Graphemes, Grapheme{ID: "mark", Text: "\u0301"})
	d.Orthography.Script.Revision = Hash(s)
	a = Compile(context.Background(), d, map[ScriptRef]ScriptDefinition{d.Orthography.Script: s})
	f := productiveRequest(a, "paradigm")
	f.Lexeme = "stone"
	forms := a.Execute(context.Background(), f)
	for _, c := range forms.Candidates {
		q = productiveRequest(a, "analyze-written")
		q.Text = c.Written
		r = a.Execute(context.Background(), q)
		found := false
		for _, got := range r.Candidates {
			if Hash(got) == Hash(c) {
				found = true
			}
		}
		if !found {
			t.Fatal("normalization lost analysis", c, r)
		}
	}
}

func TestIndexedInverseLimitsAndCancellation(t *testing.T) {
	d := indexedFixture(t, 1000)
	// All stems become a possible epsilon signature if every source segment can
	// be consumed. The composer, not an unsound pruning shortcut, decides validity.
	d.Productive.Stratum.Rules = []spike.SoundRule{{ID: "delete-C", Domain: "morpheme", Input: []spike.Match{{Class: "C"}}}, {ID: "delete-V", Domain: "morpheme", Input: []spike.Match{{Class: "V"}}}}
	a := Compile(context.Background(), d, nil)
	q := productiveRequest(a, "analyze")
	r := a.Execute(context.Background(), q)
	if r.Status != "execution_limit" || len(r.Candidates) != 0 || r.KnownValid != 0 || r.Inverse.Stop != "execution_limit" {
		t.Fatal("adversarial domain returned a partial answer", r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r = a.Execute(ctx, q); r.Status != "cancelled" {
		t.Fatal(r)
	}
	if !a.report.Ready["inflect"] {
		t.Fatal("index disabled independent forward")
	}
}

func BenchmarkProductiveA2(b *testing.B) {
	for _, n := range []int{16, 128, 1000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			d := indexedFixture(b, n)
			a := Compile(context.Background(), d, nil)
			f := productiveRequest(a, "inflect")
			f.Lexeme = "lex-0001"
			f.Features = Features{"number": "many", "shape": "plain"}
			form := a.Execute(context.Background(), f)
			for _, mode := range []string{"CompileCold", "InverseCold", "InverseWarm", "AmbiguityWarm"} {
				b.Run(mode, func(b *testing.B) {
					q := productiveRequest(a, "analyze")
					q.Segments = form.Candidates[0].Segments
					q.Limit = 256
					if mode == "AmbiguityWarm" {
						q.Segments = []string{"n", "i", "n"}
					}
					r := a.Execute(context.Background(), q)
					b.Logf("status=%s valid=%d examined=%d metrics=%+v", r.Status, r.KnownValid, r.Examined, r.Inverse)
					if r.Status != "success" && r.Status != "ambiguous" {
						b.Fatal(r)
					}
					b.ReportAllocs()
					b.ResetTimer()
					for b.Loop() {
						current := a
						if mode == "CompileCold" || mode == "InverseCold" {
							current = Compile(context.Background(), d, nil)
						}
						if mode != "CompileCold" {
							if r := current.Execute(context.Background(), q); r.Status != "success" && r.Status != "ambiguous" {
								b.Fatal(r)
							}
						}
					}
				})
			}
		})
	}
}
