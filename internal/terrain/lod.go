package terrain

import (
	"fmt"
	"math"
)

const DetailLevels = 8
const DetailTileSize = 32

// World coordinates are the original sample coordinates, not viewport pixels.
// Each octave is a nodal residual: new coefficients are zero on EVERY coarser
// node. Tile boundaries use global indices, so request order cannot form seams.
type DetailPoint struct {
	Shore         float64    `json:"shore,omitempty"`
	ShoreSediment float64    `json:"shoreSediment,omitempty"`
	ShoreSlope    float64    `json:"shoreSlope,omitempty"`
	Marine        float64    `json:"marine,omitempty"`
	Snow          float64    `json:"snow,omitempty"`
	Wetland       float64    `json:"wetland,omitempty"`
	Authored      bool       `json:"authored,omitempty"`
	Gradient      [4]float64 `json:"gradient"`
	Elevation     float64    `json:"elevation"`
	Parent        float64    `json:"parent"`
	WaterLevel    float64    `json:"waterLevel"`
	WaterDepth    float64    `json:"waterDepth"`
	WaterBody     int        `json:"waterBody"`
	Cell          int        `json:"cell"`
	Temperature   float64    `json:"temperature"`
	Moisture      float64    `json:"moisture"`
	Vegetation    float64    `json:"vegetation"`
	Rock          float64    `json:"rock"`
	Sand          float64    `json:"sand"`
	Floodplain    float64    `json:"floodplain"`
	RiverLevel    float64    `json:"riverLevel"`
	RiverDepth    float64    `json:"riverDepth"`
}

type DetailFeature struct {
	RiverID    string       `json:"riverId,omitempty"`
	Class      string       `json:"class,omitempty"`
	NetworkID  string       `json:"networkId,omitempty"`
	Regime     string       `json:"regime,omitempty"`
	Morphology string       `json:"morphology,omitempty"`
	Depth      float64      `json:"depth,omitempty"`
	ID         string       `json:"id"`
	ParentID   string       `json:"parentId"`
	Kind       string       `json:"kind"`
	Level      float64      `json:"level"`
	Width      float64      `json:"width"`
	Widths     []float64    `json:"widths,omitempty"`
	Discharge  float64      `json:"discharge"`
	Grade      float64      `json:"grade"`
	Path       [][3]float64 `json:"path"`
}

type DetailTile struct {
	Level    int             `json:"level"`
	X        int             `json:"x"`
	Y        int             `json:"y"`
	Size     int             `json:"size"`
	Step     float64         `json:"step"`
	Points   []DetailPoint   `json:"points"`
	Features []DetailFeature `json:"features"`
}

type DetailModel struct {
	World     *Environment
	Seed      uint32
	Features  []DetailFeature
	channels  map[int][]int
	foothills []float64
}

func NewDetailModel(e *Environment) *DetailModel {
	m := &DetailModel{World: e, Seed: Seed(e.Options.Seed) ^ 0x91ab6731}
	m.buildFoothills()
	m.buildDrainage()
	m.refineHydroFeatures()
	m.indexDrainage()
	return m
}

func (m *DetailModel) field(name string, x, y float64) float64 {
	w, h := m.World.Options.Columns, m.World.Options.Rows
	x, y = clamp(x, 0, float64(w-1)), clamp(y, 0, float64(h-1))
	x0, y0 := int(x), int(y)
	x1, y1 := min(w-1, x0+1), min(h-1, y0+1)
	a, b := x-float64(x0), y-float64(y0)
	f := m.World.Fields[name]
	if f == nil {
		return 0
	}
	return (f[y0*w+x0]*(1-a)+f[y0*w+x1]*a)*(1-b) + (f[y1*w+x0]*(1-a)+f[y1*w+x1]*a)*b
}

func (m *DetailModel) cell(x, y float64) int {
	w, h := m.World.Options.Columns, m.World.Options.Rows
	return int(clamp(math.Round(y), 0, float64(h-1)))*w + int(clamp(math.Round(x), 0, float64(w-1)))
}

