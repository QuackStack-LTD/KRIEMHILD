# KRIEMHILD terrain climate and biome distribution revision

Status: implemented as climate version 4 and promoted to the new-world default after the 24-seed fixed-terrain and full-pipeline comparisons passed, including four held-out seeds. See [model and validation notes](CLIMATE.md).

## Purpose and controlling principle

Correct excessive tropical rainforest and overly uniform lush vegetation by improving monthly thermal regimes, rainfall seasonality, and natural vegetation classification. Restore the missing temperate and continental environments while preserving plausible existing dry regions.

**Reduce rainforest overrepresentation through seasonal climate classification and temperate climate regimes, without globally drying the world.**

This requirement supersedes any earlier recommendation to automatically lower the Earth-like rainfall multiplier toward 1.0, strengthen subtropical drying everywhere, or increase rain-shadow suppression indiscriminately. Excess rainforest can coexist with sufficient or excessive desert and savanna. Annual rainfall, its seasonal distribution, thermal classification, and vegetation classification must be diagnosed separately.

No biome quota, target area, post-generation recoloring, or feedback from vegetation into climate is permitted. Distribution measurements are validation evidence only.

## 1. Establish the baseline before changing moisture

The current backend `model.DefaultSpec()` and frontend Earth-like defaults use climate version 1 and rainfall 1.65. Versions 2 and 3 are explicit experimental options. Version 3 includes the seasonal thermal calculation and sea-surface correction for atmospheric uplift. These observations describe the current implementation; they do not establish that its planetary precipitation is excessive.

Begin version-4 experiments at the existing Earth-like rainfall setting. Do not reduce the rainfall multiplier or its equivalent internal moisture-capacity scale simply to obtain less green on the map.

Measure, for each fixed seed and climate version:

- Total annual precipitation volume over land and the whole planet, and area-weighted mean precipitation in mm/year.
- Monthly and annual precipitation by latitude, coast distance, elevation, and windward/leeward exposure.
- Annual PET, annual precipitation/PET, monthly water deficits, and dry-month counts.
- Coldest and warmest month, annual temperature range, growing-season length, and winter severity.
- Independent thermal, moisture, and rainfall-seasonality regimes, followed by vegetation distribution.
- Ocean evaporation, land evaporation, atmospheric storage change, and conservation residuals.

Distinguish physical land precipitation from the dry-land denominator used for vegetation shares. Report inland water area separately. Use spherical area weighting rather than raw pixel counts.

Only change the global rainfall baseline if measured precipitation and water-budget evidence demonstrate a global supply error. Document the evidence, compare old and candidate settings on the same seeds, and pass the dry-biome safeguards below. A neutral-looking parameter value is not itself a calibration criterion. Keep rainfall available as a user-controlled world parameter and preserve the meaning of saved values for older versions.

## 2. Introduce a separately versioned climate revision

Implement climate version 4 as an explicit option during development. Promote it to the default for newly created worlds only after the fixed-seed diagnostics, compatibility tests, and gallery pass acceptance.

Preserve the compatibility contract:

```text
saved world -> saved climate version -> original climate and vegetation behavior
new world with no explicit version -> validated default climate version 4
explicit regeneration of an old specification -> that specification's version
explicit user upgrade -> version 4 in the resulting new revision
```

Versions 0, 1, 2, and 3 must retain their generation, monthly fields, sampling, and vegetation behavior. Do not reinterpret an omitted version in historical data as the new default. Loading, importing, exporting, reopening, or visiting a historical Age must not regenerate or migrate climate.

Audit all version gates, frontend defaults, preset defaults, request decoding, cache identities, tile sampling, and persistence. In particular, `startTerrainJob` currently clamps requested climate versions upward to `model.ClimateVersion`; changing that constant alone would silently upgrade explicit old-version requests. Default only an omitted version on a new-world request, preserving explicit version 0 as well as other supported versions. Keep historical decoding separate from new-world defaulting.

## 3. Keep climate independent of vegetation

The dependency direction is:

```text
terrain + latitude + elevation + coast distance
                    |
                    v
           monthly temperature
                    |
                    v
 atmospheric circulation and moisture transport
                    |
                    v
    monthly precipitation and water balance
                    |
                    v
             climate regime
                    |
                    v
          natural vegetation biome
```

