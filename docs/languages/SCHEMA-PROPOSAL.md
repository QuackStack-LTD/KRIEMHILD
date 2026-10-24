# Proposed Languages schema contract

**Architectural model approved; serialized contract remains pre-release.** This separates the intended
product contract from the deliberately smaller executable Phase 0 spike. Product
implementation should not publish a version until this contract is agreed and
the storage/API round-trip is tested.

## Identity and revision envelopes

| Structure             | Fields and meaning                                                                                                                                                                         |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Language identity     | Existing `history.Entity` with kind `language`; `id` and `continuityId` follow existing 32-hex-ID conventions                                                                              |
| Age state             | `ageId`, author-defined `kind`/`status`, description/notes, lineage/variety links, speakers/regions, script associations, `draftRevision`, optional `activeRevision`, retained `revisions` |
| Revision reference    | SHA-256 `hash`, schema/canonicalizer, language or script identity, parent/source hashes, body byte size; provenance and authorship live in an envelope excluded from linguistic choice     |
| Definition body       | `schema`, `languageId`, typed module definitions, module dependency declarations, regression cases and author notes; immutable after save                                                  |
| Compile reference     | Definition hash, exact compiler/engine/adapter versions, dependency hashes, deterministic diagnostic/readiness report; never substituted for definition data                               |
| Script identity/state | Independent kind `script`, stable continuity, Age state, retained immutable script definitions; many-to-many language associations                                                         |
| Execution request     | World/Age, language ID, exact definition hash, exact engine/compiler/adapter, operation, structured input, explicit policy, optional seed, candidate/work limits                           |
| Execution result      | Status and identity, request echo, candidates/analyses, intermediates/rule IDs, diagnostics, known counts, omission/truncation/limit state                                                 |

Reference hashes are World scoped by membership, not globally authorized merely
because a caller knows a hash. Source identities never change when a browser
selects a different revision. Age inheritance copies editable state and shares
immutable bodies. A descendant is a distinct identity with a source revision;
it does not mutate an ancestor or silently copy every rule.

Revision authorship/provenance is explicit metadata, not a generation seed.
Wall-clock timestamps never participate in linguistic derivation. Definitions
may contain incomplete modules; only operations whose dependencies are fully
validated become executable. Saving is not compilation or activation.

## Common typed structures

- `FeatureDefinition`: stable ID, label, value type, allowed atoms or reference
  type, optional author notes. No obligatory gender/number/case/tense inventory.
- `FeatureValue`: tagged atom, typed structure, variable reference or explicitly
  absent value. Absence is distinct from an empty string and from a default.
- `FeatureStructure`: type ID and named feature values. Type compatibility and
  occurs/cycle checks belong to unification. Unsupported higher-order types
  must fail compilation until implemented.
- `Selector`: tagged segment ID, natural-class ID, boundary, or bounded sequence.
  Its tag prevents ambiguity between a segment named C and a class named C.
- `RuleHeader`: stable ID, rule type/version, enabled state, explicit order or
  priority, stratum/domain, author notes/provenance. A rule type must have a
  registered parser, checker, compiler and executor before it is executable.
- `Diagnostic`: code, severity, module, rule/lexeme/reference ID, field path,
  explanation and remedy. Diagnostic output is bounded with omitted counts.
- `TraceStep`: operation/rule ID and exact version, source span or positions,
  before/after typed representation, tested conditions and rejection reason.
- `Dependency`: owner module, referenced module/entity and pinned revision,
  required operation/capability, supported compatibility version.

## Module contracts

