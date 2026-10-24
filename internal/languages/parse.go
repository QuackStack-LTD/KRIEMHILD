package languages

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"kriemhild/internal/languages/spike"
	"sort"
	"strings"
)

func surface(c spike.Candidate) string {
	if c.Written != "" {
		return c.Written
	}
	return strings.Join(c.Segments, " ")
}
func (a *Artifact) parse(ctx context.Context, q Request, out Result) Result {
	fail := func(code, message string) Result {
		out.Status = code
		out.Sentences = nil
		out.KnownValid = 0
		out.Diagnostics = []Diagnostic{{Code: code, Severity: "error", Module: "syntax", Path: "request", Message: message}}
		return out
	}
	if !a.report.Ready["parse"] {
		return fail("missing_dependency", "Compile valid constructions and morphological dependencies before parsing.")
	}
	if q.Lexeme != "" || len(q.Features) > 0 || q.Template != "" || len(q.Segments) > 0 || len(q.Roles) > 0 || len(q.Text) > 4096 || q.Limit < 1 || q.Limit > 256 || (q.Policy != "all" && q.Policy != "explicit" && q.Policy != "seeded") || q.Policy == "seeded" && q.Seed == "" || q.Policy != "seeded" && q.Seed != "" {
		return fail("invalid_input", "Parsing requires bounded text, an optional construction ID and an explicit valid selection policy.")
	}
	if q.Construction != "" {
		if _, ok := a.grammar[q.Construction]; !ok {
			return fail("invalid_input", "Unknown construction.")
		}
	}
	ids := []string{}
	for id := range a.grammar {
		if q.Construction == "" || id == q.Construction {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	work, prepared, bytes := 0, 0, 0
	for _, id := range ids {
		c := a.grammar[id]
		options := map[string][]spike.Candidate{}
		for _, role := range c.Roles {
			for _, lex := range a.definition.Lexicon {
				if ctx.Err() != nil {
					return fail("cancelled", ctx.Err().Error())
				}
				if lex.Category != role.Category {
					continue
				}
				request := Request{Request: spike.Request{Revision: a.report.Revision, Engine: EngineVersion, Operation: "inflect", Lexeme: lex.ID, Features: role.Features, Policy: "all", Limit: 256}, Adapter: AdapterVersion}
				r := a.Execute(ctx, request)
				if r.Status != "success" && r.Status != "ambiguous" && r.Status != "no_valid_candidate" {
					return fail(r.Status, "A role's morphological alternatives could not be completely enumerated.")
				}
				for _, form := range r.Candidates {
					prepared++
					bytes += len(Canonical(form))
					if prepared > 10000 || bytes > 8<<20 {
						return fail("execution_limit", "Parsing exceeded 10,000 prepared role forms or its 8 MiB payload budget.")
					}
					options[role.ID] = append(options[role.ID], form)
				}
			}
		}
		var walk func(int, string, map[string]spike.Candidate) bool
		walk = func(position int, remaining string, bound map[string]spike.Candidate) bool {
			work++
			if work > 10000 || ctx.Err() != nil {
				return false
			}
			if position == len(c.Order) {
				if remaining != "" {
					return true
				}
				for _, eq := range c.Agreements {
					for _, key := range eq.Features {
						l, lok := bound[eq.LeftRole].Features[key]
						r, rok := bound[eq.RightRole].Features[key]
						if !lok || !rok || l != r {
							return true
						}
					}
				}
				s := Sentence{Construction: c.ID, Roles: Clone(bound), Tokens: []string{}, Trace: []spike.Trace{}}
				for _, part := range c.Order {
					if part.Role != "" {
						s.Tokens = append(s.Tokens, surface(bound[part.Role]))
					} else {
						s.Tokens = append(s.Tokens, part.Literal)
					}
				}
				s.Written = strings.Join(s.Tokens, c.Separator)
				a.gloss(&s, c)
				for _, eq := range c.Agreements {
					s.Trace = append(s.Trace, spike.Trace{Stage: "agreement", RuleID: eq.ID, Before: []string{eq.LeftRole, eq.RightRole}, After: Clone(eq.Features)})
				}
				out.Sentences = append(out.Sentences, s)
				bytes += len(Canonical(s))
				return bytes <= 8<<20
			}
			if position > 0 {
				if !strings.HasPrefix(remaining, c.Separator) {
					return true
				}
				remaining = strings.TrimPrefix(remaining, c.Separator)
			}
			part := c.Order[position]
			if part.Role == "" {
				if !strings.HasPrefix(remaining, part.Literal) {
					return true
				}
				return walk(position+1, strings.TrimPrefix(remaining, part.Literal), bound)
			}
			forms := options[part.Role]
			if old, ok := bound[part.Role]; ok {
				forms = []spike.Candidate{old}
			}
			for _, form := range forms {
				word := surface(form)
				if !strings.HasPrefix(remaining, word) {
					continue
				}
				next := Clone(bound)
				next[part.Role] = form
				if !walk(position+1, strings.TrimPrefix(remaining, word), next) {
					return false
				}
			}
			return true
		}
		if !walk(0, q.Text, map[string]spike.Candidate{}) {
			if ctx.Err() != nil {
				return fail("cancelled", ctx.Err().Error())
			}
			return fail("execution_limit", "Complete parsing exceeded the 10,000-step or 8 MiB budget; no partial candidate set is selected.")
		}
	}
	out.Examined = work
	sort.Slice(out.Sentences, func(i, j int) bool { return string(Canonical(out.Sentences[i])) < string(Canonical(out.Sentences[j])) })
	out.KnownValid = len(out.Sentences)
	if len(out.Sentences) == 0 {
		out.Status = "no_valid_candidate"
		return out
	}
	out.Status = "success"
	out.Selection = "sha256-canonical-sentences/1"
	if q.Policy == "seeded" {
		sum := sha256.Sum256(Canonical(struct {
			Version, Seed, Revision, Operation string
			Candidates                         []Sentence
		}{out.Selection, q.Seed, a.report.Revision, q.Operation, out.Sentences}))
		index := binary.BigEndian.Uint64(sum[:8]) % uint64(len(out.Sentences))
		out.Sentences = []Sentence{out.Sentences[index]}
	} else if len(out.Sentences) > 1 {
		out.Status = "ambiguous"
		if q.Policy == "explicit" {
			out.Status = "choice_required"
		}
	}
	if len(out.Sentences) > q.Limit {
		out.Sentences = out.Sentences[:q.Limit]
		out.Truncated = true
		out.Status = "candidate_limit"
	}
	return out
}
