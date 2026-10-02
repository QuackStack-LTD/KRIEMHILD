package project

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"net/http"
	"sort"
	"strings"
)

type ExportRequest struct {
	Language string   `json:"language,omitempty"`
	Snapshot string   `json:"snapshot"`
	IDs      []string `json:"ids"`
	Format   string   `json:"format"`
	Title    string   `json:"title"`
}
type ExportResult struct {
	Data      []byte
	MIME      string
	Extension string
}

func textDocument(raw json.RawMessage) string {
	var n map[string]any
	json.Unmarshal(raw, &n)
	var visit func(map[string]any) string
	visit = func(v map[string]any) string {
		if t, ok := v["text"].(string); ok {
			return t
		}
		var out strings.Builder
		children, _ := v["content"].([]any)
		for _, child := range children {
			if c, ok := child.(map[string]any); ok {
				out.WriteString(visit(c))
				if c["type"] == "paragraph" || c["type"] == "heading" {
					out.WriteString("\n\n")
				}
			}
		}
		return out.String()
	}
	return strings.TrimSpace(visit(n))
}
func htmlDocument(raw json.RawMessage) string {
	var n map[string]any
	json.Unmarshal(raw, &n)
	var visit func(map[string]any) string
	visit = func(v map[string]any) string {
		if t, ok := v["text"].(string); ok {
			out := html.EscapeString(t)
			if marks, ok := v["marks"].([]any); ok {
				for _, raw := range marks {
					m, _ := raw.(map[string]any)
					tag := map[string]string{"bold": "strong", "italic": "em", "strike": "s", "code": "code", "underline": "u"}[fmt.Sprint(m["type"])]
					if tag != "" {
						out = "<" + tag + ">" + out + "</" + tag + ">"
					}
				}
			}
			return out
		}
		tag := map[string]string{"paragraph": "p", "heading": "h3", "bulletList": "ul", "orderedList": "ol", "listItem": "li", "blockquote": "blockquote", "codeBlock": "pre"}[fmt.Sprint(v["type"])]
		attributes := ""
		attrs, _ := v["attrs"].(map[string]any)
		if v["type"] == "heading" {
			level, _ := attrs["level"].(float64)
			tag = fmt.Sprintf("h%d", min(6, max(1, int(level))))
		}
		if v["type"] == "paragraph" || v["type"] == "heading" || v["type"] == "blockquote" {
			attributes = ` dir="auto"`
		}
		if v["type"] == "orderedList" {
			if start, ok := attrs["start"].(float64); ok && start >= 1 && start <= 1000000 {
				attributes = fmt.Sprintf(` start="%d"`, int(start))
			}
		}
		if v["type"] == "hardBreak" {
			return "<br/>"
		}
		if v["type"] == "horizontalRule" {
			return "<hr/>"
		}
		var out strings.Builder
		children, _ := v["content"].([]any)
		for _, child := range children {
			if c, ok := child.(map[string]any); ok {
				out.WriteString(visit(c))
			}
		}
		if tag != "" {
			return "<" + tag + attributes + ">" + out.String() + "</" + tag + ">"
		}
		return out.String()
	}
	return visit(n)
}
func xmlText(s string) string { var b bytes.Buffer; xml.EscapeText(&b, []byte(s)); return b.String() }
func zipFiles(files map[string][]byte, epub bool) ([]byte, error) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	if epub {
		w, e := z.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
		if e != nil {
			return nil, e
		}
		w.Write([]byte("application/epub+zip"))
	}
	names := []string{}
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		w, e := z.Create(name)
		if e != nil {
			return nil, e
		}
		if _, e = w.Write(files[name]); e != nil {
			return nil, e
		}
	}
	if e := z.Close(); e != nil {
		return nil, e
	}
	return b.Bytes(), nil
}

