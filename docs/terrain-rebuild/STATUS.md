# Continuous terrain rebuild — working milestone, not final acceptance

The complete 13-phase brief is **not finished**. This branch now contains an integrated,
running replacement path for new terrain. The old editors have now been retired;
remaining scientific, rendering and migration acceptance gates are still outstanding.
See [retirement scope and subsequent validation](RETIREMENT.md).

## Implemented

* Independent `terrain/model`, `generate`, `landmass`, `tectonics`, `detail`, and `tiles`
  packages. The common model does not import the old solver, HTTP, or rendering.
* Versioned specs and persisted spherical grammar; independent layout, morphology,
  tectonic, island and surface seed streams. Azimuthal local shape coordinates and
  spherical noise wrap longitude. Major shapes use asymmetric tapered strokes,
  branches, hooks, ribbons, fractures and post-union gulf/strait cuts. Curved island
  systems use interrupted, varied-size chains. Coverage correction is bounded.
* Reduced-order plate motion influences uplift, trenches and spreading ridges.
  Relief, shelves, bathymetry, monthly moisture transport, temperature, precipitation,
  PET, priority-flood drainage, accumulation, rivers and basin water planes are stored.
  Moisture transport conserves atmospheric water mass on spherical cells; rainfall is
  the condensed water removed, with finite soil storage and PET-limited recycling.
  See [climate equations, provenance and limits](CLIMATE.md).
* One coordinate sampler at every zoom, fourteen coast/relief frequency bands, shared
  tile halos, shoreline antialiasing, 256/512-sample tiles through level 18, cancellation
  and bounded concurrency. Server PNG cache is bounded to 64 MiB; client texture URL
  cache is bounded to 192 entries. Cache eviction cannot change sampled fields.
* Cancellable generation jobs, validation, stage timing, inspection, terrain textures,
  river vectors and explicit preview acceptance into the existing World/Age store.
  New terrain rejects legacy solver mutations and legacy detail-generation requests.
* Existing project, SQLite/PostgreSQL storage abstraction, historical snapshots, ZIP
  checksums, ownership and revision checks remain in use. Continuous sources are
  saved as versioned metadata plus binary fields, alongside the compatibility field
  projection. Loading uses those records, never regeneration. New ZIP schema is 5;
  legacy ZIP exports stay at schema 4. Unsupported model versions fail explicitly.
* React workspace with Generator, Map Builder and Settlement Builder; pan/zoom,
  inspection, styles, layer visibility/locks, point/path/polygon authoring, moving and
  vertex editing, sparse landscape operations, undo/redo and manual/autosave.
  Camera and appearance preferences are persisted. Style changes do not regenerate.
  Layer order, object color and creation/destruction years are editable.
* Settlement previews with slope/water checks, footprints, districts, streets,
  variable building polygons, layer LOD and stable parent/child identities. Preview
  acceptance checks the revision and validates a cloned state before committing.
  Acceptance is one undo entry, including settlements larger than the history limit.
  Parent moves transform all descendants atomically and honor child layer locks.
* PNG/SVG/PDF viewport exports at 1024/2048/4096 pixels and native World ZIP exports.
  Exports request terrain sampling at the output resolution and reject changed
  revisions. SVG keeps authored vector overlays and embeds sampled terrain textures.
* Globe and regional Three.js views use the same sampler and sparse edits as the map.
  Separate bed and water meshes preserve water elevations. Meshes are bounded to
  160×96 samples per view; this is not yet the complete adaptive 3D LOD system.
* Annual and monthly climate are projected into existing climate query/archive
  contracts for supported legacy planetary scales. Local edits mark analytical fields
  stale instead of invoking the old global climate generator.

## Retained deliberately

The new continuous workspace is the only active terrain generator and map/settlement
editor. The old frontend implementations and pre-existing terrain repair changes have
been removed. The backend compatibility package has moved to `internal/legacyterrain`;
it retains archive DTOs, validation, queries and historical fixture algorithms, but
production routes no longer invoke its terrain or detail generators. Old projects open
in a read-only archive viewer.

The World/Age graph, histories, writing, library, biosphere, entities, authentication,
collaboration and storage remain in their existing packages. Pre-existing uncommitted
terrain repairs were backed up outside the repository before removal.

## Validation and evidence before retirement

* Baseline JavaScript: 134 passed. Baseline production build passed.
* Final JavaScript: **136 passed**; final production build passed. All tests in
  `internal/httpapi`, `internal/world`, replacement terrain subpackages and
  `internal/settlements` passed. See `final-go-tests.txt`, `js-tests.txt`, `build.txt`.
* Baseline full Go command: other packages passed; `internal/terrain` timed out after
  601 seconds. See `baseline-go.txt`; this is recorded as a pre-existing failure.
* Replacement tests cover deterministic fields despite tile-quality changes, seam
  sampling, request-order/concurrent rendering, halos, cancellation, malformed archives,
  river water/discharge invariants, seam-crossing channel indexes, settlement siting
  and geometry, atomic parent/child movement, grouped undo after restart, sparse edit
  undo, cache eviction, and shared bed/water samples in map inspection and 3D meshes.
  Climate tests check monthly/annual sums, budget closure and a mountain rain shadow.
