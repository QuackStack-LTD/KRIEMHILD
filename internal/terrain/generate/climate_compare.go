package generate

import (
	"context"
	"fmt"
	"kriemhild/internal/terrain/model"
	"math"
)

// ClimateOnFixedTerrain is an offline diagnostic, never a load/migration path.
// It returns a separate grid and retains source geography, water masks and rivers.
// The returned immutable surface and rivers are shared; callers must not edit them.
func ClimateOnFixedTerrain(ctx context.Context, source *model.Planet, version int) (*model.Planet, error) {
	if source.Surface == nil {
		return nil, fmt.Errorf("fixed climate comparison requires a saved surface")
	}
	p := *source
	p.Spec.ClimateVersion = version
	if err := p.Spec.Validate(); err != nil {
		return nil, err
	}
	p.Diagnostics = model.Diagnostics{}
	p.Grid = model.Grid{Width: source.Grid.Width, Height: source.Grid.Height, Fields: map[string][]float64{}}
	for key, a := range source.Grid.Fields {
		p.Grid.Fields[key] = append([]float64(nil), a...)
	}
	weather := p
	weather.Grid = model.Grid{Width: 192, Height: 96, Fields: map[string][]float64{}}
	for _, key := range []string{"bed", "coast", "windX", "windY", "temperature", "precipitation", "pet", "moisture"} {
		weather.Grid.Fields[key] = make([]float64, 192*96)
	}
	for i := range weather.Grid.Fields["bed"] {
		pt := weather.Grid.Point(i%192, i/192)
		weather.Grid.Fields["bed"][i] = source.Surface.At(source.Surface.Bed, pt[0], pt[1])
		weather.Grid.Fields["coast"][i] = source.Surface.At(source.Surface.Coast, pt[0], pt[1])
	}
	if err := climate(ctx, &weather); err != nil {
		return nil, err
	}
	sea := seaLevelClimate(weather.Grid)
	for key := range weather.Grid.Fields {
		if key == "bed" || key == "coast" {
			continue
		}
		a := make([]float64, p.Grid.Width*p.Grid.Height)
		for i := range a {
			pt := p.Grid.Point(i%p.Grid.Width, i/p.Grid.Width)
			a[i] = weather.Grid.At(key, pt[0], pt[1])
		}
		p.Grid.Fields[key] = a
	}
	for i, bed := range p.Grid.Fields["bed"] {
		pt := p.Grid.Point(i%p.Grid.Width, i/p.Grid.Width)
		for _, key := range append([]string{"temperature"}, temperatureMonths[:]...) {
			if version >= 2 {
				p.Grid.Fields[key][i] = sea.At(key, pt[0], pt[1]) - math.Max(0, bed)*.0065
			} else {
				p.Grid.Fields[key][i] -= (bed - weather.Grid.At("bed", pt[0], pt[1])) * .0065
			}
		}
	}
	if version >= 2 {
		materializedMoisture(&p.Grid, p.Spec)
	}
	if version >= 1 {
		summarizeSeasons(&p.Grid)
	} else {
		for _, key := range seasonalKeys {
			delete(p.Grid.Fields, key)
		}
	}
	p.Diagnostics.ClimateBudget = weather.Diagnostics.ClimateBudget
	p.Diagnostics.MoistureBudgetError = weather.Diagnostics.MoistureBudgetError
	p.Diagnostics.ClimateBands = ClimateMeasurements(&p)
	return &p, nil
}
