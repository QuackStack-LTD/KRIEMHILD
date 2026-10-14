package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"kriemhild/internal/terrain/generate"
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/terrain/tiles"
	"math"
	"os"
	"path/filepath"
)

func main() {
	count := flag.Int("count", 50, "seeds")
	out := flag.String("out", "docs/terrain-rebuild/gallery", "directory")
	flag.Parse()
	os.MkdirAll(*out, 0755)
	html := "<!doctype html><meta charset=utf-8><title>Coasts and local detail</title><style>body{background:#101a23;color:#ddd;font:16px system-ui;margin:30px}figure{display:inline-block}img{max-width:512px}article{border-top:1px solid #567;margin:30px 0}</style><h1>Coastlines, archipelagos and local detail</h1><a href='index.html'>World overview gallery</a>"
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
		p, err := generate.Generate(context.Background(), spec, nil)
		if err != nil {
			panic(err)
		}
		img := image.NewNRGBA(image.Rect(0, 0, 512, 256))
		for x := 0; x < 2; x++ {
			b, err := tiles.Render(context.Background(), p, 0, x, 0, 256, "coastline", nil)
			if err != nil {
				panic(err)
			}
			tile, err := png.Decode(bytes.NewReader(b))
			if err != nil {
				panic(err)
			}
			draw.Draw(img, image.Rect(x*256, 0, (x+1)*256, 256), tile, image.Point{}, draw.Src)
		}
		file := fmt.Sprintf("seed-%03d-coastline.png", i)
		f, err := os.Create(filepath.Join(*out, file))
		if err != nil {
			panic(err)
		}
		png.Encode(f, img)
		f.Close()
		html += fmt.Sprintf("<article><h2>%s</h2><figure><img src='%s' loading=lazy><figcaption>Coastlines</figcaption></figure>", spec.Seed, file)
		arc := p.Masses[len(p.Masses)-1].Center
		// Find an emerged coast, then bisect its continuous field to centre a local
		// view on actual shore geometry rather than a simulation-cell boundary.
		shore := p.Masses[0].Center
		found := false
		for y := 10; y < p.Grid.Height-10 && !found; y++ {
			for x := 0; x < p.Grid.Width-1; x++ {
				a, b := p.Grid.Point(x, y), p.Grid.Point(x+1, y)
				if landmass.Potential(p, a[0], a[1])*landmass.Potential(p, b[0], b[1]) >= 0 {
					continue
				}
				for k := 0; k < 25; k++ {
					m := model.Point{(a[0] + b[0]) / 2, a[1]}
					if landmass.Potential(p, a[0], a[1])*landmass.Potential(p, m[0], m[1]) < 0 {
						b = m
					} else {
						a = m
					}
				}
				shore = a
				found = true
				break
			}
		}
		for _, view := range []struct {
			name string
			z    int
			pt   model.Point
		}{{"archipelago", 5, arc}, {"local", 12, shore}} {
			n := 1 << view.z
			x := int((model.Wrap(view.pt[0]) + math.Pi) / model.Tau * float64(2*n))
			y := int((.5 - view.pt[1]/math.Pi) * float64(n))
			b, err := tiles.Render(context.Background(), p, view.z, x, y, 512, "natural", nil)
			if err != nil {
				panic(err)
			}
			name := fmt.Sprintf("seed-%03d-%s.png", i, view.name)
			os.WriteFile(filepath.Join(*out, name), b, 0644)
			html += fmt.Sprintf("<figure><img src='%s' loading=lazy><figcaption>%s · z%d · 512 samples</figcaption></figure>", name, view.name, view.z)
		}
		html += "</article>"
		os.WriteFile(filepath.Join(*out, "detail.html"), []byte(html), 0644)
		fmt.Println(spec.Seed)
	}
}
