# Terrain Resources implementation and handover

Date: 2026-11-01. Branch `rebuild/continuous-terrain`; starting HEAD
`2ff34a5d00cb90cd9cbd5dabeb9d46c881fa6dfd` (Brief 5 documentation), following
`6a805d0`, `dfcad40`, and `eb40802`. The checkout already contained uncommitted
Languages Brief 6 changes and fixtures. Those files were preserved. No repository
AGENTS.md was found. Environment: Windows 10 Home 10.0.19045 amd64, PowerShell,
Go 1.27.1 (`.tools/go/bin/go.exe`), Node 24.21.0, npm 11.19.0. Go checks used
`CGO_ENABLED=0`.

## Findings and delivery

The retained geological eligibility, connected-resource regions, provenance,
placer transport, query and persistence code was present, but the continuous
adapter did not populate it. Biosphere generation and accepted ranges were
present and functional; the native geological inputs and water classification
needed integration. See [the implementation document](TERRAIN_RESOURCES.md) for
the physical dependency chain, causal predicates, units, coordinates and API.

Delivered: builder circle drawing/panning/radius controls and right-side reports;
settlement-only profile terrain circles and fit-to-50-km controls; late physical
resource generation with systems/districts/endowment/provenance; preserved native
drainage projections; groundwater/spring integration; actual accepted Biosphere
range intersections and freshness; strict native source/hierarchy validation;
archive resource chunking with explicit schema 7; documentation and regression
tests. Surface imagery applies saved authored operations while labeling dependent
base analyses stale.

Integration checks caught and resolved missing hydrology projection fields,
incorrect marine classification of below-sea-level inland water, invalid negative
spring temperature properties, thermal classification without elevated heat, and
oversized resource archive files. The archive fix preserves its per-file bound
through versioned chunks. A separate native coordinate convention fixes radius
alignment and half-footprint seam/polar area accounting without reinterpreting
legacy cell-centre geography.

## Commands and results

Commands ran from the repository root. Logs are disposable local evidence in
`.cache/terrain-resources/`; the test sources are the permanent reproducible
evidence. Commands below use `.tools/go/bin/go.exe` as `go`.

| Command | Result |
| --- | --- |
| `go test -count=1 ./internal/terrain/model ./internal/terrain/tectonics ./internal/biosphere ./internal/settlements` | Baseline passed: 0.920 / 1.262 / 3.282 / 39.368 s. |
| `go test ./internal/terrain/resources ./internal/legacyterrain ./internal/httpapi -run '^TestNaturalResourcePrerequisitesAndValidation$'` | Initial retained eligibility baseline passed; new package initially had no tests. |
| `go test -count=1 -timeout 12m -p 2 ./internal/biosphere ./internal/history ./internal/storage ./internal/terrain/... ./internal/settlements` | Passed all selected packages; continuous generation 181.508 s, settlement tests 47.122 s. Resource-specific tests were added and run separately. |
| `go test -count=1 -timeout 10m ./internal/httpapi ./internal/legacyterrain ./internal/terrain/resources -run 'Test(Continuous|Natural|Biosphere|Resource|Renewed|SettlementResource|SourceTracks|WorldProject)' -v` | Passed: HTTP API 194.726 s, retained natural/surface checks 41.829 s, resource dependency/cancellation 0.662 s. Includes native archive round-trip, two climate-version cold restarts, old project formats, Biosphere ZIP/inheritance, resource API authorization/read-only behavior, and causal negative cases. |
| `go test -count=1 ./internal/httpapi ./internal/legacyterrain -run 'Test(ResourceRenewalMultiSeed|ContinuousArchiveRoundTrip|RenewedResourceCausalRequirements)' -v` | Passed after final provenance changes: 62.980 / 0.740 s. Three physical seeds replay full geology/hydrology/resources byte-identically; physical planet hashes are unchanged by resource generation. |
| `go test -count=1 ./internal/biosphere` | Passed, including dedicated nonstandard ocean-ID/inland-lake classification regression. |
| `go vet ./internal/terrain/... ./internal/legacyterrain ./internal/biosphere ./internal/httpapi` and final `go vet ./internal/httpapi ./internal/legacyterrain ./internal/terrain/resources ./internal/biosphere` | Passed. |
| `go build ./...` | Passed with CGO disabled. |
| `npm.cmd test` | Passed, 145 tests, zero skipped; final run 51.343 s. Includes two new spherical radius/seam tests. |
| `npm.cmd run build` | Passed initially; subsequent preview builds isolated below. |
| `node node_modules/vite/bin/vite.js build --outDir .cache/terrain-resources/dist-final` | Passed, 11.62 s. |
| `node node_modules/prettier/bin/prettier.cjs --check src/continuous/Studio.jsx src/continuous/TerrainResourceReport.jsx src/continuous/terrain-resources.js src/continuous/terrain-resources.css src/worldbuilding/Profile.jsx src/worldbuilding/SettlementResources.jsx` | Passed for changed UI files. |
| `npm.cmd run check:ui` | Failed: Prettier reports 32 existing files. These warning paths have no diff against HEAD. Includes unchanged Biosphere/UI/continuous files; no bulk reformat applied. |
| `node scripts/check-ui.mjs` | Passed independently: UI boundaries, tabs, dialogs and compatibility. |
| `git diff --check` | Passed; Windows line-ending notices are not whitespace failures. |

