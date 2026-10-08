# Geospatial realism implementation

Implemented from the supplied `../KRIEMHILD_Climate_Geospatial_Realism_Guide.md` (the actual filename in the workspace), particularly its causal pipeline, acceptance criteria, debug layers and calibration scenarios.

## Enable

In **World → Map**, enable **Real-world geology & climate**. This also enables Physical environment. Both modes layer onto the selected preset; selecting another preset preserves the switches. Presets supply themed physical inputs while terrain weights, neighbor preferences and neighborhood radius remain active inside physical constraints. Turning realism off retains physical generation; turning Physical environment off restores the named preset's original WFC palette. Reduced presets gain any missing standard biome types in either physical mode. Custom types and renamed biome identities are retained within the 32-type limit. Preset selection, switches and physical controls persist together.

All eight presets have explicit physical profiles in `src/terrain/presets.js`. Islands uses 25% land and tropical latitudes; Frozen North uses cold northern latitudes; Endless Desert uses subtropical latitudes with low rainfall; Villages and Fields favors moderate temperatures and lower relief. World of Lava increases volcanic activity, heat and land coverage while keeping lava tied to volcanic sources. Profiles initialize visible, editable controls rather than invisibly overriding user settings.

Both physical modes now generate stronger connected tectonic mountain belts. A shorter coastal uplift falloff preserves ranges on narrow continents and islands. Elevation shading reveals ridges even under snow or forest, and physical 3D elevation bypasses tile-height blurring. `mountains_test.go` checks connected ranges, peak heights and response to the relief control across four seeds and both modes.

API clients set `environment.realism: true` on `POST /api/sessions`. Realistic sessions report `kriemhild-realism-v2`; basic physical sessions report `kriemhild-environment-v3`. Both use the [continuous terrain and water model](CONTINUOUS_TERRAIN.md), including local lake levels and dry below-sea-level basins.

## Guide audit and changes

The current engine already supplied continuous elevation/bathymetry, synthetic moving plates, latitude/elevation temperatures, seasonal continentality, prevailing winds, priority-flood drainage and physical candidate masks. These were stronger than the guide's original five-band WFC baseline, but important causal and presentation gaps remained.

| Guide requirement | Implementation in realism mode |
| --- | --- |
| Continuous climate and ocean moderation | Existing elevation lapse rate, coastal current proxy and inland seasonal range retained. Twelve derived climate categories plus ocean replace legacy band labels. |
| Wind, moisture and rain shadows | Smooth circulation transitions; fractional meridional transport replaces rounded-away wind components. Calibrated moisture retention allows distant mountains to intercept moisture. Uplift removes water from air before it reaches the lee. |
| Seasonal climate classification | Local summer/winter precipitation proxy and growing-season suitability distinguish rainforest, monsoon, savanna, hot/cold desert, Mediterranean, oceanic, continental, boreal, tundra, ice cap and alpine conditions. |
| Tectonic geology | Synthetic convergent/divergent/transform boundaries, collision relief, trenches/ridges and seeded hotspots retained. Subduction vent suitability selects an overriding plate. Regional maxima produce discrete vents, with seeded active/dormant state. |
| Volcano → lava/ash | Explicit volcano entities include origin, activity, lava path and ash deposits. Active land vents send lava strictly downhill; ash follows the spatial wind field. Submarine vents do not paint terrestrial lava or ash. |
| Rivers and groundwater | Explicit river segments contain source, mouth, path and discharge. Segments join at confluences and terminate at water bodies; accumulated runoff increases downstream. Freshwater distance influences groundwater, fertility and human geography. |
| Glaciers | Existing snow accumulation, summer ablation and cold downhill ice propagation retained; snow mass balance is separately inspectable. Elevation allows cold tropical mountains. |
| Dunes | Sand comes from eroding sedimentary rock, coasts and river/lake margins. Fractional wind transport and terrain/vegetation trapping produce deposition; aridity, vegetation, slope and temperature constrain dunes. Connected dune fields share wind-derived ridge orientation, rendered on the map. |
| Reef habitat | Warm, shallow, clear, saline, well-lit ocean habitat is constrained by substrate and slope. Coastal shelves and submerged volcanic rims produce fringing, barrier, patch and atoll regions, preserved by WFC. |
| Mesas | Layered sedimentary geology, resistant caprock, erosion, relief and aridity jointly constrain suitability. |
| Wetlands and oases | Drainage, groundwater, freshwater proximity, slope, temperature and aridity constrain placement. |
| Farmland and villages | Fertility, growing season, moderate climate, water and accessible slopes determine scores. Spaced villages require nearby suitable farms and fresh water. Farms/villages now become actual terrain instead of unused scores. |
| WFC | Environmental masks restrict candidates; suitability multiplies local selection weights. Neighbor boosts and stability retain local coherence. Cleanup and painting remain bounded by environmental masks. |
| Rendering/debug | All 75 numerical fields have overlays, including climate, wind, geology, sediment, snow, water surfaces/depths and reef forms. Hover reports ground elevation and local water level/depth. The shared map texture includes relief/depth shading, rivers, dunes and vents; 3D water covers only actual water bodies. |

## Validation

`internal/terrain/realism_test.go` checks deterministic fields/entities across four seeds; finite values; downstream drainage/discharge; downhill lava; reef, dune, mesa, oasis and settlement prerequisites; and WFC mask preservation. Controlled elevation maps separately test wet equatorial coasts, continental drying/seasonality, a perpendicular mountain rain shadow, tropical reef habitat, polar mountain glaciers, and directional sediment transport.

The controlled eastward-wind continent produced approximately 1,270 versus 15 mm/year at coastal/interior sample locations. Adding a mountain ridge produced approximately 1,932 versus 22 mm/year at windward/leeward samples. These are regression calibration outputs, not climatological predictions.

`tests/realism-browser-audit.js` exercises the actual switch/API, generation completion, overlays, physical/rules-only modes, reduced-palette extension and persistence. WFC retains reference parity; physical tests validate the new continuous-surface invariants.

## Scope of the approximation

This is a lightweight procedural model. Seasonal rainfall, currents, geology, erosion and basin water balance are deterministic proxies. Closed dry basins drain internally; overflowing lakes use spillways. River discharge uses relative runoff units. Dunes and reefs have connected regions, without time-evolving sediment or coral growth. Plate boundaries are synthetic; evolving island arcs, migrating hotspot chains, roads and cities are not simulated. Glaciers remain suitability fields/terrain.

The inherited four-sided ocean frame remains in sphere mode, and physical maps retain the 24,576-cell limit. PNG/3D use the environmental map texture; SVG remains a flat biome-region export. WFC retains source compatibility. Physical terrain and hydrology intentionally supersede the source's single-island and below-zero-means-water assumptions.
