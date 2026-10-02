package project

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func TestDOCXPreservesManuscriptStructureWithoutPrivateMetadata(t *testing.T) {
	s, st := fixture(t)
	scene := simpleScene(st)
	scene.Name = "Chapter <one>"
	scene.Properties = map[string]any{"private": "PRIVATE_PLANNING_SENTINEL"}
	scene.Document = json.RawMessage(`{"type":"doc","content":[{"type":"heading","attrs":{"level":2,"blockID":"PRIVATE_BLOCK_SENTINEL"},"content":[{"type":"text","text":"A heading"}]},{"type":"paragraph","content":[{"type":"text","text":"Bold & italic","marks":[{"type":"bold"},{"type":"italic"}]},{"type":"hardBreak"},{"type":"text","text":"underlined","marks":[{"type":"underline"}]},{"type":"text","text":"struck","marks":[{"type":"strike"}]}]},{"type":"orderedList","attrs":{"start":5},"content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Ordered item"}]},{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Nested bullet"}]}]}]}]}]},{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"A quotation"}]}]},{"type":"codeBlock","content":[{"type":"text","text":"one\n\ttwo"}]},{"type":"paragraph","content":[{"type":"text","text":"حكاية النهر"}]},{"type":"horizontalRule"}]}`)
	st = put(t, s, st, scene)
	result, e := s.Export(ExportRequest{Snapshot: st.Age.Snapshot, IDs: []string{scene.ID}, Format: "docx", Title: "A selected book"})
	if e != nil {
		t.Fatal(e)
	}
	z, e := zip.NewReader(bytes.NewReader(result.Data), int64(len(result.Data)))
	if e != nil {
		t.Fatal(e)
	}
	files := map[string]string{}
	for _, f := range z.File {
		input, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		raw, e := io.ReadAll(input)
		input.Close()
		if e != nil {
			t.Fatal(e)
		}
		if bytes.Contains(raw, []byte("PRIVATE_")) {
			t.Fatal("private planning or block metadata exported")
		}
		decoder := xml.NewDecoder(bytes.NewReader(raw))
		for {
			_, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("invalid XML %s: %v", f.Name, err)
			}
		}
		files[f.Name] = string(raw)
	}
	document := files["word/document.xml"]
	for _, want := range []string{`w:val="Title"`, `w:val="Heading2"`, `<w:b/>`, `<w:i/>`, `<w:u w:val="single"/>`, `<w:strike/>`, `<w:br/>`, `<w:tab/>`, `<w:numId w:val="1"/>`, `<w:numId w:val="2"/>`, `w:val="Quote"`, `w:val="Code"`, `<w:bidi/>`, `<w:rtl/>`, `Chapter &lt;one&gt;`, `Bold &amp; italic`, `حكاية النهر`, `Nested bullet`, `<w:pBdr>`} {
		if !strings.Contains(document, want) {
			t.Errorf("missing DOCX structure %s", want)
		}
	}
	for _, want := range []string{`<w:start w:val="5"/>`, `w:val="bullet"`, `w:left="1440"`} {
		if !strings.Contains(files["word/numbering.xml"], want) {
			t.Errorf("missing numbering %s", want)
		}
	}
	if !strings.Contains(files["word/_rels/document.xml.rels"], `Target="styles.xml"`) || !strings.Contains(files["[Content_Types].xml"], `PartName="/word/numbering.xml"`) {
		t.Fatal("missing package part relationships")
	}
}

func TestMixedExportSelectionHasDeterministicTotalOrder(t *testing.T) {
	s, st := fixture(t)
	a, b := simpleScene(st), simpleScene(st)
	a.Name, a.Order = "Z chapter", 1
	b.Name, b.Order = "A chapter", 2
	middle := Record{ID: NewID(), Kind: "note", Name: "M note"}
	st = apply(t, s, st, Command{Action: "put-many", Records: []Record{a, b, middle}})
	first, e := s.Export(ExportRequest{Snapshot: st.Age.Snapshot, IDs: []string{a.ID, middle.ID, b.ID}, Format: "docx"})
	if e != nil {
		t.Fatal(e)
	}
	second, e := s.Export(ExportRequest{Snapshot: st.Age.Snapshot, IDs: []string{b.ID, middle.ID, a.ID}, Format: "docx"})
	if e != nil || !bytes.Equal(first.Data, second.Data) {
		t.Fatal("selection order changed the compiled document", e)
	}
}
