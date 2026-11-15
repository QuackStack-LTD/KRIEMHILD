// climate-distribution generates paired, area-weighted climate evidence and a
// static gallery. Diagnostic limits never feed into generation.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/png"
	"kriemhild/internal/terrain/generate"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/terrain/tiles"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type evidence struct {
	Spec        model.Spec                              `json:"spec"`
	Bands       map[string]model.ClimateBandDiagnostics `json:"bands"`
	Budget      []model.ClimateWaterBudget              `json:"monthlyWaterBudget,omitempty"`
	BudgetError float64                                 `json:"waterBudgetError"`
	Hashes      map[string]string                       `json:"fieldHashes"`
	Transitions map[string]float64                      `json:"transitionsFromV1,omitempty"`
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func writeJSON(path string, value any) {
	b, err := json.MarshalIndent(value, "", "  ")
	must(err)
	must(os.WriteFile(path, append(b, '\n'), 0644))
}
func labels(p *model.Planet) []string {
	a := make([]string, p.Grid.Width*p.Grid.Height)
	for i, bed := range p.Grid.Fields["bed"] {
		if !p.Emerged(i) || p.Grid.Fields["lake"][i] > math.Max(0, bed) {
			continue
		}
		pt := p.Grid.Point(i%p.Grid.Width, i/p.Grid.Width)
		a[i] = generate.BiomeAt(p, pt[0], pt[1], bed, false)
	}
	return a
}

func main() {
	out := flag.String("out", "docs/terrain-rebuild/climate-v4/fixed", "output directory")
	manifest := flag.String("manifest", "docs/terrain-rebuild/climate-v4/seeds.json", "frozen seed manifest")
	seedList := flag.String("seeds", "", "optional comma separated subset")
	versionList := flag.String("versions", "1,3,4", "climate versions")
	fixed := flag.Bool("fixed", true, "compare climates on identical saved relief and water masks")
	width := flag.Int("surface-width", 1024, "authoritative terrain width")
	render := flag.Bool("render", true, "write paired maps and gallery")
	flag.Parse()
	var seeds []string
	if *seedList != "" {
		seeds = strings.Split(*seedList, ",")
	} else {
		b, err := os.ReadFile(*manifest)
		must(err)
		var m struct{ Tuning, HeldOut []string }
		must(json.Unmarshal(b, &m))
		seeds = append(m.Tuning, m.HeldOut...)
	}
	var versions []int
	for _, v := range strings.Split(*versionList, ",") {
		n, err := strconv.Atoi(v)
		must(err)
		versions = append(versions, n)
	}
	must(os.MkdirAll(*out, 0755))
	gallery := "<!doctype html><meta charset=utf-8><title>Climate distribution comparison</title><style>body{background:#172128;color:#eee;font:16px system-ui;margin:24px}table{width:100%}td{vertical-align:top}img{width:100%;image-rendering:auto}th{padding:8px}h2{margin-top:32px}</style><h1>Paired climate distribution</h1><p>Same seed, projection, scales and palette. Fixed terrain: " + strconv.FormatBool(*fixed) + ". All biome shares exclude inland water. Rainfall remains 1.65.</p>"
	all := map[string]map[string]evidence{}
	for _, seed := range seeds {
		spec := model.DefaultSpec()
		spec.ClimateVersion = 1
		spec.Seed = seed
		spec.SurfaceWidth = *width
		var source *model.Planet
		var err error
		if *fixed {
			source, err = generate.Generate(context.Background(), spec, nil)
			must(err)
		}
		byVersion := map[string]evidence{}
		var baselineLabels []string
		gallery += "<h2>" + html.EscapeString(seed) + "</h2><table><tr>"
		for _, version := range versions {
			gallery += fmt.Sprintf("<th>Climate %d</th>", version)
		}
		gallery += "</tr>"
		for _, version := range versions {
			spec.ClimateVersion = version
			var p *model.Planet
			if *fixed {
				p, err = generate.ClimateOnFixedTerrain(context.Background(), source, version)
			} else {
				p, err = generate.Generate(context.Background(), spec, nil)
			}
			must(err)
			key := strconv.Itoa(version)
			e := evidence{Spec: p.Spec, Bands: generate.ClimateMeasurements(p), Budget: p.Diagnostics.ClimateBudget, BudgetError: p.Diagnostics.MoistureBudgetError, Hashes: map[string]string{}}
			for name, a := range p.Grid.Fields {
				h := sha256.New()
				var b [8]byte
				for _, v := range a {
					binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
					h.Write(b[:])
				}
				e.Hashes[name] = fmt.Sprintf("%x", h.Sum(nil))
			}
			biomes := labels(p)
			if version == 1 {
				baselineLabels = biomes
			}
			if baselineLabels != nil {
				e.Transitions = map[string]float64{}
				area := 0.
				for i, label := range biomes {
					if label == "" || baselineLabels[i] == "" {
						continue
					}
					weight := math.Cos(p.Grid.Point(i%p.Grid.Width, i/p.Grid.Width)[1])
					area += weight
					e.Transitions[baselineLabels[i]+" -> "+label] += weight
				}
				for k := range e.Transitions {
					e.Transitions[k] /= math.Max(1, area)
				}
			}
			byVersion[key] = e
			if *render {
				maps(*out, seed, version, p, biomes)
			}
			d := e.Bands["all"]
			fmt.Printf("%s v%d: rain %.0f PET %.0f; forest %.2f%% rainforest %.2f%% seasonal %.2f%% taiga %.2f%% desert %.2f%% savanna %.2f%%\n", seed, version, d.Precipitation, d.PET, 100*d.Biomes["forest"], 100*d.Biomes["rainforest"], 100*d.Biomes["tropical-woodland"], 100*d.Biomes["taiga"], 100*d.Biomes["desert"], 100*d.Biomes["savanna"])
		}
		for _, style := range []string{"biome", "thermal", "precipitation", "aridity", "coldest", "warmest", "dryMonths", "range"} {
			gallery += "<tr>"
			for _, version := range versions {
				gallery += fmt.Sprintf("<td>%s<img loading=lazy src=\"%s-v%d-%s.png\"></td>", style, html.EscapeString(seed), version, style)
			}
			gallery += "</tr>"
		}
		gallery += "</table>"
		all[seed] = byVersion
		writeJSON(filepath.Join(*out, seed+".json"), byVersion)
		writeJSON(filepath.Join(*out, "results.json"), all)
		if *render {
			must(os.WriteFile(filepath.Join(*out, "index.html"), []byte(gallery), 0644))
		}
	}
}

