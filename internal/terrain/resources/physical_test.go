package resources

import (
	"context"
	"errors"
	"kriemhild/internal/terrain/model"
	"testing"
)

func TestSourceTracksPhysicalStateAndCancellation(t *testing.T) {
	p := &model.Planet{Spec: model.DefaultSpec(), Grid: model.Grid{Width: 2, Height: 2, Fields: map[string][]float64{"temperature": {1, 2, 3, 4}}}}
	initial := Source(p)
	p.Spec.TileSize = 512
	p.Diagnostics.Timings = map[string]float64{"render": 1}
	if Source(p) != initial {
		t.Fatal("render-only changes invalidated resource layer")
	}
	for _, change := range []func(){func() { p.Seed++ }, func() { p.SeaOffset++ }, func() { p.Grid.Width++ }, func() { p.Grid.Fields["temperature"][0]++ }} {
		old := Source(p)
		change()
		if Source(p) == old {
			t.Fatal("physical change did not invalidate resource layer")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Populate(ctx, p, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled work continued: %v", err)
	}
}
