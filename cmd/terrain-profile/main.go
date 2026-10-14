// terrain-profile records phase timings and optional CPU profiles on identical
// seeded inputs. Timing metadata never enters saved generation state.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"kriemhild/internal/terrain"
	"os"
	"runtime/pprof"
	"time"
)

func main() {
	output := flag.String("out", "docs/landshape-gallery/performance.json", "timing report")
	cpu := flag.String("cpuprofile", "", "optional CPU profile")
	flag.Parse()
	if *cpu != "" {
		f, err := os.Create(*cpu)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if err = pprof.StartCPUProfile(f); err != nil {
			panic(err)
		}
		defer pprof.StopCPUProfile()
	}
	reports := []map[string]any{}
	for _, size := range [][2]int{{64, 40}, {160, 100}, {192, 128}} {
		o, err := terrain.DecodeEnvironment([]byte(fmt.Sprintf(`{"columns":%d,"rows":%d,"seed":"KRIEMHILD","realism":true,"landPercent":42}`, size[0], size[1])))
		if err != nil {
			panic(err)
		}
		start, last := time.Now(), time.Now()
		phases := map[string]float64{}
		e := terrain.BuildEnvironmentObserved(o, func(name string) { now := time.Now(); phases[name] = now.Sub(last).Seconds(); last = now })
		reports = append(reports, map[string]any{"columns": size[0], "rows": size[1], "cells": size[0] * size[1], "seconds": time.Since(start).Seconds(), "phases": phases, "layout": e.Layout})
		fmt.Fprintf(os.Stderr, "%dx%d %.3fs\n", size[0], size[1], time.Since(start).Seconds())
	}
	raw, err := json.MarshalIndent(reports, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(*output, append(raw, '\n'), 0644); err != nil {
		panic(err)
	}
}
