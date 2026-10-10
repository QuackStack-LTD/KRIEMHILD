# Physical hydrology and biome transitions

New physical-environment worlds use a dedicated hydrology layer after terrain,
geology and climate are established. The existing mountain surface is preserved.
Rules-only generation remains available without this simulation.

## Water model

1. Derive permeability and lithology from geology, then precipitation,
   evapotranspiration, infiltration, aquifer recharge, baseflow, snowmelt and
   glacier melt from climate and terrain.
2. Use a deterministic priority flood to identify basins and spill elevations.
   Accumulate upstream water on drainage routes; calculate supported lake levels
   from inflow, rainfall, evaporation and infiltration. Unsupported depressions
   remain dry. Ocean-connected marine water is separate from inland depressions.
3. Reconcile routes with actual terrain and supported water surfaces. Natural
   flow cannot use an artificial uphill route across an unfilled depression.
   Lakes have connected internal drainage and a spill outlet or closed storage.
4. Select visible channels using contributing area, discharge, climate and stream
   order. Upstream water can sustain a river through a dry downstream biome.
   Annual and dry-season discharge distinguish perennial, seasonal, ephemeral
   and frozen-seasonal reaches. Insignificant runoff remains in the raster.
5. Derive springs from supplied aquifers and geological emergence; temperature
   and mineral composition are separate properties. Derive wetlands from
   saturation, slope, climate, permeability and nearby water. Coastal gradients,
   sediment, glaciers and river mouths determine coastal morphology.
6. Generate gravity-fed irrigation canals only where a settlement's dry fields
   have an available water source and a suitable route. Canals are a separate
   network. Locks, pumps and aqueducts can be described explicitly at route cells;
   the generator does not invent such infrastructure to justify an uphill canal.
7. Validate the graph before accepting a generated or imported world. Diagnostics
   identify the affected object and reason: uphill flow, drainage cycles, missing
   destinations, disconnected tributaries, unsupported lakes/springs/wetlands,
   invalid water budgets, excessive channel density or unjustified canals.

This is a deterministic annual/seasonal geographic model, not a time-stepped
fluid solver. Discharge units are 1,000 mm over one base-grid cell per year, not
cubic metres per second. Geological origins and coastal classifications are
terrain-conditioned models, not a simulation of geological history. Braids,
meanders, deltas and spring outflows refine the stored network; they do not
replace it when the camera changes scale.

## Materials and shore width

Physical material colors blend continuously using temperature, moisture,
vegetation, rock exposure, snow, sand and wetland saturation. A biome label no
longer switches an entire polygon directly from mountain to sand. Both the 2D
detail map and 3D terrain textures use the same material function and preset
colors. Explicit painted overrides remain supported.

Fine shore sand is limited to a narrow band next to the waterline, low height
above the water surface, suitable sediment and gentle slopes. Steep rocky lake
edges retain rock. Arid sand supply has a smaller contribution outside actual
dunes. These material changes do not flatten mountains or force a beach onto
every coastline.

## Persistence and compatibility

`Environment.Hydrology` stores versioned basins, water budgets, reaches,
upstream/downstream IDs, springs, wetlands, marine regions, coasts and canals.
Associated raster fields store drainage, watersheds, discharge, recharge,
permeability and other inputs independently of rendering. Project ZIPs include
these records and fields with the existing base-world data; explored detail
tiles retain their stored geometry and network references. Import validates
stored hydrology and uses saved detail before procedural generation.

Existing projects without this layer still load with their saved geography.
Generate a new physical world to obtain the new hydrology. Import does not
silently regenerate an older world's lakes, mountains or explored tiles.
Builder landscape operations remain explicit local overrides; this change does
not recompute an entire world's drainage after every authored terrain edit.

Tests cover downhill connectivity, wet/dry and warm/cold climates, distant
catchment supply, narrow shores, rocky lake margins, material continuity,
shoreline channel clipping, and cold ZIP restoration of the graph and detail.

## Continental networks and coverage

River selection now operates across each connected landmass. It ranks supplied
catchments by contributing area, accumulated water and distance to the outlet,
without a mountain-location bonus. Substantial basins retain their trunk routes;
smaller tributaries are selected toward a climate- and resolution-scaled coverage
target. Streams still need at least three contributing cells and real runoff.
The selector does not add water to satisfy a quota. Rivers continue through
supported lake outlets and dry downstream landscapes using upstream discharge.

