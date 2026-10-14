# Landmass and coastline repair

This revision replaces regular center spacing and quota-grown partitions with a filled, globally thresholded continental field. The controls, layout families and coverage priority are documented in [Continents and biomes](CONTINENTS_AND_BIOMES.md). [The gallery](landshape-gallery/index.html) measures the final mask, including connected supercontinents, rifted regions, bays, peninsula tips, narrow isthmuses and actual emerged island sizes.

## Coastline topology at detail scales

New generated environments store a `coastal-sdf-1` recipe with seed, a 1.75-cell band and bounded amplitude. An exact Euclidean distance transform derives the signed shoreline field from the final marine footprint. Three deterministic world-coordinate noise scales add coves, fragmented shores and small islands inside that band. Deeper zoom samples the same field more densely; it does not invent a new field per tile or zoom level. Coarse nodes remain exact anchors. Continental interiors, open basins, inland lake footprints and authored cover retain their parent classification.

Detailed elevation, ocean membership, local water level and seabed transitions use this shoreline. Marine river mouths are clipped against it; supplied inland channels have a protected corridor to prevent a new cove from severing a tributary or junction. Lake identity and the coarse drainage graph remain authoritative. Adjacent tiles use identical coordinates and halos. Restoration rebuilds the distance and channel caches from saved state, including after a cold server restart. Worlds without the optional recipe keep the previous fixed-footprint behavior.

This is bounded coastal refinement, not a second high-resolution global hydrology simulation. It cannot create a major ocean basin inside a continental interior or a new inland lake catchment. Its physical widths are fractions of a geographically scaled base cell; world scale controls their distance in kilometres. Four-ocean-edge protection applies at fine levels too. Rendering and inspection choose categorical cover from the matching wet/dry side and never interpolate enum IDs.

## Performance

The mask now needs two field evaluations at most and a fixed 24-step threshold search. It has no repeated scan from each cell to every boundary cell and no recursive rebuilding for lower continent counts. The shoreline distance transform is linear in grid area and handles horizontal wrapping. Nearest-plate assignments are cached for the shared generation stages. Monthly orbit/declination terms are computed once per climate pass, and water accumulation reuses field slices while preserving stored float rounding.

`go run ./cmd/terrain-profile -out report.json -cpuprofile terrain.cpu` reports individual `BuildEnvironment` phases at 64 × 40, 160 × 100 and 192 × 128. Timings are observational and do not enter generated state. The 24,576-cell limit and 160 × 100 UI default remain unchanged. See [validation](VALIDATION.md) for measured baseline and final timings.

## Climate diagnostic and limited transport correction

Geography was repaired first, with the climate equations unchanged. A six-seed comparison then still showed excessive loss of moisture along warm continental paths. The final transport pass returns a bounded fraction of already precipitated vapor to the downwind supply, scaled by temperature and circulation rainfall potential. It cannot increase vapor above the incoming amount. `moistureRecycling` exposes the contribution; monthly PET, aridity thresholds and climate-to-biome mapping are unchanged.

Evapotranspiration recycling is a physical source of inland atmospheric moisture, documented by [NASA's Amazon wet-season research](https://science.nasa.gov/earth/earth-observatory/the-amazon-makes-its-own-wet-season-91161/). The model's maximum return fraction of 0.55 is a procedural calibration parameter, not a measured universal constant. This normalized transport approximation does not claim a closed global atmospheric mass solver. Monthly land water accounting still conserves its available rainfall, storage and snowmelt. [Before/after measurements](landshape-gallery/moisture-recycling-comparison.json) use identical geography and seeds; the final twenty-world report includes biome group shares, latitude bands and largest connected dryland regions.

## Compatibility

Old landmass structures and missing optional coastline fields remain readable. New generation stores the versions and parameters needed to reproduce the coast. Imported heights, existing saved terrain and user-painted cover are not regenerated on load. The legacy WFC terrain-point mode is unchanged. The suite also covers climate classes, categorical detail, hydrology, geological/resource prerequisites, terrain archives and cold restart.
