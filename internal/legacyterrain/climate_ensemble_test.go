package terrain

import "testing"

// Broad preset bounds are ensemble checks, not quotas imposed on each world.
// A dominant continent may be drier than scattered maritime fragments.
func TestEarthLikeBiomeEnsembleBounds(t *testing.T) {
	seeds := []string{"KRIEMHILD", "1", "2", "3", "5", "7", "11", "19"}
	totals := map[string]float64{}
	equator, subtropic, storm := 0., 0., 0.
	for _, seed := range seeds {
		o := realismOptions(t, `{"columns":160,"rows":100,"landPercent":42}`)
		o.Seed = seed
		e := BuildEnvironment(o)
		r := e.ClimateDiagnostics()
		for kind, share := range r.BiomeShares {
			totals[kind] += share / float64(len(seeds))
		}
		equator += r.Bands[0].DryPercent / float64(len(seeds))
		subtropic += r.Bands[2].DryPercent / float64(len(seeds))
		storm += r.Bands[4].DryPercent / float64(len(seeds))
		if len(r.DryComponents) > 0 && float64(r.DryComponents[0])/float64(r.LandCells) > .65 {
			t.Error("single dryland region dominates the world", seed, r.DryComponents[0], r.LandCells)
		}
		if r.SmallDryCells > max(25, (r.Kinds["desert"]+r.Kinds["steppe"])/10) {
			t.Error("drylands fragment into isolated pixels", seed)
		}
		t.Log(seed, r.BiomeShares, r.LargestBiomeRegions)
	}
	for kind, bounds := range map[string][2]float64{"desert": {10, 50}, "semiarid": {3, 30}, "forest": {15, 60}, "grassland-woodland": {5, 40}, "cold": {1, 35}} {
		if totals[kind] < bounds[0] || totals[kind] > bounds[1] {
			t.Errorf("Earth-like ensemble %s %.1f%% outside %v", kind, totals[kind], bounds)
		}
	}
	if totals["desert"]+totals["semiarid"] > 60 || equator > 55 || storm > 25 || subtropic < equator+20 || subtropic < storm+30 {
		t.Fatal("circulation or world habitability regressed", totals, equator, subtropic, storm)
	}
}
