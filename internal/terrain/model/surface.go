package model

import "math"

// Surface is the complete, finite terrain created before a v3 world is ready.
// Zoom only interpolates these persisted samples and saved channel geometry.
type Surface struct {
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Bed       []float32 `json:"-"`
	Coast     []float32 `json:"-"`
	Roughness []float32 `json:"-"`
	LakeLevel []float32 `json:"-"`
	Coasts    [][]Point `json:"coasts"`
}

func (s *Surface) At(a []float32, lon, lat float64) float64 {
	x := (Wrap(lon) + math.Pi) / Tau * float64(s.Width)
	y := Clamp((.5-lat/math.Pi)*float64(s.Height)-.5, 0, float64(s.Height-1))
	i, j := int(x)%s.Width, int(y)
	u, v := x-math.Floor(x), y-float64(j)
	i1, j1 := (i+1)%s.Width, min(j+1, s.Height-1)
	// The saved shoreline uses these same two triangles. A bilinear saddle
	// would curve between saved straight edges and drift at inspection zooms.
	if u+v <= 1 {
		return float64(a[j*s.Width+i])*(1-u-v) + float64(a[j*s.Width+i1])*u + float64(a[j1*s.Width+i])*v
	}
	return float64(a[j1*s.Width+i1])*(u+v-1) + float64(a[j1*s.Width+i])*(1-u) + float64(a[j*s.Width+i1])*(1-v)
}

type RiverStatistics struct {
	Count    int     `json:"count"`
	LengthKM float64 `json:"lengthKm"`
}
type HydrologyDiagnostics struct {
	Classes             map[string]RiverStatistics `json:"classes"`
	Basins              int                        `json:"basins"`
	BasinAreasKM2       []float64                  `json:"basinAreasKm2"`
	Lakes               int                        `json:"lakes"`
	LakeKinds           map[string]int             `json:"lakeKinds"`
	Wetlands            int                        `json:"wetlands"`
	Springs             int                        `json:"springs"`
	NetworkCoverage     float64                    `json:"networkCoverage"`
	TargetCoverage      float64                    `json:"targetCoverage"`
	InvalidTerminations int                        `json:"invalidTerminations"`
	LengthsKM           []float64                  `json:"lengthsKm"`
	CatchmentsKM2       []float64                  `json:"catchmentsKm2"`
	LowlandHeadwaters   int                        `json:"lowlandHeadwaters"`
	Distributaries      int                        `json:"distributaries"`
}
