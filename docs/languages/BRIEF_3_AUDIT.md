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

Delivery results and the acceptance/gap matrix will be appended with actual
commands, rather than replacing this starting-state record.
