# Languages: versioned product engine

Brief 5 adds a separate [typed sequence corpus](SEQUENCE_CORPUS.md), exact
immutable pins, validation-only and read-only execution through the production
engine, complete structured expectations and actionable JSON/text reports.
[Current evidence and remaining coverage gaps](BRIEF_5_AUDIT.md) distinguish the
four author-approved retained goldens from unreviewed future expectations.
Language/Script/archive semantics and the candidate-only corpus are unchanged.

Brief 4 adds an explicit opt-in **language schema 4** spelling bridge:
[specification and workflow](SEQUENCE_ORTHOGRAPHY.md),
[current audit, commands and gaps](BRIEF_4_AUDIT.md). It connects productive
surface/boundary streams to Unicode-15 grapheme clusters and pinned Script 2
signs/rendering, with finite literal rules, explicit normalization, typed
provenance and bounded eligible inverse spelling before optional A2 analysis.
Existing schemas and goldens are retained without automatic migration.

Brief 3 starts with [BRIEF_3_AUDIT.md](BRIEF_3_AUDIT.md). The explicitly opted-in
schema-3 [indexed inverse adapter](INDEXED_A2.md) preserves A1 alongside the new
path; existing definitions are not migrated. The A1 description below continues
to describe schema 2, including its enumeration limit.

[Custom vector Scripts](CUSTOM_SCRIPTS.md) add an independent opt-in Script
schema 2: draw/import normalized glyphs, author roles/readings and structured
sequences, preview LTR/RTL output, then save/compile/activate immutable revisions
in the existing Scripts workspace. Unicode Script schema 1 remains compatible.
Script 2 alone has no spelling-rule executor; language schema 4 supplies it.
Vertical layout, font export, prosody and recursive grammar remain unsupported.
The historical Brief 3 results and completion/gap matrices are
recorded in [BRIEF_3_AUDIT.md](BRIEF_3_AUDIT.md).

The continuation audit is [CONTINUATION_AUDIT.md](CONTINUATION_AUDIT.md).
The first delivered increment, **A1**, adds an opt-in productive adapter alongside
the existing engine. It does not replace the verified kernel or migrate saved
languages. [PRODUCTIVE_A1.md](PRODUCTIVE_A1.md) defines its semantics;
[BACKEND_DECISION_A1.md](BACKEND_DECISION_A1.md) records measured limits and the
provisional backend decision. The schema-1 baseline remains documented below.

## Productive composition (A1)

Choose **Productive composition · schema 2** in the creation wizard, or explicitly
save/import a schema-2 revision. Empty inventories remain empty. Author productive
features, paradigms/cells, ordered operations and allomorph conditions in Morphology.
Supported operations are prefix, suffix, zero and whole-form circumfix. Conditions
use explicit feature bundles, category, the immediately preceding operation and
supported segment/class edge contexts. `first` and `all` precedence are authored;
there is no inferred default allomorph. An empty operation chain is explicitly the
identity relation. Cell overrides provide complete suppletion.

Compile validates/indexes plans without pre-expanding surface forms. Requests apply
the ordered plan, retain typed morpheme boundaries through a named consuming rule
stratum, erase boundaries explicitly, validate surface phonotactics and optionally
write through the pinned script. Insertion/deletion/substitution see snapshots;
insertion runs once per gap in the typed stream, never recursively on its output.
Word phonology rules must be moved explicitly to this stratum in schema 2.

Forward, phonological inverse and written inverse preserve lexical/cell/allomorph
identity, homophones and epsilon stages. Results include actual typed intermediate
states, selected/rejected alternatives, failed paths and ordinary readable traces.
Inverse traverses the same finite authored lexical domain. It is **not** an optimized
general inverse FST: the varied 128-entry benchmark completes; the 1,000-entry
benchmark reaches the deterministic work cap. A work limit clears partial results
and cannot make a seeded choice from an incomplete prefix. This remains a narrow
Milestone A increment, not completion of the entire advanced roadmap.

Schema 2 uses `languages/2`, `finite-compose/1`, `languages-compiler/2` and the
unchanged `kriemhild-typed-json/1`. Requests must pin engine, adapter **and compiler**.
Compile references and cache keys include compiler; the source hash includes all
authored rules and pinned dependency hashes. Schema-1 requests still omit compiler
and retain their existing result bytes; capability metadata names their implicit
compiler `languages-compiler/1`. A1's pinned Unicode Scripts remain schema 1. No automatic migration
changes a source body, Age state, active revision or script dependency.

