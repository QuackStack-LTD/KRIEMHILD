// Package languages owns versioned linguistic definitions and deterministic
// compilation. It has no network, model-service or persistence dependency.
package languages

import (
	"encoding/json"
	"fmt"
	"kriemhild/internal/languages/spike"
	"regexp"
	"strings"
)

const (
	SchemaVersion      = 1
	EngineVersion      = "languages/1"
	AdapterVersion     = "token-relation/1"
	CanonicalVersion   = spike.CanonicalVersion
	MaxDefinitionBytes = 2 << 20
	MaxRevisions       = 256
)

type Diagnostic = spike.Diagnostic
type Features = spike.Features
type RevisionRef struct {
	Hash          string `json:"hash"`
	Schema        int    `json:"schema"`
	Canonicalizer string `json:"canonicalizer"`
	Size          int    `json:"size"`
	Parent        string `json:"parent,omitempty"`
	CreatedAt     int64  `json:"createdAt"`
	Author        string `json:"author,omitempty"`
	Note          string `json:"note,omitempty"`
}
type Link struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Revision string `json:"revision,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

// State is Age-specific, small and editable. Bodies and artifacts never live in it.
type State struct {
	Kind        string        `json:"kind,omitempty"`
	Status      string        `json:"status,omitempty"`
	Description string        `json:"description,omitempty"`
	Links       []Link        `json:"links,omitempty"`
	Draft       string        `json:"draftRevision"`
	Active      string        `json:"activeRevision,omitempty"`
	Revisions   []RevisionRef `json:"revisions"`
	Compiled    []CompileRef  `json:"compiled,omitempty"`
}
type CompileRef struct {
	Compiler     string      `json:"compiler,omitempty"`
	Revision     string      `json:"revision"`
	Engine       string      `json:"engine"`
	Adapter      string      `json:"adapter"`
	Dependencies []ScriptRef `json:"dependencies,omitempty"`
	Report       Report      `json:"report"`
}
type ScriptRef struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
}
type Orthography struct {
	Script        ScriptRef       `json:"script"`
	Normalization string          `json:"normalization"`
	Mappings      []spike.Mapping `json:"mappings"`
}
type Grapheme struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	Label      string `json:"label,omitempty"`
	GlyphAsset string `json:"glyphAsset,omitempty"`
}
type ScriptDefinition struct {
	Writing   *WritingSystem `json:"writing,omitempty"`
	Schema    int            `json:"schema"`
	ScriptID  string         `json:"scriptId"`
	Name      string         `json:"name"`
	Notes     string         `json:"notes"`
	Direction string         `json:"direction"`
	Graphemes []Grapheme     `json:"graphemes"`
}

// Values are explicitly tagged. A variable is a unification variable, never
// executable code. Higher-order features can be retained without being executed.
type Value struct {
	Kind      string            `json:"kind"` // atom, variable, structure, absent
	Atom      string            `json:"atom,omitempty"`
	Variable  string            `json:"variable,omitempty"`
	Structure *FeatureStructure `json:"structure,omitempty"`
}
type FeatureStructure struct {
	Type   string           `json:"type"`
	Fields map[string]Value `json:"fields"`
}
type FeatureType struct {
	ID     string              `json:"id"`
	Fields map[string][]string `json:"fields"`
}
type Role struct {
	ID       string   `json:"id"`
	Category string   `json:"category"`
	Features Features `json:"features"`
}
type Agreement struct {
	ID        string   `json:"id"`
	LeftRole  string   `json:"leftRole"`
	RightRole string   `json:"rightRole"`
	Features  []string `json:"features"`
}
type Constituent struct {
	Role         string `json:"role,omitempty"`
	Literal      string `json:"literal,omitempty"`
	Construction string `json:"construction,omitempty"`
}
type Construction struct {
	ID         string        `json:"id"`
	Meaning    string        `json:"meaning"`
	Roles      []Role        `json:"roles"`
	Order      []Constituent `json:"order"`
	Agreements []Agreement   `json:"agreements"`
	Separator  string        `json:"separator"`
}
type Syntax struct {
	Types         []FeatureType      `json:"types"`
	Structures    []FeatureStructure `json:"structures"`
	Constructions []Construction     `json:"constructions"`
}
type Historical struct {
	SourceLanguage string                `json:"sourceLanguage"`
	SourceRevision string                `json:"sourceRevision"`
	SourceAge      string                `json:"sourceAge"`
	Changes        []spike.SoundRule     `json:"changes"`
	Exceptions     []HistoricalException `json:"exceptions"`
	Notes          string                `json:"notes"`
}
type HistoricalException struct {
	Lexeme   string   `json:"lexeme"`
	RuleID   string   `json:"ruleId"`
	Segments []string `json:"segments"`
}
type RegressionCase struct {
	ExpectedRejectedDerivations []ProductiveDerivation `json:"expectedRejectedDerivations,omitempty"`
	ExpectedDerivations         []ProductiveDerivation `json:"expectedDerivations,omitempty"`
	ExpectedDiagnostics         []Diagnostic           `json:"expectedDiagnostics,omitempty"`
	ID                          string                 `json:"id"`
	Request                     Request                `json:"request"`
	ExpectedStatus              string                 `json:"expectedStatus"`
	Expected                    []spike.Candidate      `json:"expected"`
	ExpectedSentences           []Sentence             `json:"expectedSentences,omitempty"`
	Notes                       string                 `json:"notes"`
}
type Definition struct {
	SequenceWriting *SequenceWriting  `json:"sequenceWriting,omitempty"`
	Productive      *ProductiveModel  `json:"productive,omitempty"`
	Lexical         *LexicalModel     `json:"lexical,omitempty"`
	Schema          int               `json:"schema"`
	LanguageID      string            `json:"languageId"`
	Name            string            `json:"name"`
	Notes           string            `json:"notes"`
	Phonology       *spike.Phonology  `json:"phonology"`
	Lexicon         []spike.Lexeme    `json:"lexicon"`
	Morphology      *spike.Morphology `json:"morphology"`
	Syntax          *Syntax           `json:"syntax"`
	Orthography     *Orthography      `json:"orthography"`
	History         *Historical       `json:"history"`
	Tests           []RegressionCase  `json:"tests"`
}
type RoleInput struct {
	Lexeme   string   `json:"lexeme"`
	Features Features `json:"features"`
}
type Request struct {
	Spelling *SpellingRequest `json:"spelling,omitempty"`
	spike.Request
	Compiler     string               `json:"compiler,omitempty"`
	Adapter      string               `json:"adapter"`
	Construction string               `json:"construction,omitempty"`
	Roles        map[string]RoleInput `json:"roles,omitempty"`
}
type Report struct {
	Capabilities       []OperationCapability `json:"capabilities,omitempty"`
	Schema             int                   `json:"schema,omitempty"`
	Compiler           string                `json:"compiler,omitempty"`
	Revision           string                `json:"revision"`
	Engine             string                `json:"engine"`
	Adapter            string                `json:"adapter"`
	Kernel             string                `json:"kernel"`
	Status             string                `json:"status"`
	Ready              map[string]bool       `json:"ready"`
	Dependencies       []ScriptRef           `json:"dependencies,omitempty"`
	Diagnostics        []Diagnostic          `json:"diagnostics"`
	DiagnosticsOmitted int                   `json:"diagnosticsOmitted"`
	Forms              int                   `json:"forms"`
	States             int                   `json:"states"`
}
type Sentence struct {
	Gloss        []GlossWord                `json:"gloss,omitempty"`
	Construction string                     `json:"construction"`
	Roles        map[string]spike.Candidate `json:"roles"`
	Tokens       []string                   `json:"tokens"`
	Written      string                     `json:"written"`
	Trace        []spike.Trace              `json:"trace"`
}
type GlossWord struct {
	Role     string   `json:"role"`
	Lexeme   string   `json:"lexeme"`
	Surface  string   `json:"surface"`
	Meanings []string `json:"meanings"`
	Features Features `json:"features"`
}
type Result struct {
	Spelling            *SpellingResult        `json:"spelling,omitempty"`
	Inverse             *InverseMetrics        `json:"inverse,omitempty"`
	RejectedDerivations []ProductiveDerivation `json:"rejectedDerivations,omitempty"`
	Schema              int                    `json:"schema,omitempty"`
	Compiler            string                 `json:"compiler,omitempty"`
	Derivations         []ProductiveDerivation `json:"derivations,omitempty"`
	spike.Result
	Adapter      string       `json:"adapter"`
	Kernel       string       `json:"kernel"`
	Input        Request      `json:"input"`
	Dependencies []ScriptRef  `json:"dependencies,omitempty"`
	Sentences    []Sentence   `json:"sentences,omitempty"`
	Tests        []CaseResult `json:"tests,omitempty"`
}

var identity = regexp.MustCompile(`^[a-f0-9]{32}$`)
var digest = regexp.MustCompile(`^[a-f0-9]{64}$`)

func ValidID(s string) bool   { return identity.MatchString(s) }
func ValidHash(s string) bool { return digest.MatchString(s) }
func Canonical(v any) []byte  { return spike.Canonical(v) }
func Hash(v any) string       { return spike.Hash(v) }
func Clone[T any](v T) T {
	var out T
	if err := json.Unmarshal(Canonical(v), &out); err != nil {
		panic(err)
	}
	return out
}
func Decode(raw []byte) (Definition, error) {
	var d Definition
	if err := spike.DecodeJSON(raw, &d); err != nil {
		return d, err
	}
	if (d.Schema != SchemaVersion && !ProductiveSchema(d.Schema)) || !ValidID(d.LanguageID) || strings.TrimSpace(d.Name) == "" || len(d.Name) > 160 || len(d.Notes) > 8192 {
		return d, fmt.Errorf("choose supported language schema 1, 2, 3 or 4, a language identity and a name of at most 160 bytes")
	}
	if d.Schema != 4 {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return d, err
		}
		if _, ok := fields["sequenceWriting"]; ok {
			return d, fmt.Errorf("sequenceWriting requires explicit language schema 4, including when null")
		}
	} else if err := spellingShape(d.SequenceWriting); err != nil {
		return d, err
	}
	if d.Schema == 1 && d.Productive != nil || ProductiveSchema(d.Schema) && d.Morphology != nil {
		return d, fmt.Errorf("productive rules require schema 2; schema 2 cannot also contain legacy morphology")
	}
	if d.Schema == 1 {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return d, err
		}
		if _, present := fields["productive"]; present {
			return d, fmt.Errorf("productive is a schema-2 field, including when null; schema 1 is not reinterpreted")
		}
	}
	if len(d.Lexicon) > 1000 || len(d.Tests) > 128 {
		return d, fmt.Errorf("definition exceeds 1,000 lexemes or 128 regression cases")
	}
	if d.Lexical != nil && (len(d.Lexical.Morphemes) > 1000 || len(d.Lexical.Entries) > 1000 || len(d.Lexical.Relations) > 2000 || len(d.Lexical.Derivations) > 1000 || len(d.Lexical.Sources) > 128 || len(d.Lexical.Attestations) > 2000) {
		return d, fmt.Errorf("lexical structure exceeds its bounded inventories")
	}
	if ProductiveSchema(d.Schema) {
		if err := productiveShape(d.Productive); err != nil {
			return d, err
		}
		for _, lex := range d.Lexicon {
			if len(lex.Underlying) > 256 || len(lex.Features) > 64 || len(lex.Senses) > 32 || len(lex.Overrides) > 64 {
				return d, fmt.Errorf("lexicon: productive lexical entry exceeds bounded form/features/senses/overrides")
			}
		}
	}
	return d, nil
}
func DecodeScript(raw []byte) (ScriptDefinition, error) {
	var d ScriptDefinition
	if err := spike.DecodeJSON(raw, &d); err != nil {
		return d, err
	}
	if (d.Schema != SchemaVersion && d.Schema != 2) || !ValidID(d.ScriptID) || strings.TrimSpace(d.Name) == "" || len(d.Name) > 160 || len(d.Notes) > 8192 || len(d.Graphemes) > 512 {
		return d, fmt.Errorf("invalid script identity, schema, name or size")
	}
	if d.Schema == 1 {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return d, err
		}
		if _, ok := fields["writing"]; ok {
			return d, fmt.Errorf("writing requires explicit Script schema 2, including when null")
		}
	}
	if d.Schema == 2 {
		if err := vectorShape(d.Writing); err != nil {
			return d, err
		}
	}
	if d.Direction != "undecided" && d.Direction != "ltr" && d.Direction != "rtl" && d.Direction != "ttb" {
		return d, fmt.Errorf("choose an explicit script direction or undecided")
	}
	seen := map[string]bool{}
	for _, g := range d.Graphemes {
		if g.ID == "" || len(g.ID) > 80 || seen[g.ID] || len(g.Text) > 256 || len(g.GlyphAsset) > 256 {
			return d, fmt.Errorf("invalid or duplicate grapheme")
		}
		seen[g.ID] = true
	}
	return d, nil
}
func (s *State) Ref(h string) *RevisionRef {
	if s == nil {
		return nil
	}
	for i := range s.Revisions {
		if s.Revisions[i].Hash == h {
			return &s.Revisions[i]
		}
	}
	return nil
}
func ValidateState(s *State) error {
	if s == nil {
		return nil
	}
	if len(s.Revisions) == 0 || len(s.Revisions) > MaxRevisions || len(s.Compiled) > 32 || len(s.Links) > 128 || len(s.Description) > 8192 || len(s.Kind) > 80 || len(s.Status) > 80 {
		return fmt.Errorf("invalid language/script state limits")
	}
	seen := map[string]bool{}
	for _, r := range s.Revisions {
		if !ValidHash(r.Hash) || seen[r.Hash] || (r.Schema != SchemaVersion && !ProductiveSchema(r.Schema)) || r.Canonicalizer != CanonicalVersion || r.Size < 2 || r.Size > MaxDefinitionBytes || len(r.Note) > 1000 || len(r.Author) > 256 {
			return fmt.Errorf("invalid immutable definition reference")
		}
		if r.Parent != "" && !seen[r.Parent] {
			return fmt.Errorf("definition parent must precede its child")
		}
		seen[r.Hash] = true
	}
	if !seen[s.Draft] || s.Active != "" && !seen[s.Active] {
		return fmt.Errorf("definition head is not retained")
	}
	for _, l := range s.Links {
		if !ValidID(l.ID) || len(l.Kind) > 80 || len(l.Notes) > 1000 || l.Revision != "" && !ValidHash(l.Revision) {
			return fmt.Errorf("invalid language relationship")
		}
	}
	for _, c := range s.Compiled {
		if !seen[c.Revision] || c.Engine == "" || c.Adapter == "" || c.Report.Revision != c.Revision || c.Report.Engine != c.Engine || c.Report.Adapter != c.Adapter || c.Report.Compiler != c.Compiler || len(c.Report.Diagnostics) > 256 || len(c.Dependencies) > 8 {
			return fmt.Errorf("invalid compile reference")
		}
	}
	return nil
}
