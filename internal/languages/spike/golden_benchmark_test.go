package spike

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestAuthoredGoldenOutputsAndCompleteTraces(t *testing.T) {
	c := compileFixture(t, fixture(t))
	for _, name := range []string{"regular-many", "seeded"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile("testdata/" + name + ".golden.json")
			if err != nil {
				t.Fatal(err)
			}
			var expected Result
			if err = json.Unmarshal(raw, &expected); err != nil {
				t.Fatal(err)
			}
			actual := c.Execute(context.Background(), expected.Request)
			if !bytes.Equal(Canonical(actual), Canonical(expected)) {
				t.Fatalf("golden contract changed; review the authored rule derivation and version before updating\nexpected: %s\nactual: %s", Canonical(expected), Canonical(actual))
			}
		})
	}
}
func BenchmarkCompileReference(b *testing.B) {
	d := fixture(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, report := Compile(context.Background(), d, TokenTrieBackend{})
		if report.Status != "compiled" {
			b.Fatal(report)
		}
	}
}
func BenchmarkInflectReference(b *testing.B) {
	c := compileFixture(b, fixture(b))
	r := request(c, "inflect")
	r.Lexeme = "stone"
	r.Features = Features{"number": "many"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if c.Execute(context.Background(), r).Status != "success" {
			b.Fatal("inflection failed")
		}
	}
}
func BenchmarkCompile1000Lexemes(b *testing.B) {
	d := fixture(b)
	source := d.Lexicon[0]
	d.Lexicon = nil
	for i := 0; i < 1000; i++ {
		lexeme := copyJSON(source)
		lexeme.ID = fmt.Sprintf("benchmark-%04d", i)
		lexeme.Senses[0].ID = fmt.Sprintf("sense-%04d", i)
		d.Lexicon = append(d.Lexicon, lexeme)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, report := Compile(context.Background(), d, TokenTrieBackend{})
		if report.Status != "compiled" || report.Forms != 2000 {
			b.Fatal(report)
		}
	}
}
