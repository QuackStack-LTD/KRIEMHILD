# Brief 4: checkout audit and delivery record

2026-10-27, `rebuild/continuous-terrain`, clean
`4ce394d10a61dbfdc4fdd50ef2350e20cb96a9b8`. Recent commits: `4ce394d`
verification, `02611b3` Script 2, `f4fe6e0` A2, `a563840` audit. Historical
Brief 3 evidence was read, not treated as a fresh execution. No AGENTS.md
was found in the workspace or its parent chain.

| Capability | Status | Evidence at this checkout / next increment |
| --- | --- | --- |
| Schema 1/2/3, old bytes and deterministic A1/A2 | working | Full product/spike tests rerun; `schema.go`, `productive_*`, `inverse_index.go`, old full goldens |
| Immutable state, World/Age, activation/CAS, retention | working | Full history/storage and focused HTTP tests rerun; `storage/languages.go`, `history/languages.go` |
| Script 1 Unicode mapping | working | Old goldens and pinned dependency tests rerun |
| Script 2 vectors, roles/readings and LTR/RTL | working | `vector_*`, full product/storage/API tests rerun; real editor browser baseline scheduled below |
| Passive SVG import | working | Negative/normalization/golden/API tests rerun; no original markup execution |
| Sequence spelling language-to-sign bridge | unsupported | No typed rule source or executor; add explicit schema 4 |
| Extended grapheme segmentation / cluster selection | unsupported | Strings retained only; add pinned tested Go UAX29 library |
| Script 2 Unicode/sign associations | persisted-only | Author annotations are not spelling rules or inferred glyph mappings |
| Sequence inverse | unsupported | Add bounded inverse derived from eligible forward rules; lossy stages explicit |
| Recursive feature metadata | persisted-only | No general recursive executor |
| Vertical/font/prosody/recursive grammar | unsupported | Preserve honest capabilities; out of this slice |
| Current browser/platform/database parity | unverified | Fresh checks launched against unchanged source; results appended after completion |

Fresh baseline commands (Windows amd64 Go 1.27.1, Node 24.21.0):

- `CGO_ENABLED=0 .tools/go/bin/go.exe test -count=1 ./internal/languages/... ./internal/history ./internal/storage`: pass, product 7.154s, spike 1.408s, history 1.257s, storage 4.792s.
- `.tools/go/bin/go.exe test -count=1 ./internal/httpapi -run '^TestLanguage'`: pass, 4.980s.
- `node --test tests/languages.test.mjs tests/vector-script.test.mjs`: 7 pass.
- `npm.cmd run check:ui`: existing Biosphere.jsx/BiosphereEditors.jsx formatting failures; unchanged.
- Go build, production UI build, 13 browser scenarios, 14-case Windows/Linux replay and SQLite/PostgreSQL restart/import checks launched; actual results will be appended.

The baseline replay/runtime images are Brief 3 images with the same implementation
source as this HEAD (the HEAD changes documentation only). The Windows CLI is
rebuilt now. Final checks will build new images from the delivered source.
Retired legacyterrain timeout and old generic smoke-route mismatch remain
historical unresolved failures; no test/source-grid changes are authorized here.
The collaborative browser explicitly reported unavailable after status/open;
repository Playwright tests provide browser evidence on disposable Worlds only.

The reviewed-before-coding semantics are in [SEQUENCE_ORTHOGRAPHY.md](SEQUENCE_ORTHOGRAPHY.md).
New behavior is opt-in language schema 4, never a reinterpretation of A2/schema 3.
The first complete scope is finite literal sequence rewriting, explicit whole
grapheme normalization, pinned Script 2 rendering and bounded eligible inverse
followed optionally by A2. No automatic migration or independent data store.

## Baseline completed before application implementation

