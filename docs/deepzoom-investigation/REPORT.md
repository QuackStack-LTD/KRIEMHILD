# Deep-zoom and new-world climate investigation

22 October 2026. This is a continuation of [the viewport investigation](../viewport-investigation/REPORT.md). All changes are uncommitted on `rebuild/continuous-terrain`, HEAD `2e4f5e4ede02b69e4cdba9c525212c6d7d827820`.

## 1. Outcome and acceptance

The atomic renderer remains the default. Confirmed repairs cover wrapped sample identity, recovery from corrupt cached PNGs, request ordering across styles, and batched wheel events with a moving pointer. The source-resolution limit is now explicit. Complete coarse and fine frames remain atomic; the source grid and three-worker concurrency were preserved.

Climate version 2 corrects a demonstrated elevation-transfer error and its dependent PET/moisture summaries. It is an **explicit experimental new-world option**, not the default. Versions 0 and 1 retain their behavior. The seven-seed comparison does **not** establish that version 2 solves the complaint about too little temperate land. It frequently increases rainforest. No biome thresholds or area quotas were changed.

**Acceptance remains incomplete for universally fast high-DPI rendering, genuinely finer local geography, and the broader climate/vegetation balance.** The evidence below supports retaining the specific repairs, not declaring those larger goals solved.

## 2. Repository, environment and recovered worlds

[Inventory](inventory.json) records the initial branch, commit, dirty tree, complete terrain specifications and saved-world linkage. Previous viewport changes were already present. The user's original database was opened read-only and backed up consistently; authoring tests used `.cache/deepzoom-20261022T122540Z` on port 8125. A second independent initial snapshot was used on port 8126 for revision-matched comparisons. No user world was regenerated, migrated or edited.

The new history worlds are **dc** and **jjb**. Neither has a saved terrain link in this snapshot. A newly saved continuous terrain project, `977d134ee5aadc480e1410ade3c84587`, does provide the requested clue: seed `d1feae98-6510-4a72-951b-44e93a01ae86`, continuous-3, climate version 1, Earth-like, rainfall 1.65, relief 0.8, radius 6371 km, tilt 23.4°, surface 4096×2048 and climate/drainage grid 512×256. It is not established that this unlinked project is the preview of either named history world.

The original user server on port 8124 was not restarted. Its backend predates these Go changes. The current test backend on 8125 supports the new option. A generated, accepted and reopened version-2 example is recorded in [climate-ui/results.json](climate-ui/results.json); its URL is `http://127.0.0.1:8125/world/654eeab56b475687a27310c565a72de7/build?tileDebug=1`. This is disposable test data.

## 3. Reproduction and manifests

Windows, 1280×800, DPR 1, actual SVG 892×695, natural + 0.55 hillshade, source revision 0. The geographic target is around x=1.3, y=0.22. Camera widths: 2, 0.25, 0.03, 0.003, 0.0003, 0.00008, then out and return. Logs record actual cameras, image bounds, generation, revision, selected LOD, network timing, cache state and output dimensions.

[Native-browser baseline](browser-before.json) and [headless baseline](browser-measured/results.json) preserve the original observations. The collaborative browser later explicitly reported that no automation host was available; subsequent checks used the repository's Playwright/Chromium harness. Background native-tab runs were slow and are not mixed with foreground performance results.

The wheel-driven A/B runs did not always produce exactly matching cameras. Those UI timings are retained in `browser-measured/` and `browser-after-verified/`, but are **excluded from a controlled performance claim**. `browser-after/` is an interrupted experiment. Exact geographic comparison instead uses the opt-in `tileDebug=1` camera-restoration event and an untouched snapshot; see [matched/results.json](matched/results.json).

## 4. Confirmed causes and retained repairs

