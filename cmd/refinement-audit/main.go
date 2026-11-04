// Experimental only: compare existing procedural residuals with a cached local
// field. Neither approach is connected to the production terrain sampler.
package main

import (
	"encoding/json"
	"flag"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
	"math"
	"os"
	"path/filepath"
	"time"
)

func main() {
	source := flag.String("load", ".cache/deepzoom-planet", "extracted planet")
	out := flag.String("out", "docs/deepzoom-investigation/refinement.json", "result")
	flag.Parse()
	must := func(e error) {
		if e != nil {
			panic(e)
		}
	}
	meta, e := os.ReadFile(filepath.Join(*source, "metadata"))
	must(e)
	fields, e := os.ReadFile(filepath.Join(*source, "fields"))
	must(e)
	p, e := model.Decode(meta, fields)
	must(e)
	field := landmass.NewField(p)
	sampler := detail.New(p)
	s := p.Surface
	coarse := map[[2]int]float64{}
	vertex := func(x, y int) float64 {
		x = (x%s.Width + s.Width) % s.Width
		y = max(0, min(s.Height-1, y))
		key := [2]int{x, y}
		if v, ok := coarse[key]; ok {
			return v
		}
		lon, lat := -math.Pi+model.Tau*float64(x)/float64(s.Width), math.Pi/2-math.Pi*(float64(y)+.5)/float64(s.Height)
		b, c, _ := field.Bed(lon, lat)
		b, _ = landmass.Conditioned(p, lon, lat, b, c)
		coarse[key] = b
		return b
	}
	residual := func(lon, lat float64) float64 {
		sample := sampler.Sample(lon, lat)
		if sample.Water != "dry" || sample.CoastDistance < .5 {
			return 0
		}
		bed := s.At(s.Bed, lon, lat)
		// Protect incised channels without inventing any new water or biome geometry.
		riverWeight := model.Clamp(1-math.Abs(sample.Bed-bed)/2, 0, 1)
		x := (model.Wrap(lon) + math.Pi) / model.Tau * float64(s.Width)
		y := (.5-lat/math.Pi)*float64(s.Height) - .5
		i, j := int(math.Floor(x)), int(math.Floor(y))
		u, v := x-math.Floor(x), y-math.Floor(y)
		base := 0.
		if u+v <= 1 {
			base = vertex(i, j)*(1-u-v) + vertex(i+1, j)*u + vertex(i, j+1)*v
		} else {
			base = vertex(i+1, j+1)*(u+v-1) + vertex(i, j+1)*(1-u) + vertex(i+1, j)*(1-v)
		}
		b, c, _ := field.Bed(lon, lat)
		b, _ = landmass.Conditioned(p, lon, lat, b, c)
		return 5 * math.Tanh((b-base)/5) * riverWeight
	}
	const n = 128
	lon0, lat0, span := .3*math.Pi-.0001, .28*math.Pi-.0001, .0002
	a := make([]float64, (n+1)*(n+1))
	start := time.Now()
	energy, maxValue := 0., 0.
	for y := 0; y <= n; y++ {
		for x := 0; x <= n; x++ {
			v := residual(lon0+span*float64(x)/n, lat0+span*float64(y)/n)
			a[y*(n+1)+x] = v
			energy += v * v
			maxValue = math.Max(maxValue, math.Abs(v))
		}
	}
	generationMS := float64(time.Since(start).Microseconds()) / 1000
	sampleCached := func(x, y float64) float64 {
		i, j := min(n-1, int(x)), min(n-1, int(y))
		u, v := x-float64(i), y-float64(j)
		return (a[j*(n+1)+i]*(1-u)+a[j*(n+1)+i+1]*u)*(1-v) + (a[(j+1)*(n+1)+i]*(1-u)+a[(j+1)*(n+1)+i+1]*u)*v
	}
	start = time.Now()
	sum := 0.
	for repeat := 0; repeat < 3; repeat++ {
		for y := 0; y < n; y++ {
			for x := 0; x < n; x++ {
				sum += sampleCached(float64(x)+.5, float64(y)+.5)
			}
		}
	}
	cachedMS := float64(time.Since(start).Microseconds()) / 3000
	start = time.Now()
	maxError, seamError := 0., 0.
	for y := 0; y < n; y += 4 {
		for x := 0; x < n; x += 4 {
			lon, lat := lon0+span*(float64(x)+.5)/n, lat0+span*(float64(y)+.5)/n
			v := residual(lon, lat)
			maxError = math.Max(maxError, math.Abs(v-sampleCached(float64(x)+.5, float64(y)+.5)))
			seamError = math.Max(seamError, math.Abs(v-residual(lon, lat)))
		}
	}
	directMS := float64(time.Since(start).Microseconds()) / 1000
	result := map[string]any{"experimentalOnly": true, "seed": p.Spec.Seed, "boundsRadians": []float64{lon0, lat0, span, span}, "points": (n + 1) * (n + 1), "continuousBuildMS": generationMS, "cachedRepeatMS": cachedMS, "cachedBytes": len(a) * 8, "residualRMSMetres": math.Sqrt(energy / float64(len(a))), "maxResidualMetres": maxValue, "cachedIntermediateMaxErrorMetres": maxError, "repeatedAbsoluteSampleError": seamError, "direct1024TwiceMS": directMS, "checksum": sum, "decision": "retain as isolated experiment; bounded roughness is not validated geomorphology, ecology or authored refinement"}
	b, e := json.MarshalIndent(result, "", "  ")
	must(e)
	must(os.WriteFile(*out, b, 0644))
}
