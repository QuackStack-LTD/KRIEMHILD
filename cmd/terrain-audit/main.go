// terrain-audit records area-weighted climate evidence and reproducible galleries.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"kriemhild/internal/terrain/generate"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/terrain/tiles"
	"math"
	"os"
	"path/filepath"
	"time"
)

type group struct {
	Area   float64            `json:"areaWeight"`
	Biomes map[string]float64 `json:"biomes"`
	Fields map[string]float64 `json:"means"`
}

func main() {
	count := flag.Int("count", 30, "Earthlike seeds")
	out := flag.String("out", "docs/terrain-repair/baseline", "output")
	pictures := flag.Bool("images", false, "render world and coast tiles")
	preset := flag.String("preset", "earthlike", "preset for the ensemble")
	flag.Parse()
	must(os.MkdirAll(*out, 0755))
	results := []any{}
	html := "<!doctype html><meta charset=utf-8><title>Terrain repair audit</title><style>body{background:#15202a;color:#eee;font:16px system-ui}img{width:256px}section{display:flex;flex-wrap:wrap}figure{margin:8px}h2{width:100%}</style><h1>Deterministic terrain audit</h1>"
	for seed := 0; seed < *count; seed++ {
		spec := model.DefaultSpec()
		spec.Preset = *preset
		switch *preset {
		case "archipelago":
			spec.LandCoverage = .18
			spec.Continents = 6
			spec.Islands = 2
			spec.Relief = .8
			spec.Rainfall = 1.2
		case "rugged":
			spec.LandCoverage = .35
			spec.Islands = 1.2
			spec.Relief = 1.6
		case "supercontinent":
			spec.LandCoverage = .4
			spec.Continents = 3
			spec.Islands = .7
			spec.Relief = 1.2
		case "ocean":
			spec.LandCoverage = .12
			spec.Continents = 4
			spec.Islands = 1.7
			spec.Relief = .8
		case "arid":
			spec.LandCoverage = .35
			spec.Islands = .5
			spec.Rainfall = .4
		}
		spec.Seed = fmt.Sprintf("repair-%02d", seed)
		start := time.Now()
		p, err := generate.Generate(context.Background(), spec, nil)
		must(err)
		meta, fieldData, err := p.Encode()
		must(err)
		_, err = model.Decode(meta, fieldData)
		must(err)
		groups := map[string]*group{}
		fields := []string{"temperature", "precipitation", "pet", "moisture", "bed", "coast", "roughness", "windX", "windY", "soil", "aridity", "rainShadow", "exposure", "slope", "seasonality", "dryMonths", "warmDry", "coldRain"}
		for m := 0; m < 12; m++ {
			for _, f := range []string{"temperature", "rainfall", "pet"} {
				fields = append(fields, fmt.Sprintf("%s-%02d", f, m))
			}
		}
		missing := []string{}
		for _, f := range fields {
			if len(p.Grid.Fields[f]) != p.Grid.Width*p.Grid.Height {
				missing = append(missing, f)
			}
		}
		labels := make([]int, p.Grid.Width*p.Grid.Height)
		components := 0
		for i := range p.Grid.Fields["bed"] {
			if !p.Emerged(i) || labels[i] != 0 {
				continue
			}
			components++
			q := []int{i}
			labels[i] = components
			for h := 0; h < len(q); h++ {
				j := q[h]
				x, y := j%p.Grid.Width, j/p.Grid.Width
				ns := []int{y*p.Grid.Width + (x+1)%p.Grid.Width, y*p.Grid.Width + (x+p.Grid.Width-1)%p.Grid.Width}
				if y > 0 {
					ns = append(ns, j-p.Grid.Width)
				}
				if y+1 < p.Grid.Height {
					ns = append(ns, j+p.Grid.Width)
				}
				for _, k := range ns {
					if labels[k] == 0 && p.Emerged(k) {
						labels[k] = components
						q = append(q, k)
					}
				}
			}
		}
		for i, b := range p.Grid.Fields["bed"] {
			if !p.Emerged(i) {
				continue
			}
			pt := p.Grid.Point(i%p.Grid.Width, i/p.Grid.Width)
			weight := math.Cos(pt[1])
			latband := fmt.Sprintf("latitude-%+03d", int(math.Floor(pt[1]*180/math.Pi/15))*15)
			biome := generate.BiomeAt(p, pt[0], pt[1], b, p.Grid.Fields["lake"][i] > 0 && p.Grid.Fields["lake"][i] > b)
			for _, name := range []string{"land", latband, fmt.Sprintf("continent-%d", labels[i])} {
				g := groups[name]
				if g == nil {
					g = &group{Biomes: map[string]float64{}, Fields: map[string]float64{}}
					groups[name] = g
				}
				g.Area += weight
				g.Biomes[biome] += weight
				for _, f := range fields {
					if len(p.Grid.Fields[f]) > i {
						g.Fields[f] += p.Grid.Fields[f][i] * weight
					}
				}
			}
		}
		for _, g := range groups {
			for k := range g.Biomes {
				g.Biomes[k] /= g.Area
			}
			for k := range g.Fields {
				g.Fields[k] /= g.Area
			}
		}
		entry := map[string]any{"spec": spec, "groups": groups, "missingFields": missing, "componentShapes": shapeMetrics(p, labels), "diagnostics": p.Diagnostics, "masses": p.Masses, "riverCount": len(p.Rivers), "generationMs": time.Since(start).Milliseconds()}
		results = append(results, entry)
		b, _ := json.MarshalIndent(entry, "", "  ")
		must(os.WriteFile(filepath.Join(*out, fmt.Sprintf("seed-%02d.json", seed)), b, 0644))
		if *pictures {
			html += fmt.Sprintf("<section><h2>%s</h2>", spec.Seed)
			for _, style := range []string{"natural", "coastline", "elevation", "biome"} {
				for x := 0; x < 2; x++ {
					data, e := tiles.Render(context.Background(), p, 0, x, 0, 256, style, nil)
					must(e)
					name := fmt.Sprintf("%02d-%s-%d.png", seed, style, x)
					must(os.WriteFile(filepath.Join(*out, name), data, 0644))
					html += fmt.Sprintf("<figure><img loading=lazy src='%s'><figcaption>%s %d</figcaption></figure>", name, style, x)
				}
			}
			found := 0
			for i, c := range p.Grid.Fields["coast"] {
				if math.Abs(c) > 45 || found == 2 {
					continue
				}
				pt := p.Grid.Point(i%p.Grid.Width, i/p.Grid.Width)
				x := int((pt[0] + math.Pi) / model.Tau * 64)
				y := int((.5 - pt[1]/math.Pi) * 32)
				data, e := tiles.Render(context.Background(), p, 5, x, y, 256, "natural", nil)
				must(e)
				name := fmt.Sprintf("%02d-coast-%d.png", seed, found)
				must(os.WriteFile(filepath.Join(*out, name), data, 0644))
				html += fmt.Sprintf("<figure><img loading=lazy src='%s'><figcaption>Coast detail z5</figcaption></figure>", name)
				found++
			}
			html += "</section>"
			must(os.WriteFile(filepath.Join(*out, "index.html"), []byte(html), 0644))
		}
		fmt.Printf("%s: steppe %.3f desert %.3f rivers %d (%s)\n", spec.Seed, p.Diagnostics.BiomeShares["steppe"], p.Diagnostics.BiomeShares["desert"], len(p.Rivers), time.Since(start).Round(time.Millisecond))
	}
	b, _ := json.MarshalIndent(results, "", "  ")
	must(os.WriteFile(filepath.Join(*out, "metrics.json"), b, 0644))
}
func must(e error) {
	if e != nil {
		panic(e)
	}
}
