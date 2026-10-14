# Validation record

## 2026-10-13–14 — ocean framing and causal Terrain/Biosphere revision

[Implementation and compatibility](TERRAIN_BIOSPHERE_PIPELINE.md) · [20-world visual gallery](landshape-gallery/index.html) · [CSV measurements](landshape-gallery/measurements.csv) · [shape/gap JSON](landshape-gallery/metrics.json) · [climate JSON](landshape-gallery/climate-metrics.json).

All twenty worlds use 160 × 100, 42% requested land and standard separation. Automatic placement produces 4–6 major continents and retries a lower count when necessary. Measured final land is **41.37–43.10%**, minimum major-continent gaps are **9.63 cells**, and **60.0–67.9%** of water is open ocean at least half the selected gap from land. The largest connected water body contains **97.73–99.73%** of water. Every world has **zero land cells in the reserved ocean**, including all four map edges. The gallery includes world silhouettes, biome/climate/precipitation/aridity maps, and continental/regional detail views of the same seeds.

| Seed | Major continents | Land | Gap (cells) | Open ocean | Subtropical dryland | Small dryland components |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| KRIEMHILD | 6 | 41.92% | 9.63 | 60.0% | 94.9% | 0 |
| 1 | 5 | 42.02% | 9.63 | 63.2% | 91.8% | 2 |
| 2 | 4 | 41.92% | 9.63 | 67.0% | 96.1% | 2 |
| 3 | 6 | 42.02% | 9.63 | 60.7% | 93.2% | 1 |
| 4 | 4 | 43.10% | 9.63 | 67.9% | 90.2% | 2 |
| 5 | 6 | 41.83% | 9.63 | 62.5% | 87.8% | 0 |
| 6 | 5 | 41.85% | 9.63 | 64.6% | 92.2% | 3 |
| 7 | 5 | 41.52% | 9.63 | 64.8% | 93.3% | 4 |
| 8 | 5 | 41.42% | 9.63 | 64.1% | 94.6% | 4 |
| 9 | 4 | 41.39% | 9.63 | 66.6% | 93.1% | 5 |
| 10 | 5 | 41.72% | 9.63 | 64.8% | 91.7% | 9 |
| 11 | 5 | 41.96% | 9.63 | 62.4% | 92.0% | 4 |
| 12 | 5 | 41.56% | 9.63 | 66.4% | 93.8% | 2 |
| 13 | 5 | 41.37% | 9.63 | 66.2% | 88.2% | 3 |
| 14 | 4 | 41.65% | 9.63 | 67.0% | 95.0% | 3 |
| 15 | 5 | 41.61% | 9.63 | 63.5% | 93.6% | 4 |
| 16 | 4 | 41.64% | 9.63 | 65.7% | 97.0% | 2 |
| 17 | 5 | 42.04% | 9.63 | 64.7% | 95.6% | 1 |
| 18 | 5 | 42.15% | 9.63 | 66.8% | 94.7% | 2 |
| 19 | 4 | 42.04% | 9.63 | 65.5% | 93.4% | 7 |

Subtropical drylands occupy 87.8–97.0% of land in the 20–35° band, compared with 13.6–52.4% at 0–10° and 2.0–29.4% at 50–65°. These are calibration outcomes, not imposed biome quotas. Tests cover regional maps, windward/lee fixtures, cold and warm drylands, persistence, and categorical detail consistency.

New regression coverage includes twenty foundation seeds; narrow/standard/wide gaps at 80 × 50, 160 × 100 and 240 × 150; all four ocean edges; irregular shorelines; deterministic shape variety; monthly actual evaporation and water yield; and deterministic basin-constrained currents. Conflicting high-land presets must emit a coverage warning rather than filling ocean reserves.

Biosphere scenarios cover photic versus benthic/aphotic depth, sunlight hard limits, suitable cell fractions, steppe and authored clearing, overlapping river channels and terrestrial banks, ocean barriers, horizontal wrapping, kilometre dispersal from survivable cells, flowering-dependent pollination, cold resident behavior, v1 compatibility, locked records, API acceptance/restart/ZIP, and matching authored cover in detail and climate snapshots.

