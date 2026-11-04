package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"kriemhild/internal/terrain/generate"
	"kriemhild/internal/terrain/hydrology"
	"kriemhild/internal/terrain/model"
	"os"
	"strings"
)

func main() {
	seeds := flag.String("seeds", "Kriemhild,gallery-001,gallery-002", "comma-separated deterministic worlds")
	flag.Parse()
	out := []any{}
	for _, seed := range strings.Split(*seeds, ",") {
		spec := model.DefaultSpec()
		spec.Seed = seed
		spec.SurfaceWidth = 1024
		p, err := generate.Generate(context.Background(), spec, nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		g := hydrology.Build(p)
		out = append(out, map[string]any{"seed": seed, "version": spec.Version, "climateVersion": spec.ClimateVersion, "rivers": len(p.Rivers), "reaches": len(g.Reaches), "nodes": len(g.Nodes), "counts": g.Counts, "issues": g.Issues, "biomes": p.Diagnostics.BiomeShares, "latitudeBiomes": p.Diagnostics.LatitudeBiomes, "moistureBudgetError": p.Diagnostics.MoistureBudgetError})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		panic(err)
	}
}
