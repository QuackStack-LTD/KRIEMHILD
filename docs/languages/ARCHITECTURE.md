# Languages: repository audit and proposed architecture

Status: **Model approved by the author; Phase 0 prototype verified; product integration in progress.**
Audited revision: `d05c02859916f06844b1206a7bd9cb8113ab03cb`.
The approved model now has an additive product integration; its supported subset
and recorded checks are in [README.md](README.md) and
[VERIFICATION.md](VERIFICATION.md). The table below preserves the Phase 0 plan;
it does not claim every advanced capability is complete.
The supplied Languages brief requires an architecture note before broad changes
and an agreed schema proposal at the Phase 0 exit. The executable spike is
isolated from the running application and authors' databases.

## Existing architecture and integration points

| Concern           | Actual implementation                                                                                                                        | Proposed additive integration                                                                                  |
| ----------------- | -------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| Frontend          | React 19.2, Vite 7, JavaScript/JSX, no Next.js/router framework                                                                              | Lazy Languages workspace in the same shell                                                                     |
| Routes/navigation | `src/App.jsx` route regexp, sidebar destinations, page dispatch; `internal/httpapi/history.go:validHistoryRoute` server deep-link allowlist  | `/worlds/{world}/languages?language={id}&section={section}`; add to both allowlists and sidebar                |
| Icons/design      | `src/worldbuilding/NavigationIcon.jsx`, shared `ui.css`, domain CSS, `ui/Dialog.jsx`, feedback provider                                      | Existing typography, controls, tabs, dialogs, focus/escape/navigation guards; scoped Languages CSS             |
| Forms/state       | React local state; `useUnsavedChanges.js`; shared async confirmation; `historyRequest`                                                       | Explicit Save with retry/conflict; no compile on every keystroke; accessible structured editors                |
| Authentication    | `src/auth/api.js` installs same-origin CSRF fetch; Go `secureServe` + `authorize`                                                            | World-scoped routes; real view/edit checks and existing CSRF protection                                        |
| Backend           | Go module `kriemhild`, Go 1.26 declaration; current toolchain Go 1.27.1; standard `http.ServeMux`                                            | Internal Go packages, typed requests/results; no new service or mandatory native dependency                    |
| Persistence       | `internal/storage`: SQLite locally, pgx/PostgreSQL hosted; JSON World records, optimistic World revisions, content-addressed `history_blobs` | Immutable canonical definition JSON in existing blob store, referenced by typed language metadata              |
| Identity/Ages     | `internal/history/model.go`, `entities.go`: Age-specific Entity records with stable `ID`/`ContinuityID`; `inherit` clones records            | Language identity remains stable across Ages; state is copied with the existing Age behavior                   |
| Revisions/history | `UpdateHistory`, `recordRevision`, restored World revisions; history snapshots/blobs; no general domain CRDT                                 | Append immutable definition references; World revision comparison for every mutation; no silent conflict merge |
| Relationships     | Entity registry and typed validated relations; organization/culture references                                                               | Language-family/variety/speaker/location/script relationships with validated references                        |
| Portability       | `HistoryBundle`, World ZIP export/import, blob hash verification; no Git-backed live database                                                | Include all reachable definition versions/dependencies in bundles; standalone human-readable JSON export       |
| Tests/build       | Go tests, Node test runner, Playwright repository regressions, Prettier/UI boundaries, Vite build                                            | Unit/golden/determinism/limit tests first, then real storage/API/browser/permissions coverage                  |
| Deployment        | Multi-stage Dockerfile, CGO disabled; existing CI builds image and runs SQLite/PostgreSQL smoke checks                                       | Remain compatible with a single Go executable; add Languages to real smoke coverage                            |

No existing language/script subsystem, FST binding, language table, or language
registry type was found. The Library remains documents and archives. It is not
the storage identity or navigation owner of Languages.

### Architectural mismatches requiring care

1. The hosted `secureServe` currently holds `accessMu` around normal API handlers.
   Running compilation there would block unrelated requests. Language execution
   must authorize/capture an immutable request under the existing access policy,
   release the global gate for bounded computation, then reacquire and recheck
   identity, membership and expected revision before publishing a mutation or
   returning private results. Add targeted revocation/concurrency tests; do not
   remove the map gate globally.
2. World writes currently rewrite small JSON record collections. Embedding a
   complete lexicon in each Entity would enlarge every unrelated map save.
   Store immutable definition bodies as blobs; keep compact references/statuses
   on the Age record. Lexical search/indexes are derived and rebuildable.
3. There is no live Git storage backend. Human-readable JSON portability and
   canonical hashes fit this repository; claiming automatic Git synchronization
   would be incorrect.
4. The full retired `internal/legacyterrain` suite has an already documented
   baseline timeout. Do not conceal that by changing Docker/CI test selection.
5. Initially Docker CLI could not connect to `dockerDesktopLinuxEngine`.
   Starting the installed Docker Desktop enabled a real isolated Linux build,
   tests and Windows/Linux byte comparisons; see `SPIKE.md`. This does not
   establish that the unchanged full production Docker suite passes.
