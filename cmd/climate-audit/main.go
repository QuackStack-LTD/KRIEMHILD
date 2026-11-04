// climate-audit records active-generator fields, classification and render costs.
// Run before and after a targeted change with identical seeds and configuration.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
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
	rainScale := flag.Float64("rainfall", model.DefaultSpec().Rainfall, "rainfall setting for paired experiments")
	relief := flag.Float64("relief", model.DefaultSpec().Relief, "relief setting for paired experiments")
	load := flag.String("load", "", "read an existing planet's metadata and fields from this directory; never generate")
	flag.Parse()
	if err := os.MkdirAll(*out, 0755); err != nil {
		panic(err)
	}
	for _, seed := range strings.Split(*seeds, ",") {
		spec := model.DefaultSpec()
		spec.ClimateVersion = *climateVersion
		spec.Seed = seed
		spec.Rainfall, spec.Relief = *rainScale, *relief
		start := time.Now()
		var p *model.Planet
		var err error
		if *load == "" {
			p, err = generate.Generate(context.Background(), spec, nil)
		} else {
			meta, e := os.ReadFile(filepath.Join(*load, "metadata"))
			if e != nil {
				panic(e)
			}
			fields, e := os.ReadFile(filepath.Join(*load, "fields"))
			if e != nil {
				panic(e)
			}
			p, err = model.Decode(meta, fields)
			if err == nil {
				spec, seed = p.Spec, p.Spec.Seed
			}
		}
		if err != nil {
			panic(err)
		}
		generationMS := float64(time.Since(start).Microseconds()) / 1000
		g := p.Grid
		hashes := map[string]string{}
		for key, field := range g.Fields {
			h := sha256.New()
			var b [8]byte
			for _, v := range field {
				binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
				h.Write(b[:])
			}
			hashes["grid/"+key] = fmt.Sprintf("%x", h.Sum(nil))
		}
		if p.Surface != nil {
			for key, field := range map[string][]float32{"bed": p.Surface.Bed, "coast": p.Surface.Coast, "roughness": p.Surface.Roughness, "lakeLevel": p.Surface.LakeLevel} {
				h := sha256.New()
				var b [4]byte
				for _, v := range field {
					binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
					h.Write(b[:])
				}
				hashes["surface/"+key] = fmt.Sprintf("%x", h.Sum(nil))
			}
		}
		writeJSON(filepath.Join(*out, seed+"-hashes.json"), hashes)
		n := g.Width * g.Height
		labels := make([]string, n)
		weights := make([]float64, n)
		land, globe, emerged, lake := 0., 0., 0., 0.
		shares := map[string]float64{}
		regimes := map[string]float64{}
		bands := map[string]map[string]float64{}
		bandArea := map[string]float64{}
		values := map[string][]weighted{}
		causal := map[string]map[string][]weighted{}
		derived := map[string][]float64{}
		for _, key := range []string{"growingMonths", "freezingMonths", "dryMonthsMeasured", "warmSeasonRain", "warmSeasonPET", "coldSeasonRainFraction"} {
			derived[key] = make([]float64, n)
		}
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
			if spec.ClimateVersion >= 1 {
				c := generate.ClimateRegimeAt(p, pt[0], pt[1], g.Fields["bed"][i])
				regimes[c.Thermal+"/"+c.Moisture+"/"+c.Rainfall] += weight
			}
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
			for m := 0; m < 12; m++ {
				temp, rain, pet := g.Fields[fmt.Sprintf("temperature-%02d", m)][i], g.Fields[fmt.Sprintf("rainfall-%02d", m)][i], g.Fields[fmt.Sprintf("pet-%02d", m)][i]
				if temp > 10 {
					derived["growingMonths"][i]++
					derived["warmSeasonRain"][i] += rain
					derived["warmSeasonPET"][i] += pet
				}
				if temp < 0 {
					derived["freezingMonths"][i]++
				}
				if rain < pet*.5 {
					derived["dryMonthsMeasured"][i]++
				}
				if temp < 15 {
					derived["coldSeasonRainFraction"][i] += rain / math.Max(1, g.Fields["precipitation"][i])
				}
			}
			for key, a := range derived {
				values[key] = append(values[key], weighted{a[i], weight})
			}
			coastBand := "1500+"
			coast := g.Fields["coast"][i]
			if coast < 100 {
				coastBand = "0-100"
			} else if coast < 500 {
				coastBand = "100-500"
			} else if coast < 1500 {
				coastBand = "500-1500"
			}
			elevation := "2500+"
			bed := g.Fields["bed"][i]
			if bed < 200 {
				elevation = "0-200"
			} else if bed < 1000 {
				elevation = "200-1000"
			} else if bed < 2500 {
				elevation = "1000-2500"
			}
			for _, group := range []string{"latitude/" + band, "coastKM/" + coastBand, "latitudeCoast/" + band + "/" + coastBand, "elevationM/" + elevation, "biome/" + labels[i]} {
				if causal[group] == nil {
					causal[group] = map[string][]weighted{}
				}
				for _, key := range []string{"temperature", "precipitation", "pet", "aridity", "bed", "coast", "dryMonths"} {
					causal[group][key] = append(causal[group][key], weighted{g.Fields[key][i], weight})
				}
				for key, a := range derived {
					causal[group][key] = append(causal[group][key], weighted{a[i], weight})
				}
				causal[group]["seasonalRange"] = append(causal[group]["seasonalRange"], weighted{hi - lo, weight})
			}
		}
		causalStats := map[string]any{}
		for group, fields := range causal {
			stats := map[string]any{}
			area := 0.
			for _, v := range fields["temperature"] {
				area += v.w
			}
			stats["dryLandShare"] = area / land
			for field, v := range fields {
				stats[field] = summarize(v)
			}
			causalStats[group] = stats
		}
		writeJSON(filepath.Join(*out, seed+"-causal-fields.json"), causalStats)
		for key, a := range derived {
			img := image.NewNRGBA(image.Rect(0, 0, g.Width, g.Height))
			maximum := 12.
			if key == "warmSeasonRain" || key == "warmSeasonPET" {
				maximum = 2400
			}
			if key == "coldSeasonRainFraction" {
				maximum = 1
			}
			for i, v := range a {
				c := color.NRGBA{46, 108, 143, 255}
				if labels[i] != "" {
					f := model.Clamp(v/maximum, 0, 1)
					c = color.NRGBA{uint8(255 * f), uint8(180 * (1 - math.Abs(2*f-1))), uint8(255 * (1 - f)), 255}
				}
				img.SetNRGBA(i%g.Width, i/g.Width, c)
			}
			f, err := os.Create(filepath.Join(*out, seed+"-"+key+".png"))
			if err != nil {
				panic(err)
			}
			if err = png.Encode(f, img); err != nil {
				panic(err)
			}
			f.Close()
		}
		for k := range shares {
			shares[k] /= land
		}
		for k := range regimes {
			regimes[k] /= land
		}
		writeJSON(filepath.Join(*out, seed+"-regimes.json"), regimes)
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
			small := 0
			for _, area := range areas {
				if area < .0001 {
					small++
				}
			}
			connectivity[biome] = map[string]any{"regions": len(areas), "largestLandShare": areas[0], "largestBiomeFraction": areas[0] / shares[biome], "medianLandShare": areas[len(areas)/2], "regionsBelowOneBasisPoint": small}
		}
		var memory runtime.MemStats
		// January through December, four columns by three rows, fixed scales.
		for _, field := range []string{"temperature", "rainfall"} {
			img := image.NewNRGBA(image.Rect(0, 0, g.Width*4, g.Height*3))
			for m := 0; m < 12; m++ {
				for i, value := range g.Fields[fmt.Sprintf("%s-%02d", field, m)] {
					v := max(0., min(1., value/300))
					if field == "temperature" {
						v = max(0., min(1., (value+40)/80))
					}
					img.SetNRGBA(m%4*g.Width+i%g.Width, m/4*g.Height+i/g.Width, color.NRGBA{uint8(255 * v), uint8(180 * (1 - math.Abs(2*v-1))), uint8(255 * (1 - v)), 255})
				}
			}
			f, err := os.Create(filepath.Join(*out, seed+"-monthly-"+field+".png"))
			if err != nil {
				panic(err)
			}
			if err = png.Encode(f, img); err != nil {
				panic(err)
			}
			f.Close()
		}
		runtime.GC()
		runtime.ReadMemStats(&memory)
		render := []map[string]any{}
		for _, style := range []string{"biome", "temperature", "precipitation", "aridity", "seasonality", "elevation", "mask", "pet", "moisture", "soil", "rainShadow", "slope"} {
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
		latitudeLand := map[string]float64{}
		for band, area := range bandArea {
			latitudeLand[band] = area / land
		}
		writeJSON(filepath.Join(*out, seed+"-latitude-land.json"), latitudeLand)
		if spec.ClimateVersion >= 1 {
			palettes := map[string]map[string]color.NRGBA{
				"thermal":  {"ice": {240, 245, 250, 255}, "tundra": {170, 182, 173, 255}, "boreal": {61, 103, 92, 255}, "temperate": {83, 157, 72, 255}, "tropical": {12, 97, 69, 255}},
				"moisture": {"arid": {215, 175, 97, 255}, "semi-arid": {185, 171, 95, 255}, "subhumid": {139, 166, 84, 255}, "moist": {83, 157, 72, 255}, "humid": {21, 104, 87, 255}},
				"rainfall": {"seasonal": {132, 158, 96, 255}, "summer-dry": {203, 161, 77, 255}, "perennial-wet": {24, 108, 153, 255}},
			}
			for axis, palette := range palettes {
				img := image.NewNRGBA(image.Rect(0, 0, g.Width, g.Height))
				for i, label := range labels {
					c := color.NRGBA{46, 108, 143, 255}
					if label != "" {
						pt := g.Point(i%g.Width, i/g.Width)
						r := generate.ClimateRegimeAt(p, pt[0], pt[1], g.Fields["bed"][i])
						key := r.Thermal
						if axis == "moisture" {
							key = r.Moisture
						}
						if axis == "rainfall" {
							key = r.Rainfall
						}
						c = palette[key]
					}
					img.SetNRGBA(i%g.Width, i/g.Width, c)
				}
				var b bytes.Buffer
				if err := png.Encode(&b, img); err != nil {
					panic(err)
				}
				if err := os.WriteFile(filepath.Join(*out, seed+"-regime-"+axis+".png"), b.Bytes(), 0644); err != nil {
					panic(err)
				}
			}
			writeJSON(filepath.Join(*out, seed+"-regime-palettes.json"), palettes)
		}
		writeJSON(filepath.Join(*out, seed+".json"), map[string]any{"spec": spec, "generationMs": generationMS, "liveHeapBytes": memory.HeapAlloc, "landFraction": emerged / globe, "dryLandFraction": land / globe, "lakeFraction": lake / globe, "oceanFraction": 1 - emerged/globe, "biomeLandShares": shares, "latitudeBiomes": bands, "fields": fields, "connectivity": connectivity, "render": render, "diagnostics": p.Diagnostics, "grid": []int{g.Width, g.Height}, "sampling": "cos(latitude)-weighted grid; biome/field/connectivity statistics exclude ocean and lake cells"})
		fmt.Printf("%s: %.1fs; land %.3f; forest %.3f; rainforest %.3f; temperate rainforest %.3f; steppe %.3f; desert %.3f\n", seed, generationMS/1000, land/globe, shares["forest"], shares["rainforest"], shares["temperate-rainforest"], shares["steppe"], shares["desert"])
		if *load != "" {
			break
		}
	}
}
