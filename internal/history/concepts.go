package history

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
)

// ConceptContent is a versioned, validated composition schema, independent of
// the flexible legacy profile properties. Embedded records inherit their Age
// from the owning Entity; promoted records become ordinary logical entities.
type ConceptContent struct {
	Version    int                `json:"version"`
	Template   string             `json:"template,omitempty"`
	CustomType string             `json:"customType,omitempty"`
	Tags       []string           `json:"tags,omitempty"`
	Claim      *Claim             `json:"claim,omitempty"`
	Fields     []TypedField       `json:"fields,omitempty"`
	Contexts   []ConceptVariation `json:"contexts,omitempty"`
	Components []ConceptComponent `json:"components,omitempty"`
}
type Claim struct {
	Statement      string `json:"statement"`
	Subject        string `json:"subject,omitempty"`
	Predicate      string `json:"predicate,omitempty"`
	Object         string `json:"object,omitempty"`
	WorldTruth     string `json:"worldTruth,omitempty"`
	Interpretation string `json:"interpretation,omitempty"`
}
type FieldDefinition struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Options []string          `json:"options,omitempty"`
	Fields  []FieldDefinition `json:"fields,omitempty"`
}
type TypedField struct {
	Definition FieldDefinition `json:"definition"`
	Value      json.RawMessage `json:"value"`
}
type ConceptVariation struct {
	ID             string `json:"id"`
	ContextID      string `json:"contextId"`
	Description    string `json:"description,omitempty"`
	Stance         string `json:"stance,omitempty"`
	Prevalence     string `json:"prevalence,omitempty"`
	Interpretation string `json:"interpretation,omitempty"`
}
type ComponentLink struct {
	Authority      string `json:"authority,omitempty"`
	Prevalence     string `json:"prevalence,omitempty"`
	Interpretation string `json:"interpretation,omitempty"`
	Provenance     string `json:"provenance,omitempty"`
	ID             string `json:"id"`
	TargetID       string `json:"targetId"`
	Kind           string `json:"kind"`
	Description    string `json:"description,omitempty"`
	ContextID      string `json:"contextId,omitempty"`
	Stance         string `json:"stance,omitempty"`
	ValidFrom      string `json:"validFrom,omitempty"`
	ValidTo        string `json:"validTo,omitempty"`
}
type ConceptComponent struct {
	ID          string             `json:"id"`
	Type        string             `json:"type"`
	Title       string             `json:"title"`
	Description string             `json:"description,omitempty"`
	StartYear   *int64             `json:"startYear,omitempty"`
	EndYear     *int64             `json:"endYear,omitempty"`
	Claim       *Claim             `json:"claim,omitempty"`
	Fields      []TypedField       `json:"fields,omitempty"`
	Contexts    []ConceptVariation `json:"contexts,omitempty"`
	Links       []ComponentLink    `json:"links,omitempty"`
	EntityID    string             `json:"entityId,omitempty"`
}

var componentKinds = map[string]string{"belief": "doctrine", "narrative": "sacred-narrative", "practice": "practice", "value": "concept", "symbol": "cultural-symbol", "festival": "festival", "place": "concept", "variant": "cultural-variant", "history": "concept", "section": "concept"}

