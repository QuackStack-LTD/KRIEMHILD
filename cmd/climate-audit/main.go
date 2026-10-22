// climate-audit records active-generator fields, classification and render costs.
// Run before and after a targeted change with identical seeds and configuration.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"kriemhild/internal/terrain/generate"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/terrain/tiles"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type distribution struct{ Mean, P10, Median, P90, Min, Max float64 }
type weighted struct{ v, w float64 }

func summarize(v []weighted) distribution {
	sort.Slice(v, func(i, j int) bool { return v[i].v < v[j].v })
	if len(v) == 0 {
		return distribution{}
	}
	sum, area := 0., 0.
	for _, x := range v {
		sum += x.v * x.w
		area += x.w
	}
	quantile := func(q float64) float64 {
		a := 0.
		for _, x := range v {
			a += x.w
			if a >= area*q {
				return x.v
			}
		}
		return v[len(v)-1].v
	}
	return distribution{sum / area, quantile(.1), quantile(.5), quantile(.9), v[0].v, v[len(v)-1].v}
}
func writeJSON(path string, value any) {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(path, b, 0644); err != nil {
		panic(err)
	}
}
func main() {
	out := flag.String("out", "docs/terrain-investigation/climate-baseline", "evidence directory")
	seeds := flag.String("seeds", "Kriemhild,gallery-001,gallery-002", "comma separated seeds")
	climateVersion := flag.Int("climate-version", model.ClimateVersion, "0 reproduces the frozen pre-repair climate")
	flag.Parse()
	if err := os.MkdirAll(*out, 0755); err != nil {
		panic(err)
	}
	for _, seed := range strings.Split(*seeds, ",") {
		spec := model.DefaultSpec()
		spec.ClimateVersion = *climateVersion
		spec.Seed = seed
		start := time.Now()
		p, err := generate.Generate(context.Background(), spec, nil)
		if err != nil {
			panic(err)
		}
		generationMS := float64(time.Since(start).Microseconds()) / 1000
		g := p.Grid
		n := g.Width * g.Height
		labels := make([]string, n)
		weights := make([]float64, n)
		land, globe, emerged, lake := 0., 0., 0., 0.
		shares := map[string]float64{}
		bands := map[string]map[string]float64{}
		bandArea := map[string]float64{}
		values := map[string][]weighted{}
		for i := 0; i < n; i++ {
			pt := g.Point(i%g.Width, i/g.Width)
			weight := math.Cos(pt[1])
			weights[i] = weight
			globe += weight
			if !p.Emerged(i) {
				continue
			}
			emerged += weight
			if g.Fields["lake"][i] > 0 && g.Fields["lake"][i] > g.Fields["bed"][i] {
				lake += weight
				continue
			}
			land += weight
			labels[i] = generate.BiomeAt(p, pt[0], pt[1], g.Fields["bed"][i], false)
			shares[labels[i]] += weight
			band := fmt.Sprintf("%02d-%02d", int(math.Abs(pt[1])*180/math.Pi)/15*15, int(math.Abs(pt[1])*180/math.Pi)/15*15+15)
			if bands[band] == nil {
				bands[band] = map[string]float64{}
			}
			bands[band][labels[i]] += weight
			bandArea[band] += weight
			for k, a := range g.Fields {
				if k == "bed" || k == "slope" || k == "aridity" || k == "seasonality" || k == "temperature" || k == "precipitation" || k == "pet" || k == "dryMonths" || k == "rainShadow" || k == "exposure" || strings.HasPrefix(k, "temperature-") || strings.HasPrefix(k, "rainfall-") || strings.HasPrefix(k, "pet-") {
					values[k] = append(values[k], weighted{a[i], weight})
				}
			}
			lo, hi := math.Inf(1), math.Inf(-1)
			for m := 0; m < 12; m++ {
				v := g.Fields[fmt.Sprintf("temperature-%02d", m)][i]
				lo = math.Min(lo, v)
				hi = math.Max(hi, v)
			}
			values["temperature-range"] = append(values["temperature-range"], weighted{hi - lo, weight})
			values["coldest-month"] = append(values["coldest-month"], weighted{lo, weight})
			values["warmest-month"] = append(values["warmest-month"], weighted{hi, weight})
		}
		for k := range shares {
			shares[k] /= land
		}
		for k, b := range bands {
			for biome := range b {
				b[biome] /= bandArea[k]
			}
		}
		fields := map[string]distribution{}
		for k, v := range values {
			fields[k] = summarize(v)
		}
		seen := make([]bool, n)
		components := map[string][]float64{}
		for i, label := range labels {
			if label == "" || seen[i] {
				continue
			}
			queue := []int{i}
			seen[i] = true
			area := 0.
			for len(queue) > 0 {
				at := queue[len(queue)-1]
				queue = queue[:len(queue)-1]
				area += weights[at]
				x, y := at%g.Width, at/g.Width
				for _, nb := range []int{y*g.Width + (x+1)%g.Width, y*g.Width + (x+g.Width-1)%g.Width, max(0, y-1)*g.Width + x, min(g.Height-1, y+1)*g.Width + x} {
					if !seen[nb] && labels[nb] == label {
						seen[nb] = true
						queue = append(queue, nb)
					}
				}
			}
			components[label] = append(components[label], area/land)
		}
		connectivity := map[string]any{}
		for biome, areas := range components {
			sort.Sort(sort.Reverse(sort.Float64Slice(areas)))
			connectivity[biome] = map[string]any{"regions": len(areas), "largestLandShare": areas[0], "largestBiomeFraction": areas[0] / shares[biome]}
		}
		var memory runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&memory)
		render := []map[string]any{}
		for _, style := range []string{"biome", "temperature", "precipitation", "aridity", "seasonality", "elevation"} {
			img := image.NewNRGBA(image.Rect(0, 0, 512, 256))
			for x := 0; x < 2; x++ {
				start := time.Now()
				b, err := tiles.Render(context.Background(), p, 0, x, 0, 256, style, nil)
				if err != nil {
					panic(err)
				}
				render = append(render, map[string]any{"style": style, "z": 0, "x": x, "ms": float64(time.Since(start).Microseconds()) / 1000, "bytes": len(b)})
				tile, err := png.Decode(bytes.NewReader(b))
				if err != nil {
					panic(err)
				}
				draw.Draw(img, image.Rect(x*256, 0, (x+1)*256, 256), tile, image.Point{}, draw.Src)
			}
			f, err := os.Create(filepath.Join(*out, seed+"-"+style+".png"))
			if err != nil {
				panic(err)
			}
			if err = png.Encode(f, img); err != nil {
				panic(err)
			}
			f.Close()
		}
		for repeat := 0; repeat < 3; repeat++ {
			start := time.Now()
			b, err := tiles.Render(context.Background(), p, 6, 54, 22, 256, "natural", nil)
			if err != nil {
				panic(err)
			}
			render = append(render, map[string]any{"style": "natural", "z": 6, "x": 54, "y": 22, "repeat": repeat, "ms": float64(time.Since(start).Microseconds()) / 1000, "bytes": len(b)})
		}
		writeJSON(filepath.Join(*out, seed+".json"), map[string]any{"spec": spec, "generationMs": generationMS, "liveHeapBytes": memory.HeapAlloc, "landFraction": emerged / globe, "dryLandFraction": land / globe, "lakeFraction": lake / globe, "oceanFraction": 1 - emerged/globe, "biomeLandShares": shares, "latitudeBiomes": bands, "fields": fields, "connectivity": connectivity, "render": render, "diagnostics": p.Diagnostics, "grid": []int{g.Width, g.Height}, "sampling": "cos(latitude)-weighted grid; biome/field/connectivity statistics exclude ocean and lake cells"})
		fmt.Printf("%s: %.1fs; land %.3f; forest %.3f; rainforest %.3f; temperate rainforest %.3f; steppe %.3f; desert %.3f\n", seed, generationMS/1000, land/globe, shares["forest"], shares["rainforest"], shares["temperate-rainforest"], shares["steppe"], shares["desert"])
	}
}
