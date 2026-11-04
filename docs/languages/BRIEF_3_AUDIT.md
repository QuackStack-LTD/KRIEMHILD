# Brief 3 audit and delivery record

Baseline: clean `131d46b7ed67cf4e75f317794ffb0d7446979958`, branch
`rebuild/continuous-terrain`, 2026-10-26. Recent Languages implementation is
`a7475f8`; `131d46b` changes Illustrator/navigation only. No Brief 2 note exists
under that name; CONTINUATION_AUDIT, PRODUCTIVE_A1, BACKEND_DECISION_A1 and
VERIFICATION provide the relevant historical evidence. This audit is committed
before implementation. Historical Windows/Linux/browser/SQL claims below are
not treated as newly rerun evidence.

| Capability | Status | Current code/test evidence | Missing behavior / next increment |
| --- | --- | --- | --- |
| Strict source schemas and canonical bodies | working | `schema.go`, spike decoder; engine/spike tests rerun | Add explicitly versioned profiles, preserve old bytes |
| A1 typed productive plans / ordered lazy composition | working | `productive_schema.go`, `productive_compile.go`, `productive_execute.go`; productive tests rerun | No general FST optimizer |
| Conditioned allomorphy, whole-form circumfix, zero, suppletion | working | `composeCell`, `affix`; round-trip and provenance fixtures rerun | Infix/reduplication/interdigitation unsupported |
| Boundary stratum and typed traces | working | `applyRule`, `ProductiveStep`; insertion/deletion/ordering goldens rerun | Only retain-until-surface stratum |
| Inverse composition | partial | A1 enumerates every lexical cell before comparison; benchmark rerun | 1,000 entries reaches execution_limit at 1,384 examined cells; indexed pruning is the first priority |
| Version/compiler/dependency identity | working | `Versions`, `ArtifactKey`, pinned ScriptRef; storage/HTTP tests | Preserve schema 1 and A1; new optimization profile must not rewrite old diagnostic results |
| Cancellation/work/payload bounds | working | productive inner loops, two HTTP slots, five-second context, limits tests | New index loops and script geometry need separate explicit bounds |
| World/Age state, immutable revisions, CAS, activation | working | `storage/languages.go`, history model; storage/HTTP tests rerun | Extend existing lifecycle, no new storage |
| World ZIP ownership/hash/dependency traversal | working | `walkLinguisticBodies`, mixed-revision/archive tests rerun | New source versions require an older-reader barrier |
| API readiness and UI authoring | working | capabilities endpoint, productive editor, Laboratory; Node checks/build rerun | New executable profiles must be exposed from server capabilities |
| Independent Unicode script mapping / normalization | working | `DecodeScript`, `write.go`, script pinning and old goldens rerun | No ordered sequence spelling rules |
| Glyph asset/direction metadata | persisted-only | Script schema 1 Grapheme.GlyphAsset/Direction | No custom geometry or layout execution |
| Vector glyph editor / SVG import | unsupported | No existing implementation found | Script schema 2 normalized geometry, editor and safe import |
| Mixed signs/readings/semantic classifiers | unsupported | No executable sign-sequence model | Explicit authored roles, readings and sequence selection |
| Custom deterministic rendering / RTL | unsupported | Unicode strings only; direction metadata does not affect rendering | Pinned vector renderer, no font-export claim |
| Grapheme-cluster editor semantics | unsupported | Complete Unicode mapping strings are retained, but no cluster UI | Keep clusters separate from code points and glyphs |
| Prosody / recursive grammar | unsupported | schema-2 execution rejects grammar/history; no typed prosody executor | Later priorities; no misleading controls |
| Recursive typed feature structures | persisted-only | `schema.go`, flat grammar validator rejects recursive execution | Versioned recursive executor remains future work |
| Windows/Linux replay, SQL runtime | unverified | Historical A1 scripts/results read; not rerun at baseline | Rerun current build after each delivered slice |

## Baseline commands actually executed

Windows amd64, Go 1.27.1, Node 24.21.0, npm 11.19.0.

| Command | Observed result |
| --- | --- |
| `git status --short`; `git rev-parse HEAD`; `git branch --show-current`; `git log -6 --oneline` | Clean, revision/branch above |
| `.tools/go/bin/go.exe test -count=1 ./internal/languages/... ./internal/history ./internal/storage` | Pass: product 1.417s, spike 1.254s, history 0.909s, storage 4.697s |
| `.tools/go/bin/go.exe test ./internal/httpapi -run TestLanguage` | Pass (Go cache) |
| `.tools/go/bin/go.exe build ./...` | Pass, exit 0 |
| `node --test tests/languages.test.mjs` | 5 pass, zero failures/skips |
| `npm.cmd run build` | Pass, 21.88s |
| `.tools/go/bin/go.exe test ./internal/languages -run '^$' -bench '^BenchmarkProductiveA1$' -benchtime=3x -count=5 -benchmem` | Pass; varied 16/128/1,000 corpus; 1,000 inverse repeatedly stops at work cap with zero partial results and 1,384 examined cells |

