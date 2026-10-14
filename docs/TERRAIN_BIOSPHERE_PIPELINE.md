# Terrain and Biosphere causal pipeline

New generation uses the following order. Stored worlds are restored from their
saved fields and detail tiles; none of these stages run merely to open a world.

1. Establish geographic scale, seeded plates and continental foundations.
2. Union filled continental fields, subtract geological cuts, and preserve an irregular ocean frame on **all four sides**. Wrapped
   physical distances still apply, but no generated land crosses the displayed cut.
3. Derive crust type, age and thickness proxies, boundary motion, relief, shelves,
   ocean depth and archipelagos from the shared foundations.
4. Compute basin-constrained ocean thermal advection, atmospheric moisture
   transport, seasonal rainfall, PET, snow and a soil-water bucket with spin-up.
5. Feed the monthly water surplus and actual evaporation into hydrology. Route
   recharge, baseflow, rivers, lakes and sediment through the existing drainage
   system. The configured, bounded erosion passes update climate before routing.
6. Reconcile final monthly climate and categorical surface cover, then derive
   geological history and resource eligibility. Climate and cover remain distinct;
   the old numeric climate overlay retains its original IDs.
7. Derive a versioned Biosphere snapshot from the saved terrain, authored surface
   and climate, canonical cover, and categorical terrain fallback for old records.
8. Cluster connected habitats, retain their cell conditions, expand vertical strata,
   and generate compatible communities and spatially connected native ranges.

## Continental layout

The current `filled-union-2` layout uses five seeded spatial families and variable foundation areas. Count and ocean separation are preferences: collisions can merge masses and rifts can divide them. Filled envelopes, asymmetric lobes and multiscale domain warps define the coast; global cuts follow the union. A bounded global sea-level search replaces connected quota growth and owner partitions. The irregular ocean frame keeps every displayed edge wet. Coverage conflicts are reported instead of filling cutouts or rerunning smaller continent counts. See [the implementation and controls](CONTINENTS_AND_BIOMES.md) and [detail topology](COASTLINE_REPAIR.md).

## Water and geological provenance

`annualPET`, `soilWater`, `droughtMonths`, `growingMonths`, `climateWaterYield`,
`climateActualET` and `climateSnowmelt` expose the monthly budget. PET is potential
demand; actual evaporation cannot exceed available liquid water and storage.
Storm runoff leaves the soil bucket before monthly evaporation; its fraction
depends on permeability and slope. The remaining infiltration, storage, evaporation
and overflow conserve water. Wetlands do not create an extra rainfall supply.
Hydrology consumes the surplus after evaporation and soil storage, rather than a
second annual temperature-only evaporation estimate. The lapse-rate control is in
degrees C/km; old configurations default to 6.5.

Monthly prevailing winds trace seasonal upwind moisture access over a fixed
geographic distance. They redistribute the transported annual precipitation among
months without multiplying the annual water supply a second time.

`currentX`, `currentY` and `current` are wind/basin thermal-advection proxies. The
ocean path stops at land; the resulting coastal anomaly decays inland. A longitude
sine wave no longer creates alternating warm and cold coasts independently of
basin geometry. This is a bounded procedural model, not an ocean circulation solver.

`crustType`, `crustAge`, `crustThickness`, `convergence` and `extension` share the
plate geometry. Ocean-floor depth increases with the crust-age proxy outside
shelves. Geological histories infer marine deposition, wetland preservation,
evaporation and burial from regional relief, water balance, sediment and deformation.
Each new history explicitly records its inference and confidence. Ages and histories
are procedural estimates, not a simulated reconstruction of plate motions over time.
Existing source/reservoir/seal/trap checks remain responsible for resource eligibility.

## Biosphere version 2

Version 1 lifeforms and states remain readable. Their stored depth values are not
silently reinterpreted. New habitat signatures include a version marker, all source
environmental data and categories; changed cells, profiles and overlays receive new
habitat revision IDs. Old confirmed geometry remains available for review.

River channels and their terrestrial banks overlap in coarse cells. Riverbank
patches retain the underlying biome, exposed substrates and vertical layers.
Dispersal tracks all media present in a cell: a river does not turn a whole coarse
land cell into an ocean-like barrier, and its fish still follow connected water.