func detailHash(seed uint32, x, y, level int) float64 {
	v := uint32(x)*374761393 ^ uint32(y)*668265263 ^ uint32(level)*2246822519 ^ seed
	v = (v ^ (v >> 13)) * 1274126177
	return float64(v^(v>>16)) / 4294967295
}

// The continuous parent water footprint is fixed for all refinement levels.
// Dry endorheic basins never become water merely because elevation is negative.
func (m *DetailModel) base(x, y float64) DetailPoint {
	// A small invertible, cell-interior coordinate warp resolves parent shores
	// into coves and uneven banks. It fixes every grid edge/node, stays inside
	// its parent cell, and cannot split or join a parent's water footprint.
	for step := 0; step < 3; step++ {
		ux, uy := x-math.Floor(x), y-math.Floor(y)
		vx := .055 * math.Pow(math.Sin(math.Pi*ux), 2) * m.detailNoise(x*2.7, y*2.7, 41)
		vy := .055 * math.Pow(math.Sin(math.Pi*uy), 2) * m.detailNoise(x*2.7, y*2.7, 59)
		x, y = x+vx, y+vy
	}

	c := m.cell(x, y)
	p := DetailPoint{Elevation: m.field("elevation", x, y), Cell: c, Temperature: m.field("temperature", x, y), Moisture: m.field("moisture", x, y)}
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	a, b := x-float64(x0), y-float64(y0)
	weights := []float64{(1 - a) * (1 - b), a * (1 - b), (1 - a) * b, a * b}
	wet, best, level, wetGround, dryGround := 0., 0., 0., 0., 0.
	for k, xy := range [][2]int{{x0, y0}, {x0 + 1, y0}, {x0, y0 + 1}, {x0 + 1, y0 + 1}} {
		j := m.cell(float64(xy[0]), float64(xy[1]))
		body := int(m.World.get("waterBody", j))
		if body > 0 {
			wet += weights[k]
			wetGround += weights[k] * m.World.get("elevation", j)
			level += weights[k] * m.World.get("waterLevel", j)
			if weights[k] > best {
				best = weights[k]
				p.WaterBody = body
			}
		} else {
			dryGround += weights[k] * m.World.get("elevation", j)
		}
	}
	if wet > 0 {
		p.WaterLevel = level / wet
	}
	// Passive margins resolve into beaches and tidal lowlands; active/rocky
	// margins retain steeper profiles. Endpoints stay the original samples.
	shorePower := 1.0 + .4*(1-clamp(m.field("slope", x, y)*2, 0, 1))
	// Continuous banks meet the surface at the inherited shoreline. Clamping
	// a positive interpolated bed below water here would reintroduce a cliff.
	if wet > .5 {
		p.Elevation = p.WaterLevel + math.Pow(2*wet-1, shorePower)*(wetGround/wet-p.WaterLevel)
		p.WaterDepth = p.WaterLevel - p.Elevation
	} else {
		p.WaterBody = 0
		if wet > 0 {
			p.Elevation = p.WaterLevel + math.Pow(1-2*wet, shorePower)*(dryGround/(1-wet)-p.WaterLevel)
		}
	}
	p.Shore = wet
	p.Snow = m.field("snow", x, y)
	p.Wetland = m.field("wetland", x, y)
	p.Vegetation = clamp(p.Moisture*(1-p.Snow), 0, 1)
	return p
}

