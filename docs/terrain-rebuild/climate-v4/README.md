# Climate version 4 validation

Version 4 is the default for newly created worlds. Saved versions 0-3 retain
their original generation/classification paths. Earth-like rainfall remains
**1.65**; there are no biome quotas or distribution inputs to generation.

The [revised implementation brief](../CLIMATE_V4_DISTRIBUTION_SPEC.md) includes
the dryland safeguards and the later savanna/steppe requirements. See the
[model description](../CLIMATE.md) for equations and approximation limits.

## Paired evidence

- [Fixed-terrain report](fixed/REPORT.md) and [gallery](fixed/index.html): all
  24 seeds pass, including four held-out seeds. The same relief, rivers, water
  masks, projection, palette and scales isolate climate changes.
- [Full-pipeline report](full/REPORT.md): all 24 seeds pass. This additionally
  includes climate effects on the established fine-relief conditioning and
  hydrology. These effects can change bed/lake samples, so this is a separate
  comparison from fixed terrain.
- [Seed manifest](seeds.json) and [regression limits](regression-limits.json)
  were recorded before candidate distribution review. Limits were not relaxed
  after initial candidates failed.

Equal-world means from the full pipeline:

| Environment | Version 1 | Version 4 |
| --- | ---: | ---: |
| Temperate broadleaf and mixed forest | 11.43% | 23.63% |
| Tropical rainforest | 10.95% | 6.77% |
| Savanna | 4.31% | 0.46% |
| All steppe | 6.08% | 6.30% |
| Desert | 2.44% | 0.69% |

Land precipitation averages 2212.0 -> 2271.7 mm/year, while PET averages
1120.5 -> 973.1 mm/year. This correction does not globally dry the planet.
The lower dry-biome shares are observations, not prescribed minima or targets.
These are broad plausibility/regression results, not a calibrated reconstruction
of Earth's total precipitation or biome proportions.

Version-4 steppe comprises 4.34% temperate, 1.79% subtropical and 0.16% tropical
steppe (rounding explains differences from the total). All are semi-arid;
tropical winter warmth selects tropical steppe, while mild winters and a warm
summer distinguish subtropical from temperate steppe. Tropical steppe can have
more rainfall because PET is greater. Soil fertility is not inferred from the
biome name. Savanna remains separate from both steppe and adequately watered
tropical seasonal forest.

## Checks completed

- The complete `internal/terrain/...` test suite passes with version 4 as the
  default, including deterministic generation, sampling, hydrology, tiles, and
  frozen pre-change field/surface hashes and vegetation shares for versions 0-3.
- Climate-specific HTTP/API tests pass, including explicit version 0, old
  generation requests, project archives, historical Age isolation, and cold
  restart. A separate archive check confirms steppe subtype identity survives
  the legacy compatibility projection and archive restore.
- Independent persisted-month checks confirm rainforest thermal/moisture
  invariants; fixtures cover seasonal tropical forest, savanna, all three
  steppes, temperate forest/woodland/grassland, Mediterranean seasonality,
  boreal/tundra/ice transitions, solar evaporation, circulation, rain shadows,
  and the absence of biome feedback into weather.
- The JavaScript suite passed 147 tests before default promotion. The final
  default/version/terrain checks then passed all nine targeted tests, including
  the added new-default assertion. The isolated production frontend build
  passed; the existing running application's build/data were not replaced.
- The broader `go test -p 2 ./...` run passed the other packages but timed out
  after ten minutes in the unchanged legacy terrain package. The test active
  at timeout, `TestArchipelagoRecordsRoundTripAndRejectInvalidOrigins`, passed
  separately in 8.4 seconds. The complete legacy package was not rerun.

The T3 browser opened the gallery but its automation snapshot/evaluation calls
failed. Gallery PNGs were inspected directly; an automated browser acceptance
run is not claimed. Restart the application normally to load the new code.
