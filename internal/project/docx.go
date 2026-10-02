package project

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

type exportNode struct {
	Type  string         `json:"type"`
	Text  string         `json:"text"`
	Attrs map[string]any `json:"attrs"`
	Marks []struct {
		Type string `json:"type"`
	} `json:"marks"`
	Content []exportNode `json:"content"`
}
type wordList struct {
	ordered       bool
	start, indent int
}
type wordWriter struct {
	language string
	body     strings.Builder
	lists    []wordList
}
type wordContext struct {
	list, indent int
	quote        bool
}

func firstStrongRTL(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Arabic, r) || unicode.Is(unicode.Hebrew, r) {
			return true
		}
		if unicode.IsLetter(r) {
			return false
		}
	}
	return false
}
func nodeText(node exportNode) string {
	var b strings.Builder
	b.WriteString(node.Text)
	for _, child := range node.Content {
		b.WriteString(nodeText(child))
	}
	return b.String()
}
func wordRun(node exportNode, code bool) string {
	if node.Type == "hardBreak" {
		return "<w:r><w:br/></w:r>"
	}
	if node.Type != "text" {
		var b strings.Builder
		for _, child := range node.Content {
			b.WriteString(wordRun(child, code))
		}
		return b.String()
	}
	flags := map[string]bool{}
	for _, m := range node.Marks {
		flags[m.Type] = true
	}
	var properties, content strings.Builder
	if code || flags["code"] {
		properties.WriteString(`<w:rFonts w:ascii="Courier New" w:hAnsi="Courier New"/>`)
	}
	if flags["bold"] {
		properties.WriteString(`<w:b/><w:bCs/>`)
	}
	if flags["italic"] {
		properties.WriteString(`<w:i/><w:iCs/>`)
	}
	if flags["strike"] {
		properties.WriteString(`<w:strike/>`)
	}
	if flags["underline"] {
		properties.WriteString(`<w:u w:val="single"/>`)
	}
	if firstStrongRTL(node.Text) {
		properties.WriteString(`<w:rtl/>`)
	}
	for i, line := range strings.Split(node.Text, "\n") {
		if i > 0 {
			content.WriteString(`<w:br/>`)
		}
		for j, part := range strings.Split(line, "\t") {
			if j > 0 {
				content.WriteString(`<w:tab/>`)
			}
			content.WriteString(`<w:t xml:space="preserve">` + xmlText(part) + `</w:t>`)
		}
	}
	return `<w:r><w:rPr>` + properties.String() + `</w:rPr>` + content.String() + `</w:r>`
}
func (w *wordWriter) paragraph(node exportNode, style string, context wordContext) {
	var properties strings.Builder
	if style != "" {
		properties.WriteString(`<w:pStyle w:val="` + style + `"/>`)
	}
	if context.list > 0 {
		fmt.Fprintf(&properties, `<w:numPr><w:ilvl w:val="0"/><w:numId w:val="%d"/></w:numPr>`, context.list)
	}
	if firstStrongRTL(nodeText(node)) {
		properties.WriteString(`<w:bidi/>`)
	}
	if context.indent > 0 {
		fmt.Fprintf(&properties, `<w:ind w:left="%d"/>`, context.indent)
	}
	w.body.WriteString(`<w:p><w:pPr>` + properties.String() + `</w:pPr>`)
	for _, child := range node.Content {
		w.body.WriteString(wordRun(child, node.Type == "codeBlock"))
	}
	w.body.WriteString(`</w:p>`)
}
func (w *wordWriter) walk(node exportNode, context wordContext) {
	switch node.Type {
	case "paragraph", "heading", "codeBlock":
		style := "Normal"
		if context.quote {
			style = "Quote"
		}
		if node.Type == "codeBlock" {
			style = "Code"
		}
		if node.Type == "heading" {
			level, _ := node.Attrs["level"].(float64)
			style = fmt.Sprintf("Heading%d", min(6, max(1, int(level))))
		}
		w.paragraph(node, style, context)
	case "bulletList", "orderedList":
		start := 1
		if n, ok := node.Attrs["start"].(float64); ok && n >= 1 && n <= 1000000 {
			start = int(n)
		}
		indent := context.indent + 720
		w.lists = append(w.lists, wordList{node.Type == "orderedList", start, indent})
		id := len(w.lists)
		for _, item := range node.Content {
			first := true
			for _, child := range item.Content {
				current := wordContext{indent: indent, quote: context.quote}
				if first && (child.Type == "paragraph" || child.Type == "heading" || child.Type == "codeBlock") {
					current.list = id
					first = false
				}
				w.walk(child, current)
			}
		}
	case "blockquote":
		context.quote = true
		context.indent += 720
		for _, child := range node.Content {
			w.walk(child, context)
		}
	case "horizontalRule":
		w.body.WriteString(`<w:p><w:pPr><w:pBdr><w:bottom w:val="single" w:sz="4" w:space="1" w:color="auto"/></w:pBdr></w:pPr></w:p>`)
	default:
		for _, child := range node.Content {
			w.walk(child, context)
		}
	}
}
func (w *wordWriter) text(text, style string) {
	w.paragraph(exportNode{Type: "paragraph", Content: []exportNode{{Type: "text", Text: text}}}, style, wordContext{})
}
func (w *wordWriter) manuscript(raw json.RawMessage) error {
	var node exportNode
	if e := json.Unmarshal(raw, &node); e != nil {
		return e
	}
	w.walk(node, wordContext{})
	return nil
}
func wordStyles(language string) string {
	var b strings.Builder
	b.WriteString(`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:docDefaults><w:rPrDefault><w:rPr><w:lang w:val="` + xmlText(language) + `"/></w:rPr></w:rPrDefault></w:docDefaults><w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style>`)
	for i := 1; i <= 6; i++ {
		fmt.Fprintf(&b, `<w:style w:type="paragraph" w:styleId="Heading%d"><w:name w:val="heading %d"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:keepNext/><w:outlineLvl w:val="%d"/></w:pPr><w:rPr><w:b/><w:sz w:val="%d"/></w:rPr></w:style>`, i, i, i-1, 36-(i-1)*2)
	}
	b.WriteString(`<w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:basedOn w:val="Normal"/><w:rPr><w:b/><w:sz w:val="44"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="Quote"><w:name w:val="Quote"/><w:basedOn w:val="Normal"/></w:style><w:style w:type="paragraph" w:styleId="Code"><w:name w:val="Code"/><w:basedOn w:val="Normal"/><w:rPr><w:rFonts w:ascii="Courier New" w:hAnsi="Courier New"/></w:rPr></w:style></w:styles>`)
	return b.String()
}
func (w *wordWriter) files() map[string][]byte {
	var numbering strings.Builder
	numbering.WriteString(`<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`)
	for i, list := range w.lists {
		format, text := "bullet", "•"
		if list.ordered {
			format, text = "decimal", "%1."
		}
		fmt.Fprintf(&numbering, `<w:abstractNum w:abstractNumId="%d"><w:multiLevelType w:val="singleLevel"/><w:lvl w:ilvl="0"><w:start w:val="%d"/><w:numFmt w:val="%s"/><w:lvlText w:val="%s"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="%d" w:hanging="360"/></w:pPr></w:lvl></w:abstractNum>`, i, list.start, format, text, list.indent)
	}
	for i := range w.lists {
		fmt.Fprintf(&numbering, `<w:num w:numId="%d"><w:abstractNumId w:val="%d"/></w:num>`, i+1, i)
	}
	numbering.WriteString(`</w:numbering>`)
	return map[string][]byte{
		"[Content_Types].xml":          []byte(`<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/><Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/></Types>`),
		"_rels/.rels":                  []byte(`<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="document" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`),
		"word/_rels/document.xml.rels": []byte(`<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="styles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/><Relationship Id="numbering" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/></Relationships>`),
		"word/document.xml":            []byte(`<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + w.body.String() + `<w:sectPr/></w:body></w:document>`),
		"word/styles.xml":              []byte(wordStyles(w.language)),
		"word/numbering.xml":           []byte(numbering.String()),
	}
}
