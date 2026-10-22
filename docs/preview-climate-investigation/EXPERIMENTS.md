# Experiments and rejected changes

The starting point is branch `rebuild/continuous-terrain`, HEAD
`2e4f5e4ede02b69e4cdba9c525212c6d7d827820`, with the preceding investigations'
uncommitted changes retained. This directory records the subsequent work; older
evidence under `docs/deepzoom-investigation` remains historical.

## Climate experiments, in execution order

All complete worlds use continuous-3, a 4096 × 2048 authoritative surface,
Earth-like defaults, rainfall 1.65, relief 0.8 and the named seed. Climate 0/1/2
equations are retained. Climate 3 is an explicit experimental option, not the
default and not a migration of saved worlds.

1. `saved-v1`: read-only audit of the saved `d1feae98-6510-4a72-951b-44e93a01ae86`
   planet. No generation or database writes. Its source is
   `.cache/deepzoom-planet/{metadata,fields}` from the prior SQLite backup.
2. `thermal-baseline.json`: the old monthly equation independently of vegetation.
   At latitude 55°, height 100 m, coastal warmest month is 7.266°C and inland
   warmest month is 8.760°C. Both fail the classifier's 10°C growing-season limit.
3. `thermal-candidate.json` and `candidate-v3-seasonal-only`: replace only the
   seasonal anomaly with a periodic response to daily-mean solar forcing and
   land/ocean heat capacity. Retain the annual mean and lapse rate. No vegetation
   thresholds change. This fixes a demonstrated lack of summer warmth, not the
   entire annual thermal model.
4. `orography-before.txt`: a controlled ocean-depth substitution changes maximum
   annual rainfall by 20,153.260 mm. The windward difference incorrectly includes
   submerged terrain. `orography-after.txt` records exactly zero difference after
   restricting atmospheric uplift/lee relief to positive height above sea level.
   `candidate-v3-orography` is the next independent full-world run.
5. `evaporation-before.txt` measures free-water evaporation at 2.806× the model's
   temperature-based reference PET in the controlled coast fixture. A trial adds
   `evap = math.Min(evap, es[i]/120)` in the ocean branch for climate 3.
   `evaporation-after.txt` records the cap's intended numerical effect.
   `candidate-v3-evaporation` records its full-world consequence: ordinary forest
   falls to 3.44%, steppe rises to 28.1%, desert rises to 21.8% of dry land.
   **The cap is rejected and removed.** Reference vegetation PET is not itself a
   calibrated ocean latent-energy budget; a universal cap is not justified by
   the ratio alone. The rejected test log is an experiment, not the final suite.

The final experimental candidate combines steps 3 and 4, retains the original
evaporation/transport/PET formulae, and records separate monthly water-budget
terms. These terms expose the unresolved supply/demand calibration rather than
concealing it with biome thresholds. No global biome quota or smoothing is used.

## Scope of the seasonal candidate

The anomaly uses published land/ocean response parameters from
[McKinnon & Huybers (2014), Table 1](https://agupubs.onlinelibrary.wiley.com/doi/full/10.1002/2014GL060404):
2 and 76 metres water-equivalent heat capacity, 2.48 W/m²/K damping, and a 0.52
land-response scale. This implementation is **not** that paper's calibrated model.
It substitutes coast distance for atmospheric back trajectories, fixes coalbedo
at 0.7, uses an uncalibrated near-coast land response and 50-km coastal transition,
and does not simulate seasonal meridional heat convergence, ocean currents,
clouds, snow/ice feedback or lake heat storage. Its annual mean remains the prior
latitude/maritime approximation. These limitations prevent recommendation as a
fully validated Earth-climate replacement.

## Preview measurement controls

`preview-headless/dpr-*.json` uses production `viewportRequest`, `previewRequest`,
`TerrainPyramid`, three bounded workers, real PNG responses and browser decoding.
Each output identifies its exact camera, source session/revision, styles, actual
browser DPR, raster size/LOD, output pixels, stage timings, bytes and response
cache headers. Each cold browser cache has an immediately paired warm cache.
Backend cache state must be read from response headers, not inferred from browser
navigation. These timings exclude Studio's 80-ms debounce and React scheduling.

The initial shared-browser run completed 120 cases, but the automation host
disconnected before all results could be recovered. Only `preview-matrix-0.json`
was recovered. It is not the basis for the complete timing matrix. The browser
tool explicitly reported that no host was available and directed a headless
fallback. `preview-headless` reruns use actual DPR 1/2/3 contexts.

`preview-process-restart` is the superseded cold-cache experiment: restarting the
backend preserves its disk tile cache. Its response headers exposed the mistaken
assumption. `preview-cold` rotates the **disposable** persistent cache before each
backend process; archived caches are retained under `.cache/preview-cold-data`.
No user cache or saved world is deleted to create a cold benchmark.

## Retained preview refinements

The initial preset-only cold matrix revealed that global Balanced selected LOD 1
and skipped the coarse transaction (`z > 1`), taking about 2.2 s to show its first
frame. The condition is now `z > 0`; final measurements record a complete LOD-0
frame at about 0.62 s and fine completion at about 2.8 s. Fine-cache revisits
still skip coarse work. This is a separately tested scheduling correction.

The preset-only memory probes also showed that low-density PNGs did not remove
SVG's native-DPR hillshade blend surfaces. Both UI builds were then routed to
the same pristine backend, with an identical initial camera/style before any
request. JS heap was about 31 MiB; the dominant retained allocation was in the GPU
process. Removing hillshade substantially lowered it. The retained preview-only
compositor combines base and shade at target raster density, closes both source
bitmaps, and commits one PNG. Transparent-edge tests against the SVG operation
have at most one channel-value error. Empty raw-layer declarations let exports
rebuild natural and hillshade independently; the Illustrator bypasses composition.

The export-isolation test independently exposed nondeterministic worker insertion
order. Before sorting, SVGs became identical after geographic image sorting,
while exported PNG hashes differed. After sorting y/x, all presets have identical
SVG and PNG hashes. `preview-export-race-before` retains the failing captures.

Final data are in `preview-final`, `preview-cold-final`, `preview-cold-natural`,
`preview-memory-final`, `preview-integration` and `matched-previews`. Earlier
memory datasets include an exited-PID collection failure, differing initial
saved views, and a cross-port Origin mismatch in the diagnostic proxy; they are
preserved but excluded from final comparisons. Peak calculations include the
post-GC sample as well as pre-readiness samples. They remain sampled observations,
not guarantees of the instantaneous maximum.

## Frozen-relief climate control

The full paired seeds revealed identical coasts/roughness but different fine beds.
Existing `landmass.Conditioned` uses climate moisture for gullies/dunes, so this
is an existing generator dependency. The opt-in `TestFixedSurfaceClimateAudit`
samples the same saved relief into both weather runs, then replaces only climate
fields in disposable decoded copies. It does not rerun terrain conditioning or
hydrology. Bed/coasts/roughness/lake levels, rivers, and drainage/water masks match
exactly. `fixed-surface-v2` and `fixed-surface-v3` contain the maps, field hashes and
proofs. The dry-land denominator is identical, and ordinary forest still increases.

The first full UI acceptance attempt timed out waiting for navigation and did not
produce an accepted terrain link. It is not counted as a passing test or assigned
a speculative product cause. The retest explicitly observed the acceptance HTTP
response, then verified the stored climate version and exact reopened sample.
The retained final proof additionally closes/restarts the backend and verifies
zero generation jobs on reopening.
