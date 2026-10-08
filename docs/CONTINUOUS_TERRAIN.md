# Continuous terrain and water

Both physical generation modes use the same continuous ground surface. Real-world mode additionally applies its climate/entity rules. Presets continue to initialize physical settings and terrain preferences.

## Ground versus water

`elevation` is the land or underwater bed elevation relative to sea level, in metres. It is never replaced by a water-surface height. `waterLevel` is the local surface elevation, `waterDepth = waterLevel - elevation` on wet cells, and `bathymetry` mirrors that depth. Dry cells have zero water depth and water-body ID. `mask` is now explicitly dry land (1) versus water (0), including lakes; it cannot be reconstructed from the sign of elevation.

Only below-sea-level cells connected to the ocean frame become ocean automatically. Enclosed basins are identified using priority-flood spill elevations. A deterministic runoff/evaporation balance sets a local lake surface or leaves the basin dry. Upstream closed basins stop contributing provisional runoff to downstream basins. Closed dry basins drain downhill into internal sinks rather than following artificial uphill spill paths. Lakes may lie above or below global sea level; their beds have variable depths beneath a level surface.

`waterBodies` exports each ocean/lake footprint, its surface elevation and maximum depth. Continental coverage is an initial geometry target: lake filling and exposed closed basins can change the final dry-land percentage slightly.

## Relief

Generation builds a shared surface in this order: continental regions and ocean basins; shelves and margins; lowlands and regional highlands; broad collision ranges; valleys, secondary ridges and summit detail; enclosed depressions; gradient reconciliation; water connectivity and drainage; climate and habitat.

- Passive margins have broad, gently sloping shelves (typically 4?8 cells at a 100-cell map height), followed by a continental slope spread across another 7 cells. Coastlines lead into lowlands before reaching major uplift. These widths scale with resolution.
- Collision belts have independently varying regional and core widths. Broad highlands surround the mountain mass; coherent valleys and passes interrupt ridges, and smaller summit detail is added to that existing mass. Seeded age varies the height and relief of young versus eroded ranges. Plateaus use a separate broad field.
- Hotspots have a wide volcanic base and narrower summit. Drowned volcanic platforms blend continuously into the ocean basin; their rims enclose deeper lagoons. Subduction and divergence add trenches and submarine ridges beyond shelves.
- Formation metadata (`geology`) explicitly identifies passive terrain, active margins, young ranges, eroded ranges, plateaus and volcanic edifices. Active/volcanic formations can have steeper gradients. The generator does not currently synthesize impact craters or fault escarpments.
- Before deriving water, climate or drainage, a deterministic constraint pass reconciles unresolved elevation steps. Adjacent ordinary cells differ by at most 650 m (plus 4 m quantization); adjacent active/volcanic cells may differ by up to 1,100 m. Coastal transitions are ordinarily much gentler. Imported heightfields remain authoritative.
- Seeded enclosed depressions can cut below zero while retaining an enclosing rim. Water connectivity and runoff determine whether they stay dry or form lakes.

These are procedural scale relationships, not a simulation with a calibrated horizontal distance per cell. Vertical exaggeration is a display setting and does not alter the generated elevations or drainage.

## Reefs

Ocean connectivity, water depth, temperature, salinity, clarity, light penetration, substrate and slope gate reef habitat. Fringing reefs follow shorelines; barrier reefs follow shallow offshore shelf crests; patch reefs follow patches of exposed substrate; atolls occupy submerged volcanic rims around deeper lagoons. Suitable water is only potential habitat: a seeded, spatially coherent reef-province field and finer substrate continuity restrict development to portions of suitable coastlines. Offshore banks are localized geological features, rather than whole shallow coastal bands. The output includes `reefType` and connected `reefs` regions. Their terrain masks preserve these shapes through WFC rather than randomly dropping individual suitable cells.

## Rendering and inspection

Ground relief and water-depth shading work in 2D and the shared 3D texture. The 3D water mesh covers only wet cells, using each cell's local lake/ocean surface height. This replaces the global sea sheet that previously covered dry depressions. Flat and globe meshes support the same water footprint. Vertical exaggeration scales both bed and water surfaces together.

View overlays include water depth, water level, water-body ID, basin ID, ocean connectivity, catchment area, landform, shelf, seamount, light, reef type, regional highland strength, mountain-core strength and geological formation type. Hover reports ground elevation, local surface/depth and reef form. Real-world mode exposes 75 environmental fields.

## Validation and model bounds

Controlled tests verify a dry depression at −320 m, a wet basin with an independent surface and 46 depth levels, and ocean flooding only after opening a sea connection. Cross-section tests check the basin?slope?shelf?coast?lowland?mountain progression and broad mountain support. Multi-seed tests bound neighboring elevation steps, require gentle passive coasts and substantial lowlands, and retain summit-height variation. Tropical reef tests require all four forms, less than 5% ocean coverage per seed and less than 3% across the sample. Other tests check lowland/depth variation, drainage termination and runoff conservation, habitat constraints and all four reef forms. Water-mesh tests check dry-cell exclusion, lake levels, depth shading and finite flat/globe coordinates. All eight presets are exercised in both physical modes.

Water balance, erosion, shelves and reef development are seeded procedural approximations. There is no seasonal lake-level simulation, tidal flooding or time-evolving coral ecology. Reef shapes are resolved at the map's cell resolution. The inherited ocean frame and physical-map size limit remain. WFC mode without physical generation keeps its original discrete terrain behavior.
