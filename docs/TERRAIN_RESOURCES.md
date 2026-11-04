# Terrain Resources

Map Builder and Settlement Builder have a **Terrain Resources** button beside
3D Terrain and Medieval Illustrator. Click the terrain to place a circle, drag
to set its radius, or use the radius controls. Scroll to zoom and Shift-drag or
select Pan to move the map. Escape closes the resource preview. Preview circles
are editor overlays and are excluded from map exports.

Settlement profiles have a circular terrain preview, the complete settlement
outline, and an observed-radius slider. The minimum fits the irregular outline
with a small stroke margin; the maximum is 50 km from the fitted centre. This
applies to settlement records, including cities, towns, villages and hamlets,
and excludes political territories. An unplaced settlement explains that it needs
a saved map representation. A footprint requiring more than 50 km is reported as
unsupported, rather than being clipped or silently exceeding the limit.

## What survived and what changed

The retained `internal/legacyterrain/natural_*` implementation already had
geological formations, source/reservoir/seal/trap eligibility, deterministic
connected regions, resource provenance, causal eligibility validation on import,
and downstream placer transport. The new continuous generator's read adapter did
not populate these layers. Its old resource query therefore had no native
inventory to report.

`internal/terrain/resources.Populate` now connects the **completed** continuous
planet to that machinery. It runs in the production generation job after final
surface, climate, rivers and lakes, before accepting the generated session:

```text
saved continuous crust, tectonics, relief, climate and receiver graph
  → connected geological provinces and procedural historical episodes
  → basement, cover strata, intrusive bodies and faults
  → weathering, erosion and sediment transport; supplied groundwater
  → eligible mineral/petroleum/coal/evaporite systems
  → connected districts and resource occurrences; preserved endowment
```

It does not invoke the retired terrain solver, reroute the rivers, change the
continuous planet or construct another continent/elevation map. The retained
eligibility predicates run before seeded variation and validate every resulting
occurrence. Related commodities use a shared system variation field, rather than
independent map decorations. New catalog entries are appended, preserving the
indices used by old commodity-specific seeds.

Province records retain crust, age, setting, fault/deformation/uplift proxies and
chronological formation, deposition, burial, deformation, uplift and erosion
events. Past wetland, anoxic marine, evaporative and warm-humid environments are
procedural episodes constrained by the shared province/basin state. They are not
inferred solely from today's forest, desert or elevation. The continuous engine
does not retain a reconstructed plate time-series: these histories are explicitly
generated/inferred, with confidence and inference notes.

The catalog includes the requested energy resources, major metals and industrial
materials, groundwater, springs and geothermal potential. Added families include
manganese, rare earths, tungsten, molybdenum, diamonds, emerald, ruby/sapphire,
opal, garnet, amethyst, sand/gravel and granite. Existing quartz, generic gemstone
and construction-stone records remain readable. Diamonds require ancient cratonic
crust and a kimberlite transport host; gemstone families have distinct predicates.
Oil and thermogenic gas require the simplified petroleum chain; biogenic and
coalbed methane retain their explicitly different causes.

Each native occurrence has system and district IDs, host references, procedural
reason codes, confidence, extent in km², quantity with units, grade with separate
units, depth and exposure state. Sizes use bounded heavy-tailed seeded variation,
host extent and preservation. These are regional geological endowment estimates,
not extraction forecasts or surveyed reserves. Oil, gas, host material and aquifer
recharge have different quantity units and must not be added together. Ecological,
renewable-energy and surface-water potential uses explicitly indexed area units,
never mineral tonnage or an invented physical extraction yield.

## Water and Biosphere

The report separates saved river flow, dry-season flow proxies, nearby lake
footprints, spring emergence zones and estimated groundwater recharge. River flow
is interpolated only over the part of each saved segment inside the circle;
outside endpoint values are excluded. Connected
river reaches are never summed into a fictitious supply. Recharge over the circle
is a sampled annual-volume estimate. Groundwater follows supplied infiltration,
rock permeability and the saved receiver projection. Shallow alluvial, regional
fractured-rock, confined sedimentary, karst and potential artesian conditions are
distinguished; artesian potential additionally needs elevated supplied upstream
head. Springs require groundwater and an emergence pathway. Their chemistry
depends on host rock; geothermal potential and hot springs require elevated heat.

