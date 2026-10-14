# Continuous terrain repair — 14 October 2026

The existing continuous terrain implementation remains authoritative. This repair does not restore the retired terrain editors or change Library, Characters, Writing, Biosphere, or their routes. The earlier retirement is commit `d21f684`. New generation uses `continuous-2`; saved `continuous-1` planets keep their original physical and biome rules.

## 1. Diagnosis and diversity

The reported overwhelming steppe classification was **not reproduced** by the 30-seed baseline. Steppe occupied 14.2–26.6% of land (median 19.4%). The active `biome` renderer never used the biome ID: it used the natural material calculation with a fixed shade. That made different climates look alike. The old morphology also shared one random stream and reused similar templates across several named families.

The renderer now maps actual IDs to explicit colors and supplies the same legend to the UI. Unknown categories have a conspicuous diagnostic color. Annual precipitation and PET both use mm/year; the conserved monthly moisture transport was retained. Added persisted diagnostics include soil storage, P/PET, seasonal variability, dry months, warm dry months, cold-season rainfall, wind exposure, rain shadow and slope. Seasonal classification distinguishes tropical woodland, maritime rainforest, Mediterranean shrubland, wetland and woodland without biome quotas or relaxing the original desert/steppe aridity thresholds.

Landmasses now use independent per-continent morphology streams, ten families, anisotropic proportions and shear, with lobes, cuts, island chains and varied orientations. Broad plateaus, inland depressions and climate-conditioned coastal/land detail extend the existing physical field.

## 2. Elevation and 3D

The contract remains signed metres relative to sea level, separate bed/water-surface/depth, radians internally and degrees at inspection/API bounds. Below-sea-level inland land is distinguished from ocean by the coast field. Renderers do not derive height from color.

`Flat` means unshaded 2D; `Relief` combines elevation with adjustable hillshade; `3D Terrain` renders canonical bed vertices and a separate water mesh. Exaggeration is a display control with a natural-scale setting. The previous fixed regional mesh has become visible camera-driven tiles through the existing mesh endpoint. Tiles have 65×65 vertices, a bounded GPU cache, cancellation, fallback geometry while loading and edge skirts. Camera movement requests higher levels without resetting the orbit. The near clipping plane follows viewing distance to avoid water/bed depth fighting in planetary overviews. Raycast UVs resolve back to the same geographic inspector and feature index.

Tests compare mesh samples to the canonical sampler, neighboring mesh edges and coincident parent/child vertices. A common 10-metre gradient stencil supplies matching normals across tiles and levels. The browser observed LOD 1 → LOD 2 after zooming and LOD 10 in a regional view. A second diagnosis found that the old local relief octave series began at approximately continental wavelengths. Mountain-conditioned ridges now add hundreds of metres of kilometre-scale relief while retaining macro anchors. Regional screenshots verify physical ridges at both natural and exaggerated scales. Larger geographic extents naturally have very small relief at true scale.

## 3. On-demand detail and caching

There is one equirectangular address system: level `z`, `2^(z+1)` columns and `2^z` rows, through level 18. The frontend chooses 256/512-pixel tiles from CSS viewport size and DPR (capped at 3), prioritizes the visible center, fetches parents, debounces changes and cancels superseded work. Cache keys include source identity, local edit revision, style, tile address and texture dimensions. URL textures survive builder component switches, with 192 cached entries.

Physical detail evaluates the same global coordinate function at every level. Climate/roughness-conditioned dunes, gullies, cold ridges and coastal cuts are derived from persisted parent fields, and their added envelope vanishes at macro-grid anchors. Request order, cache eviction and restarting do not become part of the seed. Canonical source data and edits persist in projects and ZIPs; no world-wide fine grid is generated.

The backend separates immutable physical halo samples from presentation. Eight halo tiles can be reused across palettes, and each entry deduplicates concurrent sampling. Prepared landmass trigonometric constants are tested to produce exactly the same elevations as the unprepared evaluator. Three server detail workers bound concurrent work. Encoded tiles/meshes have a 64 MiB memory cache and a checksummed 512 MiB disposable disk cache, with content-derived source and edit keys. Local operations select only overlapping mesh/tile dependencies. Old-version response headers report their actual generator version. Opening the 3D modal suspends underlying 2D tile requests so they do not compete for the same worker slots.