| Cause | Reproduction | Repair |
|---|---|---|
| Integer-equivalent longitude halo addresses took different floating-point wrapping paths | LOD-18 shared-edge test disagreed on ice/tundra at a threshold | Canonicalize sample indices before conversion to radians; preserve unwrapped coordinates for edit-query bounds |
| A successfully downloaded but corrupt PNG remained cached | Decode failed; manual retry decoded the same corrupt blob again | Evict only that exact failed cache entry; preserve unaffected cached nodes |
| Whole-viewport style ordering discarded reusable physical data | Backend holds eight physical nodes; base requests for more nodes evicted samples before shading requested them | Fetch required styles together per node, using the same three workers; retain encoded blobs, not extra atlases |
| A complete fine cache still waited for a coarse render/encode | Warm logs recorded avoidable coarse encoding, including a 959 ms observation | Bypass coarse work only when every requested fine node/style is cached |
| Pointer conversion and state update could use different cameras during batched moving-pointer wheel events | Frozen old formula differs from sequential expected camera by 0.0815268 world units; fixed browser agrees to 1.11e-16 | Compute the pointer inside the functional update, using that update's current camera |
| Climate resampling applied an atmospheric lapse rate to negative seabed depth | Controlled fixture failed with 23.220621°C maximum error; saved-world coastal site was 14.89°C instead of 25.52°C | Version 2 interpolates sea-level temperatures, reapplies positive atmospheric elevation, then recomputes dependent PET/dry-month summaries |

Derived raster cache keys now include `raster-2`, so old disk images cannot survive the sampler correction. Terrain identity and saved source versions are unchanged.

## 5. Ruled-out hypotheses and failed experiments

Requested LOD increases through 1, 4, 7, 11, 14 and 16 in the tested sequence. No LOD ceiling, output budget or atlas budget was reached at those cameras. The fixed surface, not a renderer-only LOD cap, limits additional relief information. Equal-density parent/child tests and independently rendered edge tests still pass; the longitude precision case above was the new exception.

The first fixed-pointer browser test was invalid: synthetic wheel events rounded fractional pointer coordinates. Its reported 56 km drift must **not** be attributed to the product. `anchor-before/`, `anchor-final/` and the failed test output are retained. The corrected test uses emitted integer coordinates, accepts SVG matrix rounding separately, and compares camera-state mathematics at 1e-12. The distinct moving-pointer case verifies the actual state-update repair; see [anchor-verified](anchor-verified/).

Initial stress testing stopped because the fixture reopened in Terrain Generator mode while the test expected Map Builder's illustrator control. The test now selects Map Builder explicitly. Another stress run exceeded 180 seconds at DPR 3 while requests were still progressing. It is preserved in `stress-verified/`; it was not a successful run. The final run passes in [stress-final](stress-final/).

The first settlement fixture selected ocean and correctly returned HTTP 400. The successful run explicitly uses a sampled dry location at x=1.3, y=0.22. The product's submerged-site rejection was retained.

## 6. Real request-to-display pipeline and limits

`Studio.zoom` updates the camera → `visibleViewport` applies SVG letterboxing → `visibleTiles` selects density → `viewportRequest` captures one rectangle/revision → `renderViewport` obtains `pyramidNodes` through `TerrainPyramid.get` and `requestTile` → `continuousTile` validates revision and captures authored operations → `tiles.RenderMeasured` obtains `physicalSamples` → `detail.Sampler.Sample` reads the saved surface/grid and river geometry → raster/PNG generation → client decode, atlas assembly and one resampling → `ViewportSurface` decodes all layers before one commit.

| Limit | Current value and behavior |
|---|---|
| Camera width | 0.00008–3 world units |
| Raster LOD | 0–18, selected from projected pixel demand |
| Node size | 256 or 512 pixels |
| DPR policy | At most 3 |
| Viewport output | 32 megapixels; explicit failure above budget |
| Atlas | 64 megapixels; explicit failure above budget |
| Client encoded cache | 64 MiB LRU |
| Backend physical cache | Eight immutable halo nodes |
| Frontend/backend detail workers | Three each; unchanged |
| Retry/deadline | Three attempts, 250/750 ms backoff, 15 s per attempt; obsolete work aborts |
| Authoritative source | 4096×2048 here; supported widths 1024/2048/4096 |

The LOD formula remains `ceil(log2(max(1, physicalWidth / visibleWorldWidth / nodeSize)))`, clamped to 0–18. Output pixel dimensions depend on viewport/DPR, not geographic zoom. Geographic x=[0,2], y=[0,1]; upper coverage edges are exclusive.

## 7. Sampling, detail and refinement decision

The source's equatorial/N–S spacing is about 9.77 km. At the minimum camera width, a nominal horizontal view spans only about 1.6 km at the equator. `Surface.At` interpolates two saved triangles per source cell. Climate fields use the saved grid; rivers retain separately sampled channel geometry. More raster pixels do not add new landforms.