Logs: ignored `.cache/languages/brief3/baseline-*`. Benchmark allocations are
total allocations, not measured peak heap. The prior retired `internal/legacyterrain`
ten-minute timeout and old generic container smoke-route mismatch remain unrelated
baseline failures; they were not rerun or suppressed here. Existing repository UI
formatting failures in the two Biosphere files remain unrelated too.

## First increment: formal semantics and compatibility decision

A1 inverse returns counts/traces for unrelated rejected forms. An indexed search
cannot skip those paths and claim byte-identical full results. Preserve schema 1
and schema 2 exactly. Add explicit opt-in **language schema 3**, `languages/3`,
`finite-compose/2`, `languages-compiler/3`. Forward composition rules stay the same;
inverse diagnostics describe the searched candidate domain and explicit pruning.
There is no automatic migration or change to old golden files.

The derived survivor trie indexes each compatible lexical cell's ordered underlying
tokens (or its explicit suppletive override), omitting tokens any authored consuming
sound rule could change/delete. Every remaining token must survive in order in a
successful surface form. Query subsequence traversal prunes only impossible cells;
the ordinary bounded composer then validates all selected cells and constructs
actual traces. Homophones, cells and allomorph branches remain distinct. Epsilon
signatures attach to the root and are never discarded. Written pruning uses NFD
base-code-point subsequences of explicit pinned mappings, omitting marks to remain
conservative under cross-chunk canonical normalization; exact written equality is
still checked by the composer. This is an index, not general inverse phonology.

Index identity includes the canonical revision (therefore lexical/rule data and
dependency pins), schema, compiler, engine, adapter and index version. Request
identity also includes the complete canonical analysis input/policy. Construction
and traversal are deterministic/cancellable and bounded. Work/trace limits are
not raised. No partial analysis set is selected after an execution limit.

Next, a separate opt-in **Script schema 2** will embed typed normalized vector
geometry, stable glyph/sign IDs, explicit role/reading associations and structured
sequences. Script schema 1 remains exact. SVG is converted to this inert geometry,
never rendered as imported markup. Authored geometry stays in canonical immutable
bodies and World ZIPs; SVG output and indexes remain derived. A new World archive
schema **4** gates either new profile, including retained/pinned dependencies.
Old language/script revisions are not migrated. Sequence spelling, vertical layout,
prosody, recursive grammar and installable font export remain unsupported until
separately implemented and verified.

## Delivered increments

Audit commit: `a563840`. A2 implementation: `f4fe6e0`. Script slice: `02611b3`.
The separate Script slice
adds the opt-in schema and real editor/API/storage path documented in
[CUSTOM_SCRIPTS.md](CUSTOM_SCRIPTS.md). This section updates the baseline above;
it does not turn historical evidence into checks performed against this checkout.

A2 uses a derived necessary-subsequence index and the existing productive composer.
It does not raise the execution cap or pre-expand every surface form. The varied
1,000-entry regular request (the current authored lexeme limit) completes with one valid analysis and 17 examined
composition cells; the suppletion request returns all 53 analyses. Five warm
samples have medians 2.377 ms and 4.767 ms respectively. Cold compile+request has
median 29.340 ms. A1 retains its original explicit limit and old goldens. Inputs,
work/stopping metrics, limits and min/median/max timings are in
[INDEXED_A2.md](INDEXED_A2.md). Fixtures use fixed counters, not process randomness;
queries use all-candidates policy, not seeded sampling. Allocation totals are
measured; peak heap/RSS and production-scale performance remain unverified.
Adversarial all-mutable signatures still reach a clear cap without partial output.

The Script browser workflow draws artwork, edits numeric points, uses undo/redo,
assigns stable glyph identity and explicit readings/roles, composes a sequence,
previews, saves/compiles/activates, reloads, and renders N after N+1 changes.
Another flow rejects active SVG, reviews normalized SVG, preserves glyph identity,
renders mixed/silent roles, handles ambiguity, saves RTL and replays the complete
result after HTTP ZIP import. Artwork is source; SVG is derived. Original SVG is
never mounted or fetched. Profiles do not imply automatic historical-script shaping.

Correctness fixes found during delivery: imported metrics must scale from their
own viewBox; group-depth checks must not count an empty child array as a level;
revision hashes need wrapping on narrow screens; rendering must leave editing
enabled so another sequence can cancel/supersede it. Move-only paths are diagnosed
instead of advertised as completed artwork. Fixtures/browser checks cover these.
Old goldens were not regenerated to conceal changes.

## Current commands and actual results

