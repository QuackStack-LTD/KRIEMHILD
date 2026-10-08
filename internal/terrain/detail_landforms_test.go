package terrain

import (
	"math"
	"testing"
)

// A deliberately flat non-mountain parent isolates relief generation from the
// already successful mountain engine. A noise-only colour renderer cannot pass.
func plainDetail(t *testing.T, river bool, grade float64) *DetailModel {
	t.Helper()
	o, err := DecodeEnvironment([]byte(`{"columns":16,"rows":16,"seed":"landform-regression"}`))
	if err != nil {
		t.Fatal(err)
	}
	e := BuildEnvironment(o)
	for _, f := range e.Fields {
		for i := range f {
			f[i] = 0
		}
	}
	for i := range e.Mask {
		e.Mask[i] = 1
		e.Fields["elevation"][i] = 1000 - float64(i%16)*grade
		e.Fields["flow"][i] = -1
		e.Fields["moisture"][i] = .2
		e.Fields["geology"][i] = 1
		e.Fields["drainageElevation"][i] = e.Fields["elevation"][i]
		if river && i/16 == 8 && i%16 < 15 {
			e.Fields["flow"][i] = float64(i + 1)
			e.Fields["accumulation"][i] = 4 + float64((i%16)*(i%16))
			e.Fields["catchmentArea"][i] = 4 + float64((i%16)*(i%16))
		}
	}
	return NewDetailModel(e)
}

func TestDetailLowlandsHaveContinuousPhysicalRelief(t *testing.T) {
	m := plainDetail(t, false, 0)
	lo, hi, maxJump := 0., 0., 0.
	previous := m.Sample(5, 6.375, 8).Elevation
	for k := 0; k <= 1024; k++ {
		x := 5 + float64(k)/256
		p := m.Sample(x, 6.375, 8)
		d := p.Elevation - 1000
		lo = math.Min(lo, d)
		hi = math.Max(hi, d)
		maxJump = math.Max(maxJump, math.Abs(p.Elevation-previous))
		previous = p.Elevation
	}
	if lo > -25 || hi < 25 {
		t.Fatalf("flat lowlands: relief %.1f..%.1f m", lo, hi)
	}
	if maxJump > 18 {
		t.Fatalf("one-sample cliff in ordinary terrain: %.1f m", maxJump)
	}
	// Foothill context increases surrounding relief without touching any root
	// elevation or creating a new mountain core.
	before := m.Sample(6.375, 6.375, 8).Elevation
	for i := range m.foothills {
		m.foothills[i] = .65
	}
	after := m.Sample(6.375, 6.375, 8).Elevation
	if math.Abs(after-1000) <= math.Abs(before-1000) {
		t.Fatal("nearby ranges do not influence relief")
	}
	if m.Sample(6, 6, 8).Elevation != 1000 {
		t.Fatal("mountain apron changed parent")
	}
	t.Logf("ordinary lowland relief %.1f..%.1f m; max step %.2f m", lo, hi, maxJump)
}

func TestDetailMajorRiversInciseAndWiden(t *testing.T) {
	m := plainDetail(t, true, 20)
	checked := 0
	for _, f := range m.Features {
		if f.Discharge < 20 || len(f.Path) < 3 {
			continue
		}
		k := len(f.Path) / 2
		p := f.Path[k]
		center := m.Sample(p[0], p[1], 8)
		parent := m.Sample(p[0], p[1], 0)
		bank := m.Sample(p[0], p[1]+f.Widths[k]*2, 8)
		if center.Elevation >= parent.Elevation-3 || center.Elevation >= bank.Elevation-3 || center.RiverDepth <= 0 {
			t.Fatalf("river has no physical channel: parent %.1f center %.1f bank %.1f depth %.1f", parent.Elevation, center.Elevation, bank.Elevation, center.RiverDepth)
		}
		if f.Path[len(f.Path)-1][0] < 14 && f.Widths[len(f.Widths)-1] < f.Widths[0] {
			t.Fatal("river narrows downstream despite growing discharge")
		}
		checked++
	}
	if checked < 5 {
		t.Fatal("insufficient channel coverage")
	}
	tile, _ := m.Tile(3, 1, 1)
	for _, f := range tile.Features {
		if f.Kind != "river" || len(f.Path) < 2 {
			t.Fatal("placeholder marker survived")
		}
	}
}

func TestDetailMeandersRespondToGradient(t *testing.T) {
	flat, steep := plainDetail(t, true, 5), plainDetail(t, true, 300)
	deviation := func(m *DetailModel) float64 {
		sum := 0.
		for _, f := range m.Features {
			if len(f.Path) < 3 {
				continue
			}
			a, b := f.Path[0], f.Path[len(f.Path)-1]
			for _, p := range f.Path {
				sum += segmentDistance(p[0], p[1], a[0], a[1], b[0], b[1])
			}
		}
		return sum
	}
	if deviation(flat) < deviation(steep)*2 {
		t.Fatal("flat and steep rivers have the same meandering")
	}
}

func TestDetailRiverRibbonsStopAtWaterBodies(t *testing.T) {
	m := detailFixture(t)
	for _, f := range m.Features {
		for _, p := range f.Path {
			if m.base(p[0], p[1]).WaterBody > 0 {
				t.Fatalf("river ribbon extends into open water: %s", f.ID)
			}
		}
	}
}

func TestDetailWetlandAndDunesChangeGeometry(t *testing.T) {
	m := plainDetail(t, false, 0)
	relief := func() float64 {
		sum := 0.
		for k := 0; k < 128; k++ {
			z := m.Sample(5+float64(k)/128, 6.375, 8).Elevation - 1000
			sum += z * z
		}
		return math.Sqrt(sum / 128)
	}
	dry := relief()
	for i := range m.World.Fields["wetland"] {
		m.World.Fields["wetland"][i] = 1
	}
	wet := relief()
	if wet >= dry*.4 {
		t.Fatal("wetland has the same relief as uplands")
	}
	for i := range m.World.Fields["wetland"] {
		m.World.Fields["wetland"][i] = 0
		m.World.Fields["dune"][i] = 1
	}
	sand := relief()
	if sand < 3 || sand == dry {
		t.Fatal("dunes are only a colour")
	}
}
