package languages

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const SVGImportBytes = 64 << 10

type SVGImportResult struct {
	Glyph    Glyph    `json:"glyph"`
	Warnings []string `json:"warnings"`
}

var svgNumber = regexp.MustCompile(`^[+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?`)

// The importer never mounts markup, invokes a browser, resolves entities, or
// accesses files/URLs. Only normalized typed geometry leaves this parser.
func ImportGlyphSVG(ctx context.Context, source, id, name string) (SVGImportResult, error) {
	result := SVGImportResult{Glyph: Glyph{ID: id, Name: name, ViewBox: [4]float64{0, 0, 1000, 1000}, Baseline: 800, Advance: 1000, Scale: 1, Paths: []VectorPath{}}, Warnings: []string{"SVG converted to inert absolute path commands; original markup is not retained."}}
	reject := func(message string) (SVGImportResult, error) {
		return SVGImportResult{}, fmt.Errorf("svg_import_rejected: %s", message)
	}
	if len(source) > SVGImportBytes || len(id) == 0 || len(id) > 80 || len(name) > 160 {
		return reject("Limit SVG to 64 KiB and use bounded glyph identity/name.")
	}
	decoder := xml.NewDecoder(strings.NewReader(source))
	decoder.Strict = true
	type paint struct {
		fill, stroke string
		width        float64
	}
	stack := []paint{}
	ids := map[string]bool{}
	elements, commands := 0, 0
	root, ended := false, false
	for {
		if err := ctx.Err(); err != nil {
			return SVGImportResult{}, fmt.Errorf("cancelled: %w", err)
		}
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return reject("Malformed XML: " + err.Error())
		}
		switch token := token.(type) {
		case xml.Directive:
			return reject("DOCTYPE, entity declarations and XML directives are not supported.")
		case xml.ProcInst:
			if token.Target != "xml" || root {
				return reject("Processing instructions are not supported.")
			}
		case xml.CharData:
			if strings.TrimSpace(string(token)) != "" {
				return reject("Only vector geometry is supported; text must be authored as sign readings separately.")
			}
		case xml.StartElement:
			if ended {
				return reject("SVG must have exactly one root.")
			}
			elements++
			if elements > 256 || len(stack) >= 16 {
				return reject("Limit geometry to 256 elements and nesting depth 16.")
			}
			tag := token.Name.Local
			if token.Name.Space != "" && token.Name.Space != "http://www.w3.org/2000/svg" {
				return reject("Foreign namespaces are forbidden.")
			}
			if !root && tag != "svg" {
				return reject("Use an SVG root element.")
			}
			if root && tag == "svg" {
				return reject("Nested SVG viewports are unsupported.")
			}
			allowed := map[string]bool{"id": true, "fill": true, "stroke": true, "stroke-width": true}
			switch tag {
			case "svg":
				for _, a := range []string{"xmlns", "viewBox", "width", "height", "version"} {
					allowed[a] = true
				}
			case "g":
			case "path":
				allowed["d"] = true
			case "rect":
				for _, a := range []string{"x", "y", "width", "height"} {
					allowed[a] = true
				}
			case "circle":
				for _, a := range []string{"cx", "cy", "r"} {
					allowed[a] = true
				}
			case "ellipse":
				for _, a := range []string{"cx", "cy", "rx", "ry"} {
					allowed[a] = true
				}
			case "line":
				for _, a := range []string{"x1", "y1", "x2", "y2"} {
					allowed[a] = true
				}
			case "polygon", "polyline":
				allowed["points"] = true
			default:
				return reject("Unsupported/active element " + tag + ". Use path, rect, circle, ellipse, line, polygon, polyline or passive groups.")
			}
			attrs := map[string]string{}
			seenAttrs := map[string]bool{}
			for _, a := range token.Attr {
				if a.Name.Space != "" || !allowed[a.Name.Local] || seenAttrs[a.Name.Local] {
					return reject("Unsupported, duplicate or active attribute " + a.Name.Local + ". External resources, handlers, style, transforms, filters and URLs are forbidden.")
				}
				attrs[a.Name.Local] = a.Value
				seenAttrs[a.Name.Local] = true
			}
			if value := attrs["id"]; value != "" {
				if ids[value] {
					return reject("Duplicate SVG ID " + value)
				}
				if len(value) > 80 {
					return reject("SVG IDs exceed 80 bytes.")
				}
				ids[value] = true
			}
			style := paint{fill: "#000000", stroke: "none", width: 1}
			if len(stack) > 0 {
				style = stack[len(stack)-1]
			}
			for _, field := range []string{"fill", "stroke"} {
				if value, ok := attrs[field]; ok {
					if !vectorColor.MatchString(value) {
						return reject("Use #RRGGBB or none for paint; URLs/currentColor/styles are not imported.")
					}
					if field == "fill" {
						style.fill = value
					} else {
						style.stroke = value
					}
				}
			}
			scalar := func(key string, defaultValue float64) (float64, error) {
				value, ok := attrs[key]
				if !ok {
					return defaultValue, nil
				}
				n, e := strconv.ParseFloat(value, 64)
				if e != nil || !finiteVector(n) {
					return 0, fmt.Errorf("%s must be a finite unitless number within ±8192", key)
				}
				return n, nil
			}
			if _, ok := attrs["stroke-width"]; ok {
				n, e := scalar("stroke-width", 1)
				if e != nil || n < 0 || n > 256 {
					return reject("Stroke width must be 0–256.")
				}
				style.width = n
			}
			if tag == "svg" {
				root = true
				if ns, ok := attrs["xmlns"]; ok && ns != "http://www.w3.org/2000/svg" {
					return reject("Invalid SVG namespace.")
				}
				if value := attrs["viewBox"]; value != "" {
					numbers, e := svgNumbers(ctx, value, 4)
					if e != nil || len(numbers) != 4 {
						return reject("Use four bounded viewBox numbers.")
					}
					copy(result.Glyph.ViewBox[:], numbers)
				} else {
					width, e := scalar("width", 1000)
					if e != nil {
						return reject(e.Error())
					}
					height, e := scalar("height", 1000)
					if e != nil {
						return reject(e.Error())
					}
					result.Glyph.ViewBox = [4]float64{0, 0, width, height}
					result.Warnings = append(result.Warnings, "No viewBox supplied; unitless width/height define the coordinate viewBox.")
				}
				b := result.Glyph.ViewBox
				if b[2] < 1 || b[2] > 4096 || b[3] < 1 || b[3] > 4096 {
					return reject("ViewBox width/height must be 1–4096.")
				}
				result.Glyph.Baseline = b[1] + b[3]*.8
				result.Glyph.Advance = b[2]
			} else if tag != "g" {
				path := VectorPath{ID: fmt.Sprintf("path-%d", len(result.Glyph.Paths)+1), Fill: style.fill, Stroke: style.stroke, StrokeWidth: style.width, Commands: []PathCommand{}}
				add := func(op string, values ...float64) {
					path.Commands = append(path.Commands, PathCommand{Op: op, Values: values})
				}
				switch tag {
				case "path":
					path.Commands, err = parseSVGPath(ctx, attrs["d"])
					if err != nil {
						return reject(err.Error())
					}
				case "rect":
					x, e1 := scalar("x", 0)
					y, e2 := scalar("y", 0)
					w, e3 := scalar("width", 0)
					h, e4 := scalar("height", 0)
					if e1 != nil || e2 != nil || e3 != nil || e4 != nil || w <= 0 || h <= 0 {
						return reject("Rect requires bounded x/y and positive width/height.")
					}
					add("M", x, y)
					add("L", x+w, y)
					add("L", x+w, y+h)
					add("L", x, y+h)
					add("Z")
				case "line":
					x1, e1 := scalar("x1", 0)
					y1, e2 := scalar("y1", 0)
					x2, e3 := scalar("x2", 0)
					y2, e4 := scalar("y2", 0)
					if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
						return reject("Line coordinates exceed bounds.")
					}
					add("M", x1, y1)
					add("L", x2, y2)
				case "polygon", "polyline":
					p, e := svgNumbers(ctx, attrs["points"], 8192)
					if e != nil || len(p) < 4 || len(p)%2 != 0 {
						return reject("Use bounded x/y point pairs.")
					}
					add("M", p[0], p[1])
					for i := 2; i < len(p); i += 2 {
						add("L", p[i], p[i+1])
					}
					if tag == "polygon" {
						add("Z")
					}
				case "circle", "ellipse":
					cx, e1 := scalar("cx", 0)
					cy, e2 := scalar("cy", 0)
					rx, e3 := scalar("rx", 0)
					ry, e4 := scalar("ry", 0)
					if tag == "circle" {
						rx, e3 = scalar("r", 0)
						ry = rx
					}
					if e1 != nil || e2 != nil || e3 != nil || e4 != nil || rx <= 0 || ry <= 0 {
						return reject("Circle/ellipse requires bounded center and positive radii.")
					}
					k := .5522847498307936
					add("M", cx+rx, cy)
					add("C", cx+rx, cy+k*ry, cx+k*rx, cy+ry, cx, cy+ry)
					add("C", cx-k*rx, cy+ry, cx-rx, cy+k*ry, cx-rx, cy)
					add("C", cx-rx, cy-k*ry, cx-k*rx, cy-ry, cx, cy-ry)
					add("C", cx+k*rx, cy-ry, cx+rx, cy-k*ry, cx+rx, cy)
					add("Z")
				}
				commands += len(path.Commands)
				if commands > VectorMaxCommands || len(result.Glyph.Paths) >= 128 {
					return reject("Geometry exceeds 4096 commands or 128 paths.")
				}
				for _, c := range path.Commands {
					for _, v := range c.Values {
						if !finiteVector(v) {
							return reject("Normalized geometry exceeds ±8192.")
						}
					}
				}
				result.Glyph.Paths = append(result.Glyph.Paths, path)
				if tag != "path" {
					result.Warnings = append(result.Warnings, tag+" converted to path geometry.")
				}
			}
			stack = append(stack, style)
		case xml.EndElement:
			if len(stack) == 0 {
				return reject("Unbalanced XML.")
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				ended = true
			}
		}
	}
	if !root || !ended || len(result.Glyph.Paths) == 0 {
		return reject("SVG has no supported vector geometry.")
	}
	if !finiteVector(result.Glyph.Baseline) {
		return reject("Normalized baseline exceeds ±8192.")
	}
	return result, nil
}

