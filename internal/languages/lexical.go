package languages

// Lexical structure is authored source data. Underlying forms remain explicit;
// derivational annotations are not executed by the finite inflection adapter.
type Morpheme struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Underlying []string `json:"underlying"`
	Meanings   []string `json:"meanings"`
	Features   Features `json:"features"`
	Notes      string   `json:"notes"`
}
type LexicalEntry struct {
	Lexeme    string   `json:"lexeme"`
	Morphemes []string `json:"morphemes"`
	Notes     string   `json:"notes"`
}
type LexicalRelation struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Target string `json:"target"`
	Notes  string `json:"notes"`
}
type Derivation struct {
	ID     string   `json:"id"`
	Kind   string   `json:"kind"`
	Inputs []string `json:"inputs"`
	Target string   `json:"target"`
	Notes  string   `json:"notes"`
}
type LexicalSource struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	AgeID      string `json:"ageId"`
	LanguageID string `json:"languageId"`
	Revision   string `json:"revision"`
	Lexeme     string `json:"lexeme"`
	Target     string `json:"target"`
	Confidence string `json:"confidence"`
	Notes      string `json:"notes"`
}
type Attestation struct {
	ID     string `json:"id"`
	Lexeme string `json:"lexeme"`
	Form   string `json:"form"`
	Source string `json:"source"`
	Date   string `json:"date"`
	Notes  string `json:"notes"`
}
type LexicalModel struct {
	Morphemes    []Morpheme        `json:"morphemes"`
	Entries      []LexicalEntry    `json:"entries"`
	Relations    []LexicalRelation `json:"relations"`
	Derivations  []Derivation      `json:"derivations"`
	Sources      []LexicalSource   `json:"sources"`
	Attestations []Attestation     `json:"attestations"`
}

func (a *Artifact) checkLexical() []Diagnostic {
	l := a.definition.Lexical
	if l == nil {
		return nil
	}
	out := []Diagnostic{}
	bad := func(id, path, message string) {
		if len(out) < 256 {
			out = append(out, Diagnostic{Code: "invalid_lexical_structure", Severity: "error", Module: "lexicon", RuleID: id, Path: path, Message: message})
		} else {
			a.report.DiagnosticsOmitted++
		}
	}
	if len(l.Morphemes) > 1000 || len(l.Entries) > 1000 || len(l.Relations) > 2000 || len(l.Derivations) > 1000 || len(l.Sources) > 128 || len(l.Attestations) > 2000 {
		bad("", "lexical", "Lexical structure exceeds its bounded inventories.")
		return out
	}
	lexemes, segments, morphemes, seen := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, v := range a.definition.Lexicon {
		lexemes[v.ID] = true
	}
	if a.definition.Phonology != nil {
		for _, s := range a.definition.Phonology.Segments {
			segments[s.ID] = true
		}
	}
	id := func(namespace, value string) {
		key := namespace + "/" + value
		if value == "" || len(value) > 80 || seen[key] {
			bad(value, "lexical."+namespace, "Use unique bounded stable IDs.")
		}
		seen[key] = true
	}
	for _, m := range l.Morphemes {
		id("morphemes", m.ID)
		morphemes[m.ID] = true
		if len(m.Underlying) > 256 || len(m.Meanings) > 32 || len(m.Features) > 64 || len(m.Notes) > 2000 {
			bad(m.ID, "lexical.morphemes", "Morpheme data exceeds its bounds.")
		}
		for _, s := range m.Underlying {
			if !segments[s] {
				bad(m.ID, "lexical.morphemes", "Underlying form references an undeclared segment.")
			}
		}
	}
	for _, e := range l.Entries {
		id("entries", e.Lexeme)
		if !lexemes[e.Lexeme] || len(e.Morphemes) > 64 || len(e.Notes) > 2000 {
			bad(e.Lexeme, "lexical.entries", "Entry must reference a lexical identity and bounded component list.")
		}
		for _, m := range e.Morphemes {
			if !morphemes[m] {
				bad(e.Lexeme, "lexical.entries", "Segmentation references an unknown morpheme.")
			}
		}
	}
	for _, r := range l.Relations {
		id("relations", r.ID)
		if !lexemes[r.Source] || !lexemes[r.Target] || r.Kind == "" || len(r.Kind) > 80 || len(r.Notes) > 2000 {
			bad(r.ID, "lexical.relations", "Relation requires existing lexical identities and an authored kind.")
		}
	}
	edges := map[string][]string{}
	for _, r := range l.Derivations {
		id("derivations", r.ID)
		if !lexemes[r.Target] || len(r.Inputs) == 0 || len(r.Inputs) > 16 || len(r.Notes) > 2000 || r.Kind == "" {
			bad(r.ID, "lexical.derivations", "Derivations require bounded existing source/target lexemes and an explicit kind.")
		}
		for _, v := range r.Inputs {
			if !lexemes[v] {
				bad(r.ID, "lexical.derivations", "Unknown derivational input.")
			}
		}
		edges[r.Target] = append(edges[r.Target], r.Inputs...)
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var walk func(string, int) bool
	walk = func(v string, depth int) bool {
		if visiting[v] || depth > 32 {
			return false
		}
		if done[v] {
			return true
		}
		visiting[v] = true
		for _, next := range edges[v] {
			if !walk(next, depth+1) {
				return false
			}
		}
		delete(visiting, v)
		done[v] = true
		return true
	}
	for _, r := range l.Derivations {
		if !walk(r.Target, 0) {
			bad(r.ID, "lexical.derivations", "Derivations must be acyclic and at most 32 levels deep.")
			break
		}
	}
	for _, s := range l.Sources {
		id("sources", s.ID)
		if !lexemes[s.Target] || !ValidID(s.LanguageID) || !ValidID(s.AgeID) || !ValidHash(s.Revision) || s.Lexeme == "" || len(s.Confidence) > 160 || len(s.Notes) > 2000 {
			bad(s.ID, "lexical.sources", "Pin a real source revision/lexeme and target; confidence is author-defined metadata.")
		}
	}
	for _, v := range l.Attestations {
		id("attestations", v.ID)
		if !lexemes[v.Lexeme] || len(v.Form) > 4096 || len(v.Source) > 2000 || len(v.Date) > 160 || len(v.Notes) > 2000 {
			bad(v.ID, "lexical.attestations", "Attestation references an unknown lexeme or exceeds its bounds.")
		}
	}
	if len(l.Derivations) > 0 {
		out = append(out, Diagnostic{Code: "unsupported_operation", Severity: "info", Module: "lexicon", Path: "lexical.derivations", Message: "Derivational structures are retained source annotations. This adapter inflects explicit underlying forms; it does not execute derivation/compounding rules."})
	}
	return out
}
