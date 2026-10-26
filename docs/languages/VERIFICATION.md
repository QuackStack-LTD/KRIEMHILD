# Languages implementation verification

Current Brief 3 evidence is in [BRIEF_3_AUDIT.md](BRIEF_3_AUDIT.md), including
starting revision, current commands/results, acceptance matrix and explicit gaps.
[A2](INDEXED_A2.md) completes the varied 1,000-entry inverse through a derived
index. [Custom Scripts](CUSTOM_SCRIPTS.md) have real drawing/import/composition,
publication, deterministic rendering and restart/ZIP replay. Current checks
include 143 Node tests, 13 browser flows, 14 repeated Windows/Linux replay cases
and fresh SQLite/PostgreSQL runtime tests. Generic smoke-route, retired-terrain
and Biosphere-formatting limitations remain visible in the audit. Older sections
below are historical evidence, not claims about support for new schemas.

The author explicitly approved the architectural model after Phase 0, committed
as `b64058229d6b456ff5ce892940e89c9d9d5703e2`. This additive integration implements
the supported initial engine described in [README.md](README.md); it does not
claim every advanced roadmap feature is complete.

## Recorded checks

- **27 active Go packages passed**, excluding only the previously documented
  retired `internal/legacyterrain` package. `go build ./...` and affected Go vet
  checks passed. The retired full-suite timeout remains a baseline failure; no
  test or production Docker gate was removed or weakened.
- **131 Node tests passed**, including draft invariants, metadata isolation and
  language/script changes in Age/merge review. Production React and shared UI
  boundary checks passed.
- **Five real browser tests passed** on an isolated database: local Age/name
  selection survives saves without changing another Age; historical
  preview/approval/apply; stale response after selecting another definition;
  incomplete creation/save/reload, invalid UTF-8 file rejection, dirty navigation and 390 px layout; script,
  compile, inflection, parsing, trace/corpus inspection and stale revision state.
  The suite deletes its disposable Worlds afterward.
- Storage/API tests cover immutable/canonical bodies, Unicode, retained revisions,
  World/head conflicts, Age inheritance, profile tamper rejection, script pinning,
  portable/corrupt bundles, historical approval replay/source preservation,
  view/edit/CSRF boundaries, session/membership revocation and releasing the global
  access gate during computation.
- Linux engine Docker tests/vet/build passed. Raw UTF-8 Windows/Linux replay
  comparisons cover compile, inflect, seeded generation, parsing, corpus and
  historical preview. Final comparisons are repeated after the independent
  writing dependency fix.
- Real **non-root Linux app runtimes with SQLite and PostgreSQL passed**:
  frontend, scripts, save/compile/activate, deterministic operations, historical
  apply, source preservation, restart, ZIP export, ZIP import into another World
  and replay. Temporary containers, volumes and network are removed afterward.
- Language-bearing World ZIPs use schema 2 to reject older readers that would
  discard new state. Schema 1 Worlds without languages remain compatible.

Local logs/artifacts are in ignored `.cache/languages/`: active Go/Node tests,
focused tests, browser logs/screenshots, builds, `product-cross-platform.json`,
`container-smoke.json` and benchmarks. These are real executions on disposable
data, not generated evidence or saved author Worlds.

## Reference and performance

Public fixture revision:
`9d5c2c6d141549aac64bc61da9ca7c9ef37cea3cfd876a8f1a728be23994d778`.
Independent script revision:
`71c608c5aba1c9848539bbe4138c8f0e8c3a937e42d1e23c49589da3ba5bce4e`.

Regular `stone` + `number=many`: `pat → pata → pada → pad̪a`.
Irregular `person` plural: `nin`. Parsing `pad̪a / nin` retains two homophonic
left-role analyses and enforces agreement. Historical `t → d` retains a reviewed
`person → nin` exception. Goldens record complete requests/intermediates/rule IDs.

Single-iteration Windows measurements (Intel i5-6300HQ, Go 1.27.1) initially gave
approximately **2.6 ms** for the product fixture and **104 ms / 30.5 MB allocated**
for a validated 1,000-lexeme / 2,000-form compile. These are allocation totals,
not peak heap, stable statistics or large-language scalability claims. The large
benchmark asserts readiness and 2,000 actual forms; it cannot benchmark an invalid
duplicate-sense inventory unnoticed. These final measurements include the
independent writing dependency artifact.

## Baseline limits

Implementation/browser/container checks used separate databases and preserved
the original active author's instance/data. The unchanged full production Docker
suite is not claimed green: retired-terrain tests retain their recorded timeout,
and the old generic container smoke script still targets retired terrain routes.
The Languages runner exercises current language/World routes without modifying
that unrelated baseline.