The end-to-end [Biosphere audit](landshape-gallery/biosphere-metrics.json) uses the same 160 × 100 physical pipeline, sparse guild richness and realistic suitability. Species counts include procedurally generated draft analogues, not empirical taxonomic identifications. Empty or incomplete niches remain diagnosed rather than bypassing survival or dependency constraints.

| Seed | Layered habitats | Occupied habitats | Lifeform profiles | Occurrences | Valid links | Dark photosynthesizers |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| KRIEMHILD | 4873 | 1919 | 11229 | 20124 | 18197 | 0 |
| 7 | 4905 | 1882 | 11201 | 20207 | 18321 | 0 |
| 19 | 4384 | 2023 | 12996 | 21477 | 19444 | 0 |

All three audits pass state/relationship validation and have zero dangling spatial links. Reports retain model version and source hash, habitat sizes and strata, guild counts, maximum range size, diagnostic counts and representative exclusions. Niche templates are reused across compatible patches; native origins are persisted to make preview acceptance idempotent.

Final validation results:

- **Go tests:** all packages pass in the final combined coverage. The full `go test ./...` run exposed two terrain fixture assumptions: the annual-rainfall fixture needed to rebuild its monthly budget, and per-world river-junction counts needed to scale with the actual major-continent count. After correcting those fixtures, all **94 terrain tests and their subtests** were rerun in four disjoint, exhaustive `go test ./internal/terrain/... -run ...` groups. [Exact test inventory and results](landshape-gallery/terrain-test-results.json). The latest Biosphere, history and HTTP API suites also pass, including acceptance, restart and ZIP round trips. Optional PostgreSQL integration tests require a configured test database and were skipped.
- **Static analysis:** `go vet ./...` passes.
- **Frontend:** `npm test` passes **128/128** tests. `npm run build` passes; the largest JavaScript chunk is **515.35 kB**, below the unchanged 600 kB warning limit.
- **Browser:** realism, detail and biome audits pass, including all eight presets in both physical modes, hover size stability and consistent biome inspection across zoom levels. The named-host dependency editor also passes selection/removal, season entry and life-stage editing without browser exceptions. Audits used an isolated SQLite test server and Edge automation after the T3 collaborative preview explicitly reported no automation host.
- **Generated-world evidence:** all twenty terrain worlds and all three full Biosphere audits pass their recorded invariants. Gallery images and measurements were regenerated from the final physical pipeline.
- **Repository checks:** `git diff --check` passes; changed text artifacts are valid UTF-8 without doubled carriage returns.

The drainage checks still require valid connected networks, rendered junction geometry, river coverage bounds, per-continent tributaries and an ensemble mean longest river of at least 4,400 km. The fixture updates do not add artificial runoff or relax water conservation.

The earlier nine-seed layout metrics are superseded by this twenty-seed run. The sections below retain historical validation context.

## Opt-in geospatial realism

- Preset composition regression: all eight presets complete generation in both physical modes through the API and live browser, retaining their selected name, physical profile, terrain weights and neighborhood radius. Turning physics off restores reduced preset palettes.
- Mountain regression: four seeds in each physical mode produce prominent connected high-elevation regions at 160 × 100 resolution. Reduced relief decreases mountainous area. Actual generated ranges have multiple local summits with over 750 metres of summit-height spread and supporting neighboring terrain. Extreme heights remain distinct after elevation quantization. 2D shading displays ridges and physical 3D bypasses tile-height blur.

- New Go calibration tests pass for equatorial coastal rain, dry continental interiors, ocean moderation, mountain rain shadows, elevation cooling, warm shallow reefs, polar glaciers and wind-driven sand deposition.
- Four seeded worlds reproduce all fields and entities exactly. Tests enforce river drainage/discharge, strictly downhill lava, geological and ecological prerequisites, freshwater/farm-dependent settlements and WFC masks.
- `tests/realism-browser-audit.js` passed against the live Go service: switch routing, finished maps, all 75 environmental fields inspectable, climate/wind rendering, mode recovery, reduced-palette completion and settings persistence.
- The real-world map initialized the bundled 3D view without application errors.
- WFC reference parity is retained. Physical tests now validate the new water/ground separation, not the reference's automatic flooding of negative elevations.
- Detailed audit and limitations: [GEOSPATIAL_REALISM.md](GEOSPATIAL_REALISM.md).