`TestFinerSamplingWithinSavedTriangleAddsNoRelief` checks 256/512/1024/2048 samples within one triangle. Fixed-position samples are deterministic. The display continues sampling finer river/authored geometry rather than capping all layers at the base relief resolution. The footer explains the base-relief limit.

[Refinement experiment](refinement.json) compares direct continuous residual evaluation with a cached local field derived from the existing procedural field. For 16,641 points, direct construction took 386.6 ms; cached revisits took about 0.33 ms, using 133,128 bytes. Residual RMS was 2.75 m, with 0.00973 m maximum cached interpolation error at tested intermediate points. Repeated absolute samples agreed exactly.

**Decision:** retain this as an isolated experiment. Bounded roughness is not validated geomorphology, watershed refinement or authored-edit dependency tracking. Neither experimental approach was connected to production. New rivers, lakes and biomes were not invented for visual effect.

## 8. Correctness, screenshots and request faults

The before/after extreme rasters have identical source, revision, camera, geographic image bounds, LOD and 892×695 output. All pixels agree exactly: [numeric comparison](matched/pixels.json).

![Before: complete extreme viewport](extreme-before.png)
![After: same viewport](matched/extreme-after.png)

[Fault results](faults/results.json) recover from 503, 429, 408, network disconnection, corrupt PNG bytes and revision conflicts. Transient persistent failures stop after nine observed attempts across three workers; the permanent 409 case made three attempts total across workers, with no per-request retry loop. A valid frame survives failure, and Retry restores matching layer generations.

The final stress run validates all 21 styles, illustration, obsolete request rejection, synchronized layers, DPR 2 and 3 at 1920×1080. Three synthetic global/interior/world-edge atlas comparisons match their monolithic reference exactly. No output blur or source-resolution reduction was introduced.

## 9. Performance and memory

The controlled backend style-order experiment uses the same saved planet, sixteen 256-pixel nodes, natural + hillshade and three repetitions. It avoids HTTP image-cache contamination and compares output hashes:

| Order | Three durations | Median |
|---|---|---:|
| All base nodes, then all shading nodes | 7.273 / 8.418 / 7.020 s | 7.273 s |
| Required styles together per node | 4.401 / 3.753 / 3.761 s | 3.761 s |

All PNG hashes match. This supports the request-order change; it is not a claim that every browser gesture is 48% faster. [Results/profile](sampling-final/results.json), [original profile](sampling-before/profile.txt).

Final DPR-2 global frame: complete coarse 268 ms, final 934 ms, output 3064×1950. Final DPR-3 global frame: coarse 394 ms, final 55.762 s, output 4596×2925. The latter remains expensive. Backend cache state differs from the earlier failed DPR-3 run, so the entire timeout-to-success difference is not attributed to one optimization.

Three separate DPR-3 extreme-view browser runs completed. The first final frame took 42.974 s; subsequent browser-cold/backend-warm runs took 1.969 and 1.939 s. Encoded node storage was 986,472 bytes each. Browser working sets after requested GC were 1638, 1697 and 1457 MiB. These are different browser processes, not proof of a long-session memory plateau. No before/after memory improvement is claimed. Atomic frames, decoded output and browser resources still have substantial cost; [memory records](memory-0.json).

**Architecture decision:** keep the atomic renderer and optimize it further for large high-DPI views. Do not raise concurrency, remove safety budgets or enlarge the source based on these results.

## 10. Climate-field diagnosis in the new saved world

The source already uses climate version 1. Stale version activation is ruled out for this saved terrain. Dry-land shares are forest 7.66%, woodland 2.55%, steppe 10.15%, desert 5.57%, tropical rainforest 6.35%, temperate rainforest 2.88%, tundra 18.81% and ice 12.28%.

About 23.18% of its dry land is at 45–60° absolute latitude, where 62.40% is classified tundra. The annual thermal equation, elevation and growing-season limit explain why changing rainfall labels alone cannot supply broad temperate forests there. This identifies a thermal-model/calibration concern; it is not evidence that every high-latitude tundra area should become forest.

