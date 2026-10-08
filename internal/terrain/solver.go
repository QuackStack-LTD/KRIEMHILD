package terrain

import (
	"fmt"
	"math"
	"math/bits"
	"sort"
)

type ContinentOptions struct {
	Count    int     `json:"count"`
	Strength float64 `json:"strength"`
}
type ClimateOptions struct {
	Strength float64 `json:"strength"`
	Layout   string  `json:"layout"`
}
type Options struct {
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	Radius2    int               `json:"radius2"`
	Selection  string            `json:"selection"`
	Stability  float64           `json:"stability"`
	Continents *ContinentOptions `json:"continents"`
	Climate    *ClimateOptions   `json:"climate"`
	WrapX      bool              `json:"wrapX"`
	Seed       uint32            `json:"seed"`
}
type Point struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	K    int     `json:"k"`
	Kind Kind    `json:"kind"`
}
type Wave struct{ FX, FY, Phase float64 }
type Choice struct {
	Type   int     `json:"type"`
	Weight float64 `json:"weight"`
}
type Snapshot struct {
	Dom    []uint32 `json:"dom"`
	Locked []int    `json:"locked"`
	Pinned []int    `json:"pinned"`
	Status string   `json:"status"`
}
type Solver struct {
	Rules                                                                             *Rules
	W, H, N, T, K, Z, Radius2                                                         int
	WrapX                                                                             bool
	Selection                                                                         string
	Stability                                                                         float64
	RNG                                                                               RNG
	Dom                                                                               []uint32
	Locked, Pinned, Pos                                                               []int
	EnvironmentMasks                                                                  []uint32
	Environment                                                                       *Environment
	Full                                                                              uint32
	Offsets                                                                           [][2]int
	Queue                                                                             []int
	InQueue                                                                           []bool
	Buckets                                                                           [][]int
	Open                                                                              int
	Compat                                                                            map[uint32]uint32
	Status, Message                                                                   string
	Steps, Backtracks, Repairs, Cleaned, MaxRepairs, LastRepair, LastRadius, Conflict int
	TrailCell                                                                         []int
	TrailMask                                                                         []uint32
	Recording                                                                         bool
	Prefer                                                                            []int
	ClimShare, ContShare                                                              []float32
	ClimStrength, ContStrength, ContNeutral                                           float64
	ClimBoth                                                                          bool
	ClimWaves                                                                         []Wave
	ContPoints                                                                        []Point
}

