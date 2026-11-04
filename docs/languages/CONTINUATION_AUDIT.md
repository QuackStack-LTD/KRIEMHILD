# Languages continuation audit

The current Brief 3 audit and delivered A2/custom Script evidence are in
[BRIEF_3_AUDIT.md](BRIEF_3_AUDIT.md). The tables below preserve the historical
continuation/A1 starting states; current capability and gap matrices supersede
their inverse-index and custom-renderer limitations for explicitly opted-in new
schemas only. Existing saved definitions are not migrated.

Audited source: `6aed8ff27b2f4bd7a43734eb81a6ee6250ff1402`, branch
`rebuild/continuous-terrain`. The working tree was clean. This audit distinguishes
the product adapter from the frozen Phase 0 kernel. ARCHITECTURE and SPIKE retain
historical descriptions; README and actual source describe the integrated product.

## Evidence by capability

| Capability | Current status | Exact implementation/tests | Missing behavior | Proposed next increment |
| --- | --- | --- | --- | --- |
| Public schema/decode/canonicalization | working | `schema.go:Decode`, `spike/schema.go:DecodeJSON`; `engine_test.go`, spike strict-JSON tests | Explicit compiler identity separate from engine/adapter; new rule schema | Keep schema 1 byte stable; opt-in schema 2 |
| World/Age identity and immutable persistence | working | `history/model.go`, `storage/languages.go`; `storage/languages_test.go:TestLanguageRevisionsAgesRestartAndPortability` | No new storage mechanism needed | Retain existing CAS/blob/reference lifecycle |
| Module validation/readiness | partial | `engine.go:Compile`, `checkLexical`, `checkCorpus`; `engine_test.go` isolation tests | Machine-readable capability inventory; richer structured remedies | Versioned compiler identity and server capability report |
| Finite phonology execution | working | `spike/execute.go:phonology`, consuming snapshot rules; spike ordering/insertion/deletion goldens | No prosody, tone, stress or morpheme boundaries | Separate boundary-aware stratum |
| Morphology schema and validation | partial | `spike/schema.go:Operation`, `spike/compile.go`; `LanguageEditors.jsx:morphologySchema` | UI lists unsupported kinds without typed parameters | New typed rule model; do not reinterpret old kinds |
| Finite morphology compilation/execution | working | `spike/compile.go` expands lexical cells, `relation.go:TokenTrieBackend.Build`, `paradigm.go`; product inflection golden | Pre-expanded relations are not productive composition | Lazy ordered composition over an explicitly finite lexical domain |
| Productive allomorphy/non-concatenative forms | unsupported | Unsupported-operation tests in `engine_test.go` and spike tests | Conditions, precedence, circumfix anchors, boundary semantics | Explicit allomorph conditions and circumfix around the whole current form |
| Inverse lexical analysis | working | `spike/relation.go:Lookup,InverseText`; `lifecycle_test.go:TestSentenceAnalysisAndCorpus`, product parse golden | No productive inverse; no large-lexicon inverse index | Complete bounded inverse over the same productive forward domain |
| Request validation determinism | partial | `paradigm.go` ranges over `q.Features` when returning first unknown feature | Multiple invalid feature keys can produce differently ordered first diagnostics | Sorted validation in new adapter; retain old replay version |
| Derivation traces | partial | `spike.Trace`, product full-result goldens | No typed morpheme-boundary state or allomorph rejection provenance | Record actual typed intermediate states during execution |
| Flat grammar generation/parsing | partial | `engine.go:realize`, `parse.go`, `gloss.go`; product parse golden and agreement fixtures | Recursive AST/unification/semantics unsupported; realization/parse general round trips not established | Keep schema 1 grammar; explicitly reject unsupported new-adapter grammar |
| Recursive typed structures | persisted-only | `schema.go:Value,FeatureStructure`; compileGrammar rejects execution | Recursive realization/parser, occurs checks, roles/semantics | Later milestone D |
| Independent scripts and chunk spelling | working | `schema.go:DecodeScript`, `write.go`, pinned dependency checks; engine/lifecycle/storage/browser fixtures | Sequence spelling rules, true grapheme editing, custom rendering | Later milestone E; metadata is not font execution |
| Direction/glyph asset metadata | persisted-only | `ScriptDefinition`, `Grapheme`; structured editor | Custom glyph/font/layout engine | Do not advertise rendering support |
| Lexical annotations/provenance | persisted-only | `lexical.go` typed model and referential validation | Derivation/compounding execution, etymological inference | Keep authored provenance; no inferred claims |
| Lexicon search/import | partial | Browser structured editor pagination and strict JSON import | Server revision indexes, paginated search, CSV/TSV preview/diff | Later milestone F; 1,000-entry bound remains |
| Historical change | partial | `history.go:PreviewEvolution`; lifecycle/history HTTP tests and history golden | Single sound-change batch only; no named stages, analogy or reconstruction | Later milestone G; preserve preview/hash/approval/source immutability |
| Corpus regression tooling | partial | `corpus.go`, Laboratory capture, exact candidate/trace comparison | Selected runs, filters, exact diff UI | Later milestone H |
| Permissions/compute isolation/cancellation | working | `httpapi/languages.go`, `access.go`; release-gate/session/membership tests | New inner loops need their own cancellation and budget checks | Reuse existing admission/publication gates |
| Cache | partial | `cache.go`, lifecycle cache isolation/concurrency tests | Reservation accounting is not measured peak heap; no derived lexical index | Preserve bounds; include new compiler/adapter in new cache keys |
| Product UI | working | `Languages.jsx`, `LanguageEditors.jsx`, `LanguageHistory.jsx`; 5 opt-in browser flows in `tests/languages.browser.spec.mjs` | New rule editors/capability reporting; larger inventory accessibility evidence | Accessible structured editor plus actual server readiness |
| World ZIP portability | working | `storage/languages.go` retained-dependency collector; storage/HTTP history tests | Schema 2 definition coverage not yet present | Preserve old canonical bodies; test mixed schema revisions and fresh import |
| Windows/Linux replay and SQL runtime | unverified | Prior evidence in VERIFICATION, scripts `languages-container-smoke.mjs`, `languages-runtime-check.mjs` | Prior results have not yet been rerun during this continuation | Rerun replay/containers after implementation; do not relabel old evidence as new |
| Large-language/native backend choice | unverified | `golden_test.go` benchmarks are single-sample and duplicate forms | Repeated varied corpus, warm/cold execution, inverse worst cases, peak memory/native deployment review | Benchmark owned adapter first; no native dependency in this increment |

