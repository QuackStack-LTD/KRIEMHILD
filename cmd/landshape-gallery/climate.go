package main

import (
	"fmt"
	"image"
	"image/color"
	"kriemhild/internal/terrain"
	"math"
	"path/filepath"
	"strings"
)

func writeClimateGallery(out string, k int, e *terrain.Environment, page *strings.Builder) {
	w, h := e.Options.Columns, e.Options.Rows
	colors := map[string]color.RGBA{}
	for _, b := range terrain.Biomes {
		var r, g, bb uint8
		fmt.Sscanf(b.Color, "#%02x%02x%02x", &r, &g, &bb)
		colors[b.ID] = color.RGBA{r, g, bb, 255}
	}
	defaults, _ := terrain.DecodeConfig(terrain.Defaults)
	for _, t := range defaults.Types {
		if _, ok := colors[t.ID]; ok {
			var r, g, b uint8
			fmt.Sscanf(t.Color, "#%02x%02x%02x", &r, &g, &b)
			colors[t.ID] = color.RGBA{r, g, b, 255}
		}
	}
	climate := map[string]string{"rainforest": "jungle", "monsoon": "savanna", "tropical-seasonal": "savanna", "desert": "desert", "steppe": "steppe", "mediterranean": "mediterranean", "subtropical": "forest", "oceanic": "forest", "continental": "grass", "boreal": "taiga", "tundra": "tundra", "alpine": "alpine", "polar": "snow"}
	model := terrain.NewDetailModel(e)
	center := e.ContinentalStructures[0].Center
	focus := center
	nearest := math.Inf(1)
	for i, land := range e.Mask {
		if land == 0 {
			continue
		}
		x, y := i%w, i/w
		coastal := false
		for _, j := range []int{i - 1, i + 1, i - w, i + w} {
			if j >= 0 && j < len(e.Mask) && e.Fields["waterBody"][j] == 1 {
				coastal = true
			}
		}
		if coastal {
			d := math.Hypot(float64(x)-center[0], float64(y)-center[1])
			if d < nearest {
				nearest = d
				focus = [2]float64{float64(x), float64(y)}
			}
		}
	}
	for _, layer := range []string{"climate", "precipitation", "aridityIndex", "biome", "continental-zoom", "regional-zoom", "local-zoom"} {
		img := image.NewRGBA(image.Rect(0, 0, w*3, h*3))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				wx, wy := float64(x), float64(y)
				level := 0
				if layer == "continental-zoom" || layer == "regional-zoom" || layer == "local-zoom" {
					fraction := .5
					level = 1
					if layer == "regional-zoom" {
						fraction = .2
						level = 3
					}
					if layer == "local-zoom" {
						fraction = .04
						level = 6
					}
					center := focus
					wx = center[0] + (float64(x)-float64(w)/2)*fraction
					wy = center[1] + (float64(y)-float64(h)/2)*fraction
					wx = math.Mod(wx+float64(w), float64(w))
					wy = math.Max(0, math.Min(float64(h-1), wy))
				}
				i := int(math.Round(wy))*w + int(math.Round(wx))%w
				c := color.RGBA{34, 62, 79, 255}
				if e.Mask[i] != 0 {
					switch layer {
					case "climate":
						c = colors[climate[e.ClimateKind(i)]]
					case "precipitation", "aridityIndex":
						v := e.Fields[layer][i] / 3000
						if layer == "aridityIndex" {
							v = e.Fields[layer][i] / 2
						}
						v = math.Max(0, math.Min(1, v))
						c = color.RGBA{uint8(210 - 160*v), uint8(150 + 30*v), uint8(80 + 120*v), 255}
					default:
						c = colors[e.BiomeAt(i)]
					}
				}
				if level > 0 {
					p := model.Sample(wx, wy, level)
					if p.WaterBody > 0 {
						c = color.RGBA{34, 62, 79, 255}
					} else if p.Biome != "" {
						c = colors[p.Biome]
					}
					shade := .83 + .17*math.Min(1, math.Max(0, p.Elevation)/2500)
					c.R = uint8(float64(c.R) * shade)
					c.G = uint8(float64(c.G) * shade)
					c.B = uint8(float64(c.B) * shade)
				}
				for dy := 0; dy < 3; dy++ {
					for dx := 0; dx < 3; dx++ {
						img.Set(x*3+dx, y*3+dy, c)
					}
				}
			}
		}
		name := fmt.Sprintf("seed-%02d-%s.png", k+1, layer)
		writePNG(filepath.Join(out, name), img)
		fmt.Fprintf(page, `<figure><img src="%s" alt="Seed %s, %s"><figcaption>%s — same seed; coastal zooms share focus (%.1f, %.1f).`, name, e.Options.Seed, layer, layer, focus[0], focus[1])
		if layer == "aridityIndex" {
			page.WriteString(" Brown: AI 0; blue: AI ≥2. Desert <0.25; steppe 0.25–0.50; dry subhumid 0.50–0.65.")
		}
		if layer == "precipitation" {
			page.WriteString(" Brown: 0; blue: ≥3000 mm/year.")
		}
		page.WriteString("</figcaption></figure>")
	}
}
