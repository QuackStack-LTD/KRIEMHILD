# Brief 5: sequence corpus audit

2026-10-28, Windows amd64, branch `rebuild/continuous-terrain`, clean starting
HEAD `3a82adf190ff0931db7d8b7ac7ee1ad238e210a3`. Recent commits: `3a82adf`
delivery record, `54e8e47` artifact ownership, `8a0772e` retained NFC replay,
`35d1151` normalization trace, `0b8b0fe` workbench. No AGENTS.md was found in
the repository or ancestor chain. Go 1.27.1, Node 24.21.0; CGO disabled.

| Capability | Status | Current evidence / next increment |
| --- | --- | --- |
| Language schemas 1–4, A1/A2, Script 1/2 | working | Fresh `go test -count=1 ./internal/languages/...`; complete old goldens retained |
| Sequence forward, inverse/A2, Unicode, renderer | working | `sequence_*test.go`, `graphemes_test.go`, production tests rerun |
| Immutable revisions, dependency ownership, archive 4/5 | working | Fresh history/storage tests, including `languages_sequence_test.go` |
| Sequence API and final authorization | working | Fresh `go test -count=1 ./internal/httpapi -run '^TestLanguage'` |
| Candidate-only corpus | working | `corpus.go`; sequence requests explicitly rejected with `unsupported_case` |
| Structured sequence corpus | unsupported | No typed expectation/runner; add standalone verification format, without changing Definition.Tests |
| Human review of new corpus expectations | unverified | Existing full-result goldens can be wrapped without regeneration; author review requested explicitly |
| Fresh browser/platform/database parity | unverified | Existing Brief 4 evidence is historical until rerun below |
| Prosody, recursive grammar, general inversion, font export | unsupported | Outside Brief 5; no new controls or semantics |

Baseline executions before editing application logic:

- `CGO_ENABLED=0 .tools/go/bin/go.exe test -count=1 ./internal/languages/... ./internal/history ./internal/storage`: pass; languages 11.018s, spike 1.884s, history 1.327s, storage 7.546s.
- `.tools/go/bin/go.exe test -count=1 ./internal/httpapi -run '^TestLanguage'`: pass, 6.608s.
- `npm.cmd run build`: pass, 26.02s.
- `npm.cmd test`: 143 pass, 98.740s.

The author process/database on port 8124 is excluded from all work. Temporary
Worlds, test directories, isolated servers and disposable containers are used.
The retired legacyterrain timeout, generic container smoke-route mismatch and
Biosphere formatting failures remain unrelated boundaries; no tests are weakened.

## First increment and version implications

Implement `sequence-corpus/1` as external verification data referencing immutable
language/Script hashes, with exact typed results, strict whole-file validation,
explicit review provenance and production-engine execution. Keep old candidate
corpus behavior, language/compiler/adapter/canonicalizer/renderer/archive versions
and old golden bytes unchanged. See [SEQUENCE_CORPUS.md](SEQUENCE_CORPUS.md).

## Baseline completed

These were rerun against the starting implementation before corpus integration:

| Command | Observed result |
| --- | --- |
| `node scripts/languages-a1-replay.mjs kriemhild-languages:brief4 .cache/languages/brief5/baseline-cli.exe` | 19 complete cases, twice/platform, Windows/Linux byte identity and old goldens pass; rebuilt Windows CLI, historical Linux image has the same implementation as starting HEAD |
| `node scripts/languages-container-smoke.mjs kriemhild-languages-runtime:brief4` | Fresh SQLite/PostgreSQL create/compile/activate/execute, process restart, HTTP ZIP import to different World and old-pin replay pass |
| `KRIEMHILD_LANGUAGES_TEST=1 KRIEMHILD_UI_URL=http://127.0.0.1:8138 npx playwright test tests/languages.browser.spec.mjs tests/scripts.browser.spec.mjs tests/sequence.browser.spec.mjs` | 16 pass, 58.4s; disposable test Worlds deleted |
| `npm.cmd run check:ui` | Existing Biosphere formatting errors; the concurrent first run also saw the briefly unformatted Illustrator edit, subsequently formatted and rechecked below |

