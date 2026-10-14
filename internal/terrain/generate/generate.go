// Package generate orchestrates deterministic, cancellable generation stages.
package generate

import (
	"context"
	"fmt"
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
	"math"
	"sort"
	"time"
)

type Progress func(stage string, fraction float64)

func Generate(ctx context.Context, s model.Spec, progress Progress) (*model.Planet, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	p := &model.Planet{Schema: model.Schema, Spec: s, Grid: model.Grid{Width: 192, Height: 96, Fields: map[string][]float64{}}, Diagnostics: model.Diagnostics{Timings: map[string]float64{}, BiomeShares: map[string]float64{}, Warnings: []string{}}, Rivers: []model.River{}}
	stages := []struct {
		name string
		run  func() error
	}{
		{"continental layout", func() error { layout(p); return nil }},
		{"tectonic framework", func() error { plates(p); return nil }},
		{"relief and bathymetry", func() error { return surface(ctx, p) }},
		{"seasonal climate", func() error { return climate(ctx, p) }},
		{"complete terrain surface", func() error { return materialize(ctx, p) }},
		{"drainage and rivers", func() error { return drainage(ctx, p) }},
		{"saved shorelines", func() error { return shorelines(ctx, p) }},
		{"ecology and topology", func() error { audit(p); measure(p); return nil }},
	}
	for i, stage := range stages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if progress != nil {
			progress(stage.name, float64(i)/float64(len(stages)))
		}
		start := time.Now()
		if err := stage.run(); err != nil {
			return nil, err
		}
		p.Diagnostics.Timings[stage.name] = float64(time.Since(start).Microseconds()) / 1000
	}
	if progress != nil {
		progress("ready", 1)
	}
	return p, nil
}
func surface(ctx context.Context, p *model.Planet) error {
	g := &p.Grid
	n := g.Width * g.Height
	for _, key := range []string{"bed", "coast", "roughness", "temperature", "precipitation", "pet", "moisture", "flow", "discharge", "drainage", "lake", "windX", "windY"} {
		g.Fields[key] = make([]float64, n)
	}
	// Bounded sea adjustment reports the remaining coverage error; no quota growth.
	values := make([]float64, n)
	for i := range values {
		if i%g.Width == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		pt := g.Point(i%g.Width, i/g.Width)
		values[i] = landmass.Potential(p, pt[0], pt[1])
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	p.SeaOffset = model.Clamp(-sorted[int(float64(n)*(1-p.Spec.LandCoverage))], -.012, .012)
	for i := range values {
		if i%g.Width == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		pt := g.Point(i%g.Width, i/g.Width)
		b, c, r := landmass.Bed(p, pt[0], pt[1])
		g.Fields["bed"][i] = b
		g.Fields["coast"][i] = c * p.Spec.RadiusKM
		g.Fields["roughness"][i] = r
		g.Fields["flow"][i] = -1
	}
	return nil
}

func Biome(t, rain, pet, bed float64, wet bool) string {
	if wet {
		return "water"
	}
	if t < -8 {
		return "ice"
	}
	if t < 0 {
		return "tundra"
	}
	if bed > 3600 {
		return "alpine"
	}
	ratio := rain / math.Max(100, pet)
	if ratio < .22 {
		return "desert"
	}
	if ratio < .5 {
		return "steppe"
	}
	if ratio < .8 {
		return "grassland"
	}
	if t < 8 {
		return "taiga"
	}
	if t > 23 && rain > 1600 {
		return "rainforest"
	}
	if t > 20 && ratio < 1.1 {
		return "savanna"
	}
	return "forest"
}
func audit(p *model.Planet) {
	g := p.Grid
	seen := make([]bool, g.Width*g.Height)
	total, land := 0., 0.
	areas := []float64{}
	for i, b := range g.Fields["bed"] {
		pt := g.Point(i%g.Width, i/g.Width)
		weight := math.Cos(pt[1])
		total += weight
		if !p.Emerged(i) {
			continue
		}
		land += weight
		biome := BiomeAt(p, g.Point(i%g.Width, i/g.Width)[0], g.Point(i%g.Width, i/g.Width)[1], b, g.Fields["lake"][i] > 0 && g.Fields["lake"][i] > b)
		p.Diagnostics.BiomeShares[biome] += weight
		if seen[i] {
			continue
		}
		q := []int{i}
		seen[i] = true
		area := 0.
		for head := 0; head < len(q); head++ {
			j := q[head]
			pt := g.Point(j%g.Width, j/g.Width)
			area += math.Cos(pt[1])
			for _, k := range neighbors(j, g.Width, g.Height) {
				if !seen[k] && p.Emerged(k) {
					seen[k] = true
					q = append(q, k)
				}
			}
		}
		areas = append(areas, area)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(areas)))
	p.Diagnostics.LandCoverage = land / total
	for _, a := range areas {
		km2 := a / total * 4 * math.Pi * p.Spec.RadiusKM * p.Spec.RadiusKM
		if a/total > .015 {
			p.Diagnostics.MajorContinents++
		} else {
			p.Diagnostics.IslandAreas = append(p.Diagnostics.IslandAreas, km2)
		}
	}
	if len(areas) > 0 {
		p.Diagnostics.LargestShare = areas[0] / math.Max(land, 1)
	}
	for k, v := range p.Diagnostics.BiomeShares {
		p.Diagnostics.BiomeShares[k] = v / math.Max(land, 1)
	}
	if p.Diagnostics.MajorContinents < 3 && p.Spec.Preset != "supercontinent" {
		p.Diagnostics.Warnings = append(p.Diagnostics.Warnings, "Fewer than three major emerged continents; review silhouette.")
	}
	if math.Abs(p.Diagnostics.LandCoverage-p.Spec.LandCoverage) > .08 {
		p.Diagnostics.Warnings = append(p.Diagnostics.Warnings, fmt.Sprintf("Land coverage %.0f%% differs from preference %.0f%%; protected straits were retained.", p.Diagnostics.LandCoverage*100, p.Spec.LandCoverage*100))
	}
	if p.Diagnostics.BiomeShares["desert"]+p.Diagnostics.BiomeShares["steppe"] > .45 {
		p.Diagnostics.Warnings = append(p.Diagnostics.Warnings, "Drylands exceed 45% of land; inspect precipitation and PET.")
	}
}
func neighbors(i, w, h int) []int {
	x, y := i%w, i/w
	out := []int{y*w + (x+w-1)%w, y*w + (x+1)%w}
	if y > 0 {
		out = append(out, i-w)
	}
	if y < h-1 {
		out = append(out, i+w)
	}
	return out
}
