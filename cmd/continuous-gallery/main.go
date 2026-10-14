// continuous-gallery records reproducible views and diagnostics without a server.
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
	"os"
	"path/filepath"
	"time"
)

func main() {
	count := flag.Int("count", 50, "number of seeds")
	out := flag.String("out", "docs/terrain-rebuild/gallery", "output directory")
	flag.Parse()
	if err := os.MkdirAll(*out, 0755); err != nil {
		panic(err)
	}
	summaries := []any{}
	html := "<!doctype html><meta charset=utf-8><title>Continuous terrain seed gallery</title><style>body{background:#101a23;color:#ddd;font:16px system-ui;margin:30px}img{width:512px;max-width:100%}article{border-top:1px solid #465;padding:20px 0}figure{display:inline-block;margin:8px}figcaption{font-size:13px}</style><h1>Continuous terrain · reproducible seed gallery</h1><p>Prototype diagnostics. Review warning fields and morphology, not just coverage.</p>"
	for i := 0; i < *count; i++ {
		spec := model.DefaultSpec()
		spec.Seed = fmt.Sprintf("gallery-%03d", i)
		if i == 0 {
			spec.Seed = "Kriemhild"
		}
		if i%10 == 7 {
			spec.Preset = "archipelago"
		}
		if i%10 == 8 {
			spec.Preset = "supercontinent"
		}
		if i%10 == 9 {
			spec.Preset = "arid"
			spec.Rainfall = .45
		}
		start := time.Now()
		p, err := generate.Generate(context.Background(), spec, nil)
		if err != nil {
			panic(err)
		}
		summary := map[string]any{"seed": spec.Seed, "preset": spec.Preset, "diagnostics": p.Diagnostics}
		summaries = append(summaries, summary)
		b, _ := json.MarshalIndent(summary, "", "  ")
		os.WriteFile(filepath.Join(*out, fmt.Sprintf("seed-%03d.json", i)), b, 0644)
		html += fmt.Sprintf("<article><h2>%s · %s</h2><p>Land %.1f%% · %d continents · largest share %.1f%%</p>", spec.Seed, spec.Preset, p.Diagnostics.LandCoverage*100, p.Diagnostics.MajorContinents, p.Diagnostics.LargestShare*100)
		for _, style := range []string{"natural", "mask", "elevation", "bathymetry", "precipitation", "biome"} {
			img := image.NewNRGBA(image.Rect(0, 0, 512, 256))
			for x := 0; x < 2; x++ {
				data, err := tiles.Render(context.Background(), p, 0, x, 0, 256, style, nil)
				if err != nil {
					panic(err)
				}
				tile, err := png.Decode(bytes.NewReader(data))
				if err != nil {
					panic(err)
				}
				draw.Draw(img, image.Rect(x*256, 0, (x+1)*256, 256), tile, image.Point{}, draw.Src)
			}
			name := fmt.Sprintf("seed-%03d-%s.png", i, style)
			f, err := os.Create(filepath.Join(*out, name))
			if err != nil {
				panic(err)
			}
			if err = png.Encode(f, img); err != nil {
				panic(err)
			}
			f.Close()
			html += fmt.Sprintf("<figure><img loading=lazy src='%s'><figcaption>%s</figcaption></figure>", name, style)
		}
		html += "</article>"
		fmt.Printf("%s: %.1f%% land, %d continents, %s\n", spec.Seed, p.Diagnostics.LandCoverage*100, p.Diagnostics.MajorContinents, time.Since(start).Round(time.Millisecond))
		os.WriteFile(filepath.Join(*out, "index.html"), []byte(html), 0644)
	}
	b, _ := json.MarshalIndent(summaries, "", "  ")
	os.WriteFile(filepath.Join(*out, "metrics.json"), b, 0644)
}
