# Brief 6 — final sequence-corpus audit and handover

2026-10-29. The supported Brief 5 linguistic coverage gaps are closed by **25
dedicated reviewed external corpus cases**, including explicit definition
rejections and exact pre-erasure inverse/A2 topology with shifted and missing
boundary negatives. The original four-case corpus and historical golden bytes
are unchanged. Review is explicitly Codex independent derivation, authorized by
the author; it is not a claim of human approval. This is the final handover for
the current sequence-orthography corpus workstream.

## Checkout, instructions and environment

- Repository: `C:\Users\tomyi\worldbuilding\KRIEMHILD`. The supplied parent
  `worldbuilding` directory is not itself a Git repository; discovery selected
  its actual `KRIEMHILD` checkout.
- Branch: `rebuild/continuous-terrain`; starting and delivery HEAD:
  `2ff34a5d00cb90cd9cbd5dabeb9d46c881fa6dfd`. Starting `git status --short` was
  empty. Delivery changes are left in the working tree; no commit or PR created.
- `git log -8 --oneline`: `2ff34a5`, `6a805d0`, `dfcad40`, `eb40802`, `9f83cd1`,
  `8d2eca1`, `64d6b88`, `f44aeda`. `git merge-base --is-ancestor 6a805d0 HEAD`
  exited 0. Brief 5 is present as actual source, not inferred from its audit.