// Refinement samples one immutable physical surface. Every dyadic parent node
// is shared by all finer grids; camera events never enter the terrain function.
func (m *DetailModel) Sample(x, y float64, level int) DetailPoint {
	x = clamp(x, 0, float64(m.World.Options.Columns-1))
	y = clamp(y, 0, float64(m.World.Options.Rows-1))
	p := m.base(x, y)
	base := p.Elevation
	r := m.riverAt(x, y)
	budget := 650.
	if p.WaterBody > 0 {
		budget = p.WaterDepth * .65
	} else if m.field("waterDepth", x, y) > 0 {
		budget = math.Min(budget, math.Abs(base-p.WaterLevel)*.65)
	}
	budget *= smooth(m.sourceDistance(x, y) / .025)
	p.Parent = base + clamp(m.refinement(x, y, level-1), -budget, budget)
	p.Elevation = base + clamp(m.refinement(x, y, level), -budget, budget)
	if p.WaterBody > 0 {
		p.WaterDepth = p.WaterLevel - p.Elevation
	} else {
		p.Temperature -= (p.Elevation - base) * .0065
	}
	if r.valid && p.WaterBody == 0 {
		p.Floodplain = math.Exp(-math.Pow(r.distance/r.valley, 2)) * smooth(r.discharge/8)
		p.RiverLevel = r.level
		if r.distance < r.width*.5 && level > 0 && r.regime != "ephemeral" {
			p.RiverDepth = math.Max(0, r.level-p.Elevation)
		}
	}
	p.Rock = clamp(m.field("slope", x, y)*1.2+m.field("mountainCore", x, y)*.4, 0, 1)
	p.Sand = clamp(m.field("dune", x, y)+m.field("sandSupply", x, y)*.15*m.field("aridity", x, y), 0, 1) * (1 - p.Rock)
	p.ShoreSediment = clamp(m.field("sediment", x, y)+m.field("coastalSediment", x, y), 0, 1)
	p.ShoreSlope = m.field("slope", x, y)
	p.Marine = m.field("ocean", x, y)
	if p.Shore > 0 && p.WaterBody == 0 {
		// A narrow sub-cell shore band, measured from the continuous water
		// footprint. Steep mountain lakes retain rock down to the waterline.
		sediment := p.ShoreSediment
		gentle := 1 - smooth(p.ShoreSlope/.16)
		marine := p.Marine > 0
		width, height := .05, 2.
		if marine {
			width, height = .12, 8.
		}
		shore := smooth((p.Shore-(.5-width))/width) * math.Exp(-math.Abs(p.Elevation-p.WaterLevel)/height)
		p.Sand = math.Max(p.Sand, shore*sediment*gentle)
		// Low sediment supply creates a stony shore, not vegetation extending
		// to the water. Steep margins retain exposed rock instead of sand.
		p.Rock = math.Max(p.Rock, shore*(1-sediment*gentle)*.9)
		p.Vegetation *= 1 - shore*.85
	}
	p.Vegetation = clamp(p.Vegetation+p.Floodplain*.18-p.Rock*.3-p.Sand*.35, 0, 1)
	return p
}

func segmentDistance(x, y, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	t := 0.
	if dx*dx+dy*dy > 0 {
		t = clamp(((x-ax)*dx+(y-ay)*dy)/(dx*dx+dy*dy), 0, 1)
	}
	return math.Hypot(x-ax-t*dx, y-ay-t*dy)
}