PowerShell, repository root, `CGO_ENABLED=0`. Windows Go 1.27.1 amd64,
Node 24.21.0, npm 11.19.0, Docker 29.8.2; Linux engine Go 1.27.3 amd64.
Tests use temporary databases/Worlds, explicit browser opt-in and disposable
containers. The author's active database/server was not used for these tests.

| Command / evidence | Observed result |
| --- | --- |
| `.tools/go/bin/go.exe test -count=1 ./internal/languages/... ./internal/history ./internal/storage` | Pass: product 3.794s, spike 1.512s, history 1.149s, storage 6.624s; full packages |
| `.tools/go/bin/go.exe test -count=1 ./internal/httpapi -run '^TestLanguage'` | Pass, 11.836s; old/new language and Script lifecycle, permissions and compute-gate tests |
| `.tools/go/bin/go.exe vet ./internal/languages/... ./internal/history ./internal/storage ./internal/httpapi ./cmd/languages`; `.tools/go/bin/go.exe build ./...` | Pass |
| `npm.cmd test` | 143 pass, zero failures/skips, 133.927s; includes disposable terrain/project restart |
| `npm.cmd run build` | Pass, final React build 11.90s |
| `npx.cmd prettier --check` on changed JS/JSX/CSS/config; `node scripts/check-ui.mjs` | Pass; shared boundaries and compatibility checks pass |
| `npm.cmd run check:ui` | Fails existing formatting in `Biosphere.jsx` and `BiosphereEditors.jsx` only; unchanged |
| `$env:KRIEMHILD_LANGUAGES_TEST='1'; $env:KRIEMHILD_UI_URL='http://127.0.0.1:8137'; npx.cmd playwright test tests/languages.browser.spec.mjs tests/scripts.browser.spec.mjs` | 13 pass, 40.6s; lifecycle, A2, drawing, import, old revision, ZIP, 390 px, pagination, keyboard and stale preview |
| Same browser environment; `npx.cmd playwright test tests/languages.browser.spec.mjs --grep 'indexed inverse profile'` | Pass; also verifies wizard creates an empty schema-3 draft without invented entries/plans |
| `docker build -f docs/languages/Dockerfile.engine -t kriemhild-languages:brief3 .` | Pass: current Linux tests, vet and CGO-free engine build |
| `.tools/go/bin/go.exe build -trimpath -o .cache/languages/brief3/cli.exe ./cmd/languages`; `node scripts/languages-a1-replay.mjs kriemhild-languages:brief3 .cache/languages/brief3/cli.exe` | 14 cases, two separate-process repeats/platform, byte-identical UTF-8 output and full old/new goldens |
| `$env:GOOS='linux'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'; .tools/go/bin/go.exe build -trimpath -o .cache/languages/brief3/runtime-final/kriemhild ./cmd/kriemhild`; copy dist; `docker build -f docs/languages/Dockerfile.runtime -t kriemhild-languages-runtime:brief3 .cache/languages/brief3/runtime-final` | Pass; non-root Linux runtime |
| `node scripts/languages-container-smoke.mjs kriemhild-languages-runtime:brief3` | SQLite and PostgreSQL 17 pass create/compile/activate, pinned execution, process restart, real ZIP export/import into another World and full replay; temporary containers/volumes/network removed |

Ignored evidence: `.cache/languages/brief3/{go-final,http-final,node-final,
browser-final,browser-indexed-wizard,vector-build,docker-engine,docker-runtime,
platform-replay,container-smoke}.log`; replay hashes in
`.cache/languages/a1-platform-replay.json`; SQL reports in
`.cache/languages/container-smoke.json`. Benchmark logs: `baseline-bench.log`
and `a2-bench.log`. These are actual executions, not fixture output substituted
for commands. Benchmarks and replay omit timing/cache flags from deterministic
linguistic results.

Final engine image: `sha256:73e1dcef4fdf2f06185f34b781b8ae1682740902f6b470b58a61d61c612b316e`.
Final runtime image: `sha256:662a19750e6db214d1a61900f673292252a8a00986f233ed24e8e86a28f4d38d`.
Both contain the source behavior committed in `02611b3`. Final SQL rerun reached
World revision 16 on both drivers, exported 10,846/10,840 bytes respectively and
replayed every supported fixture before/after restart and after import. Vector
render replay SHA-256 is
`85a6ce92916919251c0980f9ac5558a9bdd9ec0ae7f2238eeac0108929606182`;
indexed inverse is
`38affc84341308b68398b6c3255be4aa8ec3ecf1a571e376c930157c4fe1a18a`.
These hashes cover complete CLI output bytes, including request, traces and SVG.

