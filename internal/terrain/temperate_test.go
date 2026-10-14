package terrain

import (
	"fmt"
	"testing"
)

// Exercise the actual default grid and both physical modes, not just the
// classifier with hand-picked warm/wet inputs. Temperate land must include
// usable lowlands and connected regions, not isolated favorable pixels.
func TestDefaultClimateDistribution(t *testing.T) {
	for _, realism := range []bool{false, true} {
		for _, seed := range []string{"KRIEMHILD", "river-basin", "temperate-coasts"} {
			t.Run(fmt.Sprintf("%s/realism=%t", seed, realism), func(t *testing.T) {
				o, err := DecodeEnvironment([]byte(`{"columns":160,"rows":100}`))
				if err != nil {
					t.Fatal(err)
				}
				o.Seed, o.Realism = seed, realism
				e := BuildEnvironment(o)
				counts := map[string]int{}
				land, temperate, mildLowland := 0, 0, 0
				mild := make([]bool, len(e.Mask))
				for i, cell := range e.Climate.Cells {
					if e.get("waterBody", i) != 0 {
						continue
					}
					land++
					kind := e.Climate.Zones[cell.Zone].Kind
					counts[kind]++
					if kind == "oceanic" || kind == "continental" || kind == "mediterranean" {
						temperate++
						if e.get("elevation", i) < 900 && e.get("temperature", i) > 5 && e.get("moisture", i) > .4 {
							mildLowland++
							mild[i] = true
						}
					}
				}
				largest := 0
				for start, yes := range mild {
					if !yes {
						continue
					}
					queue := []int{start}
					mild[start] = false
					for head := 0; head < len(queue); head++ {
						for _, j := range e.neighbors(queue[head]) {
							if mild[j] {
								mild[j] = false
								queue = append(queue, j)
							}
						}
					}
					largest = max(largest, len(queue))
				}
				t.Logf("temperate %.1f%%, mild lowland %.1f%%, largest mild region %d cells; zones %v", 100*float64(temperate)/float64(land), 100*float64(mildLowland)/float64(land), largest, counts)
				if land == 0 || float64(temperate)/float64(land) < .10 || float64(mildLowland)/float64(land) < .08 || largest < 25 {
					t.Fatal("default world lacks substantial connected temperate lowlands")
				}
				for _, kind := range []string{"desert", "steppe", "tundra", "oceanic", "continental", "mediterranean"} {
					if counts[kind] == 0 {
						t.Errorf("lost climate variety: no %s", kind)
					}
				}
				// Continental placement need not put humid land directly on the equator.
				if counts["rainforest"]+counts["monsoon"]+counts["tropical-seasonal"] == 0 {
					t.Error("lost tropical climate regimes")
				}
				if len(e.Hydrology.Diagnostics) != 0 {
					t.Fatal(e.Hydrology.Diagnostics)
				}
			})
		}
	}
}

func TestLowlandLatitudeTemperatureCalibration(t *testing.T) {
	for _, tc := range []struct{ latitude, min, max float64 }{
		{0, 24, 29}, {45, 13, 19}, {55, 3, 10}, {80, -22, -10},
	} {
		t.Run(fmt.Sprint(tc.latitude), func(t *testing.T) {
			o, err := DecodeEnvironment([]byte(`{"columns":16,"rows":16}`))
			if err != nil {
				t.Fatal(err)
			}
			o.LatitudeNorth, o.LatitudeSouth = tc.latitude+1, tc.latitude-1
			// Authored heights use four metres per height unit.
			o.Heights = make([]float64, o.Columns*o.Rows)
			for i := range o.Heights {
				o.Heights[i] = 25
			}
			e := BuildEnvironment(o)
			mean := 0.
			for _, temperature := range e.Fields["temperature"] {
				mean += temperature / float64(len(e.Mask))
			}
			if mean < tc.min || mean > tc.max {
				t.Fatalf("100m lowlands at %.0f degrees: mean %.1f outside %.0f..%.0f C", tc.latitude, mean, tc.min, tc.max)
			}
		})
	}
}
