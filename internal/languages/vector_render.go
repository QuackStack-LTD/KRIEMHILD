package languages

import (
	"context"
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
)

type ScriptRenderRequest struct {
	Revision   string        `json:"revision"`
	Engine     string        `json:"engine"`
	Adapter    string        `json:"adapter"`
	Compiler   string        `json:"compiler"`
	SequenceID string        `json:"sequenceId,omitempty"`
	Sequence   *SignSequence `json:"sequence,omitempty"`
}
type RenderedSign struct {
	Path         string            `json:"path"`
	Sign         string            `json:"sign"`
	Glyph        string            `json:"glyph"`
	Reading      string            `json:"reading"`
	Text         string            `json:"text"`
	Grapheme     string            `json:"grapheme"`
	Roles        []string          `json:"roles"`
	Associations []SignAssociation `json:"associations"`
	X            float64           `json:"x"`
	Baseline     float64           `json:"baseline"`
	Scale        float64           `json:"scale"`
	Advance      float64           `json:"advance"`
}
type ScriptTrace struct {
	Stage    string   `json:"stage"`
	Path     string   `json:"path"`
	Sign     string   `json:"sign,omitempty"`
	Glyph    string   `json:"glyph,omitempty"`
	Reading  string   `json:"reading,omitempty"`
	Roles    []string `json:"roles,omitempty"`
	X        float64  `json:"x"`
	Baseline float64  `json:"baseline"`
}
type ReadingChoice struct {
	Path     string   `json:"path"`
	Sign     string   `json:"sign"`
	Readings []string `json:"readings"`
}
type ScriptResult struct {
	Status      string              `json:"status"`
	ScriptID    string              `json:"scriptId"`
	Revision    string              `json:"revision"`
	Engine      string              `json:"engine"`
	Adapter     string              `json:"adapter"`
	Compiler    string              `json:"compiler"`
	Input       ScriptRenderRequest `json:"input"`
	Sequence    SignSequence        `json:"sequence"`
	Signs       []RenderedSign      `json:"signs"`
	Trace       []ScriptTrace       `json:"trace"`
	Choices     []ReadingChoice     `json:"choices"`
	SVG         string              `json:"svg"`
	Diagnostics []Diagnostic        `json:"diagnostics"`
}