| Check rerun now | Actual result |
| --- | --- |
| `.tools/go/bin/go.exe build ./...` with CGO disabled | Pass |
| `npm.cmd run build` | Pass, 21.17s |
| Browser opt-in, URL `http://127.0.0.1:8137`, `npx.cmd playwright test tests/languages.browser.spec.mjs tests/scripts.browser.spec.mjs` | 13 pass, 39.9s |
| Rebuilt baseline Windows CLI; `node scripts/languages-a1-replay.mjs kriemhild-languages:brief3 .cache/languages/brief4/baseline-cli.exe` | 14 full-result cases, two process repeats/platform; identical bytes and goldens |
| `node scripts/languages-container-smoke.mjs kriemhild-languages-runtime:brief3` | Fresh SQLite/PostgreSQL create, execute, restart, HTTP ZIP import and replay pass; temporary infrastructure removed |

These are fresh executions. Baseline Linux images contain the implementation
shared by the inspected HEAD, as stated above; delivery images below are rebuilt
from the new source. Baseline logs are `.cache/languages/brief4/baseline-*`.

## Implementation and compatibility decisions

Delivered code through `54e8e47`: audit/spec `2efa9e9`, pinned graphemes `df2b8c2`,
typed source/validation/forward/inverse and API/archive integration `846c913`,
inverse/Unicode hardening `23f0ed2`, application workbench `0b8b0fe`, normalization
trace policy `35d1151`, normalized revision replay `8a0772e`, owned compiled
dependencies/results `54e8e47`. Related model/execution/inverse changes are kept
together where their public types and readiness depend on one another; Unicode,
UI, portability fixtures and verification are separately reviewable.

- Language schema 4 is explicit: `languages/4`, `sequence-compose/1`,
  `languages-compiler/4`, source `sequence-orthography/1`. Old source schemas,
  bodies, A1/A2 algorithms, Script 1/2 versions and old golden files are retained.
- Orthography is authored inside the immutable language body. Its revision is
  that containing canonical revision, not an independently mutable head. Script
  identity/hash, renderer versions and Unicode policies are exact source pins.
  The canonicalizer remains `kriemhild-typed-json/1`; no migration occurs.
- Rules execute once per finite snapshot stage. Priorities are explicit;
  overlapping equal-priority patterns are rejected conservatively. Literal
  contexts and word edges are supported. Empty patterns, incompatible layers
  and missing signs/readings fail. Consuming expansions/deletion are explicit;
  there is no epsilon-input insertion, arbitrary expression or inferred mapping.
- Graphemes use Go-only rivo/uniseg 0.4.7 with Unicode 15.0 data. Existing x/text
  0.29.0 normalization is pinned separately. Authored strings remain exact;
  NFC/NFD are explicit stages with original/normalized strings and form in trace.
  Grapheme spans, UTF-8 bytes, UTF-16 offsets, code points and signs remain typed.
- Inverse metadata comes from forward rules. Bounded segmentations are validated
  by canonical forward replay (priority/contexts included). Coalescing identical
  traversal paths does not coalesce distinct preimages or lexical derivations.
  Normalization/deletion disable inverse honestly. Optional A2 runs afterward,
  with exact pre-erasure boundary verification and explicit propagation of limits.
- Compiled indexes and SVG remain derived. Existing cache identity includes the
  World, Age, source hash and compiler/adapter. Source hash includes all rule and
  dependency/policy pins. Cache reservations include the pinned Script snapshot.
  The snapshot and returned sign/provenance data are owned copies: mutation of a
  caller's draft or previous result cannot alter a cached revision.
- World archive **5** is required even for retained schema-4 bodies. The existing
  traversal now follows `sequenceWriting.script` alongside older dependencies;
  owner/hash/schema checks and missing/corrupt rejection remain. Archive 4 rejects
  these definitions. Old archive versions remain for old data.
- The existing Scripts tab supplies authoring and a saved-revision spelling
  laboratory. There is no second glyph editor. Controls use actual server
  capabilities/readiness, keep editing available, abort obsolete requests, and
  guard late results/errors. Written-output selection uses server cluster
  boundaries. Rule lists paginate at 20; source/rule/result budgets remain fixed.

