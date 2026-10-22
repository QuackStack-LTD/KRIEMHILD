// Read-only experiments against a decoded saved planet. No generation or writes
// to the source archive. Output includes fixed-position and source-density data.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/terrain/tiles"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"
)

func main() {
	source := flag.String("load", ".cache/deepzoom-planet", "decoded archive directory")
	out := flag.String("out", "docs/deepzoom-investigation/sampling-before", "results")
	flag.Parse()
	must := func(err error) {
		if err != nil {
			panic(err)
		}
	}
	meta, err := os.ReadFile(filepath.Join(*source, "metadata"))
	must(err)
	fields, err := os.ReadFile(filepath.Join(*source, "fields"))
	must(err)
	p, err := model.Decode(meta, fields)
	must(err)
	must(os.MkdirAll(*out, 0755))
	f, err := os.Create(filepath.Join(*out, "cpu.pprof"))
	must(err)
	must(pprof.StartCPUProfile(f))
	lon, lat := .3*math.Pi, .28*math.Pi
	sampler := detail.New(p)
	samples := []any{}
	for _, density := range []int{256, 512, 1024, 2048} {
		start := time.Now()
		var value model.Sample
		for i := 0; i < density; i++ {
			value = sampler.Sample(lon+(float64(i)/float64(density)-.5)*.0001, lat)
		}
		samples = append(samples, map[string]any{"density": density, "sampleSpacingRadians": .0001 / float64(density), "ms": float64(time.Since(start).Microseconds()) / 1000, "fixed": sampler.Sample(lon, lat), "last": value})
	}
	renders := []any{}
	for repeat := 0; repeat < 3; repeat++ {
		for _, z := range []int{1, 4, 7, 10, 13, 16} {
			x, y := int(1.3*math.Exp2(float64(z))), int(.22*math.Exp2(float64(z)))
			for _, style := range []string{"natural", "hillshade", "biome"} {
				start := time.Now()
				b, err := tiles.Render(context.Background(), p, z, x, y, 256, style, nil)
				must(err)
				renders = append(renders, map[string]any{"repeat": repeat, "z": z, "x": x, "y": y, "style": style, "ms": float64(time.Since(start).Microseconds()) / 1000, "bytes": len(b)})
				if repeat == 0 {
					must(os.WriteFile(filepath.Join(*out, fmt.Sprintf("z%d-%s.png", z, style)), b, 0644))
				}
			}
		}
	}
	pprof.StopCPUProfile()
	must(f.Close())
	styleOrder := []any{}
	hashes := map[string][32]byte{}
	for repeat := 0; repeat < 3; repeat++ {
		for _, order := range []string{"grouped", "adjacent"} {
			start := time.Now()
			equal := true
			render := func(node int, style string) {
				x, y := 8+node%4, 1+node/4
				b, e := tiles.Render(context.Background(), p, 3, x, y, 256, style, nil)
				must(e)
				key := fmt.Sprintf("%d/%s", node, style)
				sum := sha256.Sum256(b)
				if old, ok := hashes[key]; ok && old != sum {
					equal = false
				}
				hashes[key] = sum
			}
			if order == "grouped" {
				for _, style := range []string{"natural", "hillshade"} {
					for node := 0; node < 16; node++ {
						render(node, style)
					}
				}
			} else {
				for node := 0; node < 16; node++ {
					for _, style := range []string{"natural", "hillshade"} {
						render(node, style)
					}
				}
			}
			styleOrder = append(styleOrder, map[string]any{"repeat": repeat, "order": order, "ms": float64(time.Since(start).Microseconds()) / 1000, "identicalPNGs": equal, "nodes": 16, "size": 256})
		}
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	result := map[string]any{"styleOrder": styleOrder, "spec": p.Spec, "source": []int{p.Surface.Width, p.Surface.Height}, "climateGrid": []int{p.Grid.Width, p.Grid.Height}, "sourceSpacingEquatorialKM": model.Tau * p.Spec.RadiusKM / float64(p.Surface.Width), "samples": samples, "renders": renders, "heapAlloc": memory.HeapAlloc, "heapSys": memory.HeapSys}
	b, err := json.MarshalIndent(result, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(*out, "results.json"), b, 0644))
	fmt.Println(string(b))
}
