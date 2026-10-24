package languages

import (
	"context"
	"golang.org/x/text/unicode/norm"
	"kriemhild/internal/languages/spike"
	"strings"
)

// write transcribes an explicitly supplied surface segment sequence. It does
// not reapply phonological rules or require a lexical/morphological module.
func (a *Artifact) write(ctx context.Context, q Request, out Result) Result {
	fail := func(code, message string) Result {
		out.Status = code
		out.Candidates = []spike.Candidate{}
		out.KnownValid = 0
		out.Diagnostics = []Diagnostic{{Code: code, Severity: "error", Module: "orthography", Path: "request.segments", Message: message}}
		return out
	}
	if !a.report.Ready["write"] || a.definition.Orthography == nil {
		return fail("missing_dependency", "Compile a valid pinned script, mapping and sound inventory first.")
	}
	if q.Lexeme != "" || len(q.Features) > 0 || q.Template != "" || q.Text != "" || q.Construction != "" || len(q.Roles) > 0 {
		return fail("invalid_input", "Writing takes an explicit surface segment sequence and selection policy only.")
	}
	in := Clone(q.Request)
	in.Operation = "validate"
	in.Engine = spike.EngineVersion
	in.Revision = a.phonology.Report().Revision
	r := a.phonology.Execute(ctx, in)
	out.Result = r
	out.Revision = a.report.Revision
	out.Engine = a.report.Engine
	out.Request = q.Request
	if r.Status != "success" {
		return out
	}
	mappings := map[string]spike.Mapping{}
	for _, m := range a.definition.Orthography.Mappings {
		mappings[m.Segment] = m
	}
	form := r.Candidates[0]
	chunks := []string{}
	for _, s := range q.Segments {
		m, ok := mappings[s]
		if !ok {
			return fail("missing_dependency", "No authored grapheme mapping exists for segment "+s)
		}
		chunks = append(chunks, m.Grapheme)
		form.Trace = append(form.Trace, spike.Trace{Stage: "orthography", RuleID: m.ID, Before: []string{s}, After: []string{m.Grapheme}})
	}
	form.Written = strings.Join(chunks, "")
	before := form.Written
	switch a.definition.Orthography.Normalization {
	case "NFC":
		form.Written = norm.NFC.String(form.Written)
	case "NFD":
		form.Written = norm.NFD.String(form.Written)
	}
	if len(form.Written) > 4096 {
		return fail("execution_limit", "Written output exceeds the 4 KiB form budget.")
	}
	if form.Written != before {
		form.Trace = append(form.Trace, spike.Trace{Stage: "normalization", RuleID: a.definition.Orthography.Normalization, Before: chunks, After: []string{form.Written}})
	}
	out.Candidates = []spike.Candidate{form}
	return out
}
