package main

import (
	"encoding/csv"
	"fmt"
	"kriemhild/internal/terrain"
	"os"
	"path/filepath"
)

func writeMeasurements(out string, names []string, shapes map[string]terrain.LandShapeReport, climates map[string]terrain.ClimateDistribution) {
	file, err := os.Create(filepath.Join(out, "measurements.csv"))
	if err != nil {
		panic(err)
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	_ = writer.Write([]string{"seed", "major_continents", "land_percent", "minimum_gap_cells", "open_ocean_percent", "reserved_land_cells", "latitude_from", "latitude_to", "land_cells", "dryland_percent", "annual_rain_mm", "aridity_index"})
	for _, seed := range names {
		s := shapes[seed]
		for _, b := range climates[seed].Bands {
			values := []any{seed, s.MajorCount, s.LandFraction * 100, s.MinimumGap, s.OpenOceanFraction * 100, s.ReservedLandCells, b.South, b.North, b.Land, b.DryPercent, b.Rain, b.AI}
			row := make([]string, len(values))
			for i, v := range values {
				row[i] = fmt.Sprint(v)
			}
			if err = writer.Write(row); err != nil {
				panic(err)
			}
		}
	}
	writer.Flush()
	if err = writer.Error(); err != nil {
		panic(err)
	}
}
