# Continental foundations and authoritative biomes

This pipeline applies to new physical generation and explicit regeneration. Opening a saved world does not rerun it. Imported heightfields define their own foundation, bypass continent placement and archipelagos, and retain their elevations. The legacy WFC point generator is unchanged.

## Layout controls and priorities

`majorContinents` is independent of `plateCount`, `continentCount` (legacy terrain points), `islandFrequency`, and `islandCoastalShare`. Zero, including an omitted value in old settings, selects 4–6 major foundations from the seed. Explicit values are 1–12. `oceanSeparation` defaults to `standard`; narrow, standard, and wide reserve 5%, 8%, and 12% of the smaller grid dimension. At 160 × 100 these targets are 5, 8, and 12 cells. Discrete coast cells and diagonal protection can make measured gaps somewhat wider.

The physical generator uses seeded random sequential inhibition with cylindrical horizontal distances on full-planet maps. Irregular continental footprints participate in the partition itself, rather than being cut against center-only polygons. Exact wrapped distances reserve water between major foundations. Connected priority growth establishes each interior, then allocates remaining land by organic footprint score. Compact masses, elongated regions, curves, gulfs, hooks and compound/isthmus regions vary by seed. All four displayed edges remain in ocean, including horizontally wrapped planets. Automatic count retries down to four if an initial layout cannot approach the area target. See [the causal pipeline and Biosphere revision](TERRAIN_BIOSPHERE_PIPELINE.md).

Constraint priority is **ocean separation and connected foundations, then requested land area**. If the target cannot fit, `environment.continentalLayout.warning` records the requested and achieved foundation area and the UI displays the shortfall. Reduce continent count or separation to accommodate more land. Islands and inland water can shift the final land percentage slightly. Island candidates cannot occupy reserved ocean corridors; seam relief reconciliation also protects their submerged beds. Bathymetry, ocean connectivity, relief, runoff, climate, geological history and resources all consume this same generated foundation.

The old physical `continentCount` value is accepted for compatibility but no longer claims to count major continents. Its slider is labeled “Legacy terrain points” and disabled in physical mode. Presets explicitly initialize both new controls. Existing saved terrain and user paint retain their stored data.

## Water and energy budget

