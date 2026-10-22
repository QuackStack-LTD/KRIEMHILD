package generate

import (
	"context"
	"fmt"
	"kriemhild/internal/terrain/model"
	"math"
	"testing"
)

func TestMaritimeMidlatitudeGrowingSeason(t *testing.T) {
	g := model.Grid{Width: 96, Height: 48, Fields: map[string][]float64{}}
	for _, k := range []string{"bed", "coast", "windX", "windY", "temperature", "precipitation", "pet", "moisture"} {
		g.Fields[k] = make([]float64, g.Width*g.Height)
	}
	for i := range g.Fields["bed"] {
		g.Fields["bed"][i] = -100
	}
	// A low coastal strip and a distant interior at the same latitude.
	for y := 0; y < g.Height; y++ {
		for _, x := range []int{30, 60} {
			i := y*g.Width + x
			g.Fields["bed"][i] = 100
			g.Fields["coast"][i] = 20
			if x == 60 {
				g.Fields["coast"][i] = 2000
			}
		}
	}
	p := &model.Planet{Spec: model.DefaultSpec(), Grid: g}
	if err := climate(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	for _, y := range []int{10, 37} { // ±50.625 degrees, symmetric seasons.
		coastal, inland := y*g.Width+30, y*g.Width+60
		extreme := func(i int) (float64, float64) {
			lo, hi := math.Inf(1), math.Inf(-1)
			for m := 0; m < 12; m++ {
				v := g.Fields[fmt.Sprintf("temperature-%02d", m)][i]
				lo = math.Min(lo, v)
				hi = math.Max(hi, v)
			}
			return lo, hi
		}
		clo, chi := extreme(coastal)
		ilo, ihi := extreme(inland)
		t.Logf("latitude %.3f: coastal annual %.2f, coldest %.2f warmest %.2f; interior annual %.2f range %.2f", g.Point(30, y)[1]*180/math.Pi, g.Fields["temperature"][coastal], clo, chi, g.Fields["temperature"][inland], ihi-ilo)
		if chi <= 10 || g.Fields["temperature"][coastal] < 5 {
			t.Error("low maritime midlatitudes cannot support a forest growing season")
		}
		if ihi-ilo <= chi-clo {
			t.Error("continental seasonality should exceed maritime seasonality")
		}
	}
	if p.Diagnostics.MoistureBudgetError > 1e-9 {
		t.Fatal("water mass budget changed")
	}
	equator, subtropics := 24*g.Width+5, 16*g.Width+5
	if g.Fields["precipitation"][equator] <= g.Fields["precipitation"][subtropics]*2 {
		t.Error("tropical convection failed to exceed subtropical subsidence")
	}
	if g.Fields["temperature"][equator] < 24 {
		t.Error("maritime correction destroyed warm equator")
	}
	for m := 0; m < 12; m++ {
		north := g.Fields[fmt.Sprintf("temperature-%02d", m)][10*g.Width+30]
		south := g.Fields[fmt.Sprintf("temperature-%02d", (m+6)%12)][37*g.Width+30]
		if math.Abs(north-south) > 1e-9 {
			t.Error("hemispheric seasons are not reversed")
		}
	}
}
