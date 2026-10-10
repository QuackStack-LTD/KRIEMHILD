package terrain

import "testing"

func shoreFixture(steep bool) *DetailModel {
	e := waterSupplyFixture(false)
	for i := range e.Mask {
		x := float64(i % e.Options.Columns)
		e.set("flow", i, -1)
		e.set("accumulation", i, 0)
		e.set("waterBody", i, 0)
		e.set("waterDepth", i, 0)
		e.set("lake", i, 0)
		e.set("ocean", i, 0)
		e.set("sediment", i, .8)
		e.set("coastalSediment", i, .8)
		e.set("slope", i, .01)
		e.set("dune", i, 0)
		e.set("mountainCore", i, 0)
		e.set("elevation", i, 500+(x-15.5)*20)
		e.Mask[i] = 1
		if x < 16 {
			e.Mask[i] = 0
			e.set("waterBody", i, 2)
			e.set("lake", i, 1)
			e.set("waterLevel", i, 500)
			e.set("waterDepth", i, 500-e.get("elevation", i))
		}
		if steep {
			e.set("slope", i, .5)
			e.set("mountainCore", i, .7)
		}
	}
	return NewDetailModel(e)
}
func TestShoreSandIsNarrowAndMountainLakeRemainsRocky(t *testing.T) {
	gentle, steep := shoreFixture(false), shoreFixture(true)
	sandWidth := 0.
	maxSand := 0.
	for x := 15.; x < 17; x += .005 {
		p := gentle.Sample(x, 16.5, 6)
		if p.WaterBody == 0 {
			if p.Sand > .1 {
				sandWidth += .005
			}
			if p.Sand > maxSand {
				maxSand = p.Sand
			}
		}
		rock := steep.Sample(x, 16.5, 6)
		if rock.WaterBody == 0 && rock.Sand > .02 {
			t.Fatal("sand replaces steep lakeside mountain")
		}
	}
	if maxSand < .1 || sandWidth > .15 {
		t.Fatalf("shore band must be visible but narrow: width %.3f max %.3f", sandWidth, maxSand)
	}
	if p := steep.Sample(16.3, 16.5, 6); p.Rock < .8 {
		t.Fatal("mountain lost its rocky transition")
	}
}

func TestMarineShoreHasSandOrStoneAndPreservesWaterline(t *testing.T) {
	for _, sediment := range []float64{.8, .03} {
		m := shoreFixture(false)
		for i := range m.World.Mask {
			m.World.set("sediment", i, sediment)
			m.World.set("coastalSediment", i, 0)
			if m.World.get("waterBody", i) > 0 {
				m.World.set("ocean", i, 1)
				m.World.set("lake", i, 0)
			}
		}
		maxSand, maxRock, width := 0., 0., 0.
		for x := 15.; x < 17; x += .005 {
			p := m.Sample(x, 16.5, 6)
			if p.WaterBody > 0 {
				continue
			}
			maxSand = max(maxSand, p.Sand)
			maxRock = max(maxRock, p.Rock)
			if p.Sand > .1 || p.Rock > .3 {
				width += .005
			}
		}
		if sediment > .5 && maxSand < .5 {
			t.Fatal("sandy marine beach disappeared", maxSand)
		}
		if sediment < .1 && maxRock < .6 {
			t.Fatal("stony marine shore disappeared", maxRock)
		}
		if width > .25 {
			t.Fatal("marine beach spreads too far inland", width)
		}
		for _, x := range []float64{15.25, 15.75, 16.25} {
			if m.Sample(x, 16.5, 0).WaterBody != m.Sample(x, 16.5, 6).WaterBody {
				t.Fatal("shore moves with zoom")
			}
		}
	}
}
