// Package resources continues the completed continuous planet's physical model.
// It reuses the retained eligibility validator, not the retired terrain solver.
package resources

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	terrain "kriemhild/internal/legacyterrain"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/terrain/tectonics"
	"math"
	"sort"
)

const Version = "continuous-resources/1"

// Source excludes timings and render quality. No derived resource field is an
// input to itself; terrain/climate/drainage edits get a different identity.
func Source(p *model.Planet) string {
	h := sha256.New()
	spec := p.Spec
	spec.TileSize = 0
	raw, _ := json.Marshal(struct {
		Version    string
		Seed       uint64
		SeaOffset  float64
		GridWidth  int
		GridHeight int
		Spec       model.Spec
		Masses     []model.Mass
		Plates     []model.Plate
		Rivers     []model.River
		Lakes      []model.Lake
	}{Version, p.Seed, p.SeaOffset, p.Grid.Width, p.Grid.Height, spec, p.Masses, p.Plates, p.Rivers, p.Lakes})
	h.Write(raw)
	keys := make([]string, 0, len(p.Grid.Fields))
	for k := range p.Grid.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		_ = binary.Write(h, binary.LittleEndian, p.Grid.Fields[k])
	}
	if p.Surface != nil {
		for _, a := range [][]float32{p.Surface.Bed, p.Surface.Coast, p.Surface.Roughness, p.Surface.LakeLevel} {
			_ = binary.Write(h, binary.LittleEndian, a)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Populate is called after the final surface, climate and actual drainage graph.
// It never writes p, changes flow, invokes a terrain solver or invents rivers.
// The retained parent-grid CRS is used by saved ecology and authored geography.
func Populate(ctx context.Context, p *model.Planet, e *terrain.Environment) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p == nil || e == nil || e.Geography == nil {
		return fmt.Errorf("continuous physical terrain and geographic scale required")
	}
	w, h := e.Options.Columns, e.Options.Rows
	n := w * h
	if n == 0 || len(e.Mask) != n || len(p.Grid.Fields["flow"]) != p.Grid.Width*p.Grid.Height {
		return fmt.Errorf("incomplete physical/drainage state")
	}
	marine, lakes := append([]float64(nil), e.Fields["ocean"]...), append([]float64(nil), e.Fields["lake"]...)
	for _, k := range terrain.HydrologyFields {
		e.Fields[k] = make([]float64, n)
	}
	for _, k := range []string{"boundary", "convergence", "tectonicStress", "volcano", "crustAge", "mountainCore", "highland", "shelf", "slope", "sediment", "fluvialDeposition", "accumulation", "catchmentArea", "flow", "drainageElevation", "permeability", "lithology", "geothermal", "recharge", "runoff", "groundFlow", "baseflow", "waterTableDepth", "dryDischarge", "river", "wetland", "salinity", "summer", "winter", "groundwater", "terminalBasin", "waterBody", "ocean", "lake", "windStrength", "dryRunoff"} {
		e.Fields[k] = make([]float64, n)
	}
	// Coarsen the SAVED receiver graph using the lowest drainage node in each
	// footprint. Receivers strictly descend, so this projection cannot form loops.
	owner := make([]int, p.Grid.Width*p.Grid.Height)
	representative := make([]int, n)
	for i := range representative {
		representative[i] = -1
	}
	for i := range owner {
		pt := p.Grid.Point(i%p.Grid.Width, i/p.Grid.Width)
		x := int(math.Round((pt[0] + math.Pi) / model.Tau * float64(w-1)))
		y := int(math.Round((.5 - pt[1]/math.Pi) * float64(h-1)))
		cell := max(0, min(n-1, y*w+x))
		owner[i] = cell
		if j := representative[cell]; j < 0 || p.Grid.Fields["drainage"][i] < p.Grid.Fields["drainage"][j] {
			representative[cell] = i
		}
	}
	for i := 0; i < n; i++ {
		if i%w == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		lon := -math.Pi + model.Tau*float64(i%w)/float64(w-1)
		lat := math.Pi/2 - math.Pi*float64(i/w)/float64(h-1)
		t := tectonics.At(p, lon, lat)
		v := func(k string) float64 { return p.Grid.At(k, lon, lat) }
		// Age is a province-scale geological episode, not present biome/elevation.
		plateAge := 700. + 2900.*unit(p.Seed, uint64(t.Plate), 51)
		if !t.Continental {
			plateAge = 5 + 175*unit(p.Seed, uint64(t.Plate), 51)
		}
		values := map[string]float64{"plate": float64(t.Plate), "convergence": t.Convergence, "tectonicStress": math.Max(t.Convergence, t.Divergence), "volcano": math.Max(t.Convergence*.9, t.Divergence*.75), "crustAge": plateAge, "mountainCore": t.Convergence, "highland": model.Clamp(e.Fields["elevation"][i]/2500, 0, 1), "slope": model.Clamp(v("slope"), 0, 1), "summer": v("warmest"), "winter": v("coldest"), "geothermal": 45 + 140*t.Divergence + 100*t.Convergence, "windStrength": model.Clamp(math.Hypot(v("windX"), v("windY"))/15, 0, 1)}
		if t.Divergence > .12 {
			values["boundary"] = 2
		} else if t.Convergence > .12 {
			values["boundary"] = 1
		}
		if len(marine) == n && marine[i] > 0 {
			values["ocean"] = 1
			values["waterBody"] = 1
			values["salinity"] = 35
			values["shelf"] = math.Exp(-math.Abs(e.Fields["elevation"][i]) / 350)
		}
		if len(lakes) == n && lakes[i] > 0 {
			values["lake"] = 1
			values["waterBody"] = 2
		}
		values["sediment"] = model.Clamp(.25+t.Divergence*.7+values["shelf"]*.5+math.Log1p(v("discharge"))*.04-values["slope"]*.4, 0, 1)
		values["permeability"] = model.Clamp(.15+.4*values["sediment"]+.25*t.Boundary, 0, 1)
		values["flow"] = -1
		if j := representative[i]; j >= 0 {
			values["drainageElevation"] = p.Grid.Fields["drainage"][j]
			at := int(p.Grid.Fields["flow"][j])
			if at < 0 && p.Emerged(j) {
				values["terminalBasin"] = 1
			}
			for steps := 0; at >= 0 && steps < len(owner); steps++ {
				if owner[at] != i {
					values["flow"] = float64(owner[at])
					break
				}
				at = int(p.Grid.Fields["flow"][at])
			}
		}
		for k, value := range values {
			if e.Fields[k] == nil {
				e.Fields[k] = make([]float64, n)
			}
			e.Fields[k][i] = value
		}
		if values["ocean"] > 0 {
			e.Fields["flow"][i] = -1
		}
	}
	// End-point columns describe the same meridian; connect their drainage
	// only through actual projected receivers, never by a new routing pass.
	waterBodies(e)
	e.Hydrology = &terrain.HydrologyState{Version: 1, Projection: "continuous-receivers/1", Units: "continuous river m3/s preserved separately; recharge/runoff are annual 1000 mm depth proxies", Springs: []terrain.HydroSpring{}, Reaches: []terrain.HydroReach{}, Wetlands: []terrain.HydroWetland{}, Basins: []terrain.HydroBasin{}, Coasts: []terrain.HydroCoast{}, Canals: []terrain.HydroCanal{}, Diagnostics: []terrain.HydroDiagnostic{}}
	for _, river := range p.Rivers {
		for j, pt := range river.Path {
			x := int(math.Round((model.Wrap(pt[0]) + math.Pi) / model.Tau * float64(w-1)))
			y := int(math.Round((.5 - pt[1]/math.Pi) * float64(h-1)))
			i := max(0, min(n-1, y*w+x))
			e.Fields["river"][i] = 1
			e.Fields["accumulation"][i] = math.Max(e.Fields["accumulation"][i], river.Discharge[j])
			e.Fields["catchmentArea"][i] = math.Max(e.Fields["catchmentArea"][i], river.CatchmentKM2)
			// Dry-season factor uses the physical monthly precipitation summaries;
			// it is an inferred floor, not a simulated daily or potable yield.
			dry := model.Clamp(p.Grid.At("rainDriest", pt[0], pt[1])*12/math.Max(1, p.Grid.At("precipitation", pt[0], pt[1])), .03, 1)
			e.Fields["dryDischarge"][i] = math.Max(e.Fields["dryDischarge"][i], river.Discharge[j]*dry)
		}
	}
	e.NaturalSource = Source(p)
	e.BuildGeologicalHistory()
	if err := ctx.Err(); err != nil {
		return err
	}
	groundwater(e)
	e.BuildNaturalResources()
	if err := ctx.Err(); err != nil {
		return err
	}
	return e.ValidateNaturalData()
}

func unit(seed, cell, salt uint64) float64 {
	x := seed ^ cell*0x9e3779b97f4a7c15 ^ salt*0xbf58476d1ce4e5b9
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return float64(x^(x>>31)) / float64(^uint64(0))
}

func waterBodies(e *terrain.Environment) {
	w, h := e.Options.Columns, e.Options.Rows
	n := w * h
	seen := make([]bool, n)
	e.WaterBodies = []terrain.WaterBody{}
	for i := 0; i < n; i++ {
		if seen[i] || e.Fields["waterBody"][i] == 0 {
			continue
		}
		marine := e.Fields["ocean"][i] > 0
		cells := []int{i}
		seen[i] = true
		for head := 0; head < len(cells); head++ {
			j := cells[head]
			for _, k := range []int{j - w, j + w, j/w*w + (j%w+w-1)%w, j/w*w + (j%w+1)%w} {
				if k >= 0 && k < n && !seen[k] && e.Fields["waterBody"][k] > 0 && (e.Fields["ocean"][k] > 0) == marine {
					seen[k] = true
					cells = append(cells, k)
				}
			}
		}
		id := len(e.WaterBodies) + 1
		kind := "lake"
		if marine {
			kind = "ocean"
		}
		body := terrain.WaterBody{ID: id, Kind: kind, Cells: cells}
		for _, j := range cells {
			e.Fields["waterBody"][j] = float64(id)
			body.MaxDepth = math.Max(body.MaxDepth, e.Fields["waterDepth"][j])
			body.Level = math.Max(body.Level, e.Fields["waterLevel"][j])
		}
		e.WaterBodies = append(e.WaterBodies, body)
	}
}

func groundwater(e *terrain.Environment) {
	n := len(e.Mask)
	e.Fields["groundwaterHead"] = make([]float64, n)
	for i, c := range e.Geology.Cells {
		perm := .08
		if c.Cover >= 0 {
			f := e.Geology.Formations[c.Cover]
			switch f.Rock {
			case "alluvium":
				perm = .75
			case "limestone", "marble":
				perm = .6
			case "sandstone":
				perm = .55
			case "mudstone", "evaporite":
				perm = .08
			}
		}
		if c.Fault >= 0 {
			perm = math.Max(perm, .45)
		}
		rain := e.Fields["precipitation"][i]
		recharge := math.Max(0, rain-250) * perm * .3 / 1000
		if e.Fields["ocean"][i] > 0 {
			recharge = 0
		}
		e.Fields["permeability"][i] = perm
		e.Fields["lithology"][i] = float64(c.Basement)
		e.Fields["recharge"][i] = recharge
		e.Fields["runoff"][i] = math.Max(0, rain/1000*.45-recharge)
		e.Fields["groundFlow"][i] = recharge
		e.Fields["waterTableDepth"][i] = math.Max(1, 60*(1-e.Fields["moisture"][i])*(1-perm*.5))
		if e.Fields["waterBody"][i] > 0 {
			e.Fields["waterTableDepth"][i] = 0
		}
		e.Fields["groundwaterHead"][i] = e.Fields["elevation"][i] - e.Fields["waterTableDepth"][i]
		if e.Fields["river"][i] > 0 && e.Fields["slope"][i] < .07 && e.Fields["moisture"][i] > .6 {
			e.Fields["wetland"][i] = .8
			e.Hydrology.Wetlands = append(e.Hydrology.Wetlands, terrain.HydroWetland{ID: fmt.Sprintf("%s/wetland/%d", e.NaturalSource[:16], i), Cells: []int{i}, Kind: "floodplain", Source: "saved-continuous-river"})
		}
	}
	// Groundwater follows the saved receiver graph as a reduced regional flow
	// proxy; infiltration is budgeted, rather than inferred from mountain height.
	degree := make([]int, n)
	order := []int{}
	for i := 0; i < n; i++ {
		j := int(e.Fields["flow"][i])
		if j >= 0 && j < n {
			degree[j]++
		}
	}
	for i, d := range degree {
		if d == 0 {
			order = append(order, i)
		}
	}
	for head := 0; head < len(order); head++ {
		i := order[head]
		j := int(e.Fields["flow"][i])
		if j >= 0 && j < n {
			e.Fields["groundFlow"][j] += e.Fields["groundFlow"][i] * .45
			// Potential confined head follows an actual supplied upstream route.
			// This is a regional pressure proxy, not a drilled artesian well.
			if e.Fields["groundFlow"][i] > .015 {
				distance := e.PointDistanceKM(float64(i%e.Options.Columns), float64(i/e.Options.Columns), float64(j%e.Options.Columns), float64(j/e.Options.Columns))
				e.Fields["groundwaterHead"][j] = math.Max(e.Fields["groundwaterHead"][j], e.Fields["groundwaterHead"][i]-distance*2)
			}
			degree[j]--
			if degree[j] == 0 {
				order = append(order, j)
			}
		}
	}
	for i, c := range e.Geology.Cells {
		g := e.Fields["groundFlow"][i]
		e.Fields["groundwater"][i] = model.Clamp(g, 0, 1)
		e.Fields["baseflow"][i] = g * .2
		if e.Fields["ocean"][i] > 0 || e.Fields["waterBody"][i] > 0 || g < .08 || c.Fault < 0 && c.Cover < 0 {
			continue
		}
		j := int(e.Fields["flow"][i])
		if j < 0 || j >= n {
			continue
		}
		// At a supplied contact where the downstream bed intersects the water
		// table, a regional emergence zone exists; no arbitrary hot spring roll.
		if e.Fields["elevation"][i]-e.Fields["elevation"][j] < e.Fields["waterTableDepth"][i] {
			continue
		}
		temp := math.Max(4, e.Fields["temperature"][i]) + e.Fields["geothermal"][i]*.18
		if e.Fields["geothermal"][i] < 55 {
			temp = math.Min(temp, 34)
		}
		origin := "aquifer-contact"
		if c.Fault >= 0 {
			origin = "fracture"
		}
		minerals := map[string]float64{"silica": 15}
		if c.Cover >= 0 {
			rock := e.Geology.Formations[c.Cover].Rock
			if rock == "limestone" {
				minerals["calcium"] = 80
				minerals["magnesium"] = 25
			}
			if rock == "evaporite" {
				minerals["sodium"] = 180
				minerals["sulfate"] = 120
			}
		}
		e.Hydrology.Springs = append(e.Hydrology.Springs, terrain.HydroSpring{ID: fmt.Sprintf("%s/spring/%d", e.NaturalSource[:16], i), Cell: i, Origin: origin, Discharge: math.Min(g*.2, 1), Temperature: temp, Minerals: minerals})
		e.Fields["spring"] = ensure(e.Fields["spring"], n)
		e.Fields["spring"][i] = g * .2
	}
}
func ensure(a []float64, n int) []float64 {
	if len(a) != n {
		return make([]float64, n)
	}
	return a
}
