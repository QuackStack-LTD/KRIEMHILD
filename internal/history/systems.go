package history

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
)

// System records are independent, Age-scoped entities. SystemID is outline
// membership, not ownership of the world entities used by a system.
type SystemRecord struct {
	Version        int                `json:"version"`
	SystemID       string             `json:"systemId,omitempty"`
	Category       string             `json:"category,omitempty"`
	Mode           string             `json:"mode,omitempty"`
	Template       string             `json:"template,omitempty"`
	Classification string             `json:"classification,omitempty"`
	Possibility    string             `json:"possibility,omitempty"`
	Development    string             `json:"development,omitempty"`
	SeedAvailable  bool               `json:"seedAvailable,omitempty"`
	Strict         bool               `json:"strict,omitempty"`
	Statement      string             `json:"statement,omitempty"`
	Preconditions  string             `json:"preconditions,omitempty"`
	Operation      string             `json:"operation,omitempty"`
	Inputs         string             `json:"inputs,omitempty"`
	Outputs        string             `json:"outputs,omitempty"`
	Costs          string             `json:"costs,omitempty"`
	Failures       string             `json:"failures,omitempty"`
	Exceptions     string             `json:"exceptions,omitempty"`
	Questions      []string           `json:"questions,omitempty"`
	Unresolved     []string           `json:"unresolved,omitempty"`
	Effects        []string           `json:"effects,omitempty"`
	Constraints    []SystemConstraint `json:"constraints,omitempty"`
	Sources        []SystemSource     `json:"sources,omitempty"`
	Fields         []TypedField       `json:"fields,omitempty"`
}
type SystemConstraint struct {
	Key   string   `json:"key"`
	Mode  string   `json:"mode"` // allow, prohibit, minimum, maximum, value
	Value *float64 `json:"value,omitempty"`
	Unit  string   `json:"unit,omitempty"`
	Notes string   `json:"notes,omitempty"`
}
type SystemSource struct {
	TerrainID    string `json:"terrainId,omitempty"`
	OccurrenceID string `json:"occurrenceId,omitempty"`
	EntityID     string `json:"entityId,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

func cloneSystem(s *SystemRecord) *SystemRecord {
	if s == nil {
		return nil
	}
	b, _ := json.Marshal(s)
	var out SystemRecord
	_ = json.Unmarshal(b, &out)
	return &out
}
func (d *Document) validateSystem(e Entity) error {
	s := e.System
	if e.Deleted {
		return nil
	}
	if Registry.Types[e.Kind].Category != "systems" {
		if s != nil {
			return fmt.Errorf("system data requires a system record")
		}
		return nil
	}
	if s == nil || s.Version != 1 {
		return fmt.Errorf("system record requires version 1 data")
	}
	if e.Kind == "system" {
		if s.SystemID != "" || !slices.Contains([]string{"magic", "technology", "hybrid", "custom"}, s.Category) {
			return fmt.Errorf("invalid system category or outline parent")
		}
	} else {
		parent := d.Entity(e.AgeID, s.SystemID)
		if parent == nil || parent.Deleted || parent.Kind != "system" {
			return fmt.Errorf("system section needs an active system in this Age")
		}
	}
	for _, enum := range []struct {
		value   string
		allowed []string
	}{
		{s.Mode, []string{"", "quick", "guided", "advanced"}},
		{s.Classification, []string{"", "hard", "assumption", "observation", "hypothesis"}},
		{s.Possibility, []string{"", "possible", "impossible", "unknown"}},
		{s.Development, []string{"", "theory", "discovered", "prototype", "working", "reproducible", "widespread", "restricted", "lost", "retired"}},
	} {
		if !slices.Contains(enum.allowed, enum.value) {
			return fmt.Errorf("invalid system classification %q", enum.value)
		}
	}
	for _, text := range []string{s.Statement, s.Preconditions, s.Operation, s.Inputs, s.Outputs, s.Costs, s.Failures, s.Exceptions} {
		if len(text) > 50000 {
			return fmt.Errorf("system section exceeds 50000 characters")
		}
	}
	for _, list := range [][]string{s.Questions, s.Unresolved, s.Effects} {
		if len(list) > 100 {
			return fmt.Errorf("too many system entries")
		}
		for _, v := range list {
			if len(v) > 2000 {
				return fmt.Errorf("system entry too long")
			}
		}
	}
	if len(s.Constraints) > 100 || len(s.Sources) > 200 {
		return fmt.Errorf("too many constraints or sources")
	}
	for _, c := range s.Constraints {
		if strings.TrimSpace(c.Key) == "" || len(c.Key) > 200 || len(c.Unit) > 80 || len(c.Notes) > 10000 || !slices.Contains([]string{"allow", "prohibit", "minimum", "maximum", "value"}, c.Mode) {
			return fmt.Errorf("invalid structured constraint")
		}
		if slices.Contains([]string{"minimum", "maximum", "value"}, c.Mode) && (c.Value == nil || math.IsNaN(*c.Value) || math.IsInf(*c.Value, 0)) {
			return fmt.Errorf("numeric constraints require a finite value")
		}
	}
	for _, ref := range s.Sources {
		if len(ref.Notes) > 10000 {
			return fmt.Errorf("source notes exceed 10000 characters")
		}
		if (ref.EntityID == "") == (ref.OccurrenceID == "") {
			return fmt.Errorf("source must reference exactly one world entity or natural occurrence")
		}
		if ref.EntityID != "" {
			v := d.Entity(e.AgeID, ref.EntityID)
			if v == nil || v.Deleted || v.ID == e.ID {
				return fmt.Errorf("source entity must exist in this Age")
			}
		}
		if ref.OccurrenceID != "" {
			found := false
			for _, t := range d.Terrains {
				if t.AgeID == e.AgeID && t.ID == ref.TerrainID {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("source terrain must exist in this Age")
			}
		}
	}
	return d.validateFields(e.AgeID, s.Fields)
}
func (d *Document) validateSystemDeletion(age string, ids []string) error {
	for _, e := range d.Entities {
		if e.AgeID != age || e.Deleted || e.System == nil || slices.Contains(ids, e.ID) {
			continue
		}
		if slices.Contains(ids, e.System.SystemID) {
			return fmt.Errorf("remove or move system sections before deleting their system")
		}
		for _, s := range e.System.Sources {
			if slices.Contains(ids, s.EntityID) {
				return fmt.Errorf("%s still references this source; unlink it first", e.Name)
			}
		}
	}
	return nil
}

type SystemIssue struct {
	Code     string   `json:"code"`
	Severity string   `json:"severity"`
	EntityID string   `json:"entityId"`
	Related  []string `json:"related,omitempty"`
	Message  string   `json:"message"`
}
type SystemImpact struct {
	EntityID string   `json:"entityId"`
	Path     []string `json:"path"`
}
type SystemContext struct {
	RecordID string   `json:"recordId"`
	EntityID string   `json:"entityId"`
	Relation Relation `json:"relation"`
}
type SystemAnalysis struct {
	Contexts     []SystemContext     `json:"contexts,omitempty"`
	Issues       []SystemIssue       `json:"issues"`
	Affected     []SystemImpact      `json:"affected"`
	Requirements map[string][]string `json:"requirements"`
}

func dependency(r Relation) (string, string) {
	switch r.Kind {
	case "system-requires", "system-consumes", "system-derived-from":
		return r.Source, r.Destination
	case "system-enables", "system-produces":
		return r.Destination, r.Source
	}
	return "", ""
}

// AnalyzeSystems is advisory and never mutates canonical data. Effects and
// constraints use author-defined keys; prose is only a possible inconsistency.
func (d *Document) AnalyzeSystems(age, system, changed string) SystemAnalysis {
	out := SystemAnalysis{Issues: []SystemIssue{}, Affected: []SystemImpact{}, Requirements: map[string][]string{}}
	entities := map[string]Entity{}
	reverse := map[string][]string{}
	exceptions := map[string]map[string]bool{}
	for _, e := range d.Entities {
		if e.AgeID == age && !e.Deleted && e.System != nil {
			entities[e.ID] = e
		}
	}
	affected := map[string]bool{}
	for id, e := range entities {
		if e.System.SystemID != "" {
			reverse[e.System.SystemID] = append(reverse[e.System.SystemID], id)
		}
	}
	relevant := func(id string) bool {
		e, ok := entities[id]
		return ok && (system == "" || id == system || e.System.SystemID == system || affected[id])
	}
	add := func(code, severity, id, message string, related ...string) {
		if relevant(id) {
			out.Issues = append(out.Issues, SystemIssue{code, severity, id, related, message})
		}
	}
	for _, r := range d.Relations {
		if r.AgeID != age {
			continue
		}
		a, b := dependency(r)
		if _, ok := entities[a]; ok {
			if _, ok := entities[b]; ok {
				if !slices.Contains(out.Requirements[a], b) {
					out.Requirements[a] = append(out.Requirements[a], b)
					reverse[b] = append(reverse[b], a)
				}
			}
		}
		if r.Kind == "system-restricts" || r.Kind == "system-prohibits" || r.Kind == "system-conflicts" {
			if entities[r.Source].System != nil && entities[r.Destination].System != nil {
				reverse[r.Source] = append(reverse[r.Source], r.Destination)
				if r.Kind == "system-conflicts" {
					reverse[r.Destination] = append(reverse[r.Destination], r.Source)
				}
			}
		}
		if r.Kind == "system-exception-to" {
			if exceptions[r.Source] == nil {
				exceptions[r.Source] = map[string]bool{}
			}
			exceptions[r.Source][r.Destination] = true
		}
	}
	// Deterministic traversal, with one shortest explanation path per affected record.
	for id := range out.Requirements {
		slices.Sort(out.Requirements[id])
	}
	for id := range reverse {
		slices.Sort(reverse[id])
	}
	if changed != "" {
		seen := map[string]bool{changed: true}
		queue := []SystemImpact{{changed, []string{changed}}}
		for len(queue) > 0 {
			p := queue[0]
			queue = queue[1:]
			for _, id := range reverse[p.EntityID] {
				if seen[id] {
					continue
				}
				seen[id] = true
				next := SystemImpact{id, append(append([]string{}, p.Path...), id)}
				out.Affected = append(out.Affected, next)
				affected[id] = true
				queue = append(queue, next)
			}
		}
	}
	closure := func(id string) []string {
		seen := map[string]bool{id: true}
		q := []string{id}
		for i := 0; i < len(q); i++ {
			for _, x := range out.Requirements[q[i]] {
				if !seen[x] {
					seen[x] = true
					q = append(q, x)
				}
			}
		}
		return q
	}
	// A declared available seed can break a startup dependency cycle. Unknown or
	// theoretical availability never silently provides a bootstrap input.
	ready := map[string]bool{}
	for id, e := range entities {
		if e.System.SeedAvailable {
			ready[id] = true
		}
	}
	for pass := 0; pass < len(entities); pass++ {
		progress := false
		for id := range entities {
			if ready[id] {
				continue
			}
			ok := true
			for _, dep := range out.Requirements[id] {
				if !ready[dep] {
					ok = false
				}
			}
			if ok {
				ready[id] = true
				progress = true
			}
		}
		if !progress {
			break
		}
	}
	for id, e := range entities {
		s := e.System
		chain := closure(id)
		if !ready[id] {
			cycle := false
			for _, dep := range out.Requirements[id] {
				if slices.Contains(closure(dep), id) {
					cycle = true
				}
			}
			if cycle {
				add("startup-cycle", "conflict", id, "Required inputs form a cycle with no declared available starting input.", chain...)
			}
		}
		for _, q := range s.Questions {
			if strings.TrimSpace(q) != "" {
				add("open-question", "question", id, q)
			}
		}
		for _, missing := range s.Unresolved {
			if strings.TrimSpace(missing) != "" {
				add("missing-prerequisite", "question", id, "Unresolved prerequisite: "+missing)
			}
		}
		if e.Kind == "system-capability" && len(out.Requirements[id]) == 0 {
			add("unsupported-capability", "question", id, "No supporting rules, methods or inputs have been linked. Mystery is allowed; support remains unspecified.")
		}
		for _, dep := range chain {
			v := entities[dep]
			if v.System.Possibility == "impossible" && dep != id {
				add("impossible-prerequisite", "conflict", id, "Depends on a record declared impossible in this Age.", dep)
			}
		}
		scope := append([]string{}, chain...)
		for _, dep := range chain {
			parent := entities[dep].System.SystemID
			if parent != "" && !slices.Contains(scope, parent) {
				scope = append(scope, parent)
			}
		}
		if s.SystemID != "" && !slices.Contains(scope, s.SystemID) {
			scope = append(scope, s.SystemID)
		}
		for _, r := range d.Relations {
			if r.AgeID == age && r.Kind == "system-restricts" && slices.Contains(chain, r.Destination) && entities[r.Source].System != nil && !slices.Contains(scope, r.Source) {
				scope = append(scope, r.Source)
			}
		}
		for _, x := range scope {
			v, ok := entities[x]
			if !ok || exceptions[id][x] {
				continue
			}
			severity := "conflict"
			if v.System.Classification == "hypothesis" || v.System.Classification == "assumption" || v.System.Classification == "observation" {
				severity = "possible"
			}
			for _, limit := range v.System.Constraints {
				if limit.Mode == "prohibit" {
					for _, effect := range s.Effects {
						if strings.EqualFold(strings.TrimSpace(effect), strings.TrimSpace(limit.Key)) {
							add("prohibited-effect", severity, id, "Declared effect violates limit: "+limit.Key, x)
						}
					}
					prose := strings.ToLower(s.Statement + " " + s.Operation + " " + s.Outputs)
					if id != x && strings.Contains(prose, strings.ToLower(limit.Key)) && !slices.Contains(s.Effects, limit.Key) {
						add("possible-prose-conflict", "possible", id, "Text mentions prohibited effect '"+limit.Key+"'. Review its meaning or add an explicit exception.", x)
					}
				}
				for _, own := range s.Constraints {
					if own.Key != limit.Key || own.Unit != limit.Unit || x == id && own.Mode == limit.Mode {
						continue
					}
					contradiction := own.Mode == "allow" && limit.Mode == "prohibit"
					if own.Value != nil && limit.Value != nil {
						contradiction = contradiction || (own.Mode == "minimum" || own.Mode == "value") && limit.Mode == "maximum" && *own.Value > *limit.Value || (own.Mode == "maximum" || own.Mode == "value") && limit.Mode == "minimum" && *own.Value < *limit.Value || own.Mode == "value" && limit.Mode == "value" && *own.Value != *limit.Value
					}
					if contradiction {
						add("constraint-conflict", severity, id, "Incompatible constraints for "+own.Key+" ("+own.Unit+").", x)
					}
				}
			}
		}

		type limitOwner struct {
			id string
			c  SystemConstraint
		}
		limits := []limitOwner{}
		for _, dep := range scope {
			if exceptions[id][dep] {
				continue
			}
			for _, c := range entities[dep].System.Constraints {
				limits = append(limits, limitOwner{dep, c})
			}
		}
		for i, a := range limits {
			for _, b := range limits[i+1:] {
				if a.id == b.id || a.c.Key != b.c.Key || a.c.Unit != b.c.Unit {
					continue
				}
				conflict := (a.c.Mode == "allow" && b.c.Mode == "prohibit") || (b.c.Mode == "allow" && a.c.Mode == "prohibit")
				if a.c.Value != nil && b.c.Value != nil {
					for _, pair := range [][2]SystemConstraint{{a.c, b.c}, {b.c, a.c}} {
						x, y := pair[0], pair[1]
						conflict = conflict || (x.Mode == "minimum" || x.Mode == "value") && y.Mode == "maximum" && *x.Value > *y.Value || x.Mode == "value" && y.Mode == "value" && *x.Value != *y.Value
					}
				}
				if conflict {
					severity := "conflict"
					if entities[a.id].System.Classification == "hypothesis" || entities[b.id].System.Classification == "hypothesis" || entities[a.id].System.Classification == "assumption" || entities[b.id].System.Classification == "assumption" {
						severity = "possible"
					}
					add("dependency-limits", severity, id, "Prerequisites disagree about "+a.c.Key+" ("+a.c.Unit+").", a.id, b.id)
				}
			}
		}
		for _, r := range d.Relations {
			if r.AgeID != age || !slices.Contains([]string{"system-conflicts", "system-prohibits"}, r.Kind) {
				continue
			}
			applies := slices.Contains(scope, r.Source) && slices.Contains(scope, r.Destination)
			if r.Kind == "system-prohibits" && slices.Contains(chain, r.Destination) {
				applies = true
			}
			if applies && !exceptions[id][r.Source] {
				add("incompatible-dependencies", "conflict", id, "Required records have an explicit incompatibility.", r.Source, r.Destination)
			}
		}
	}
	for _, r := range d.Relations {
		if r.AgeID != age || !strings.HasPrefix(r.Kind, "system-") {
			continue
		}
		a, aok := entities[r.Source]
		b, bok := entities[r.Destination]
		if aok == bok {
			continue
		}
		record, other := a, r.Destination
		if bok {
			record, other = b, r.Source
		}
		if record.ID == changed || affected[record.ID] || changed == "" && relevant(record.ID) {
			out.Contexts = append(out.Contexts, SystemContext{record.ID, other, r})
		}
	}
	slices.SortFunc(out.Contexts, func(a, b SystemContext) int { return strings.Compare(a.Relation.ID, b.Relation.ID) })
	slices.SortFunc(out.Issues, func(a, b SystemIssue) int {
		return strings.Compare(a.EntityID+a.Code+a.Message+strings.Join(a.Related, ","), b.EntityID+b.Code+b.Message+strings.Join(b.Related, ","))
	})
	out.Issues = slices.CompactFunc(out.Issues, func(a, b SystemIssue) bool {
		return a.EntityID == b.EntityID && a.Code == b.Code && a.Message == b.Message && slices.Equal(a.Related, b.Related)
	})
	return out
}

func (d *Document) pruneSystems(age string) {
	for i := range d.Entities {
		e := &d.Entities[i]
		if e.AgeID != age || e.Deleted || e.System == nil {
			continue
		}
		if e.System.SystemID != "" {
			p := d.Entity(age, e.System.SystemID)
			if p == nil || p.Deleted {
				e.Deleted = true
			}
		}
	}
	for i := range d.Entities {
		e := &d.Entities[i]
		if e.AgeID != age || e.Deleted || e.System == nil {
			continue
		}
		keep := []SystemSource{}
		for _, ref := range e.System.Sources {
			missing := false
			if ref.EntityID != "" {
				target := d.Entity(age, ref.EntityID)
				missing = target == nil || target.Deleted
			}
			if ref.TerrainID != "" {
				exists := false
				for _, t := range d.Terrains {
					if t.AgeID == age && t.ID == ref.TerrainID {
						exists = true
					}
				}
				missing = missing || !exists
			}
			if missing {
				e.System.Unresolved = append(e.System.Unresolved, "Source removed from this Age: "+ref.EntityID+ref.OccurrenceID)
			} else {
				keep = append(keep, ref)
			}
		}
		e.System.Sources = keep
		for _, r := range d.Relations {
			if r.AgeID != age {
				continue
			}
			a, b := dependency(r)
			if a == e.ID {
				target := d.Entity(age, b)
				if target == nil || target.Deleted {
					e.System.Unresolved = append(e.System.Unresolved, "Prerequisite excluded from this Age: "+b)
				}
			}
		}
	}
}

func validateSystemRelation(r Relation) error {
	choices := map[string][]string{"system-knowledge": {"", "understood", "incorrect-theory", "disputed", "secret", "lost", "unknown"}, "system-access": {"", "available", "restricted", "unavailable", "unknown"}}
	if values, ok := choices[r.Kind]; ok && !slices.Contains(values, r.Status) {
		return fmt.Errorf("invalid knowledge or access state")
	}
	return nil
}