// Human-readable exports include only explicitly selected records. They never
// traverse links to private dossiers, old revisions, calendars or source Ages.
func (s *Store) Export(c ExportRequest) (ExportResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.Language == "" {
		c.Language = "und"
	}
	if len(c.Language) > 64 || !editionLanguage.MatchString(c.Language) {
		return ExportResult{}, fmt.Errorf("choose an edition language tag such as en, uk, ar, or und when unspecified")
	}
	records, e := s.records(c.Snapshot)
	if e != nil {
		return ExportResult{}, e
	}
	if len(c.IDs) == 0 {
		return ExportResult{}, fmt.Errorf("select at least one record")
	}
	selected := []Record{}
	seen := map[string]bool{}
	for _, id := range c.IDs {
		r, ok := records[id]
		if !ok {
			return ExportResult{}, fmt.Errorf("selected record not found")
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		selected = append(selected, r)
	}
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].Kind != selected[j].Kind {
			return selected[i].Kind < selected[j].Kind
		}
		if selected[i].Kind == "scene" && selected[i].Order != selected[j].Order {
			return selected[i].Order < selected[j].Order
		}
		if selected[i].Name != selected[j].Name {
			return selected[i].Name < selected[j].Name
		}
		return selected[i].ID < selected[j].ID
	})
	title := c.Title
	if title == "" {
		title = "KRIEMHILD selection"
	}
	var md, body, plain strings.Builder
	var doc wordWriter
	doc.language = c.Language
	if c.Format == "docx" {
		doc.text(title, "Title")
	}
	md.WriteString("# " + title + "\n\n")
	plain.WriteString(title + "\n\n")
	for _, r := range selected {
		text := r.Notes
		if r.Kind == "scene" {
			text = textDocument(r.Document)
		}
		md.WriteString("## " + r.Name + "\n\n" + text + "\n\n")
		plain.WriteString(strings.ToUpper(r.Name) + "\n\n" + text + "\n\n")
		body.WriteString("<article id=\"record-" + r.ID + "\"><h2>" + html.EscapeString(r.Name) + "</h2>")
		if r.Kind == "scene" {
			body.WriteString(htmlDocument(r.Document))
		} else {
			body.WriteString("<p>" + strings.ReplaceAll(html.EscapeString(text), "\n", "<br/>") + "</p>")
		}
		if r.Kind == "map" && (c.Format == "publication" || c.Format == "html") {
			graphic, err := s.publicMap(r, records, seen)
			if err != nil {
				return ExportResult{}, err
			}
			body.WriteString(graphic)
		}
		if r.Properties["_domain"] == "storyboard" && r.Asset != "" && (c.Format == "publication" || c.Format == "html") {
			data, e := s.asset(r.Asset)
			if e != nil {
				return ExportResult{}, e
			}
			body.WriteString(`<img style="max-width:100%" alt="` + html.EscapeString(r.Name) + `" src="data:` + http.DetectContentType(data) + `;base64,` + base64.StdEncoding.EncodeToString(data) + `"/>`)
		}
		body.WriteString("</article>")
		if c.Format == "docx" {
			doc.text(r.Name, "Heading1")
			if r.Kind == "scene" {
				if e := doc.manuscript(r.Document); e != nil {
					return ExportResult{}, e
				}
			} else {
				for _, line := range strings.Split(text, "\n") {
					doc.text(line, "Normal")
				}
			}
		}
	}
	page := "<!doctype html><html lang=\"" + c.Language + "\"><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><meta http-equiv=\"Content-Security-Policy\" content=\"default-src 'none'; style-src 'unsafe-inline'\"><title>" + html.EscapeString(title) + "</title><style>body{max-width:70ch;margin:3rem auto;padding:1rem;font:18px/1.65 Georgia}article{margin:3rem 0}nav a{display:block}pre{white-space:pre-wrap}</style><h1>" + html.EscapeString(title) + "</h1>"
	switch c.Format {
	case "markdown":
		return ExportResult{[]byte(md.String()), "text/markdown; charset=utf-8", "md"}, nil
	case "fountain":
		return ExportResult{[]byte(plain.String()), "text/plain; charset=utf-8", "fountain"}, nil
	case "html", "publication":
		page = strings.Replace(page, "pre{", "[hidden]{display:none!important} @media print{nav,input,button,select{display:none}} pre{", 1)
		var nav strings.Builder
		if c.Format == "publication" {
			nav.WriteString("<nav aria-label=\"Contents\">")
			for _, r := range selected {
				nav.WriteString("<a href=\"#record-" + r.ID + "\">" + html.EscapeString(r.Name) + "</a>")
			}
			nav.WriteString("</nav>")
			digest := sha256.Sum256([]byte(publicationScript))
			page = strings.Replace(page, "style-src 'unsafe-inline'", "style-src 'unsafe-inline'; img-src data:; script-src 'sha256-"+base64.StdEncoding.EncodeToString(digest[:])+"'", 1)
			nav.WriteString(`<label>Search this edition <input id="edition-search" type="search"/></label><p id="edition-status" role="status"></p><button id="edition-print" type="button">Print this edition</button>`)
			body.WriteString("<script>" + publicationScript + "</script>")
		} else {
			page = strings.Replace(page, "style-src 'unsafe-inline'", "style-src 'unsafe-inline'; img-src data:", 1)
		}
		return ExportResult{[]byte(page + nav.String() + body.String() + "</html>"), "text/html; charset=utf-8", "html"}, nil
	case "epub":
		data, e := zipFiles(epubFiles(c, title, body.String(), selected), true)
		return ExportResult{data, "application/epub+zip", "epub"}, e
	case "docx":
		data, e := zipFiles(doc.files(), false)
		return ExportResult{data, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "docx"}, e
	case "maps":
		safe := []Record{}
		for _, r := range selected {
			if r.Kind == "map" {
				r.Pins = append([]Pin{}, r.Pins...)
				pins := r.Pins[:0]
				for _, p := range r.Pins {
					if seen[p.Entity] {
						pins = append(pins, p)
					}
				}
				r.Pins = pins
				features := []Feature{}
				for _, f := range r.Features {
					if f.Entity == "" || seen[f.Entity] {
						f.Notes = ""
						features = append(features, f)
					}
				}
				r.Features = features
				r.Properties = nil
				r.Notes = ""
				safe = append(safe, r)
			}
		}
		b, e := json.MarshalIndent(map[string]any{"format": "kriemhild-planar-maps-v1", "maps": safe}, "", "  ")
		return ExportResult{b, "application/json", "json"}, e
	case "svg":
		files := map[string][]byte{}
		for _, r := range selected {
			if r.Kind == "map" {
				graphic, e := s.publicMap(r, records, seen)
				if e != nil {
					return ExportResult{}, e
				}
				files[r.ID+".svg"] = []byte(graphic)
			}
		}
		if len(files) == 0 {
			return ExportResult{}, fmt.Errorf("select at least one map")
		}
		b, e := zipFiles(files, false)
		return ExportResult{b, "application/zip", "zip"}, e
	default:
		return ExportResult{}, fmt.Errorf("unsupported export format")
	}
}

