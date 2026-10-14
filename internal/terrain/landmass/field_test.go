package landmass_test

import (
	"context"
	"kriemhild/internal/terrain/generate"
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
	"testing"
)

func TestPreparedFieldPreservesEveryValue(t *testing.T) {
	p, err := generate.Generate(context.Background(), model.DefaultSpec(), nil)
	if err != nil {
		t.Fatal(err)
	}
	f := landmass.NewField(p)
	for i := 0; i < 1000; i++ {
		lon, lat := float64(i)*.031-12, float64(i%99-49)*.03
		a, b, c := landmass.Bed(p, lon, lat)
		x, y, z := f.Bed(lon, lat)
		if a != x || b != y || c != z {
			t.Fatalf("prepared field changed coordinate %d", i)
		}
	}
}
