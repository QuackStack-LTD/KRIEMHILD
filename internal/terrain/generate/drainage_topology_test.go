package generate

import (
	"context"
	"kriemhild/internal/terrain/hydrology"
	"kriemhild/internal/terrain/model"
	"testing"
)

// The controlled terrain has a descending broad plain and a depression. The
// assertions are physical/topological rules, not a prescribed cosmetic route.
func TestSyntheticPlainAndLakeDrainage(t *testing.T) {
	p := &model.Planet{Spec: model.DefaultSpec(), Grid: model.Grid{Width: 16, Height: 8, Fields: map[string][]float64{}}}
	p.Spec.Version = "continuous-2"
	g := &p.Grid
	n := g.Width * g.Height
	for _, name := range []string{"bed", "flow", "drainage", "discharge", "coast", "precipitation", "pet", "moisture", "lake", "temperature", "roughness", "soil"} {
		g.Fields[name] = make([]float64, n)
	}
	for i := 0; i < n; i++ {
		x, y := i%g.Width, i/g.Width
		g.Fields["flow"][i] = -1
		g.Fields["bed"][i] = float64(1000 - x*40 + (y-4)*(y-4)*20)
		g.Fields["coast"][i] = 1
		g.Fields["precipitation"][i] = 1600
		g.Fields["pet"][i] = 600
		g.Fields["moisture"][i] = .8
		if x == 15 {
			g.Fields["bed"][i] = -100
			g.Fields["coast"][i] = -1
		}
	}
	g.Fields["bed"][4*16+8] = 100
	if err := drainage(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if len(p.Rivers) == 0 || len(p.Lakes) == 0 {
		t.Fatal("no drainage/lake on controlled wet terrain")
	}
	for i, f := range g.Fields["flow"] {
		if j := int(f); j >= 0 && p.Emerged(i) && g.Fields["drainage"][i] <= g.Fields["drainage"][j] {
			t.Fatal("non-descending drainage edge")
		}
	}
	graph := hydrology.Build(p)
	for _, code := range []string{"cycle", "disconnected-reach", "mismatched-confluence", "uphill-flow", "invalid-water-depth"} {
		if graph.Counts[code] != 0 {
			t.Fatal(code, graph.Counts[code])
		}
	}
}