Vegetation labels and desired biome distributions must never influence temperature, atmospheric circulation, evaporation supply, or precipitation. Physical land-water masks and moisture stores remain climate inputs; assigned vegetation categories do not.

## 4. Correct thermal regimes and continentality

Use the modern monthly seasonal calculation as the basis for version 4, with version-specific calibration:

```text
monthly temperature = latitude-based annual temperature
                    + seasonal solar response
                    + continentality response
                    + smooth regional thermal variation
                    - elevation lapse
```

The seasonal solar and continental responses may be coupled through thermal inertia; do not add a second seasonal forcing that double-counts the existing model.

Calibrate the sea-level annual temperature curve and maritime influence against recognizable equatorial, subtropical, temperate, boreal, and polar conditions. Latitude controls energy input, not a forced biome label. A location with a cool winter must not remain tropical because its annual mean is warm. Check both excessive warmth and excessive cold: the existing investigation also records substantial mid-latitude tundra in a saved version-1 world.

At comparable latitude and elevation, continental interiors should have larger annual temperature ranges, colder winters, and warmer summers than maritime locations. Cold continental interiors need not become dry to cease being tropical. Derive boreal suitability from winter severity and growing-season length rather than annual mean temperature alone.

Add subtle, deterministic, low-frequency regional thermal variation using continuous spatial fields. It may approximate regional maritime and atmospheric influences but must not become per-pixel noise, introduce longitude seams, or prescribe horizontal biome stripes. Apply elevation consistently at grid and detailed sample resolution without double-counting lapse corrections.

## 5. Preserve circulation while correcting moisture distribution

Retain moving equatorial convergence, subtropical subsidence, mid-latitude storms, windward uplift, and leeward moisture depletion. Measure their current strength before adjusting them.

- Concentrate persistent tropical wetness near the moving convergence region. Its seasonal migration should allow wet and dry seasons outside the persistently humid core.
- Preserve subtropical dry regions without automatically strengthening their dryness or widening their belts.
- Maintain adequate mid-latitude moisture for broadleaf and mixed forests. Ordinary storm-track precipitation must not imply perhumid rainforest.
- Preserve wet windward slopes and drier lee regions. Avoid charging the same mountain ascent repeatedly or treating the seabed as atmospheric relief.
- Support continental moisture penetration and finite soil-water recycling through the transport model. Do not compensate for excess rainforest with a blanket inland drying factor.

Prefer corrections to seasonal timing, thermal response, and classification before changing total moisture supply. Any circulation or transport correction must conserve atmospheric water and be evaluated separately from classification changes. Redistributing moisture is permitted when physically supported; removing water to obtain a preferred biome share is not.

## 6. Derive climate regimes from monthly fields

Classify independent thermal, annual water-balance, and precipitation-seasonality axes before selecting vegetation. Systematically derive and expose:

- Annual precipitation and annual PET, in mm/year, and their dimensionless ratio.
- Driest and wettest month precipitation, in mm/month.
- Summer and winter precipitation totals, with hemisphere-aware season definitions.
- Dry-month count, longest consecutive dry season, and monthly precipitation/PET deficits.
- Coldest and warmest month temperature, annual temperature range, months suitable for growth, and months below freezing.
- Continentality or maritime influence used by the thermal model.

Document the dry-month definition, PET calculation, seasonal windows, threshold inclusivity, and handling of zero/very small PET. Consecutive dry months must wrap across the December/January boundary. Avoid turning frozen, low-PET conditions into apparent warm-climate forest suitability.

Summaries are derived caches of persisted monthly fields. Cached and uncached classification must agree. Interpolated detail samples must satisfy the same climate requirements as grid cells; interpolation of a categorical label is not a substitute for classification.

## 7. Restrict tropical rainforest without converting it all to savanna

Require all of the following for tropical rainforest:

1. A tropical thermal regime, with coldest month at least approximately 18 degrees C.
2. Persistent monthly moisture, with a meaningful driest-month rainfall threshold and no significant dry season.
3. Sufficient annual water availability relative to PET, as well as adequate absolute annual rain.

The existing classifier already checks coldest month, minimum monthly rain, annual water balance, and dry months. The revision must investigate why generated fields or those criteria still produce the reported appearance; adding an already-present threshold is not a complete fix.