- No `AGENTS.md` was found by repository search or in the ancestor chain through
  `C:\`. The current sequence specification, corpus specification, Brief 5 audit,
  corpus runner/fixtures, sequence compiler/executor/inverse, productive A2 index,
  boundary derivations, API, storage/archive and production tests were inspected.
- Windows amd64, OS `Microsoft Windows NT 10.0.19045.0`, PowerShell. `go` is not
  on PATH; all native Go commands use `.tools/go/bin/go.exe` (Go 1.27.1).
  `CGO_ENABLED=0`; native GOOS/GOARCH `windows/amd64`. Node `v24.21.0`,
  `npm.cmd 11.19.0`, Docker client/server `29.8.2`, Linux Docker engine available.
  `npm.ps1` is blocked by the existing execution policy; `npm.cmd` works.
  No toolchain, execution-policy or global environment setting was changed.
- Fresh Linux engine image uses Go **1.27.3 linux/amd64**, `CGO_ENABLED=0`.
  The app runtime binary was cross-built with native Go 1.27.1 and verified by
  `go version -m` as `GOOS=linux`, `GOARCH=amd64`, `CGO_ENABLED=0`.
  Runtime uses Alpine 3.23 and disposable `postgres:17-alpine`.

## Baseline before edits

Commands below run from the repository root. `go` in command tables means the
literal local executable `.tools/go/bin/go.exe`; `CGO_ENABLED=0` was set for Go
commands. Complete operational logs are ignored under `.cache/languages/brief6/`.

| Exact check                                                                       | Observed baseline                                                                                               |
| --------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `go test -count=1 ./internal/languages/... ./internal/history ./internal/storage` | Pass: languages 11.222s, spike 1.606s, history 1.202s, storage 4.385s                                           |
| `go test -count=1 ./internal/httpapi -run '^TestLanguage'`                        | Pass, 6.797s                                                                                                    |
| `go build -o .cache/languages/brief6/cli.exe ./cmd/languages`                     | Pass                                                                                                            |
| `node scripts/languages-corpus-cli-check.mjs .cache/languages/brief6/cli.exe`     | Original ten scenarios pass, including exact exit 0/1/2/3, invalid inputs, pending review and runner exhaustion |
| `npm.cmd test`                                                                    | 143 pass, 0 fail, 72.269s; same complete suite subsequently required no frontend changes                        |
| `npm.cmd run build`                                                               | Pass, Vite 16.81s; npm update notice is informational                                                           |
| `npm.cmd run check:ui`                                                            | Fail: 34 unchanged files need formatting; full list below                                                       |

The language/history/storage/API/CLI baseline completed before editing. Frontend
checks were also started before the implementation changes and are not claimed
as new frontend coverage. The initial process observation was host port 8124,
PID 9596; its executable and database were excluded from all work.

## Implementation and independent provenance

Fixtures live in `internal/languages/testdata/sequence-corpus/`, with a provenance
README and nine explicitly supplied language definitions. Every corpus is one
focused case. The complete fixture corpus is 205,216 UTF-8 file bytes, 25 cases:
13 success, 1 ambiguous, 3 no_result, 3 unsupported, 5 rejected. All are reviewed;
the original four-case corpus is retained separately. NFC/Unicode focused wrappers
reuse the approved full expectations; no historical golden file was rewritten.

The author explicitly answered: “Yes—independent derivation with explicit Codex
provenance.” Newly authored expectations identify
`Codex (independent derivation; not human approval)`. Their per-case notes cite
retained full goldens, named production tests, documented semantics and explicit
derivations/work arithmetic. No engine output was captured to generate expected
data. The one-time serialization worksheet used explicit tables, canonical hashing
and retained values; it did not call Compile/Execute. Tests never write fixtures
or promote review metadata. Pending review continues to fail verification.

| Dedicated cases                                                             | What is compared                                                                                                                                                                                              |
| --------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `literal-contexts`, `left-context-miss`, `right-context-miss`, `word-edges` | Literal left/right snapshot contexts and virtual beginning/end word edges, exact rule selection, outputs, source-token spans and provenance                                                                   |
| `priority-overlap`                                                          | Higher priority consumes overlapping `p,a` before the lower-priority word-edge singleton, preserving merged `[0,1]` provenance                                                                                |
| `no-match`, `wrong-input-layer`                                             | Valid no-match is raw `no_mapping` / outcome `no_result`; a grapheme at a surface input is raw `invalid_unit` / rejected                                                                                      |
| `invalid-layer`, `invalid-conflict`                                         | Production definition validation rejects adjacent-layer incompatibility and equal-priority overlap; exact status, entire readiness map and ordered diagnostics, without Execute                               |
| `nfc-exact`, `nfd-forward`                                                  | Exact original/normalized text, normalization stage, typed spans, provenance and complete clusters; NFD is explicitly `U+00E9 -> U+0065 U+0301`                                                               |
| `unicode-15-clusters`, `unicode-coordinates`                                | Retained combining-mark, supplementary scalar, ZWJ, flag and modifier examples; four distinct coordinate tables. The reviewed text totals 42 UTF-8 bytes, 15 code points, 22 UTF-16 code units and 8 clusters |
| `deletion-forward` and three `*-inverse-unsupported` cases                  | Explicit deleted match and surviving provenance; actual inverse rejection for deletion, NFD and authored inverse-disable policy; no fabricated reverse mappings                                               |
| `missing-script-sign`, `missing-script-reading`                             | Production raw `missing_sign` and `unresolved_reading` under an exact valid Script pin, both rejected rather than relabelled unsupported/no-result                                                            |
| `topology-exact-{forward,inverse-a2}`                                       | Actual sequence inverse followed by A2, exact `p,a,d,\|,a` stream; complete historical rock/stone candidates, allomorph alternatives, derivations and boundary provenance                                     |
| `topology-mismatch-{forward,inverse-a2}`                                    | Same surface segments, boundary shifted from index 3 to 2: explicit raw spelling succeeds, A2 topology filtering returns `no_result`                                                                          |
| `topology-missing-{forward,inverse-a2}`                                     | Same surface segments, boundary omitted: explicit raw spelling succeeds, A2 rejects both analyses because their pre-erasure stream contains the boundary                                                      |

The positive topology case retains the reviewed A2 derivation bytes exactly:
post-boundary-rule `p,a,d,|,a`, followed by `erase-boundaries` producing `p,a,d,a`.
Only the containing orthography revision and its revision-bound index/request IDs
are rehashed. The independent A2 algorithm, lexical candidates, candidate IDs,
allomorph decisions and historical golden data are unchanged. Negative production
results honestly contain no accepted derivations; paired forward cases retain
the deliberately mismatched source topology. Separate tests reject leading,
trailing and adjacent morpheme boundaries instead of shifting or merging them.

During first comparison, two worksheet mistakes were exposed and retained in
`new-fixture-tests.log`: no-match diagnostics identify the stage in `ruleId`, and
the A2 index identity hashes the dependency slice at its construction time.
Source inspection showed `compileProductive/buildInverse` precedes
`compileSpelling`, so that slice is nil for these fixtures. Those two expectations
were corrected from the source ordering; no actual result was copied as a golden.
All independent work counts and linguistic derivations matched on first execution.

## Additive format and architecture

Most new execution cases use `sequence-corpus/1`. Explicit `/2` is needed because
the old format cannot honestly express three actual engine/interface shapes:
early inverse rejection before pipeline metadata initialization, the renderer's
`missing_sign`/`unresolved_reading` statuses, and compiler-validation expectations.
It additionally names cumulative Unicode code-point spans separately from UTF-8,
UTF-16 and grapheme spans. See [SEQUENCE_CORPUS.md](SEQUENCE_CORPUS.md).

- `/1` retains its old reading, expectations and canonical reports. New fields
  are rejected in `/1`, even explicit null/empty. Compatibility tests check this.
  `/2` retains the selected version in valid reports; CLI pre-decode failures
  retain the historical `/1` error envelope and exit 2.
- Definition-validation is an explicit phase using the same runner, resolver,
  preflight, artifact cache, bounds, cancellation, review gates and diffs. It
  compares the complete typed semantic validation record: status, full readiness,
  ordered diagnostics and omitted count. Compile size/capability metadata is
  outside that new record. All execution cases still compare complete Results.
- No second spelling implementation or corpus runner was added. Existing
  Compile/Execute/RenderScript remain authoritative. Corpus validate-only has
  zero compiler/execution calls; execution-mode definition validation compiles
  without Execute and without saving a compile record.
- Exact immutable source and Script revisions are resolved before any case runs.
  No active-head fallback, mutation, publication or activation occurs in the
  runner. Read-only API authorization, CSRF, final membership/ownership recheck,
  bounded jobs, CLI JSON/exit codes and canonical ordering are preserved.
- Complete result comparison, null/empty distinctions, trace/rule provenance,
  exact Unicode bytes, bounded inverse candidate sets and golden protection remain.
  Typed coordinate tables are redundant exact assertions on complete clusters,
  not normalization-insensitive comparison or a second segmenter.
- The production **16 MiB LRU reservation** and all existing source/request/work,
  trace/report/output, inverse-path/candidate and renderer limits are unchanged.
  Reservation/accounting is not measured peak memory. Bounded/interrupted inverse
  searches are not labelled exhaustive. Complete rejection comparison does not
  imply exhaustive linguistic inversion.
- No production sequence, A1/A2, boundary, Script, renderer, normalizer,
  canonicalizer, language schema or archive semantics changed. Source changes
  outside fixture data are limited to corpus format/runner and API discovery/tests.

## Regression and portability results

| Command / environment                                                                                                                                                                       | Final observed result                                                                                                                                                                                                                                                                                                             |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `go test -count=1 ./internal/languages/... ./internal/history ./internal/storage`                                                                                                           | Pass: languages 17.411s, spike 1.798s, history 2.509s, storage 13.462s; loaded concurrent build run                                                                                                                                                                                                                               |
| `go test -count=1 ./internal/httpapi -run '^TestLanguage'`                                                                                                                                  | Pass, 6.462s; old and `/2` corpus execution, view-only access, no compile/activation writes, CSRF/auth and final membership revocation                                                                                                                                                                                            |
| `go test -count=1 ./internal/languages -run '^TestSequenceCorpus'`                                                                                                                          | Pass, 2.606s; focused full-result comparisons, strict `/2` shapes, review gate, coordinate and validation diffs                                                                                                                                                                                                                   |
| `go test -count=1 ./internal/languages -run '^TestSequenceCorpusRejectsMalformedBoundaryTopology$'`                                                                                         | Pass, 1.195s; final added malformed-boundary test, also included in final Linux build tests                                                                                                                                                                                                                                       |
| `go vet ./internal/languages/... ./internal/history ./internal/storage ./internal/httpapi ./cmd/languages`                                                                                  | Pass                                                                                                                                                                                                                                                                                                                              |
| `go build ./...`                                                                                                                                                                            | Pass                                                                                                                                                                                                                                                                                                                              |
| `node scripts/languages-corpus-cli-check.mjs .cache/languages/brief6/cli.exe`                                                                                                               | **69 scenarios pass**: original ten, 25 validate/execute pairs, eight forbidden downgrades and pending definition-review gate. Observed exit distribution 0:52, 1:3, 2:13, 3:1; negative exits are asserted successes of the checker                                                                                              |
| `npm.cmd test`; `npm.cmd run build`                                                                                                                                                         | 143/143 tests and production build pass; frontend source unchanged                                                                                                                                                                                                                                                                |
| `node scripts/check-ui.mjs`                                                                                                                                                                 | Pass: UI boundaries/shared tabs/dialog/compatibility                                                                                                                                                                                                                                                                              |
| Changed-file Prettier check, changed-Go `gofmt -l`, `git diff --check`                                                                                                                      | Pass; formatting evidence recorded after audit authoring                                                                                                                                                                                                                                                                          |
| `npm.cmd run check:ui`                                                                                                                                                                      | Fail on 34 pre-existing unchanged files; not weakened or omitted                                                                                                                                                                                                                                                                  |
| `KRIEMHILD_LANGUAGES_TEST=1 KRIEMHILD_UI_URL=http://127.0.0.1:8147 npx.cmd playwright test tests/languages.browser.spec.mjs tests/scripts.browser.spec.mjs tests/sequence.browser.spec.mjs` | **17 pass**, 58.3s, final backend and disposable World/database; earlier isolated 8146 run also 17 pass, 50.2s                                                                                                                                                                                                                    |
| `docker build --no-cache -f docs/languages/Dockerfile.engine -t kriemhild-languages:brief6 .`                                                                                               | Fresh Linux CGO-free language tests, vet and CLI build pass. Final additional test-only layer rebuilt with `docker build -f docs/languages/Dockerfile.engine -t kriemhild-languages:brief6 .` and passed                                                                                                                          |
| `node scripts/languages-a1-replay.mjs kriemhild-languages:brief6 .cache/languages/brief6/cli.exe`                                                                                           | **45 scenarios, two separate processes/platform**, Windows/Linux complete byte identity; every retained golden comparison passes, plus all 25 focused corpora                                                                                                                                                                     |
| `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o .cache/languages/brief6/runtime/kriemhild ./cmd/kriemhild`                                                                     | Pass; build metadata confirms CGO-free Linux target                                                                                                                                                                                                                                                                               |
| Copy production `dist` and `docs/languages/Dockerfile.runtime` into `.cache/languages/brief6/runtime`; `docker build -t kriemhild-languages-runtime:brief6 .cache/languages/brief6/runtime` | Pass; informational existing Docker warning about the `KRIEMHILD_AUTH_MODE` ENV name                                                                                                                                                                                                                                              |
| `node scripts/languages-container-smoke.mjs kriemhild-languages-runtime:brief6`                                                                                                             | Fresh SQLite and PostgreSQL create/compile/activate/execute, process restart, schema-5 HTTP ZIP import into a different World and complete old/new corpus replay pass. Retained coverage sources are saved without compile/activation; verification leaves World revision unchanged. Temporary containers/network/volumes removed |

