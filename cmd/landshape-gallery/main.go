// Generate a reproducible visual/metric audit from the full terrain pipeline.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/png"
	"kriemhild/internal/terrain"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	out := flag.String("out", "docs/landshape-gallery", "output directory")
	seeds := flag.String("seeds", "KRIEMHILD,1,2,3,4,5,6,7,8", "comma-separated generation seeds")
	width := flag.Int("width", 160, "simulation columns")
	height := flag.Int("height", 100, "simulation rows")
	flag.Parse()
	if err := os.MkdirAll(*out, 0755); err != nil {
		panic(err)
	}
	names := strings.Split(*seeds, ",")
	reports := map[string]terrain.LandShapeReport{}
	const scale = 3
	cols := 3
	rows := (len(names) + cols - 1) / cols
	gallery := image.NewRGBA(image.Rect(0, 0, cols*(*width)*scale, rows*(*height)*scale))
	var page strings.Builder
	page.WriteString(`<!doctype html><meta charset="utf-8"><title>KRIEMHILD organic landforms</title><style>body{font:16px system-ui;background:#17262d;color:#f1eedf;margin:32px}h1{font-family:Georgia}section{display:grid;grid-template-columns:repeat(3,minmax(260px,1fr));gap:24px}img{width:100%;image-rendering:pixelated}figure{margin:0}figcaption{padding:10px 0;line-height:1.5}a{color:#a8d9e5}</style><h1>Organic continental and island silhouettes</h1><p>Identical settings; different seeds. Full terrain, archipelago, erosion and hydrology pipeline. Green: land. Blue: ocean-connected water. Pale blue: inland water. One cell represents the same geographic scale in every panel. No smoothing or decorative coast noise is added by this gallery.</p><p><a href="metrics.json">Download shape metrics</a> | <a href="gallery.png">Combined PNG</a></p><section>`)
	for k, seed := range names {
		raw, _ := json.Marshal(map[string]any{"columns": *width, "rows": *height, "seed": seed, "realism": true, "landPercent": 42, "landShape": terrain.LandShapeOptions{Large: 1, Medium: 1, Small: 1}})
		o, err := terrain.DecodeEnvironment(raw)
		if err != nil {
			panic(err)
		}
		e := terrain.BuildEnvironment(o)
		if err = e.ValidateLandStructures(); err != nil {
			panic(err)
		}
		if err = e.ValidateArchipelagos(); err != nil {
			panic(err)
		}
		if d := e.ValidateHydrology(); len(d) > 0 {
			panic(fmt.Sprint(seed, ": ", d[0]))
		}
		report := e.LandShapeDiagnostics()
		reports[seed] = report
		img := image.NewRGBA(image.Rect(0, 0, *width*scale, *height*scale))
		for i, land := range e.Mask {
			c := color.RGBA{34, 62, 79, 255}
			if land != 0 {
				c = color.RGBA{198, 211, 154, 255}
			} else if e.Fields["ocean"][i] == 0 {
				c = color.RGBA{111, 177, 193, 255}
			}
			for yy := 0; yy < scale; yy++ {
				for xx := 0; xx < scale; xx++ {
					x, y := i%(*width)*scale+xx, i/(*width)*scale+yy
					img.Set(x, y, c)
					gallery.Set((k%cols)*(*width)*scale+x, (k/cols)*(*height)*scale+y, c)
				}
			}
		}
		name := fmt.Sprintf("seed-%02d.png", k+1)
		writePNG(filepath.Join(*out, name), img)
		fmt.Fprintf(&page, `<figure><img src="%s" alt="Silhouette for seed %s"><figcaption><strong>Seed %s</strong><br>Land %.1f%%; %d landmasses; %d archipelagos<br>Largest: %d cells; aspect %.2f; hull solidity %.2f; deepest bay %.1f cells</figcaption></figure>`, name, html.EscapeString(seed), html.EscapeString(seed), report.LandFraction*100, len(report.IslandSizes), len(e.Archipelagos), report.Landmasses[0].Area, report.Landmasses[0].Aspect, report.Landmasses[0].Solidity, report.Landmasses[0].BayDepth)
		fmt.Printf("seed %s: land %.2f%%, components %d, archipelagos %d\n", seed, report.LandFraction*100, len(report.IslandSizes), len(e.Archipelagos))
	}
	page.WriteString(`</section><p>Model: organic-spines-1. Coverage is reconciled before archipelagos, lakes and erosion, so final land percentages may shift slightly. These cell-scale diagnostics measure the generated geometry, not projected physical distances. Enclosed depressions become water only when the hydrology budget supports them. Old worlds retain their stored geography.</p>`)
	writePNG(filepath.Join(*out, "gallery.png"), gallery)
	data, _ := json.MarshalIndent(reports, "", "  ")
	if err := os.WriteFile(filepath.Join(*out, "metrics.json"), data, 0644); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "index.html"), []byte(page.String()), 0644); err != nil {
		panic(err)
	}
}
func writePNG(name string, img image.Image) {
	f, err := os.Create(name)
	if err != nil {
		panic(err)
	}
	err = png.Encode(f, img)
	closeErr := f.Close()
	if err != nil {
		panic(err)
	}
	if closeErr != nil {
		panic(closeErr)
	}
}