Initial failures were corrected and rerun: the new golden comparator incorrectly
compared struct and generic-map field ordering (fixed comparator, kept goldens);
a browser locator used `Find glyph` instead of `Search glyphs`; narrow output
overflow and disabling editing during render exposed the real UI defects above.
No unrelated tests were weakened. Historical retired `internal/legacyterrain`
ten-minute timeout and generic container smoke-route mismatch were not rerun here
and remain unresolved baseline failures. The focused runtime tests current routes;
it does not make the old smoke test green or claim the unrestricted suite passes.

## Acceptance completion matrix

| Scenario | Status | Exact implementation / verification |
| --- | --- | --- |
| A: indexed inverse | working | `inverse_index.go`, `inverse_index_test.go`; product tests, repeated `BenchmarkProductiveA2`, HTTP/browser, replay and SQL restart/import above. Homophones, same-surface derivations, zero/epsilon, normalization and explicit limits tested. Bounded domain, not general inverse parsing. |
| B: drawn alphabet | working | `GlyphWorkbench.jsx`, `vector_render.go`; `tests/scripts.browser.spec.mjs` drawing/publish/reload/old revision, 390 px and keyboard scenarios. |
| C: SVG import | working | `vector_import.go`, `TestSafeSVGImportAndNormalization`, full golden, API and browser import/ZIP flow. Malformed/duplicate/extreme/nesting/command/byte/active-resource cases rejected. Passive subset only. |
| D: mixed signs | working | `vector-script.json`, full golden and browser/runtime replay: syllabic + logographic/semantic + silent classifier; roles/readings remain separate from geometry. Horizontal grouping only. |
| E: ambiguous reading | working | `TestVectorReadingsAndExplicitChoices`, browser choice-required then explicit selection; incompatible/missing readings/glyphs explicit, no guessing. |
| F: immutable revisions | working | `TestLanguageVectorScriptImmutableRestartAndZIP`, A2 retained-version storage test, browser N/N+1 comparison, SQL restart/import. |
| G: representation separation | partial | A1/A2 typed phonological/boundary traces preserved; Script traces retain independent glyph/sign/reading/role/Unicode fields. Sequence spelling/prosodic execution unsupported. |
| H: recursive grammar | unsupported | A1/A2 readiness rejects grammar execution; original flat schema-1 subset remains. No recursive parser/generator claims or controls added. |

## Final gap matrix

| Capability | Status | Evidence / next increment |
| --- | --- | --- |
| Strict source, compilation/readiness, bounded execution and traces | working | Old goldens retained; full A2 golden and exhaustive-domain comparisons. No migration. |
| Derived index and invalidation | working | Compiler/adapter/schema/revision/dependency identity; complete request policy/seed hash. Cold/warm/rebuilt equality and restart/import. Storage budgets are not measured heap. |
| General phonological inversion | partial | All-mutable signatures can still hit the explicit cap. Measure harder authored corpora before selecting another backend. |
| Glyph editor, passive SVG, LTR/RTL | working | Backend/API/UI/publication plus browser/SQL replay above. Bounded vectors, not universal script/font support. |
| Profile taxonomy and linguistic/semantic annotations | persisted-only | Authored intent/claims retained; no automatic abugida shaping, semantic interpretation or lexical reference resolution. |
| Explicit readings/contexts/classifiers/groups | working | Compatible selection and silence/layout execute with typed provenance; ambiguity requires choice. |
| Complete Unicode association | persisted-only | Strings retained in source/output independently of glyph identity. Grapheme segmentation/cursor/count operations unsupported. |
| Script-1 mapping and normalization | working | Existing Unicode pipeline/pins/goldens unchanged; artwork does not replace it. |
| Sequence orthography and inverse spelling | unsupported | Next priority: versioned ordered token/grapheme/sign rules, precedence, termination and inverse policy; execution/traces/UI/portability afterward. |
| Vertical layout / 2D quadrats / installable fonts | unsupported | Reported by capabilities and UI; require separate bounded representations/renderers. |
| Prosody / recursive grammar execution | unsupported | Later priorities, no fake controls. Schema-1 recursive feature metadata remains persisted-only. |
| Production scale / peak heap | unverified | Measured 16/128/1,000 fixtures and allocation totals; broader authored corpora and peak instrumentation remain. |
| Native backend comparison | unverified | None adopted. Owned adapter adequate for the measured slice; no licensing/deployment approval inferred. |

World archive **4** guards retained language-3 and Script-2 bodies. Canonicalizer
stays `kriemhild-typed-json/1`; source schema/compiler/engine/adapter are separately
pinned. Source hashes cover authored arrays, geometry, readings, rules and pinned
dependencies. Artifact identity includes relevant versions; SVG/indexes remain
derived. Permissions, CSRF, CAS, blob lifecycle, Age isolation and source retention
reuse existing paths. No AI, remote linguistic service, native dependency, terrain
grid changes or author-data tests were introduced.
