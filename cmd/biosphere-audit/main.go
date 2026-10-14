// Reproducible end-to-end ecology diagnostics from newly generated worlds.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"kriemhild/internal/biosphere"
	"kriemhild/internal/terrain"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type report struct {
	Seed                  string         `json:"seed"`
	Source                string         `json:"source"`
	Version               int            `json:"version"`
	Habitats              int            `json:"habitats"`
	HabitatKinds          map[string]int `json:"habitatKinds"`
	Strata                map[string]int `json:"strata"`
	Species               int            `json:"species"`
	Occurrences           int            `json:"occurrences"`
	Interactions          int            `json:"interactions"`
	Roles                 map[string]int `json:"roles"`
	InvalidLinks          int            `json:"invalidLinks"`
	DarkPhotosynthesizers int            `json:"darkPhotosynthesizers"`
	Diagnostics           []string       `json:"diagnostics"`
	DiagnosticCount       int            `json:"diagnosticCount"`
	Exclusions            []string       `json:"exclusions"`
	HabitatSizes          map[string]int `json:"habitatCellHistogram"`
	MaxRangeHabitats      int            `json:"maxRangeHabitats"`
	OccupiedHabitats      int            `json:"occupiedHabitats"`
	GenerationSeconds     float64        `json:"generationSeconds"`
}

func main() {
	seeds := flag.String("seeds", "KRIEMHILD", "comma-separated seeds")
	out := flag.String("out", "docs/landshape-gallery/biosphere-metrics.json", "JSON output")
	flag.Parse()
	reports := []report{}
	for _, seed := range strings.Split(*seeds, ",") {
		start := time.Now()
		raw, _ := json.Marshal(map[string]any{"columns": 160, "rows": 100, "landPercent": 42, "realism": true, "seed": seed})
		options, err := terrain.DecodeEnvironment(raw)
		if err != nil {
			panic(err)
		}
		e := terrain.BuildEnvironment(options)
		hs, source, err := biosphere.Habitats(e, nil, "audit")
		if err != nil {
			panic(err)
		}
		state := biosphere.State{ID: "audit", TerrainID: "audit", AgeID: "audit", Version: biosphere.Version, Source: source, Width: 160, Height: 100, Habitats: hs, WrapX: e.Geography.WrapX, CellKM: e.Geography.WorldScale}
		state.CellKM, state.CellEastWestKM = e.Climate.Scale.NorthSouthKM, e.Climate.Scale.EquatorialEastWestKM
		state.LatitudeNorth, state.LatitudeSouth = e.Climate.Scale.LatitudeNorth, e.Climate.Scale.LatitudeSouth
		settings := biosphere.Settings{Mode: "guided", Preset: "earth-like", Familiarity: .5, Richness: "sparse", Coherence: "realistic", Scope: "entire", Seed: seed, Rules: []biosphere.Rule{}}
		preview, err := biosphere.Generate(state, settings, nil)
		if err != nil {
			panic(err)
		}
		byID := map[string]biosphere.Lifeform{}
		habitats := map[string]biosphere.Habitat{}
		r := report{Seed: seed, Source: source, Version: biosphere.Version, Habitats: len(hs), Species: len(preview.Species), Occurrences: len(preview.State.Occurrences), Interactions: len(preview.State.Interactions), HabitatKinds: map[string]int{}, Strata: map[string]int{}, Roles: map[string]int{}, HabitatSizes: map[string]int{}, DiagnosticCount: len(preview.State.Diagnostics), Diagnostics: preview.State.Diagnostics[:min(20, len(preview.State.Diagnostics))], Exclusions: preview.Exclusions[:min(20, len(preview.Exclusions))]}
		for _, h := range hs {
			habitats[h.ID] = h
			r.HabitatKinds[h.Kind]++
			r.Strata[h.Stratum]++
			size := "20+"
			if len(h.Cells) < 5 {
				size = "1-4"
			} else if len(h.Cells) < 20 {
				size = "5-19"
			}
			r.HabitatSizes[size]++
		}
		for _, s := range preview.Species {
			byID[s.ID] = s.Biology
			for _, role := range s.Biology.Roles {
				r.Roles[role]++
			}
		}
		if err = biosphere.ValidateState(preview.State, byID); err != nil {
			panic(fmt.Sprintf("%s: %v", seed, err))
		}
		present := map[string]bool{}
		ranges, occupied := map[string]int{}, map[string]bool{}
		for _, o := range preview.State.Occurrences {
			present[o.HabitatID+o.SpeciesID] = true
			ranges[o.SpeciesID]++
			occupied[o.HabitatID] = true
			b, h := byID[o.SpeciesID], habitats[o.HabitatID]
			if (b.Group == "algae" || b.Group == "plant") && h.Conditions.Light < .01 {
				r.DarkPhotosynthesizers++
			}
		}
		r.OccupiedHabitats = len(occupied)
		for _, count := range ranges {
			r.MaxRangeHabitats = max(r.MaxRangeHabitats, count)
		}
		for _, link := range preview.State.Interactions {
			if !present[link.HabitatID+link.Source] || !present[link.HabitatID+link.Destination] {
				r.InvalidLinks++
			}
		}
		if r.InvalidLinks > 0 || r.DarkPhotosynthesizers > 0 {
			panic(fmt.Sprintf("invalid ecology: %+v", r))
		}
		r.GenerationSeconds = time.Since(start).Seconds()
		reports = append(reports, r)
		if err = os.MkdirAll(filepath.Dir(*out), 0755); err != nil {
			panic(err)
		}
		data, _ := json.MarshalIndent(reports, "", "  ")
		if err = os.WriteFile(*out, data, 0644); err != nil {
			panic(err)
		}
		fmt.Printf("%s: %d habitats, %d species, %d occurrences, %d links; %.1fs\n", seed, r.Habitats, r.Species, r.Occurrences, r.Interactions, r.GenerationSeconds)
	}
}
