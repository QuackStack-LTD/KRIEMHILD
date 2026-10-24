# Languages: initial product engine

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
