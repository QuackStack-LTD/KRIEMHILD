package languages

// Sequence corpora are external verification data. They deliberately do not
// extend Definition.Tests, change source hashes, or own a mutable language head.
import (
	"context"
	"fmt"
	"kriemhild/internal/languages/spike"
	"reflect"
	"strings"
	"unicode/utf8"
)

const SequenceCorpusVersion = "sequence-corpus/1"

type CorpusReview struct {
	Status     string `json:"status"`
	Reviewer   string `json:"reviewer"`
	Provenance string `json:"provenance"`
}
type CorpusPin struct {
	LanguageID       string    `json:"languageId"`
	Revision         string    `json:"revision"`
	Schema           int       `json:"schema"`
	Canonicalizer    string    `json:"canonicalizer"`
	Engine           string    `json:"engine"`
	Adapter          string    `json:"adapter"`
	Compiler         string    `json:"compiler"`
	Segmentation     string    `json:"segmentation"`
	Normalizer       string    `json:"normalizer"`
	Script           ScriptRef `json:"script"`
	Renderer         string    `json:"renderer"`
	RendererAdapter  string    `json:"rendererAdapter"`
	RendererCompiler string    `json:"rendererCompiler"`
}
type CorpusBounds struct {
	Calls       int `json:"calls"`
	Work        int `json:"work"`
	ReportBytes int `json:"reportBytes"`
}
type CorpusDomain struct {
	Alphabet  []OrthoUnit `json:"alphabet"`
	MaxLength int         `json:"maxLength"`
}
type SequenceCase struct {
	ID          string        `json:"id"`
	Description string        `json:"description"`
	Tags        []string      `json:"tags"`
	Review      CorpusReview  `json:"review"`
	Pin         CorpusPin     `json:"pin"`
	InputLayer  string        `json:"inputLayer"`
	Request     Request       `json:"request"`
	Outcome     string        `json:"outcome"`
	Comparison  string        `json:"comparison"`
	Expected    *Result       `json:"expected"`
	Domain      *CorpusDomain `json:"domain,omitempty"`
}
type SequenceCorpus struct {
	Version string         `json:"version"`
	ID      string         `json:"id"`
	Bounds  CorpusBounds   `json:"bounds"`
	Cases   []SequenceCase `json:"cases"`
}

// CorpusResolver must resolve immutable bodies under an explicit ownership
// binding. Implementations must never substitute a current active/draft head.
type CorpusResolver func(context.Context, string, string, string) ([]byte, error)

