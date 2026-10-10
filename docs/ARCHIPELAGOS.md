# Geological islands and archipelagos

New physical worlds include an island-foundation stage after continental relief and before ocean connectivity, climate, hydrology, lithology and resources. Islands are part of the same elevation/bathymetry grid as continents. Existing continental mountain generation and rendering resolutions are retained. Imported heightfields and saved worlds are authoritative; importing them does not add new islands.

## Controls

Under **World → Map**, **Island groups** sets the frequency multiplier from 0 to 3 (default 1). Zero disables the additional archipelago stage. **Coastal share** ranges from 0 to 1 (default 0.6): zero favors open-ocean groups; one allocates groups to continental margins and marginal seas. Suitable geological sites and available spacing limit these targets. The physical **Islands** preset uses frequency 2.2 and coastal share 0.45.

Both controls persist with generator settings and project saves. They apply on generation, without modifying an already saved world. Rules-only generation does not use these controls.

## Formation processes

- **Subduction arcs:** selected on the overriding side of converging plate boundaries with an oceanic plate. Chains curve toward the overriding plate and contain separated volcanic edifices.
- **Spreading chains:** oceanic divergent boundaries support raised volcanic ridge segments and drowned members.
- **Hotspot tracks:** existing hotspots and coherent oceanic mantle-plume provinces seed tracks along plate motion. Older edifices become lower and eventually subside; some tracks contain a single exposed island and a submerged companion.
- **Continental fragments:** former continental ridges on shallow shelves lose height and exposed area under inferred relative sea-level rise. Flooded saddles separate the remaining highs.
- **Shelf remnants:** resistant shelf highs form irregular clusters, with shallower drowned banks among exposed islands. Coast-distance gradients orient groups along continental margins. Surrounding shores identify marginal-sea settings.

Each group varies member count, spacing, curvature, orientation, width and height. Central members are often larger; surrounding members include small islands and submerged banks. Older and non-volcanic islands have more irregular coastlines than young volcanic edifices. Candidate sites are ranked within coherent geological provinces, then separated by irregular exclusion distances. A bounded land-area budget prevents the added islands from filling every ocean; whole foundations are accepted or rejected, never cut to a pixel quota.

Submerged foundations extend several island radii outward. Continental fragments have shallow shelves; volcanic foundations permit steeper submarine slopes. All profiles blend into the existing seabed and pass through the existing gradient limiter before drainage is computed. These are procedural geological approximations rather than a time-stepped tectonic or sea-level simulation.

## Atolls, ecology and geology

Subsiding volcanic members retain drowned rims and deeper lagoons. Mature rim segments can recruit coral only after normal checks for marine connectivity, warm water, shallow depth, light, clarity, salinity and substrate. Cold or freshwater rims stay geological banks rather than becoming coral reefs. Existing global reef-density limits remain covered by regression tests.

Climate, drainage, beaches, geology and resources consume the resulting surface. The lithology stage distinguishes hotspot basaltic provinces from subduction arcs. Island volcanoes reference their source archipelago, including arcs displaced from the plate boundary and older dormant hotspot members. No rivers or resource deposits are attached decoratively to island objects.

## Persistence and checks

`environment.archipelagos` stores optional group records with stable IDs, process, setting, plate references, source, path, cause and relative sea-level history. Each foundation records its stable ID, center, size, orientation, summit, age and emergence/subsidence stage. The complete numerical terrain is still the source of truth for actual coastlines.

These records and the terrain fields persist in SQLite/PostgreSQL saves and schema-3 ZIPs under the existing base environment. Explored detail remains stored separately. Legacy projects omit the records and remain loadable. Validation checks identities, geometry, plate references and tectonic support; restoration loads the saved data without planting new islands.

Tests compare island abundance across multiple seeds, exercise all five processes, coastal/open-ocean allocation, increased frequency, deterministic output, underwater gradients, protected continental relief, reef habitat and database/ZIP restoration.