6. `history.EntityName` already has free-text Language and Script fields. Preserve
   those authored labels; introduce validated optional stable references without
   silently interpreting every existing string as an entity ID.

## Four distinct persisted concepts

1. **Language identity**: the existing World Entity ID/ContinuityID, kind
   `language`. Languages can be unrelated, descendants, varieties, reconstructed,
   extinct or revived without compulsory grammatical categories.
2. **Language state**: typed Age-specific metadata on that Entity: description,
   author-defined kind/status, family/variety references, speakers, regions,
   script associations, draft/active definition references. Existing Age copies
   keep the identity and immutable revision references but clone editable state.
3. **Definition revision**: immutable JSON addressed by its canonical SHA-256,
   with schema version, language identity, parent/source references, typed rule
   IDs, modules, tests and provenance. An edit creates a new body/hash; it never
   overwrites its ancestor. All retained revisions remain exportable/readable.
4. **Compiled artifact**: separate engine-versioned output keyed by definition
   hash, pinned dependency hashes, compiler/engine version and adapter. Bounded
   server cache; never the sole representation of authored data. Compilation
   reports/statuses identify exactly what was compiled. Editing creates a stale
   status; execution must explicitly name an executable immutable revision.

Scripts are independent World entities, with their own immutable definitions.
Language-specific orthography references a script revision. A script can serve
many languages and a language may have no script. Pinned old dependencies remain
reproducible; newer associated script revisions produce an out-of-date notice,
not an invisible substitution during execution.

Proposed storage reuses `history_blobs` instead of introducing a second database.
`internal/storage/languages.go` should atomically insert the body, append its
reference and update the Age state with the expected World revision.
`HistoryBundle` must traverse retained language/script references and validate
hash, ownership, schema and required dependencies on import. Blob lookup must
first prove membership in the authorized World/record revision list.

Large-lexicon indexing is a derived concern: index by World, language, immutable
revision and lexeme ID, use stable pagination, and rebuild after import. Do not
make an index or a browser table the authoritative lexicon.

## Versioned definition proposal

The initial proposal uses typed JSON, not user-executable code. Stable IDs name
segments, features, natural classes, templates, constraints, lexemes, morphemes,
paradigms, constructions, spelling rules, sound changes and regression cases.
The inventory and grammatical features are author defined. Phonological token
IDs are distinct from IPA labels, written graphemes and user-perceived text.

Every module can be absent or incomplete in a saved draft. Compilation checks
references, feature types, rule order, dependency cycles and resource limits.
Readiness is per operation: an incomplete Syntax module must not falsely make a
working phonology operation unavailable, nor imply syntax is executable.

The schema provides typed structures for:

- Phonology: segment features, natural classes, ordered token patterns,
  boundaries/environments, templates, positional/forbidden-sequence constraints,
  allophones and suprasegmental metadata.
- Lexicon: stable lexemes/morphemes, multiple senses, category, token sequences,
  grammatical features, derivation/etymology/semantic relations and attestations.
- Morphology: paradigms, feature-conditioned realizations, named operations,
  allomorph priority/ambiguity, suppletion/zero marking and extensible typed
  mechanisms. Unsupported mechanisms must yield diagnostics, not approximations.
- Syntax: typed feature structures, semantic roles and abstract constructions,
  constituent order, explicit agreement and compositional realization.
- Orthography: pinned independent script references, graphemes/glyph metadata,
  sequence mappings, normalization and explicit ambiguity policy.
- History: ordered sound changes with source/target references, exceptions,
  cognates, author-supplied attestation/confidence and transformation provenance.
- Test cases and module dependency declarations.

The Phase 0 spike deliberately executes a narrower subset. Its schema/type names
are experimental and are not a promise that every proposed mechanism works.
`SPIKE.md` records supported/unsupported semantics and concrete proof results.
No schema version will be called stable until the representative fixture and
the review of this proposal establish its public contract.

Canonicalization is a specified KRIEMHILD JSON contract, not an untested claim of
RFC 8785 compliance: typed fields, lexical map-key order, integer weights/limits,
explicit array order, no executable fields, no unknown/duplicate object keys,
and no implicit Unicode normalization of authored definitions. Exact engine,
canonicalizer and seed-selection versions participate in reproducibility.

## API/UI plan

World-scoped logical routes under `/api/worlds/{world}/languages`:
list/create; retrieve/save immutable revisions; compile/validate; execute a
typed operation; compare/export/import; preview/apply history. Age and language
IDs must resolve to the same World. Use existing edit permissions for saved
mutations and view permissions for pure computation; never let POST accidentally
grant viewers edit rights. History apply needs a preview hash, expected source
and target references, and explicit author confirmation.

Responses identify status, language/Age/revision, schema/compiler/engine,
operation/input/policy/seed, candidates, diagnostics, trace and truncation.
Missing dependencies, unsupported rules, ambiguity, no valid result and resource
exhaustion are separate outcomes. Generated candidates are never auto-published
to the lexicon. Diagnostics include stable code, severity, module, rule/entry ID,
field path and remedy. No clock, locale or map iteration influences a result.

