package terrain

import (
	"math"
	"sort"
)

type NaturalHit struct {
	ID          string             `json:"id"`
	Type        string             `json:"type"`
	Name        string             `json:"name"`
	Variant     string             `json:"variant"`
	Location    string             `json:"location"`
	Distance    float64            `json:"distance"`
	Abundance   float64            `json:"abundance"`
	Rating      string             `json:"rating"`
	Description string             `json:"description"`
	Depth       float64            `json:"depthMetres"`
	References  []NaturalReference `json:"references"`
}
type GeologicalContext struct {
	ID      string        `json:"id"`
	Kind    string        `json:"kind"`
	Rock    string        `json:"rock"`
	AgeMa   float64       `json:"ageMa"`
	Process string        `json:"process"`
	Strata  []RockStratum `json:"strata,omitempty"`
}
type NaturalQuery struct {
	Climate       ClimateDescription  `json:"climate"`
	Available     bool                `json:"available"`
	Terrain       string              `json:"terrain"`
	X             float64             `json:"x"`
	Y             float64             `json:"y"`
	Radius        float64             `json:"radius"`
	DistanceUnits string              `json:"distanceUnits"`
	Geology       []GeologicalContext `json:"geology"`
	History       *GeologicalHistory  `json:"history,omitempty"`
	Goods         []NaturalHit        `json:"goods"`
	Total         int                 `json:"total"`
	Truncated     bool                `json:"truncated"`
}

// Disposable per-session cell index. It contains only occurrence references,
// so changing camera scale neither regenerates data nor changes membership.
type NaturalIndex struct {
	e     *Environment
	cells [][]int
}

func NewNaturalIndex(e *Environment) *NaturalIndex {
	idx := &NaturalIndex{e: e}
	if e == nil || e.Resources == nil || e.Geology == nil {
		return idx
	}
	idx.cells = make([][]int, len(e.Mask))
	w := e.Options.Columns
	for k, o := range e.Resources.Occurrences {
		for _, cell := range o.Geometry.Cells {
			idx.cells[cell] = append(idx.cells[cell], k)
		}
		if p := o.Geometry.Point; p != nil {
			cell := int(math.Round(p[1]))*w + int(math.Round(p[0]))
			idx.cells[cell] = append(idx.cells[cell], k)
		}
	}
	return idx
}
func (idx *NaturalIndex) Query(x, y, radius float64, wrap bool) NaturalQuery {
	out := NaturalQuery{X: x, Y: y, Radius: radius, DistanceUnits: "parent-grid cells", Goods: []NaturalHit{}, Geology: []GeologicalContext{}}
	e := idx.e
	if e == nil || e.Geology == nil || e.Resources == nil {
		return out
	}
	out.Available = true
	w, h := e.Options.Columns, e.Options.Rows
	cell := int(clamp(math.Round(y), 0, float64(h-1)))*w + int(clamp(math.Round(x), 0, float64(w-1)))
	c := e.Geology.Cells[cell]
	p := e.Geology.Provinces[c.Province]
	out.History = &p.History
	out.Geology = append(out.Geology, GeologicalContext{ID: p.ID, Kind: p.Setting, Rock: p.Rock, AgeMa: p.AgeMa, Process: p.Process})
	for _, fi := range []int{c.Cover, c.Basement, c.Intrusion} {
		if fi >= 0 {
			fm := e.Geology.Formations[fi]
			out.Geology = append(out.Geology, GeologicalContext{fm.ID, fm.Kind, fm.Rock, fm.AgeMa, fm.Process, fm.Strata})
		}
	}
	if c.Fault >= 0 {
		fault := e.Geology.Structures[c.Fault]
		out.Geology = append(out.Geology, GeologicalContext{ID: fault.ID, Kind: fault.Kind, AgeMa: fault.AgeMa, Process: "fracture-controlled fluid pathway"})
	}
	distances := map[int]float64{}
	centers := []float64{x}
	if wrap {
		centers = append(centers, x-float64(w-1), x+float64(w-1))
	}
	for _, cx := range centers {
		for yy := max(0, int(math.Floor(y-radius-.5))); yy <= min(h-1, int(math.Ceil(y+radius+.5))); yy++ {
			for xx := max(0, int(math.Floor(cx-radius-.5))); xx <= min(w-1, int(math.Ceil(cx+radius+.5))); xx++ {
				d := math.Hypot(math.Max(0, math.Abs(cx-float64(xx))-.5), math.Max(0, math.Abs(y-float64(yy))-.5))
				if d > radius {
					continue
				}
				for _, k := range idx.cells[yy*w+xx] {
					distance := d
					if point := e.Resources.Occurrences[k].Geometry.Point; point != nil {
						distance = math.Hypot(cx-point[0], y-point[1])
					}
					if distance > radius {
						continue
					}
					if previous, ok := distances[k]; !ok || distance < previous {
						distances[k] = distance
					}
				}
			}
		}
	}
	names := map[string]string{}
	for _, d := range ResourceCatalog {
		names[d.ID] = d.Name
	}
	for k, d := range distances {
		o := e.Resources.Occurrences[k]
		location := "here"
		if d > 1e-6 {
			location = "nearby"
		}
		rating := "Sparse"
		if o.Abundance >= .3 {
			rating = "Moderate"
		}
		if o.Abundance >= .6 {
			rating = "Abundant"
		}
		out.Goods = append(out.Goods, NaturalHit{o.ID, o.Type, names[o.Type], o.Variant, location, rounded(d), o.Abundance, rating, o.Cause, o.Depth, o.References})
	}
	sort.Slice(out.Goods, func(a, b int) bool {
		aHit, bHit := out.Goods[a], out.Goods[b]
		if aHit.Location != bHit.Location {
			return aHit.Location == "here"
		}
		if aHit.Distance != bHit.Distance {
			return aHit.Distance < bHit.Distance
		}
		if aHit.Abundance != bHit.Abundance {
			return aHit.Abundance > bHit.Abundance
		}
		return aHit.ID < bHit.ID
	})
	out.Total = len(out.Goods)
	if len(out.Goods) > 64 {
		out.Truncated = true
		out.Goods = out.Goods[:64]
	}
	return out
}