func vectorDiagnostic(code, id, path, message string) Diagnostic {
	return Diagnostic{Code: code, Severity: "error", Module: "script", RuleID: id, Path: path, Message: message}
}
func ValidateWriting(ctx context.Context, d ScriptDefinition) []Diagnostic {
	out := []Diagnostic{}
	bad := func(code, id, path, message string) {
		if len(out) < 256 {
			out = append(out, vectorDiagnostic(code, id, path, message))
		}
	}
	if d.Schema != 2 || d.Writing == nil {
		bad("missing_dependency", "", "writing", "Choose Script schema 2 and author normalized glyphs/signs.")
		return out
	}
	w := d.Writing
	if err := vectorShape(w); err != nil {
		bad("definition_limit", "", "writing", err.Error())
		return out
	}
	if w.Version != VectorEngine {
		bad("incompatible_engine", "", "writing.version", "Use vector-script/1; no fallback is applied.")
	}
	switch w.Profile {
	case "alphabetic", "abjad", "abugida", "syllabic", "logographic", "mixed":
	default:
		bad("invalid_profile", "", "writing.profile", "Choose an authored writing profile; profiles do not restrict mixed sign roles.")
	}
	if d.Direction != "ltr" && d.Direction != "rtl" {
		bad("unsupported_layout", "", "direction", "Only explicit left-to-right and right-to-left layout are executable. Vertical/undecided layout is unsupported.")
	}
	if !finiteVector(w.UnitsPerEm) || w.UnitsPerEm < 16 || w.UnitsPerEm > 4096 || !finiteVector(w.LineHeight) || w.LineHeight < 16 || !finiteVector(w.Spacing) || w.Spacing < 0 {
		bad("invalid_metrics", "", "writing", "Use finite unitsPerEm 16–4096, lineHeight 16–8192 and nonnegative spacing up to 8192.")
	}
	seen := map[string]bool{}
	id := func(namespace, value string) {
		if strings.TrimSpace(value) == "" || len(value) > 80 || seen[namespace+"/"+value] {
			bad("invalid_id", value, namespace, "Use a unique, nonempty stable ID of at most 80 bytes.")
		}
		seen[namespace+"/"+value] = true
	}
	glyphs := map[string]bool{}
	for _, g := range w.Glyphs {
		if ctx.Err() != nil {
			return []Diagnostic{vectorDiagnostic("cancelled", "", "writing", ctx.Err().Error())}
		}
		id("writing.glyphs", g.ID)
		glyphs[g.ID] = true
		if len(g.Paths) == 0 {
			bad("missing_artwork", g.ID, "writing.glyphs.paths", "Draw or import artwork before compiling this glyph.")
		}
		if len(g.Name) > 160 || !finiteVector(g.Baseline) || !finiteVector(g.Advance) || g.Advance < 0 || !finiteVector(g.Bearing) || !finiteVector(g.Scale) || g.Scale < .05 || g.Scale > 16 {
			bad("invalid_metrics", g.ID, "writing.glyphs", "Use finite glyph metrics, nonnegative advance, and scale 0.05–16.")
		}
		for _, v := range g.ViewBox {
			if !finiteVector(v) {
				bad("invalid_metrics", g.ID, "writing.glyphs.viewBox", "Use finite viewBox coordinates within ±8192.")
			}
		}
		if g.ViewBox[2] < 1 || g.ViewBox[2] > 4096 || g.ViewBox[3] < 1 || g.ViewBox[3] > 4096 {
			bad("invalid_metrics", g.ID, "writing.glyphs.viewBox", "ViewBox width/height must be 1–4096.")
		}
		for _, p := range g.Paths {
			if len(p.Commands) == 0 {
				bad("missing_artwork", p.ID, "writing.glyphs.paths.commands", "Author at least one path command.")
			}
			id("writing.glyphs/"+g.ID+"/paths", p.ID)
			if !vectorColor.MatchString(p.Fill) || !vectorColor.MatchString(p.Stroke) || !finiteVector(p.StrokeWidth) || p.StrokeWidth < 0 || p.StrokeWidth > 256 {
				bad("invalid_geometry", p.ID, "writing.glyphs.paths", "Use #RRGGBB or none and stroke width 0–256.")
			}
			drawing := false
			for ci, c := range p.Commands {
				if ctx.Err() != nil {
					return []Diagnostic{vectorDiagnostic("cancelled", p.ID, "writing.glyphs.paths", ctx.Err().Error())}
				}
				count, ok := map[string]int{"M": 2, "L": 2, "Q": 4, "C": 6, "Z": 0}[c.Op]
				if !ok || len(c.Values) != count || ci == 0 && c.Op != "M" {
					bad("invalid_geometry", p.ID, "writing.glyphs.paths.commands", "Use an initial M and bounded absolute M/L/Q/C/Z commands with exact coordinate counts.")
				}
				if c.Op == "L" || c.Op == "Q" || c.Op == "C" {
					drawing = true
				}
				for _, v := range c.Values {
					if !finiteVector(v) {
						bad("invalid_geometry", p.ID, "writing.glyphs.paths.commands", "Coordinates must be finite within ±8192.")
					}
				}
			}
			if !drawing {
				bad("missing_artwork", p.ID, "writing.glyphs.paths.commands", "Draw at least one line or curve; move commands alone do not form artwork.")
			}
		}
	}
	signs := map[string]ScriptSign{}
	allowed := map[string]bool{"phonetic": true, "syllabic": true, "logographic": true, "morphemic": true, "semantic": true, "classifier": true}
	for _, s := range w.Signs {
		if ctx.Err() != nil {
			return []Diagnostic{vectorDiagnostic("cancelled", "", "writing.signs", ctx.Err().Error())}
		}
		id("writing.signs", s.ID)
		signs[s.ID] = s
		if !glyphs[s.Glyph] {
			bad("missing_glyph", s.ID, "writing.signs.glyph", "Choose an existing glyph identity; no placeholder will be substituted.")
		}
		if len(s.Label) > 160 || len(s.Grapheme) > 256 {
			bad("definition_limit", s.ID, "writing.signs", "Reduce sign label/grapheme metadata to 160/256 bytes.")
		}
		roles := map[string]bool{}
		for _, r := range s.Roles {
			if !allowed[r] || roles[r] {
				bad("invalid_role", s.ID, "writing.signs.roles", "Use unique typed phonetic/syllabic/logographic/morphemic/semantic/classifier roles.")
			}
			roles[r] = true
		}
		for _, r := range s.Readings {
			id("writing.signs/"+s.ID+"/readings", r.ID)
			for _, k := range sortedFeatureKeys(r.When) {
				if k == "" || len(k) > 80 || len(r.When[k]) > 160 {
					bad("invalid_context", r.ID, "writing.signs.readings.when", "Use bounded explicit author context atoms.")
				}
			}
		}
		for _, a := range s.Associations {
			switch a.Kind {
			case "phoneme", "grapheme", "lexeme", "morpheme", "meaning", "classifier":
			default:
				bad("invalid_association", s.ID, "writing.signs.associations", "Choose a typed linguistic/semantic author annotation.")
			}
			if len(a.Value) > 512 {
				bad("definition_limit", s.ID, "writing.signs.associations", "Reduce the author annotation to 512 bytes.")
			}
		}
	}
	var check func([]SignUse, string, int)
	check = func(items []SignUse, path string, depth int) {
		for i, item := range items {
			if ctx.Err() != nil {
				bad("cancelled", "", path, ctx.Err().Error())
				return
			}
			p := fmt.Sprintf("%s.%d", path, i)
			for _, key := range sortedFeatureKeys(item.Context) {
				if key == "" || len(key) > 80 || len(item.Context[key]) > 160 {
					bad("invalid_context", item.Sign, p+".context", "Use bounded explicit author context atoms.")
				}
			}
			switch item.Kind {
			case "sign":
				s, ok := signs[item.Sign]
				if !ok {
					bad("missing_sign", item.Sign, p, "Reference an authored sign identity.")
				}
				if len(item.Children) > 0 {
					bad("invalid_sequence", item.Sign, p, "Sign nodes cannot contain children.")
				}
				if len(item.Roles) == 0 {
					bad("choice_required", item.Sign, p, "Choose the sign's roles explicitly.")
				}
				used := map[string]bool{}
				for _, r := range item.Roles {
					found := false
					for _, a := range s.Roles {
						if a == r {
							found = true
						}
					}
					if !found || used[r] {
						bad("invalid_role", item.Sign, p, "Selected roles must be unique authored roles of this sign.")
					}
					used[r] = true
				}
			case "group":
				if depth >= 4 || len(item.Children) == 0 || item.Sign != "" || item.Reading != "" || len(item.Roles) > 0 || len(item.Context) > 0 {
					bad("invalid_sequence", "", p, "Groups contain only ordered horizontal children, within depth 4.")
				}
				if depth < 4 {
					check(item.Children, p+".children", depth+1)
				}
			case "break":
				if depth > 0 || len(item.Children) > 0 || item.Sign != "" || item.Reading != "" || len(item.Roles) > 0 || len(item.Context) > 0 {
					bad("unsupported_layout", "", p, "Line breaks are explicit top-level nodes only.")
				}
			default:
				bad("unsupported_node", "", p, "Choose sign, ordered horizontal group, or explicit line break.")
			}
		}
	}
	for _, s := range w.Sequences {
		id("writing.sequences", s.ID)
		if len(s.Name) > 160 {
			bad("definition_limit", s.ID, "writing.sequences", "Reduce the sequence name to 160 bytes.")
		}
		check(s.Items, "writing.sequences/"+s.ID, 0)
	}
	return out
}
func CompileScript(ctx context.Context, d ScriptDefinition) Report {
	report := Report{Schema: d.Schema, Revision: Hash(d), Engine: VectorEngine, Adapter: VectorAdapter, Compiler: VectorCompiler, Kernel: VectorEngine, Status: "incomplete", Ready: map[string]bool{"render": false}, Capabilities: ScriptCapabilities()[1].Operations, Diagnostics: []Diagnostic{}}
	if _, err := DecodeScript(Canonical(d)); err != nil {
		report.Status = "invalid"
		report.Diagnostics = []Diagnostic{vectorDiagnostic("invalid_definition", "", "definition", err.Error())}
		return report
	}
	report.Diagnostics = ValidateWriting(ctx, d)
	if ctx.Err() != nil {
		report.Status = "cancelled"
		return report
	}
	if len(report.Diagnostics) > 0 {
		report.Status = "invalid"
	} else if len(d.Writing.Glyphs) > 0 && len(d.Writing.Signs) > 0 {
		report.Status = "compiled"
		report.Ready["render"] = true
	}
	return report
}
func number(v float64) string {
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}
func pathData(commands []PathCommand) string {
	var b strings.Builder
	for _, c := range commands {
		b.WriteString(c.Op)
		for _, v := range c.Values {
			b.WriteByte(' ')
			b.WriteString(number(v))
		}
		b.WriteByte(' ')
	}
	return strings.TrimSpace(b.String())
}

