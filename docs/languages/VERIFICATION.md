# Languages implementation verification

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