// The script is fixed application code, authorized by its CSP hash. Authored
// text is escaped HTML, never interpolated into executable code or an index.
const publicationScript = `(()=>{const input=document.getElementById('edition-search');const articles=[...document.querySelectorAll('article')];const status=document.getElementById('edition-status');const filter=()=>{const query=input.value.trim().toLocaleLowerCase();let found=0;for(const article of articles){article.hidden=!article.textContent.toLocaleLowerCase().includes(query);if(!article.hidden)found++;const link=document.querySelector('nav a[href="#'+article.id+'"]');if(link)link.hidden=article.hidden;}status.textContent=found+' of '+articles.length+' entries';};input.addEventListener('input',filter);document.getElementById('edition-print').addEventListener('click',()=>print());for(const svg of document.querySelectorAll('svg')){const groups=[...svg.querySelectorAll('[data-floor]')];if(!groups.length)continue;const select=document.createElement('select');const label=document.createElement('label');label.textContent='Map floor ';label.append(select);for(const value of ['all',...new Set(groups.map(g=>g.dataset.floor))]){const option=document.createElement('option');option.value=value;option.textContent=value==='all'?'All floors':value;select.append(option);}select.addEventListener('change',()=>{for(const group of groups)group.style.display=select.value==='all'||group.dataset.floor===select.value?'':'none';});svg.before(label);}filter();})();`

