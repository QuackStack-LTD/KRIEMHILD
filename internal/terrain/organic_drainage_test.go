package terrain

import (
	"math"
	"testing"
)

func TestOrganicDrainageNetworks(t *testing.T) {
	totalLength, totalJoins := 0., 0
	for _, seed := range []string{"KRIEMHILD", "1", "3", "river-basin"} {
		o := realismOptions(t, `{"columns":160,"rows":100}`)
		o.Seed = seed
		e := BuildEnvironment(o)
		top := e.Hydrology.Topology
		totalLength += e.Hydrology.Statistics.Lengths.Max
		totalJoins += top.Confluences
		t.Logf("%s length=%+v confluences=%d tributarySystems=%d splits=%d rejoin=%d outlets=%d coverage=%.1f", seed, e.Hydrology.Statistics.Lengths, top.Confluences, top.RiversWithTributaries, top.Splits, top.RejoiningBranches, top.DistributaryOutlets, e.Hydrology.Statistics.CoveragePercent)
		if diagnostics := e.ValidateHydrology(); len(diagnostics) > 0 {
			t.Fatal(diagnostics[:min(5, len(diagnostics))])
		}
		if top.Confluences < 35 || top.RejoiningBranches == 0 || top.DistributaryOutlets == 0 || e.Hydrology.Statistics.CoveragePercent > 25 {
			t.Fatal("fragmented or excessive drainage", seed, top)
		}
		model := NewDetailModel(e)
		features := map[string]DetailFeature{}
		for _, feature := range model.Features {
			features[feature.ID] = feature
		}
		worldJunctions := 0
		for _, reach := range e.Hydrology.Reaches {
			trunk := features[reach.ID]
			if trunk.Level > 0 || len(trunk.Path) == 0 {
				continue
			}
			incoming := 0
			for _, id := range reach.Upstream {
				branch := features[id]
				if branch.Level == 0 && len(branch.Path) > 0 {
					end := branch.Path[len(branch.Path)-1]
					if math.Hypot(end[0]-trunk.Path[0][0], end[1]-trunk.Path[0][1]) > 1e-7 {
						t.Fatal("rendered confluence disconnected", seed, id)
					}
					incoming++
				}
			}
			if incoming > 1 {
				worldJunctions++
			}
		}
		if worldJunctions < 8 {
			t.Fatal("world-scale tributaries hidden", seed, worldJunctions)
		}
	}
	// Original organic-continent regression averaged only 4025 km and 117
	// confluences across these four worlds. Require longer connected trunks,
	// without demanding the same course or length on every continent.
	if totalLength/4 < 4400 || totalJoins < 145 {
		t.Fatal("short independent coastal rivers displaced continental drainage", totalLength/4, totalJoins)
	}
}

func TestRiverMorphologyUsesGeographicGradient(t *testing.T) {
	o := realismOptions(t, `{"columns":16,"rows":16,"geography":{"map_coverage":"local","world_scale":200}}`)
	g, err := o.geographicScale()
	if err != nil {
		t.Fatal(err)
	}
	e := &Environment{Options: o, Geography: g}
	if grade := e.channelMorphologyGrade(8*16+7, 8*16+8, 100); grade > 11 {
		t.Fatal("gentle planetary floodplain classified as steep", grade)
	}
	o.Geography.WorldScale = .1
	e.Geography, _ = o.geographicScale()
	if grade := e.channelMorphologyGrade(8*16+7, 8*16+8, 100); grade != 100 {
		t.Fatal("steep local channel gained a false delta", grade)
	}
}