## Baseline commands actually rerun before implementation

Windows amd64, Go **1.27.1**, Node **24.21.0**, npm **11.19.0**.
Run from the repository root with `.tools/go/bin` on PATH:

| Command | Actual result |
| --- | --- |
| `git status --short`; `git rev-parse HEAD` | Clean; revision above |
| `go test ./internal/languages/... ./internal/history ./internal/storage` | Pass: product 1.533s, spike 0.987s, history 1.358s, storage 4.109s; spike CLI has no tests |
| `go test ./internal/httpapi -run 'TestLanguage\|TestLanguages'` | Pass, 4.824s |
| `go build ./...` | Pass, exit 0 |
| `node --test tests/languages.test.mjs` | 4 pass, 0 fail |
| `npm run build` | Pass, 15.92s |

Logs: ignored `.cache/languages/continuation-baseline-*`. These focused checks
do **not** establish an entirely green repository. The previously recorded retired
`internal/legacyterrain` ten-minute timeout and old generic container smoke-route
mismatch remain unrelated baseline failures. Neither test selection nor the
production Docker gate has been changed to hide them. They were not rerun here.
No destructive test uses the author's active database.

## Priority and compatibility decision before coding

Product blockers: productive plans currently cannot execute; boundary information
cannot survive through rules; unsupported UI choices cannot author meaningful
parameters; invalid-map diagnostics violate deterministic result expectations.
Advanced extensions: general FST composition/minimization, large-lexicon indexes,
prosody, recursive grammar, script fonts, reconstruction and native backends.

The next milestone is **A1**, a narrow opt-in productive path, not all of A–H.
Add public **language schema 2**, `languages/2`, `finite-compose/1`, and independent
`languages-compiler/2`. Schema 1, `languages/1`, `token-relation/1`, script schema 1
and all existing canonical bodies/goldens remain supported without migration.
No current definition is automatically upgraded. New schema decoding must reject
new fields in schema 1 and reject a legacy morphology module alongside productive
plans in schema 2. Historical/recursive grammar execution on schema 2 remains
explicitly unsupported in A1. World ZIP needs a new compatibility gate if an old
schema-2 archive reader could discard new definition metadata.