`atmosphere.go` computes a smooth seasonal rainfall potential from signed latitude, orbital declination and circulation strength. Migrating tropical ascent, subtropical subsidence, mid-latitude storm tracks and polar drying affect precipitation potential, not categorical map paint. This follows the broad circulation described by [NOAA](https://www.noaa.gov/jetstream/global/global-atmospheric-circulations). Ocean-supplied vapor is transported downwind using each latitude band's actual zonal direction and fractional meridional transport. Moisture decays with geographic travel distance; cold/warm current effects are confined to nearby coasts.

A wind-aligned upstream transect of approximately 1000 km supplies coherent windward enhancement and lee suppression. Vapor depletion uses the actual incremental ascent, so the same mountain is not charged repeatedly at every cell in that transect. This distinction prevents excessive inland desiccation. These are deterministic, reduced-order climate processes, not a global circulation model or a forecast.

The twelve monthly precipitation and potential evaporation values define **AI = annual precipitation / annual potential evapotranspiration**. `climateClass()` is authoritative: desert below 0.25, steppe from 0.25 to 0.50; colder growing conditions, elevation, temperature seasonality and rainfall seasonality distinguish the other climates. These calibrated bands are ecological approximations, not universal biome laws. [Shen et al. (2019)](https://bg.copernicus.org/articles/16/2857/2019/) discuss the broader relationships among climate, soils and biomes. [Steppe](https://education.nationalgeographic.org/resource/steppe/) occupies the dry grassland niche between more humid vegetation and desert.

The same monthly budget updates moisture before hydrology, after erosion changes relief, and finally after hydrological surface features are known. `moisture = clamp(AI, 0, 1)` and legacy `aridity = clamp(1-AI, 0, 1)` are diagnostic conversions; neither separately classifies vegetation. The `desert` and `steppe` fields agree with monthly zone kinds. Dunes need sand transport, deposition, suitable slope and an arid climate. Oases require hydrological support inside a desert. Neither feature defines the regional climate.

## Climate to surface cover

The ecological mapping is implemented once in `internal/terrain/biomes.go`. The append-only catalog `internal/terrain/biomes.json` defines persisted numeric IDs, names, fallback colors and valid terrain candidates. JavaScript imports that same catalog. The mapping is evaluated after monthly classification:

| Climate / condition | Dominant surface cover |
| --- | --- |
| Rainforest | Jungle / tropical rainforest |
| Monsoon or tropical seasonal | Seasonal forest or savanna according to growing-season soil water |
| Desert | Desert or cold desert; localized dunes, oasis or mesa only with supporting fields |
| Steppe | Dedicated semi-arid steppe |
| Warm dry-subhumid AI 0.50–0.65 | Xeric scrubland, unless a seasonal tropical or Mediterranean regime takes priority |
| Mediterranean | Mediterranean woodland / shrubland |
| Humid temperate | Forest, grassland or meadow according to water balance and growing-season soil moisture |
| Boreal | Taiga when adequately moist; open cold grassland otherwise |
| Tundra / alpine / polar | Tundra, alpine barrens, snow/ice respectively |
| Hydrologically saturated ground | Wetland; humidity alone does not suffice |
| Exposed high summits | Rocky mountain overlay; ordinary forested slopes retain their cover |

Suitability for farming and settlement remains a separate layer. No population or settlement is generated by a biome choice. The original numeric `climate` field retains its existing IDs and remains a compatibility overlay; `Climate.Cells[i].Zone → Climate.Zones[zone].Kind` is the authoritative physical climate. `realisticCandidates()` and basic physical candidates consume the stored surface biome. Four new default palette types are injected during explicit physical generation when missing, with a clear error if the 32-type limit prevents this. Their default legacy weights are zero, keeping them out of ordinary legacy selection; physical masks make the dedicated type available where required.

## Rendering, inspection and persistence

`fields.biome` is categorical. `DetailPoint.biome` carries its string identity through detail tiles; provisional tiles inherit it too. World rendering, refined rendering and hover resolve the same nearest parent category. Enum values are never bilinearly interpolated. Decorative noise remains a compatibility fallback for older worlds without a biome field; it cannot overturn an authoritative biome in new worlds. Elevation shading, rivers and local relief remain independent of cover. Pinned user paint takes precedence.

Inspection includes coordinates/latitude, temperature, annual rain, summer/winter rainfall, monthly climate zone, AI and the rendered surface category. The seasonal description also reports circulation rainfall potential, upwind moisture availability and rain-shadow influence. Overlays include `biome`, `aridityIndex`, `rainfallPotential`, `moistureSupply`, `rainShadow`, `steppe`, `desert`, `continentalOwner` and `oceanReserve`, alongside the original diagnostics.

New fields are optional when reading old worlds. Numeric biome IDs are validated without requiring the new field on historical data. Persisted climate cells retain their surface category. Editing terrain explicitly recalculates climate without relocating continents or importing a new layout.

## Reproducible evidence

Run `go run ./cmd/landshape-gallery` for the default twenty seeds at 160 × 100 and 42% land. [The gallery](landshape-gallery/index.html) includes silhouettes, monthly climate classes, precipitation, AI, surface biomes, and continental/regional detail samples from the same seed. Its JSON reports measure actual major components (at least 1% of map area), smaller fragments, wrapped coast-to-coast gaps, open ocean, latitude-bin dryland percentages and small disconnected dryland components. Shape hull diagnostics unwrap longitude before measuring aspect ratios or bays.

See [VALIDATION.md](VALIDATION.md) for measured results and test commands. This calibration establishes recognizable regional patterns and coherent water budgets; it does not claim terrestrial biome proportions for every seed, nor identical climates on different continental arrangements.