func maps(out, seed string, version int, p *model.Planet, biomes []string) {
	thermal := map[string][3]uint8{"ice": {240, 245, 250}, "tundra": {170, 182, 173}, "boreal": {61, 103, 92}, "temperate": {83, 157, 72}, "tropical": {12, 97, 69}}
	g := p.Grid
	images := map[string]*image.NRGBA{}
	for _, key := range []string{"biome", "thermal", "precipitation", "aridity", "coldest", "warmest", "dryMonths", "range"} {
		images[key] = image.NewNRGBA(image.Rect(0, 0, g.Width, g.Height))
	}
	for i, biome := range biomes {
		pt := g.Point(i%g.Width, i/g.Width)
		c := generate.ClimateRegimeAt(p, pt[0], pt[1], g.Fields["bed"][i])
		for key, img := range images {
			col := [3]uint8{46, 108, 143}
			if biome != "" {
				switch key {
				case "biome":
					col = tiles.BiomeColors[biome]
				case "thermal":
					col = thermal[c.Thermal]
				default:
					v := 0.
					switch key {
					case "precipitation":
						v = c.Rain / 3000
					case "aridity":
						v = c.Aridity / 2
					case "coldest":
						v = (c.Coldest + 40) / 80
					case "warmest":
						v = (c.Warmest + 40) / 80
					case "dryMonths":
						v = c.DryMonths / 12
					case "range":
						v = (c.Warmest - c.Coldest) / 50
					}
					v = model.Clamp(v, 0, 1)
					col = [3]uint8{uint8(255 * v), uint8(180 * (1 - math.Abs(2*v-1))), uint8(255 * (1 - v))}
				}
			}
			img.SetNRGBA(i%g.Width, i/g.Width, color.NRGBA{col[0], col[1], col[2], 255})
		}
	}
	for key, img := range images {
		f, err := os.Create(filepath.Join(out, fmt.Sprintf("%s-v%d-%s.png", seed, version, key)))
		must(err)
		must(png.Encode(f, img))
		must(f.Close())
	}
}
