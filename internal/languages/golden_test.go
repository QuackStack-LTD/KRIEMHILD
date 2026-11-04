package languages

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func productFixture(t *testing.T) (Definition, map[ScriptRef]ScriptDefinition) {
	t.Helper()
	raw, err := os.ReadFile("testdata/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	d, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile("testdata/script.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := DecodeScript(raw)
	if err != nil {
		t.Fatal(err)
	}
	return d, map[ScriptRef]ScriptDefinition{{ID: s.ScriptID, Revision: Hash(s)}: s}
}
func TestProductGoldenReplay(t *testing.T) {
	d, deps := productFixture(t)
	a := Compile(context.Background(), d, deps)
	if Hash(d) != "9d5c2c6d141549aac64bc61da9ca7c9ef37cea3cfd876a8f1a728be23994d778" {
		t.Fatal("authored fixture changed; review schema and fixture versions")
	}
	for _, name := range []string{"inflect", "seeded", "parse", "corpus"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile("testdata/" + name + ".request.json")
			if err != nil {
				t.Fatal(err)
			}
			var q Request
			if err = json.Unmarshal(raw, &q); err != nil {
				t.Fatal(err)
			}
			golden, err := os.ReadFile("testdata/" + name + ".golden.json")
			if err != nil {
				t.Fatal(err)
			}
			var expected Result
			if err = json.Unmarshal(golden, &expected); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 10; i++ {
				actual := a.Execute(context.Background(), q)
				if string(Canonical(actual)) != string(Canonical(expected)) {
					t.Fatalf("%s golden or trace changed", name)
				}
			}
		})
	}
	raw, err := os.ReadFile("testdata/descendant.json")
	if err != nil {
		t.Fatal(err)
	}
	target, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile("testdata/history.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected Evolution
	if err = json.Unmarshal(raw, &expected); err != nil {
		t.Fatal(err)
	}
	if string(Canonical(PreviewEvolution(context.Background(), d, target))) != string(Canonical(expected)) {
		t.Fatal("historical golden or provenance changed")
	}
}
func BenchmarkProductCompile(b *testing.B) {
	raw, _ := os.ReadFile("testdata/reference.json")
	d, _ := Decode(raw)
	raw, _ = os.ReadFile("testdata/script.json")
	s, _ := DecodeScript(raw)
	deps := map[ScriptRef]ScriptDefinition{{ID: s.ScriptID, Revision: Hash(s)}: s}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		Compile(context.Background(), d, deps)
	}
}
func BenchmarkProductCompile1000Lexemes(b *testing.B) {
	raw, _ := os.ReadFile("testdata/reference.json")
	d, _ := Decode(raw)
	raw, _ = os.ReadFile("testdata/script.json")
	s, _ := DecodeScript(raw)
	deps := map[ScriptRef]ScriptDefinition{{ID: s.ScriptID, Revision: Hash(s)}: s}
	base := d.Lexicon[0]
	d.Lexicon = nil
	d.Tests = nil
	for i := 0; i < 1000; i++ {
		lex := Clone(base)
		lex.ID = fmt.Sprintf("lexeme-%04d", i)
		for j := range lex.Senses {
			lex.Senses[j].ID = fmt.Sprintf("lexeme-%04d-sense-%d", i, j)
		}
		d.Lexicon = append(d.Lexicon, lex)
	}
	if report := Compile(context.Background(), d, deps).Report(); !report.Ready["inflect"] || report.Forms != 2000 {
		b.Fatalf("large benchmark must compile 2,000 real forms: %+v", report)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		Compile(context.Background(), d, deps)
	}
}