Formal semantics and resource policy are specified in [PRODUCTIVE_A1.md](PRODUCTIVE_A1.md).
Implementation follows this committed audit; completion/evidence will be appended
without replacing the original baseline observations.

## Delivered A1 and remaining scope

The preceding table records the original `6aed8ff` baseline, not a claim that its
unsupported capabilities were already implemented. Audit/semantics were committed
first as `a009b1b`; the independent banner change is `62d4c62`.

| Capability | Current status | Exact implementation/tests | Missing behavior | Proposed next increment |
| --- | --- | --- | --- | --- |
| Schema-2 typed productive plans and version pins | working | `productive_schema.go`, `productive_compile.go`, `schema.go`; productive validation/limit tests | No automatic schema migration, deliberately | Add mechanisms only with a version decision |
| Ordered lazy composition, conditioned allomorphy and whole-form circumfix | working | `productive_execute.go:affix,composeCell`; `TestProductiveCompositionAndCompleteInverse`, `TestProductivePrefixAndClassConditionsRoundTrip` | Not a general FST optimizer; no infix/reduplication/interdigitation | A2 indexed inverse composition |
| Boundary stratum and actual intermediate provenance | working | Typed tokens/steps; insertion/deletion/order tests; full `productive-inflect` and `productive-inverse` result goldens | One retain-until-surface stratum; no boundary-consuming or transparent strata | Separate typed stratum extensions |
| Forward/inverse for the declared finite domain | partial | Same request-time composition in both directions; complete fixture round trips, homophones and distinct epsilon derivations | The 1,000-entry benchmark hits the work cap rather than completing inverse | A2 pruning/indexing without losing analyses |
| New-adapter deterministic diagnostics, cancellation and limits | working | Sorted feature validation, `productive_limits_test.go`, live-branch/step/payload tests | Legacy engine still retains its old multiple-invalid-feature diagnostic ordering | Keep the legacy limitation explicit; do not silently reinterpret old requests |
| Corpus typed trace comparison | partial | Accepted/rejected typed derivations captured explicitly, `corpus.go`; browser capture/save/compile/run | Seeded expectations are revision-sensitive; selected cases/filter/diff UI still absent | New adapter rejects `revision_sensitive_case`; external exact seeded replay supported |
| Compiler/cache/compile-reference identity | working | `Versions`, compiler-pinned requests/ref/cache keys; storage restart tests, frontend readiness pin test | Native backend adapter version not selected | Keep dependencies rebuildable and exact |
| Persistence, mixed revisions, Age isolation and ZIP guard | working | `TestProductiveMixedRevisionsPinnedScriptRestartAndArchive`, pinned-hidden-source test, HTTP tests | No automatic corpus migration | World schema 3 for retained/pinned schema-2 bodies only |
| API/capability report/authoring UI | working | `capabilities.go`, HTTP endpoint, productive structured editor, Laboratory typed trace view; 8 isolated browser flows | Recursive grammar/history unsupported in new adapter; old engine retains its existing subset | Keep unsupported tabs explicit |
| Windows/Linux and SQLite/PostgreSQL portability | working | `scripts/languages-a1-replay.mjs`, extended real runtime smoke, restart/import replay; all full result bytes/goldens checked | No native dependency portability claim | Repeat these gates for A2 |
| Full Milestone A / backend Milestone B | partial | Narrow A1 and repeated varied benchmarks in BACKEND_DECISION_A1 | General optimized composition/inverse, process-cold/peak memory and native comparison remain | A2 first; candidate backend only if evidence requires it |

Additional correctness finding during continuation: seeded selection incorporates
the source revision. Saving a case changes that revision and can change the
expected selection. Schema 2 therefore rejects seeded corpus cases explicitly;
the UI explains using an all-candidates capture instead. Legacy behavior is
documented, not silently redefined. Neither limitation is called resolved for
every old request.

Actual continuation commands/results and portability details are appended to
[VERIFICATION.md](VERIFICATION.md). No native library, AI/model call, new service,
dependency lockfile change, source-grid change or retired-terrain test change was
introduced. C–G and the remaining H extensions are still unsupported/partial as
the original table states; this is completion of the first justified slice only.
