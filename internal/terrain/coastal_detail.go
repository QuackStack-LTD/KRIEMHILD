package terrain

import "math"

// Versioned recipe is opt-in for new generated worlds. Old saves and imports
// have no recipe and retain their original detail shoreline semantics.
type CoastalModel struct {
	Version   string  `json:"version"`
	Seed      uint32  `json:"seed"`
	Band      float64 `json:"bandCells"`
	Amplitude float64 `json:"amplitudeCells"`
}

// Squared Euclidean distance transform: lower envelopes of parabolas in two
// separable passes, O(width*height). Tripled rows implement horizontal wrapping.
func euclideanDistance(target []bool, w, h int, wrap bool) []float64 {
	const far = 1e12
	span := max(w, h)
	if wrap {
		span = max(3*w, h)
	}
	f, d := make([]float64, span), make([]float64, span)
	v, z := make([]int, span), make([]float64, span+1)
	transform := func(n int) {
		k := 0
		v[0] = 0
		z[0] = math.Inf(-1)
		z[1] = math.Inf(1)
		for q := 1; q < n; q++ {
			s := ((f[q] + float64(q*q)) - (f[v[k]] + float64(v[k]*v[k]))) / float64(2*(q-v[k]))
			for s <= z[k] {
				k--
				s = ((f[q] + float64(q*q)) - (f[v[k]] + float64(v[k]*v[k]))) / float64(2*(q-v[k]))
			}
			k++
			v[k] = q
			z[k] = s
			z[k+1] = math.Inf(1)
		}
		k = 0
		for q := 0; q < n; q++ {
			for z[k+1] < float64(q) {
				k++
			}
			a := q - v[k]
			d[q] = float64(a*a) + f[v[k]]
		}
	}
	out := make([]float64, w*h)
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			f[y] = far
			if target[y*w+x] {
				f[y] = 0
			}
		}
		transform(h)
		for y := 0; y < h; y++ {
			out[y*w+x] = d[y]
		}
	}
	length := w
	if wrap {
		length = 3 * w
	}
	for y := 0; y < h; y++ {
		for x := 0; x < length; x++ {
			f[x] = out[y*w+x%w]
		}
		transform(length)
		offset := 0
		if wrap {
			offset = w
		}
		for x := 0; x < w; x++ {
			out[y*w+x] = math.Sqrt(d[x+offset])
		}
	}
	return out
}

func (m *DetailModel) buildCoastalDistance() {
	e := m.World
	if e.Coastline == nil || e.Coastline.Version != "coastal-sdf-1" {
		return
	}
	w, h := e.Options.Columns, e.Options.Rows
	land, sea := make([]bool, w*h), make([]bool, w*h)
	for i := range land {
		sea[i] = e.get("waterBody", i) == 1
		land[i] = !sea[i]
	}
	// Protect already-routed inland channel corridors before refining their
	// geometry. New marine mouths may move, but a tributary must not be severed
	// from a dry parent junction by a decorative coastal cove.
	m.coastChannels = map[int][][4]float64{}
	visible := e.VisibleChannels()
	for i, v := range visible {
		if !v {
			continue
		}
		j := int(e.get("flow", i))
		if j < 0 || e.get("waterBody", i) > 0 || e.get("waterBody", j) > 0 {
			continue
		}
		ax, ay, bx, by := float64(i%w), float64(i/w), float64(j%w), float64(j/w)
		if math.Abs(ax-bx) > 2 || math.Abs(ay-by) > 2 {
			continue
		}
		for y := max(0, int(math.Min(ay, by))-2); y <= min(h-1, int(math.Max(ay, by))+2); y++ {
			for x := max(0, int(math.Min(ax, bx))-2); x <= min(w-1, int(math.Max(ax, bx))+2); x++ {
				m.coastChannels[y*w+x] = append(m.coastChannels[y*w+x], [4]float64{ax, ay, bx, by})
			}
		}
	}
	wrap := e.Geography != nil && e.Geography.WrapX
	toLand, toSea := euclideanDistance(land, w, h, wrap), euclideanDistance(sea, w, h, wrap)
	m.coastDistance = make([]float64, w*h)
	for i := range land {
		m.coastDistance[i] = toSea[i] - .5
		if sea[i] {
			m.coastDistance[i] = .5 - toLand[i]
		}
	}
}

func (m *DetailModel) coastalWet(x, y, wet float64, body int) float64 {
	if len(m.coastDistance) == 0 || body > 1 {
		return wet
	}
	c := m.World.Coastline
	d := m.values(m.coastDistance, x, y)
	if math.Abs(d) >= c.Band {
		return wet
	}
	// Preserve every coarse anchor, including edited terrain. The field is
	// level-independent: finer meshes discover new nodes, never move old nodes.
	nodes := (math.Pow(math.Sin(math.Pi*x), 2) + math.Pow(math.Sin(math.Pi*y), 2)) * .5
	if nodes < 1e-12 {
		return wet
	}
	w, h := float64(m.World.Options.Columns), float64(m.World.Options.Rows)
	edge := math.Min(math.Min(x, w-1-x), math.Min(y, h-1-y))
	fade := smooth((c.Band-math.Abs(d))/.8) * smooth((edge-1.5)/1.5)
	for _, s := range m.coastChannels[m.cell(x, y)] {
		fade *= smooth((segmentDistance(x, y, s[0], s[1], s[2], s[3]) - .6) / .4)
	}
	if cover := m.World.Fields["authoredCover"]; len(cover) > 0 && cover[m.cell(x, y)] > 0 {
		return wet
	}
	// River geometry is clipped to this same immutable field during construction.
	// Do not consult its spatial index here: that would make the shoreline depend
	// on whether drainage construction has completed.
	n := noise(c.Seed)
	wx := x + (n(x*.43+13, y*.43+79)-.5)*.4
	wy := y + (n(x*.43+51, y*.43+31)-.5)*.4
	residual := (n(wx*2.1+17, wy*2.1+29)-.5)*1.5 + (n(wx*6.3+47, wy*6.3+61)-.5)*.6 + (n(wx*17+97, wy*17+113)-.5)*.18
	return clamp(wet-residual*c.Amplitude*nodes*fade, 0, 1)
}
