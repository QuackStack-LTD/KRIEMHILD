package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

var editionLanguage = regexp.MustCompile(`(?i)^(?:[a-z]{2,8}(?:-[a-z0-9]{1,8})*|x(?:-[a-z0-9]{1,8})+)$`)

func epubFiles(c ExportRequest, title, body string, records []Record) map[string][]byte {
	ids := make([]string, 0, len(records))
	var links strings.Builder
	for _, r := range records {
		ids = append(ids, r.ID)
		links.WriteString(`<li><a href="content.xhtml#record-` + r.ID + `">` + xmlText(r.Name) + `</a></li>`)
	}
	identity, _ := json.Marshal(struct {
		Snapshot, Title, Language, Exporter string
		IDs                                 []string
	}{c.Snapshot, title, c.Language, "kriemhild-epub-v2", ids})
	hash := sha256.Sum256(identity)
	identifier := "urn:sha256:" + hex.EncodeToString(hash[:])
	language := xmlText(c.Language)
	head := `<?xml version="1.0" encoding="utf-8"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="` + language + `" lang="` + language + `"><head><title>` + xmlText(title) + `</title></head><body>`
	return map[string][]byte{
		"META-INF/container.xml": []byte(`<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`),
		"OEBPS/content.xhtml":    []byte(head + `<h1 dir="auto">` + xmlText(title) + `</h1>` + body + `</body></html>`),
		"OEBPS/nav.xhtml":        []byte(head + `<nav epub:type="toc"><h1>Contents</h1><ol>` + links.String() + `</ol></nav></body></html>`),
		"OEBPS/content.opf":      []byte(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="book">` + identifier + `</dc:identifier><dc:title>` + xmlText(title) + `</dc:title><dc:language>` + language + `</dc:language><meta property="dcterms:modified">` + time.Now().UTC().Format("2006-01-02T15:04:05Z") + `</meta></metadata><manifest><item id="content" href="content.xhtml" media-type="application/xhtml+xml"/><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest><spine><itemref idref="content"/></spine></package>`),
	}
}