Portability report: `.cache/languages/a1-platform-replay.json`, 45 result byte
counts/hashes; runtime report: `.cache/languages/container-smoke.json`, also
recorded in `brief6/runtime-check.log`. The original `/1` corpus report is still
119,208 bytes, SHA-256
`a16f11aa8c6d1c1822d9b34ff1d6952892e7c5cebf960d2b63d3b8fc43dbfef9`,
exactly the previously reported value. No old `.golden.json` is modified.

Runtime image:
`sha256:0ccc3257596dcf6542684c62b78b2abccb30a74347d4d9ada0164d27e17f022c`.
Linux tests/vet/build and replay used the same production implementation; the
last engine-image rebuild adds only the malformed-boundary test. Its image
identity is
`sha256:dadf5e09a1b487b41c361683e6aeb5bcc457ec8d311b1c2e1db17da1b75d104a`
(`docker-engine-final-tests.log`). The earlier replayed image was
`sha256:0ed9b64dc7d27c856b1c62d609a1725fce1bd4e05cfb7a3eefe85d1a880cb2d9`.

Schema compatibility evidence includes native storage/history tests
`TestLanguageSequenceImmutableRestartArchiveAndOwnership` (archive 5 and
archive-4 reader rejection), `TestLanguageIndexedRestartZIPAndImmutableVersions`
(archive 4 and older reader barrier), vector/mixed productive revision tests,
and `TestLanguageRevisionsAgesRestartAndPortability` (older schema-2 language
archive retained/imported). Schemas 1–4 and Scripts 1/2 retain old bodies and
execution goldens. Schema-5 real HTTP ZIP replay was rerun on both databases;
older archive reader/barrier checks are unit/integration evidence, not claims
that every old archive has been tested against every database.

