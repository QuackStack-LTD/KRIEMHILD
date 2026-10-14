package terrain

import (
	"math"
	"testing"
)

func TestMountainSummitVariation(t *testing.T) {
	// Measure actual generated summits, including their supporting terrain.
	for _, seed := range []string{"KRIEMHILD", "1", "2", "3"} {
		o, _ := DecodeEnvironment([]byte(`{"columns":160,"rows":100}`))
		o.Seed = seed
		e := BuildEnvironment(o)
		lo, hi, count := math.Inf(1), 0., 0
		for i, z := range e.Fields["elevation"] {
			if z < 2000 {
				continue
			}
			peak := true
			for _, j := range nb(i, o.Columns, o.Rows) {
				if e.get("elevation", j) >= z {
					peak = false
				}
				if e.get("elevation", j) < z-1104 {
					t.Fatal("unsupported summit")
				}
			}
			if peak {
				count++
				lo = math.Min(lo, z)
				hi = math.Max(hi, z)
			}
		}
		if count < 3 || hi-lo < 750 {
			t.Fatalf("uniform/absent summits: %s count=%d range=%.0f", seed, count, hi-lo)
		}
	}
	if !(mountainCeiling(9000) < mountainCeiling(10000) && mountainCeiling(10000) < 8000) {
		t.Fatal("high summits flatten at ceiling")
	}
	if round(mountainCeiling(20000)/4) == round(mountainCeiling(24000)/4) {
		t.Fatal("high relief summits collapse to the same quantized height")
	}
}

func TestPhysicalMountainRanges(t *testing.T) {
	for _, realism := range []bool{false, true} {
		for _, seed := range []string{"KRIEMHILD", "1", "2", "3"} {
			o, err := DecodeEnvironment([]byte(`{"columns":160,"rows":100}`))
			if err != nil {
				t.Fatal(err)
			}
			o.Realism = realism
			o.Seed = seed
			e := BuildEnvironment(o)
			seen := make([]bool, len(e.Mask))
			peak, largest, count := 0., 0, 0
			for i, z := range e.Fields["elevation"] {
				if z > peak {
					peak = z
				}
				if z < 1800 || seen[i] {
					continue
				}
				q := []int{i}
				seen[i] = true
				for head := 0; head < len(q); head++ {
					for _, j := range nb(q[head], o.Columns, o.Rows) {
						if !seen[j] && e.get("elevation", j) >= 1800 {
							seen[j] = true
							q = append(q, j)
						}
					}
				}
				count += len(q)
				largest = max(largest, len(q))
			}
			t.Logf("realism=%v seed=%s peak=%.0fm mountainCells=%d largestRange=%d", realism, seed, peak, count, largest)
			if peak < 3000 || largest < 20 {
				t.Fatal("mountains do not form prominent connected ranges")
			}
			low := o
			low.Ruggedness = 30
			le := BuildEnvironment(low)
			lowHigh := 0
			for _, z := range le.Fields["elevation"] {
				if z >= 1800 {
					lowHigh++
				}
			}
			if lowHigh >= count {
				t.Fatal("relief control does not change mountain extent")
			}
		}
	}
}
