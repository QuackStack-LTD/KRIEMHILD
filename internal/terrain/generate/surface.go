package generate

import (
	"context"
	"fmt"
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
	"math"
	"runtime"
	"sync"
)

// Materialize the whole globe, including unvisited regions, before drainage and
// publication. No renderer calls the procedural field for these worlds.
func materialize(ctx context.Context, p *model.Planet) error {
	if p.Spec.Version != model.Version {
		return nil
	}
	w := p.Spec.SurfaceWidth
	if w == 0 {
		w = 4096
	}
	s := &model.Surface{Width: w, Height: w / 2}
	n := s.Width * s.Height
	s.Bed, s.Coast, s.Roughness = make([]float32, n), make([]float32, n), make([]float32, n)
	s.LakeLevel = make([]float32, n)
	field := landmass.NewField(p)
	var workers sync.WaitGroup
	count := min(8, runtime.GOMAXPROCS(0))
	for worker := 0; worker < count; worker++ {
		workers.Add(1)
		go func(start int) {
			defer workers.Done()
			for y := start; y < s.Height; y += count {
				if ctx.Err() != nil {
					return
				}
				lat := math.Pi/2 - math.Pi*(float64(y)+.5)/float64(s.Height)
				for x := 0; x < s.Width; x++ {
					lon := -math.Pi + model.Tau*float64(x)/float64(s.Width)
					bed, coast, rough := field.Bed(lon, lat)
					bed, coast = landmass.Conditioned(p, lon, lat, bed, coast)
					if coast <= 0 {
						bed = math.Min(0, bed)
					}
					i := y*s.Width + x
					s.Bed[i], s.Coast[i], s.Roughness[i] = float32(bed), float32(coast*p.Spec.RadiusKM), float32(rough)
				}
			}
		}(worker)
	}
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	p.Surface = s
	// Drainage has its own finer grid. Climate is conservative at its original
	// resolution; resampling does not run weather or add terrain during zoom.
	old := p.Grid
	var sea model.Grid
	if p.Spec.ClimateVersion >= 2 {
		sea = seaLevelClimate(old)
	}
	g := model.Grid{Width: 512, Height: 256, Fields: map[string][]float64{}}
	for key := range old.Fields {
		g.Fields[key] = make([]float64, g.Width*g.Height)
	}
	for i := range g.Fields["bed"] {
		pt := g.Point(i%g.Width, i/g.Width)
		for key, a := range g.Fields {
			a[i] = old.At(key, pt[0], pt[1])
		}
		g.Fields["bed"][i] = s.At(s.Bed, pt[0], pt[1])
		delta := (g.Fields["bed"][i] - old.At("bed", pt[0], pt[1])) * .0065
		g.Fields["temperature"][i] -= delta
		for month := 0; month < 12; month++ {
			g.Fields[fmt.Sprintf("temperature-%02d", month)][i] -= delta
		}
		if p.Spec.ClimateVersion >= 2 {
			lapse := math.Max(0, g.Fields["bed"][i]) * .0065
			g.Fields["temperature"][i] = sea.At("temperature", pt[0], pt[1]) - lapse
			for _, key := range temperatureMonths {
				g.Fields[key][i] = sea.At(key, pt[0], pt[1]) - lapse
			}
		}
		g.Fields["coast"][i] = s.At(s.Coast, pt[0], pt[1])
		g.Fields["roughness"][i] = s.At(s.Roughness, pt[0], pt[1])
		g.Fields["flow"][i] = -1
	}
	p.Grid = g
	if p.Spec.ClimateVersion >= 2 {
		materializedMoisture(&p.Grid, p.Spec)
	}
	if p.Spec.ClimateVersion >= 1 {
		summarizeSeasons(&p.Grid)
	}
	return nil
}

