package world

import (
	"kriemhild/internal/terrain"
	"math"
)

func clamp(v, a, b float64) float64 { return math.Max(a, math.Min(b, v)) }
func distance(o Operation, x, y float64) float64 {
	d := math.Hypot(x-o.Center[0], y-o.Center[1])
	if len(o.Path) > 1 {
		d = math.Inf(1)
		for i := 1; i < len(o.Path); i++ {
			a, b := o.Path[i-1], o.Path[i]
			dx, dy := b[0]-a[0], b[1]-a[1]
			t := 0.
			if dx*dx+dy*dy > 0 {
				t = clamp(((x-a[0])*dx+(y-a[1])*dy)/(dx*dx+dy*dy), 0, 1)
			}
			d = math.Min(d, math.Hypot(x-a[0]-t*dx, y-a[1]-t*dy))
		}
	}
	return d
}
func sampleGrid(values []float64, w, h int, x, y float64) float64 {
	x = clamp(x, 0, float64(w-1))
	y = clamp(y, 0, float64(h-1))
	ix, iy := int(x), int(y)
	a, b := x-float64(ix), y-float64(iy)
	return values[iy*w+ix]*(1-a)*(1-b) + values[iy*w+min(w-1, ix+1)]*a*(1-b) + values[min(h-1, iy+1)*w+ix]*(1-a)*b + values[min(h-1, iy+1)*w+min(w-1, ix+1)]*a*b
}
func Apply(p terrain.DetailPoint, x, y float64, ops []Operation) terrain.DetailPoint {
	for _, o := range ops {
		if o.Kind == "river" && o.River != nil {
			p = applyRiver(p, x, y, *o.River)
			continue
		}
		r := distance(o, x, y) / o.Radius
		if r >= 1 {
			continue
		}
		f := 1 - r*r*(3-2*r)
		p.Authored = true
		switch o.Kind {
		case "raise", "island":
			p.Elevation += math.Abs(o.Amount) * f
		case "lower", "valley":
			p.Elevation -= math.Abs(o.Amount) * f
		case "flatten":
			p.Elevation += (o.Target - p.Elevation) * f
		case "smooth":
			target := sampleGrid(o.Samples, 9, 9, (x-o.Center[0]+o.Radius)*4/o.Radius, (y-o.Center[1]+o.Radius)*4/o.Radius)
			p.Elevation += (target - p.Elevation) * f
		case "crater":
			p.Elevation += math.Abs(o.Amount) * (-math.Pow(math.Max(0, 1-r*r/.64), 2) + .22*math.Exp(-math.Pow((r-.8)/.1, 2))) * f
			p.Vegetation *= 1 - f
			p.Rock = math.Max(p.Rock, f)
		case "water":
			if p.Elevation < o.Target {
				p.WaterBody = 1000000 + o.Sequence
				p.WaterLevel = o.Target
				p.WaterDepth = o.Target - p.Elevation
				p.Vegetation = 0
			}
		case "drain":
			p.WaterBody = 0
			p.WaterDepth = 0
			p.RiverDepth = 0
		case "plant":
			if p.WaterBody == 0 {
				p.Vegetation += (1 - p.Vegetation) * f
			}
		case "clear":
			p.Vegetation *= 1 - f
			p.Rock = math.Max(p.Rock, f*.3)
		case "river":
			p.Elevation -= math.Abs(o.Amount) * f
			p.RiverLevel = p.Elevation + math.Abs(o.Amount)*f*.6
			p.RiverDepth = math.Abs(o.Amount) * f * .6
			p.Vegetation *= 1 - f
		}
		if p.WaterBody > 0 {
			p.WaterDepth = math.Max(0, p.WaterLevel-p.Elevation)
			if p.WaterDepth == 0 {
				p.WaterBody = 0
			}
		}
	}
	return p
}
func (s *State) Elevation(e *terrain.Environment, x, y float64) float64 {
	base := sampleGrid(e.Fields["elevation"], e.Options.Columns, e.Options.Rows, x, y)
	return Apply(terrain.DetailPoint{Elevation: base}, x, y, s.Ops(Bounds{x, y, 0, 0})).Elevation
}