UI additions: dedicated Languages top-level destination, language list with real
search/filters, create wizard, then contextual Overview / Phonology / Lexicon /
Morphology / Syntax & Semantics / Scripts & Orthography / Historical Linguistics /
Engine Laboratory. Reuse the current Age bar, dialogs and navigation guards.
Use structured forms; advanced JSON is an inspection/import option, not the only
way to author rules. Narrow screens use a contextual section selector. Text
editing/counting uses grapheme clusters; custom glyph rendering stays explicitly
unsupported until real rendering exists.

## Proposed file changes

| Milestone      | New code                                                                                                                                              | Existing files to extend                                                                                                         |
| -------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| 0: audit/spike | `docs/languages/*`, `internal/languages/spike/*`, representative JSON/golden fixtures                                                                 | No production shell, storage, database, auth or runtime changes                                                                  |
| 1: lifecycle   | `src/worldbuilding/Languages.jsx`, `languages.css`, `internal/history/languages.go`, `internal/storage/languages.go`, `internal/httpapi/languages.go` | App routes/sidebar, NavigationIcon, registry, history Entity validation/cloning, bundle import/export, server route registration |
| 2: compiler    | `internal/languages/{schema,decode,compile,diagnostic,canonical,dependencies}.go`                                                                     | Typed language request/response integration, compile status and bounded artifact cache                                           |
| 3: phonology   | phonology/pattern/selection modules, structured phonology editor and laboratory                                                                       | Compiler readiness, saved fixtures and traces                                                                                    |
| 4: morphology  | relation adapter, lexicon/paradigm modules, structured editors/search                                                                                 | Immutable definition persistence and derived indexed search                                                                      |
| 5: scripts     | independent script types, orthography/transcription modules, script editor                                                                            | Registry/references/bundle membership and dependency staleness                                                                   |
| 6: grammar     | typed AST/unification/realization/parse modules and construction editor                                                                               | Laboratory, traces and readiness                                                                                                 |
| 7: history     | transformation preview/apply and comparisons                                                                                                          | Age references, target revision persistence, shared confirmation/history                                                         |
| 8: hardening   | corpus/benchmarks, API/permission/portability/browser/container tests, developer guide                                                                | Existing CI/smoke checks, without hiding existing baseline failures                                                              |

## Library research and provisional decision

- [HFST upstream](https://github.com/hfst/hfst): toolkit/bridge with several
  native backends; upstream describes GPLv3 licensing and additional dependency
  license considerations. Introducing it would change the current CGO-free
  packaging. No binding is selected or redistributed in Phase 0.
- [foma upstream](https://fomafst.github.io/) and its
  [README](https://github.com/mhulden/foma/blob/master/foma/README): Apache 2.0,
  C API and CLI, with composition/inversion and linguistic replacement rules.
  Upstream README reports 0.10.0; that is not a verified currently installed
  version. Native build/deployment and trace preservation need their own spike.
- [GF reference](https://www.grammaticalframework.org/doc/gf-reference.html):
  useful abstract/concrete grammar separation. The
  [core project](https://github.com/GrammaticalFramework/gf-core) introduces a
  separate compiler/runtime toolchain. Use the separation as prior art; no GF
  source or compulsory natural-language grammar is imported.
- [UniMorph](https://github.com/unimorph/unimorph): useful terminology and
  interoperability reference, not an exhaustive or mandatory feature inventory.

The first experiment is a KRIEMHILD-owned finite relation adapter, represented
by bounded acyclic token transducers for explicitly enumerated fixture forms.
It must implement forward and inverse traversal with exact underlying analysis
and trace payloads. This proves the adapter contract without a native dependency.
It is **not** a replacement for HFST/foma's productive composition algorithms,
nor evidence of large-lexicon scalability. Keep native backend choice open until
representative productive rules, packaging and performance are measured.

## Milestone exit checks

0. Repository note + versioned schema proposal + authored fixture with inventory,
   natural class, phonotactics, paradigm, context rule, forward/reverse relation,
   traces, seeded repeatability; Windows run and honest Docker/build evidence.
1. Create/save/reload/search real incomplete drafts; Age copy, permissions,
   stale write conflict, version references and round-trip export pass.
2. Valid/incomplete/invalid definitions produce correct readiness/diagnostics;
   immutable compilation and dependency staleness are tested.
3. Deterministic form generation, rule ordering, positional constraints,
   rejection traces, ambiguity and candidate exhaustion are tested.
4. Lexicon editing, regular and irregular paradigms, reverse ambiguity,
   collisions/import previews and limits are tested against real storage.
5. Independent scripts, pinned dependencies, Unicode normalization and
   multi-code-point graphemes are tested; no false custom-font claim.
6. Explicit constructions, feature unification, generation/parsing and glosses
   pass; unsupported constructions fail explicitly.
7. Preview/apply creates a target revision without mutating source Ages;
   collisions, approval, stale preview and rollback are tested.
8. Actual repository/build/deployment, resource, accessibility and portability
   checks run; remaining failures/limitations are recorded rather than hidden.
