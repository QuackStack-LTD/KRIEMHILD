# Languages continuation audit

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