// Smooth captures a local target field once; replay never samples a later world.
func (s *State) Prepare(o *Operation, e *terrain.Environment) {
	if o.Kind == "river" {
		s.prepareRiver(o, e)
	}
	if o.Kind == "smooth" {
		o.Samples = make([]float64, 81)
		for j := 0; j < 9; j++ {
			for i := 0; i < 9; i++ {
				x := o.Center[0] - o.Radius + float64(i)*o.Radius/4
				y := o.Center[1] - o.Radius + float64(j)*o.Radius/4
				v := 0.
				for _, d := range [][2]float64{{0, 0}, {-.25, 0}, {.25, 0}, {0, -.25}, {0, .25}} {
					v += s.Elevation(e, x+d[0]*o.Radius, y+d[1]*o.Radius)
				}
				o.Samples[j*9+i] = v / 5
			}
		}
	}
}
func (s *State) ApplyTile(tile *terrain.DetailTile, parent *terrain.DetailTile) {
	b := Bounds{float64(tile.X) * 32 * tile.Step, float64(tile.Y) * 32 * tile.Step, 32 * tile.Step, 32 * tile.Step}
	ops := s.Ops(b.Expand(2))
	if len(ops) == 0 {
		return
	}
	s.riverFeatures(tile, b, ops)
	original := append([]terrain.DetailPoint(nil), tile.Points...)
	for j := 0; j < 33; j++ {
		for i := 0; i < 33; i++ {
			at := j*33 + i
			x, y := math.Min(s.W, b.X+float64(i)*tile.Step), math.Min(s.H, b.Y+float64(j)*tile.Step)
			tile.Points[at] = Apply(original[at], x, y, ops)
			if parent != nil {
				u, v := float64(tile.X%2*16)+float64(i)/2, float64(tile.Y%2*16)+float64(j)/2
				ix, iy := int(u), int(v)
				a, c := u-float64(ix), v-float64(iy)
				z := func(dx, dy int) float64 { return parent.Points[min(32, iy+dy)*33+min(32, ix+dx)].Elevation }
				if a+c <= 1 {
					tile.Points[at].Parent = z(0, 0)*(1-a-c) + z(1, 0)*a + z(0, 1)*c
				} else {
					tile.Points[at].Parent = z(1, 1)*(a+c-1) + z(1, 0)*(1-c) + z(0, 1)*(1-a)
				}
			} else {
				tile.Points[at].Parent = tile.Points[at].Elevation
			}
		}
	}
	for j := 0; j < 33; j++ {
		for i := 0; i < 33; i++ {
			p := &tile.Points[j*33+i]
			base := original[j*33+i]
			x, y := math.Min(s.W, b.X+float64(i)*tile.Step), math.Min(s.H, b.Y+float64(j)*tile.Step)
			// Add the edit's derivative to the stored global normal. Recomputing
			// the entire tile with one-sided edge differences would introduce
			// lighting seams and change untouched terrain after a small stamp.
			for axis := 0; axis < 2; axis++ {
				a, z := base, base
				a.Elevation -= base.Gradient[axis] * tile.Step
				z.Elevation += base.Gradient[axis] * tile.Step
				dx, dy := 0., 0.
				if axis == 0 {
					dx = tile.Step
				} else {
					dy = tile.Step
				}
				lo := Apply(a, x-dx, y-dy, ops).Elevation - a.Elevation
				hi := Apply(z, x+dx, y+dy, ops).Elevation - z.Elevation
				p.Gradient[axis] = base.Gradient[axis] + (hi-lo)/(2*tile.Step)
			}
			left, right, up, down := tile.Points[j*33+max(0, i-1)], tile.Points[j*33+min(32, i+1)], tile.Points[max(0, j-1)*33+i], tile.Points[min(32, j+1)*33+i]
			bl, br, bu, bd := original[j*33+max(0, i-1)], original[j*33+min(32, i+1)], original[max(0, j-1)*33+i], original[min(32, j+1)*33+i]
			dx := float64(min(32, i+1)-max(0, i-1)) * tile.Step
			dy := float64(min(32, j+1)-max(0, j-1)) * tile.Step
			p.Gradient[2] = base.Gradient[2] + ((right.Parent-br.Parent)-(left.Parent-bl.Parent))/dx
			p.Gradient[3] = base.Gradient[3] + ((down.Parent-bd.Parent)-(up.Parent-bu.Parent))/dy
		}
	}
}

// Coarse view composition is needed when entering an editor, not per brush stroke.
func (s *State) Environment(base *terrain.Environment) *terrain.Environment {
	if base == nil || len(s.Operations) == 0 {
		return base
	}
	e := *base
	e.Fields = make(map[string][]float64, len(base.Fields))
	for name, f := range base.Fields {
		e.Fields[name] = f
	}
	for _, name := range []string{"elevation", "waterBody", "waterLevel", "waterDepth", "vegetation", "river"} {
		e.Fields[name] = make([]float64, len(base.Mask))
		copy(e.Fields[name], base.Fields[name])
	}
	w := base.Options.Columns
	for i := range e.Fields["elevation"] {
		x, y := float64(i%w), float64(i/w)
		ops := s.Ops(Bounds{x, y, 0, 0})
		if len(ops) == 0 {
			continue
		}
		p := Apply(terrain.DetailPoint{Elevation: base.Fields["elevation"][i], WaterBody: int(base.Fields["waterBody"][i]), WaterLevel: base.Fields["waterLevel"][i], WaterDepth: base.Fields["waterDepth"][i], Vegetation: e.Fields["vegetation"][i]}, x, y, ops)
		e.Fields["elevation"][i] = p.Elevation
		e.Fields["waterBody"][i] = float64(p.WaterBody)
		e.Fields["waterLevel"][i] = p.WaterLevel
		e.Fields["waterDepth"][i] = p.WaterDepth
		e.Fields["vegetation"][i] = p.Vegetation
		if p.RiverDepth > 0 {
			e.Fields["river"][i] = 1
		}
	}
	return &e
}
