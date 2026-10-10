package terrain

import (
	"container/heap"
	"math"
	"sort"
)

type floodCell struct {
	cell   int
	height float64
}
type floodQueue []floodCell

func (q floodQueue) Len() int { return len(q) }
func (q floodQueue) Less(i, j int) bool {
	if q[i].height == q[j].height {
		return q[i].cell < q[j].cell
	}
	return q[i].height < q[j].height
}
func (q floodQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *floodQueue) Push(v any)   { *q = append(*q, v.(floodCell)) }
func (q *floodQueue) Pop() any     { a := *q; v := a[len(a)-1]; *q = a[:len(a)-1]; return v }

func (e *Environment) simulateHydrology() {
	n, w, h := len(e.Mask), e.Options.Columns, e.Options.Rows
	f, set := e.get, e.set
	filled := make([]float64, n)
	seen := make([]bool, n)
	rank := make([]int, n)
	queue := &floodQueue{}
	for i := 0; i < n; i++ {
		set("flow", i, -1)
		if f("ocean", i) > 0 || i%w == 0 || i%w == w-1 || i < w || i >= n-w {
			seen[i] = true
			filled[i] = math.Max(f("elevation", i), f("waterLevel", i))
			heap.Push(queue, floodCell{i, filled[i]})
		}
	}
	sequence := 0
	for queue.Len() > 0 {
		v := heap.Pop(queue).(floodCell)
		rank[v.cell] = sequence
		sequence++
		for _, j := range nb(v.cell, w, h) {
			if seen[j] {
				continue
			}
			seen[j] = true
			filled[j] = math.Max(f("elevation", j), v.height+.001)
			set("flow", j, float64(v.cell))
			heap.Push(queue, floodCell{j, filled[j]})
		}
	}
	for i := 0; i < n; i++ {
		if f("waterBody", i) > 0 {
			continue
		}
		j := e.steepestDrain(i, int(f("flow", i)), rank, func(k int) float64 { return filled[k] })
		if j >= 0 {
			set("flow", i, float64(j))
		}
	}
	e.accumulateWater()
	// Group real depressions at a common spill surface. The routing surface is
	// provisional only; it never raises the actual land or floods every sink.
	used := make([]bool, n)
	for i := 0; i < n; i++ {
		if used[i] || f("ocean", i) > 0 || filled[i]-f("elevation", i) < 4 {
			continue
		}
		cells := []int{i}
		used[i] = true
		floor, spill := f("elevation", i), filled[i]
		lowest := i
		for head := 0; head < len(cells); head++ {
			at := cells[head]
			floor = math.Min(floor, f("elevation", at))
			spill = math.Min(spill, filled[at])
			if f("elevation", at) < f("elevation", lowest) {
				lowest = at
			}
			for _, j := range nb(at, w, h) {
				if !used[j] && f("ocean", j) == 0 && filled[j]-f("elevation", j) > 4 && math.Abs(filled[j]-filled[i]) < 1 {
					used[j] = true
					cells = append(cells, j)
				}
			}
		}
		origin := "drainage-basin"
		if f("glacier", lowest) > .1 || f("snow", lowest) > .4 {
			origin = "glacial"
		} else if f("volcano", lowest) > .55 {
			origin = "volcanic-caldera"
		} else if f("tectonicStress", lowest) > .3 {
			origin = "tectonic"
		}
		if origin == "drainage-basin" && f("slope", lowest) < .06 && f("elevation", lowest) < 600 {
			for _, bank := range e.drainageNeighbors(lowest) {
				to := int(f("flow", bank))
				if to < 0 || f("accumulation", bank) < 4 || math.Abs(f("elevation", bank)-floor) > 20 {
					continue
				}
				for _, from := range e.drainageNeighbors(bank) {
					if int(f("flow", from)) != bank {
						continue
					}
					ax, ay := float64(bank%w-from%w), float64(bank/w-from/w)
					bx, by := float64(to%w-bank%w), float64(to/w-bank/w)
					if math.Abs(ax*by-ay*bx) > .5 {
						origin = "floodplain-oxbow"
						break
					}
				}
			}
		}
		e.Hydrology.Basins = append(e.Hydrology.Basins, HydroBasin{ID: e.hydroID("basin", lowest), Cells: cells, Floor: floor, Spill: spill, Level: floor, Origin: origin, Outlet: -1, Regime: "dry"})
	}
	sort.SliceStable(e.Hydrology.Basins, func(i, j int) bool { return e.Hydrology.Basins[i].Spill > e.Hydrology.Basins[j].Spill })
	for b := range e.Hydrology.Basins {
		for _, i := range e.Hydrology.Basins[b].Cells {
			set("basin", i, float64(b+1))
		}
	}
	for b := range e.Hydrology.Basins {
		basin := &e.Hydrology.Basins[b]
		supply := 0.
		root := basin.Cells[0]
		for _, i := range basin.Cells {
			if rank[i] < rank[root] {
				root = i
			}
			supply += f("runoff", i)
			for _, j := range e.drainageNeighbors(i) {
				if int(f("flow", j)) == i && int(f("basin", j)) != b+1 {
					supply += f("accumulation", j)
				}
			}
		}
		costs := func(level float64) (loss, rain, evap, seep float64) {
			for _, i := range basin.Cells {
				coverage := smooth(clamp((level-f("elevation", i))/4, 0, 1))
				r := f("precipitation", i) / 1000 * coverage
				lakeTemperature := f("temperature", i) - math.Max(0, level-f("elevation", i))*.0065
				v := math.Max(.12, (lakeTemperature+25)*.028) * coverage
				s := f("permeability", i) * .18 * coverage
				rain += r
				evap += v
				seep += s
				loss += v + s - r + f("runoff", i)*coverage
			}
			return
		}
		capacity, _, _, _ := costs(basin.Spill)
		level := basin.Spill
		if supply < capacity {
			lo, hi := basin.Floor, basin.Spill
			for k := 0; k < 40; k++ {
				mid := (lo + hi) / 2
				cost, _, _, _ := costs(mid)
				if cost > supply {
					hi = mid
				} else {
					lo = mid
				}
			}
			level = (lo + hi) / 2
		}
		// Small lowland and spring-fed basins can be real lakes too. Require
		// sustained supply or geological support for a single-cell footprint.
		if len(basin.Cells) == 1 && basin.Origin == "drainage-basin" && supply < .5 && f("groundFlow", root) < .2 || basin.Spill-basin.Floor < 4 || level-basin.Floor < 1.5 || f("aridity", root) > .55 && level-basin.Floor < 4 {
			level = basin.Floor
		}
		if basin.Origin == "drainage-basin" && f("groundFlow", root) > .2 && f("groundFlow", root) > supply*.3 {
			basin.Origin = "spring-fed"
		}
		basin.Level = f32(level)
		loss, rain, evap, seep := costs(level)
		landYield := 0.
		for _, i := range basin.Cells {
			landYield += f("runoff", i) * smooth(clamp((level-f("elevation", i))/4, 0, 1))
		}
		basin.Budget = WaterBudget{Inflow: math.Max(0, supply-landYield), Rain: rain, Evaporation: evap, Infiltration: seep, Outflow: math.Max(0, supply-loss)}
		if level == basin.Floor {
			basin.Budget.Infiltration = basin.Budget.Inflow
			basin.Budget.Outflow = 0
		}
		body := WaterBody{ID: len(e.WaterBodies) + 1, Kind: "lake", Level: basin.Level}
		closed := level < basin.Spill-.001
		if closed {
			basin.Budget.Outflow = 0
			basin.Regime = "seasonal"
			basin.Salinity = clamp(f("aridity", root)*110, 0, 150)
		} else {
			basin.Regime = "perennial"
			basin.Outlet = root
		}
		for _, i := range basin.Cells {
			if f("elevation", i) < level-.5 {
				e.Mask[i] = 0
				set("lake", i, 1)
				set("waterBody", i, float64(body.ID))
				set("waterLevel", i, basin.Level)
				set("waterDepth", i, basin.Level-f("elevation", i))
				set("bathymetry", i, f("waterDepth", i))
				set("salinity", i, basin.Salinity)
				body.Cells = append(body.Cells, i)
				body.MaxDepth = math.Max(body.MaxDepth, f("waterDepth", i))
			}
		}
		if len(body.Cells) > 0 {
			remaining := map[int]bool{}
			for _, i := range body.Cells {
				remaining[i] = true
			}
			for _, start := range body.Cells {
				if !remaining[start] {
					continue
				}
				delete(remaining, start)
				cells := []int{start}
				for head := 0; head < len(cells); head++ {
					for _, j := range nb(cells[head], w, h) {
						if remaining[j] {
							delete(remaining, j)
							cells = append(cells, j)
						}
					}
				}
				part := WaterBody{ID: len(e.WaterBodies) + 1, Kind: "lake", Level: body.Level, Cells: cells}
				for _, i := range cells {
					set("waterBody", i, float64(part.ID))
					part.MaxDepth = math.Max(part.MaxDepth, f("waterDepth", i))
				}
				e.WaterBodies = append(e.WaterBodies, part)
				basin.WaterBodies = append(basin.WaterBodies, part.ID)
			}
			basin.WaterBody = basin.WaterBodies[0]
			if closed {
				basin.Regime = "endorheic"
			}
		}
		// Lake-covered fractions receive precipitation directly. Do not count
		// both the previous land yield and that rain in downstream discharge.
		for _, i := range basin.Cells {
			coverage := smooth(clamp((level-f("elevation", i))/4, 0, 1))
			set("runoff", i, f("runoff", i)*(1-coverage)+f("precipitation", i)/1000*coverage)
			set("dryRunoff", i, math.Min(f("runoff", i), f("dryRunoff", i)*(1-coverage)+f("precipitation", i)/1000*coverage*.35))
		}
		set("hydroLoss", root, basin.Budget.Evaporation+basin.Budget.Infiltration)
		if closed {
			set("hydroLoss", root, basin.Budget.Inflow+basin.Budget.Rain)
		}

		e.accumulateWater()
	}
	// Reconcile the provisional graph with actual terrain and supported water.
	// Land cannot use an uphill spill route simply because priority flood found it.
	for i := 0; i < n; i++ {
		if f("waterBody", i) > 0 {
			continue
		}
		j := int(f("flow", i))
		surface := func(k int) float64 {
			if f("waterBody", k) > 0 {
				return f("waterLevel", k)
			}
			return f("elevation", k)
		}
		best := e.steepestDrain(i, j, rank, surface)
		set("flow", i, float64(best))
	}
	// Each lake has a connected internal routing tree and exactly one natural
	// spill outlet, or a closed storage root. Multiple incoming rivers are kept.
	for b := range e.Hydrology.Basins {
		basin := &e.Hydrology.Basins[b]
		if basin.WaterBody == 0 {
			continue
		}
		wantsOutlet := basin.Outlet >= 0
		basin.Outlet = -1
		wetArea := 0.
		for _, bodyID := range basin.WaterBodies {
			for _, i := range e.WaterBodies[bodyID-1].Cells {
				wetArea += smooth(clamp((basin.Level-f("elevation", i))/4, 0, 1))
			}
		}
		for _, bodyID := range basin.WaterBodies {
			body := e.WaterBodies[bodyID-1]
			root := body.Cells[0]
			exit := -1
			bestRank := n + 1
			for _, i := range body.Cells {
				if bestRank == n+1 && rank[i] < rank[root] {
					root = i
				}
				if wantsOutlet {
					for _, j := range nb(i, w, h) {
						if int(f("waterBody", j)) == body.ID {
							continue
						}
						z := f("elevation", j)
						if f("waterBody", j) > 0 {
							z = f("waterLevel", j)
						}
						if z <= basin.Level && rank[j] < bestRank {
							root, exit, bestRank = i, j, rank[j]
						}
					}
				}
			}
			set("flow", root, float64(exit))
			if exit >= 0 {
				if basin.Outlet < 0 {
					basin.Outlet = root
				}
				basin.Outlets = append(basin.Outlets, root)
			}
			visited := map[int]bool{root: true}
			q := []int{root}
			for head := 0; head < len(q); head++ {
				for _, j := range nb(q[head], w, h) {
					if !visited[j] && int(f("waterBody", j)) == body.ID {
						visited[j] = true
						q = append(q, j)
						set("flow", j, float64(q[head]))
					}
				}
			}
			for _, i := range body.Cells {
				set("hydroLoss", i, 0)
			}
			if exit >= 0 {
				area := 0.
				for _, i := range body.Cells {
					area += smooth(clamp((basin.Level-f("elevation", i))/4, 0, 1))
				}
				set("hydroLoss", root, (basin.Budget.Evaporation+basin.Budget.Infiltration)*area/wetArea)
			}
		}

	}
	for i := 0; i < n; i++ {
		if f("ocean", i) > 0 {
			set("flow", i, -1)
		}
		if f("flow", i) < 0 && f("waterBody", i) == 0 {
			set("terminalBasin", i, 1)
		}
		if f("waterBody", i) == 0 {
			set("hydroLoss", i, 0)
		}
	}
	for i := 0; i < n; i++ {
		if f("lake", i) > 0 {
			old := f("temperature", i)
			temperature := old - f("waterDepth", i)*.0065
			set("temperature", i, temperature)
			set("summer", i, temperature+(f("summer", i)-old)*.45)
			set("winter", i, temperature+(f("winter", i)-old)*.45)
			set("moisture", i, 1)
			set("aridity", i, 0)
		}
	}
	order := e.accumulateWater()
	// Preserve flat tie-breaking for indexing, but record actual surfaces at
	// every step. The tiny water/flat offset is below 1 cm, not an invented ridge.
	for k := len(order) - 1; k >= 0; k-- {
		i := order[k]
		z := f("elevation", i)
		if f("waterBody", i) > 0 {
			z = f("waterLevel", i)
		}
		if j := int(f("flow", i)); j >= 0 {
			z = math.Max(z, f("drainageElevation", j)+.0001)
		}
		set("drainageElevation", i, z)
	}
}