* HTTP integration tests cover legacy and new ZIPs, checksum-validated round trips,
  cold database restoration, inherited Age isolation and cancelled/invalid jobs.
* Browser audit used separate databases on ports 8135/8136. Preview generation and
  acceptance into World/Age succeeded. A 111-entity settlement was accepted and saved.
  A 300-metre raise changed the inspected bed by 300 metres; Undo restored it exactly.
  No application errors occurred in the tested interaction sequence. T3 snapshot calls
  failed with an automation error, so there is **no successful application screenshot
  audit**; DOM inspection and interaction were performed through T3 preview tools.
* The final server on 8137 generated, accepted, saved and reopened a fresh world.
  Globe and regional 3D initialized 1214×631 canvases with no WebGL/application errors.
  Export checks produced a parsed SVG containing eight embedded terrain textures,
  a decoded 1024×512 PNG, and a PDF with the expected header. See `browser-audit.json`.
* A 256-sample tile measured 985.5 ms cold and 3.1 ms from the server cache during
  concurrent gallery work. This is a development observation, not a comparison to the
  old renderer or a representative performance benchmark.
* `gallery/index.html` and `gallery/detail.html` contain reproducible world, silhouette,
  elevation, bathymetry, rainfall, material, coastline, island-area and local views.
  `metrics.json` records each seed; `metrics-summary.json` records aggregate results.
  Generation timings exclude adapter serialization and HTTP overhead unless explicitly
  listed. Exact final test outputs live alongside this file.
* Final ensemble: **50 seeds, 450 checked PNGs**; 33 of 35 Earthlike seeds have at
  least three major continents. Earthlike medians: land 25.5%, desert 10.5%, steppe
  19.5%, macro lake coverage 3.7%. Largest atmospheric budget residual across all
  seeds: 6.94e-13. Median generation time was 3.55 seconds during concurrent gallery
  work. These results do not establish morphology acceptance or a performance win.

## Remaining acceptance gaps

1. Climate is still a reduced model requiring calibration and convergence studies.
   The initial independent-rainfall implementation produced virtually no desert;
   the replacement conserves atmospheric moisture and derives rainfall from loss.
   Budget, rain-shadow and field consistency tests pass. Ocean heat transport, periodic
   soil equilibrium, adaptive seasons and coupled thermal/water budgets remain open.
2. Continental/archipelago grammar still needs visual refinement. Some seeds merge into
   two large regions; supercontinent coverage can be far below the requested preference.
   Coast/strait/peninsula and repeated-shape diagnostics are incomplete. Local terrain
   is continuously sampled but still lacks the requested richness of erosion forms,
   nested materials, fjords, deltas, reefs and island geological origins.
3. Drainage is a reduced macro graph, with local channel carving. It does not yet meet
   the full requirements for terrain-following meanders, endorheic water budgets,
   wetlands, deltas, sediment balance and tributary detail. River paths need better
   vector simplification at each display scale; straight macro channel artifacts are
   still visible in some island close-ups. Channel sampling wraps the seam;
   displayed polylines split there rather than drawing across the entire map.
4. New scientific resource/geology records and ecosystem habitat integration are not
   complete. Existing legacy records still open. Continuous climate queries work, but
   seasonal naming uses a basic annual cycle, not the full adaptive season system.
5. Adaptive 3D mesh LOD, authored-object overlays in 3D, complete illustrated atlas
   symbols, snapping and full viewport-filtered vector APIs remain unfinished.
6. Settlement layouts are an initial implementation. Ports lack full harbor/dock
   construction; mining, monastery, ruins, historical growth, advisory overrides,
   alternative-site comparison and complete lot/road topology need more work.
7. Some editing controls remain incomplete: full property schema, brush previews beyond
   the radius, global object search, geometry snapping and settlement growth controls.
8. Tile invalidation is local, but the server still samples tiles while holding a bounded
   worker slot and does not yet coalesce duplicate concurrent requests. Persistence
   currently duplicates compatibility fields; reducing that duplication requires
   finishing migration of the remaining platform callers.
9. The final old-engine deletion, full 3D/export parity, complete browser keyboard/focus/
   failure audit, PostgreSQL-specific testing, memory profiling and whole-suite terrain
   test completion remain outstanding.

## Reproduce

```powershell
.tools/go/bin/go.exe test ./internal/httpapi ./internal/world ./internal/terrain/model ./internal/terrain/generate ./internal/terrain/landmass ./internal/terrain/tectonics ./internal/terrain/detail ./internal/terrain/tiles ./internal/settlements -timeout 15m
npm.cmd test
npm.cmd run build
.tools/go/bin/go.exe run ./cmd/continuous-gallery -count 50
.tools/go/bin/go.exe run ./cmd/continuous-zoom-gallery -count 50
node scripts/summarize-continuous-gallery.mjs
```

Run the application normally with `npm.cmd start`; fresh terrain routes use the new
workspace. See the baseline README in this directory for rollback paths and tag.
