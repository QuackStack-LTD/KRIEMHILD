# Portable world projects

Use **View → Save → Save world ZIP** after generation finishes. **Open world ZIP** imports that archive; **Open world folder** imports its extracted `KRIEMHILD` folder. PNG/SVG buttons remain visual exports and are not project files.

The ZIP is self-contained. It includes the original world, the actual generated detail tiles, their parent hierarchy, the stored detail model, and the current edits. The original Go process, session ID, browser cache and server tile cache are not needed to reopen it. Saving waits for visible detail requests and pending brush operations. Previously visited tiles remain in the server's project store even when the browser evicts them from its rendering cache.

## Schema 4 layout (schemas 1 through 3 remain readable)

```text
KRIEMHILD/
  manifest.json
  base/
    config.json
    solver.json
    environment.json
    fields/<chunk-x>/<chunk-y>.f64
  natural/
    geology.json
    resources.json
  climate/
    base/metadata.json
    base/cells/<chunk-x>/<chunk-y>.json
    authored/metadata.json                  # when terrain edits exist
    authored/cells/<chunk-x>/<chunk-y>.json
  detail/
    model.json
    tiles/<level>/<tile-x>/<tile-y>.json
  edits/
    world.json
  world/
    project.json
  view/
    builder.json
```

- **Manifest:** format/schema and detail-engine versions, stable world identity, original generation request/seed, dimensions, wrapping and coordinate conventions, field encoding/order, geographic region IDs and parent relationships, and each file's path, byte length and SHA-256 checksum.
- **Base:** the actual generated solver domains and constraints, palette/rules, environment settings, land/water mask, continents' control points, tectonic plates, named climate classifications, water-body memberships, reef regions and geographic entities. All physical fields are preserved, including elevations, bathymetry, water levels, drainage/catchments, climate, geology and biomes. Their stored values are loaded directly; generation is not rerun.
- **Natural layers:** new physical worlds store geological provinces, formations, strata, faults, history, resource occurrences and potential fields in the two manifest-referenced `natural/` files. Coordinates, identities, causes and overlapping memberships load exactly as saved. Older worlds omit these files and are not regenerated. See [lithology and natural goods](NATURAL_RESOURCES.md).
- **Climate:** manifest references identify the base climate and, separately, the climate calculated from authored terrain operations. Each layer has a versioned metadata file (resolved planetary settings, geographic scale, reference calendar, connected zone cell sets and stable IDs) and 32 by 32 cell chunks holding twelve monthly profiles, local modifiers and contiguous season definitions. The authored layer carries the terrain-operation signature. Import validates complete coverage, season ranges, measurements, checksums and references; it restores exact data without running the simulation. Older projects without climate remain without it.
- **Physical chunks:** 32 × 32 parent-grid cells, clipped at world edges. Values are float64 little-endian, ordered first by the manifest's field inventory and then row-major within each field. This preserves the original numerical values without lossy quantization.
- **Detail model:** stored geological apron and complete selected river geometry, identities, parent links, widths and grades. This supplies the immutable context for generating only missing regions.
- **Detail tiles:** actual 33 × 33 elevation samples, parent elevations, gradients, hydrological/environmental properties and anchored features. Every generated tile and its ancestors are included. Import restores their bytes; runtime requests load the stored tile first. New descendants inherit the stored parent surface, including differences from the procedural model.
- **Generator edits:** the current explicit biome assignments and pins, plus operation records, separate from the pre-edit generated base. The current snapshot is authoritative, including undo and cleanup. The original generator's visible brush undo stack is session-local.
- **Authored project:** `world/project.json` stores project metadata, layers, hierarchy, human entities, non-destructive terrain/water/vegetation operations, persistent Builder undo/redo and its camera/tool state. Schema 1 opens with an empty authored project. Composed detail uses the saved generated tile and these ordered operations; edits never overwrite the stored base.
- **Generator view:** the existing `view/builder.json` filename is retained for compatibility. It stores the generator palette/settings, display seed, rendering offsets, active 2D/3D mode and camera. Returning to an explored area uses its stored data.

