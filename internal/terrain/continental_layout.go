package terrain

import (
	"container/heap"
	"fmt"
	"math"
)

// Layout constraints are generation provenance, never a recipe for reloading
// stored terrain. Zero majorContinents chooses 4–6 from the seed.
type ContinentalLayout struct {
	Count                 int     `json:"count"`
	GapCells              float64 `json:"gapCells"`
	RequestedLandPercent  float64 `json:"requestedLandPercent"`
	FoundationLandPercent float64 `json:"foundationLandPercent"`
	Warning               string  `json:"warning,omitempty"`
}

func (o EnvironmentOptions) oceanGap() float64 {
	fraction := .08
	if o.OceanSeparation == "narrow" {
		fraction = .05
	}
	if o.OceanSeparation == "wide" {
		fraction = .12
	}
	return math.Max(1, fraction*float64(min(o.Columns, o.Rows)))
}

func (e *Environment) layoutCenters() []Point {
	w, h := float64(e.Options.Columns), float64(e.Options.Rows)
	rng := RNG(Seed(e.Options.Seed) ^ 0x8ec0137b)
	count := e.Options.MajorContinents
	if count == 0 {
		count = 4 + int(rng.Next()*3)
	}
	centers := []Point{}
	// Random sequential inhibition keeps room for independent footprints without
	// maximizing every distance (which produced an almost regular latitude grid).
	spacing := math.Sqrt(w*h/float64(count)) * .68
	for k := 0; k < count; k++ {
		best := math.Inf(-1)
		var chosen Point
		for trial := 0; trial < 256; trial++ {
			p := Point{X: (.12 + rng.Next()*.76) * w, Y: (.12 + rng.Next()*.76) * h, K: 1, Kind: Kind{ID: "land", Name: "Major continent", Color: "#9ebd79"}}
			score := math.Inf(1)
			for _, old := range centers {
				dx := p.X - old.X
				if e.Geography != nil && e.Geography.WrapX {
					dx = math.Remainder(dx, w)
				}
				score = math.Min(score, math.Hypot(dx, p.Y-old.Y))
			}
			if score > best {
				best = score
				chosen = p
			}
			if score >= spacing*(.85+rng.Next()*.3) {
				chosen = p
				break
			}
		}
		centers = append(centers, chosen)
	}
	return centers
}

func (e *Environment) layoutNuclei() []LandmassStructure {
	e.Points = e.layoutCenters()
	shapes := make([]LandmassStructure, 0, len(e.Points))
	radius := math.Sqrt(float64(len(e.Mask)) * e.Options.LandPercent / 100 / float64(len(e.Points)) / math.Pi)
	for k, p := range e.Points {
		rng := RNG(Seed(e.Options.Seed) ^ uint32(k+1)*0x471cab31)
		names := []string{"compact", "elongated", "indented", "indented", "curved", "isthmus", "hook", "branched", "crescent", "rifted"}
		name := names[int(rng.Next()*float64(len(names)))]
		a, b, _, _, convergence, bearing := e.platesAt(p.X, p.Y)
		s := makeLandmass(Seed(e.Options.Seed)^uint32(k+1)*0x738a913, name, [2]float64{p.X, p.Y}, radius*(.6+rng.Next()*.8), bearing+(rng.Next()-.5)*math.Pi*2, e.Options.shapeOptions())
		s.ID = fmt.Sprintf("continent/%08x/%d", Seed(e.Options.Seed), k)
		s.Plate, s.OtherPlate = a, b
		s.Process = "continental-foundation"
		if convergence > .3 {
			s.Process = "continental-collision"
		}
		if convergence < -.3 {
			s.Process = "continental-rifting"
		}
		shapes = append(shapes, s)
	}
	return shapes
}

type layoutCell struct {
	cell  int
	score float64
}
type layoutQueue []layoutCell