Use approximately 60 mm/month as an initial tropical driest-month criterion for comparison, not as proof of adequate monthly water balance. Calibrate the complete predicate using monthly PET and dry-season severity, and use that same predicate in tests and diagnostics. Do not use an absolute latitude cutoff to manufacture a rainforest area.

The intended distinctions are:

| Monthly climate | Natural vegetation |
| --- | --- |
| Tropical, wet throughout the year, ample water balance | Tropical rainforest |
| Tropical, abundant wet-season rain, meaningful seasonal drought | Tropical seasonal forest or woodland |
| Tropical, sustained growing-season water limitation and long dry season | Savanna |
| Non-tropical, adequate moisture and suitable growing season | Temperate or boreal vegetation |

High annual rain with a seasonal dry period must have a seasonal forest outcome available. Failure of the rainforest predicate alone must never imply savanna, steppe, or desert. Adequately moist former rainforest cells with non-tropical winters should become suitable temperate vegetation rather than dry biomes.

## 8. Make the missing temperate environments explicit

Distinguish at least these internal vegetation outcomes:

| Outcome | Required climate basis |
| --- | --- |
| Temperate broadleaf and mixed forest | Non-tropical winters, adequate growing season and moisture, no severe growth-limiting dry season |
| Temperate woodland | Temperate thermal regime with moderate moisture limitation permitting open tree cover |
| Temperate grassland | Temperate growing season with insufficient sustained water availability for closed forest |
| Steppe | Semi-arid water balance; distinguish from more humid grassland |
| Temperate steppe | Semi-arid mid-latitude climate with cold winters and marked summer/winter contrast |
| Subtropical steppe | Semi-arid climate with mild winters; may have a short winter wet season and dry Mediterranean-like summers |
| Tropical steppe | Semi-arid tropical transition to desert; higher evaporative demand permits greater absolute rainfall than temperate steppe |
| Mediterranean forest, woodland, or scrub | Temperate warmth, substantially drier summers, and wetter winters |
| Temperate rainforest | Very high rainfall, persistent moisture, suitable temperate temperatures, and strong maritime influence |
| Boreal forest / taiga | Long cold winters, sufficiently long and warm growing season, and adequate moisture |
| Tundra | Growing season too short or cool for forest |
| Ice / polar | Thermal conditions unsuitable for sustained vegetation |
| Desert | Genuine severe water limitation after thermal and water-balance assessment |

Allow humid subtropical conditions to support appropriate non-tropical forest. Do not force them into either tropical rainforest or dry vegetation. Moist continental locations with severe winters and warm summers must not miss the boreal/temperate transition solely because a single warmest-month threshold is too restrictive.

Temperate rainforest must remain geographically uncommon as an observed result of its demanding moisture and maritime requirements. Its area must not be capped by a quota. Mediterranean classification must depend on seasonal summer/winter contrast in either hemisphere, and must retain water-balance distinctions among forest, woodland, and scrub.

Preserve persisted biome identifiers for older versions. Existing `forest`, `woodland`, `grassland`, `tropical-woodland`, and `shrubland` identifiers may map to explicit version-4 outcomes if their meaning is unambiguous and reported. If new identifiers are needed, extend persistence, palettes, inspection, exports, and consumers consistently. Do not make climates appear more diverse solely by changing colors or names. Wetlands still require hydrological support rather than humidity alone.

Version 4 must use explicit `temperate-steppe`, `subtropical-steppe`, and `tropical-steppe` identities. Preserve historical `steppe` behavior in versions 0-3 and aggregate all steppe identities in dryland regression checks. A typical 250-500 mm/year temperate steppe is a diagnostic reference, not a universal rainfall cutoff. Do not infer soil fertility from a steppe label. Semi-arid Mediterranean-like climates may be subtropical steppe; sufficiently moist Mediterranean climates retain woodland/scrub.

Audit possible savanna overrepresentation independently of rainforest. Savanna requires a tropical thermal regime and sustained water limitation; high annual rain with a seasonal drought must retain a seasonal forest outcome where water availability supports it. Report savanna separately from tropical steppe and seasonal woodland, including old/new transitions and dry-month severity. A lower savanna share is acceptable when supported by corrected climate; approximate stability is an upper expansion safeguard, not a required minimum.

## 9. Expand diagnostics without controlling generation

For each geographic latitude band, report land area, mean annual temperature, mean annual precipitation, PET, aridity, dry months, biome distribution, and fractions classified as tropical, perhumid, temperate, and boreal. Include finer latitude bins and separate hemispheres where useful.

