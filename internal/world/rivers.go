package world

import (
	"fmt"
	"kriemhild/internal/terrain"
	"math"
	"sort"
)

func (o Operation) influenceRadius() float64 {
	r := o.Radius
	if o.River != nil {
		for _, w := range o.River.Widths {
			r = math.Max(r, (w*5.5+.04)*3)
		}
	}
	return r
}

func (s *State) validateRiver(o Operation) error {
	f := o.River
	if f == nil {
		return nil
	} // Older projects are upgraded once when opened.
	if o.Kind != "river" || f.Kind != "river" || f.ID != "authored/"+o.ID || len(f.Path) < 2 || len(f.Path) > 32768 || len(f.Widths) != len(f.Path) || !finite(f.Width) || f.Width <= 0 || f.Width > 1 || !finite(f.Level) || f.Level < 0 || f.Level > 8 || !finite(f.Discharge) || f.Discharge < 0 || f.Discharge > 1e6 || !finite(f.Grade) || f.Grade < 0 || f.Grade > 40000 || !finite(f.Depth) || f.Depth < 0 || f.Depth > 12000 {
		return fmt.Errorf("invalid saved river channel")
	}
	box := Box(o.Path)
	for i, p := range f.Path {
		if !s.validPoint([2]float64{p[0], p[1]}) || !box.Expand(.000001).contains(Bounds{p[0], p[1], 0, 0}) || !finite(p[2]) || math.Abs(p[2]) > 1e7 || !finite(f.Widths[i]) || f.Widths[i] <= 0 || f.Widths[i] > 1 || i > 0 && p[2] > f.Path[i-1][2]+1e-7 {
			return fmt.Errorf("invalid saved river profile")
		}
	}
	return nil
}

// Capture channel geometry and its downhill water profile once. Reloads use
// this saved data, never the current terrain or camera to recreate a river.
func (s *State) prepareRiver(o *Operation, e *terrain.Environment) {
	if o.River != nil {
		return
	}
	sample := func(x, y float64) float64 {
		p := terrain.DetailPoint{Elevation: sampleGrid(e.Fields["elevation"], e.Options.Columns, e.Options.Rows, x, y)}
		ops := s.Ops(Bounds{x, y, 0, 0})
		prior := ops[:0]
		for _, op := range ops {
			if op.ID != o.ID && (o.Sequence == 0 || op.Sequence < o.Sequence) {
				prior = append(prior, op)
			}
		}
		return Apply(p, x, y, prior).Elevation
	}
	points := append([][2]float64(nil), o.Path...)
	if sample(points[0][0], points[0][1]) < sample(points[len(points)-1][0], points[len(points)-1][1]) {
		for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
			points[i], points[j] = points[j], points[i]
		}
	}
	width := clamp(o.Radius*.08, .02, .34)
	q := math.Max(2, math.Pow(math.Max(.012, width-.018)/.012, 2))
	f := &terrain.DetailFeature{ID: "authored/" + o.ID, Kind: "river", Width: width, Discharge: q, Level: clamp(6-math.Log2(1+q)*1.1, 0, 6), Depth: math.Abs(o.Amount)}
	length := 0.
	for i := 1; i < len(points); i++ {
		a, b := points[i-1], points[i]
		length += math.Hypot(b[0]-a[0], b[1]-a[1])
		prev, next := points[max(0, i-2)], points[min(len(points)-1, i+1)]
		for k := 0; k < 16; k++ {
			t := float64(k) / 16
			p := [3]float64{}
			for axis := 0; axis < 2; axis++ {
				// Interpolating cubic, bounded by the authored segment's extent.
				p[axis] = clamp(.5*((2*a[axis])+(-prev[axis]+b[axis])*t+(2*prev[axis]-5*a[axis]+4*b[axis]-next[axis])*t*t+(-prev[axis]+3*a[axis]-3*b[axis]+next[axis])*t*t*t), math.Min(a[axis], b[axis]), math.Max(a[axis], b[axis]))
			}
			p[2] = sample(p[0], p[1])
			if len(f.Path) > 0 {
				p[2] = math.Min(p[2], f.Path[len(f.Path)-1][2])
			}
			f.Path = append(f.Path, p)
		}
	}
	end := points[len(points)-1]
	f.Path = append(f.Path, [3]float64{end[0], end[1], math.Min(sample(end[0], end[1]), f.Path[len(f.Path)-1][2])})
	f.Grade = (f.Path[0][2] - f.Path[len(f.Path)-1][2]) / math.Max(.001, length)
	for i := range f.Path {
		t := float64(i) / float64(len(f.Path)-1)
		f.Widths = append(f.Widths, width*(.7+.3*t)*(1+.07*math.Sin(6*math.Pi*t)*math.Sin(math.Pi*t)))
	}
	f.Width = f.Widths[0]
	o.River = f
}

func applyRiver(p terrain.DetailPoint, x, y float64, f terrain.DetailFeature) terrain.DetailPoint {
	r := terrain.SampleChannel(f, x, y)
	if !r.Valid || r.Distance > r.Valley*3 || p.WaterBody > 0 {
		return p
	}
	target, influence := terrain.ChannelBed(r)
	// Fade the finite edit boundary without a visible step in the surface.
	edge := clamp(3-r.Distance/r.Valley, 0, 1)
	influence *= edge * edge * (3 - 2*edge)
	p.Elevation += (target - p.Elevation) * influence
	p.Floodplain = math.Max(p.Floodplain, math.Exp(-math.Pow(r.Distance/r.Valley, 2)))
	p.Vegetation = clamp(p.Vegetation+p.Floodplain*.18, 0, 1)
	if r.Distance < r.Width*.5 {
		p.RiverLevel = r.Level
		p.RiverDepth = math.Max(0, r.Level-p.Elevation)
		p.Vegetation = 0
	}
	p.Authored = true
	return p
}

func (s *State) PrepareLegacyRivers(e *terrain.Environment) {
	if e == nil {
		return
	}
	ids := []string{}
	for id, o := range s.Operations {
		if o.Kind == "river" && o.River == nil && !o.Deleted {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return s.Operations[ids[i]].Sequence < s.Operations[ids[j]].Sequence })
	for _, id := range ids {
		o := s.Operations[id]
		s.prepareRiver(&o, e)
		s.apply("operation/"+id, raw(o))
	}
}

func (s *State) riverFeatures(tile *terrain.DetailTile, bounds Bounds, ops []Operation) {
	for _, o := range ops {
		if o.River != nil && o.River.Level <= float64(tile.Level)+1 && o.Bounds().Intersects(bounds) {
			tile.Features = append(tile.Features, *o.River)
		}
	}
}