Native HFST/foma selection remains open. No native FST tool, model service or AI
dependency was added. Recursive grammar, productive advanced morphology and
custom glyph layout remain clearly distinguished from working operations.

## Continuation A1 evidence

Baseline source `6aed8ff`; audit/semantics commit `a009b1b`; independent banner
commit `62d4c62`. A1 adds the schema-2 adapter described in PRODUCTIVE_A1, preserving
the schema-1 body hash and original full-result goldens. Its fixture revision is
`8492e7dc99f30a1d4bab66c3cb8960c4d744426719731b629fb58d0fe0038f04`.
The new inflection golden contains `i + pat + a → i + pad + a → ipada` with actual
typed boundaries, ordered epsilon confirmation, rule IDs and preserved underlying
form. Inverse `pada` retains stone/rock analyses and rejected nonmatching surfaces.

Commands actually exercised on disposable data during continuation:

```powershell
.tools/go/bin/go.exe test ./internal/languages/... ./internal/history ./internal/storage
.tools/go/bin/go.exe test ./internal/httpapi -run TestLanguage
.tools/go/bin/go.exe vet ./internal/languages/... ./internal/history ./internal/storage ./internal/httpapi ./cmd/languages
.tools/go/bin/go.exe build ./...
npm.cmd test
npm.cmd run build
node scripts/check-ui.mjs
$env:KRIEMHILD_LANGUAGES_TEST='1'
$env:KRIEMHILD_UI_URL='http://127.0.0.1:8137'
npx.cmd playwright test tests/languages.browser.spec.mjs
docker build -f docs/languages/Dockerfile.engine -t kriemhild-languages:a1 .
.tools/go/bin/go.exe build -trimpath -o .cache/languages/a1-cli.exe ./cmd/languages
node scripts/languages-a1-replay.mjs kriemhild-languages:a1
node scripts/languages-container-smoke.mjs kriemhild-languages-runtime:a1
```

Go suites, vet and build passed. **132 Node tests passed**, production frontend
build passed, changed-file Prettier checks and shared UI boundaries passed.
**Eight browser tests passed** on an isolated database, including the five legacy
flows, actual productive generation/typed trace capture/save/corpus execution,
empty schema-2 wizard creation, 27-cell pagination/search, keyboard use, 390 px
editing, and banner navigation/height at desktop and mobile widths. The suite
deletes its own Worlds. Existing retained sources, author Worlds and active data
are not fixtures.

The repository-wide `npm run check:ui` also ran and **failed on pre-existing
formatting in `src/worldbuilding/Biosphere.jsx` and `BiosphereEditors.jsx`**. Those
files are unchanged from `6aed8ff`; unrelated formatting was not rewritten to
hide this failure. This is additional baseline evidence, not an all-green claim.
The retired-terrain timeout and generic old smoke-route mismatch above also remain.

The engine Docker image runs Linux Go tests/vet/build with CGO disabled. The new
replay script runs nine cases twice in separate Windows/Linux CLI processes:
legacy compile/inflect/seed/parse/corpus, productive compile/forward/inverse and
productive seeded selection. It compares raw UTF-8 bytes, complete traces and
goldens; its JSON records bytes/hashes. SQL runtime smoke now also saves/compiles/
activates schema 2, runs both productive goldens, restarts, exports a real World
ZIP, imports into another World and replays both old/new results. Non-root
SQLite and PostgreSQL runtimes passed and temporary containers/volumes are removed.
Historical schema-1 preview/apply remains exercised alongside the new adapter.

New tests also cover mixed schema-1/schema-2 immutable revisions, compiler pins,
Age inheritance/isolation, changed current script drafts with old dependency pins,
reference/body schema mismatch, corrupt/missing dependencies, and a schema-2
source retained only through a pinned dependency after its source Age disappears.
Archive schema 3 is required for those bodies; unrelated Library JSON is not
classified as a definition. Existing permissions/CSRF/session-revocation and
compute-gate tests remain in the focused HTTP suite.

Five-sample varied-inventory benchmark inputs, commands, limits and allocation
figures are in [BACKEND_DECISION_A1.md](BACKEND_DECISION_A1.md). The 1,000-entry
inverse **does not complete**: it returns an explicit work limit and no partial
choice. General native/backend comparison and peak-memory instrumentation are
still pending. The next increment is A2 indexed inverse composition.

Ignored evidence files use `.cache/languages/a1-*` (core/HTTP/Node/browser/build/
UI checks, Docker/runtime/replay, benchmarks). The clean source archive and its
verification record identify the exact implementation commit separately; Git
history and local Go SDK/cache/data directories are not source-archive contents.
