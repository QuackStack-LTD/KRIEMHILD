package languages

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func vectorFixture(t testing.TB) ScriptDefinition {
	t.Helper()
	raw, err := os.ReadFile("testdata/vector-script.json")
	if err != nil {
		t.Fatal(err)
	}
	d, err := DecodeScript(raw)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func vectorRequest(d ScriptDefinition) ScriptRenderRequest {
	return ScriptRenderRequest{Revision: Hash(d), Engine: VectorEngine, Adapter: VectorAdapter, Compiler: VectorCompiler, SequenceID: "mixed-phrase"}
}
func TestVectorFullGoldens(t *testing.T) {
	d := vectorFixture(t)
	raw, err := os.ReadFile("testdata/vector-render.request.json")
	if err != nil {
		t.Fatal(err)
	}
	var q ScriptRenderRequest
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	check := func(name string, actual any) {
		t.Helper()
		raw, err := os.ReadFile("testdata/" + name + ".golden.json")
		if err != nil {
			t.Fatal(err)
		}
		var expected any
		if err := json.Unmarshal(raw, &expected); err != nil {
			t.Fatal(err)
		}
		var result any
		if err := json.Unmarshal(Canonical(actual), &result); err != nil {
			t.Fatal(err)
		}
		if string(Canonical(result)) != string(Canonical(expected)) {
			t.Fatalf("%s: complete result differs from reviewed golden", name)
		}
	}
	check("vector-render", RenderScript(context.Background(), d, q))
	raw, err = os.ReadFile("testdata/vector-import.svg")
	if err != nil {
		t.Fatal(err)
	}
	imported, err := ImportGlyphSVG(context.Background(), string(raw), "imported-glyph", "Imported artwork")
	if err != nil {
		t.Fatal(err)
	}
	check("vector-import", imported)
}
func TestVectorStrictSourceDepthAndGeometryBudgets(t *testing.T) {
	d := vectorFixture(t)
	raw := Canonical(d)
	for name, bad := range map[string][]byte{
		"unknown":   []byte(strings.Replace(string(raw), `"profile":"mixed"`, `"profile":"mixed","executable":"no"`, 1)),
		"duplicate": []byte(strings.Replace(string(raw), `"schema":2`, `"schema":2,"schema":2`, 1)),
		"unicode":   append([]byte(`{"schema":2,"name":"`), 0xff),
	} {
		if _, err := DecodeScript(bad); err == nil {
			t.Fatalf("%s source was accepted", name)
		}
	}
	d.Writing.Sequences = nil
	q := vectorRequest(d)
	q.SequenceID = ""
	items := []SignUse{{Kind: "sign", Sign: "living-marker", Roles: []string{"classifier"}}}
	for i := 0; i < 4; i++ {
		items = []SignUse{{Kind: "group", Children: items}}
	}
	q.Sequence = &SignSequence{ID: "depth", Items: items}
	if r := RenderScript(context.Background(), d, q); r.Status != "success" || len(r.Signs) != 1 {
		t.Fatal("declared depth four failed", r)
	}
	q.Sequence.Items = []SignUse{{Kind: "group", Children: items}}
	if r := RenderScript(context.Background(), d, q); r.Status != "execution_limit" || r.SVG != "" {
		t.Fatal("excessive depth accepted", r)
	}
	d = vectorFixture(t)
	d.Writing.Glyphs[0].Paths[0].Commands = []PathCommand{{Op: "M", Values: []float64{0, 0}}}
	if CompileScript(context.Background(), d).Ready["render"] {
		t.Fatal("a move-only path was presented as artwork")
	}
	for i := 0; i < 300; i++ {
		d.Writing.Glyphs[0].Paths[0].Commands = append(d.Writing.Glyphs[0].Paths[0].Commands, PathCommand{Op: "L", Values: []float64{float64(i), float64(i)}})
	}
	q = vectorRequest(d)
	q.SequenceID = ""
	q.Sequence = &SignSequence{ID: "geometry", Items: []SignUse{}}
	for i := 0; i < 512; i++ {
		q.Sequence.Items = append(q.Sequence.Items, SignUse{Kind: "sign", Sign: "syllable-ka", Reading: "ka-reading", Roles: []string{"syllabic"}})
	}
	if r := RenderScript(context.Background(), d, q); r.Status != "execution_limit" || r.SVG != "" || len(r.Signs) != 0 || len(r.Trace) != 0 {
		t.Fatal("geometry/output budget returned partial output", r.Status)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r := RenderScript(ctx, d, q); r.Status != "cancelled" || r.SVG != "" {
		t.Fatal("render ignored cancellation", r.Status)
	}
	for i := 0; i < 4097; i++ {
		d.Writing.Glyphs[0].Paths[0].Commands = append(d.Writing.Glyphs[0].Paths[0].Commands, PathCommand{Op: "L", Values: []float64{1, 1}})
	}
	if _, err := DecodeScript(Canonical(d)); err == nil {
		t.Fatalf("oversized canonical geometry accepted: %d commands", len(d.Writing.Glyphs[0].Paths[0].Commands))
	}
}
func TestVectorMixedRolesLayoutAndImmutableRevision(t *testing.T) {
	d := vectorFixture(t)
	q := vectorRequest(d)
	r := RenderScript(context.Background(), d, q)
	if r.Status != "success" || len(r.Signs) != 3 || r.Signs[0].Text != "ka" || r.Signs[1].Text != "sol" || r.Signs[2].Text != "" || r.Signs[2].Roles[0] != "classifier" || len(r.Signs[1].Roles) != 2 {
		t.Fatalf("roles/readings conflated: %+v", r)
	}
	if r.Signs[0].Grapheme != "e\u0301" || r.Signs[0].X != 0 || r.Signs[1].X != 1080 || r.Signs[2].X != 2160 || r.Signs[2].Scale != .75 {
		t.Fatal("authored grapheme/metrics changed", r.Signs)
	}
	if !strings.Contains(r.SVG, `data-glyph="sun"`) || strings.Contains(r.SVG, "href=") || strings.Contains(r.SVG, "font") {
		t.Fatal("not a self-contained vector", r.SVG)
	}
	original := string(Canonical(r))
	next := Clone(d)
	next.Direction = "rtl"
	next.Writing.Glyphs[0].Name = "Renamed without changing identity"
	next.Writing.Glyphs[0].Paths[0].Commands[1].Values[1] = 200
	rtl := RenderScript(context.Background(), next, vectorRequest(next))
	if rtl.Status != "success" || rtl.Signs[0].X != -1080 || rtl.Signs[1].X != -2160 || rtl.Signs[0].Glyph != r.Signs[0].Glyph {
		t.Fatal("RTL or stable identity failed", rtl)
	}
	if string(Canonical(RenderScript(context.Background(), d, q))) != original || rtl.Revision == r.Revision {
		t.Fatal("new revision rewrote old output")
	}
	next.Direction = "ttb"
	report := CompileScript(context.Background(), next)
	if report.Ready["render"] || report.Diagnostics[0].Code != "unsupported_layout" {
		t.Fatal("vertical falsely advertised", report)
	}
}
func TestVectorReadingsAndExplicitChoices(t *testing.T) {
	d := vectorFixture(t)
	q := vectorRequest(d)
	q.SequenceID = ""
	q.Sequence = &SignSequence{ID: "test", Name: "Explicit choices", Items: []SignUse{{Kind: "sign", Sign: "polyphonic", Roles: []string{"phonetic", "semantic"}, Context: Features{"vowel": "front"}}}}
	r := RenderScript(context.Background(), d, q)
	if r.Status != "choice_required" || r.SVG != "" || len(r.Choices) != 1 || len(r.Choices[0].Readings) != 1 || r.Choices[0].Readings[0] != "front" {
		t.Fatal("reading silently guessed", r)
	}
	q.Sequence.Items[0].Reading = "back"
	if r = RenderScript(context.Background(), d, q); r.Status != "unresolved_reading" {
		t.Fatal("context ignored", r)
	}
	q.Sequence.Items[0].Reading = "front"
	r = RenderScript(context.Background(), d, q)
	if r.Status != "success" || r.Signs[0].Text != "ke" || len(r.Signs[0].Roles) != 2 {
		t.Fatal(r)
	}
	q.Sequence.Items = append(q.Sequence.Items, SignUse{Kind: "break"}, q.Sequence.Items[0])
	r = RenderScript(context.Background(), d, q)
	if r.Status != "success" || r.Signs[1].Baseline != 2400 || r.Signs[1].X != 0 {
		t.Fatal("line break not explicit", r)
	}
	q.Sequence.Items[0].Sign = "missing"
	if r = RenderScript(context.Background(), d, q); r.Status != "missing_sign" || r.SVG != "" {
		t.Fatal("missing sign replaced", r)
	}
}
func TestVectorLimitsDecodeAndCancellation(t *testing.T) {
	d := vectorFixture(t)
	d.Writing.Glyphs[0].Paths[0].Commands[0].Values[0] = 1e90
	if report := CompileScript(context.Background(), d); report.Ready["render"] {
		t.Fatal("extreme geometry accepted")
	}
	d = vectorFixture(t)
	q := vectorRequest(d)
	q.SequenceID = ""
	q.Sequence = &SignSequence{ID: "large", Items: []SignUse{}}
	for i := 0; i < 513; i++ {
		q.Sequence.Items = append(q.Sequence.Items, SignUse{Kind: "sign", Sign: "living-marker", Roles: []string{"classifier"}})
	}
	if r := RenderScript(context.Background(), d, q); r.Status != "execution_limit" || r.SVG != "" || len(r.Signs) > 0 {
		t.Fatal("partial oversized output", r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if report := CompileScript(ctx, d); report.Ready["render"] {
		t.Fatal("cancelled compile ready")
	}
	old := ScriptDefinition{Schema: 1, ScriptID: d.ScriptID, Name: "Old", Direction: "ltr", Graphemes: []Grapheme{}}
	raw := Canonical(old)
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	fields["writing"] = nil
	if _, err := DecodeScript(Canonical(fields)); err == nil {
		t.Fatal("schema 1 accepted a schema 2 field")
	}
}
func TestSafeSVGImportAndNormalization(t *testing.T) {
	source := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><g fill="none" stroke="#182126" stroke-width="2"><path id="a" d="m10 10 h20 v20 l-20 0 z"/><circle cx="60" cy="60" r="10"/><rect x="20" y="60" width="20" height="20"/></g></svg>`
	result, err := ImportGlyphSVG(context.Background(), source, "stable-id", "Authored import")
	if err != nil {
		t.Fatal(err)
	}
	if result.Glyph.ID != "stable-id" || len(result.Glyph.Paths) != 3 || result.Glyph.ViewBox[2] != 100 || result.Glyph.Paths[0].Commands[1].Op != "L" || result.Glyph.Paths[0].Commands[1].Values[0] != 30 || len(result.Warnings) < 3 {
		t.Fatal("SVG was not normalized", result)
	}
	for _, bad := range []string{
		`<svg><script>alert(1)</script></svg>`, `<svg onload="x()"><path d="M0 0L1 1"/></svg>`, `<svg><foreignObject/></svg>`, `<svg><image href="https://example.com/a"/></svg>`, `<svg><path fill="url(https://example.com/a)" d="M0 0L1 1"/></svg>`, `<svg><use href="#a"/></svg>`, `<svg><path id="same" d="M0 0L1 1"/><path id="same" d="M1 1L2 2"/></svg>`, `<svg><path id="" id="b" d="M0 0L1 1"/></svg>`, `<svg><path d="M0 0A2 2 0 0 0 2 2"/></svg>`, `<svg><path d="M1e99 0L1 1"/></svg>`, `<svg><path transform="scale(2)" d="M0 0L1 1"/></svg>`, `<!DOCTYPE svg [<!ENTITY foo SYSTEM "https://example.com/private">]><svg/>`, `<svg><path d="M0 0L"/></svg>`, `<svg><path d="M0 0L1 1"></svg>`, `<svg>` + strings.Repeat(`<g>`, 17) + `<path d="M0 0L1 1"/>` + strings.Repeat(`</g>`, 17) + `</svg>`, `<svg>` + strings.Repeat(`<path d="M0 0L1 1"/>`, 257) + `</svg>`, `<svg><path d="M0 0` + strings.Repeat(` L1 1`, 4097) + `"/></svg>`, strings.Repeat("x", SVGImportBytes+1),
	} {
		if _, err := ImportGlyphSVG(context.Background(), bad, "id", "test"); err == nil {
			t.Fatalf("unsafe/malformed input accepted: %.100s", bad)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ImportGlyphSVG(ctx, source, "id", "test"); err == nil {
		t.Fatal("import ignored cancellation")
	}
}
