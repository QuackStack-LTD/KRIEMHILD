package landmass

import (
	"context"
	"fmt"
	"kriemhild/internal/terrain/model"
	"math"
)

type Component struct {
	ID    string          `json:"id"`
	Paths [][]model.Point `json:"paths"` // normalized map coordinates; longitude wraps
	Cells int             `json:"cells"`
}

// Connected components follow the saved surface triangles, including their
// diagonal and longitude seam. Generated mass centres are not connectivity.
// Temporary arrays stay below 64 MiB at the supported 4096 x 2048 source size.
func Components(ctx context.Context, w, h int, values []float32) ([]Component, error) {
	if w < 2 || h < 2 || w*h != len(values) || w > 4096 || h > 2048 {
		return nil, fmt.Errorf("invalid land surface")
	}
	labels := make([]int32, len(values))
	queue := make([]int32, 0, len(values))
	result := []Component{}
	for i, v := range values {
		if i%w == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if v <= 0 || labels[i] != 0 {
			continue
		}
		id := int32(len(result) + 1)
		if id > 10000 {
			return nil, fmt.Errorf("land selection exceeds the 10000 component budget")
		}
		queue = append(queue[:0], int32(i))
		labels[i] = id
		for head := 0; head < len(queue); head++ {
			if head%8192 == 0 && ctx.Err() != nil {
				return nil, ctx.Err()
			}
			j := int(queue[head])
			x, y := j%w, j/w
			for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}, {1, -1}, {-1, 1}} {
				yy := y + d[1]
				if yy < 0 || yy >= h {
					continue
				}
				k := yy*w + (x+d[0]+w)%w
				if values[k] > 0 && labels[k] == 0 {
					labels[k] = id
					queue = append(queue, int32(k))
				}
			}
		}
		result = append(result, Component{ID: fmt.Sprintf("land-%d", i), Cells: len(queue), Paths: [][]model.Point{}})
	}
	type edge struct{ a, b int }
	points := map[edge]model.Point{}
	links := map[edge][]edge{}
	owners := map[edge]int32{}
	order := []edge{}
	// Trace a finite map, with a duplicate seam column and a wet exterior.
	// Connectivity still wraps above: seam fragments share one component ID.
	// This closes polar caps and all-land worlds instead of losing their area.
	stride := w + 3
	sample := func(node int) (float64, int32) {
		x, y := node%stride-1, node/stride-1
		xx, yy := max(0, min(w, x))%w, max(0, min(h-1, y))
		i := yy*w + xx
		if x < 0 || x > w || y < 0 || y >= h {
			return -max(.001, math.Abs(float64(values[i]))), 0
		}
		return float64(values[i]), labels[i]
	}
	for y := -1; y < h; y++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		for x := -1; x <= w; x++ {
			a, b := (y+1)*stride+x+1, (y+1)*stride+x+2
			for _, ids := range [][3]int{{a, b, a + stride}, {b, b + stride, a + stride}} {
				crossing := []edge{}
				owner := int32(0)
				var height [3]float64
				for k, i := range ids {
					v, label := sample(i)
					height[k] = v
					if label > 0 {
						owner = label
					}
				}
				for k := 0; k < 3; k++ {
					u, v := ids[k], ids[(k+1)%3]
					cu, cv := height[k], height[(k+1)%3]
					if (cu > 0) == (cv > 0) {
						continue
					}
					e := edge{min(u, v), max(u, v)}
					if _, ok := points[e]; !ok {
						if len(points) >= 300000 {
							return nil, fmt.Errorf("land selection exceeds the 300000 coastline vertex budget")
						}
						px, py := float64(u%stride-1)/float64(w)*2, (float64(u/stride)-.5)/float64(h)
						qx, qy := float64(v%stride-1)/float64(w)*2, (float64(v/stride)-.5)/float64(h)
						dx := qx - px
						t := cu / (cu - cv)
						xx := px + dx*t
						points[e] = model.Point{xx, py + (qy-py)*t}
						owners[e] = owner
						order = append(order, e)
					}
					crossing = append(crossing, e)
				}
				if len(crossing) == 2 {
					u, v := crossing[0], crossing[1]
					links[u] = append(links[u], v)
					links[v] = append(links[v], u)
				}
			}
		}
	}
	seen := map[edge]bool{}
	for _, start := range order {
		if seen[start] {
			continue
		}
		path := []model.Point{}
		current := start
		for !seen[current] {
			seen[current] = true
			path = append(path, points[current])
			next := start
			for _, e := range links[current] {
				if !seen[e] {
					next = e
					break
				}
			}
			current = next
		}
		if len(path) > 2 {
			path = append(path, points[current])
			owner := owners[start]
			if owner > 0 {
				result[owner-1].Paths = append(result[owner-1].Paths, path)
			}
		}
	}
	return result, nil
}