Native T3 preview status/open was attempted first. Open timed out; subsequent
status explicitly reported no connected automation host and instructed no retry.
Repository Playwright suites then supplied the browser evidence. A combined final
preview launch command was blocked by automatic command review; the separate
foreground build/server path completed safely. One early launch also preceded
build completion and failed to locate the executable; after the build completed,
the server and all browser tests passed. Neither event was an application/test
regression or a reason to touch the author process.

## Performance and resource evidence

Re-ran the existing four-expectation verification benchmark because corpus decode
and result-comparison code changed:

```text
CGO_ENABLED=0 GOMAXPROCS=4 go test ./internal/languages -run '^$' -bench '^BenchmarkSequenceCorpus$' -benchtime=3x -count=5 -benchmem
```

Windows amd64, Go 1.27.1, Intel Core i5-6300HQ. Five sample means/mode, three
complete runs/sample. Original file 129,840 bytes; typed canonical corpus 45,212
bytes; four expectations, two language revisions, one Script; each run 18 calls,
14,800,000 charged work units and seven bounded domain streams. Primary outcomes
three success and one ambiguous, stopping reason completed. Browser/replay work
was concurrent during sampling; these are not exclusive-machine latency samples.

| Mode                                                             | Min / median / max sample mean, ms/op | Median B/op | Median allocs/op |
| ---------------------------------------------------------------- | ------------------------------------- | ----------- | ---------------- |
| Warm decoded transport snapshots; artifacts recompiled every run | 52.991 / 81.913 / 148.386             | 8,134,973   | 101,091          |
| Cold fixture/source reads and strict decode included             | 66.710 / 103.232 / 120.607            | 9,556,760   | 114,269          |

