package terrain

import (
	"fmt"
	"math"
)

// Foundation count is a preference: collisions merge and rifts split masses.
// Provenance only; opening an old saved world never reconstructs its mask.
type ContinentalLayout struct {
	Version               string  `json:"version,omitempty"`
	Family                string  `json:"family,omitempty"`
	Count                 int     `json:"count"`
	GapCells              float64 `json:"gapCells"`
	SeaThreshold          float64 `json:"seaThreshold,omitempty"`
	RequestedLandPercent  float64 `json:"requestedLandPercent"`
	FoundationLandPercent float64 `json:"foundationLandPercent"`
	Warning               string  `json:"warning,omitempty"`
}

func (o EnvironmentOptions) oceanGap() float64 {
	f := .08
	if o.OceanSeparation == "narrow" {
		f = .05
	}
	if o.OceanSeparation == "wide" {
		f = .12
	}
	return math.Max(1, f*float64(min(o.Columns, o.Rows)))
}
func (e *Environment) layoutFamily() string {
	// Avalanche numeric seeds before taking a family: the first LCG draw of
	// adjacent seed integers otherwise selected the same family repeatedly.
	return []string{"clustered", "dominant-and-fragments", "hemispheric", "elongated-belt", "scattered-remnants"}[int(detailHash(Seed(e.Options.Seed), 53, 67, 129)*5)]
}
func (e *Environment) layoutCenters() []Point {
	w, h := float64(e.Options.Columns), float64(e.Options.Rows)
	rng := RNG(uint32(detailHash(Seed(e.Options.Seed), 31, 79, 217) * 4294967295))
	count := e.Options.MajorContinents
	if count == 0 {
		count = 3 + int(rng.Next()*6)
	}
	family := e.layoutFamily()
	anchors := [][2]float64{{.22 + rng.Next()*.20, .20 + rng.Next()*.60}, {.58 + rng.Next()*.20, .20 + rng.Next()*.60}}
	angle := (rng.Next() - .5) * math.Pi
	separation := e.Options.oceanGap() / float64(min(e.Options.Columns, e.Options.Rows))
	centers := make([]Point, 0, count)
	weights := e.foundationWeights(count)
	total := 0.
	for _, v := range weights {
		total += v
	}
	radii := make([]float64, count)
	for k, v := range weights {
		radii[k] = math.Sqrt(w * h * e.Options.LandPercent / 100 * v / total / math.Pi)
	}
	for k := 0; k < count; k++ {
		x, y := .5, .5
		for trial := 0; trial < 96; trial++ {
			x, y = .10+rng.Next()*.80, .12+rng.Next()*.76
			switch family {
			case "clustered":
				a := anchors[k%2]
				x = a[0] + (rng.Next()-.5)*.48
				y = a[1] + (rng.Next()-.5)*.55
			case "dominant-and-fragments":
				if k == 0 {
					x, y = .3+rng.Next()*.16, .35+rng.Next()*.3
				} else {
					x = .12 + rng.Next()*.76
				}
			case "hemispheric":
				x = .22 + float64(k%2)*.56 + (rng.Next()-.5)*.24
				y = .22 + float64(k%2)*.48 + (rng.Next()-.5)*.38
			case "elongated-belt":
				u := (float64(k)+rng.Next())/float64(count) - .5
				x = .5 + math.Cos(angle*.4)*u*.9 + (rng.Next()-.5)*.16
				y = .5 + math.Sin(angle*.4)*u*.65 + (rng.Next()-.5)*.36
			}
			spread := 1 + (separation-.08)*3
			x, y = .5+(x-.5)*spread, .5+(y-.5)*spread
			x, y = clamp(x, .12, .88), clamp(y, .13, .87)
			weight := 1.
			for j, p := range centers {
				d := math.Hypot(x*w-p.X, y*h-p.Y)
				exclusion := radii[k] + radii[j] + e.Options.oceanGap()*.35
				collision := family == "clustered" && k%2 == j%2 || family == "hemispheric" && k%2 == j%2 || family == "elongated-belt" && k%3 == j%3
				strength := 9.
				if collision {
					strength = 1.2
				}
				weight *= math.Exp(-strength * math.Max(0, 1-d/exclusion))
			}
			// Rejection sampling from the weighted layout family; the first
			// accepted proposal wins. No farthest-point optimization or partitions.
			if rng.Next() < weight {
				break
			}
		}
		centers = append(centers, Point{X: x * w, Y: y * h, K: 1, Kind: Kind{ID: "land", Name: "Continental foundation", Color: "#9ebd79"}})
	}
	return centers
}
func (e *Environment) foundationWeights(count int) []float64 {
	values := make([]float64, count)
	for k := range values {
		values[k] = math.Exp((detailHash(Seed(e.Options.Seed), k, 11, 71) - .5) * 2.1)
		if e.layoutFamily() == "dominant-and-fragments" {
			values[k] *= .45
			if k == 0 {
				values[k] = 5.5
			}
		}
	}
	return values
}
func (e *Environment) layoutNuclei() []LandmassStructure {
	e.Points = e.layoutCenters()
	total := 0.
	weights := e.foundationWeights(len(e.Points))
	for _, v := range weights {
		total += v
	}
	shapes := make([]LandmassStructure, 0, len(e.Points))
	for k, p := range e.Points {
		rng := RNG(Seed(e.Options.Seed) ^ uint32(k+1)*0x471cab31)
		names := []string{"compact", "elongated", "indented", "curved", "isthmus", "hook", "branched", "crescent", "rifted"}
		name := names[int(rng.Next()*float64(len(names)))]
		a, b, _, _, convergence, bearing := e.platesAt(p.X, p.Y)
		radius := math.Sqrt(float64(len(e.Mask)) * e.Options.LandPercent / 100 * weights[k] / total / math.Pi)
		radius *= 1.04 - (e.Options.oceanGap()/float64(min(e.Options.Columns, e.Options.Rows))-.08)*1.2
		s := makeLandmass(Seed(e.Options.Seed)^uint32(k+1)*0x738a913, name, [2]float64{p.X, p.Y}, radius, bearing+(rng.Next()-.5)*math.Pi*2, e.Options.shapeOptions())
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

// Diagnostic owner never restricts flooding, cell ownership or growth.
func (e *Environment) continentalUnion(x, y float64) (float64, int) {
	value, owner := math.Inf(-1), 0
	void := math.Inf(-1)
	for k, s := range e.ContinentalStructures {
		sx := x
		if e.Geography != nil && e.Geography.WrapX {
			sx = s.Center[0] + math.Remainder(x-s.Center[0], float64(e.Options.Columns))
		}
		land, cut := s.fieldParts(sx, y)
		v := land * s.Scale
		void = math.Max(void, cut*s.Scale)
		if v > value {
			value, owner = v, k
		}
	}
	// Subtract flooded structures after union. Otherwise an overlapping lobe
	// from the next foundation silently fills a deep gulf or tectonic rift.
	return math.Min(value, -void*8), owner
}
func (e *Environment) separatedContinentalMask(sample func(float64, float64) float64) {
	w, h := e.Options.Columns, e.Options.Rows
	n := w * h
	small := float64(min(w, h))
	e.ContinentalStructures = e.layoutNuclei()
	e.Layout = &ContinentalLayout{Version: "filled-union-2", Family: e.layoutFamily(), Count: len(e.ContinentalStructures), GapCells: e.Options.oceanGap(), RequestedLandPercent: e.Options.LandPercent}
	scores, reserve, owners := make([]float64, n), make([]float64, n), make([]float64, n)
	evaluate := func() int {
		positive := 0
		for i := range scores {
			x, y := float64(i%w), float64(i/w)
			v, owner := e.continentalUnion(x, y)
			edge := math.Min(math.Min(x, float64(w-1)-x), math.Min(y, float64(h-1)-y))
			// Smooth offshore taper precedes a small safety frame. No allocation
			// grows toward that frame, and no polygon seam determines the coastline.
			v -= small * (.6 + .4*sample(x/small*7+317, y/small*9+193)) * math.Exp(-edge/(small*.065))
			scores[i], owners[i] = v, float64(owner+1)
			frame := 1.25 + sample(x/small*11+37, y/small*13+91)*.8
			reserve[i] = 0
			if edge <= frame {
				reserve[i] = 1
			}
			if v >= 0 && reserve[i] == 0 {
				positive++
			}
		}
		return positive
	}
	present := evaluate()
	target := float64(n) * e.Options.LandPercent / 100
	// One bounded scale correction, not recursive repacking or per-owner quotas.
	correction := clamp(math.Sqrt(target/math.Max(1, float64(present))), .78, 1.50)
	if math.Abs(correction-1) > .025 {
		for k := range e.ContinentalStructures {
			e.ContinentalStructures[k].rescale(correction)
		}
		evaluate()
	}
	lo, hi := -small*.035, small*.035
	for step := 0; step < 24; step++ {
		threshold := (lo + hi) / 2
		area := 0
		for i, v := range scores {
			if reserve[i] == 0 && v >= threshold {
				area++
			}
		}
		if float64(area) > target {
			lo = threshold
		} else {
			hi = threshold
		}
	}
	threshold := (lo + hi) / 2
	used := 0
	for i, v := range scores {
		e.Mask[i] = 0
		if reserve[i] == 0 && v >= threshold {
			e.Mask[i] = 1
			used++
		}
	}
	e.Fields["continentalShape"], e.Fields["continentalOwner"], e.Fields["oceanReserve"] = scores, owners, reserve
	e.Layout.SeaThreshold = threshold
	e.Layout.FoundationLandPercent = float64(used) * 100 / float64(n)
	if math.Abs(e.Layout.FoundationLandPercent-e.Options.LandPercent) > 1 {
		e.Layout.Warning = fmt.Sprintf("Coast shape and ocean framing take priority: requested %.1f%% land, achieved %.1f%% continental foundations. Count and separation are layout preferences; collisions and rifts can merge or divide masses.", e.Options.LandPercent, e.Layout.FoundationLandPercent)
	}
}
