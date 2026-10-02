package project

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

type ContentImportRequest struct {
	Expected string `json:"expected"`
	Age      string `json:"age"`
	Format   string `json:"format"`
	Name     string `json:"name"`
	Text     string `json:"text"`
}
type ContentPreview struct {
	Revision string   `json:"revision"`
	Records  []Record `json:"records"`
	Warnings []string `json:"warnings"`
}

func (s *Store) PreviewContent(c ContentImportRequest) (ContentPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := ContentPreview{Records: []Record{}, Warnings: []string{}}
	st, e := s.state(c.Age)
	if e != nil {
		return out, e
	}
	if c.Expected != st.Revision {
		return out, ErrConflict
	}
	out.Revision = st.Revision
	if len(c.Text) > 2<<20 || !utf8.ValidString(c.Text) || !nameOK(c.Name) {
		return out, fmt.Errorf("use valid UTF-8 text up to 2 MiB and a source name")
	}
	digest := sha256.Sum256([]byte(c.Text))
	provenance := map[string]any{"name": c.Name, "format": c.Format, "sha256": hex.EncodeToString(digest[:])}
	add := func(r Record) error {
		if len(out.Records) >= 500 {
			return fmt.Errorf("import at most 500 entries per preview")
		}
		r.ID = NewID()
		if r.Kind == "" {
			r.Kind = "entity"
		}
		if r.Kind != "entity" && r.Kind != "note" && r.Kind != "scene" {
			return fmt.Errorf("text imports support entities, notes and scene text")
		}
		if r.Kind == "entity" && r.Type == "" {
			r.Type = "Imported entity"
		}
		if r.Properties == nil {
			r.Properties = map[string]any{}
		}
		for key := range r.Properties {
			if strings.HasPrefix(key, "_") && key != "_domain" {
				return fmt.Errorf("reserved property in imported entry")
			}
		}
		r.Properties["importSource"] = provenance
		out.Records = append(out.Records, r)
		return nil
	}
	switch c.Format {
	case "csv":
		reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(c.Text, "\ufeff")))
		header, e := reader.Read()
		if e != nil {
			return out, fmt.Errorf("CSV header: %w", e)
		}
		keys := map[string]bool{}
		for i, key := range header {
			key = strings.TrimSpace(key)
			if key == "" || keys[key] {
				return out, fmt.Errorf("CSV headers must be unique and nonempty")
			}
			header[i] = key
			keys[key] = true
		}
		if !keys["name"] {
			return out, fmt.Errorf("CSV requires a name column")
		}
		for {
			row, e := reader.Read()
			if e == io.EOF {
				break
			}
			if e != nil {
				return out, fmt.Errorf("CSV row: %w", e)
			}
			r := Record{Properties: map[string]any{}}
			for i, value := range row {
				switch header[i] {
				case "name":
					r.Name = value
				case "type":
					r.Type = value
				case "kind":
					r.Kind = value
				case "notes":
					r.Notes = value
				case "id":
					r.Properties["sourceID"] = value
				default:
					r.Properties[header[i]] = value
				}
			}
			if r.Kind == "scene" {
				return out, fmt.Errorf("use Markdown/plain text to import scenes")
			}
			if e = add(r); e != nil {
				return out, e
			}
		}
		out.Warnings = append(out.Warnings, "CSV columns become text properties; quantity structures and entity links are not inferred. Source IDs are retained as metadata, not merged with existing identities.")
	case "json":
		var rows []struct {
			Name       string         `json:"name"`
			Type       string         `json:"type"`
			Kind       string         `json:"kind"`
			Notes      string         `json:"notes"`
			ID         string         `json:"id"`
			Properties map[string]any `json:"properties"`
		}
		decoder := json.NewDecoder(strings.NewReader(c.Text))
		decoder.DisallowUnknownFields()
		if e = decoder.Decode(&rows); e != nil {
			return out, fmt.Errorf("JSON must be an array of name/type/kind/notes/id/properties entries: %w", e)
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			return out, fmt.Errorf("unexpected trailing JSON")
		}
		for _, row := range rows {
			if row.Kind == "scene" {
				return out, fmt.Errorf("use Markdown/plain text to import scenes")
			}
			r := Record{Name: row.Name, Type: row.Type, Kind: row.Kind, Notes: row.Notes, Properties: row.Properties}
			if r.Properties == nil {
				r.Properties = map[string]any{}
			}
			if row.ID != "" {
				r.Properties["sourceID"] = row.ID
			}
			if e = add(r); e != nil {
				return out, e
			}
		}
		out.Warnings = append(out.Warnings, "Every row becomes a new identity. Entity-valued properties must use valid existing IDs in the source Age; names are never matched automatically.")
	case "markdown":
		// Preserve Markdown source as text, rather than executing HTML or losing
		// unfamiliar extensions through a partial parser.
		paragraphs := []any{}
		for _, line := range strings.Split(strings.ReplaceAll(c.Text, "\r\n", "\n"), "\n") {
			node := map[string]any{"type": "paragraph"}
			if line != "" {
				node["content"] = []any{map[string]any{"type": "text", "text": line}}
			}
			paragraphs = append(paragraphs, node)
		}
		doc, _ := json.Marshal(map[string]any{"type": "doc", "content": paragraphs})
		e = add(Record{Kind: "scene", Name: c.Name, Document: doc, SettingAge: st.Age.ID, SettingSnapshot: st.Age.Snapshot, Order: len(st.Records) + 1})
		if e != nil {
			return out, e
		}
		out.Warnings = append(out.Warnings, "Markdown syntax is preserved literally in one scene. Formatting, remote images and executable HTML are not interpreted; the original text remains editable.")
	default:
		return out, fmt.Errorf("supported text import formats: csv, json, markdown")
	}
	if len(out.Records) == 0 {
		return out, fmt.Errorf("no importable entries")
	}
	records := map[string]Record{}
	for id, r := range st.Records {
		records[id] = r
	}
	for _, r := range out.Records {
		records[r.ID] = r
	}
	if e = validateRecords(records); e != nil {
		return out, e
	}
	out.Warnings = append(out.Warnings, "Acceptance creates a separate Age derived from the selected Age, preserving its history.")
	return out, nil
}
