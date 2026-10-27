# Sequence orthography /1 specification

This is the formal implementation scope for Brief 4. Authored rules live in the
immutable language body as `sequenceWriting`, not a parallel store. Explicit
language schema **4**, `languages/4`, `sequence-compose/1`,
`languages-compiler/4`. Productive morphology remains `productive/1` and existing
A2 remains unchanged under its old versions. Canonicalizer remains typed-json/1.
World archive **5** must reject older readers that cannot interpret schema 4.

## Layers and pins

Source declares `surface` or `surface-boundaries`. A composed form uses the
existing post-phonological derivation immediately before boundary erasure; the
surface variant explicitly selects only its segments. The underlying lexeme,
morphology, typed boundary steps and chosen derivation remain separate evidence.
Raw source-unit runs are explicit inputs, never mislabeled lexical analyses.

Units are tagged segment, boundary, grapheme, sign; edge is a context-only virtual
word edge. Values are exact authored segment/boundary/cluster strings; sign units
contain structured Script 2 sign/reading/role/context references, never Unicode-
derived glyph IDs. Stages declare input/output layer. The first rewrite converts
surface units to graphemes, optional grapheme rewrites/normalization follow, and
an optional final grapheme-to-sign stage produces the existing renderer's input.
No sign rewriting, generic expressions, natural-class inference or automatic shaping.