## Delivery checks actually run

PowerShell from repository root; Windows amd64, Go 1.27.1, Node 24.21.0,
npm 11.19.0, Docker 29.8.2. Linux engine uses Go 1.27.3 amd64; runtime Alpine
is non-root, read-only except disposable data/tmp, with 256 MiB memory limit.
All test databases, containers and Worlds are synthetic/disposable. Neither the
author's active database nor the live 8124 process was used for tests.

| Command | Actual result |
| --- | --- |
| `CGO_ENABLED=0 .tools/go/bin/go.exe test -count=1 ./internal/languages/... ./internal/history ./internal/storage` | Pass: product 2.224s, spike 0.905s, history 1.122s, storage 5.184s |
| `.tools/go/bin/go.exe test -count=1 ./internal/httpapi -run '^TestLanguage'` | Pass, 4.434s; includes new full-result API fixtures, viewer restrictions and malformed Unicode |
| `.tools/go/bin/go.exe vet ./internal/languages/... ./internal/history ./internal/storage ./internal/httpapi ./cmd/languages`; `.tools/go/bin/go.exe build ./...` | Pass, CGO disabled |
| `npm.cmd test` | 143 pass, zero failures/skips, 90.410s; includes disposable project/server restart |
| `npm.cmd run build` | Pass, production React build 11.41s |
| Changed-file `npx.cmd prettier --check`; `node scripts/check-ui.mjs` | Pass; UI boundaries/compatibility checks pass |
| `npm.cmd run check:ui` at baseline | Existing two Biosphere formatting failures; those files remain unchanged |
| Browser opt-in, URL `http://127.0.0.1:8138`; `npx.cmd playwright test tests/languages.browser.spec.mjs tests/scripts.browser.spec.mjs tests/sequence.browser.spec.mjs` | 16 pass, 48.6s against final code; all prior workflows plus spelling, cluster selection, NFC publication, old revision/ZIP replay, conflicts, pagination, 390 px/keyboard and stale preview |
| `docker build -f docs/languages/Dockerfile.engine -t kriemhild-languages:brief4 .` | Fresh Linux engine tests, vet and CGO-free build pass |
| Rebuilt Windows CLI; `node scripts/languages-a1-replay.mjs kriemhild-languages:brief4 .cache/languages/brief4/cli.exe` | 19 cases, two separate process repeats/platform, raw UTF-8 bytes and complete goldens match; includes NFC, combining/emoji clusters, forward SVG and inverse/A2 |
| CGO-free `GOOS=linux GOARCH=amd64` app build and `docker build -f docs/languages/Dockerfile.runtime -t kriemhild-languages-runtime:brief4 .cache/languages/brief4/runtime-final` | Pass |
| `node scripts/languages-container-smoke.mjs kriemhild-languages-runtime:brief4` | Fresh SQLite and PostgreSQL 17 save/compile/activate/execute/restart/HTTP ZIP import into another World/replay pass, including newer normalized activation and retained earlier results |

Ignored evidence is under `.cache/languages/brief4/`: `go-final.log`,
`http-final.log`, `node-final.log`, `build-ui.log`, `browser-current.log`,
`platform-replay-final.log`, `runtime-current.log`, `docker-engine-final.log`,
`docker-runtime-final.log` and `benchmark.log`. Harness JSON results are also
written to `.cache/languages/a1-platform-replay.json` and
`.cache/languages/container-smoke.json` (existing harness names, now extended).

Initial failures were inspected and corrected, not hidden: a helper sentence
expanded an input's accessible name; spelling diagnostics needed display beside
the new editor; Go vet required a keyed external-package test literal. A2 failure
propagation and dependency/result aliasing were hardened during review, with
explicit boundary, inner-cancellation, trace-limit and snapshot tests. New full
goldens were authored/reviewed for the new profile; old ones were not regenerated.
Retired legacyterrain timeout and generic old smoke-route mismatch were not rerun
or fixed. No unrestricted whole-repository green-test claim is made.