// Public map labels and pins resolve only through the explicit selection.
// Raster backgrounds are included as an indivisible, author-selected asset.
func (s *Store) publicMap(r Record, records map[string]Record, selected map[string]bool) (string, error) {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1000 700" role="img" aria-labelledby="title-` + r.ID + `" style="width:100%;height:auto"><title id="title-` + r.ID + `">` + html.EscapeString(r.Name) + `</title><rect width="1000" height="700" fill="#e7ddc7"/>`)
	if r.Asset != "" {
		data, e := s.asset(r.Asset)
		if e != nil {
			return "", e
		}
		b.WriteString(`<image width="1000" height="700" preserveAspectRatio="none" href="data:` + http.DetectContentType(data) + `;base64,` + base64.StdEncoding.EncodeToString(data) + `"/>`)
	}
	if t := r.Terrain; t != nil {
		for i, h := range t.Heights {
			fill := "#a2b879"
			if t.Water[i] != 0 {
				fill = "#8eb8c5"
			} else if h > 7000 {
				fill = "#a29685"
			}
			fmt.Fprintf(&b, `<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" fill="%s"/>`, float64(i%t.Columns)*1000/float64(t.Columns), float64(i/t.Columns)*700/float64(t.Rows), 1000/float64(t.Columns)+0.1, 700/float64(t.Rows)+0.1, fill)
		}
	}
	for _, f := range r.Features {
		if f.Entity != "" && !selected[f.Entity] {
			continue
		}
		points := []string{}
		for _, p := range f.Points {
			points = append(points, fmt.Sprintf("%.3f,%.3f", p.X*1000, p.Y*700))
		}
		fill, stroke := "none", "#82594b"
		if f.Kind == "river" || f.Kind == "lake" {
			stroke = "#367c9b"
		}
		tag := "polyline"
		if f.Kind == "border" || f.Kind == "lake" || f.Kind == "climate" || f.Kind == "biome" {
			tag = "polygon"
		}
		if f.Kind == "lake" {
			fill = "#8eb8c5"
		}
		if f.Kind == "climate" {
			fill = "#ecd0b6"
			stroke = "#bb743d"
		}
		if f.Kind == "biome" {
			fill = "#c7d9c7"
			stroke = "#52815c"
		}
		fmt.Fprintf(&b, `<%s points="%s" fill="%s" stroke="%s" stroke-width="2"><title>%s</title></%s>`, tag, strings.Join(points, " "), fill, stroke, html.EscapeString(f.Name), tag)
	}
	for _, p := range r.Pins {
		if !selected[p.Entity] {
			continue
		}
		label := html.EscapeString(records[p.Entity].Name)
		fmt.Fprintf(&b, `<a href="#record-%s"><circle cx="%.3f" cy="%.3f" r="5" fill="#6a3025"/><text x="%.3f" y="%.3f" font-size="14" fill="#231f1b">%s</text></a>`, p.Entity, p.X*1000, p.Y*700, p.X*1000+8, p.Y*700-8, label)
	}
	if drawing, e := localDrawing(r); e != nil {
		return "", e
	} else if drawing != nil {
		for _, shape := range drawing.Shapes {
			points := []string{}
			for _, p := range shape.Points {
				points = append(points, fmt.Sprintf("%.3f,%.3f", p.X*1000, p.Y*700))
			}
			tag, fill := "polyline", "none"
			if shape.Kind == "room" {
				tag = "polygon"
				fill = "#bfb190"
			}
			fmt.Fprintf(&b, `<g data-floor="%d"><%s points="%s" fill="%s" stroke="#51463b" stroke-width="3"><title>%s (floor %d)</title></%s></g>`, shape.Level, tag, strings.Join(points, " "), fill, html.EscapeString(shape.Name), shape.Level, tag)
		}
		for _, t := range drawing.Tokens {
			if !selected[t.Entity] {
				continue
			}
			fmt.Fprintf(&b, `<g data-floor="%d"><circle cx="%.3f" cy="%.3f" r="8" fill="#526a7e"/><text x="%.3f" y="%.3f" font-size="14">%s</text></g>`, t.Level, t.X*1000, t.Y*700, t.X*1000+12, t.Y*700, html.EscapeString(records[t.Entity].Name))
		}
	}
	b.WriteString(`</svg>`)
	return b.String(), nil
}