The independent saved-field diagnostic reconstructs the original coarse climate, removes its lapse at each source vertex, and compares temperatures at exactly the same materialized sites. About 6.01% of emerged land has more than 1°C transfer error; maximum 10.63°C in this saved world. Mean spurious cooling is only 0.089°C. Thus the confirmed resampling defect is important locally but does **not** explain the entire cold distribution. [Evidence](temperature-resampling-before.json).

The version-2 correction removes source-vertex atmospheric elevation before interpolation, applies the new positive elevation, persists its atmospheric reference height, and recalculates monthly PET, dry months, warm-dry/cold-rain summaries, aridity and moisture. The same positive-elevation convention also applies to authored edits in sampling, smoothing, tile and mesh paths; version-0/1 edit temperatures retain their old behavior. Rain transport, rainfall thresholds, hydrology algorithms and biome thresholds were not changed. Soil retains the coarse transport model's reservoir approximation.

## 11. Paired biome results

Percentages use cos(latitude)-weighted **dry land**, excluding ocean and lakes. Configuration and seed are identical within each pair. Lake coverage can change through the existing climate/hydrology coupling; [summary.json](summary.json) records denominators, means and compatibility checks. Ordinary forest is not a sum of all forest categories.

| Seed | Forest v1→v2 | Tropical rainforest | Temperate rainforest | Steppe | Desert |
|---|---:|---:|---:|---:|---:|
| Kriemhild | 11.90→11.13 | 13.12→14.53 | 5.05→6.01 | 7.83→7.84 | 1.35→1.39 |
| gallery-001 | 11.11→9.63 | 17.45→20.55 | 6.74→8.23 | 5.67→5.72 | 2.65→2.66 |
| gallery-002 | 6.56→6.05 | 12.01→13.32 | 4.01→4.51 | 9.51→9.39 | 3.97→4.02 |
| 8fef5907901a | 10.83→10.17 | 9.96→11.25 | 3.46→4.18 | 10.39→10.29 | 5.95→6.01 |
| gallery-003 | 9.20→8.54 | 13.03→14.61 | 4.23→5.03 | 8.45→8.38 | 2.99→3.04 |
| gallery-004 | 11.68→11.23 | 6.68→8.13 | 3.63→4.26 | 8.43→8.47 | 1.32→1.35 |
| New saved seed d1feae98… | 7.66→7.34 | 6.35→7.60 | 2.88→3.33 | 10.15→10.20 | 5.57→5.53 |

Version 2 generally lowers corrected PET and increases moisture availability; more rainforest is an explainable result, not evidence that the original complaint has been solved. Version 1 remains the default. Version 2 is opt-in pending broader thermal/atmospheric validation.

Full before/after maps, monthly atlases, fields, latitude distributions and region statistics are in `climate-v1/` and `climate-v2/`. The recovered saved world's additional thermal/moisture/rainfall regime maps and palettes are in `new-world-v1-final/`. The active seasonal classifier's synthetic thermal, Mediterranean, tropical, boreal, alpine and moisture-gradient tests pass. They validate classification of supplied fields; they do not validate the realism of every generated climate field.

## 12. Persistence and editor integration

All six prior version-1 seeds exactly match the previous investigation's climate/biome/latitude/connectivity statistics and all 36 diagnostic image hashes. Version-0 compatibility tests pass. No migration was introduced.

The version-2 UI test selects the option, verifies its submitted request, generates once, accepts, reloads and compares the same terrain/climate sample exactly. No second generation occurs. The HTTP storage test exercises climate-version-2 historical acceptance, Age isolation and cold restore. Monthly fields and reference elevation survive codec round trips.

The final editor sequence passes gradual/rapid zoom, both pan directions, return/reload, Map Builder placement, terrain raise, Undo, Redo and settlement selection. Entity coordinates remain unchanged during navigation. A dry-site settlement was accepted and exported: SVG 2,237,733 bytes and PNG 111,546 bytes. See [integration-verified](integration-verified/).

## 13. Files changed in this continuation