## Acceptance and capability matrix

| Scenario / capability | Status | Exact evidence and limit |
| --- | --- | --- |
| A: forward spelling and pinned renderer | working | `sequence_schema.go`, `sequence_compile.go`, `sequence_execute.go`; `sequence-forward` full golden, API/browser/render/platform/SQL replay. Composed morphology and explicit raw-unit examples remain distinct. |
| B: conflicts/termination | working | `TestSequenceValidationCompatibilityAndReadiness`, `TestSequenceBoundariesNormalizationAndSnapshotOrder`; overlap/priority, contexts, empty patterns, unsupported layers, finite cyclic stage order, no-map and fixed budgets. |
| C: Unicode/cluster spans | working | `graphemes.go`, `TestPinnedExtendedGraphemeBoundaries`, `sequence-unicode` and normalized full goldens, API invalid UTF-8, browser cluster selection. Unicode 15.0 tested subset; no later-version/universal editor claim. |
| D: bounded eligible inverse | working | `sequence_inverse.go`, exhaustive forward-domain comparison through length 3 over the entire six-segment inventory, ambiguous segmentation, combining-fragment merging, contexts/word edges, boundary topology, homophones and explicit caps/cancellation. Normalization/deletion/author-disabled inverse are unsupported. |
| E: immutable revisions | working | `TestLanguageSequenceImmutableRestartArchiveAndOwnership`, `TestSequenceArtifactOwnsDependenciesAndResults`, browser normalized revision/old replay, SQL activation/restart. Old rules, geometry and readings remain pinned. |
| F: World archive | working | Archive 5, old-reader rejection, missing retained dependency rejection and cross-World reference rejection; actual HTTP ZIP import/replay in browser and both SQL runtimes. |
| G: platform/storage parity | working | 19 complete CLI replay cases Windows/Linux; SQLite/PostgreSQL create/restart/import. Derived artifact keys differ by World/Age/source; outputs exclude cache/timing metadata. |
| H: honest boundaries | working | Server schema-4 capabilities/readiness, unsupported-inverse warnings, UI gating and explicit unsupported scope below. Script 2 standalone capabilities remain unchanged. |
| Literal/contextual spelling and Script 2 sign bridge | working | Actual typed execution, not settings-only. Rules and pins persist in canonical language source. |
| Rule authoring/publication/compare/replay | working | Existing Save/Validate/Compile/Activate/History controls plus Scripts editor and browser workflows. Changing kind cannot silently discard authored rules. |
| Written-output cluster cursor/selection | working | `ClusterText` uses returned UTF-16 boundaries; byte/code-point offsets explicitly labeled. |
| General raw-text editor cursor behavior | partial | Raw source fields remain native Unicode inputs; cluster-aware selection is delivered for processed output, not every text field. |
| General inverse phonology / lossy inverse | unsupported | A2 unchanged; NFC/NFD/deletion do not pretend to have complete reverse relations. |
| Structured sequence corpus capture | unsupported | Old candidate-only corpus explicitly rejects sequence cases; full pinned external goldens execute. Add typed expectations and explicit capture in a later slice. |
| Native backend / production-scale throughput / measured peak memory | unverified | No native backend introduced. Bounded fixtures and allocations measured below; no peak-heap or universal scale claim. |
| Semantic associations / writing profile intent | persisted-only | Author metadata, not automatic meaning, lexical reference resolution or shaping. |
| Vertical/2D quadrats/advanced bidi/font export/prosody/recursive grammar | unsupported | Existing honest unsupported boundaries remain; no misleading controls added. |

## Repeatable performance evidence

Command:

```powershell
.tools/go/bin/go.exe test ./internal/languages -run '^$' -bench '^BenchmarkSequenceSpelling$' -benchtime=3x -count=5 -benchmem
```

Fixed deterministic counter-generated literal rule patterns over the authored six
segment inventory (unique extra length-three patterns, not repeated fake entries).
Input repeats the explicit `p a d a` test stream to vary sequence size. First
rewrite stage only measures spelling, excluding SVG; renderer correctness/limits
are tested separately. Cold means compile + execute with definitions already in
memory, not process/disk cold. Warm reuses one immutable artifact. Five samples,
three operations per sample; desktop/concurrent check activity affects timings.
No seed/random selection. Values are observed min/median/max, not confidence bounds.

| Units / rules | Trial | Min / median / max ms | Median total allocated KiB/op | Outcome / steps |
| --- | --- | --- | --- | --- |
| 32 / 16 | warm | 0.651 / 0.709 / 0.911 | 207.6 | success, 380 steps, 1 spelling |
| 32 / 16 | cold | 2.061 / 2.137 / 2.326 | 457.9 | success, 380 execution steps |
| 128 / 64 | warm | 6.909 / 7.162 / 7.814 | 1626.5 | success, 5,027 steps |
| 128 / 64 | cold | 11.526 / 12.095 / 13.245 | 2545.1 | success, 5,027 execution steps |
| 512 / 128 | warm | 48.338 / 48.879 / 50.816 | 11441.1 | success, 39,555 steps |
| 512 / 128 | cold | 61.759 / 62.661 / 63.962 | 14261.6 | success, 39,555 execution steps |
| 12 ambiguous `d` output clusters | inverse | 0.634 / 0.643 / 0.766 | 260.7 | execution_limit at 1,175 steps / live-path cap; zero partial preimages |

These are allocation totals, **not measured peak heap/RSS**. Serialized
intermediate/result and trace budgets are not allocator measurements. The
100,000-step / 128-path / 512-unit / 1 MiB trace / 8 MiB payload limits were not
raised. Pathological inputs terminate explicitly. Larger authored languages,
retained/peak heap profiling and broader script-specific performance are next
measurement tasks, not demonstrated production scalability.

## Final retained evidence and review preview

Code behavior is committed through `54e8e47`; following documentation commits
do not change that behavior. Final images built from this source:

- Engine: `sha256:9b02f78527b79b3ceeafedb3b9c5696b1901391564f4bc2458c0cea425f43c90`.
- Application runtime: `sha256:4b4f1099a4508d14ef87c299ead9d7c62b1716cc2b2497e18ce8efc44169be9f`.

The final SQLite and PostgreSQL runs reached World revision **25**, exported
16,207 / 16,220 bytes respectively, restarted actual app processes and replayed
all pinned cases after importing into a different World. Both retain the
original sequence revision after activating the newer NFC revision. All
temporary containers, volumes and networks were removed by the harness.

Complete CLI UTF-8 output hashes (two process repeats on each platform):

| Fixture | Bytes | SHA-256 |
| --- | --- | --- |
| sequence forward | 26,043 | `f86504aadb78be5776192177341349e1100f20f03899b79eee285aac2b6b6544` |
| inverse before A2 | 36,535 | `1468de56a226bcdd7ec15cdc5f62c2a082f513c779e16dd7bf6487a6bf1b307f` |
| combining/emoji clusters | 3,724 | `200046b86e4425623bb46c634aa60b24cdfea6b584657e5f869bd69b403c3ba6` |
| explicit NFC revision | 29,594 | `70133dccf0aae535a861747b374766c1bf0ca95c6cfb3320000367f60fa596cf` |

An isolated review app runs at `http://127.0.0.1:8138` using versioned binary/dist
`.cache/languages/brief4/verified-54e8e47` and disposable test data. The author's
existing 8124 instance and the earlier 8137 preview are unchanged. No author
World was migrated, saved or normalized by this work.
