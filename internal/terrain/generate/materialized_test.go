package generate_test

import (
	"context"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/terrain/generate"
	"kriemhild/internal/terrain/model"
	"math"
	"reflect"
	"testing"
)

func TestCompletedSurfaceAndBasinNetwork(t *testing.T) {
	spec := model.DefaultSpec()
	spec.SurfaceWidth = 1024
	p, err := generate.Generate(context.Background(), spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Surface == nil || len(p.Surface.Bed) != 1024*512 || len(p.Surface.Coasts) == 0 {
		t.Fatal("ready world lacks whole-world surface or shoreline")
	}
	checked := 0
	for _, shore := range p.Surface.Coasts {
		for i := 1; i < len(shore); i++ {
			a, b := shore[i-1], shore[i]
			if math.Abs(p.Surface.At(p.Surface.Coast, a[0], a[1])) > 1e-5 || math.Abs(p.Surface.At(p.Surface.Coast, b[0], b[1])) > 1e-5 {
				continue
			}
			lon, lat := model.Wrap(a[0]+model.Wrap(b[0]-a[0])*.5), (a[1]+b[1])*.5
			if math.Abs(p.Surface.At(p.Surface.Coast, lon, lat)) > 1e-5 {
				t.Fatal("illustration shoreline drifts from terrain between vertices")
			}
			checked++
		}
	}
	if checked < 100 {
		t.Fatal("insufficient saved shoreline")
	}
	// Remove the procedural source entirely: even unvisited coordinates must
	// still return precisely the saved terrain, including across the seam.
	copy := *p
	copy.Seed++
	copy.Masses = nil
	a, b := detail.New(p), detail.New(&copy)
	for i := 0; i < 500; i++ {
		lon, lat := float64(i)*.137-30, float64(i%99-49)*.03
		if a.Sample(lon, lat) != b.Sample(lon, lat) {
			t.Fatal("viewing depends on procedural generation")
		}
	}
	d := p.Diagnostics.Hydrology
	if d == nil || d.InvalidTerminations != 0 || d.Classes["major"].Count < 2 || d.LowlandHeadwaters < 10 || d.NetworkCoverage < .1 || d.Lakes == 0 || d.Wetlands == 0 {
		t.Fatalf("incomplete network: %+v", d)
	}
	byID := map[string]model.River{}
	for _, r := range p.Rivers {
		byID[r.ID] = r
	}
	longest := 0.
	for _, r := range p.Rivers {
		longest = math.Max(longest, r.LengthKM)
		if r.Destination == "ocean" {
			pt := r.Path[len(r.Path)-1]
			if math.Abs(p.Surface.At(p.Surface.Coast, pt[0], pt[1])) > .001 {
				t.Fatalf("river %s ends offshore", r.ID)
			}
		}
		if r.Class == "distributary" {
			parent := byID[r.Parent]
			main := byID[parent.Destination]
			if math.Abs(parent.Discharge[len(parent.Discharge)-1]-r.Discharge[0]-main.Discharge[0]) > 1e-7 {
				t.Fatal("delta creates or loses discharge")
			}
		}
		if next, ok := byID[r.Destination]; ok {
			end := r.Path[len(r.Path)-1]
			found := false
			for _, pt := range next.Path {
				if end == pt {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("disconnected tributary %s -> %s", r.ID, next.ID)
			}
		}
	}
	if longest < 1500 {
		t.Fatalf("no continental trunk: %.1f km", longest)
	}
	meta, fields, err := p.Encode()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := model.Decode(meta, fields)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Surface, loaded.Surface) {
		t.Fatal("saved surface differs after reopening")
	}
	t.Logf("%d major rivers, %.1f km longest trunk, %d lowland sources, %.1f%% network cells, %d lakes, %d wetlands, %d splits", d.Classes["major"].Count, longest, d.LowlandHeadwaters, d.NetworkCoverage*100, d.Lakes, d.Wetlands, d.Distributaries)
}
