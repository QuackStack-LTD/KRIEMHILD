# Validation record

Validated on Windows on 2026-10-07 with Go 1.27.1, Node 24.21.0 and the T3 collaborative Chromium preview.

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

The live browser audit passed all eight presets in both physical modes and confirmed all 75 field overlays. A 160 ? 100 Earth map completed without errors; its terrain and flat 3D rendering were visually inspected after the rebuild.


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