`GET /api/worlds/{world}/languages/capabilities` reports the actual supported
profiles. New compile reports expose capabilities separately from dependency
readiness; the Laboratory renders those server capabilities. Unsupported schema-2
grammar/history are explained in their tabs while retained data and immutable
revision restoration stay available. Prosody, recursive grammar, sequence spelling,
custom fonts and reconstruction are still unsupported.

Corpus capture saves typed traces only after explicit author action and Save.
Schema 2 refuses seeded corpus cases: adding expectations changes the revision used
for seeded selection. Capture an `all` run instead, or replay the exact external
seeded request against its retained revision. Legacy schema-1 diagnostic ordering
for multiple unknown features and revision-sensitive seeded corpus behavior remain
documented limitations; schema 2 does not silently reinterpret those old requests.

Existing limits remain, with at most 64 productive paradigms, 64 cells each, 16
operations/cell, 16 allomorphs/operation, 32 boundary rules, 16 selectors/context,
256 live branches, 256 surface segments and 512 typed tokens. Execution checks
cancellation in traversal/rule loops and caps 100,000 steps and 8 MiB of logical
serialized intermediate/result payload. That payload cap is not a measured peak
Go heap limit. The server retains its two-slot admission and five-second context.

World archives with any retained schema-2 definition or pinned dependency use
**World archive schema 3**. Older readers reject it. Worlds containing only
schema-1 languages still use archive schema 2, and ordinary Worlds retain schema 1.
Import checks the body schema against its reference, including mixed revision
histories; a Library document is not mistaken for an authoritative definition.

The author approved stable identity, separate Age state, immutable definition
revisions, and separately cached compiled artifacts. Languages uses existing
World storage, permissions, history, navigation, and portable World ZIPs. It
introduces no AI integration, model calls, translation service, mandatory native
executable, or new database.

## Author workflow

Open **Languages** in the World sidebar. The existing Age bar selects the state.
Create a language or independent script, including an incomplete draft. No
reference inventory, grammatical categories or example lexicon are inserted.

**Save draft** appends an immutable body when the definition changes; metadata
updates affect only Age state. **Validate draft** reports readiness without
saving. **Compile saved revision** records an exact report and builds a
disposable artifact. **Activate revision** explicitly chooses a compiled body
for this Age. Generated candidates never become lexemes automatically.

Overview, Phonology, Lexicon, Morphology, Syntax, Scripts, History and Laboratory
share the selected draft. Structured lists paginate/search above 20 rows. Lexeme
editing needs no DSL. Rule selectors distinguish segment, natural class and word
boundary. Stable IDs and structured feature maps remain inspectable/editable.

The Laboratory generates words, validates segment sequences, enumerates
paradigms, inflects, analyzes phonological/written forms, writes explicit surface
segments, realizes/parses supported constructions and runs regression cases.
Results expose requests, versions, valid/rejected counts, caps, diagnostics and
traces. Sentence annotations show all authored senses/features, not inferred
translations. Capturing a result creates a draft regression case; Save remains
explicit. Cases can be renamed, annotated or removed.

Unsaved edits require a discard decision before navigation. Failed saves retain
the draft. Conflicts offer comparison with the latest saved body; refreshing a
World revision does not silently replace fields. Viewers can inspect, validate
and run compiled revisions. Save, compile-report persistence, activation and
historical apply require edit permission. Late responses cannot overwrite a
newly selected definition.

## Model, versions and portability

| Concept            | Representation                                                                                        |
| ------------------ | ----------------------------------------------------------------------------------------------------- |
| Identity           | Existing `history.Entity`, kind `language` / `script`, ID/ContinuityID                                |
| Age state          | Typed record state: metadata, links, draft/active heads, retained revision/compile references         |
| Immutable revision | Canonical JSON in `history_blobs`; SHA-256, size, parent, author, note and timestamp in its reference |
| Artifact           | Immutable in-memory relations/indexes, separate bounded LRU, rebuilt after restart                    |