Final report-specific checks also passed:
`go test -count=1 ./internal/httpapi -run 'Test(ResourceRoutesReadOnlyAuthorizationAndSavedPreview|SettlementResourceFit)'`
(31.705 s), followed by affected-package vet and build. These checks verify
native potential evaporation against its source field, omit unsupported legacy
placeholders, check complete biome sample proportions, and fit all saved map
representations of a settlement on the selected terrain. The final endowment
check additionally prevents solar, timber and freshwater potential from being
misrepresented as tonnes of host rock.

After the final unit and water refinements,
`go test -count=1 ./internal/httpapi -run 'Test(ResourceRenewalMultiSeed|ContinuousArchiveRoundTrip)'`
passed in 64.611 s. The final resource API/settlement geometry selection passed in
29.133 s, including clipped river-flow interpolation: segment endpoints outside
the selected circle cannot inflate its local flow estimate. Affected-package vet
and `go build ./...` passed again. Final UI build: 9.75 s; changed-file formatting
and the independent UI-structure checks passed.

## Test scope and measured inventory

`TestResourceRenewalMultiSeed` uses actual completed native planets with 1,024-wide
surfaces and seeds `resource-causal-0`, `resource-causal-1`, `resource-causal-2`.
Final inventories have respectively 12,629 / 16,948 / 16,774 occurrences and
9,010 / 12,182 / 12,028 systems. Shared multi-commodity systems number
1,449 / 1,768 / 1,814. Common construction-material cell coverage exceeds rare
strategic-resource coverage on all three worlds. The second seed contains two
diamond occurrences and one rare-earth occurrence; absence on another seed is
valid. Full causal validation runs for every occurrence. Quantities vary and
carry independent extent/grade/quantity units.

These are deterministic verification workloads, not production-scale throughput
or peak-memory profiling. No measured reserve accuracy, calibrated global rarity,
maximum sustainable water extraction or large-world performance claim is made.

Dedicated causal tests remove ancient peat, burial, traps, migration, maturation,
preservation and evaporative history to assert rejection. They test cratonic
diamond transport, carbonatite REEs, supplied geothermal circulation and distinct
aquifer conditions. Existing tests additionally verify actual upstream placer
sources. Geometry tests cover irregular, point, high-latitude and seam-crossing
settlements; a crossing river segment with both endpoints outside the circle is
still detected. API tests reject invalid/nonfinite/out-of-range circles, clipping
settlement radii, private access and political territories; viewers can inspect
without changing World revision or save state.

## Browser/runtime verification

The T3 collaborative browser was used with isolated Windows CGO-free engines,
separate frontend builds and disposable SQLite data on ports 8156–8158. The author's
process on port 8124 remained PID 9596. No request, stop, restart or database edit
was directed at that process. The original working data directory was not opened
for writes by these test engines.

Observed builder button activation, circle placement, report loading and radius
updates. Observed a saved irregular settlement's circular PNG (384 px, transparent
corners), fit radius around 1.84 km, full outline and 50 km radius, with image URL
and inventory changing together. Its river query reported actual native river
segments and separate dry-season proxies. Generated and accepted 22 lifeforms
in three local native habitats, then observed all seven applicable groups in the
settlement report, including fungi, bacteria and viruses, without a false stale
warning. Plants are supported and test-covered; this polar test area's selected
habitats did not generate plants. Native screenshots were intermittently
unavailable; DOM inspection and a successfully saved profile screenshot provided
browser evidence. No alternative browser automation system was substituted.

The final compiled engine on port 8158 also exported and imported a native World
ZIP (71,656,516 bytes) through the actual HTTP endpoints into a separate copy.
The complete 50 km settlement resource report compared byte-identically before
and after import, including its resource source and provenance. The author's
World was not involved.

The final isolated preview on port 8158 was then checked again after the last
report refinements. It showed 124 generated/physical scalar summaries, native and
display PET means matching at 118.592 mm/year, biome proportions summing to one,
the 384 px fitted terrain image and the 50 km maximum. Renewable/surface-water
potential carried indexed-area units, aquifers carried annual recharge units and
construction deposits carried host-material tonnes. Earlier temporary preview
processes on 8156 and 8157 were stopped; only the isolated 8158 preview was left
running, alongside the untouched author process on 8124.

## Scope boundaries and unrun checks

The physical generator supplies a present-day tectonic snapshot, not full plate
history. Province histories, burial/maturity, mineralization and endowment are
explicit procedural inferences. The regional parent grid does not resolve ore
veins, exact spring outlets or individual organisms at settlement scale. Habitat
overlap is a saved species range, not proof that each point contains an organism.
Large report truncation, sampling, water units and stale analyses are visible in
the UI. An outline larger than the 50 km envelope is explicitly unsupported.

This task did not run a PostgreSQL instance, separate Linux resource replay,
whole-repository `go test ./...`, full legacy generator suite, dedicated profiling
or the standalone Playwright suites. They are unverified here. Historical retired
legacy timeout/container smoke-route failures were not rerun and are not claimed
fixed. No terrain physics, hydrology solver or unrelated Languages changes were
made to make checks pass. The generic UI-format failure remains separate from
functional resource results. Existing data is not automatically rewritten; restart
the application normally to activate the new code, with the source checkout and
matching frontend build.