func (m *DetailModel) buildDrainage() {
	e := m.World
	w := e.Options.Columns
	visible := e.VisibleChannels()
	incoming := make([]int, len(e.Mask))
	for i := range incoming {
		incoming[i] = -1
	}
	for i := range e.Mask {
		j := int(e.get("flow", i))
		if j >= 0 && (incoming[j] < 0 || e.get("accumulation", i) > e.get("accumulation", incoming[j])) {
			incoming[j] = i
		}
	}
	tangent := func(i int) (float64, float64) {
		x, y := 0., 0.
		if from := incoming[i]; from >= 0 {
			x += float64(i%w - from%w)
			y += float64(i/w - from/w)
		}
		if to := int(e.get("flow", i)); to >= 0 {
			x += float64(to%w - i%w)
			y += float64(to/w - i/w)
		}
		length := math.Hypot(x, y)
		if length > 0 {
			x /= length
			y /= length
		}
		return x, y
	}
	for i := range e.Mask {
		j := int(e.get("flow", i))
		if j < 0 || e.Mask[i] == 0 || e.get("waterBody", i) > 0 || !visible[i] {
			continue
		}
		z0, z1 := e.get("drainageElevation", i), e.get("drainageElevation", j)
		if z1 > z0 {
			continue
		}
		level := clamp(6-math.Log2(1+e.get("accumulation", i))*1.1, 0, 6)
		id := fmt.Sprintf("%x/drainage/%d", m.Seed, i)
		// Curves preserve all original junctions and use the same tangent on both
		// sides of a confluence, resolving grid routes without relocating them.
		ax, ay, bx, by := float64(i%w), float64(i/w), float64(j%w), float64(j/w)
		dx0, dy0 := tangent(i)
		dx1, dy1 := tangent(j)
		q := e.get("accumulation", i)
		grade := math.Max(0, z0-z1)
		widthAt := func(c int) float64 {
			slope := 0.
			if to := int(e.get("flow", c)); to >= 0 {
				slope = math.Max(0, e.get("drainageElevation", c)-e.get("drainageElevation", to))
			}
			return (.018 + math.Min(.32, math.Sqrt(e.get("accumulation", c))*.012)) * (.45 + .55/(1+slope/180))
		}
		path := make([][3]float64, 25)
		widths := make([]float64, len(path))
		bend := (detailHash(m.Seed, i, j, 17)*2 - 1) * .18 / (1 + grade/90)
		nx, ny := -(by - ay), bx-ax
		for k := range path {
			t := float64(k) / float64(len(path)-1)
			u := 1 - t
			path[k] = [3]float64{u*u*u*ax + 3*u*u*t*(ax+dx0*.28) + 3*u*t*t*(bx-dx1*.28) + t*t*t*bx, u*u*u*ay + 3*u*u*t*(ay+dy0*.28) + 3*u*t*t*(by-dy1*.28) + t*t*t*by, z0*(1-t) + z1*t}
			wave := bend * math.Pow(math.Sin(math.Pi*t), 2) * math.Sin(2*math.Pi*t)
			path[k][0] += nx * wave
			path[k][1] += ny * wave
			widths[k] = (widthAt(i)*(1-t) + widthAt(j)*t) * (1 + .07*math.Sin(6*math.Pi*t)*math.Sin(math.Pi*t))
		}

		if e.Hydrology != nil && e.Hydrology.NetworkVersion >= 2 && e.get("waterBody", j) == 0 {
			// Preserve both confluence endpoints while bringing coastal meanders back
			// onto the inherited land corridor. Never clip an interior reach in half.
			original := append([][3]float64(nil), path...)
			for attempt := 0; attempt < 12; attempt++ {
				dry := true
				for _, p := range path {
					if m.base(p[0], p[1]).WaterBody > 0 {
						dry = false
						break
					}
				}
				if dry {
					break
				}
				factor := math.Pow(.5, float64(attempt+1))
				if attempt == 11 {
					factor = 0
				}
				for k := range path {
					t := float64(k) / float64(len(path)-1)
					x, y := ax+(bx-ax)*t, ay+(by-ay)*t
					path[k][0] = x + (original[k][0]-x)*factor
					path[k][1] = y + (original[k][1]-y)*factor
				}
			}
		}
		// End at the continuously refined shore, not at a submerged grid-cell
		// centre. The receiving lake/ocean takes over the water surface there.
		for k := 1; k < len(path); k++ {
			wet := m.base(path[k][0], path[k][1])
			if wet.WaterBody == 0 {
				continue
			}
			a, b := path[k-1], path[k]
			lo, hi := 0., 1.
			for n := 0; n < 24; n++ {
				t := (lo + hi) / 2
				p := m.base(a[0]+(b[0]-a[0])*t, a[1]+(b[1]-a[1])*t)
				if p.WaterBody > 0 {
					hi = t
				} else {
					lo = t
				}
			}
			end := lo
			path[k] = [3]float64{a[0] + (b[0]-a[0])*end, a[1] + (b[1]-a[1])*end, wet.WaterLevel}
			widths[k] = widths[k-1] + (widths[k]-widths[k-1])*end
			path = path[:k+1]
			widths = widths[:k+1]
			for n := range path {
				t := float64(n) / float64(k)
				path[n][2] = z0*(1-t) + wet.WaterLevel*t
			}
			break
		}
		m.Features = append(m.Features, DetailFeature{ID: id, ParentID: fmt.Sprintf("%x/drainage/%d", m.Seed, j), Kind: "river", Level: level, Width: widths[0], Widths: widths, Discharge: q, Grade: grade, Path: path})

	}
}