Age inheritance clones editable state and shares immutable bodies. Age/merge
comparison includes language/script changes. A descendant may pin an ancestor's
exact revision. Existing free-text Language/Script labels on names are preserved,
not reinterpreted as IDs. Profile commands cannot inject definition references.
The Age's **Name** is identity metadata; **Definition label** belongs to the
immutable snapshot. Renaming in an Age does not change the linguistic source
hash, and saving/restoring a definition does not erase an Age-specific name.

Writes use World/head comparisons and existing history attribution. A known hash
does not authorize a read: it must be retained by the requested World/Age/record.
Pinned script, historical and lexical sources are verified when saved. Export
follows retained bodies/dependencies even after source Age deletion. Import
checks ownership, canonical bytes, sizes, hashes, schema and dependencies before
commit; existing World-copy semantics preserve stable language identities.
World ZIPs containing language/script records use archive schema **2**, so older
schema 1 readers reject them instead of silently discarding new fields. Existing
World ZIPs without these records remain schema 1 and continue to import. Older
pre-Languages binaries must not write a database containing the new typed state;
use the pre-upgrade backup when reverting to such a binary.

Versions: `schema: 1`, `languages/1`, `token-relation/1`, and
`kriemhild-typed-json/1`. The product adapter uses the verified
`languages-spike/1` kernel; its fixture schema is not the public persisted schema.
Requests require exact source/engine/adapter versions. A changed draft is stale
relative to its previous compile. Older compiled revisions are selected
explicitly. Updating a script draft does not modify languages pinned to its old
revision.

Canonical JSON uses typed field order, sorted string-map keys and authored array
order; it is not RFC 8785. No implicit source Unicode normalization occurs.
Decode rejects duplicate/unknown keys, invalid UTF-8/surrogates, unsupported
schemas, excess size/depth and trailing JSON. Writing explicitly chooses `none`,
`NFC` or `NFD`, applied to the complete form with a trace. Incompatible source
shapes need a schema decision; behavior changes need version/golden review.

## Supported subset and remaining scope

| Area       | Executable / persisted now                                                                                                                                                                                                | Partial / planned                                                                                                                                            |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Phonology  | Segment IDs/features/status/labels, natural classes, finite templates, allowed/forbidden patterns, ordered contextual substitution/deletion/insertion, word boundaries                                                    | Prosodic/stress/tone trees, morpheme-boundary strata, productive recursive rules                                                                             |
| Lexicon    | Lexemes/senses, underlying forms, categories/features, paradigm links and cell suppletion; morphemes/segmentation, semantic links, derivational annotations, pinned cognate/borrowing provenance/confidence, attestations | Automatic derivation/compounding execution, large lexical indexes, specialist bulk CSV tooling                                                               |
| Morphology | Flat authored features, explicit cells, ordered prefix/suffix/zero, lexical suppletion; bounded forward enumeration/inverse relations                                                                                     | Conditional productive allomorph grammars, infix/circumfix/reduplication/interdigitation execution, weighted policies; unsupported kinds produce diagnostics |
| Scripts    | Independent reusable revisions, Unicode graphemes, direction/glyph metadata, segment-to-grapheme chunks, normalization, ambiguous inverse matching; writing needs no lexicon                                              | Arbitrary sequence spelling rules, multiple active schemes, custom glyph/font/layout execution                                                               |
| Grammar    | Explicit roles/categories, flat unification/agreement, order/literals/separators, morphological realization, bounded parsing, authored interlinear annotations                                                            | Recursive typed structures/constructions and higher-order unification; typed source structures are retained but explicitly rejected for execution            |
| History    | Pinned sources/targets, ordered sound changes, target inventory/phonotactics, lexical exceptions, pure preview, confirmed append-only apply, source preservation/comparison                                               | Automatic reconstruction, grammar/orthography change solvers or inferred contact                                                                             |
| Tooling    | Real API/storage/UI, JSON and portable World import/export, CLI, corpus/goldens, cancellation/permissions/cache, real browser and SQLite/PostgreSQL container tests                                                       | Native HFST/foma remains open pending productive-rule performance/licensing/deployment work                                                                  |

The advanced roadmap is broader than this initial engine. A retained annotation
is not an executable rule. Finite relations preserve distinct lexeme/cell
analyses and homophonic ambiguity. Underspecified features enumerate compatible
authored cells without invented defaults. Flat agreement propagates explicit
values and rejects conflicts. Parsing consumes exact authored separators. There
is no generic natural-language parser/translator.

