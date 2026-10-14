package generate

import (
	"kriemhild/internal/terrain/model"
	"math"
	"sort"
	"strings"
)

// Measurements explicitly describe macro-grid resolution; they do not pretend
// to measure the infinite-frequency continuous shoreline or classify every bay.
func measure(p *model.Planet) {
	g, d := &p.Grid, &p.Diagnostics
	d.CoverageDeviation = d.LandCoverage - p.Spec.LandCoverage
	d.RiverDestinations = map[string]int{}
	d.LatitudeBiomes = map[string]map[string]float64{}
	bandAreas := map[string]float64{}
	catchments := map[int]float64{}
	land, lakes := 0., 0.
	dlon, dlat := model.Tau/float64(g.Width), math.Pi/float64(g.Height)
	for i, b := range g.Fields["bed"] {
		if b <= 0 {
			continue
		}
		x, y := i%g.Width, i/g.Width
		lat := g.Point(x, y)[1]
		area := dlon * dlat * math.Cos(lat) * p.Spec.RadiusKM * p.Spec.RadiusKM
		land += area
		if g.Fields["lake"][i] > b {
			lakes += area
		}
		band := "temperate"
		if math.Abs(lat) < math.Pi/6 {
			band = "tropical"
		} else if math.Abs(lat) > math.Pi/3 {
			band = "polar"
		}
		if d.LatitudeBiomes[band] == nil {
			d.LatitudeBiomes[band] = map[string]float64{}
		}
		biome := Biome(g.Fields["temperature"][i], g.Fields["precipitation"][i], g.Fields["pet"][i], b, g.Fields["lake"][i] > b)
		d.LatitudeBiomes[band][biome] += area
		bandAreas[band] += area
		for _, j := range neighbors(i, g.Width, g.Height) {
			if g.Fields["bed"][j] > 0 {
				continue
			}
			edge := dlat * p.Spec.RadiusKM
			if j/g.Width != y {
				edge = dlon * math.Cos(lat) * p.Spec.RadiusKM
			}
			d.CoastLengthKM += edge
		}
		outlet := i
		for step := 0; step < len(g.Fields["bed"]); step++ {
			j := int(g.Fields["flow"][outlet])
			if j < 0 || g.Fields["bed"][j] <= 0 {
				break
			}
			outlet = j
		}
		catchments[outlet] += area
	}
	for band, shares := range d.LatitudeBiomes {
		for biome, area := range shares {
			shares[biome] = area / math.Max(1, bandAreas[band])
		}
	}
	for _, area := range catchments {
		d.CatchmentAreas = append(d.CatchmentAreas, area)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(d.CatchmentAreas)))
	d.LakeCoverage = lakes / math.Max(land, 1)
	d.Compactness = 4 * math.Pi * land / math.Max(1, d.CoastLengthKM*d.CoastLengthKM)
	lengths, downstream := map[string]float64{}, map[string]string{}
	for _, r := range p.Rivers {
		kind := r.Destination
		if strings.HasPrefix(kind, "river-") {
			kind = "confluence"
		}
		d.RiverDestinations[kind]++
		downstream[r.ID] = r.Destination
		for i := 1; i < len(r.Path); i++ {
			a, b := r.Path[i-1], r.Path[i]
			h := math.Pow(math.Sin((b[1]-a[1])/2), 2) + math.Cos(a[1])*math.Cos(b[1])*math.Pow(math.Sin(model.Wrap(b[0]-a[0])/2), 2)
			lengths[r.ID] += 2 * math.Asin(math.Sqrt(model.Clamp(h, 0, 1))) * p.Spec.RadiusKM
		}
	}
	for _, r := range p.Rivers {
		length := 0.
		seen := map[string]bool{}
		for id := r.ID; id != "" && !seen[id]; id = downstream[id] {
			seen[id] = true
			length += lengths[id]
		}
		d.LongestRiverKM = math.Max(d.LongestRiverKM, length)
	}
}