func DecodeSequenceCorpus(raw []byte) (SequenceCorpus, error) {
	var c SequenceCorpus
	if err := spike.DecodeJSON(raw, &c); err != nil {
		return c, err
	}
	return c, ValidateSequenceCorpus(c)
}
func corpusError(id, message string) error { return fmt.Errorf("sequence_corpus %s: %s", id, message) }
func validCorpusText(s string, n int) bool { return strings.TrimSpace(s) != "" && len(s) <= n }
func CorpusOutcome(status string) string {
	switch status {
	case "success":
		return "success"
	case "ambiguous", "choice_required":
		return "ambiguous"
	case "no_result", "no_mapping", "no_valid_candidate":
		return "no_result"
	case "unsupported", "unsupported_inverse", "unsupported_normalization", "unsupported_operation", "missing_dependency", "not_ready":
		return "unsupported"
	case "cancelled":
		return "cancelled"
	case "execution_limit", "candidate_limit", "limit_reached":
		return "limit_reached"
	default:
		return "rejected"
	}
}
func validCorpusPin(p CorpusPin) bool {
	return ValidID(p.LanguageID) && ValidHash(p.Revision) && p.Schema == 4 &&
		p.Canonicalizer == CanonicalVersion && p.Engine == SequenceEngine && p.Adapter == SequenceAdapter && p.Compiler == SequenceCompiler &&
		p.Segmentation == GraphemeVersion && p.Normalizer == NormalizationVersion && ValidID(p.Script.ID) && ValidHash(p.Script.Revision) &&
		p.Renderer == VectorEngine && p.RendererAdapter == VectorAdapter && p.RendererCompiler == VectorCompiler
}
func validCorpusUnits(units []OrthoUnit) bool {
	if len(units) > 512 {
		return false
	}
	for i, u := range units {
		if len(u.Value) > 4096 {
			return false
		}
		switch u.Kind {
		case "segment", "grapheme":
			if u.Value == "" || u.Sign != nil {
				return false
			}
		case "boundary":
			if u.Value != "morpheme" || u.Sign != nil || i == 0 || i == len(units)-1 || units[i-1].Kind == "boundary" {
				return false
			}
		case "sign":
			if u.Sign == nil || u.Value != "" || len(u.Sign.Children) > 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
func ValidateSequenceCorpus(c SequenceCorpus) error {
	if !corpusStringsValid(reflect.ValueOf(c)) {
		return corpusError(c.ID, "invalid UTF-8 or string exceeds 64 KiB")
	}
	if c.Version != SequenceCorpusVersion || !validCorpusText(c.ID, 80) || len(c.Cases) == 0 || len(c.Cases) > 32 {
		return corpusError(c.ID, "require sequence-corpus/1, an ID and 1–32 cases")
	}
	if c.Bounds.Calls < 1 || c.Bounds.Calls > 128 || c.Bounds.Work < 1 || c.Bounds.Work > 100000000 || c.Bounds.ReportBytes < 1024+len(c.Cases)*1024 || c.Bounds.ReportBytes > 2<<20 {
		return corpusError(c.ID, "invalid calls/work/reportBytes bounds (max 128/100000000/2097152)")
	}
	seen := map[string]bool{}
	pins := map[string]bool{}
	total := 0
	for _, test := range c.Cases {
		bad := func(s string) error { return corpusError(test.ID, s) }
		if !validCorpusText(test.ID, 80) || seen[test.ID] || !validCorpusText(test.Description, 1000) || len(test.Tags) > 16 {
			return bad("invalid/duplicate case identity, description or tags")
		}
		seen[test.ID] = true
		for _, tag := range test.Tags {
			if !validCorpusText(tag, 80) {
				return bad("invalid tag")
			}
		}
		if (test.Review.Status != "pending" && test.Review.Status != "reviewed") || !validCorpusText(test.Review.Provenance, 2000) || len(test.Review.Reviewer) > 256 || test.Review.Status == "reviewed" && !validCorpusText(test.Review.Reviewer, 256) {
			return bad("explicit review status, provenance and reviewer for reviewed expectations required")
		}
		if !validCorpusPin(test.Pin) {
			return bad("missing or incompatible exact language, Unicode, Script or renderer pins")
		}
		pins[test.Pin.Revision] = true
		if len(pins) > 8 {
			return bad("at most eight language revisions")
		}
		q := test.Request
		if q.Operation != "spell" && q.Operation != "unspell" && q.Operation != "graphemes" {
			return bad("only production sequence operations supported")
		}
		if q.Revision != test.Pin.Revision || q.Engine != test.Pin.Engine || q.Adapter != test.Pin.Adapter || q.Compiler != test.Pin.Compiler || q.Policy != "all" || q.Seed != "" || q.Limit < 1 || q.Limit > 128 || len(Canonical(q)) > 16384 || len(q.Text) > 4096 || len(q.Segments) > 256 {
			return bad("request contradicts exact pins/policy or exceeds input limits")
		}
		if test.InputLayer != "surface" && test.InputLayer != "surface-boundaries" && test.InputLayer != "grapheme" && test.InputLayer != "sign" && test.InputLayer != "unicode" {
			return bad("declare an explicit input layer")
		}
		if q.Spelling != nil && !validCorpusUnits(q.Spelling.Units) {
			return bad("invalid typed units or morpheme boundary topology")
		}
		if q.Operation == "graphemes" && test.InputLayer != "unicode" {
			return bad("graphemes uses the unicode input layer")
		}
		if test.Comparison != "exact" && test.Comparison != "unordered-preimages" {
			return bad("comparison must be exact or unordered-preimages")
		}
		if test.Comparison == "unordered-preimages" && q.Operation != "unspell" {
			return bad("unordered preimages only applies to inverse")
		}
		e := test.Expected
		if e == nil || len(e.Tests) > 0 || string(Canonical(e.Input)) != string(Canonical(q)) || string(Canonical(e.Request)) != string(Canonical(q.Request)) || e.LanguageID != test.Pin.LanguageID || e.Canonicalizer != test.Pin.Canonicalizer || e.Revision != test.Pin.Revision || e.Engine != test.Pin.Engine || e.Adapter != test.Pin.Adapter || e.Compiler != test.Pin.Compiler || e.Schema != 4 || test.Outcome != CorpusOutcome(e.Status) {
			return bad("expectation identity/request/status inconsistent")
		}
		if e.Status == "" || len(Canonical(e)) > 256<<10 {
			return bad("missing status or expectation exceeds 256 KiB")
		}
		total += len(Canonical(e))
		if total > 1<<20 {
			return bad("total expectations exceed 1 MiB")
		}
		if err := validateCorpusResult(*e); err != nil {
			return bad(err.Error())
		}
		if e.Spelling != nil && (e.Spelling.Version != SequenceVersion || e.Spelling.Segmentation != test.Pin.Segmentation || e.Spelling.Normalizer != test.Pin.Normalizer || q.Operation != "graphemes" && (e.Spelling.Script != test.Pin.Script || e.Spelling.Renderer != test.Pin.Renderer || e.Spelling.RendererAdapter != test.Pin.RendererAdapter || e.Spelling.RendererCompiler != test.Pin.RendererCompiler)) {
			return bad("expectation contradicts Unicode/Script/renderer pins")
		}
		if test.Domain != nil {
			if q.Operation != "spell" || test.Comparison != "exact" || test.Outcome != "success" || len(test.Domain.Alphabet) == 0 || len(test.Domain.Alphabet) > 8 || test.Domain.MaxLength < 0 || test.Domain.MaxLength > 3 {
				return bad("exhaustive domain requires raw empty spell, success, exact comparison and bounded alphabet/length")
			}
			count, level := 1, 1
			units := map[string]bool{}
			for _, u := range test.Domain.Alphabet {
				key := string(Canonical(u))
				if u.Kind != "segment" || !validCorpusUnits([]OrthoUnit{u}) || units[key] {
					return bad("domain alphabet requires unique segment units")
				}
				units[key] = true
			}
			for i := 0; i < test.Domain.MaxLength; i++ {
				level *= len(test.Domain.Alphabet)
				count += level
			}
			if count > 64 || count*2+1 > c.Bounds.Calls {
				return bad("domain exceeds 64 streams or declared call budget")
			}
		}
	}
	return nil
}

func corpusStringsValid(v reflect.Value) bool {
	if v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		return v.IsNil() || corpusStringsValid(v.Elem())
	}
	switch v.Kind() {
	case reflect.String:
		return v.Len() <= 65536 && utf8.ValidString(v.String())
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !corpusStringsValid(v.Field(i)) {
				return false
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if !corpusStringsValid(v.Index(i)) {
				return false
			}
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			if !corpusStringsValid(iter.Key()) || !corpusStringsValid(iter.Value()) {
				return false
			}
		}
	}
	return true
}

func validateCorpusResult(r Result) error {
	switch r.Status {
	case "success", "ambiguous", "choice_required", "no_result", "no_mapping", "no_valid_candidate", "unsupported", "unsupported_inverse", "unsupported_normalization", "unsupported_operation", "missing_dependency", "not_ready", "cancelled", "execution_limit", "candidate_limit", "invalid_input", "invalid_unicode", "invalid_rule", "invalid_reference", "invalid_unit", "invalid_selection", "invalid_policy", "unknown_segment", "missing_glyph", "missing_reading":
	default:
		return fmt.Errorf("unknown expected engine status")
	}
	if r.Spelling == nil {
		if r.Status == "success" || r.Status == "ambiguous" {
			return fmt.Errorf("successful sequence expectations require structured spelling output")
		}
		return nil
	} // Rejections before spelling initialization.
	s := r.Spelling
	if (r.Status == "success" || r.Status == "ambiguous") && (r.Input.Operation == "spell" && len(s.Spellings) == 0 || r.Input.Operation == "unspell" && len(s.Preimages) == 0) {
		return fmt.Errorf("successful spelling/inverse expectation cannot omit its complete result set")
	}
	if len(s.Spellings) > 128 || len(s.Preimages) > 128 || (CorpusOutcome(r.Status) == "limit_reached" || r.Status == "cancelled") && (len(s.Spellings) > 0 || len(s.Preimages) > 0) {
		return fmt.Errorf("invalid result count or partial result labelled complete")
	}
	clusters := func(text string, c []GraphemeCluster) error {
		actual, err := SegmentGraphemes(context.Background(), text)
		if err != nil {
			return err
		}
		if len(c) > 0 && string(Canonical(c)) != string(Canonical(actual)) || len(text) > 0 && len(c) == 0 {
			return fmt.Errorf("invalid grapheme/byte/UTF-16/code-point spans")
		}
		return nil
	}
	if r.Input.Operation == "graphemes" {
		if err := clusters(r.Input.Text, s.Clusters); err != nil {
			return err
		}
		if s.Selection != nil && (s.Selection.Start < 0 || s.Selection.End < s.Selection.Start || s.Selection.End > len(s.Clusters)) {
			return fmt.Errorf("invalid cluster selection")
		}
	}
	runs := append([]SpellingRun{}, s.Spellings...)
	for _, p := range s.Preimages {
		if !validCorpusUnits(p.Units) {
			return fmt.Errorf("invalid inverse boundary topology")
		}
		runs = append(runs, p.Forward)
	}
	for _, run := range runs {
		if len(run.Stages) > 8 || !validCorpusUnits(run.Input) {
			return fmt.Errorf("invalid expected stages or input units")
		}
		if err := clusters(run.Text, run.Clusters); err != nil {
			return err
		}
		for _, stage := range run.Stages {
			if stage.OffsetUnit != offsetUnit(stage.InputLayer) {
				return fmt.Errorf("incorrect named stage offset unit")
			}
			for _, m := range stage.Matches {
				if m.Start < 0 || m.End <= m.Start || m.End > len(stage.Before) || m.Rule == "" {
					return fmt.Errorf("invalid rule match span")
				}
			}
			for _, v := range stage.After {
				for _, i := range v.Sources {
					if i < 0 || i >= len(run.Input) {
						return fmt.Errorf("invalid source-token provenance span")
					}
				}
			}
		}
	}
	return nil
}
