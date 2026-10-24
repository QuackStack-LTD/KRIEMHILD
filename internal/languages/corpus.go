package languages

import (
	"context"
	"kriemhild/internal/languages/spike"
)

type CaseResult struct {
	ID             string  `json:"id"`
	Passed         bool    `json:"passed"`
	ExpectedStatus string  `json:"expectedStatus"`
	Result         *Result `json:"result"`
}

func (a *Artifact) checkCorpus() []Diagnostic {
	diagnostics := []Diagnostic{}
	seen := map[string]bool{}
	for _, c := range a.definition.Tests {
		if c.ID == "" || len(c.ID) > 80 || seen[c.ID] || c.Request.Revision != "" || c.Request.Operation == "regression" || c.Request.Engine != EngineVersion || c.Request.Adapter != AdapterVersion || c.ExpectedStatus == "" || len(c.Expected) > 256 || len(c.ExpectedSentences) > 256 || len(c.Notes) > 2000 {
			diagnostics = append(diagnostics, Diagnostic{Code: "invalid_regression", Severity: "error", Module: "tests", RuleID: c.ID, Path: "tests", Message: "Use a unique bounded case ID, exact engine/adapter, an empty revision binding for this definition, a nonrecursive operation and explicit expectations."})
		}
		seen[c.ID] = true
	}
	a.report.Ready["regression"] = len(a.definition.Tests) > 0 && len(diagnostics) == 0
	return diagnostics
}
func (a *Artifact) runCorpus(ctx context.Context, q Request, out Result) Result {
	fail := func(code, message string) Result {
		out.Status = code
		out.Diagnostics = []Diagnostic{{Code: code, Severity: "error", Module: "tests", Path: "request", Message: message}}
		return out
	}
	if !a.report.Ready["regression"] {
		return fail("missing_dependency", "Add valid authored regression cases first.")
	}
	if q.Policy != "explicit" || q.Seed != "" || q.Limit < 1 || q.Limit > 256 || q.Lexeme != "" || len(q.Features) > 0 || q.Template != "" || len(q.Segments) > 0 || q.Text != "" || q.Construction != "" || len(q.Roles) > 0 {
		return fail("invalid_input", "A regression run takes only its exact revision, engine/adapter, explicit policy and result limit.")
	}
	out.Status = "success"
	bytes := 0
	for _, test := range a.definition.Tests {
		if ctx.Err() != nil {
			return fail("cancelled", ctx.Err().Error())
		}
		request := Clone(test.Request)
		request.Revision = a.report.Revision
		r := a.Execute(ctx, request)
		bytes += len(Canonical(r))
		if bytes > 8<<20 {
			out.Tests = nil
			out.KnownValid = 0
			return fail("execution_limit", "Regression output exceeded its 8 MiB budget; narrow the corpus.")
		}
		// Normalize nil/empty sequences in expectations without weakening exact
		// comparison of the authored outputs, features, rule IDs or trace order.
		expected := test.Expected
		if expected == nil {
			expected = []spike.Candidate{}
		}
		actual := r.Candidates
		if actual == nil {
			actual = []spike.Candidate{}
		}
		diagnostics := r.Diagnostics
		if len(diagnostics) == 0 {
			diagnostics = nil
		}
		passed := r.Status == test.ExpectedStatus && string(Canonical(actual)) == string(Canonical(expected)) && string(Canonical(r.Sentences)) == string(Canonical(test.ExpectedSentences)) && string(Canonical(diagnostics)) == string(Canonical(test.ExpectedDiagnostics))
		out.Tests = append(out.Tests, CaseResult{ID: test.ID, Passed: passed, ExpectedStatus: test.ExpectedStatus, Result: &r})
		out.Examined++
		if passed {
			out.KnownValid++
		} else {
			out.Status = "regression_failure"
			out.Rejected++
		}
	}
	return out
}