// Marching triangles uses the same linear coast field as the sampler. Edges
// join by lattice IDs, including the longitude seam; no viewport-specific coast.
func shorelines(ctx context.Context, p *model.Planet) error {
	if p.Surface == nil {
		return nil
	}
	s := p.Surface
	// Resolve lake footprints against the saved relief now, not during viewing.
	// The water plane is also stored on the dry shore so interpolation intersects
	// the bed at the same height instead of inventing a new zoom-dependent lake.
	for _, lake := range p.Lakes {
		if len(lake.Path) > 0 {
			continue
		}
		cy := (.5-lake.Center[1]/math.Pi)*float64(s.Height) - .5
		ry := lake.Radius / math.Pi * float64(s.Height)
		cx := (lake.Center[0] + math.Pi) / model.Tau * float64(s.Width)
		rx := ry / math.Max(.05, math.Cos(lake.Center[1]))
		for y := max(0, int(cy-ry)-1); y <= min(s.Height-1, int(cy+ry)+1); y++ {
			for xx := int(cx-rx) - 1; xx <= int(cx+rx)+1; xx++ {
				x := (xx%s.Width + s.Width) % s.Width
				i := y*s.Width + x
				lon := -math.Pi + model.Tau*float64(x)/float64(s.Width)
				lat := math.Pi/2 - math.Pi*(float64(y)+.5)/float64(s.Height)
				if s.Coast[i] > 0 && math.Hypot(model.Wrap(lon-lake.Center[0])*math.Cos(lat), lat-lake.Center[1]) < lake.Radius {
					s.LakeLevel[i] = max(s.LakeLevel[i], float32(lake.Level))
				}
			}
		}
	}
	type edge struct{ a, b int }
	points := map[edge]model.Point{}
	links := map[edge][]edge{}
	sequence := []edge{}
	grid := model.Grid{Width: s.Width, Height: s.Height}
	for y := 0; y < s.Height-1; y++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		for x := 0; x < s.Width; x++ {
			a, b := y*s.Width+x, y*s.Width+(x+1)%s.Width
			for _, ids := range [][3]int{{a, b, a + s.Width}, {b, b + s.Width, a + s.Width}} {
				cross := []edge{}
				for k := 0; k < 3; k++ {
					u, v := ids[k], ids[(k+1)%3]
					cu, cv := float64(s.Coast[u]), float64(s.Coast[v])
					if cu > 0 && s.LakeLevel[u] > 0 {
						cu = float64(s.Bed[u] - s.LakeLevel[u])
					}
					if cv > 0 && s.LakeLevel[v] > 0 {
						cv = float64(s.Bed[v] - s.LakeLevel[v])
					}
					if (cu > 0) == (cv > 0) {
						continue
					}
					if s.Coast[u] > 0 && s.Coast[v] > 0 && (s.LakeLevel[u] > 0 || s.LakeLevel[v] > 0) {
						cu, cv = float64(s.Bed[u]-s.LakeLevel[u]), float64(s.Bed[v]-s.LakeLevel[v])
						if s.LakeLevel[u] == 0 {
							cu = math.Max(.001, cu)
						}
						if s.LakeLevel[v] == 0 {
							cv = math.Max(.001, cv)
						}
					}
					e := edge{min(u, v), max(u, v)}
					if _, ok := points[e]; !ok {
						pu, pv := grid.Point(u%s.Width, u/s.Width), grid.Point(v%s.Width, v/s.Width)
						t := cu / (cu - cv)
						points[e] = model.Point{model.Wrap(pu[0] + model.Wrap(pv[0]-pu[0])*t), pu[1] + (pv[1]-pu[1])*t}
						sequence = append(sequence, e)
					}
					cross = append(cross, e)
				}
				for k := 0; k+1 < len(cross); k += 2 {
					u, v := cross[k], cross[k+1]
					links[u] = append(links[u], v)
					links[v] = append(links[v], u)
				}
			}
		}
	}
	seen := map[edge]bool{}
	for _, start := range sequence {
		if seen[start] {
			continue
		}
		path := []model.Point{}
		current := start
		for !seen[current] {
			seen[current] = true
			path = append(path, points[current])
			next := start
			for _, e := range links[current] {
				if !seen[e] {
					next = e
					break
				}
			}
			current = next
		}
		if len(path) > 2 {
			path = append(path, points[current])
			s.Coasts = append(s.Coasts, path)
		}
	}
	for _, lake := range p.Lakes {
		if len(lake.Path) < 2 {
			continue
		}
		left, right := []model.Point{}, []model.Point{}
		radius := lake.Width * .5 / (p.Spec.RadiusKM * 1000)
		for i, pt := range lake.Path {
			a, b := lake.Path[max(0, i-1)], lake.Path[min(len(lake.Path)-1, i+1)]
			c := math.Max(.05, math.Cos(pt[1]))
			dx, dy := model.Wrap(b[0]-a[0])*c, b[1]-a[1]
			d := math.Max(1e-12, math.Hypot(dx, dy))
			nx, ny := -dy/d*radius/c, dx/d*radius
			left = append(left, model.Point{model.Wrap(pt[0] + nx), pt[1] + ny})
			right = append(right, model.Point{model.Wrap(pt[0] - nx), pt[1] - ny})
		}
		for i := len(right) - 1; i >= 0; i-- {
			left = append(left, right[i])
		}
		left = append(left, left[0])
		s.Coasts = append(s.Coasts, left)
	}
	return nil
}