Native T3 preview status/open succeeded. Native inspection and repository browser
tests use the isolated previews, never author port 8124. Logs are ignored under
`.cache/languages/brief5/`; historical audits remain historical, not rerun claims.

## Delivered implementation

- `23317f4` audit/spec; `f44aeda` active Illustrator mode; `64d6b88` strict format,
  complete result comparison, read-only runner and reviewed corpus; `8d2eca1`
  CLI/API and retained revision/restart/archive integration; `9f83cd1` input/pin/
  work hardening and persistent mode instruction; `eb40802` CLI JSON/exit checks;
  `dfcad40` incomplete-success rejection and honest outcome classification;
  `6a805d0` reuses the production 16 MiB LRU reservation budget inside a run.
- Schema `sequence-corpus/1` is external verification data. Old language schemas
  1–4, Script 1/2, source hashes, compiler/adapter/renderer versions, canonicalizer,
  A1/A2 algorithm and archive 4/5 paths are unchanged. No migrations or AI calls.
- Author explicitly approved reuse of four existing full-result expectations.
  `sequence.corpus.json` embeds their exact typed values; the unit test compares
  them to retained files. No existing golden or linguistic rule was regenerated.
- Production Compile/Execute/RenderScript are called through the existing engine.
  The runner has no alternate spelling implementation. It snapshots source and
  validates every case/pin before execution; no active-head lookup, publication,
  compile persistence or activation. CLI resolver is explicit files; API resolver
  is exact retained source under the selected World/Age ownership boundary.
- JSON and complete typed expectation validation rejects unknown/duplicate fields,
  UTF-8/surrogate errors, missing/contradictory pins, malformed named spans,
  boundary topology, impossible success/cap shapes, missing stages, false domain
  claims and size/work bounds. Case diffs identify fields, not just final strings.
- Seven raw streams over `{p,a}`, lengths 0–2, including empty, run forward then
  eligible inverse: all original typed preimages recovered. This is a bounded
  membership property, not proof of general inverse phonology or an independently
  enumerated universal candidate set. Primary inverse golden includes two distinct
  rock/stone A2 analyses with complete derivations.
- API verification is view-only under existing CSRF/access controls, bounded jobs
  outside the global gate and final authorization/retained-pin recheck. Tests
  revoke membership after initial access; no computed private corpus is disclosed.
- Per-run derived artifacts are reused by exact source revision under the existing
  16 MiB LRU reservation budget, then discarded. Oversized artifacts are not retained.
  No persistent corpus cache or second source store. Cold/rebuilt, reordered and
  caller-mutated result tests preserve pinned output. Runtime archive/restart
  replay rebuilds artifacts from retained canonical bodies.

## Delivery commands and actual results

`CGO_ENABLED=0`, local `.tools/go/bin/go.exe` for all Go commands.

