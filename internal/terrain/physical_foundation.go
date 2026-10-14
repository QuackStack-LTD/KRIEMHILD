package terrain

import "math"

func (o EnvironmentOptions) lapseRate() float64 {
	if o.LapseRate == 0 {
		return 6.5
	}
	return o.LapseRate
}

// Crust fields are provenance for newly generated surfaces, not a reconstruction
// imposed on imported heights or an assertion of a simulated geological past.
func (e *Environment) buildCrustFoundation() {
	for _, name := range []string{"crustType", "crustAge", "crustThickness", "convergence", "extension"} {
		e.Fields[name] = make([]float64, len(e.Mask))
	}
	ridge := make([]int, len(e.Mask))
	for i := range ridge {
		_, _, da, db, convergence, _ := e.platesAt(float64(i%e.Options.Columns), float64(i/e.Options.Columns))
		proximity := math.Exp(-math.Pow((math.Sqrt(db)-math.Sqrt(da))/3, 2))
		e.set("convergence", i, math.Max(0, convergence)*proximity)
		e.set("extension", i, math.Max(0, -convergence)*proximity)
		if convergence < -.2 && proximity > .4 {
			ridge[i] = 1
		}
	}
	distance := e.gridDistance(ridge, 1)
	for i, land := range e.Mask {
		age := math.Min(180, 5+distance[i]*8)
		thickness := 7.
		if land != 0 {
			e.set("crustType", i, 1)
			plate := int(e.get("plate", i))
			age = 700 + 2200*detailHash(Seed(e.Options.Seed), plate, 0, 843)
			thickness = 32 + 22*e.get("convergence", i) - 8*e.get("extension", i)
		}
		e.set("crustAge", i, age)
		e.set("crustThickness", i, thickness)
	}
}

// Wind-driven, basin-constrained thermal-advection proxy. Trace only connected
// ocean, then carry its coastal temperature anomaly a short distance inland.
// It deliberately makes no claim to solve full ocean fluid dynamics.
func (e *Environment) buildOceanCurrents() {
	w, h := e.Options.Columns, e.Options.Rows
	for _, name := range []string{"current", "currentX", "currentY"} {
		e.Fields[name] = make([]float64, len(e.Mask))
	}
	baseline := func(lat float64) float64 { return 28 - 48*math.Pow(math.Sin(math.Abs(lat)*math.Pi/180), 4) }
	queue := []int{}
	visited := make([]bool, len(e.Mask))
	for i, land := range e.Mask {
		if land != 0 {
			continue
		}
		queue = append(queue, i)
		visited[i] = true
		lat := e.cellLatitude(i)
		ab := math.Abs(lat)
		zonal := -1.
		if ab >= 30 && ab < 60 {
			zonal = 1
		}
		meridional := .45
		if lat < 0 {
			meridional = -meridional
		}
		// Boundary currents turn where zonal flow meets the continental coast.
		nx := i%w + int(zonal)
		if e.Geography != nil && e.Geography.WrapX {
			nx = (nx + w) % w
		}
		if nx < 0 || nx >= w || e.Mask[i/w*w+nx] != 0 {
			zonal *= .25
			meridional = -meridional
		}
		e.set("currentX", i, zonal)
		e.set("currentY", i, meridional)
		source := i
		steps := max(2, h/12)
		for step := 1; step <= steps; step++ {
			x := int(round(float64(i%w) - zonal*float64(step)))
			y := int(round(float64(i/w) - meridional*float64(step)))
			if e.Geography != nil && e.Geography.WrapX {
				x = (x%w + w) % w
			}
			if x < 0 || x >= w || y < 0 || y >= h || e.Mask[y*w+x] != 0 {
				break
			}
			source = y*w + x
		}
		e.set("current", i, clamp((baseline(e.cellLatitude(source))-baseline(lat))*.3, -4, 4))
	}
	for head := 0; head < len(queue); head++ {
		i := queue[head]
		for _, j := range e.neighbors(i) {
			if visited[j] {
				continue
			}
			visited[j] = true
			queue = append(queue, j)
			e.set("current", j, e.get("current", i)*.7)
		}
	}
}
