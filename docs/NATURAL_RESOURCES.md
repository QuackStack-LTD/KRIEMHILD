# Lithology, geological history and natural goods

New physical worlds run three ordered stages: existing terrain/climate/hydrology, inferred geological history, then natural resources. The latter two stages do not alter established mountains, rivers, elevations, coastlines or detail tiles. Rules-only maps and older saved worlds retain their original data without retroactively generating deposits.

## Stored geological model

`Environment.Geology` version 1 contains irregular connected province cell sets, overlapping basement/cover/intrusion formations, top-down rock strata, fault polylines and a cell membership index. Provinces reuse existing plate, stress, volcanic, shelf and mountain fields. The model includes stable interiors, ancient and young mountain belts, volcanic arcs, rifts and oceanic crust. Basement and cover are independent: sediments may cover granite, and uplifted marine strata may occur in mountains.

Each province has a stable seed-derived ID, rock, age in millions of years before present, formation process, ordered events and environmental history. History records former marine settings, ancient wetlands, restricted evaporative basins, past tropical weathering, organic preservation, burial, heating and traps. Historical climate is inferred independently of current climate. Weathering, erosion, sediment transport and soil properties consume the existing climate and downhill drainage graph.

Strata carry age, depth, thickness, rock, permeability, organic content and depositional environment. They include carbonate, organic shale, coal-bearing shale, sandstone reservoirs, shale/evaporite seals, banded iron formations and mafic/ultramafic cumulates. Fault fracture halos intersect the layered formations. These are lightweight procedural explanations, not a tectonic reconstruction or a subsurface engineering model.

## Resource rules

`Environment.Resources` version 1 contains actual generated occurrences and continuous potential fields. Occurrences store stable IDs, type, variant, geometry, cause, normalized abundance/potential, quality, confidence, accessibility, depth and causal references. Resource records remain invisible on the map. Region footprints overlap; springs, geysers and vents use point geometries. Wind and solar are explicitly renewable potentials, not ore deposits.

The catalog covers fossil fuels and geothermal energy; iron, copper, gold, bauxite, silver, lead, zinc, tin, nickel, cobalt, lithium, chromium, platinum-group metals and uranium; salt, limestone, gypsum, silica, clay, phosphate, sulfur, potash, gemstones and construction stone; aquifers, mineral/hot springs, geysers, submarine vents and freshwater; reef coral, timber, marine/freshwater fisheries, wildlife, fertile soil, peat, wind and solar potential.

Every candidate must satisfy its geological/ecological predicates before seeded mineralization variation can select it. For example, petroleum requires preserved organic source rock, sufficient thermal maturation, a permeable sandstone pathway/reservoir, an overlying seal and a trap. Coal requires buried ancient wetland strata. Laterites require stable weathered surfaces and appropriate parent rock. Construction stone takes its type from rock formations. Aquifers require recharge and permeable or fractured hosts. Springs reference existing supplied hydrological springs; their temperature and mineral chemistry are independent properties. Geothermal potential does not imply a spring. Reef goods require an existing reef and suitable warm, shallow, clear marine water.

Gold, tin and gemstone placers originate only from actual primary occurrences. Erosion supplies material which follows the stored flow graph into lower-gradient river sediment; each placer references its upstream source. No disconnected secondary deposits are added.

Amounts are dimensionless estimates, not tonnes or surveyed reserves. Ecological yields use coarse physical habitat, climate, soils and hydrological supply; species populations and mining economics are not simulated. Resource detail is currently anchored to the parent grid and does not create new deposits when zooming. Builder edits remain separate from the generated natural layers and do not automatically reroll them.

## Hover queries

Both Generator and Builder, in 2D and 3D, show terrain first, geological context and generated goods. The nearby search radius is configurable from 0 to 20 parent-grid cells; zero returns only intersecting occurrences. Local goods show abundance, while nearby goods show distance. The panel retains the last inspected position so it can be read or adjusted after leaving the map.

`GET /api/sessions/{id}/world/natural?x=12.5&y=8&radius=2` queries a disposable spatial index over stored occurrences. Coordinates use the existing parent grid (x east, y south); region cells occupy half-cell footprints. Results deduplicate by occurrence ID and sort local first, then distance and abundance. At most 64 results are returned, with total/truncation metadata. Queries never infer deposits merely from a suitable biome. Legacy worlds return `available: false`.

Requests debounce cursor movement, cancel obsolete requests and use a bounded browser cache. The normal rendering response omits the invisible layer payloads; server persistence retains them. Rendering resolution and detail tessellation are unchanged.

## Persistence and validation

Explicit **Save** stores both layers with the complete environment in the existing SQLite/PostgreSQL project transaction. ZIP schema 3 stores `natural/geology.json` and `natural/resources.json`, referenced by the root manifest and protected by its size/checksum inventory. Existing base, authored modifications and explored tiles remain separate. Import reconstructs stored records; it never regenerates them from the seed. Schema 1/2 worlds with no natural layers remain readable.

Validation checks versions, finite values, bounds, unique identities, reciprocal region membership, formation/stratum chronology, causal references, resource prerequisites, potential-field coverage and downstream placer connectivity. Inconsistent records produce an error identifying the affected record or cell.

Tests cover repeatable generation, unchanged terrain/hydrology, overlapping formations, local/nearby queries, eligibility failures, geothermal/reef prerequisites, primary-to-placer transport, invalid imports, maximum physical-world archive size, hover request races and exact ZIP/database restoration after cold restart. See [portable world projects](WORLD_PROJECTS.md) for the complete archive layout.