## Loading and lifetime

Import locates the unique manifest and resolves paths relative to it. It validates checksums, array dimensions, physical fields, drainage references, parent-region links and shared parent elevation anchors before installing the project. It rejects missing files, duplicate identities, invalid paths and unsupported schema/detail-engine versions; it never silently regenerates corrupted data.

Physical base fields are loaded for the world overview. Detail is validated at import and restored into a disposable disk store, then read by geographic tile on demand. The display cache remains bounded. Missing detail is generated from the saved model and retained for the next save. Server storage under `KRIEMHILD_DATA_DIR/cache/` (default `data/cache/`, `/data/cache/` in Docker) is a working cache, not an external dependency of the ZIP. Session deletion removes that session's cache. A saved ZIP can be imported after the entire cache is deleted or the server restarts.

Click **Save** in the application header to commit a world to SQLite/PostgreSQL. Generation, edits, exploration and imports remain session-only until that action. Recent Worlds restores saved projects; Delete world removes the full project and explored detail after confirmation. ZIP export does not write to the database and import does not overwrite a saved version until Save. Repeated saves update the same world ID atomically. See [saving and deletion](AUTOSAVE.md) and [database configuration](CONTAINERS.md).

Completed physical and rules-only maps can be saved. Unfinished WFC solves must finish first. Current import/export limits are 256 MiB per upload/archive, 1 GiB expanded, 16 MiB per entry and 65,536 entries. The exporter refuses projects that would exceed the import limits instead of producing an unusable archive. Very large projects may require a future streaming format revision. Reopening older schema/engine versions requires an explicit compatible interpreter or migration, not a seed-only fallback.

## Verification

Go tests delete the original cache, create a fresh server instance, restore identical physical fields and edits, and serve stored tile bytes with generation unavailable. A deliberately distinct stored sample proves saved detail wins over procedural output; a newly generated descendant must inherit that sample. Folder and ZIP imports use the same hierarchy. Tests reject damaged checksums, missing tiles, invalid parents, duplicate regions and traversal paths.

The process-level integration test saves a world with explored detail and a brush pin, deletes its session, terminates the Go process, starts a new process and imports the ZIP. World fields, biome assignments, pins and explored tile bytes must match. The shared-browser audit exercises the actual save button and ZIP file input and verifies camera restoration without creating another generated world.

## Seasonal climate model

Physical generation adds `kriemhild-climate-1` after terrain, hydrology, geology and resources. It extends the existing annual temperature and advected precipitation fields with twelve monthly climatic means. Rainfall is conserved when distributed across the local orbital year. Orbital eccentricity changes orbital speed and forcing; axial tilt, latitude, continentality and water/forest moderation shape thermal curves. Circulation changes rainfall and winds. Soil-water and snow storage are spun up over repeated years. The model is a procedural climatology, not a weather forecast or a coupled atmospheric circulation solver; it does not move existing rivers or mountains.

The climate score `clamp(round(1 + 3T + 2P + M), 1, 6)` seeds an estimate. Curve calibration distinguishes arid, tropical wet/dry, temperate and cold regimes. Adjacent annual intervals merge by temperature, precipitation, snow and soil-moisture similarity. Custom mode retains the requested count by subdivision without changing any monthly values. Every phase records its start month, duration and name; intervals may cross the year boundary. A representative land calendar aligns hemispheres before aggregation; individual locations retain their own phases and actual timing.

In **World > Map > Climate, seasons & planetary scale**, select automatic or custom seasons, optional custom season names, and optionally set tilt, year length, eccentricity, perihelion, circulation, radius and longitude bounds. Empty planetary values are deterministic seed-generated settings. Missing geographic metadata uses an approximate Earth-sized planet and is labeled approximate. Coastal thermal moderation uses the geographic cell spacing in kilometres. Map width/height are simulation cells; canvas pixels are cells multiplied by Cell px. Resolution never enters the season-count formula. API settings live in `environment.seasonalClimate`; optional `rules` classify fictional zones by bounded mean temperature and annual rainfall.

