package terrain

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

// Count connected landmasses; diagonal contact counts as connected, so tiny
// channels cannot artificially inflate the number of continents in this test.
func landmasses(e *Environment) []int {
	w, h := e.Options.Columns, e.Options.Rows
	seen := make([]bool, w*h)
	sizes := []int{}
	for c, land := range e.Mask {
		if land == 0 || seen[c] {
			continue
		}
		q := []int{c}
		seen[c] = true
		for head := 0; head < len(q); head++ {
			i := q[head]
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					x, y := i%w+dx, i/w+dy
					if e.Geography != nil && e.Geography.WrapX {
						x = (x + w) % w
					}
					if x < 0 || x >= w || y < 0 || y >= h {
						continue
					}
					j := y*w + x
					if !seen[j] && e.Mask[j] != 0 {
						seen[j] = true
						q = append(q, j)
					}
				}
			}
		}
		sizes = append(sizes, len(q))
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sizes)))
	return sizes
}

func TestPhysicalLandDistribution(t *testing.T) {
	for _, seed := range []string{"KRIEMHILD", "1", "2", "3", "4", "5", "6", "7", "8", "9"} {
		o, err := DecodeEnvironment([]byte(fmt.Sprintf(`{"columns":160,"rows":100,"seed":%q}`, seed)))
		if err != nil {
			t.Fatal(err)
		}
		e := BuildEnvironment(o)
		sizes := landmasses(e)
		t.Logf("seed=%s landmasses=%v", seed, sizes)
		major := 0
		total := 0
		for _, size := range sizes {
			total += size
			if size >= e.Options.Columns*e.Options.Rows*3/100 {
				major++
			}
		}
		if major < 2 {
			t.Errorf("seed %s produced only %d substantial landmasses: %v", seed, major, sizes)
		}
		// Lakes remove land and exposed seamounts/closed ocean-disconnected
		// basins add it after the initial continental-area target is selected.
		if abs(total-6720) > 480 {
			t.Errorf("land coverage too far from target: got %d cells, target 42%% of 16000", total)
		}
		if e.Geography == nil || !e.Geography.WrapX {
			t.Fatal("default planet lost longitude continuity")
		}
		for y := 0; y < o.Rows; y++ {
			left := y * o.Columns
			right := left + o.Columns - 1
			connected := false
			for _, j := range e.neighbors(left) {
				if j == right {
					connected = true
				}
			}
			if !connected {
				t.Fatal("longitude seam split the world")
			}
		}

	}
}

func TestPhysicalDeterminismAndPointScale(t *testing.T) {
	o, _ := DecodeEnvironment([]byte(`{"columns":160,"rows":100,"seed":"KRIEMHILD","continentCount":8}`))
	a, b := BuildEnvironment(o), BuildEnvironment(o)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("identical settings are not deterministic")
	}
	o.ContinentCount = 40
	c := BuildEnvironment(o)
	if reflect.DeepEqual(a.Heights, c.Heights) {
		t.Fatal("point count does not affect geography")
	}
	if len(landmasses(c)) <= len(landmasses(a)) {
		t.Fatalf("more points should produce smaller regions in this regression world: %v vs %v", landmasses(a), landmasses(c))
	}
}