The physical sample cache is additional memory, not part of the 64 MiB encoded-byte budget; eight 512-pixel halo tiles use roughly 210 MiB for sample structs before indexing/runtime overhead. Disk cache eviction does not remove source geography or authored work.

## 4. Rivers

Baseline worlds already contained 248–698 persisted reaches. The old display had tiny uniform lines, no geometry-based river picking and no meaningful overview/local hierarchy. A high 1,800 m³/s threshold also discarded smaller drainage reaches.

New worlds retain channels above 300 m³/s, persist Strahler order, and refine saved reaches within their valley corridors while preserving endpoints, confluences and monotonic water profiles. The same paths feed carving, normal rendering, illustration and inspection. Overview visibility uses source discharge and stream order; continental and local views reveal more reaches. Line width combines physical width with a restrained optical minimum. Longitude-crossing paths split for display while retaining source identity. Inspector output includes discharge, width, destination and stream order.

## 5. Shared Medieval Illustrator

The old exporter had actually been deleted. History at `terrain-before-retirement-20261014` supplied the useful polygon interior-anchor strategy. The restored mode uses one `IllustrationScene` and one persisted `world.Illustration` model in both Map Builder and Settlement Builder, on the existing terrain canvas.

World, continent, country and city scopes select different source detail. Countries, capitals, settlements, routes, districts and buildings retain source IDs. Generated labels follow source renames unless explicitly overridden. Country labels try contained positions; labels with no collision-free fit appear in a manual-placement list. Labels, annotations and symbols can be moved, lines/polygons have editable vertices, and properties can be changed without modifying physical geography. Layer visibility, lock, opacity and ordering are stored separately. Relief follows its illustration layer controls. Controls wait for pending writes to prevent newly created annotations from overwriting subsequent text edits.

Presentation includes two visual languages, parchment, relief symbols, coasts, canonical waterways, settlement symbols, a title, compass, frame and latitude-correct scale. SVG/PNG/PDF use the same scene, with editor handles removed and raster terrain sampled at the requested output size; World ZIP stores editable illustration state. Undo/redo uses existing world history. Archive tests verify illustration/source separation and ZIP preservation.

## 6. Picking and editing

Picking now explicitly converts CSS pixels through the SVG's actual letterboxed viewport to normalized geographic coordinates and project coordinates. Hit tolerance stays in screen pixels. A geometry index handles polygon interiors, narrow line distance and points, with stable priority and Alt-click overlap cycling. Visible entities are filtered by viewport, layer and detail; the API can return the complete inventory rather than silently stopping at 2,000 objects. The index rebuilds after geometry changes. Land inspection identifies contributing landmass geometry rather than the nearest center.

The browser selected a country, road and capital, then selected again after resizing. A natural river overlapping the fixture correctly took line priority; the fixture test then hid the river layer to isolate authored-entity selection. Unit tests exercise zoom widths down to 0.00008, pan, letterboxing, resize, overlap priority and geometry changes. Those tests use CSS coordinates; DPR affects raster sampling, not pointer coordinates.

## 7. Seeds

The input starts blank, with random generation as the default. Each blank request gets a new cryptographic seed, including API requests that omit it. The input remains blank after generation. Explicit strings, including whitespace and Unicode, remain exact. The actual persisted seed is displayed separately with copy/reuse controls. Clearing an explicit seed restores random behavior. API checks generated two different blank-seed worlds and two identical explicit-seed sample sets.

## 8. Verification

- JavaScript suite: 64 tests passed, including six repair regression tests.
- Production Vite build: passed.
- Active Go packages: HTTP API, world, continuous terrain, settlements, storage, history, auth and biosphere passed.
- Additional terrain checks: seasonal classification, prepared-field equality, cancellation, ordering, shared mesh edges, parent anchors, disk-cache restart/checksum/budget, illustration history and ZIP round-trip passed.
- An extreme 100,000 km radius, 12-continent wet planet produced 5,260 refined river reaches and 8.5 MB of metadata. Its round-trip regression exposed the old 4 MiB metadata limit; the bounded 16 MiB envelope now passes with full field/reach validation retained.
- `go vet` on changed backend packages passed.
- Thirty Earthlike seeds and ten special-preset samples generated; all repaired audit samples validated through encode/decode.
- Browser checks passed for selection/resize, illustration annotation editing, undo/redo, save/reopen, SVG and World ZIP downloads, and 3D refinement. The T3 browser was used initially; after its host disconnected, local headless Chrome completed interaction verification.
- The repository-wide `go test ./...` exceeded its default 10-minute package timeout in unchanged `internal/legacyterrain`. The reported mountain test passed separately in 134.66 seconds. This is not recorded as a clean full-repository run.

