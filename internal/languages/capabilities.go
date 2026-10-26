package languages

type OperationCapability struct {
	Operation string `json:"operation"`
	Support   string `json:"support"`
	Scope     string `json:"scope"`
}
type EngineCapability struct {
	Schema        int                   `json:"schema"`
	Engine        string                `json:"engine"`
	Adapter       string                `json:"adapter"`
	Compiler      string                `json:"compiler"`
	Canonicalizer string                `json:"canonicalizer"`
	Operations    []OperationCapability `json:"operations"`
}

// Capabilities describes actual adapters, independently from any draft's
// dependency readiness. It is metadata, not a linguistic result or cache input.
func Capabilities() []EngineCapability {
	out := []EngineCapability{}
	for _, schema := range []int{1, 2, 3} {
		e, a, c := Versions(Definition{Schema: schema})
		if schema == 1 {
			c = "languages-compiler/1"
		}
		profile := EngineCapability{Schema: schema, Engine: e, Adapter: a, Compiler: c, Canonicalizer: CanonicalVersion}
		for _, op := range []string{"generate", "validate", "inflect", "paradigm", "analyze", "analyze-written", "write", "realize", "parse", "regression"} {
			support, scope := "supported", "Bounded authored schema-1 subset; see operation readiness."
			if ProductiveSchema(schema) {
				scope = "Bounded productive/1 composition; inverse traverses the declared finite lexical domain."
				switch op {
				case "generate", "validate":
					scope = "Frozen segmental phonology subset, independent from productive morphology."
				case "write":
					scope = "Pinned segment-to-grapheme mappings and explicit whole-output normalization."
				case "regression":
					scope = "Exact authored requests, candidates and typed derivation traces."
				}
				if op == "realize" || op == "parse" {
					support = "unsupported"
					scope = "A1 does not execute schema-2 grammar."
				}
			}
			if schema == 3 && (op == "analyze" || op == "analyze-written") {
				scope = "survivor-trie/1 prunes impossible lexical cells, then composes every eligible analysis within explicit budgets."
			}
			profile.Operations = append(profile.Operations, OperationCapability{op, support, scope})
		}
		out = append(out, profile)
	}
	return out
}