Source pins Script identity/hash and renderer engine/adapter/compiler. It also
pins `uniseg/0.4.7-unicode/15.0.0` and `x-text/0.29.0-unicode/15.0.0` policies.
The [upstream library](https://github.com/rivo/uniseg/tree/v0.4.7) implements extended
grapheme segmentation; this is Unicode 15.0 behavior, not a claim about later
Unicode revisions. Source arrays retain authored order. Version/source/dependency
changes create new artifact identities and require explicit save/compile/activation.

## Rewrite semantics and termination

At each input position, match literal typed input and optional adjacent left/right
contexts against the stage's immutable input snapshot. Choose highest numeric
priority; advance by consumed input length. Potentially overlapping equal-priority
patterns (including prefixes) are rejected at compile time, conservatively even
where contexts might make them exclusive. Array order is never an implicit tie
breaker. Outputs are not rescanned within a stage. Stages run once in array order;
even A→B then B→A terminates. Empty-input rules are invalid. Empty output explicitly
deletes matched input. Empty whole input succeeds with an empty output, not a
fabricated character/glyph. No matching rule is `no_mapping`, unless the authored
same-layer `identity` policy preserves the unit explicitly. Cross-layer identity
is invalid. Boundaries match/delete only through explicit typed rules.

After grapheme-producing stages, adjacent output fragments are resegmented as a
whole string, merging provenance where fragments form one cluster. Normalization
is only a declared NFC/NFD stage, never source cleanup. Its trace retains original
and normalized strings. The implementation must reject any unsupported provenance
alignment rather than guess. Spans name their offset unit: source token, grapheme
cluster or sign. UTF-8/UTF-16/code-point offsets are separately named.

Every output carries source indices and applied rule/stage provenance. Traces
retain stage inputs/outputs, match spans and replace/delete/unchanged actions.
Results distinguish unchanged success, no mapping, invalid/conflicting rules,
missing glyph/reading, unsupported behavior, cancellation and limit reached.
SVG is derived through the pinned Script 2 renderer, never stored as rule source.

## Inverse policy and ordering

`inverse=all` derives reverse transitions from forward rule outputs. It enumerates
bounded output segmentations, reconstructs typed input, then validates candidates
by replaying the canonical deterministic forward stage (including priority and
contexts). Duplicate traversal paths for an identical preimage/canonical trace are
coalesced explicitly; distinct source sequences, lexemes and derivations remain.
Same-layer identity is tested by forward replay, never assumed to have applied.
Deletion/empty-output and normalization are lossy: inverse readiness is false with
`unsupported_inverse`; forward remains available. `inverse=unsupported` is an
explicit author choice. No independent inverse rule source or silent first choice.

Inverse orthography runs **before optional A2 morphological analysis**. For boundary
inputs, A2 candidates must also reproduce the same pre-erasure boundary stream.
A2's algorithm and schema-3 results are unchanged. All valid results within budget
are returned in stable rule/sequence order; a limit clears partial spelling sets.
An explicit selected preimage may be marked, but alternatives are retained.

## Budgets and UI contract

Maximum source body remains 2 MiB, 8 stages, 128 total rules, 16 input/output units
per rule, 4 context units per side. Maximum 512 units, 4 KiB text/cluster, 100,000
work steps, 128 inverse paths/results, 1 MiB trace and 8 MiB payload; the existing
renderer also enforces its independent node/geometry/SVG limits. Check cancellation
inside matching, segmentation, inverse enumeration and rendering. These are
budgets/accounting, not measured peak heap. Do not raise bounds for benchmarks.

Use existing creation/Scripts/Laboratory workflow, server readiness and immutable
revision controls. Show rules, source boundaries, stages, grapheme clusters,
independent sign/glyph IDs, preview and inverse alternatives. Cluster selection
uses server segmentation offsets, never environment-dependent Intl.Segmenter.
Rule lists paginate; stale jobs are cancelled/superseded and editing stays usable.
No second glyph editor or active imported SVG.

## Application workflow and API

Create a language with **Sequence orthography · schema 4**, or explicitly import
such a revision. Existing languages do not migrate. In **Scripts**, select an
exact Script 2 revision, author stages and rules, then use the existing Save draft,
Validate, Compile saved revision, Activate and definition revision controls.
The rule editor is paginated at 20 and reuses the existing Script workbench's
artwork; this editor only references signs, readings and roles.

The Scripts spelling laboratory offers composed lexical forms or explicitly
supplied surface units (`|` denotes the declared morpheme boundary). Explicit
units test orthography only, not lexical/phonotactic validity. Choose `spell`,
`unspell` or `graphemes`; enabled state comes from server capabilities/readiness
and exact saved compile identity. Draft edits are never executed as saved source.
The ordinary Laboratory keeps its existing operations; sequence operations live
beside their rules in Scripts. Run results expose stages/provenance, separate
clusters/code points, sign/glyph IDs and a generated SVG image. Inverse choices
retain the complete alternative set. Editing examples or source cancels obsolete
work, including stale errors from dependency fetches.

Use the existing `POST /api/worlds/{world}/languages/{language}/run` with its
World/Age and `definitionRevision` envelope and an exact `request`. Schema 4
requests pin `revision`, `engine=languages/4`, `adapter=sequence-compose/1`,
`compiler=languages-compiler/4`, `policy=all`, and `limit` (1–128 for complete
sequence result sets). `operation` selects spell/unspell/graphemes.

- Spell: `lexeme` plus an authored `features` bundle, or `spelling.units` tagged
  source units; `segments` is also accepted within its older 256-segment bound.
- Unspell: exact `text` for grapheme output, or structured `spelling.units` for
  sign output. `spelling.analyze=true` runs A2 afterward. `spelling.choice` marks
  an explicitly selected returned preimage ID without discarding alternatives.
- Graphemes: exact `text`, optionally `spelling.selection={start,end}` using
  grapheme-cluster indices (half-open). `ClusterText` uses the returned UTF-16
  boundaries for its read-only written-output selection; raw author input fields
  remain ordinary Unicode text fields. No universal text-editor cursor claim.

The response's ordinary `result` retains language identity/hash, compiler,
adapter, canonical request and pinned dependencies. `result.spelling` contains
source/output layers, segmentation/normalizer/renderer pins, deterministic work
counts, structured spellings/preimages, clusters and selections. A spelling run
retains its composed form and typed morphology when applicable. `stages` records
normalization form, original/normalized text and named offsets. Render results
retain exact Script revision and independent sign/glyph/reading/role identities.

The orthography revision is the containing canonical language revision: there is
no separately mutable orthography head. Rules, policies and all pins participate
in that hash and existing World/Age/compiler artifact keys. Cached artifacts also
account for the pinned Script source; no SVG is authoritative. Compile/execute
uses existing permission capture, bounded compute and final authorization/CAS.
World archive 5 traverses every retained rule-bearing revision and pinned Script,
validating owners, schema/hash identity and missing/corrupt dependencies. Archive
4 cannot silently accept these bodies. Source strings are never normalized on Save.

## Precisely limited support

Version 1 uses literal typed units, not natural classes, arbitrary regular
expressions, variables or user code. Epsilon-input insertion is unsupported;
consuming output expansion and explicit deletion are available. Deletion and
normalization disable inverse for the entire pipeline. A declared inverse policy
does not make a lossy stage executable. Inverse candidates are bounded preimages
of these rules, not general phonological inversion. A2 still has its own
256-segment and other budgets; failure propagates explicitly instead of becoming
no-result. No output set is presented as exhaustive after a limit.

Unicode support is the pinned UAX29 Unicode 15.0 extended-cluster implementation,
tested with composed/decomposed strings, multiple marks, supplementary code
points, ZWJ emoji, flags and modifiers. It is not support for later Unicode
versions, arbitrary fonts, shaping or semantics. Normalization uses pinned x/text
0.29.0 Unicode 15.0 tables. No stripping of combining marks. Newer Unicode data
or changed interpretation requires version/policy and golden review.

Sequence corpus expectations in the old candidate-only corpus format are
explicitly rejected (`unsupported_case`); complete pinned external goldens are
used here. A richer typed corpus expectation/editor is future work. Vertical
layout, two-dimensional quadrats, advanced bidi, fonts, prosody, recursive grammar,
semantic interpretation and generalized ligatures remain unsupported.

The added Go-only dependency is rivo/uniseg v0.4.7 (MIT), pinned in go.mod/go.sum;
its license is retained in [licenses/uniseg.txt](licenses/uniseg.txt). No native
backend or CGO requirement was introduced.