Pin a point to inspect its annual summary and seasonal descriptions. These are templates over the stored profiles, including quantitative monthly means, precipitation totals, humidity, winds, transitions and local influences. Climate zones and local season-count overlays are available in both editors. Rendering responses omit full climate profiles; the existing point-query endpoint supplies only the selected profile and descriptions. Browser caches include the map revision so edits also refresh pinned descriptions.

The database stores base climate inside the compressed immutable environment part and edited climate as a separate `authored-climate` part. Historical map snapshots retain both, preserving independent Age states. A terrain-operation signature invalidates the edited climate; the next query or save recalculates the coarse climate across the grid, including downwind changes, without regenerating terrain or hydrology. This conservative full-grid invalidation avoids leaving stale neighboring rain shadows; local microclimate below grid resolution is not claimed. Saving remains explicit.


## Planetary coordinates and river evolution

New physical terrains default to an Earth-sized full planet. `environment.geography` records `width_px`, `height_px`, `pixels_per_cell`, `world_scale` (north?south kilometres per cell), `planetary_radius` (kilometres), `map_coverage`, `projection`, and optional longitude bounds. The implemented projection is equirectangular. Latitude bounds remain the existing environment settings. Full planets wrap longitude; regional boundaries can carry an explicit `external-drainage` continuation rather than inventing an ocean. Pixel dimensions must match the grid. A custom full-planet cell distance must agree with radius and row count; for regional maps it derives an extent around the selected latitude midpoint.

The resolved `geography` layer stores geographic bounds and exact spherical row areas. Drainage measures distances with great-circle arcs and weights runoff by physical cell area. `catchmentKM2`, `dischargeM3s`, and `drainageDistanceKM` accompany the legacy normalized fields. River records carry physical length, catchment, discharge and channel width. Climate uses the same scale. Parent-grid coordinates remain stable for maps, edits and stored tiles; rendering pixels do not redefine geography.

Network version 3 admits supplied catchments in plains, hills, wetlands, uplands and mountain regions. Configurable area/discharge thresholds govern channel initiation; an additional resolved contributing-area floor prevents single-cell runoff from becoming rivers. Basin routes and confluences determine river identity, while stream order controls small-stream visibility. Positive lake outflow can be supported by catchment rainfall or groundwater as well as visible inlets. Dry catchments cannot be promoted merely to satisfy a coverage target. Minor numerical pits may be breached by at most three metres per pass; deeper basins retain their water balance. Seam-crossing river geometry is split into map-edge pieces that share its network identity.

`environment.erosion` configures 0?6 passes, duration in million years and strength. The default is two bounded passes over two million years. An initial geological history supplies bedrock resistance and permeability before erosion. Stream power, slope, persistence, tectonic stress and available relief control incision; sediment is routed downstream and deposited where transport capacity falls. Elevation actually changes, temperature and precipitation are refreshed, and drainage/basins are recomputed after each pass. Broad mountain structure and supported gradients are retained. The final geology and resources consume the resulting surface and deposition.

The versioned `geomorphology` layer records sediment-volume budgets, formation references, incision/deposition, valley width, relative age and former routing. Recorded landforms include valleys, tributary gullies, alluvial floodplains/fans, supported entrenched gorges, terraces and abandoned channels where the computed course changed. This is a bounded procedural geomorphology approximation: durations and volumes explain the generated landscape, not a calibrated prediction of a real river. Fine meanders and braid morphology continue to use the existing detail system; the coarse grid does not resolve every cutoff, waterfall retreat or underground karst conduit.

Geography, geomorphology and the complete drainage graph are stored in the existing versioned environment metadata; numerical fields use the existing terrain chunks. Database snapshots, historical Ages and ZIPs restore these exact values and explored detail without rerunning erosion. Legacy environments without these optional layers retain their original surface and routing. Import validates spherical areas, physical units, sediment budget, landform IDs and geological references.
