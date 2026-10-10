package terrain

// Wide-water components separated by narrow straits define marine regions.
// All retain the same sea level and connected water body, so region names do
// not introduce physical walls into an ocean or its marginal seas.
func (e *Environment) buildMarineRegions() {
	n, w, h := len(e.Mask), e.Options.Columns, e.Options.Rows
	land := make([]int, n)
	for i := range land {
		if e.get("ocean", i) == 0 {
			land[i] = 1
		}
	}
	clearance := distance(land, w, h, 1)
	labels := make([]int, n)
	q := []int{}
	for i := range land {
		if land[i] > 0 || clearance[i] < 2 || labels[i] > 0 {
			continue
		}
		cells := []int{i}
		id := len(e.Hydrology.MarineRegions) + 1
		labels[i] = id
		edge := false
		for head := 0; head < len(cells); head++ {
			at := cells[head]
			if at%w == 0 || at%w == w-1 || at < w || at >= n-w {
				edge = true
			}
			for _, j := range nb(at, w, h) {
				if land[j] == 0 && clearance[j] >= 2 && labels[j] == 0 {
					labels[j] = id
					cells = append(cells, j)
				}
			}
		}
		kind := "sea"
		if edge {
			kind = "ocean"
		}
		e.Hydrology.MarineRegions = append(e.Hydrology.MarineRegions, MarineRegion{ID: e.hydroID(kind, i), Kind: kind})
		q = append(q, cells...)
	}
	if len(q) == 0 {
		return
	}
	for head := 0; head < len(q); head++ {
		i := q[head]
		for _, j := range nb(i, w, h) {
			if land[j] == 0 && labels[j] == 0 {
				labels[j] = labels[i]
				q = append(q, j)
			}
		}
	}
	linked := map[[2]int]bool{}
	for i, id := range labels {
		if id == 0 {
			continue
		}
		region := &e.Hydrology.MarineRegions[id-1]
		region.Cells = append(region.Cells, i)
		for _, j := range nb(i, w, h) {
			other := labels[j]
			if other > 0 && other != id && !linked[[2]int{id, other}] {
				linked[[2]int{id, other}] = true
				region.Connections = append(region.Connections, e.Hydrology.MarineRegions[other-1].ID)
			}
		}
	}
}