- `camera.js`, `Studio.jsx`: source-resolution diagnostics, current-camera wheel conversion, experimental climate option, opt-in exact diagnostic camera restoration.
- `terrain-pyramid.js`: exact failed-blob eviction, fine-cache shortcut, per-node related-style fetching and bounded stage timings.
- `tiles/render.go`, `tiles/physical.go`, `continuous.go`, `continuous_mesh.go`: canonical wrapped sample addressing, raster cache namespace, consistent authored temperature corrections and separate sampling/edit/raster/PNG server timings.
- `generate/climate_transfer.go`, `generate/surface.go`, `model/climate.go`, `detail/sampler.go`, `generate/climate_regime.go`: version-2 elevation and moisture correction, with older semantics preserved.
- `model/model.go`, `model/codec.go`: explicit supported version 2 and validation of its persisted reference elevation.
- `climate-audit`, `deepzoom-audit`, `refinement-audit`: read-only climate, sampling, profiling and isolated refinement experiments.
- Tests cover corrupt recovery, fine-cache coverage/revisions, worker bounds/style locality, finite source information, maximum-LOD wrap identity, physical shared edges, climate transfer, monthly summaries, persistence and browser/editor behavior.

Previous viewport-investigation changes remain present; the initial [inventory](inventory.json) distinguishes those from this continuation.

## 14. Regression results

94 JavaScript tests pass. Active Go terrain and HTTP suites pass; the final affected tile/model/HTTP rerun passes, followed by targeted maximum-LOD addressing, version-2 cold-storage, authored temperature and generator/climate compatibility tests. `go vet` passes for the active terrain/HTTP and three audit commands. Production build and `git diff --check` pass. Logs are in [regression](regression/).

The final browser stress, corrected anchor, exact raster comparison, editor integration, settlement exports and version-2 UI persistence checks pass. Earlier timeouts, rejected tests and fixture mistakes are preserved and described above. The full legacy repository test suite and non-Chromium browsers were not validated in this continuation.

## 15. Remaining limitations and next decision

The broader biome objective remains open. The annual thermal model is a reduced latitude/elevation/continentality approximation, not an ocean-circulation or calibrated energy-budget model. Its maritime term and seasonal amplitude need independent validation across coastal/interior/high-latitude controls before another physical change. A candidate must pass those controls and the full paired seed suite; changing vegetation shares alone is insufficient.

Physical calibration can be evaluated against the energy/storage/transport separation in [North and Coakley's seasonal energy-balance study](https://faculty.washington.edu/battisti/589paleo2005/Papers/NorthCoakley.pdf). Growing-season refinements should be validated separately from temperature simulation; [Beck et al.'s climate-classification definitions](https://www.nature.com/articles/s41597-023-02549-6) provide a reproducible monthly reference. Neither paper is claimed to validate the current generator.

Real local geomorphic refinement remains experimental. High-DPI latency and native browser memory remain material risks. The three memory runs do not establish unlimited-session stability. Original unsaved previews and their exact seeds cannot be recovered from a history workspace URL alone. The test servers are disposable; version-2 results should not be treated as a migration path for saved user worlds.

## 16. Reproduction

Use PowerShell from the repository root. Go is `.tools/go/bin/go.exe`. Set `KRIEMHILD_BROWSER_EXECUTABLE` to the installed Chromium executable.

```powershell
python tests/deepzoom-inventory.py
.tools/go/bin/go.exe run ./cmd/climate-audit -load .cache/deepzoom-planet -out docs/deepzoom-investigation/recheck-saved
.tools/go/bin/go.exe run ./cmd/deepzoom-audit -out docs/deepzoom-investigation/recheck-sampling
.tools/go/bin/go.exe run ./cmd/refinement-audit
npm.cmd test
.tools/go/bin/go.exe test ./internal/terrain/... ./internal/httpapi -timeout 8m
npm.cmd run build
node tests/deepzoom-faults.mjs
node tests/deepzoom-anchor.mjs
node tests/deepzoom-matched.mjs
node tests/deepzoom-climate-ui.mjs
```

Browser scripts require an isolated server on 8125, and `deepzoom-matched.mjs` requires the untouched comparison snapshot on 8126. The inventory script creates a new consistent backup; start the current server with `-data <that backup directory>`, never the user's `data` directory. Extract `continuous/metadata` and `continuous/fields` for the recorded project into `.cache/deepzoom-planet` for read-only commands. For paired generation use the seven seeds above and `-climate-version 1` or `2` with separate output directories. Script environment overrides for editor tests and memory tests are recorded in the adjacent JSON evidence and test source.