func (m *DetailModel) Tile(level, tx, ty int) (DetailTile, error) {
	span := float64(DetailTileSize) / math.Exp2(float64(level))
	if level < 0 || level > DetailLevels || tx < 0 || ty < 0 || float64(tx)*span >= float64(m.World.Options.Columns-1) || float64(ty)*span >= float64(m.World.Options.Rows-1) {
		return DetailTile{}, fmt.Errorf("detail tile outside world or supported levels")
	}
	t := DetailTile{Level: level, X: tx, Y: ty, Size: DetailTileSize + 1, Step: span / DetailTileSize, Points: make([]DetailPoint, 0, (DetailTileSize+1)*(DetailTileSize+1)), Features: []DetailFeature{}}
	x0, y0 := float64(tx)*span, float64(ty)*span
	// One-sample halo supplies identical derivatives on independently requested
	// tile borders. Without it, lighting creates seams even on matching geometry.
	const haloSize = DetailTileSize + 3
	halo := make([]DetailPoint, haloSize*haloSize)
	parentNodes := map[[2]int]float64{}
	parentNode := func(x, y int) float64 {
		key := [2]int{x, y}
		if z, ok := parentNodes[key]; ok {
			return z
		}
		z := m.Sample(float64(x)*t.Step*2, float64(y)*t.Step*2, level-1).Elevation
		parentNodes[key] = z
		return z
	}
	for y := -1; y <= DetailTileSize+1; y++ {
		for x := -1; x <= DetailTileSize+1; x++ {
			wx, wy := x0+float64(x)*t.Step, y0+float64(y)*t.Step
			p := m.Sample(wx, wy, level)
			if level > 0 {
				// Geomorph from the actual parent triangles, not a separately
				// evaluated curved surface between their vertices. Arrival at
				// blend zero is therefore exactly the existing parent mesh.
				u, v := wx/(2*t.Step), wy/(2*t.Step)
				ix, iy := int(math.Floor(u)), int(math.Floor(v))
				a, b := u-float64(ix), v-float64(iy)
				if a+b <= 1 {
					p.Parent = parentNode(ix, iy)*(1-a-b) + parentNode(ix+1, iy)*a + parentNode(ix, iy+1)*b
				} else {
					p.Parent = parentNode(ix+1, iy+1)*(a+b-1) + parentNode(ix+1, iy)*(1-b) + parentNode(ix, iy+1)*(1-a)
				}
			}
			halo[(y+1)*haloSize+x+1] = p
		}
	}
	for y := 0; y <= DetailTileSize; y++ {
		for x := 0; x <= DetailTileSize; x++ {
			at := (y+1)*haloSize + x + 1
			p := halo[at]
			left, right, up, down := halo[at-1], halo[at+1], halo[at-haloSize], halo[at+haloSize]
			p.Gradient = [4]float64{(right.Elevation - left.Elevation) / (2 * t.Step), (down.Elevation - up.Elevation) / (2 * t.Step), (right.Parent - left.Parent) / (2 * t.Step), (down.Parent - up.Parent) / (2 * t.Step)}
			t.Points = append(t.Points, p)
		}
	}
	for _, f := range m.Features {
		// Keep the saved network available to diagnostic overlays at all scales.
		// Normal rendering still applies each feature's visual level threshold.
		if f.Level > float64(level)+1 && f.RiverID == "" {
			continue
		}
		minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		for _, p := range f.Path {
			minX = math.Min(minX, p[0])
			minY = math.Min(minY, p[1])
			maxX = math.Max(maxX, p[0])
			maxY = math.Max(maxY, p[1])
		}
		if maxX >= x0 && minX <= x0+span && maxY >= y0 && minY <= y0+span {
			t.Features = append(t.Features, f)
		}
	}
	return t, nil
}