These are allocation totals, not retained/peak heap or RSS. The 25 new focused
fixtures were verified, not throughput-profiled as a large corpus. No scale,
peak-memory, production SLA or speedup claim follows from these samples, and no
engine/cache limit was raised. Dedicated peak-memory and production-scale
profiling remain unverified final scope boundaries.

## Final capability matrix

“Reviewed corpus-covered” below requires a dedicated reviewed external case;
production tests alone are explicitly classified separately.

| Behavior                                                                                                                                    | Final classification      | Evidence / precise boundary                                                                                                   |
| ------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| Literal left/right contexts and context failure                                                                                             | Reviewed corpus-covered   | Three context cases, immutable snapshot traces                                                                                |
| Virtual word edges, beginning and end                                                                                                       | Reviewed corpus-covered   | `word-edges` plus context cases                                                                                               |
| Explicit priority and overlapping consuming rules                                                                                           | Reviewed corpus-covered   | `priority-overlap`, full source/rule provenance                                                                               |
| Equal-priority conflict validation                                                                                                          | Reviewed corpus-covered   | `invalid-conflict`, definition-validation rejection                                                                           |
| Incompatible pipeline layers and request units                                                                                              | Reviewed corpus-covered   | `invalid-layer`, `wrong-input-layer`                                                                                          |
| Valid no-match                                                                                                                              | Reviewed corpus-covered   | `no-match`, raw no_mapping distinct from malformed input                                                                      |
| Morpheme stream preserved in forward trace                                                                                                  | Reviewed corpus-covered   | Exact/shifted/missing topology forward cases                                                                                  |
| Exact pre-erasure inverse followed by A2                                                                                                    | Reviewed corpus-covered   | Full two-analysis derivations, boundary at index 3, shifted and absent negatives                                              |
| Leading/trailing/adjacent malformed boundaries                                                                                              | Test-covered only         | Dedicated strict topology test rejects rather than repairs                                                                    |
| Stored physical word-boundary tokens/erasure                                                                                                | Unsupported or unverified | Word edges are virtual; production unit validation only permits morpheme boundary tokens                                      |
| NFC with original and normalized forms                                                                                                      | Reviewed corpus-covered   | `nfc-exact`, unchanged retained golden                                                                                        |
| NFD with exact trace/output                                                                                                                 | Reviewed corpus-covered   | `nfd-forward`, no normalization-equivalent comparison; only supported stable cluster alignment claimed                        |
| Combining/supplementary/multi-code-point clusters, Unicode 15.0                                                                             | Reviewed corpus-covered   | Focused Unicode wrapper, exact four-coordinate `/2` table                                                                     |
| Later Unicode, arbitrary font/shaping/editor behavior                                                                                       | Unsupported or unverified | Pinned Unicode 15.0 implementation and authored examples only                                                                 |
| Deletion and surviving source provenance                                                                                                    | Reviewed corpus-covered   | `deletion-forward`, explicit deleted span                                                                                     |
| Lossy-stage / configured inverse rejection                                                                                                  | Reviewed corpus-covered   | Three actual unsupported_inverse expectations; no preimages invented                                                          |
| Missing Script sign/reading                                                                                                                 | Reviewed corpus-covered   | Actual missing_sign / unresolved_reading statuses                                                                             |
| Complete inverse homophone derivations                                                                                                      | Reviewed corpus-covered   | Retained original inverse and dedicated topology case                                                                         |
| Cancelled/limit/incomplete versus no-result status honesty                                                                                  | Test-covered only         | Original inner-cancellation/A2-cap tests, runner budgets and CLI exit 3; no dedicated reviewed interrupted-search golden      |
| Bounded forward/inverse membership over `{p,a}` length 0–2                                                                                  | Reviewed corpus-covered   | Existing seven-stream corpus property; no lossy stages/A2 or wider-domain exhaustiveness claim                                |
| Strict decoding, complete pins, review gate, deterministic diffs                                                                            | Test-covered only         | Adversarial `/1`/`/2` tests and 69 CLI scenarios; successful linguistic cases do not substitute for these negative tests      |
| Read-only authorization, final revocation, ownership                                                                                        | Test-covered only         | Focused HTTP API and storage tests; both DB runtime runs check no replay writes                                               |
| LRU reservation, cancellation and bounded work/report behavior                                                                              | Test-covered only         | Preserved existing tests/limits plus `/2` validation gate/shape checks; not measured peak memory                              |
| Language schemas 1–4, A1/A2, Script 1/2 compatibility                                                                                       | Test-covered only         | Historical goldens, native tests and 45-scenario cross-platform replay; sequence corpus itself targets schema 4               |
| Archive 4/5 and older archive compatibility                                                                                                 | Test-covered only         | Storage barriers/import tests and real schema-5 ZIP replay on both DBs                                                        |
| Windows/Linux bytes, rebuilt definitions, DB restart/import                                                                                 | Reviewed corpus-covered   | Actual reviewed corpus replayed twice/platform and on both DBs after restart/import; unrelated app portability is not implied |
| Full end-user corpus-authoring UI                                                                                                           | Unsupported or unverified | Existing spelling/glyph UI retained; no full corpus editor implemented                                                        |
| Large-corpus throughput and peak heap/RSS                                                                                                   | Unsupported or unverified | Only small verification benchmark measured                                                                                    |
| General inverse phonology/spelling, prosody/stress/tone, recursive grammar, vertical/quadrat/bidi/font export, inferred semantics/ligatures | Unsupported or unverified | Explicit final scope exclusions; no code/controls or claims added                                                             |