| Command | Actual result |
| --- | --- |
| `go test -count=1 ./internal/languages/... ./internal/history ./internal/storage` | Pass; latest combined run languages 8.559s, spike 2.314s, history 1.572s, storage 5.122s; final additional strict-shape test rerun passes |
| `go test -count=1 ./internal/httpapi -run '^TestLanguage'` | Pass, 7.405s; includes corpus permissions, read-only verification and membership revocation |
| `go vet ./internal/languages/... ./internal/history ./internal/storage ./internal/httpapi ./cmd/languages`; `go build ./...` | Pass |
| `npm.cmd test` | Baseline 143 pass. First delivery run 142 pass / 1 terrain restart readiness failure; isolated test then passed and complete repeat passed 143, 32.833s; details below |
| `npm.cmd run build` | Pass, final production UI built for isolated server and runtime |
| `node scripts/check-ui.mjs`; changed-file `prettier --check`; `git diff --check` | Pass |
| `npm.cmd run check:ui` | Fails only unchanged Biosphere.jsx/BiosphereEditors.jsx formatting on final run; those files were not edited |
| `node scripts/languages-corpus-cli-check.mjs .cache/languages/brief5/cli.exe` | 10 scenarios pass: validation/execution, mismatch, budget, pending review, contradictory pin, unknown/duplicate field, malformed UTF-8 and oversized input; exit 0/1/2/3 checked |
| Browser opt-in on isolated `http://127.0.0.1:8139`, same three Languages suites | 17 pass, 59.1s, including new corpus replay after newer activation; test Worlds deleted |
| `KRIEMHILD_ILLUSTRATOR_TEST_PROJECT=<disposable> npx playwright test tests/illustrator.browser.spec.mjs -g 'selection mode stays'` | 1 pass, 8.1s; highlighted state, instruction with hidden panel, cancellation verified |
| `docker build -f docs/languages/Dockerfile.engine -t kriemhild-languages:brief5 .` | Linux CGO-free Languages tests/vet/CLI build pass; image includes new corpus tests |
| `node scripts/languages-a1-replay.mjs kriemhild-languages:brief5 .cache/languages/brief5/cli.exe` | 20 complete cases including corpus, twice in separate Windows/Linux processes; identical bytes and all retained goldens |
| Linux app cross-build, runtime image, `node scripts/languages-container-smoke.mjs kriemhild-languages-runtime:brief5` | Fresh SQLite/PostgreSQL 17 create/compile/activate/execute, process restart, real HTTP ZIP import into a different World, full corpus replay pass; temporary containers/networks/volumes removed |

Intermediate strict-expectation testing exposed a test wrapper that updated only
one of the two production request fields, causing preflight rejection; its test
failure message then indexed an absent case. Both wrapper fields and the test
error check were corrected. The production runner did not execute partial data.
An isolated mode-project setup script initially used a wrong relative import;
corrected before the successful browser run. Neither issue was hidden by selection.

The retired `internal/legacyterrain` timeout and old generic container smoke-route
mismatch were not rerun or changed in this brief. They remain historical known
failures, not newly reproduced failures or passing checks. No full-repository
test-green claim. No author World/database or port 8124 process was touched;
its original PID 9596/executable remained running during verification.

One delivery `npm test` run had 142 pass / 1 fail: unchanged
`tests/project-restart.test.mjs` reported `Test server did not become ready`
at its third disposable startup (line 177), with no startup stderr. It ran while
Go builds/container checks were active. `node --test tests/project-restart.test.mjs`
then passed alone (31.867s, total 32.040s), without source/test/timeout changes.
This suggests timing sensitivity, not a demonstrated linguistic regression;
the failed run remains recorded. A complete `npm.cmd test` repeat then passed
all 143 tests in 32.833s (`final-node-repeat.log`), with the same source/selection.

Final verified implementation is `6a805d0` (subsequent changes are documentation).
Linux engine image `sha256:4998d28e5e6f8db8f2e1c69971a463fac1b457b29d4588c0a6b9a38ffbd0a0e6`;
runtime image `sha256:48f9e5d07b1193b2a13cea932c0539d573a1dfce726a7752fe6c6950975fa430`.
Corpus CLI result: 119,208 UTF-8 bytes,
SHA-256 `a16f11aa8c6d1c1822d9b34ff1d6952892e7c5cebf960d2b63d3b8fc43dbfef9`,
identical on Windows/Linux in two separate processes each. Final SQLite and
PostgreSQL runtime checks were rebuilt/rerun after the LRU bound change.
The isolated preview is `http://127.0.0.1:8139`, using the verified binary/dist
snapshot and `.cache/languages/brief5/data`; author port 8124 is unchanged.

## Corpus benchmark (separate from spelling benchmark)

Command: `go test ./internal/languages -run '^$' -bench '^BenchmarkSequenceCorpus$'
-benchtime=3x -count=5 -benchmem`. Windows amd64 Go 1.27.1, benchmark GOMAXPROCS=4,
CGO disabled. Five repeated samples/mode, three complete runs/sample; no random
seed. Four unique approved expectations, two language revisions and one pinned
Script. Corpus file 129,840 bytes; canonical typed corpus 45,212 bytes. Each run
has 18 production calls, seven bounded domain inputs, 14,800,000 charged work
units; primary outcomes three success and one ambiguous; stopping reason completed.

