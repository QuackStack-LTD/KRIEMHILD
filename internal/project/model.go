package project

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

const Format = 2

type Marker struct {
	Version int    `json:"version"`
	ID      string `json:"id"`
	Name    string `json:"name"`
}
type Head struct {
	Revision string `json:"revision"`
}
type Root struct {
	Version      int            `json:"version"`
	World        Marker         `json:"world"`
	Ages         map[string]Age `json:"ages"`
	Parent       string         `json:"parent,omitempty"`
	MergeParents []string       `json:"mergeParents,omitempty"`
	Message      string         `json:"message"`
	SavedAt      string         `json:"savedAt"`
}
type Age struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Snapshot       string   `json:"snapshot"`
	SourceAge      string   `json:"sourceAge,omitempty"`
	SourceSnapshot string   `json:"sourceSnapshot,omitempty"`
	SourceRevision string   `json:"sourceRevision,omitempty"`
	Undo           []string `json:"undo"`
	Redo           []string `json:"redo"`
}
type Snapshot struct {
	Version int               `json:"version"`
	Records map[string]string `json:"records"`
}
type Field struct {
	Key  string `json:"key"`
	Type string `json:"type"`
}
type Pin struct {
	Entity string  `json:"entity"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}
type Record struct {
	ID              string          `json:"id"`
	Kind            string          `json:"kind"`
	Name            string          `json:"name"`
	Notes           string          `json:"notes,omitempty"`
	Type            string          `json:"type,omitempty"`
	Status          string          `json:"status,omitempty"`
	Properties      map[string]any  `json:"properties,omitempty"`
	Fields          []Field         `json:"fields,omitempty"`
	From            string          `json:"from,omitempty"`
	To              string          `json:"to,omitempty"`
	Asset           string          `json:"asset,omitempty"`
	Width           int             `json:"width,omitempty"`
	Height          int             `json:"height,omitempty"`
	Pins            []Pin           `json:"pins,omitempty"`
	Document        json.RawMessage `json:"document,omitempty"`
	Story           string          `json:"story,omitempty"`
	Order           int             `json:"order,omitempty"`
	SettingAge      string          `json:"settingAge,omitempty"`
	SettingSnapshot string          `json:"settingSnapshot,omitempty"`
	References      []string        `json:"references,omitempty"`
	Terrain         *Terrain        `json:"terrain,omitempty"`
	Features        []Feature       `json:"features,omitempty"`
	Calendar        *Calendar       `json:"calendar,omitempty"`
	Chronology      *Chronology     `json:"chronology,omitempty"`
	Event           *Event          `json:"event,omitempty"`
	SettingDate     string          `json:"settingDate,omitempty"`
}
type State struct {
	Role      string            `json:"role,omitempty"`
	ReadOnly  bool              `json:"readOnly,omitempty"`
	Revision  string            `json:"revision"`
	Root      Root              `json:"root"`
	Age       Age               `json:"age"`
	Records   map[string]Record `json:"records"`
	Recovered bool              `json:"recovered"`
}
type Command struct {
	Expected   string      `json:"expected"`
	Action     string      `json:"action"`
	Age        string      `json:"age"`
	Name       string      `json:"name,omitempty"`
	Record     *Record     `json:"record,omitempty"`
	ID         string      `json:"id,omitempty"`
	IDs        []string    `json:"ids,omitempty"`
	Incoming   string      `json:"incoming,omitempty"`
	Records    []Record    `json:"records,omitempty"`
	Event      *Record     `json:"event,omitempty"`
	Chronology *Chronology `json:"chronology,omitempty"`
}
type Difference struct {
	ID       string  `json:"id"`
	Change   string  `json:"change"`
	Before   *Record `json:"before"`
	After    *Record `json:"after"`
	Conflict bool    `json:"conflict"`
	Current  *Record `json:"current,omitempty"`
}

func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	h := hex.EncodeToString(b)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}
func validID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef-", c) {
			return false
		}
	}
	return true
}
func validHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, e := hex.DecodeString(s)
	return e == nil
}
func nameOK(s string) bool { return strings.TrimSpace(s) != "" && utf8.RuneCountInString(s) <= 300 }

func validateDocument(raw json.RawMessage) error {
	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		return fmt.Errorf("invalid manuscript document")
	}
	count := 0
	var walk func(map[string]any, int) error
	walk = func(n map[string]any, depth int) error {
		count++
		if depth > 40 || count > 100000 {
			return fmt.Errorf("manuscript is too complex")
		}
		t, _ := n["type"].(string)
		allowed := map[string]bool{"doc": true, "paragraph": true, "text": true, "heading": true, "bulletList": true, "orderedList": true, "listItem": true, "blockquote": true, "codeBlock": true, "hardBreak": true, "horizontalRule": true}
		if !allowed[t] {
			return fmt.Errorf("unsupported document node %q", t)
		}
		if t == "doc" && depth != 0 {
			return fmt.Errorf("nested manuscript root")
		}
		children, _ := n["content"].([]any)
		if t == "text" {
			text, ok := n["text"].(string)
			if !ok || text == "" || len(children) != 0 {
				return fmt.Errorf("invalid text node")
			}
		}
		if (t == "hardBreak" || t == "horizontalRule") && len(children) != 0 {
			return fmt.Errorf("leaf node cannot contain children")
		}
		if (t == "doc" || t == "blockquote" || t == "bulletList" || t == "orderedList" || t == "listItem") && len(children) == 0 {
			return fmt.Errorf("%s requires content", t)
		}
		if attrs, ok := n["attrs"]; ok && attrs != nil {
			a, ok := attrs.(map[string]any)
			if !ok {
				return fmt.Errorf("invalid node attributes")
			}
			if t == "heading" {
				if level, ok := a["level"].(float64); !ok || level < 1 || level > 6 || level != math.Trunc(level) {
					return fmt.Errorf("invalid heading level")
				}
			}
		}
		if text, ok := n["text"]; ok {
			if _, ok = text.(string); !ok {
				return fmt.Errorf("invalid text")
			}
		}
		if content, ok := n["content"]; ok {
			children, ok := content.([]any)
			if !ok {
				return fmt.Errorf("invalid document content")
			}
			for i, child := range children {
				c, ok := child.(map[string]any)
				if !ok {
					return fmt.Errorf("invalid document node")
				}
				ct, _ := c["type"].(string)
				block := ct == "paragraph" || ct == "heading" || ct == "blockquote" || ct == "codeBlock" || ct == "bulletList" || ct == "orderedList" || ct == "horizontalRule"
				validChild := false
				switch t {
				case "doc", "blockquote":
					validChild = block
				case "listItem":
					validChild = block && (i != 0 || ct == "paragraph")
				case "bulletList", "orderedList":
					validChild = ct == "listItem"
				case "paragraph", "heading":
					validChild = ct == "text" || ct == "hardBreak"
				case "codeBlock":
					validChild = ct == "text"
				}
				if !validChild {
					return fmt.Errorf("%s cannot contain %s", t, ct)
				}
				if err := walk(c, depth+1); err != nil {
					return err
				}
			}
		}
		if marks, ok := n["marks"]; ok {
			ms, ok := marks.([]any)
			if !ok {
				return fmt.Errorf("invalid marks")
			}
			for _, m := range ms {
				mark, ok := m.(map[string]any)
				if !ok {
					return fmt.Errorf("invalid mark")
				}
				mt, _ := mark["type"].(string)
				if mt != "bold" && mt != "italic" && mt != "strike" && mt != "code" && mt != "underline" {
					return fmt.Errorf("unsupported mark %q", mt)
				}
			}
		}
		return nil
	}
	if node["type"] != "doc" {
		return fmt.Errorf("manuscript root must be doc")
	}
	return walk(node, 0)
}

func validateRecords(records map[string]Record) error {
	entities := func(id string) bool { r, ok := records[id]; return ok && r.Kind == "entity" }
	schemas := map[string]Record{}
	for id, r := range records {
		if !validID(id) || id != r.ID || !nameOK(r.Name) {
			return fmt.Errorf("record needs a valid ID and a name of 1–300 characters")
		}
		if r.Kind == "schema" {
			if _, ok := schemas[r.Name]; ok {
				return fmt.Errorf("duplicate entity type %q", r.Name)
			}
			schemas[r.Name] = r
		}
	}
	for _, r := range records {
		if r.Asset != "" && !validHash(r.Asset) {
			return fmt.Errorf("invalid asset reference")
		}
		if err := validateLocalMap(r, records); err != nil {
			return err
		}
		if err := validateDomains(r, records); err != nil {
			return err
		}
		if err := validateProduction(r, records); err != nil {
			return err
		}
		if err := validateP2(r, records); err != nil {
			return err
		}
		switch r.Kind {
		case "schema":
			seen := map[string]bool{}
			for _, f := range r.Fields {
				if !nameOK(f.Key) || seen[f.Key] {
					return fmt.Errorf("field names must be unique and nonempty")
				}
				seen[f.Key] = true
				switch f.Type {
				case "text", "number", "boolean", "entity":
				default:
					return fmt.Errorf("unsupported field type")
				}
			}
		case "entity":
			if !nameOK(r.Type) {
				return fmt.Errorf("entity type is required")
			}
			if r.Status != "" && r.Status != "active" && r.Status != "destroyed" && r.Status != "unknown" {
				return fmt.Errorf("invalid lifecycle status")
			}
			if schema, ok := schemas[r.Type]; ok {
				for _, f := range schema.Fields {
					v, exists := r.Properties[f.Key]
					if !exists || v == nil || v == "" {
						continue
					}
					valid := false
					switch f.Type {
					case "text":
						_, valid = v.(string)
					case "number":
						_, valid = v.(float64)
					case "boolean":
						_, valid = v.(bool)
					case "entity":
						s, ok := v.(string)
						valid = ok && entities(s)
					}
					if !valid {
						return fmt.Errorf("%s: invalid value for %s", r.Name, f.Key)
					}
				}
			}
		case "relation":
			if !entities(r.From) || !entities(r.To) {
				return fmt.Errorf("relationship endpoints must exist in this Age")
			}
		case "map":
			if r.Asset != "" && !validHash(r.Asset) {
				return fmt.Errorf("invalid image reference")
			}
			if r.Width < 1 || r.Height < 1 || r.Width > 20000 || r.Height > 20000 {
				return fmt.Errorf("invalid map dimensions")
			}
			pins := map[string]bool{}
			for _, p := range r.Pins {
				if pins[p.Entity] {
					return fmt.Errorf("an entity can have only one pin per map")
				}
				pins[p.Entity] = true
				if !entities(p.Entity) || math.IsNaN(p.X) || math.IsNaN(p.Y) || p.X < 0 || p.X > 1 || p.Y < 0 || p.Y > 1 {
					return fmt.Errorf("map pins need existing entities and coordinates between 0 and 1")
				}
			}
		case "scene":
			if !validID(r.SettingAge) || !validHash(r.SettingSnapshot) {
				return fmt.Errorf("scene setting must pin an Age snapshot")
			}
			if err := validateDocument(r.Document); err != nil {
				return err
			}
		case "note", "calendar", "chronology", "event":
		default:
			return fmt.Errorf("unsupported record kind %q", r.Kind)
		}
	}
	return nil
}