Groundwater flow/head, dry-season yield and spring circulation are simplified
regional proxies. Spring discharge is not presented as m³/s. Potability, pumping
sustainability, aquifer engineering and measured chemical analysis are not modeled.
Lake metadata bounds identify nearby footprint candidates; fresh surface coverage
is estimated independently from actual terrain samples.

The Biosphere generator was already functional, with saved habitat cell ranges,
per-cell conditions, occurrences, interactions and historical inheritance. It was
not placing individual organisms. Its native inputs lacked the geological layer,
and its water-body-ID assumption could confuse inland water and separate oceans.
The bridge supplies geology, soils and groundwater to habitat generation; the
physical ocean mask now distinguishes marine water from inland lakes. Existing
legacy records without an ocean mask retain the old ID convention.

The report intersects accepted habitat ranges and displays all present lifeform
groups, including plants, animals, fungi, bacteria, archaea, protists, algae and
viruses. Absent, extinct and deleted records are excluded. No organisms are
invented by opening a report. Accepted ranges retain their authored data; a
source-signature mismatch is shown as stale, rather than silently remapping them.

## Coordinates, persistence and limits

Resource and habitat topology uses the retained 192 × 96 parent grid. Native
coordinates are vertices: longitude `360*x/(width-1)-180`, latitude
`90-180*y/(height-1)`. Seam columns and polar rows have half footprints. Native
radius queries use this CRS; legacy cell-centre queries retain their convention.
The tests check a 50 m circle at a cell boundary and total spherical footprint
area `4πR²`, including the seam and poles.

This is regional geology: an Earth-radius parent cell is about 211 km tall. A small
circle can intersect the same regional occurrence or habitat as a nearby circle.
The terrain image and river-segment queries use the finer continuous snapshot.
No sub-kilometre ore-vein or individual-lifeform positioning is implied. Complete
deposit quantities are not prorated to the selected circle. All native grid fields
are available under `native.*`, preserving their own coordinate indices and units;
`surface.*` describes continuous samples. Empty legacy-only placeholder layers are
omitted. Biome proportions and main terrain/climate summaries use continuous
samples. Scalar summaries use
256 equal-area samples; categorical field means have no physical interpretation.
Large builder queries return at most 512 occurrences with an explicit total and
truncation notice. Settlement queries are limited to 50 km; builder queries to
5,000 km or half the planet circumference, whichever is smaller.

Authored surface operations appear in the terrain image and sampled surface
fields. They mark the base geological/river analysis stale; they do not invent
replacement fossil histories or silently recompute accepted biology. Regenerated
physical snapshots have different source identities and dependent resource layers.

GET routes are read-only and use the existing World/session view authorization:

```text
/api/sessions/{id}/terrain/resources?longitude=…&latitude=…&radiusKm=…
/api/worlds/{world}/settlements/{entity}/resources?age=…&radiusKm=…
/api/worlds/{world}/settlements/{entity}/resources/preview?age=…&radiusKm=…
```

Native inventories use resource schema 2, with a SHA-256 physical dependency and
generator `continuous-resources/1`. Old schema-1 inventories remain readable. The
dependency includes the spec, seed, native grid dimensions/fields, surface fields,
tectonics and drainage, excluding rendering tile size and diagnostic timings.
Restore/import checks the resource dependency against the decoded planet. The
existing saved environment retains all geology, water and resource data. Older
continuous snapshots can derive missing data in memory for the preview; the GET
does not save it or overwrite an existing World.

Large resource JSON inventories exceed the established 16 MiB per-file archive
limit. World Project archive schema **7** adds ordered, checksum-covered resource
JSON parts of at most 8 MiB, with strict names, order, count and reference checks.
The existing limits remain intact; schemas 1–6 still read, and smaller compatible
archives retain their established layout. Older binaries reject schema 7. Whole
World bundles continue to retain their existing snapshot/blob representation.

See [verification and handover](TERRAIN_RESOURCES_AUDIT.md) for commands, results
and limitations. Geological system categories are procedural simplifications
informed by [USGS mineral-system classifications](https://pubs.usgs.gov/of/2020/1042/ofr20201042_table1.pdf)
and [USGS porphyry/epithermal descriptions](https://www.usgs.gov/publications/porphyry-and-epithermal-mineral-deposits).
These sources do not validate the fictional quantities or physical proxies.