Writing requires mappings for every declared segment before readiness. Invalid
or incomplete spelling cannot remove valid phonological candidates. Dependency
sets compile independently; a **partial** report may contain usable operations
and precise errors in another module.

Historical preview copies source lexeme fields into the target, retains
target-only lexemes and the target's structural annotations, and changes no
source data. Exceptions are final validated target forms, bypassing sound
changes. Apply recomputes the preview, checks its hash and World/target head,
requires explicit author approval and appends a revision. It does not compile or
activate the target. Retained revisions remain available for comparison.
**Restore saved revision as draft** changes the draft pointer after confirmation,
preserving every retained body and the independently selected active revision.

## Determinism, diagnostics and bounds

Canonical candidate ordering precedes versioned SHA-256 seed selection; words
and sentences have separate selection IDs. Seeds are required only for seeded
choice. Unknown operations and irrelevant fields fail explicitly. Choice never
uses an unexplained truncated search prefix. Display caps expose known counts;
work exhaustion returns a limit result, never fabricated output. Timestamps,
locale and process randomness do not participate in linguistic execution.

Statuses include `success`, `ambiguous`, `choice_required`, `no_valid_candidate`,
`candidate_limit`, `execution_limit`, `cancelled`, `missing_dependency`,
`invalid_input`, `unsupported_operation`, `uncompiled_revision`,
`incompatible_engine` and `regression_failure`. Diagnostics identify modules,
paths and rule/reference IDs.

Bounds include 2 MiB JSON transport/bodies, depth 32, 1,000 lexemes, 128 corpus
cases, 256 retained revisions, 32 compile references per Age state, 256 displayed
candidates/segments per form, 4 KiB text/forms and 1 KiB seeds. The finite kernel
caps forms at 10,000, states at 100,000 per relation, and form/trace payloads at
8 MiB. Parsing caps prepared forms/steps at 10,000 and payloads at 8 MiB. Exports
cap pinned traversal at 32 levels/4,096 bodies and retain existing archive limits.

The server admits two compute jobs without an accumulating queue and uses a
five-second compute context. Artifacts use a 16 MiB LRU reservation budget;
reservations are conservative accounting, not measured peak heap. Oversized
artifacts execute in a bounded job but are not retained. Keys include World,
Age, definition/pinned dependency hashes, engine and adapter. Compilation holds
neither the cache mutex nor the global access gate. Publication reacquires the
gate and rechecks session/membership before disclosing results or writing.

## Reproduction and extension

```powershell
go test ./internal/languages/... ./internal/history ./internal/storage
go test ./internal/httpapi -run 'TestLanguage|TestLanguages'
node --test tests/languages.test.mjs
npm run build
go run ./cmd/languages -definition internal/languages/testdata/reference.json -script internal/languages/testdata/script.json
go run ./cmd/languages -operation run -definition internal/languages/testdata/reference.json -script internal/languages/testdata/script.json -request internal/languages/testdata/inflect.request.json
```

The authored reference includes phonology, regular/irregular paradigms, Unicode
writing, grammar, corpus and descendant changes. Goldens assert full outputs,
traces and provenance. Review intentional changes and version implications;
never regenerate expected outputs solely to suppress a failure.

New rules require typed schema, validation, dependency/readiness analysis,
bounded compilation/execution, forward/inverse tests where supported, stable
traces and documentation. Unknown fields cannot be silently dropped or run as
scripts. Keep native bindings behind the adapter and retain authoritative source.

The browser suite requires an isolated server and explicit opt-in:

```powershell
$env:KRIEMHILD_UI_URL='http://127.0.0.1:8136'
$env:KRIEMHILD_LANGUAGES_TEST='1'
npx playwright test tests/languages.browser.spec.mjs
```

`Dockerfile.engine` verifies the Linux engine. `Dockerfile.runtime` plus
`scripts/languages-container-smoke.mjs` verifies a real Linux app/React build,
SQLite/PostgreSQL, restart, export/import and replay. These preserve the unchanged
production Docker gates and their known retired-terrain baseline failures.
See [verification](VERIFICATION.md), [architecture](ARCHITECTURE.md) and the
[Phase 0 spike](SPIKE.md).