func cloneContent(c *ConceptContent) *ConceptContent {
	if c == nil {
		return nil
	}
	b, _ := json.Marshal(c)
	var copy ConceptContent
	_ = json.Unmarshal(b, &copy)
	return &copy
}
func (d *Document) conceptReference(age, id string) error {
	e := d.Entity(age, id)
	if e == nil || e.Deleted {
		return fmt.Errorf("concept reference %q must name an active entity in this Age", id)
	}
	return nil
}
func validateClaim(c *Claim) error {
	if c == nil {
		return nil
	}
	if strings.TrimSpace(c.Statement) == "" || len(c.Statement) > 30000 {
		return fmt.Errorf("claim needs a statement of at most 30000 characters")
	}
	if !slices.Contains([]string{"", "true", "false", "disputed", "unknown", "undecided", "not-applicable"}, c.WorldTruth) {
		return fmt.Errorf("invalid world-truth status")
	}
	return nil
}
func (d *Document) validateFields(age string, fields []TypedField) error {
	if len(fields) > 100 {
		return fmt.Errorf("at most 100 custom fields per section")
	}
	seen := map[string]bool{}
	for _, f := range fields {
		if seen[f.Definition.ID] {
			return fmt.Errorf("duplicate field ID")
		}
		seen[f.Definition.ID] = true
		if err := validateFieldDefinition(f.Definition, 0); err != nil {
			return err
		}
		if err := d.validateFieldValue(age, f.Definition, f.Value, 0); err != nil {
			return fmt.Errorf("field %s: %w", f.Definition.Name, err)
		}
	}
	return nil
}
func validateFieldDefinition(f FieldDefinition, depth int) error {
	if depth > 3 || f.ID == "" || len(f.ID) > 160 || strings.TrimSpace(f.Name) == "" || len(f.Name) > 200 {
		return fmt.Errorf("invalid custom field definition")
	}
	if !slices.Contains([]string{"text", "long-text", "number", "boolean", "date", "age", "select", "multi-select", "entity", "section"}, f.Type) {
		return fmt.Errorf("unknown custom field type")
	}
	if len(f.Options) > 100 || len(f.Fields) > 30 {
		return fmt.Errorf("custom field definition too large")
	}
	if f.Type == "select" || f.Type == "multi-select" {
		if len(f.Options) == 0 {
			return fmt.Errorf("selection fields need options")
		}
		seen := map[string]bool{}
		for _, o := range f.Options {
			if o == "" || len(o) > 200 || seen[o] {
				return fmt.Errorf("invalid or duplicate selection option")
			}
			seen[o] = true
		}
	}
	if f.Type == "section" {
		if len(f.Fields) == 0 {
			return fmt.Errorf("repeatable section needs typed subfields")
		}
		seen := map[string]bool{}
		for _, child := range f.Fields {
			if seen[child.ID] {
				return fmt.Errorf("duplicate subfield")
			}
			seen[child.ID] = true
			if err := validateFieldDefinition(child, depth+1); err != nil {
				return err
			}
		}
	} else if len(f.Fields) > 0 {
		return fmt.Errorf("only repeatable sections contain field definitions")
	}
	return nil
}
func (d *Document) validateFieldValue(age string, f FieldDefinition, raw json.RawMessage, depth int) error {
	if depth > 3 || len(raw) > 100000 {
		return fmt.Errorf("field value too large")
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return fmt.Errorf("invalid field value")
	}
	str, isString := v.(string)
	switch f.Type {
	case "text", "long-text":
		if !isString || len(str) > 30000 {
			return fmt.Errorf("text required")
		}
	case "number":
		n, ok := v.(float64)
		if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
			return fmt.Errorf("finite number required")
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("boolean required")
		}
	case "date":
		if _, ok := parseYear(v); !ok {
			return fmt.Errorf("date must be a whole year until a calendar is defined")
		}
	case "age":
		if !isString || d.Age(str) == nil {
			return fmt.Errorf("Age reference must belong to this World")
		}
	case "entity":
		if !isString {
			return fmt.Errorf("entity ID required")
		}
		return d.conceptReference(age, str)
	case "select":
		if !isString || !slices.Contains(f.Options, str) {
			return fmt.Errorf("choose a defined option")
		}
	case "multi-select":
		xs, ok := v.([]any)
		if !ok {
			return fmt.Errorf("list of options required")
		}
		seen := map[string]bool{}
		for _, x := range xs {
			s, ok := x.(string)
			if !ok || !slices.Contains(f.Options, s) || seen[s] {
				return fmt.Errorf("unknown or repeated option")
			}
			seen[s] = true
		}
	case "section":
		rows, ok := v.([]any)
		if !ok || len(rows) > 100 {
			return fmt.Errorf("repeatable section requires at most 100 rows")
		}
		defs := map[string]FieldDefinition{}
		for _, child := range f.Fields {
			defs[child.ID] = child
		}
		for _, row := range rows {
			m, ok := row.(map[string]any)
			if !ok {
				return fmt.Errorf("section row must be an object")
			}
			for key, val := range m {
				def, ok := defs[key]
				if !ok {
					return fmt.Errorf("undefined section field %s", key)
				}
				b, _ := json.Marshal(val)
				if err := d.validateFieldValue(age, def, b, depth+1); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
func (d *Document) validateVariations(age string, items []ConceptVariation) error {
	seen := map[string]bool{}
	if len(items) > 200 {
		return fmt.Errorf("too many contextual variations")
	}
	for _, v := range items {
		if v.ID == "" || seen[v.ID] {
			return fmt.Errorf("variation needs a unique ID")
		}
		seen[v.ID] = true
		if err := d.conceptReference(age, v.ContextID); err != nil {
			return err
		}
		if len(v.Description) > 30000 || len(v.Stance) > 200 || len(v.Prevalence) > 500 {
			return fmt.Errorf("variation text too long")
		}
	}
	return nil
}
func (d *Document) validateContent(e Entity) error {
	c := e.Content
	if c == nil || e.Deleted {
		return nil
	}
	if c.Version != 1 {
		return fmt.Errorf("unsupported concept content version %d", c.Version)
	}
	if len(c.Components) > 300 || len(c.Tags) > 100 || len(c.CustomType) > 200 {
		return fmt.Errorf("concept content too large")
	}
	for _, tag := range c.Tags {
		if len(tag) > 100 {
			return fmt.Errorf("tag too long")
		}
	}
	if err := validateClaim(c.Claim); err != nil {
		return err
	}
	if err := d.validateFields(e.AgeID, c.Fields); err != nil {
		return err
	}
	if err := d.validateVariations(e.AgeID, c.Contexts); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, part := range c.Components {
		kind, ok := componentKinds[part.Type]
		if !ok || part.ID == "" || len(part.ID) > 160 || seen[part.ID] || strings.TrimSpace(part.Title) == "" || len(part.Title) > 200 || len(part.Description) > 100000 {
			return fmt.Errorf("component requires a supported type, unique ID and title")
		}
		seen[part.ID] = true
		if part.EntityID != "" {
			if part.EntityID == e.ID {
				return fmt.Errorf("component cannot link to itself")
			}
			if err := d.conceptReference(e.AgeID, part.EntityID); err != nil {
				return err
			}
			if part.Claim != nil || len(part.Fields) > 0 || len(part.Contexts) > 0 || len(part.Links) > 0 || part.StartYear != nil || part.EndYear != nil {
				return fmt.Errorf("promoted component must use its independent record")
			}
			continue
		}
		for _, year := range []*int64{part.StartYear, part.EndYear} {
			if year != nil && (*year < -9007199254740991 || *year > 9007199254740991) {
				return fmt.Errorf("component year out of range")
			}
		}
		if err := validateClaim(part.Claim); err != nil {
			return err
		}
		if err := d.validateFields(e.AgeID, part.Fields); err != nil {
			return err
		}
		if err := d.validateVariations(e.AgeID, part.Contexts); err != nil {
			return err
		}
		if len(part.Links) > 200 {
			return fmt.Errorf("too many component links")
		}
		links := map[string]bool{}
		for _, l := range part.Links {
			if l.ID == "" || links[l.ID] {
				return fmt.Errorf("component link needs unique ID")
			}
			links[l.ID] = true
			for _, text := range []string{l.Description, l.Stance, l.Authority, l.Prevalence, l.Interpretation, l.Provenance, l.ValidFrom, l.ValidTo} {
				if len(text) > 30000 {
					return fmt.Errorf("component relationship metadata too long")
				}
			}
			if err := d.conceptReference(e.AgeID, l.TargetID); err != nil {
				return err
			}
			rule, ok := Registry.Relations[l.Kind]
			if !ok || rule.Family != "cultural" || !matchesType(&Entity{Kind: kind}, rule.Sources) || !matchesType(d.Entity(e.AgeID, l.TargetID), rule.Destinations) {
				return fmt.Errorf("invalid component relationship type or endpoint")
			}
			if l.ContextID != "" {
				if err := d.conceptReference(e.AgeID, l.ContextID); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Promotion is atomic and leaves a reference at the original article position.
func (d *Document) promoteComponent(c Command) error {
	b, _ := json.Marshal(d)
	var next Document
	_ = json.Unmarshal(b, &next)
	parent := next.Entity(c.AgeID, c.EntityID)
	if parent == nil || parent.Deleted || parent.Content == nil {
		return fmt.Errorf("concept not found")
	}
	index := -1
	for i, part := range parent.Content.Components {
		if part.ID == c.ComponentID {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("component not found")
	}
	part := parent.Content.Components[index]
	if part.EntityID != "" {
		return fmt.Errorf("component is already independent")
	}
	props, _ := json.Marshal(map[string]any{"summary": part.Description, "startYear": part.StartYear, "endYear": part.EndYear})
	entity := Entity{ID: ID(), Name: part.Title, Kind: componentKinds[part.Type], Properties: props, Content: &ConceptContent{Version: 1, Claim: part.Claim, Fields: part.Fields, Contexts: part.Contexts}}
	parentID := parent.ID
	if err := next.Apply(Command{Kind: "entity", AgeID: c.AgeID, Entity: &entity}); err != nil {
		return err
	}
	// Appending the promoted entity may relocate the slice.
	parent = next.Entity(c.AgeID, parentID)
	edited := *parent
	edited.Content = cloneContent(parent.Content)
	edited.Content.Components[index] = ConceptComponent{ID: part.ID, Type: part.Type, Title: part.Title, EntityID: entity.ID}
	if err := next.Apply(Command{Kind: "entity", AgeID: c.AgeID, Entity: &edited}); err != nil {
		return err
	}
	relation := Relation{Source: parentID, Destination: entity.ID, Kind: "has-component", Description: "Promoted from embedded content"}
	if err := next.Apply(Command{Kind: "relation", AgeID: c.AgeID, Relation: &relation}); err != nil {
		return err
	}
	for _, link := range part.Links {
		r := Relation{ID: part.ID + "/" + link.ID, Source: entity.ID, Destination: link.TargetID, Kind: link.Kind, Description: link.Description, ContextID: link.ContextID, Stance: link.Stance, Authority: link.Authority, Prevalence: link.Prevalence, Interpretation: link.Interpretation, Provenance: link.Provenance, ValidFrom: link.ValidFrom, ValidTo: link.ValidTo}
		if err := next.Apply(Command{Kind: "relation", AgeID: c.AgeID, Relation: &r}); err != nil {
			return err
		}
	}
	*d = next
	return nil
}

// Referenced content must be explicitly edited before removing its subject.
// Validate against a copy so rejected deletion never leaves partial tombstones.
func (d *Document) validateConceptDeletion(age string, ids []string) error {
	next := *d
	next.Entities = append([]Entity(nil), d.Entities...)
	for i := range next.Entities {
		if next.Entities[i].AgeID == age && slices.Contains(ids, next.Entities[i].ID) {
			next.Entities[i].Deleted = true
		}
	}
	for _, e := range next.Entities {
		if e.AgeID == age && !e.Deleted && e.Content != nil {
			if err := next.validateContent(e); err != nil {
				return fmt.Errorf("edit referenced content in %s before deleting: %w", e.Name, err)
			}
		}
	}
	return nil
}