- `npm run build`: passed (React/Vite production bundle; local Three.js and Delaunator chunks).
- `go test ./...`: passed, including landmass distribution, determinism, coverage and point-count regression tests.
- `go vet ./...`: passed.
- `npm test`: 17 groups cover WFC reference parity, continuous physical invariants, preset composition, API validation and flat/globe water geometry. Go tests additionally cover landforms, mountains, climate, drainage, dry/wet basins, varying lake depths and all four reef forms.
- `git diff --check`: passed.

## Historical browser comparison (before the landmass correction)

The comparison below recorded the first migration. It also reproduced the reference's single-central-island defect, so it is not a current physical-map acceptance test. WFC remains reference-compatible; physical geometry now intentionally differs.

At a 1280 × 800 viewport, both the reference and KRIEMHILD generated a 160 × 100 physical map with seed text `KRIEMHILD` (numeric seed `4139746970`), the default palette, Voronoi cells and textures. The RGBA bytes of their 960 × 600 map canvases have the **same SHA-256**:

```text
2fdc5fa77b9dd458fb79de5e5ed2aecb093735f12cbda7a73ab795c6a6c6e360
```

Both sidebar rectangles were x=920, y=0, width=360, height=800. This establishes pixel equality for this specific seeded rendering, not every possible setting or browser.

## Interaction checks

- All four tabs and expandable terrain controls work.
- Pause stopped generation; Step advanced the random-pick counter from 0 to 1 while remaining paused; resume completed generation.
- A grass brush stroke changed a completed rules-only map. Undo restored the exact pre-stroke canvas hash.
- PNG export produced a 142,612-byte image, SVG a valid 206,275-byte vector document, and palette JSON contained all 23 default types. Export downloads were intercepted in the preview during validation rather than saved outside the project.
- The bundled 3D module initialized a WebGL canvas without an external network dependency. Sphere selection set a 48 × 24 grid, disabled manual height and activated the globe viewer.
- At 390px width, the sidebar stacked below the map and the document had no horizontal overflow.
- No application errors were observed in the verified interactions. The preview intermittently timed out taking snapshots/resizing; those tool calls were retried, and DOM/API checks were used where necessary. A canvas readback performance warning came from the validation hash checks.

Reference fixture hashes are recorded in `reference-checksums.json`. The adjacent TerrainGenOnSteroids source was not changed. Existing deletions in KRIEMHILD's Git working tree were not restored or committed.

## Landmass regression

The original physical geometry produced one substantial landmass for all ten audited seeds at 160 × 100 and 42% land coverage. The corrected geometry produces at least two landmasses of 480 cells or more for every seed, while retaining exactly 6,720 land cells and an ocean frame. For seed `KRIEMHILD`, the two largest changed from **6,707 / 13** cells to **1,517 / 1,230** cells. Additional seeds tested: `1` through `9`.

Repeated identical settings produce identical fields and heights. Increasing continental points from 8 to 40 changes the geometry and produces more, smaller regions in the regression world. High coverage can still connect land into a supercontinent; the generator does not force a fixed number of continents.

The follow-up live browser audit (`tests/browser-audit.js`) passed all checks: physical-mode availability, real Go requests for all eight presets, maximum stability, point count and marker delivery, physical biome rename/persistence, and preset settings persistence. A page reload retained the Islands preset, rules-only mode, 40 points, `audit` seed, and 48 × 32 dimensions rather than reverting to unrelated defaults.

## Regional elevation and reef density revision

