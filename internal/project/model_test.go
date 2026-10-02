package project

import (
	"encoding/json"
	"testing"
)

func TestRichTextStructure(t *testing.T) {
	valid := []string{
		`{"type":"doc","content":[{"type":"paragraph"}]}`,
		`{"type":"doc","content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Title","marks":[{"type":"bold"}]}]},{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Item"}]}]}]}]}`,
	}
	invalid := []string{
		`{"type":"doc","content":[{"type":"text","text":"unwrapped"}]}`,
		`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"doc","content":[{"type":"paragraph"}]}]}]}`,
		`{"type":"doc","content":[{"type":"bulletList","content":[{"type":"paragraph"}]}]}`,
		`{"type":"doc","content":[{"type":"heading","attrs":{"level":99}}]}`,
		`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"link"}]}]}]}`,
		`{"type":"doc","content":[]}`,
	}
	for _, raw := range valid {
		if e := validateDocument(json.RawMessage(raw)); e != nil {
			t.Fatalf("valid document rejected: %v", e)
		}
	}
	for _, raw := range invalid {
		if e := validateDocument(json.RawMessage(raw)); e == nil {
			t.Fatalf("invalid document accepted: %s", raw)
		}
	}
}