func svgNumbers(ctx context.Context, value string, limit int) ([]float64, error) {
	numbers := []float64{}
	for value != "" {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		value = strings.TrimLeftFunc(value, func(r rune) bool { return unicode.IsSpace(r) || r == ',' })
		if value == "" {
			break
		}
		token := svgNumber.FindString(value)
		if token == "" {
			return nil, fmt.Errorf("invalid numeric geometry")
		}
		n, err := strconv.ParseFloat(token, 64)
		if err != nil || !finiteVector(n) {
			return nil, fmt.Errorf("geometry exceeds numeric bounds")
		}
		numbers = append(numbers, n)
		if len(numbers) > limit {
			return nil, fmt.Errorf("geometry exceeds numeric count")
		}
		value = value[len(token):]
	}
	return numbers, nil
}
func parseSVGPath(ctx context.Context, data string) ([]PathCommand, error) {
	out := []PathCommand{}
	op := byte(0)
	x, y, startX, startY := 0.0, 0.0, 0.0, 0.0
	for strings.TrimSpace(data) != "" {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data = strings.TrimLeftFunc(data, func(r rune) bool { return unicode.IsSpace(r) || r == ',' })
		if data == "" {
			break
		}
		if data[0] >= 'A' && data[0] <= 'Z' || data[0] >= 'a' && data[0] <= 'z' {
			op = data[0]
			data = data[1:]
		}
		upper := byte(strings.ToUpper(string(op))[0])
		relative := op >= 'a' && op <= 'z'
		count, ok := map[byte]int{'M': 2, 'L': 2, 'H': 1, 'V': 1, 'Q': 4, 'C': 6, 'Z': 0}[upper]
		if !ok {
			return nil, fmt.Errorf("unsupported path command; use M/L/H/V/Q/C/Z, with relative forms allowed")
		}
		if len(out) == 0 && upper != 'M' {
			return nil, fmt.Errorf("path must begin with M")
		}
		values := []float64{}
		for i := 0; i < count; i++ {
			data = strings.TrimLeftFunc(data, func(r rune) bool { return unicode.IsSpace(r) || r == ',' })
			token := svgNumber.FindString(data)
			if token == "" {
				return nil, fmt.Errorf("path command has incomplete/malformed coordinates")
			}
			n, err := strconv.ParseFloat(token, 64)
			if err != nil || !finiteVector(n) {
				return nil, fmt.Errorf("path coordinate exceeds bounds")
			}
			values = append(values, n)
			data = data[len(token):]
		}
		switch upper {
		case 'Z':
			x, y = startX, startY
			op = 0
		case 'H':
			if relative {
				values[0] += x
			}
			x = values[0]
			upper = 'L'
			values = []float64{x, y}
		case 'V':
			if relative {
				values[0] += y
			}
			y = values[0]
			upper = 'L'
			values = []float64{x, y}
		default:
			if relative {
				for i := 0; i < len(values); i += 2 {
					values[i] += x
					values[i+1] += y
				}
			}
			x, y = values[len(values)-2], values[len(values)-1]
			if upper == 'M' {
				startX, startY = x, y
				if relative {
					op = 'l'
				} else {
					op = 'L'
				}
			}
		}
		for _, v := range values {
			if !finiteVector(v) || math.IsNaN(v) {
				return nil, fmt.Errorf("normalized path exceeds bounds")
			}
		}
		out = append(out, PathCommand{Op: string(upper), Values: values})
		if len(out) > VectorMaxCommands {
			return nil, fmt.Errorf("path exceeds 4096 commands")
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty path")
	}
	return out, nil
}
