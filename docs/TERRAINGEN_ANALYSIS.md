# TerrainGenOnSteroids migration analysis

The migration targets the local source snapshot in `../TerrainGenOnSteroids` as it existed on 2026-10-07. That snapshot already includes a physical environment pipeline; this is not just the earlier rules-only generator. The adjacent platform specification documents describe a broader application and are not part of this reproduction's scope.

## Source map

| Reference | KRIEMHILD | Responsibility |
|---|---|---|
| `index.html` | `src/App.jsx` | Same page topology, labels, defaults, IDs, tab panels, and accessibility attributes; heading changed to KRIEMHILD |
| `style.css` | `src/style.css` | Original colors, spacing, 360px sidebar, controls, editors, 800px responsive breakpoint |
| `main.js` | `src/terrain/controller.js` | Original drawing, interaction, presets, persistence, exports; asynchronous Go session integration and effect cleanup |
| `wfc.js` | `internal/terrain/rules.go`, `solver.go` | Native configuration compiler, RNG, solver, brush, climate, continents |
| `environment.js` | `internal/terrain/environment.go` | Native physical terrain and environmental masks |
| `editor.js`, `patterns.js`, `presets.js` | `src/terrain/` | Preserved editor interactions, texture drawing and preset definitions |
| `view3d.js`, `svgexport.js` | `src/terrain/` | Preserved view/export algorithms with local package imports |
| `tiles.json` | `internal/terrain/tiles.json` | Unmodified default palette, embedded in Go and served through `/api/defaults` |

The reference logic and UI assets are derived from the supplied source. Reference files under `tests/reference` are used only as comparison fixtures and are never included in the frontend bundle.

## Generation semantics

1. A cell's domain is an unsigned 32-bit mask. The compiler symmetrizes adjacency when either terrain type permits a pair and computes shortest type distances for minimal brush repair.
2. Mulberry32 chooses cells and types. Numeric seed strings are converted to unsigned 32-bit values; other strings use FNV-1a over UTF-16 code units, matching JavaScript even for non-ASCII seeds.
3. Entropy selection chooses a random cell from the first nonempty option-count bucket. Random selection samples uniformly across buckets. Bucket insertion and removal order are preserved because they affect seeded results.
4. A neighbor's `weightNear` replaces the base weight; the largest matching override wins. Continental shares, blended climate multipliers, brush type-distance preference, and exponential stability then multiply it.
5. Propagation uses a LIFO work queue. Contradictions undo the pick, forbid it, then reopen connected unlocked cells with an expanding repair radius when needed.
6. Cleanup visits cells in seeded shuffled order, considers earlier changes in that pass, and respects pinned cells and environmental masks.
7. Brush paint tries progressively wider margins and preserves older paint until the fallback pass. Undo restores domains, locks, pins, and status. As in the reference, it does not rewind the RNG or counters.

## Physical pipeline

The Go implementation follows the reference sequence: continental influence and coastal geometry; land coverage cutoff; plate motion and relief; bathymetry; coastal distance; elevation-adjusted temperature and seasonal range; latitude winds; twelve moisture transport sweeps and rain shadows; priority-flood drainage; accumulation and lakes; groundwater, sediment, dunes, snow, glacier flow, reefs, wetlands, farming/settlement suitability, lava and ash; candidate terrain masks; WFC and cleanup.

**Corrected geography:** the supplied source's global elliptical falloff produced a single substantial island for all ten audited default seeds. Coastal falloff is now limited to the border and warped continental-point boundaries form ocean basins. Physical v3 further separates ground from water, adds lowland/seabed relief, water-balanced lakes/dry basins, and reef forms. Physical generation is validated through the new invariants; exact reference parity remains for WFC. The old physical-map pixel hash is historical.

Field writes retain the reference's float32 rounding, including intermediate hydrology and precipitation buffers. Heights remain quantized in four-unit increments. Math-library transcendental functions can differ at the last bits across engines/platforms, so numerical field comparisons use a small relative tolerance while discrete terrain domains are compared exactly.

Physical mode makes baseline biome adjacency unrestricted and removes legacy climate/continental weights; hard exclusions come from environmental candidate masks. Legacy controls are disabled or hidden while explicit latitude limits, physical inputs, neighborhood radius and point count remain active. The app defaults to WFC; presets now compose with physical/realism modes through themed environmental profiles plus the preset's terrain preferences. Preset selection no longer disables physics. Physical continental markers are forwarded to the renderer. Brush actions that violate environmental masks are rejected.

## Follow-up gap audit

| Gap found | Correction |
|---|---|
| Physical mode kept one central landmass across seeds | Border-only falloff and warped basins; ten-seed connected-component regression test |
| Physical mode silently discarded preset themes; preset selection disabled physical modes | Explicit physical profiles retain each theme; switches and preset selection compose; all eight presets audited in both physical modes |
| Settings reset on reload while the remembered preset name remained | Persist and restore generation/display settings; migrate older saved presets to their settings |
| Stability values above 30 rejected although the slider reaches 300 | Accept the reference's full 0–300 range; physical mode visibly limits its separate range |
| Physical continental marker toggle drew nothing | Send the macro layer's points with names/colors in session state |
| Renaming a terrain type removed its physical classification | Preserve `environmentType` through editor, normalization, JSON persistence and Go masks |
| Late preset responses could override a newer choice | Sequence preset requests and discard stale responses |

The reference files remain unchanged. WFC probability and propagation behavior, rendering, textures, brush repair, PNG/SVG and 3D modules remain implemented; their existing parity tests continue to run.

## Backend contract

| Endpoint | Behavior |
|---|---|
| `GET /api/health` | Application and native engine status |
| `GET /api/defaults` | Embedded palette |
| `POST /api/sessions` | Validate configuration and initialize a physical or rules-only map |
| `POST /api/sessions/:id/step` | Collapse up to `count` cells, yielding after a short compute budget |
| `POST /api/sessions/:id/cleanup` | Run up to `count` cleanup passes |
| `POST /api/sessions/:id/paint` | Apply constrained cells/type and refill |
| `POST /api/sessions/:id/snapshot` | Create an opaque undo token |
| `POST /api/sessions/:id/restore` | Restore a server-owned undo token |
| `DELETE /api/sessions/:id` | Dispose of a map |

Per-session mutexes and a client request queue preserve action ordering. Generation tickets prevent late responses from replacing newer maps. Each React mount owns an abortable listener scope and releases timers, animation, 3D resources and its backend session when unmounted. Requests have size/range bounds, unknown actions return JSON errors, and map construction has a concurrency limit.

Animation cadence and elapsed-time statistics include HTTP scheduling, so timing is not identical to synchronous JavaScript. The selected seed, algorithm operation order, final cell masks, visual geometry, and user-facing controls are the compatibility targets.