| Condition | Min / median / max sample ns/op converted to ms | Median B/op | Median allocs/op |
| --- | --- | --- | --- |
| Warm transport snapshots; derived artifacts still recompiled each run | 53.243 / 59.149 / 75.599 | 8,123,725 | 101,047 |
| Cold fixture/source file read and strict decoding included | 49.504 / 56.899 / 72.778 | 9,381,581 | 114,202 |

These are distributions of five benchmark sample means, not individual-request
percentiles or an exclusive-machine SLA. Earlier concurrently loaded samples
were slower (up to 369.767ms); both logs are retained. Allocation totals are
neither retained nor peak heap/RSS. Peak memory is unverified. No fixed engine
limit was raised. This is a small verification corpus, not a large-language
scale claim or a benchmark comparison against a native backend.

## Acceptance / remaining capability matrix

| Capability | Status | Evidence / exact next step |
| --- | --- | --- |
| Typed corpus, strict decoding, fixed versions/pins | working | `sequence_corpus.go`, strict decode/preflight tests and CLI checks |
| Validation-only without engine execution/writes | working | Zero calls; API test asserts World revision and compile/active state unchanged |
| Production execution, complete typed traces/Unicode/sign/SVG comparison | working | Four approved full-result expectations; tests and 20-case replay |
| JSON and human reports, outcomes, field diffs, exit codes | working | `CorpusSummary`, recursive diffs, CLI 10-scenario check |
| Review provenance and golden protection | working | Explicit author approval; pending review cannot produce a passing execution |
| Forward/Unicode/NFC/pinned Script golden coverage | working | Retained four corpus cases include decomposed marks, supplementary/emoji/ZWJ/flag/modifier clusters, explicit NFC, vector signs/readings and old pins |
| Complete inverse+A2 homophone derivations | working | Reviewed inverse golden retains both analyses; strict complete-result comparison |
| Boundaries, contexts, priority, expansion/deletion/NFD, missing readings/signs | partial | Existing production sequence unit tests rerun; not all have dedicated author-reviewed external corpus cases. Next: review and capture those existing controlled scenarios without inventing goldens |
| Corpus malformed input, pins, limits, inner cancellation | working | Strict/span/topology/status/oversize tests; low A2 request cap returns explicit limit with no partial preimages |
| Inverse inner enumeration cancellation and trace/output bounds | working | Existing `sequence_inverse_test.go` rerun plus new runner cancellation/report/call/work caps |
| Bounded exhaustive recovery | working | Seven streams `{p,a}` length 0–2; no lossy stages/A2 or general-reversibility claim |
| Exact pre-erasure boundary/A2 corpus fixture breadth | partial | Existing boundary production test passes; current external inverse golden includes derivation boundary provenance, not a dedicated `surface-boundaries` external case |
| Immutable old revisions, cache independence and caller ownership | working | Reordered/rebuilt/caller-mutation tests, storage restart and newer active revision, runtime imported-World equality |
| Archive 4 barrier, archive 5 and old archives | working | History/storage tests and real HTTP ZIP replay; no archive code/schema changes |
| Windows/Linux structured result bytes | working | 20 cases twice/platform, including corpus/Unicode/old goldens |
| SQLite/PostgreSQL restart/import parity | working | Fresh disposable runtime checks against current implementation |
| End-user corpus editor/capture UI | unsupported | CLI/API are executable; full authoring platform is out of scope. Existing spelling/glyph UI remains functional |
| Corpus peak heap/RSS and large corpus scaling | unverified | Only bounded four-case corpus measured; add dedicated profiling before scale claims |
| Prosody, recursive grammar, general inverse phonology, vertical/fonts/automatic semantics | unsupported | No implementation or misleading controls added |

The smallest coherent Brief 5 runner and initial reviewed corpus are delivered.
The complete requested linguistic coverage matrix is **not** claimed finished:
dedicated reviewed context/boundary/NFD/lossy/error corpus expectations are the
next increment. Existing executable engine tests supply regression coverage in
the meantime; schema fields or UI controls are not counted as that evidence.
