// Package spike is the isolated Phase 0 linguistic-engine experiment.
// It is not wired into product routes, persisted Worlds, or a stable public API.
package spike

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	SchemaVersion    = 1
	EngineVersion    = "languages-spike/1"
	CanonicalVersion = "kriemhild-typed-json/1"
	SelectionVersion = "sha256-canonical-candidates/1"
)

type Features map[string]string
type Feature struct {
	ID     string   `json:"id"`
	Values []string `json:"values"`
}
type Segment struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Phonemic bool     `json:"phonemic"`
	Features Features `json:"features"`
}
type NaturalClass struct {
	ID       string   `json:"id"`
	Features Features `json:"features"`
}

// Exactly one selector is required. Boundary currently supports word edges only.
type Match struct {
	Segment  string `json:"segment,omitempty"`
	Class    string `json:"class,omitempty"`
	Boundary string `json:"boundary,omitempty"`
}
type Pattern struct {
	ID    string  `json:"id"`
	Slots []Match `json:"slots"`
}
type SoundRule struct {
	ID     string   `json:"id"`
	Domain string   `json:"domain"`
	Input  []Match  `json:"input"`
	Output []string `json:"output"`
	Left   []Match  `json:"left"`
	Right  []Match  `json:"right"`
}
type Phonology struct {
	Features  []Feature      `json:"features"`
	Segments  []Segment      `json:"segments"`
	Classes   []NaturalClass `json:"classes"`
	Templates []Pattern      `json:"templates"`
	Allowed   []Pattern      `json:"allowed"`
	Forbidden []Pattern      `json:"forbidden"`
	Rules     []SoundRule    `json:"rules"` // Applied once each, in authored array order.
}
type Sense struct {
	ID      string `json:"id"`
	Meaning string `json:"meaning"`
}
type Lexeme struct {
	ID         string     `json:"id"`
	Category   string     `json:"category"`
	Senses     []Sense    `json:"senses"`
	Underlying []string   `json:"underlying"`
	Features   Features   `json:"features"`
	Paradigm   string     `json:"paradigm"`
	Overrides  []Override `json:"overrides"`
}
type Override struct {
	Cell     string   `json:"cell"`
	RuleID   string   `json:"ruleId"`
	Segments []string `json:"segments"`
}
type Operation struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"` // prefix, suffix, zero; other mechanisms fail explicitly.
	Segments []string `json:"segments"`
}
type Cell struct {
	ID         string      `json:"id"`
	Features   Features    `json:"features"`
	Operations []Operation `json:"operations"`
}
type Paradigm struct {
	ID    string `json:"id"`
	Cells []Cell `json:"cells"`
}
type Morphology struct {
	Features  []Feature  `json:"features"`
	Paradigms []Paradigm `json:"paradigms"`
}
type Mapping struct {
	ID       string `json:"id"`
	Segment  string `json:"segment"`
	Grapheme string `json:"grapheme"` // An authored string; never split into code points.
}

// Embedded fixture mapping only. Product scripts will be independent World
// identities with pinned immutable revisions, as described in ARCHITECTURE.md.
type Orthography struct {
	ID            string    `json:"id"`
	Normalization string    `json:"normalization"`
	Mappings      []Mapping `json:"mappings"`
}
type Definition struct {
	Schema      int          `json:"schema"`
	LanguageID  string       `json:"languageId"`
	Name        string       `json:"name"`
	Notes       string       `json:"notes"`
	Phonology   *Phonology   `json:"phonology"`
	Lexicon     []Lexeme     `json:"lexicon"`
	Morphology  *Morphology  `json:"morphology"`
	Orthography *Orthography `json:"orthography"`
}
type Diagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Module   string `json:"module"`
	RuleID   string `json:"ruleId,omitempty"`
	Path     string `json:"path"`
	Message  string `json:"message"`
}
type Trace struct {
	Stage  string   `json:"stage"`
	RuleID string   `json:"ruleId"`
	Before []string `json:"before"`
	After  []string `json:"after"`
}
type Candidate struct {
	Lexeme     string   `json:"lexeme,omitempty"`
	Cell       string   `json:"cell,omitempty"`
	Features   Features `json:"features,omitempty"`
	Underlying []string `json:"underlying"`
	Segments   []string `json:"segments"`
	Written    string   `json:"written,omitempty"`
	Trace      []Trace  `json:"trace"`
}
type Request struct {
	Revision  string   `json:"revision"`
	Engine    string   `json:"engine"`
	Operation string   `json:"operation"`
	Lexeme    string   `json:"lexeme,omitempty"`
	Features  Features `json:"features,omitempty"`
	Template  string   `json:"template,omitempty"`
	Segments  []string `json:"segments,omitempty"`
	Text      string   `json:"text,omitempty"`
	Policy    string   `json:"policy"` // all, seeded, explicit
	Seed      string   `json:"seed,omitempty"`
	Limit     int      `json:"limit"`
}
type Result struct {
	Status             string       `json:"status"`
	LanguageID         string       `json:"languageId"`
	Revision           string       `json:"revision"`
	Engine             string       `json:"engine"`
	Canonicalizer      string       `json:"canonicalizer"`
	Selection          string       `json:"selection"`
	Request            Request      `json:"request"`
	Candidates         []Candidate  `json:"candidates"`
	Diagnostics        []Diagnostic `json:"diagnostics"`
	Truncated          bool         `json:"truncated"`
	Examined           int          `json:"examined"`
	KnownValid         int          `json:"knownValid"`
	Rejected           int          `json:"rejected"`
	DiagnosticsOmitted int          `json:"diagnosticsOmitted"`
}