func (q layoutQueue) Len() int { return len(q) }
func (q layoutQueue) Less(i, j int) bool {
	if q[i].score == q[j].score {
		return q[i].cell < q[j].cell
	}
	return q[i].score > q[j].score
}
func (q layoutQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *layoutQueue) Push(v any)   { *q = append(*q, v.(layoutCell)) }
func (q *layoutQueue) Pop() any     { old := *q; v := old[len(old)-1]; *q = old[:len(old)-1]; return v }

func (e *Environment) separatedContinentalMask(sample func(float64, float64) float64) {
	w, h := e.Options.Columns, e.Options.Rows
	n := w * h
	e.ContinentalStructures = e.layoutNuclei()
	count := len(e.ContinentalStructures)
	gap := e.Options.oceanGap()
	e.Layout = &ContinentalLayout{Count: count, GapCells: gap, RequestedLandPercent: e.Options.LandPercent}
	owners := make([]int, n)
	scores := make([]float64, n)
	seams := make([]int, n)
	wrap := e.Geography != nil && e.Geography.WrapX
	// Deform the partition at continental scale before reserving ocean room.
	// This creates winding basins, not straight Voronoi cuts painted afterward.
	for i := range owners {
		x, y := float64(i%w), float64(i/w)
		angle := x / float64(w) * 2 * math.Pi
		px := x + (sample(math.Cos(angle)*2+41, math.Sin(angle)*2+y/float64(h)*4+17)-.5)*float64(min(w, h))*.12
		py := y + (sample(math.Cos(angle)*2+83, math.Sin(angle)*2+y/float64(h)*4+61)-.5)*float64(min(w, h))*.10
		best := math.Inf(1)
		for k, s := range e.ContinentalStructures {
			dx := px - s.Center[0]
			if wrap {
				dx = math.Remainder(dx, float64(w))
			}
			// Footprint-aware competition follows the actual lobes, bends and
			// gulfs. A center-only Voronoi partition clipped them into polygons.
			d := math.Hypot(dx, py-s.Center[1])/math.Sqrt(s.Scale) - 3*s.field(s.Center[0]+dx, py)
			if d < best {
				best = d
				owners[i] = k
			}
		}
		s := e.ContinentalStructures[owners[i]]
		sx := x
		if wrap {
			sx = s.Center[0] + math.Remainder(x-s.Center[0], float64(w))
		}
		scores[i] = s.field(sx, y)
		{
			// A smooth polar ocean bias rounds coastlines before the safety margin;
			// quota growth must not flatten an entire coast against a map edge.
			edge := math.Min(y, float64(h-1)-y)
			edge = math.Min(edge, math.Min(x, float64(w-1)-x))
			scores[i] -= 3 * math.Exp(-edge/(float64(min(w, h))*.055))
		}
	}
	for i := range seams {
		seams[i] = 1
		for _, j := range e.neighbors(i) {
			if owners[i] != owners[j] {
				seams[i] = 0
				break
			}
		}
	}
	// Exact Euclidean distance to the boundary, including the longitude seam.
	boundary := []int{}
	for i, v := range seams {
		if v == 0 {
			boundary = append(boundary, i)
		}
	}
	capacity := make([]int, count)
	starts := make([]int, count)
	for k := range starts {
		starts[k] = -1
	}
	eligible := make([]bool, n)
	e.Fields["oceanReserve"] = make([]float64, n)
	e.Fields["continentalOwner"] = make([]float64, n)
	for i := range owners {
		x, y := i%w, i/w
		distance := math.Inf(1)
		for _, j := range boundary {
			dx := math.Abs(float64(x - j%w))
			if wrap {
				dx = math.Min(dx, float64(w)-dx)
			}
			dy := float64(y - j/w)
			distance = math.Min(distance, dx*dx+dy*dy)
		}
		// Half a cell extra protects diagonal connections and later shore erosion.
		reserve := distance < math.Pow(gap*.5+.5, 2)
		edge := math.Min(float64(y), float64(h-1-y))
		edge = math.Min(edge, math.Min(float64(x), float64(w-1-x)))
		// Keep geographic-scale bays in the frame itself when a high land target
		// grows all the way to it. Sub-cell noise alone quantized into long lines.
		coarse := sample(float64(x)/float64(w)*9+217, float64(y)/float64(h)*11+193) - .5
		fine := sample(float64(x)/float64(w)*23+317, float64(y)/float64(h)*21+393) - .5
		polarRoom := math.Max(2, gap*.55+1+float64(min(w, h))*(.03+.06*coarse+.09*fine))
		// Map framing is independent of cylindrical physics: wrap distances and
		// water transport remain valid, but the displayed cut passes through sea.
		reserve = reserve || edge <= polarRoom
		eligible[i] = !reserve && edge > polarRoom
		if reserve {
			e.Fields["oceanReserve"][i] = 1
		}
		e.Fields["continentalOwner"][i] = float64(owners[i] + 1)
		if eligible[i] {
			k := owners[i]
			capacity[k]++
			if starts[k] < 0 || scores[i] > scores[starts[k]] {
				starts[k] = i
			}
		}
	}
	// Allocate coverage to foundations, then grow connected interiors. Oceans
	// outrank the area target when constraints conflict; report any shortfall.
	total := 0
	for _, v := range capacity {
		total += v
	}
	wanted := min(total, int(round(float64(n)*e.Options.LandPercent/100)))
	used := 0
	visited := make([]bool, n)
	frontier := &layoutQueue{}
	heap.Init(frontier)
	for k, start := range starts {
		if start < 0 {
			continue
		}
		// Establish each major interior, then allocate the remaining area by
		// organic footprint score across all continents. Proportional capacity
		// quotas forced even tiny shapes to fill their Voronoi compartments.
		quota := min(capacity[k], max(1, int(float64(wanted)/float64(count)*.45)))
		q := &layoutQueue{{start, scores[start]}}
		heap.Init(q)
		visited[start] = true
		for taken := 0; q.Len() > 0 && taken < quota; taken++ {
			cell := heap.Pop(q).(layoutCell).cell
			e.Mask[cell] = 1
			used++
			for _, j := range e.neighbors(cell) {
				if eligible[j] && owners[j] == k && !visited[j] {
					visited[j] = true
					heap.Push(q, layoutCell{j, scores[j]})
				}
			}
		}
		for q.Len() > 0 {
			heap.Push(frontier, heap.Pop(q))
		}
	}
	for frontier.Len() > 0 && used < wanted {
		cell := heap.Pop(frontier).(layoutCell).cell
		e.Mask[cell] = 1
		used++
		for _, j := range e.neighbors(cell) {
			if eligible[j] && owners[j] == owners[cell] && !visited[j] {
				visited[j] = true
				heap.Push(frontier, layoutCell{j, scores[j]})
			}
		}
	}
	e.Fields["continentalShape"] = scores
	e.Layout.FoundationLandPercent = float64(used) * 100 / float64(n)
	// Automatic count is a range, not a promise of six crowded masses. If a
	// particular set of irregular footprints cannot fit, retry with fewer major
	// foundations before compromising the requested area or any ocean reserve.
	if e.Options.MajorContinents == 0 && count > 4 && e.Layout.FoundationLandPercent < e.Options.LandPercent-1 {
		for reduced := count - 1; reduced >= 4; reduced-- {
			e.Options.MajorContinents = reduced
			clear(e.Mask)
			e.separatedContinentalMask(sample)
			if e.Layout.FoundationLandPercent >= e.Options.LandPercent-1 {
				break
			}
		}
		e.Options.MajorContinents = 0
		return
	}
	if e.Layout.FoundationLandPercent < e.Options.LandPercent-.1 {
		e.Layout.Warning = fmt.Sprintf("Ocean separation and connected continents take priority: requested %.1f%% land, achieved %.1f%% continental foundations. Reduce continent count or ocean separation for more land.", e.Options.LandPercent, e.Layout.FoundationLandPercent)
	}
}