Logs, screenshots and machine-readable results accompany this report. The audit used an isolated database under `.tools/repair-audit`; the user's main database and running server were not replaced.

## 9. Ensemble and visual review

Open [the comparison gallery](index.html), [summary metrics](summary.json), [baseline gallery](baseline/index.html) or [repaired gallery](after/index.html). Each of 30 seeds has world materials, coastline, elevation, biome and two coast-detail images. Special presets cover archipelago, rugged, supercontinent, arid and ocean with two seeds each and the same preset settings as the UI.

| Measurement | Baseline | Repaired |
|---|---:|---:|
| Steppe median | 19.4% | 18.1% |
| Steppe range | 14.2–26.6% | 12.9–23.2% |
| Desert median | 9.5% | 9.5% |
| Seeds above 45% steppe | 0/30 | 0/30 |
| Seeds above 60% steppe + desert | 0/30 | 0/30 |
| Major continent median | 4 | 5 |
| River reach median | 466 | 1,247 |
| Maximum moisture budget relative error | 5.84e-13 | 5.67e-13 |

The added diagnostic fields are present in every repaired sample. Reports retain monthly temperature/rainfall/PET, global and 15-degree latitude-band shares, connected-component shares, mass family parameters and component area/compactness/orientation/aspect ratio. These are area-weighted macro-grid measurements. Median biome shares are independent summaries and do not sum to one.

Visual contact sheets show the full ensemble without replacing unattractive seeds. They show a broader range of stretched, compact, branching and divided landforms and clearly differentiated biome regions. Polar projection stretching remains visible. Some close-up coast samples are on snowy northern margins; these are deterministic samples rather than a curated collection of attractive beaches.

Reproduce repaired evidence with `go run ./cmd/terrain-audit -count 30 -images -out docs/terrain-repair/after`, then `node scripts/terrain-repair-report.mjs`. Use `-preset NAME -count 2` for each special preset. Baseline evidence was captured by an executable built before the repair, not by pretending the current renderer is the baseline.

## 10. Timings and remaining limits

[Cold/warm measurements](timings.json) recorded a 256-pixel tile at 1,412 ms cold and 2.13 ms warm; recoloring the same physical samples took 24.7 ms; a mesh including shared gradient data took 451 ms cold and 3.67 ms warm. [Restart measurements](restart-timings.json) confirm disk hits after restarting the server: the first tile request took 59.1 ms, subsequent tile reads about 2.7 ms, and mesh reads about 5.9 ms. These are individual localhost observations, not percentile guarantees. Concurrent browser export/saving caused substantially higher wall times in [the contention sample](optimized-timings.json). The 30-seed generation median was 5.12 seconds during concurrent audit/test work, versus 3.37 seconds for the baseline run; these runs are not controlled performance comparisons.

Limits that remain relevant:

- Climate and the drainage graph are still a 192×96 global approximation. Fine terrain is deterministic conditioned geometry, not a full erosion, sediment-transport or fluid simulation. Zooming does not dynamically invent an unlimited tributary network or physically simulated deltas/braids.
- The 3D viewer renders terrain/water and resolves ground picks to shared entities; it does not yet extrude all authored buildings or render a full entity scene in 3D. Skirts hide mixed-level cracks and shared finite-difference gradients supply normals; transitions do not use vertex morphing.
- The illustrator's editable coast/symbol scaffold is sampled for its saved scene extent. Underlying raster terrain refines during zoom, but vector coast geometry is not an unlimited adaptive vector coastline. Automatic city layout, arbitrary print-page composition and every hand-drawn symbol variation are not implemented here.
- The audit measures component silhouettes and macro coast length. It does not yet provide validated semantic peninsula/gulf counts, multi-scale coastline fractal measurements or a robust rotation-invariant duplicate-silhouette detector.
- Entity inventories are loaded completely, then indexed/filtered locally. Very large authored worlds still need paged viewport loading. Selection invariance is covered by unit tests and representative browser interactions, not an exhaustive device/projection acceptance matrix.
- Cold detail still requires computation and can take seconds under load. The entire brief's sub-second latency and cinematic close-up quality should not be treated as proven by these tests.

These limits are explicit; this report does not claim every scientific or visual acceptance criterion in the brief is complete.