// Canonical is an explicit typed-JSON contract, not RFC 8785. Maps are string
// keyed, weights/limits are integers, arrays retain semantic ordering.
func Canonical(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
func Hash(v any) string     { sum := sha256.Sum256(Canonical(v)); return hex.EncodeToString(sum[:]) }
func copyJSON[T any](v T) T { var out T; _ = json.Unmarshal(Canonical(v), &out); return out }

// Check keys before typed decoding: DisallowUnknownFields alone accepts
// duplicate JSON keys and would silently overwrite authored rules/features.
func checkJSON(d *json.Decoder, depth int) error {
	if depth > 32 {
		return fmt.Errorf("definition exceeds nesting limit")
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	switch t {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := k.(string)
			if !ok || seen[key] {
				return fmt.Errorf("duplicate or invalid object key %q", k)
			}
			seen[key] = true
			if err := checkJSON(d, depth+1); err != nil {
				return err
			}
		}
		end, err := d.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return fmt.Errorf("invalid object")
		}
	case json.Delim('['):
		for d.More() {
			if err := checkJSON(d, depth+1); err != nil {
				return err
			}
		}
		end, err := d.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return fmt.Errorf("invalid array")
		}
	case json.Delim('}'), json.Delim(']'):
		return fmt.Errorf("unexpected delimiter")
	}
	return nil
}
func Decode(raw []byte) (Definition, error) {
	var def Definition
	if len(raw) > 2<<20 {
		return def, fmt.Errorf("definition exceeds 2 MiB spike limit")
	}
	if !utf8.Valid(raw) {
		return def, fmt.Errorf("definition is not valid UTF-8")
	}
	// encoding/json replaces lone UTF-16 surrogates with U+FFFD. Reject them
	// instead of silently changing an author's segment, grapheme or identifier.
	for i := 0; i < len(raw); i++ {
		if raw[i] != '"' {
			continue
		}
		for i++; i < len(raw) && raw[i] != '"'; i++ {
			if raw[i] != '\\' {
				continue
			}
			i++
			if i >= len(raw) {
				break
			}
			if raw[i] == 'u' && i+4 < len(raw) {
				n, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
				if err != nil {
					return def, fmt.Errorf("invalid Unicode escape")
				}
				i += 4
				if n >= 0xD800 && n <= 0xDBFF {
					if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
						return def, fmt.Errorf("unpaired Unicode surrogate")
					}
					low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
					if err != nil || low < 0xDC00 || low > 0xDFFF {
						return def, fmt.Errorf("unpaired Unicode surrogate")
					}
					i += 6
				} else if n >= 0xDC00 && n <= 0xDFFF {
					return def, fmt.Errorf("unpaired Unicode surrogate")
				}
			}
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if err := checkJSON(d, 0); err != nil {
		return def, err
	}
	if _, err := d.Token(); err != io.EOF {
		return def, fmt.Errorf("expected exactly one definition")
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&def); err != nil {
		return def, err
	}
	if def.Schema != SchemaVersion || strings.TrimSpace(def.LanguageID) == "" || strings.TrimSpace(def.Name) == "" {
		return def, fmt.Errorf("unsupported schema or missing language identity/name")
	}
	return def, nil
}