The saved network includes whole river systems, their ordered reach IDs,
tributary/receiving-river links, connected watersheds and coverage targets.
Major, regional and local classes depend on length, catchment size relative to
the landmass, and discharge. Major routes are visible at world scale; local
streams retain zoom-dependent visibility. Channels keep the existing geometry,
width, incision and morphology functions. Existing terrain gradients are not
changed to force rivers through drainage divides.

Smaller supplied lowland and geological basins may now retain shallow lakes;
unsupported arid depressions remain dry. Basin origins can include spring-fed
storage or floodplain/oxbow depressions next to a curved supplied channel.
This classification describes an existing terrain basin, not a time simulation
of a migrating meander. River-connected low banks receive floodplain saturation
and can support inland marshes or swamps.

The **Hydrology report** is available in the Generation view's environmental
overlay panel and the Builder inspector. It reports whole-river counts and total
length per class, basin count/size distribution, water-body counts, coverage,
invalid destinations, and river length/catchment distributions. Lengths use
base-grid cells and areas use square base-grid cells; no kilometre scale is
assumed. Lake and wetland area targets are habitat diagnostics, not instructions
to flood unsuitable terrain. Major-route, tributary-connection, watershed and
accumulation overlays are available in both editors and in 3D terrain textures.

`networkVersion: 1` identifies this saved hierarchy. On reload, detail generation
reads the stored reaches instead of rerunning channel selection. Legacy projects
continue to load without the new report; generate a new physical world for the
new density and hierarchy. Coverage, river-system links and whole-river identity
are validated independently of their visual representation.


## Connected headwaters and receiving waters (network version 2)

New worlds anchor selected rivers to supplied mountain/glacier headwaters and trace their complete drainage routes. Lowland, forest and wetland runoff still contributes to catchment discharge, but no longer creates arbitrary disconnected visible starting points. Lake outlet reaches require a connected inlet and positive water remaining after modeled losses. Marine cells cannot drain into river channels. Rivers terminate in receiving rivers, lakes, marine water or terrain-defined terminal basins.

The selector follows supported upstream origins before tracing downstream. Lake storage that consumes its entire inflow stops that river rather than emitting a dry outlet. Coastal diagonal shortcuts through water are excluded; meanders are reduced where they would cross unrelated water. Outlet geometry starts at the lake shoreline. All reaches of a river use one visibility threshold, with tributaries appearing no earlier than their receiving river, so zoom does not expose isolated fragments.

Validation checks source support, supplied lake outlets, marine direction, receiving identities and reciprocal tributary references. Regression tests check complete routes, downhill detail geometry and exact interior junction endpoints. Network versions 0/1 remain readable; stored reaches and detail are preserved. Generate a new world to use the revised routing and source rules.


## Inland storage and readable shores

Coarse continental surfaces now resolve additional shallow storage basins in
suitable lowlands, glacial terrain and faulted regions. Candidate sites depend
on permeability, local relief and geological support; spacing and landmass
area bound their density. Excavation is shallow, preserves mountain cores and
coastlines, and obeys the existing gradient limit. Authored elevation grids
are not excavated. The existing annual water-budget solver determines which
basins actually retain water; basin creation never directly places a lake.

More supplied mountain catchments qualify as complete river systems, with
climate-scaled coverage targets and the existing inlet/outlet constraints.
Groundwater emergence recognizes smaller supplied aquifers at geological
contacts. Lake-connected low banks and convergent runoff can sustain inland
marshes and swamps; high or steep lake banks remain dry. Regression checks
measure water coverage separately on each sufficiently supplied landmass.

The generator and builder render one pixel per stored terrain node (33 by 33
pixels per tile), with standard image filtering. The 3D renderer scales that
image into its existing 256 by 256 texture before drawing river features;
it does not resample the terrain at higher resolution or use increased
anisotropic filtering. Terrain refinement remains tied to geographic zoom.

Generated shoreline samples retain narrow sandy beaches on supplied gentle
shores and stone or exposed rock on sediment-poor or steep margins. Inland
lake beaches remain narrower than marine shores. These narrow bands become
clearer as geographic detail loads at closer zoom. Stored projects keep their
geography; generate a new world for the revised inland storage and water coverage.