- `geology_test.go` checks a controlled continental cross-section, supporting terrain around a broad range, multi-seed adjacent elevation limits, gentle passive shores and substantial lowland coverage.
- Summit tests measure actual generated peaks and their neighbors, rather than an isolated noise multiplier.
- Tropical reef tests retain all four reef forms and habitat constraints while limiting coverage to under 5% of ocean cells per seed and under 3% across five seeds. The initial five-seed audit dropped from approximately 8,058 reef cells to 1,329 (about 84% fewer).
- Go tests/vet, all 17 Node test groups and the production frontend build pass. Preset composition remains covered in both physical modes.

The live browser audit passed all eight presets in both physical modes and confirmed all 75 field overlays. A 160 x 100 Earth map completed without errors; its terrain and flat 3D rendering were visually inspected after the rebuild.


## Hierarchical detail revision

- `lod_test.go`: exact parent anchors through eight bands; deterministic revisit/rebuild; shared tile-edge samples and lighting gradients; continuous shore profiles; unchanged water identity; protected drainage junctions and downhill exported stream profiles.
- `detail-camera.test.mjs`: cursor invariance under repeated zoom/pan, smooth scale transitions and visible-tile selection.
- Real API test: repeatable tile requests, shared edges, unchanged root elevations, invalid-coordinate rejection and session expiry.
- `detail-browser-audit.js`: real 2D detail loading, cursor anchoring and zoom return; 3D raycast anchor error below 1e-8. The eight-preset browser audit also passes in both physical modes.

The final detail suite also checks parent-triangle geomorph origins and clipped water triangles. All 21 Node test groups and the Go tests/vet pass. The globe viewer was exercised with cursor raycast error below 1e-8.


## Physical local relief revision

The original world/mountain generator is unchanged. Refinement now samples a coherent landform surface instead of independent octave noise. Terrain point markers are removed.

- `detail_landforms_test.go` isolates a flat, non-mountain parent: a cross-section resolves approximately -81 to +104 metres of relief, with a maximum adjacent step of 2.15 metres at 1/256 parent-cell spacing. It checks mountain-apron influence without moving parent anchors, physically lower river beds than banks, downstream widening, slope-dependent meanders, and different wetland/dune geometry.
- The complete Go suite and vet pass. Existing root-anchor, seam, deterministic revisit, climate, mountain and hydrology tests remain in place.
- All 23 Node test groups pass, including graded river-water clipping and variable-width ribbons without placeholder dots. Production frontend build passes.
- Shared-browser checks passed real detail loading, 2D cursor anchoring and zoom return, and 3D raycast anchoring (error approximately 2.8e-14). A local river-mouth view was inspected to check physical relief and remove river ribbons extending into open water.

These are procedural landform and drainage refinements constrained by the parent world, not a time-stepped erosion or ecosystem simulation. Horizontal dimensions still use parent-grid units.


## Selective drainage and portable world projects

- A shared visible-channel model uses upstream area, accumulated precipitation supply, seasonal melt, aridity and mountain/slope thresholds. Three 96 ? 64 worlds reduced visible reaches from 1,824/1,552/1,542 to 42/26/48 while preserving downstream connectivity. Artificial per-cell tributaries are removed; internal runoff remains available in physical fields.
- Go project tests restore stored world fields and edits after deleting the original cache, verify byte-identical detail without the detail generator, and require new descendants to inherit stored parent samples. Folder round trips, manifests, checksums, missing tiles, parent identities, duplicate regions and invalid paths are covered.
- `project-restart.test.mjs` deletes the original session, stops and restarts the Go process, imports the ZIP, and compares fields, painted pins, biome assignments and explored tile bytes.
- `project-browser-audit.js` exercised the real save button and ZIP input: a 12,677,536-byte ZIP restored 111 detail tiles, identical world/tile data and the original local zoom/camera after the original session was deleted. Import did not issue a generation request.

Final checks: Go tests and vet, all 24 Node test groups, and the production build pass. The live builder was also saved, its session/cache deleted, the actual local Go server stopped and rebuilt, and the captured ZIP imported into the restarted process: all 111 tiles and the local camera were restored without errors.