func RenderScript(ctx context.Context, d ScriptDefinition, q ScriptRenderRequest) (out ScriptResult) {
	out = ScriptResult{Status: "invalid_input", ScriptID: d.ScriptID, Revision: Hash(d), Engine: VectorEngine, Adapter: VectorAdapter, Compiler: VectorCompiler, Input: Clone(q), Signs: []RenderedSign{}, Trace: []ScriptTrace{}, Choices: []ReadingChoice{}, Diagnostics: []Diagnostic{}}
	fail := func(code, id, path, message string) ScriptResult {
		out.Status = code
		out.SVG = ""
		out.Signs = []RenderedSign{}
		if code == "execution_limit" || code == "cancelled" {
			out.Trace = []ScriptTrace{}
		}
		out.Diagnostics = []Diagnostic{vectorDiagnostic(code, id, path, message)}
		return out
	}
	if !CompatibleScriptVersions(d.Schema, q.Engine, q.Adapter, q.Compiler) {
		return fail("incompatible_engine", "", "request", "Pin vector-script/1, sign-layout/1 and vector-script-compiler/1; no fallback is applied.")
	}
	if q.Revision != out.Revision {
		return fail("uncompiled_revision", "", "request.revision", "Pin this exact immutable/draft Script revision.")
	}
	report := CompileScript(ctx, d)
	if !report.Ready["render"] {
		out.Status = "missing_dependency"
		if ctx.Err() != nil {
			out.Status = "cancelled"
		}
		out.Diagnostics = report.Diagnostics
		if len(out.Diagnostics) == 0 {
			out.Diagnostics = []Diagnostic{vectorDiagnostic("missing_dependency", "", "writing", "Author a glyph and a sign before rendering.")}
		}
		return out
	}
	if (q.SequenceID == "") == (q.Sequence == nil) {
		return fail("invalid_input", "", "request.sequence", "Choose exactly one stored sequence or explicit structured sequence.")
	}
	sequence := SignSequence{}
	if q.Sequence != nil {
		sequence = Clone(*q.Sequence)
	} else {
		found := false
		for _, s := range d.Writing.Sequences {
			if s.ID == q.SequenceID {
				sequence = s
				found = true
			}
		}
		if !found {
			return fail("missing_sequence", q.SequenceID, "request.sequenceId", "Choose a sequence in the pinned Script revision.")
		}
	}
	// Inline input obeys the same shape and semantic checks as authored sequences.
	check := Clone(d)
	check.Writing.Sequences = []SignSequence{sequence}
	if err := vectorShape(check.Writing); err != nil {
		return fail("execution_limit", "", "request.sequence", err.Error())
	}
	if diagnostics := ValidateWriting(ctx, check); len(diagnostics) > 0 {
		out.Status = diagnostics[0].Code
		out.Diagnostics = diagnostics
		return out
	}
	out.Sequence = sequence
	glyphs := map[string]Glyph{}
	signs := map[string]ScriptSign{}
	for _, g := range d.Writing.Glyphs {
		glyphs[g.ID] = g
	}
	for _, s := range d.Writing.Signs {
		signs[s.ID] = s
	}
	pen, baseline := 0.0, d.Writing.UnitsPerEm
	nodes, steps := 0, 0
	var failure *Diagnostic
	var layout func([]SignUse, string)
	layout = func(items []SignUse, path string) {
		for i, item := range items {
			steps++
			nodes++
			p := fmt.Sprintf("%s.%d", path, i)
			if ctx.Err() != nil {
				v := vectorDiagnostic("cancelled", "", p, ctx.Err().Error())
				failure = &v
				return
			}
			if steps > 100000 || nodes > VectorMaxSigns {
				v := vectorDiagnostic("execution_limit", "", p, "Rendering exceeds 512 sequence nodes or 100,000 steps; no partial SVG is returned.")
				failure = &v
				return
			}
			if item.Kind == "break" {
				pen = 0
				baseline += d.Writing.LineHeight
				out.Trace = append(out.Trace, ScriptTrace{Stage: "line-break", Path: p, Baseline: baseline})
				continue
			}
			if item.Kind == "group" {
				out.Trace = append(out.Trace, ScriptTrace{Stage: "group-start", Path: p, X: pen, Baseline: baseline})
				layout(item.Children, p+".children")
				if failure != nil {
					return
				}
				out.Trace = append(out.Trace, ScriptTrace{Stage: "group-end", Path: p, X: pen, Baseline: baseline})
				continue
			}
			s := signs[item.Sign]
			g := glyphs[s.Glyph]
			eligible := []string{}
			var reading *SignReading
			for ri, r := range s.Readings {
				steps++
				match := true
				for _, key := range sortedFeatureKeys(r.When) {
					steps++
					if steps > 100000 || ctx.Err() != nil {
						code := "execution_limit"
						message := "Reading resolution exceeded 100,000 steps."
						if ctx.Err() != nil {
							code = "cancelled"
							message = ctx.Err().Error()
						}
						v := vectorDiagnostic(code, s.ID, p, message)
						failure = &v
						return
					}
					if item.Context[key] != r.When[key] {
						match = false
					}
				}
				if match {
					eligible = append(eligible, r.ID)
					if r.ID == item.Reading {
						reading = &s.Readings[ri]
					}
				}
			}
			silent := true
			for _, role := range item.Roles {
				if role != "classifier" && role != "semantic" {
					silent = false
				}
			}
			if item.Reading == "" && !silent {
				out.Choices = append(out.Choices, ReadingChoice{Path: p, Sign: s.ID, Readings: eligible})
				code := "choice_required"
				if len(eligible) == 0 {
					code = "unresolved_reading"
				}
				v := vectorDiagnostic(code, s.ID, p+".reading", "Choose an authored reading explicitly; the engine never guesses.")
				failure = &v
				return
			}
			if item.Reading != "" && reading == nil {
				out.Choices = append(out.Choices, ReadingChoice{Path: p, Sign: s.ID, Readings: eligible})
				v := vectorDiagnostic("unresolved_reading", s.ID, p+".reading", "The selected reading does not exist or its authored context does not match.")
				failure = &v
				return
			}
			advance := g.Advance*(d.Writing.UnitsPerEm/g.ViewBox[3])*g.Scale + d.Writing.Spacing
			x := pen
			if d.Direction == "rtl" {
				x -= advance
				pen -= advance
			} else {
				pen += advance
			}
			text := ""
			if reading != nil {
				text = reading.Text
			}
			out.Signs = append(out.Signs, RenderedSign{Path: p, Sign: s.ID, Glyph: g.ID, Reading: item.Reading, Text: text, Grapheme: s.Grapheme, Roles: append([]string{}, item.Roles...), Associations: Clone(s.Associations), X: x, Baseline: baseline, Scale: d.Writing.UnitsPerEm / g.ViewBox[3] * g.Scale, Advance: advance})
			out.Trace = append(out.Trace, ScriptTrace{Stage: "resolve-sign", Path: p, Sign: s.ID, Glyph: g.ID, Reading: item.Reading, Roles: append([]string{}, item.Roles...), X: x, Baseline: baseline})
		}
	}
	layout(sequence.Items, "sequence.items")
	if failure != nil {
		out.Status = failure.Code
		out.Diagnostics = []Diagnostic{*failure}
		out.Signs = []RenderedSign{}
		return out
	}
	var content strings.Builder
	minX, minY, maxX, maxY := 0.0, 0.0, 1.0, d.Writing.LineHeight
	for _, placed := range out.Signs {
		g := glyphs[placed.Glyph]
		sx := placed.X + g.Bearing*placed.Scale - g.ViewBox[0]*placed.Scale
		sy := placed.Baseline - g.Baseline*placed.Scale
		minX = math.Min(minX, sx+g.ViewBox[0]*placed.Scale)
		minY = math.Min(minY, sy+g.ViewBox[1]*placed.Scale)
		maxX = math.Max(maxX, math.Max(placed.X+placed.Advance, sx+(g.ViewBox[0]+g.ViewBox[2])*placed.Scale))
		maxY = math.Max(maxY, sy+(g.ViewBox[1]+g.ViewBox[3])*placed.Scale)
		fmt.Fprintf(&content, `<g data-sign="%s" data-glyph="%s" transform="translate(%s %s) scale(%s)">`, html.EscapeString(placed.Sign), html.EscapeString(placed.Glyph), number(sx), number(sy), number(placed.Scale))
		for _, p := range g.Paths {
			margin := p.StrokeWidth * placed.Scale / 2
			for range p.Commands {
				steps++
				if ctx.Err() != nil {
					return fail("cancelled", g.ID, "writing.glyphs", ctx.Err().Error())
				}
				if steps > 100000 {
					return fail("execution_limit", g.ID, "writing.glyphs", "Rendering exceeds 100,000 geometry steps.")
				}
			}
			for _, command := range p.Commands {
				for i := 0; i+1 < len(command.Values); i += 2 {
					px, py := sx+command.Values[i]*placed.Scale, sy+command.Values[i+1]*placed.Scale
					minX = math.Min(minX, px-margin)
					minY = math.Min(minY, py-margin)
					maxX = math.Max(maxX, px+margin)
					maxY = math.Max(maxY, py+margin)
				}
			}
			fmt.Fprintf(&content, `<path d="%s" fill="%s" stroke="%s" stroke-width="%s"/>`, pathData(p.Commands), p.Fill, p.Stroke, number(p.StrokeWidth))
		}
		content.WriteString("</g>")
		if content.Len() > 2<<20 || len(Canonical(out.Trace)) > 1<<20 {
			return fail("execution_limit", g.ID, "render", "SVG exceeds 2 MiB or trace exceeds 1 MiB.")
		}
	}
	pad := d.Writing.UnitsPerEm * .25
	out.SVG = fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="%s %s %s %s" role="img" aria-label="%s"><title>%s</title>%s</svg>`, number(minX-pad), number(minY-pad), number(maxX-minX+pad*2), number(maxY-minY+pad*2), html.EscapeString(sequence.Name), html.EscapeString(sequence.Name), content.String())
	out.Status = "success"
	if len(Canonical(out)) > 8<<20 {
		return fail("execution_limit", "", "render", "Rendered result exceeds the 8 MiB payload budget.")
	}
	return out
}
