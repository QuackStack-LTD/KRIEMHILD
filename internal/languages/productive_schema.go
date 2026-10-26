package languages

import "kriemhild/internal/languages/spike"

const (
	ProductiveSchemaVersion = 2
	ProductiveEngine        = "languages/2"
	ProductiveAdapter       = "finite-compose/1"
	ProductiveCompiler      = "languages-compiler/2"
	ProductiveVersion       = "productive/1"
	IndexedSchemaVersion    = 3
	IndexedEngine           = "languages/3"
	IndexedAdapter          = "finite-compose/2"
	IndexedCompiler         = "languages-compiler/3"
)

// Versions dispatches by authored schema, never by a process-wide mutable mode.
func Versions(d Definition) (engine, adapter, compiler string) {
	if d.Schema == IndexedSchemaVersion {
		return IndexedEngine, IndexedAdapter, IndexedCompiler
	}
	if d.Schema == ProductiveSchemaVersion {
		return ProductiveEngine, ProductiveAdapter, ProductiveCompiler
	}
	return EngineVersion, AdapterVersion, ""
}
func CompatibleVersions(schema int, engine, adapter, compiler string) bool {
	e, a, c := Versions(Definition{Schema: schema})
	return (schema == 1 || ProductiveSchema(schema)) && engine == e && adapter == a && compiler == c
}
func ProductiveSchema(schema int) bool { return schema == 2 || schema == 3 }

type ProductiveModel struct {
	Version   string               `json:"version"`
	Features  []spike.Feature      `json:"features"`
	Paradigms []ProductiveParadigm `json:"paradigms"`
	Stratum   BoundaryStratum      `json:"stratum"`
}
type ProductiveParadigm struct {
	ID    string           `json:"id"`
	Cells []ProductiveCell `json:"cells"`
}
type ProductiveCell struct {
	ID         string                `json:"id"`
	Features   Features              `json:"features"`
	Operations []ProductiveOperation `json:"operations"`
}
type ProductiveOperation struct {
	ID         string      `json:"id"`
	Kind       string      `json:"kind"`
	Policy     string      `json:"policy"`
	Allomorphs []Allomorph `json:"allomorphs"`
}
type Allomorph struct {
	ID     string              `json:"id"`
	When   *AllomorphCondition `json:"when"`
	Prefix []string            `json:"prefix"`
	Suffix []string            `json:"suffix"`
}
type AllomorphCondition struct {
	Features Features      `json:"features"`
	Category string        `json:"category,omitempty"`
	Previous string        `json:"previous,omitempty"`
	Left     []spike.Match `json:"left"`
	Right    []spike.Match `json:"right"`
}
type BoundaryStratum struct {
	ID             string            `json:"id"`
	BoundaryPolicy string            `json:"boundaryPolicy"` // retain-until-surface
	Rules          []spike.SoundRule `json:"rules"`          // domain: morpheme; contexts can match boundaries
}

// ProductiveToken distinguishes boundaries from authored segment IDs.
type ProductiveToken struct {
	Segment  string `json:"segment,omitempty"`
	Boundary string `json:"boundary,omitempty"`
}
type AlternativeDecision struct {
	ID       string `json:"id"`
	Decision string `json:"decision"` // selected, condition-failed, precedence-skipped
}
type ProductiveStep struct {
	Stage        string                `json:"stage"`
	RuleID       string                `json:"ruleId"`
	Allomorph    string                `json:"allomorph,omitempty"`
	Before       []ProductiveToken     `json:"before"`
	After        []ProductiveToken     `json:"after"`
	Alternatives []AlternativeDecision `json:"alternatives,omitempty"`
}
type ProductiveDerivation struct {
	Failure   *Diagnostic      `json:"failure,omitempty"`
	Candidate string           `json:"candidate"` // canonical candidate hash
	Lexeme    string           `json:"lexeme"`
	Cell      string           `json:"cell"`
	Steps     []ProductiveStep `json:"steps"`
}
