package languages

import (
	"context"
	"fmt"
	"kriemhild/internal/languages/spike"
	"runtime"
	"testing"
)

// Synthetic, varied authored domain: unique long stems, deliberate homophones,
// polysemy and suppletion. This is a bounded workload, not a natural-language
// corpus or evidence about inventories larger than the supported 1,000 entries.
func productiveInventory(t testing.TB, size int) Definition {
	t.Helper()
	d := productiveFixture(t)
	d.Lexicon = nil
	for _, length := range []int{11, 12, 13} {
		slots := []spike.Match{}
		for i := 0; i < length; i++ {
			class := "C"
			if i%2 == 1 {
				class = "V"
			}
			if length == 13 {
				class = "V"
				if i%2 == 1 {
					class = "C"
				}
			}
			slots = append(slots, spike.Match{Class: class})
		}
		d.Phonology.Allowed = append(d.Phonology.Allowed, spike.Pattern{ID: fmt.Sprintf("long-%d", length), Slots: slots})
	}
	for i := 0; i < size; i++ {
		value := i
		stem := []string{}
		for j := 0; j < 6; j++ {
			stem = append(stem, []string{"p", "t", "n", "d"}[value%4])
			value /= 4
			if j < 5 {
				stem = append(stem, []string{"a", "i"}[value%2])
				value /= 2
			}
		}
		if i > 0 && i%13 == 0 {
			stem = append([]string{}, d.Lexicon[i-1].Underlying...)
		}
		lex := spike.Lexeme{ID: fmt.Sprintf("lex-%04d", i), Category: "noun", Underlying: stem, Paradigm: "noun", Features: Features{}, Senses: []spike.Sense{{ID: fmt.Sprintf("sense-%04d", i), Meaning: fmt.Sprintf("authored synthetic meaning %d", i)}}}
		if i%17 == 0 {
			lex.Senses = append(lex.Senses, spike.Sense{ID: fmt.Sprintf("polysemy-%04d", i), Meaning: "a separately authored second sense"})
		}
		if i%19 == 0 {
			lex.Overrides = []spike.Override{{Cell: "many", RuleID: fmt.Sprintf("suppletion-%04d", i), Segments: []string{"n", "i", "n"}}}
		}
		d.Lexicon = append(d.Lexicon, lex)
	}
	return d
}

func TestProductiveBenchmarkInventoryIsExecutableAndMalformedDataRejected(t *testing.T) {
	d := productiveInventory(t, 1000)
	a := Compile(context.Background(), d, nil)
	if !a.report.Ready["inflect"] || a.report.Status != "compiled" || len(a.productive.lexemes) != 1000 || a.report.Forms != 0 {
		t.Fatal("benchmark must measure executable plans", a.report)
	}
	unique := map[string]bool{}
	for _, lex := range d.Lexicon {
		unique[string(Canonical(lex.Underlying))] = true
	}
	if len(unique) < 900 {
		t.Fatal("benchmark merely duplicated a tiny fixture", len(unique))
	}
	q := productiveRequest(a, "inflect")
	q.Lexeme = "lex-0001"
	q.Features = Features{"number": "many", "shape": "plain"}
	if r := a.Execute(context.Background(), q); r.Status != "success" {
		t.Fatal("benchmark forward request is invalid", r)
	}
	d.Lexicon[99].Underlying = []string{"missing-segment"}
	if a = Compile(context.Background(), d, nil); a.report.Ready["inflect"] || a.report.Status != "partial" {
		t.Fatal("malformed benchmark data was counted as executable", a.report)
	}
}

func BenchmarkProductiveA1(b *testing.B) {
	for _, size := range []int{16, 128, 1000} {
		b.Run(fmt.Sprintf("%d", size), func(b *testing.B) {
			d := productiveInventory(b, size)
			a := Compile(context.Background(), d, nil)
			if !a.report.Ready["inflect"] {
				b.Fatal(a.report)
			}
			forward := productiveRequest(a, "inflect")
			forward.Lexeme = "lex-0001"
			forward.Features = Features{"number": "many", "shape": "plain"}
			form := a.Execute(context.Background(), forward)
			if form.Status != "success" {
				b.Fatal(form)
			}
			inverse := productiveRequest(a, "analyze")
			inverse.Segments = form.Candidates[0].Segments
			ambiguous := productiveRequest(a, "analyze")
			ambiguous.Segments = []string{"n", "i", "n"}
			for _, operation := range []string{"CompileCold", "ForwardWarm", "InverseWarm", "AmbiguityWarm", "ForwardCold", "RevisionInvalidation"} {
				b.Run(operation, func(b *testing.B) {
					query := inverse
					if operation == "AmbiguityWarm" {
						query = ambiguous
					}
					if operation == "InverseWarm" || operation == "AmbiguityWarm" {
						r := a.Execute(context.Background(), query)
						b.Logf("observed inverse status=%s valid=%d examined=%d rejected=%d", r.Status, r.KnownValid, r.Examined, r.Rejected)
						if r.Status != "success" && r.Status != "ambiguous" && r.Status != "execution_limit" {
							b.Fatal(r)
						}
					}
					b.ReportAllocs()
					cache := NewCache(16 << 20)
					cache.Put(ArtifactKey("bench-world", "bench-age", d), a)
					revision := 0
					b.ResetTimer()
					for b.Loop() {
						switch operation {
						case "CompileCold":
							if !Compile(context.Background(), d, nil).report.Ready["inflect"] {
								b.Fatal("invalid compile")
							}
						case "ForwardWarm":
							if a.Execute(context.Background(), forward).Status != "success" {
								b.Fatal("invalid forward")
							}
						case "InverseWarm", "AmbiguityWarm":
							r := a.Execute(context.Background(), query)
							if r.Status != "success" && r.Status != "ambiguous" && r.Status != "execution_limit" {
								b.Fatal(r)
							}
						case "ForwardCold":
							fresh := Compile(context.Background(), d, nil)
							if fresh.Execute(context.Background(), forward).Status != "success" {
								b.Fatal("invalid cold forward")
							}
						case "RevisionInvalidation":
							revision++
							next := Clone(d)
							next.Notes = fmt.Sprintf("authored revision %d", revision)
							key := ArtifactKey("bench-world", "bench-age", next)
							if cache.Get(key) != nil {
								b.Fatal("stale artifact used")
							}
							cache.Put(key, Compile(context.Background(), next, nil))
						}
					}
				})
			}
		})
	}
}

func TestProductiveRetainedArtifactMeasurement(t *testing.T) {
	if testing.Short() {
		t.Skip("explicit retained-heap measurement")
	}
	for _, size := range []int{128, 1000} {
		d := productiveInventory(t, size)
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		a := Compile(context.Background(), d, nil)
		runtime.GC()
		runtime.ReadMemStats(&after)
		t.Logf("entries=%d retained-heap-delta=%d accounted-reservation=%d (GC snapshot, not peak heap)", size, int64(after.HeapAlloc)-int64(before.HeapAlloc), a.AccountedBytes())
		runtime.KeepAlive(a)
		runtime.KeepAlive(d)
	}
}