| Module                 | Structured authored data                                                                                                                                                                                                                          | Execution/dependency contract                                                                                                                                                                                           |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Phonology              | Segment IDs and feature bundles, phonemic/phonetic status, optional IPA labels; natural-class predicates; prosodic/tone/stress/length structures; syllable/word templates; positional/sequence constraints; ordered phonological/allophonic rules | Operates on token/feature structures, never orthographic pixels or assumed one-character phonemes. Each supported pattern class has bounded semantics. Morpheme and word boundaries are distinct.                       |
| Lexicon                | Lexeme/morpheme IDs, senses, category, underlying forms, grammatical features, inflection class, derivational/compound structure, semantic relations, etymology/cognates, attestations and variety/Age forms                                      | Generated candidates enter only through an explicit author save. Homographs/homophones are diagnostics/analyses, not automatically merged identities.                                                                   |
| Morphology             | Author-defined grammatical features, paradigms/cells, feature requests, realizations and allomorph conditions/priority; tagged prefix/suffix/infix/circumfix/zero/suppletion/reduplication/interdigitation operations                             | A backend advertises the exact supported rule subset and inverse-analysis capability. A currently unsupported tagged operation remains editable in a draft but cannot masquerade as executable.                         |
| Syntax/semantics       | Abstract typed roles/arguments, category/feature structures, constructions, subconstruction references, agreement/unification equations, constituent order, explicit literals/separators and gloss mappings                                       | Generation realizes a specified construction/semantic structure. Parsing reports all supported analyses within caps. No generic sentence/translation fallback. Recursive constructions need explicit depth/work limits. |
| Orthography            | Independent pinned script revision, ordered sequence-to-grapheme relations, transcription/transliteration schemes, exceptions, segmentation and normalization policy                                                                              | Many segments/graphemes may map to one or many outputs. Analyze ambiguity explicitly. Normalization is author-selected; text uses grapheme clusters.                                                                    |
| Script definition      | Grapheme IDs, Unicode strings where meaningful, glyph asset/shape metadata, direction/layout, font/coverage metadata, historical variants                                                                                                         | Missing glyph rendering remains unsupported with retained metadata; it is not replaced by a claimed working custom script. A script is not exclusively owned by a language.                                             |
| Historical linguistics | Source/target identities/revisions/Ages, ordered sound/grammar/orthography changes, exceptions, cognate sets, borrowing/contact references, author-chosen confidence/attestation                                                                  | Compile and preview against immutable sources. Apply verifies preview/source/target/world revisions, requires author confirmation, and appends a target revision plus derivation links. Source forms remain unchanged.  |
| Regression corpus      | Stable case ID, exact operation and source revision policy, input/features/policy/seed, expected typed outputs/trace or diagnostic, notes                                                                                                         | Running tests does not approve a candidate or mutate the lexicon. Behavior changes require reviewed golden changes and an engine/schema compatibility decision.                                                         |

Optional modules are omitted/undecided rather than populated with invented
defaults. Structural validity for saving a draft is distinct from semantic
compilability and per-operation readiness. Native-script association is optional.

## Proposed canonical and compatibility rules

1. `schema` identifies the serialized domain contract. Unsupported schema versions
   are preserved only as explicitly incompatible imports, never silently decoded
   into a smaller schema and re-saved with lost fields.
2. JSON rejects unknown keys where their rule/version contract is not understood,
   duplicate keys, invalid UTF-8/surrogates, excessive depth and excessive sizes.
   A future extensions mechanism must have its own typed versioned contract.
3. Canonical bodies use fixed typed-field order and lexical string-map order;
   array order remains authored semantics. A canonicalizer version identifies
   these rules. This proposal does not claim RFC 8785 compliance.
4. No implicit Unicode normalization of authored bodies. An explicit orthography
   policy normalizes complete realized forms and records that trace step.
5. New revisions append immutable bodies. Parents/source references remain
   reachable in export. Undo/restoration changes a state reference, not a body.
6. Artifact identity includes all pinned dependency hashes and exact compiler,
   engine and adapter versions. Unsupported old engine versions report
   incompatible; they never execute under an unrecorded replacement.
7. A rule edit changes the draft hash and marks the previous compile stale for
   that draft. Executing a deliberately selected older compiled revision retains
   its exact old dependencies and output behavior.
8. Candidate sets are canonically ordered before deterministic selection. Seed
   selection cannot operate on an unexplained truncated prefix. Author weights
   may rank only already-valid alternatives and need a versioned integer policy.

## Deliberate Phase 0 restrictions

`internal/languages/spike/schema.go` is the executable **spike** contract, not
the full schema above. It has flat string feature bundles, finite token patterns,
single-pass ordered word rules, explicit paradigm cells, prefix/suffix/zero and
per-lexeme suppletion, plus fixture-only embedded transcription. It does not yet
implement persistent identities/Ages, independently saved scripts, recursive
feature structures/grammar, historical batches, productive FST composition,
weighted policies, phonetic audio or custom glyph rendering. Those capabilities
must be added and tested before being advertised as supported.
