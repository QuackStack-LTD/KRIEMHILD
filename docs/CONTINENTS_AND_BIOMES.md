# Continental foundations and authoritative biomes

This pipeline applies to new physical generation and explicit regeneration. Opening a saved world does not rerun it. Imported heightfields define their own foundation, bypass continent placement and archipelagos, and retain their elevations. The legacy WFC point generator is unchanged.

## Layout controls and priorities

`majorContinents` is independent of plate count, legacy terrain points and island controls. Zero selects 3–8 seeded foundations; explicit values select 1–12 foundations. This is a preference for geological source masses, not a promise about the number of connected continents after collision, rifting and flooding. `oceanSeparation` selects a spacing preference of 5%, 8% or 12% of the smaller dimension (5, 8 or 12 cells at 160 × 100). Natural straits and connected regions may be narrower.

`filled-union-2` chooses among clustered, dominant, hemispheric, belt and scattered layouts. Weighted rejection placement accepts probabilistic proposals instead of maximizing inter-center distance. Variable areas and tectonic bearings influence overlap; groups may accrete. `filled-sdf-2` masses combine broad asymmetric envelopes with lobes, peninsulas and three scales of coordinate deformation. Their union is cut by deep gulfs, rifts and seas **after** overlap, so a neighboring lobe cannot silently fill a gulf. The final mask is a global threshold of this field. No owner partition or per-continent area quota grows over its coastlines.

One bounded area correction and a bounded sea-level search approach the requested land fraction. **Shape and the irregular ocean frame take priority over exact coverage.** All four map edges remain ocean, including wrapped planets. If the foundation cannot approach the requested area, `continentalLayout.warning` reports the shortfall. Islands, lakes and erosion can subsequently change final coverage. Count and ocean separation are labeled as preferences in the Generator; the legacy point slider keeps its separate semantics.

Archipelagos use curved, branching and scattered groups with independently varied members in four size tiers. Coastal shelves, marginal seas, hotspots and oceanic settings remain distinct causes. Entire emerged members must fit the area budget, ocean frame and no-bridge checks; they are not truncated into strips. Diagnostics count emerged cells and visible groups, rather than treating every submerged bank as an island.

Bathymetry, relief, ocean connectivity, climate, drainage, geology and resources share the resulting foundation. Imported heights bypass placement and islands. Saved worlds retain their data and old structure versions; this revision applies only to new generation or explicit regeneration. See [coastal refinement and performance](COASTLINE_REPAIR.md).

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

`fields.biome` is categorical. `DetailPoint.biome` carries its string identity through detail tiles; provisional tiles inherit it too. World rendering, refined rendering and hover preserve the same categorical cover. Coastal detail selects a category from the matching land/water side, so a newly resolved island is not colored as ocean. Enum values are never bilinearly interpolated. Decorative noise remains a compatibility fallback for older worlds without a biome field; it cannot overturn an authoritative biome in new worlds. Elevation shading, rivers and local relief remain independent of cover. Pinned user paint takes precedence.

Inspection includes coordinates/latitude, temperature, annual rain, summer/winter rainfall, monthly climate zone, AI and the rendered surface category. The seasonal description also reports circulation rainfall potential, upwind moisture availability and rain-shadow influence. Overlays include `biome`, `aridityIndex`, `rainfallPotential`, `moistureSupply`, `moistureRecycling`, `rainShadow`, `steppe`, `desert`, `continentalOwner` and `oceanReserve`, alongside the original diagnostics.

New fields are optional when reading old worlds. Numeric biome IDs are validated without requiring the new field on historical data. Persisted climate cells retain their surface category. Editing terrain explicitly recalculates climate without relocating continents or importing a new layout.

## Reproducible evidence

Run `go run ./cmd/landshape-gallery` for the default twenty seeds at 160 × 100 and 42% land. [The gallery](landshape-gallery/index.html) includes silhouettes, monthly climate classes, precipitation, AI, surface biomes, and continental/regional/local detail samples from the same seed. Its JSON reports measure actual major components (at least 1% of map area), smaller fragments, wrapped coast-to-coast gaps, open ocean, latitude-bin dryland percentages and small disconnected dryland components. Shape hull diagnostics unwrap longitude before measuring aspect ratios or bays.

See [VALIDATION.md](VALIDATION.md) for measured results and test commands. This calibration establishes recognizable regional patterns and coherent water budgets; it does not claim terrestrial biome proportions for every seed, nor identical climates on different continental arrangements.