Habitats expose `baseBiome`, composable `overlays`, and a `stratum`. Steppe, xeric
scrub, savanna, Mediterranean woodland and cold desert use canonical terrain cover;
the old moisture shortcut cannot turn them into forest. Connected clustering also
distinguishes drought, drainage, salinity and water-depth regimes. Soil pH, depth
and drainage are documented procedural proxies. Rock substrate requires exposure;
a deep granite basement does not make an ordinary soil surface granite habitat.

Aquatic `depth` means organism depth below the water surface; `bathymetry` records
the bottom depth. Photic, twilight, aphotic and benthic habitats are separate. Forest
canopy, understory and floor likewise have distinct light levels. Photosynthetic
plants and algae require light even if a custom minimum-light tolerance is zero.
Deep decomposers can use an explicit sinking-detritus subsidy instead of fictional
photosynthesis on the abyssal floor.

Each habitat retains aligned `profiles` and `cells`. Suitability applies hard
survival limits to individual profiles, then reports the compatible fraction and
reduces the score for a small suitable fraction. Optional drought-month,
growing-month, soil-pH, stratum and dispersal-distance requirements are available in
the editor. Abundance is a fertility/light/detrital productivity proxy with reduced
predator carrying capacity, not a population-dynamics prediction.
Generated niche envelopes use shared physiological bands, so minor differences
between neighboring cells do not each invent another nearly identical species.

Native ranges use deterministic graph traversal in geographic distance. Terrestrial
organisms cannot cross ocean, freshwater ranges cannot jump saltwater, and marine
ranges follow connected water. Horizontal wrapping is explicit; old states lacking
scale use a documented 100 km/cell fallback. An explicit airborne dispersal trait
permits higher-cost barrier crossings. No fixed number of habitats limits a range.
Planetary east/west distances contract with latitude. A locally adapted organism
starts in its target habitat, and its native origin is persisted as provenance;
accepting a preview does not relocate that origin on the next regeneration.

Richness controls variants per functional guild. Producers, decomposers, consumers,
scavengers, pollinators, seed dispersers, mutualists, parasites and pathogens are
scheduled independently. Pollination and seed dispersal require flowering plants;
feeding rejects viruses and explicitly inedible targets, and host-group restrictions
are enforced. Generated interactions require local occurrence of both partners.
Dependencies can require specific lifeform IDs, medium, target life stage and
overlapping seasons. `activeSeasons` and `lifeStages` biological traits provide
the explicit phenology/stage context. Unexplained obligate dependency cycles are
rejected; mutualistic loops and explained stage/season links remain valid.
Cold temperatures do not invent migration: resident dormancy is explicit. A seasonal
visitor needs a migration trait, mobile dispersal and a connected alternative habitat.
Introduced, cultivated and invasive status requires an explicit origin/history trait.
Confirmed/locked populations and their
links survive previews and acceptance; a requested variant can add alternatives.

## Scientific basis and model limits

- [USGS: understanding plate motions](https://pubs.usgs.gov/gip/dynamic/understanding.html)
  describes relative boundary motion and the distinct convergence settings.
- [NOAA: ocean light](https://oceanservice.noaa.gov/facts/light_travel.html) and
  [ocean layers](https://www.noaa.gov/jetstream/ocean/layers-of-ocean) distinguish
  productive surface water from the dark deep ocean.
- [NOAA: atmospheric circulation](https://www.noaa.gov/jetstream/global/global-atmospheric-circulations)
  supplies the broad circulation interpretation; local geography still controls
  moisture access and rain shadows.

Soil chemistry, inferred geological history, detrital delivery, abundance and
dispersal distances are game-scale proxies. They support deterministic, inspectable
causes and constraints; they do not claim calibrated species population forecasts,
full plate reconstruction or a three-dimensional ocean simulation.

## Reproducible evidence

`go run ./cmd/landshape-gallery` writes twenty seeded terrain/climate comparisons
and CSV diagnostics. `go run ./cmd/biosphere-audit -seeds KRIEMHILD,7,19` derives
layered habitats, generates communities and validates their relationships from
the same full-size terrain pipeline. Its compact JSON includes the source hash,
strata, guilds, habitat-size distribution, range sizes and exclusion examples.