The Brief 5 partial context/priority/morpheme/NFD/lossy/exact-A2 entries are now
resolved for the supported representation. Physical word-boundary tokens remain
unsupported, explicitly distinguished from tested virtual word edges. General
normalization alignment and universal Unicode/editor behavior are not inferred
from the specific reviewed examples.

## Known failures, omissions and environment safeguards

`npm.cmd run check:ui` fails formatting on these **unchanged** files:
`src/continuous/studio.css`, `src/worldbuilding/languages.css`,
`src/worldbuilding/workspace.css`; worldbuilding JSX files `ArchiveTools`,
`Biosphere`, `BiosphereEditors`, `ConceptEditors`, `ConnectionEditor`, `Culture`,
`GlyphWorkbench`, `IllustratedMapImage`, `LanguageEditors`, `LanguageHistory`,
`Languages`, `Library`, `NavigationIcon`, `Networks`, `Profile`,
`SequenceOrthography`, `SystemDesigner`, `Workspace`, `YearTimeline`;
`src/App.jsx`; continuous JSX files `ArchivedTerrain`, `IllustrationPanel`,
`IllustrationScene`, `Studio`, `Terrain3D`, `TerrainRouter`, `TileSurface`,
`ViewportSurface`; `src/main.jsx`, `tests/ui.test.mjs`, `playwright.config.mjs`.
This reproduces Biosphere formatting and proves the current failure set is wider
than Brief 5 reported, including unchanged Languages UI. It is a pre-existing
formatting failure outside these changes, not a green UI formatting claim.
Changed files pass their own formatting checks; direct UI compatibility checks
pass. No unrelated formatting was repaired.

The retired `internal/legacyterrain` timeout and generic container smoke-route
mismatch were **not rerun** or repaired. They remain historical known issues,
not newly reproduced failures or passes. The focused Languages runtime checks
passing does not establish that the generic container smoke check passes.
`go test ./...`, all unrelated browser suites, universal archive/database
combinations, race testing and dedicated peak-memory/scale profiling were not
run. No full-repository-green claim is made.

No terrain/source-grid, hydrology, Biosphere or unrelated production source was
modified. Host port **8124 remained PID 9596**, executable
`.cache/languages/activated-a7475f8/kriemhild.exe`, at baseline and final inspection.
Its World/database and process were never queried through the app or mutated.
Tests used temporary Go directories, disposable World records, isolated preview
ports 8146/8147 and new `.cache/languages/brief6/` databases. Docker's internal
port 8124 is inside a private container network namespace with no published host
port. Docker smoke resources were removed; owned preview processes were stopped
after verification. Existing author data/environment and existing server remain.

All applicable acceptance criteria have executable evidence above. Unsupported
representations and unrun checks are final documented scope boundaries. The
workstream ends with this implementation, matrix and handover.
