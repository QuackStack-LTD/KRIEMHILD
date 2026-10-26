package languages

import (
	"fmt"
	"math"
	"regexp"
)

const VectorEngine = "vector-script/1"
const VectorAdapter = "sign-layout/1"
const VectorCompiler = "vector-script-compiler/1"
const VectorMaxCommands = 4096
const VectorMaxSigns = 512

type PathCommand struct {
	Op     string    `json:"op"`
	Values []float64 `json:"values"`
}
type VectorPath struct {
	ID          string        `json:"id"`
	Commands    []PathCommand `json:"commands"`
	Fill        string        `json:"fill"`
	Stroke      string        `json:"stroke"`
	StrokeWidth float64       `json:"strokeWidth"`
}
type Glyph struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	ViewBox  [4]float64   `json:"viewBox"`
	Baseline float64      `json:"baseline"`
	Advance  float64      `json:"advance"`
	Bearing  float64      `json:"bearing"`
	Scale    float64      `json:"scale"`
	Paths    []VectorPath `json:"paths"`
}
type SignAssociation struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}
type SignReading struct {
	ID   string   `json:"id"`
	Text string   `json:"text"`
	When Features `json:"when"`
}
type ScriptSign struct {
	ID           string            `json:"id"`
	Label        string            `json:"label"`
	Glyph        string            `json:"glyph"`
	Grapheme     string            `json:"grapheme"`
	Roles        []string          `json:"roles"`
	Readings     []SignReading     `json:"readings"`
	Associations []SignAssociation `json:"associations"`
}
type SignUse struct {
	Kind     string    `json:"kind"` // sign, group (ordered horizontal compound), break
	Sign     string    `json:"sign,omitempty"`
	Reading  string    `json:"reading,omitempty"`
	Roles    []string  `json:"roles,omitempty"`
	Context  Features  `json:"context,omitempty"`
	Children []SignUse `json:"children,omitempty"`
}
type SignSequence struct {
	ID    string    `json:"id"`
	Name  string    `json:"name"`
	Items []SignUse `json:"items"`
}
type WritingSystem struct {
	Version    string         `json:"version"`
	Profile    string         `json:"profile"`
	UnitsPerEm float64        `json:"unitsPerEm"`
	LineHeight float64        `json:"lineHeight"`
	Spacing    float64        `json:"spacing"`
	Glyphs     []Glyph        `json:"glyphs"`
	Signs      []ScriptSign   `json:"signs"`
	Sequences  []SignSequence `json:"sequences"`
}

func CompatibleScriptVersions(schema int, engine, adapter, compiler string) bool {
	return schema == 2 && engine == VectorEngine && adapter == VectorAdapter && compiler == VectorCompiler
}

var vectorColor = regexp.MustCompile(`^(none|#[0-9a-fA-F]{6})$`)

func finiteVector(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && math.Abs(v) <= 8192 }
func vectorShape(w *WritingSystem) error {
	if w == nil {
		return nil
	}
	if len(w.Glyphs) > 256 || len(w.Signs) > 512 || len(w.Sequences) > 128 {
		return fmt.Errorf("writing: exceeds 256 glyphs, 512 signs or 128 sequences")
	}
	commands := 0
	for _, g := range w.Glyphs {
		if len(g.Paths) > 128 {
			return fmt.Errorf("writing.glyphs: exceeds 128 paths per glyph")
		}
		for _, p := range g.Paths {
			commands += len(p.Commands)
			for _, c := range p.Commands {
				if len(c.Values) > 6 {
					return fmt.Errorf("writing.glyphs.paths: excessive command coordinates")
				}
			}
		}
	}
	if commands > VectorMaxCommands {
		return fmt.Errorf("writing.glyphs: exceeds 4096 total path commands")
	}
	nodes := 0
	var visit func([]SignUse, int) error
	visit = func(items []SignUse, depth int) error {
		if len(items) == 0 {
			return nil
		}
		if depth > 4 {
			return fmt.Errorf("writing.sequences: exceeds group depth 4")
		}
		for _, item := range items {
			nodes++
			if nodes > 4096 || len(item.Context) > 32 || len(item.Roles) > 6 {
				return fmt.Errorf("writing.sequences: exceeds node/context/role limits")
			}
			if err := visit(item.Children, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	for _, s := range w.Signs {
		if len(s.Roles) > 6 || len(s.Readings) > 32 || len(s.Associations) > 32 {
			return fmt.Errorf("writing.signs: exceeds role/reading/association limits")
		}
		for _, r := range s.Readings {
			if len(r.When) > 32 || len(r.Text) > 512 {
				return fmt.Errorf("writing.signs.readings: exceeds context/text limits")
			}
		}
	}
	for _, s := range w.Sequences {
		if err := visit(s.Items, 0); err != nil {
			return err
		}
	}
	return nil
}