Keep geographic bands distinct from computed thermal regimes: mid-latitude land is not automatically thermally temperate. Perhumid is a moisture property and may overlap a thermal class; report its denominator explicitly rather than summing these fractions as exclusive categories.

Report both mean cell aridity and the ratio of area-integrated precipitation to PET; these are different measurements. Include coast-distance, elevation, and windward/leeward breakdowns, plus old/new biome transition matrices. Separate tropical rainforest from temperate rainforest and tropical seasonal forest from savanna.

Warnings may identify climate collapse, unexpected drying, or failed thermal ordering. They must not trigger reseeding, parameter retuning, moisture injection, biome reassignment, or rejection sampling during normal world generation.

## 10. Protect existing dry regions in paired validation

Use the same seed, terrain settings, masks, resolution, and rainfall setting when comparing versions. First compare climate on fixed terrain to isolate climate effects; then run the complete pipeline to detect hydrology or relief interactions. Record any downstream terrain changes rather than treating different terrain as a controlled climate comparison.

Version 1 is the primary comparison for current default behavior. Version 3 provides a secondary comparison to distinguish adopting its seasonal/orographic improvements from the new version-4 changes. Include diagnostic experiments that hold monthly precipitation fixed while changing thermal or vegetation classification, so any additional dryland can be attributed to precipitation, PET, seasonality, or labels.

Validation expectations are:

| Measurement | Expected evidence |
| --- | --- |
| Incorrect tropical rainforest | Decreases where seasonal drought or non-tropical winters invalidate it |
| Tropical seasonal forest | Receives suitable seasonal tropical climates without automatic savanna conversion |
| Temperate broadleaf and mixed forest | Expands where winter temperatures, growing season, and moisture support it |
| Temperate woodland and grassland | Occur in suitable moderately dry and continental settings |
| Desert area | Approximately stable; no systematic or substantial unnecessary expansion |
| Savanna area | Approximately stable; no systematic or substantial unnecessary expansion |
| Combined desert and savanna | Does not hide expansion of either category through offsetting changes |

Also track steppe, total water-limited land, monthly deficits, and regional aridity, so expansion of dry conditions cannot be hidden by renaming vegetation. Inspect existing dry regions and newly dried cells separately from global net shares.

Before tuning version 4, establish and record numerical regression tolerances for desert, savanna, combined dry biomes, and regional water balance using baseline resolution sensitivity and paired diagnostics. Apply them to the individual-seed outliers as well as ensemble results. Do not choose tolerances after seeing a candidate's failure or accept an increase solely because an ensemble mean conceals it. These are fixed validation limits, not values read by the generator.

An individual cell or seed need not retain identical vegetation across revisions. A physically justified local correction is acceptable within the documented regression limits, but systematic desert/savanna expansion fails this revision's purpose. If moisture changes cannot satisfy those limits, defer those changes and retain the independently validated thermal/classification correction.

## 11. Tests and gallery required before default promotion

Freeze a representative seed manifest before tuning. Include `Kriemhild`, established `gallery-*` seeds, the saved-world regression seed `d1feae98-6510-4a72-951b-44e93a01ae86`, and layouts with large continental interiors, maritime mid-latitudes, northern land, and major mountain barriers. Use at least twenty Earth-like seeds and keep additional seeds out of tuning for final validation. Label non-Earth-like presets as stress cases rather than pooling their shares into the Earth-like acceptance results.

Use deterministic monthly-climate fixtures for classification boundaries and idealized terrain for causal physical relationships, together with the generated ensemble:

1. **Rainforest eligibility:** every tropical rainforest grid cell and tested detail sample satisfies the full tropical, perennial-moisture, and water-availability predicate. Test climates with equal annual rain but different monthly distributions; suitable seasonal examples become seasonal forest rather than rainforest or automatically savanna.
2. **Temperate diversity:** worlds with adequate mid-latitude land and suitable monthly climates retain broadleaf/mixed forest, woodland, and open vegetation as appropriate. Reject collapse of those climates into tropical categories or one undifferentiated forest outcome. Do not require every small island world to contain every biome.
3. **Thermal ordering:** comparable low-elevation tropical, temperate, and polar samples have decreasing mean temperature across the ensemble. Report absent bands rather than assigning them zero temperature; stratify elevation and continentality to avoid misleading comparisons.
4. **Continental response:** matched latitude/elevation samples show larger seasonal temperature ranges inland than at maritime coasts.
5. **Boreal transition:** adequately moist comparable-elevation transects and fixtures support temperate forest, taiga, tundra, and ice as winter severity and growing-season limitation increase.
6. **Temperate rainforest:** every occurrence satisfies persistent moisture and maritime requirements. Report geographic extent and fail widespread ordinary-interior classification without enforcing a generation quota.
7. **Mediterranean seasonality:** summer drought and winter rain select the appropriate regime in both hemispheres; evenly wet climates do not qualify merely because annual rainfall matches.
8. **Continental and mountain contrasts:** matched windward/coastal and lee/interior samples exhibit physically coherent precipitation differences without a systematic expansion of dry biomes. Use idealized barriers to establish causality rather than relying solely on uncontrolled global averages.
9. **Dryland protection:** paired seeds satisfy the preregistered desert, savanna, steppe, water-deficit, and regional regression checks. Report gross biome transitions as well as net area changes.
10. **Numerical integrity:** monthly fields are finite and nonnegative where appropriate, annual summaries agree with monthly values, and atmospheric mass conservation retains the established tolerance.
11. **Historical compatibility:** frozen versions 0-3 regenerate identical physical fields and vegetation; save/export/import/reopen and historical Age workflows preserve explicit versions and monthly fields. An omitted new-world version and explicit old version must take different defaulting paths.

Produce paired maps with identical projection, scale, palette semantics, and shading. Include vegetation, thermal regime, annual and seasonal precipitation, coldest/warmest temperature, PET/aridity, dry months, coast distance, and mountain exposure. Supply machine-readable per-seed results and an aggregate report, including failures and outliers. Visual approval of one attractive seed is insufficient.

## 12. Implementation areas

| Area | Required work |
| --- | --- |
| `internal/terrain/model/model.go` | Support version 4; preserve saved-version semantics; promote the new default only after validation; retain rainfall baseline unless evidence supports changing it |
| `internal/terrain/generate/climate.go` | Version-specific monthly thermal calibration and measured circulation/transport corrections; retain conservation and historical branches |
| `internal/terrain/generate/thermal_seasons.go` | Adopt and calibrate the modern seasonal response, maritime inertia, and continental range for version 4 |
| `internal/terrain/generate/climate_regime.go` | Explicit thermal, water-balance, seasonal, growing-season, and maritime diagnostics |
| `internal/terrain/generate/biome.go` | Version-specific vegetation outcomes derived only from climate and physical surface conditions; consistent monthly summaries |
| `internal/terrain/generate/measure.go` and model diagnostics | Area-weighted latitude/regime diagnostics and transparent denominators |
| `internal/terrain/model/climate.go`, sampling and tiles | Preserve version identity, monthly fields, detailed lapse handling, biome identities, and cache isolation |
| `internal/httpapi/continuous.go` | Distinguish omitted new-world versions from explicit historical versions; remove silent version upgrades |
| `src/continuous/api.js` and `src/continuous/Studio.jsx` | Align defaults/presets, expose version 4 during development, preserve displayed saved-world version, and handle explicit version 0 correctly |
| `cmd/climate-audit` and terrain tests | Extend existing audit tooling with paired comparisons, frozen manifests, monthly diagnostics, regression limits, and gallery artifacts |

Reuse the existing climate audit and compatibility fixtures. The earlier [climate investigation](../preview-climate-investigation/REPORT.md) and [experiments](../preview-climate-investigation/EXPERIMENTS.md) provide baseline evidence, not automatic approval for version 4 or a reason to overwrite historical reports.

## Final acceptance and delivery

The implementation is complete when the fixed-seed and held-out evidence demonstrates concentrated genuinely perhumid tropical rainforest; distinct tropical seasonal forest; substantial suitable temperate forest, woodland, and open vegetation; continental and boreal transitions; seasonal Mediterranean environments; and coherent mountain climate contrasts. Desert and savanna must remain approximately stable under the recorded regression checks, with no hidden increase in overall water limitation.

Deliver the versioned implementation, compatibility and plausibility tests, seed manifest, paired diagnostic report, and gallery. Document the chosen rainfall setting and why it was retained or changed. Promote version 4 to the new-world default only after these checks pass. Existing worlds must remain reproducible throughout.