func NewSolver(r *Rules, o Options, masks []uint32) *Solver {
	s := &Solver{Rules: r, W: o.Width, H: o.Height, N: o.Width * o.Height, T: len(r.Config.Types), K: len(r.Config.Continents), Z: len(r.Config.Climates), Radius2: max(1, o.Radius2), WrapX: o.WrapX, Selection: o.Selection, Stability: math.Max(0, o.Stability), RNG: RNG(o.Seed), EnvironmentMasks: masks, LastRepair: -1, LastRadius: 2, Conflict: -1, Compat: map[uint32]uint32{}}
	s.Full = uint32((uint64(1) << s.T) - 1)
	s.MaxRepairs = max(2000, s.N)
	if masks == nil {
		var water uint32
		for i, t := range r.Config.Types {
			if t.ID == "water" || t.ID == "deep_water" || t.EnvironmentType == "water" || t.EnvironmentType == "deep_water" {
				water |= bit(i)
			}
		}
		if water != 0 {
			s.EnvironmentMasks = make([]uint32, s.N)
			for i := range s.EnvironmentMasks {
				s.EnvironmentMasks[i] = s.Full
				if min(i%s.W, i/s.W, s.W-1-i%s.W, s.H-1-i/s.W) < 2 {
					s.EnvironmentMasks[i] = water
				}
			}
		}
	}
	rad := int(math.Sqrt(float64(s.Radius2)))
	for dy := -rad; dy <= rad; dy++ {
		for dx := -rad; dx <= rad; dx++ {
			if (dx != 0 || dy != 0) && dx*dx+dy*dy <= s.Radius2 {
				s.Offsets = append(s.Offsets, [2]int{dx, dy})
			}
		}
	}
	s.buildClimate(o.Climate, o.Seed)
	s.buildContinents(o.Continents, o.Seed)
	s.Dom = make([]uint32, s.N)
	s.Locked = make([]int, s.N)
	s.Pinned = make([]int, s.N)
	s.Pos = make([]int, s.N)
	s.InQueue = make([]bool, s.N)
	s.Buckets = make([][]int, s.T+1)
	s.reset()
	return s
}
func (s *Solver) neighbor(c int, d [2]int) int {
	nx, ny := c%s.W+d[0], c/s.W+d[1]
	if ny < 0 || ny >= s.H {
		return -1
	}
	if nx < 0 || nx >= s.W {
		if !s.WrapX {
			return -1
		}
		nx = (nx + s.W) % s.W
	}
	return ny*s.W + nx
}
func (s *Solver) TypeAt(c int) int {
	m := s.Dom[c]
	if m != 0 && m&(m-1) == 0 {
		return bits.Len32(m) - 1
	}
	return -1
}
func (s *Solver) Write(c int, m uint32) {
	if s.EnvironmentMasks != nil {
		m &= s.EnvironmentMasks[c]
	}
	k0, k1 := bits.OnesCount32(s.Dom[c]), bits.OnesCount32(m)
	s.Dom[c] = m
	if k0 != k1 {
		if k0 >= 2 {
			b := s.Buckets[k0]
			i, last := s.Pos[c], b[len(b)-1]
			b = b[:len(b)-1]
			if last != c {
				b[i] = last
				s.Pos[last] = i
			}
			s.Buckets[k0] = b
			s.Open--
		}
		if k1 >= 2 {
			s.Pos[c] = len(s.Buckets[k1])
			s.Buckets[k1] = append(s.Buckets[k1], c)
			s.Open++
		}
	}
}
func (s *Solver) set(c int, m uint32) {
	if s.Recording {
		s.TrailCell = append(s.TrailCell, c)
		s.TrailMask = append(s.TrailMask, s.Dom[c])
	}
	s.Write(c, m)
}
func (s *Solver) enqueue(c int) {
	if !s.InQueue[c] {
		s.InQueue[c] = true
		s.Queue = append(s.Queue, c)
	}
}
func (s *Solver) clearQueue() {
	for _, c := range s.Queue {
		s.InQueue[c] = false
	}
	s.Queue = s.Queue[:0]
}
func (s *Solver) propagate() bool {
	for len(s.Queue) > 0 {
		c := s.Queue[len(s.Queue)-1]
		s.Queue = s.Queue[:len(s.Queue)-1]
		s.InQueue[c] = false
		m := s.Dom[c]
		compat, ok := s.Compat[m]
		if !ok {
			for t := 0; t < s.T; t++ {
				if m&bit(t) != 0 {
					compat |= s.Rules.Allowed[t]
				}
			}
			s.Compat[m] = compat
		}
		for _, d := range s.Offsets {
			n := s.neighbor(c, d)
			if n < 0 {
				continue
			}
			m2 := s.Dom[n] & compat
			if m2 == s.Dom[n] {
				continue
			}
			s.set(n, m2)
			if m2 == 0 {
				s.Conflict = n
				s.clearQueue()
				return false
			}
			s.enqueue(n)
		}
	}
	return true
}
func (s *Solver) reset() {
	copy(s.Locked, s.Pinned)
	for c := 0; c < s.N; c++ {
		if s.Locked[c] == 0 && s.Dom[c] != s.Full {
			s.Write(c, s.Full)
		}
	}
	for c := 0; c < s.N; c++ {
		s.enqueue(c)
	}
	s.Status = "failed"
	if s.propagate() {
		s.Status = "running"
		if s.Open == 0 {
			s.Status = "done"
		}
	} else {
		s.Message = "These rules can’t fill the grid at all: some cell ends up with no possible type."
	}
}
func (s *Solver) selectCell() int {
	if s.Open == 0 {
		return -1
	}
	if s.Selection == "entropy" {
		for k := 2; k <= s.T; k++ {
			b := s.Buckets[k]
			if len(b) > 0 {
				return b[int(s.RNG.Next()*float64(len(b)))]
			}
		}
		return -1
	}
	r := int(s.RNG.Next() * float64(s.Open))
	for k := 2; k <= s.T; k++ {
		b := s.Buckets[k]
		if r < len(b) {
			return b[r]
		}
		r -= len(b)
	}
	return -1
}
func (s *Solver) OptionsFor(c int) []Choice {
	counts := make([]int, s.T)
	settled := 0
	var nearMask uint32
	for _, d := range s.Offsets {
		n := s.neighbor(c, d)
		if n < 0 {
			continue
		}
		m := s.Dom[n]
		if m != 0 && m&(m-1) == 0 {
			nearMask |= m
			counts[bits.Len32(m)-1]++
			settled++
		}
	}
	out := []Choice{}
	for t := 0; t < s.T; t++ {
		if s.Dom[c]&bit(t) == 0 {
			continue
		}
		boosted := -1.
		for j := 0; j < s.T; j++ {
			if nearMask&bit(j) != 0 && s.Rules.Near[t*s.T+j] > boosted {
				boosted = s.Rules.Near[t*s.T+j]
			}
		}
		w := s.Rules.Weight[t]
		if boosted >= 0 {
			w = boosted
		}
		if s.Environment != nil && s.Environment.Options.Realism {
			def := s.Rules.Config.Types[t]
			id := def.ID
			if def.EnvironmentType != "" {
				id = def.EnvironmentType
			}
			w *= s.Environment.Suitability(c, id)
		}
		if s.ContShare != nil {
			k := s.Rules.Continent[t]
			if k >= 0 {
				w *= math.Exp(s.ContStrength * float64(s.ContShare[c*s.K+k]))
			} else {
				w *= s.ContNeutral
			}
		}
		if s.ClimShare != nil {
			f := 0.
			for z := 0; z < s.Z; z++ {
				f += float64(s.ClimShare[c*s.Z+z]) * s.Rules.TypeMult[z*s.T+t]
			}
			if s.ClimStrength == 1 {
				w *= f
			} else {
				w *= math.Pow(f, s.ClimStrength)
			}
		}
		if s.Prefer != nil && s.Prefer[c] >= 0 {
			steps := min(6, s.Rules.TypeDist[s.Prefer[c]*s.T+t])
			w *= math.Pow(1000, float64(2-steps))
		}
		if s.Stability > 0 && settled > 0 {
			w *= math.Exp(s.Stability * float64(counts[t]) / float64(settled))
		}
		out = append(out, Choice{t, w})
	}
	return out
}
func (s *Solver) Step() string {
	if s.Status != "running" {
		return s.Status
	}
	c := s.selectCell()
	if c < 0 {
		s.Status = "done"
		return s.Status
	}
	opts := s.OptionsFor(c)
	total := 0.
	for _, o := range opts {
		total += o.Weight
	}
	t := opts[len(opts)-1].Type
	if total <= 0 {
		t = opts[int(s.RNG.Next()*float64(len(opts)))].Type
	} else {
		r := s.RNG.Next() * total
		for _, o := range opts {
			r -= o.Weight
			if r < 0 {
				t = o.Type
				break
			}
		}
	}
	s.Steps++
	s.TrailCell = s.TrailCell[:0]
	s.TrailMask = s.TrailMask[:0]
	s.Recording = true
	s.set(c, bit(t))
	s.Locked[c] = 1
	s.enqueue(c)
	ok := s.propagate()
	s.Recording = false
	if !ok {
		s.Backtracks++
		for i := len(s.TrailCell) - 1; i >= 0; i-- {
			s.Write(s.TrailCell[i], s.TrailMask[i])
		}
		s.TrailCell = s.TrailCell[:0]
		s.TrailMask = s.TrailMask[:0]
		s.Locked[c] = 0
		s.set(c, s.Dom[c]&^bit(t))
		s.enqueue(c)
		if !s.propagate() {
			s.repair(s.Conflict)
		}
	}
	if s.Status == "running" && s.Open == 0 {
		s.Status = "done"
	}
	return s.Status
}
func (s *Solver) repair(center int) {
	cx0, cy0 := center%s.W, center/s.W
	near := s.LastRepair >= 0 && max(abs(cx0-s.LastRepair%s.W), abs(cy0-s.LastRepair/s.W)) <= 2*s.LastRadius
	r := 2
	if near {
		r = s.LastRadius * 2
	}
	for ; ; r *= 2 {
		s.Repairs++
		if s.Repairs > s.MaxRepairs {
			s.Status = "failed"
			s.Message = fmt.Sprintf("Gave up after %d repairs: the rules contradict each other too often. Fewest options first copes better with strict rules than random order.", s.MaxRepairs)
			return
		}
		if center < 0 || r >= max(s.W, s.H) {
			s.LastRepair = -1
			s.reset()
			return
		}
		cx, cy := center%s.W, center/s.W
		x0, y0, x1, y1 := max(0, cx-r), max(0, cy-r), min(s.W-1, cx+r), min(s.H-1, cy+r)
		stack := []int{}
		seen := make([]bool, s.N)
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				c := y*s.W + x
				if s.Pinned[c] == 0 {
					s.Locked[c] = 0
				}
				seen[c] = true
				stack = append(stack, c)
			}
		}
		s.LastRepair = center
		s.LastRadius = r
		for len(stack) > 0 {
			c := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			s.enqueue(c)
			if s.Locked[c] != 0 {
				continue
			}
			if s.Dom[c] != s.Full {
				s.Write(c, s.Full)
			}
			for _, d := range s.Offsets {
				n := s.neighbor(c, d)
				if n >= 0 && !seen[n] {
					seen[n] = true
					stack = append(stack, n)
				}
			}
		}
		if s.propagate() {
			return
		}
		center = s.Conflict
	}
}
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
func (s *Solver) Cleanup() int {
	if s.Status != "done" {
		return 0
	}
	order := make([]int, s.N)
	for i := range order {
		order[i] = i
	}
	for i := s.N - 1; i > 0; i-- {
		j := int(s.RNG.Next() * float64(i+1))
		order[i], order[j] = order[j], order[i]
	}
	changed := 0
	for _, c := range order {
		if s.Pinned[c] != 0 {
			continue
		}
		t := s.TypeAt(c)
		counts := make([]int, s.T)
		total := 0
		var around uint32
		for _, d := range s.Offsets {
			n := s.neighbor(c, d)
			if n < 0 {
				continue
			}
			m := s.Dom[n]
			around |= m
			counts[bits.Len32(m)-1]++
			total++
		}
		if total == 0 || counts[t]*4 > total {
			continue
		}
		best := -1
		for u := 0; u < s.T; u++ {
			if s.EnvironmentMasks != nil && s.EnvironmentMasks[c]&bit(u) == 0 {
				continue
			}
			if u == t || (best >= 0 && counts[u] <= counts[best]) {
				continue
			}
			if s.Rules.Allowed[u]&around == around {
				best = u
			}
		}
		if best >= 0 && counts[best]*2 >= total {
			s.Write(c, bit(best))
			changed++
		}
	}
	s.Cleaned += changed
	return changed
}
func (s *Solver) Snapshot() Snapshot {
	return Snapshot{append([]uint32{}, s.Dom...), append([]int{}, s.Locked...), append([]int{}, s.Pinned...), s.Status}
}
func (s *Solver) Restore(v Snapshot) {
	s.clearQueue()
	for c, m := range v.Dom {
		if s.Dom[c] != m {
			s.Write(c, m)
		}
	}
	copy(s.Locked, v.Locked)
	copy(s.Pinned, v.Pinned)
	s.Status = v.Status
}
func (s *Solver) Paint(cells []int, t int) bool {
	if s.Status != "done" || len(cells) == 0 {
		return false
	}
	if s.EnvironmentMasks != nil {
		for _, c := range cells {
			if s.EnvironmentMasks[c]&bit(t) == 0 {
				s.Message = "This terrain is incompatible with the environment here. Change elevation or climate first."
				return false
			}
		}
	}
	maxMargin := 4 * (2*int(math.Ceil(math.Sqrt(float64(s.Radius2)))) + 2)
	dist := make([]int, s.N)
	for i := range dist {
		dist[i] = -1
	}
	q := []int{}
	for _, c := range cells {
		if dist[c] < 0 {
			dist[c] = 0
			q = append(q, c)
		}
	}
	for qi := 0; qi < len(q); qi++ {
		c := q[qi]
		if dist[c] >= maxMargin+1 {
			continue
		}
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				n := s.neighbor(c, [2]int{dx, dy})
				if n >= 0 && dist[n] < 0 {
					dist[n] = dist[c] + 1
					q = append(q, n)
				}
			}
		}
	}
	if s.Prefer == nil {
		s.Prefer = make([]int, s.N)
		for i := range s.Prefer {
			s.Prefer[i] = -1
		}
	}
	old := false
	for _, c := range q {
		s.Prefer[c] = s.TypeAt(c)
		old = old || (dist[c] > 0 && s.Pinned[c] != 0)
	}
	snap := s.Snapshot()
	rounds := 1
	if old {
		rounds = 2
	}
	for free := 0; free < rounds; free++ {
		for margin := 0; margin <= maxMargin; {
			for _, c := range q {
				d := dist[c]
				if d > margin+1 {
					break
				}
				if d == 0 {
					s.Pinned[c] = 1
					s.Locked[c] = 1
					if s.Dom[c] != bit(t) {
						s.Write(c, bit(t))
					}
				} else if d <= margin && (s.Pinned[c] == 0 || free == 1) {
					s.Pinned[c] = 0
					s.Locked[c] = 0
					if s.Dom[c] != s.Full {
						s.Write(c, s.Full)
					}
				}
				s.enqueue(c)
			}
			if s.propagate() {
				s.Status = "done"
				if s.Open > 0 {
					s.Status = "running"
				}
				return true
			}
			s.Restore(snap)
			if margin < 4 {
				margin++
			} else {
				margin = int(math.Ceil(float64(margin) * 1.35))
			}
		}
	}
	s.Message = s.Rules.Config.Types[t].Name + " can't fit there: its neighbour rules clash with what's around it."
	return false
}
func (s *Solver) buildClimate(cl *ClimateOptions, seed uint32) {
	if cl == nil || cl.Strength <= 0 || s.Z == 0 {
		return
	}
	rng := RNG(seed ^ 0x2c1b3c6d)
	tau := math.Pi * 2
	s.ClimBoth = cl.Layout != "north"
	s.ClimStrength = cl.Strength
	for i := 0; i < 3; i++ {
		s.ClimWaves = append(s.ClimWaves, Wave{tau * math.Floor(1+rng.Next()*3) / float64(s.W), tau * (rng.Next() * 2) / float64(s.H), rng.Next() * tau})
	}
	s.ClimShare = make([]float32, s.N*s.Z)
	for c := 0; c < s.N; c++ {
		zones := s.zoneShares(float64(c%s.W)+.5, float64(c/s.W)+.5)
		for z, v := range zones {
			s.ClimShare[c*s.Z+z] = float32(v)
		}
	}
}
func (s *Solver) zoneShares(x, y float64) []float64 {
	v := y / float64(s.H)
	lat := 1 - v
	if s.ClimBoth {
		lat = math.Abs(2*v - 1)
	}
	a, b, c := s.ClimWaves[0], s.ClimWaves[1], s.ClimWaves[2]
	lat += .07 * (.5*math.Sin(x*a.FX+y*a.FY+a.Phase) + .3*math.Sin(x*b.FX*2+y*b.FY+b.Phase) + .2*math.Sin(x*c.FX*3+y*c.FY*2+c.Phase))
	lat = clamp(lat, 0, 1)
	width := .55 / float64(s.Z)
	out := make([]float64, s.Z)
	total := 0.
	for z := range out {
		d := (lat - (float64(z)+.5)/float64(s.Z)) / width
		out[z] = math.Exp(-d * d)
		total += out[z]
	}
	for z := range out {
		out[z] /= total
	}
	return out
}
func (s *Solver) buildContinents(cont *ContinentOptions, seed uint32) {
	s.ContPoints = []Point{}
	if cont == nil || cont.Count <= 0 || cont.Strength <= 0 || s.K == 0 {
		return
	}
	rng := RNG(seed ^ 0x5bd1e995)
	kinds := s.Rules.Config.Continents
	byOdds := make([]int, s.K)
	for k := range byOdds {
		byOdds[k] = k
	}
	sort.SliceStable(byOdds, func(a, b int) bool { return kinds[byOdds[b]].Odds < kinds[byOdds[a]].Odds })
	for i := 0; i < cont.Count; i++ {
		x, y := rng.Next()*float64(s.W), rng.Next()*float64(s.H)
		k := byOdds[min(i, s.K-1)]
		if i >= min(2, s.K) {
			var zones []float64
			if s.ClimShare != nil {
				zones = s.zoneShares(x, y)
			}
			odds := make([]float64, s.K)
			total := 0.
			for kk := 0; kk < s.K; kk++ {
				o := kinds[kk].Odds
				if zones != nil {
					f := 0.
					for z := 0; z < s.Z; z++ {
						f += zones[z] * s.Rules.KindMult[z*s.K+kk]
					}
					o *= math.Pow(f, s.ClimStrength)
				}
				odds[kk] = o
				total += o
			}
			r := rng.Next() * total
			k = s.K - 1
			if total > 0 {
				for kk, o := range odds {
					r -= o
					if r < 0 {
						k = kk
						break
					}
				}
			} else {
				k = int(rng.Next() * float64(s.K))
			}
		}
		s.ContPoints = append(s.ContPoints, Point{x, y, k, kinds[k]})
	}
	s.ContShare = make([]float32, s.N*s.K)
	for c := 0; c < s.N; c++ {
		x, y := float64(c%s.W)+.5, float64(c/s.W)+.5
		sums := make([]float64, s.K)
		total := 0.
		for _, p := range s.ContPoints {
			dx := p.X - x
			if s.WrapX {
				dx -= float64(s.W) * round(dx/float64(s.W))
			}
			dy := p.Y - y
			w := 1 / (dx*dx + dy*dy + 1)
			sums[p.K] += w
			total += w
		}
		for k := 0; k < s.K; k++ {
			s.ContShare[c*s.K+k] = float32(sums[k] / total)
		}
	}
	s.ContStrength = cont.Strength
	s.ContNeutral = math.Exp(cont.Strength / float64(s.K))
}
