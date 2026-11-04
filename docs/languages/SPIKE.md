# Phase 0 executable experiment

The Phase 0 implementation is `internal/languages/spike`, with its original CLI
under `internal/languages/spike/cmd/linguistic-spike`. Phase 0 was isolated from
product routes and author data. The approved product integration now consumes
this verified kernel behind the separately versioned `internal/languages`
adapter; its public schema and scripts are documented in [README.md](README.md).
No model API, prompt call, remote linguistic service or invented fallback exists
in either implementation.

## Executable subset and semantics

The JSON decoder rejects duplicate/unknown fields, trailing objects, invalid
UTF-8/unpaired Unicode surrogates, excessive size/depth and incompatible schemas.
The canonical typed representation preserves authored array order and Unicode;
string-map keys serialize lexically. This is `kriemhild-typed-json/1`, not a
claim of RFC 8785 implementation. The fixture definition's hash is
`6090994d05db7748d9feb1fd59f06582473554e0d0694e4edbd7597e40a14435`.

Segments have independent IDs, labels, phonemic/phonetic status and author-defined
feature bundles. Natural classes match feature predicates. Structured selectors
are exactly one of `{segment: id}`, `{class: id}`, or `{boundary: "word"}`.
Templates enumerate declared phonemic segments only. Allowed complete-word
patterns and forbidden sequences determine validity; a matched template alone
does not prove validity.

Sound rules have typed input/output and left/right environments. The supported
domain is one word stratum. Each rule makes a single consuming left-to-right
pass over its own input snapshot, in authored array order. Insertion applies once
per gap and output is not recursively rewritten by the same rule. Deletion and
word edges have explicit semantics; outputs are bounded. These choices guarantee
termination for this subset, not for an unrestricted replacement DSL.

Morphology uses explicit feature-conditioned paradigm cells. Supported operations
are prefix, suffix and zero; lexical suppletion replaces the selected cell's
entire regular plan. Flat feature unification reports conflicting values and
does not add grammatical categories. Missing cells do not acquire invented forms.

The `RelationBackend` adapter compiles exact lexical analyses to their realized
surface sequences in a finite, acyclic two-tape token trie. Forward and inverse
traversal follow actual arcs, including one-sided epsilon arcs. Ambiguous forms
retain all analyses within limits. Written inverse traversal consumes authored
mapping strings, so alternate string segmentations remain possible; it does not
assume one code point is a phoneme or grapheme.

Fixture transcription is optional and embedded only for this experiment.
Mappings preserve complete strings, including combining marks. Explicit NFC/NFD
normalizes complete outputs, including across mapping boundaries, and records
the normalization step. Product scripts will have independent identities and
pinned revisions. This experiment is not a Unicode grapheme-break implementation
or a custom-font renderer.

All results identify exact definition/engine/selection versions and request,
include intermediate rule traces, and report rejections and omitted diagnostics.
The selection policy can return all candidates, require explicit choice or use
the recorded seed. Seeded choice hashes a versioned typed structure containing
Version, Seed, Revision, Operation and the complete canonical candidate sequence,
takes the first eight digest bytes as a big-endian uint64, and selects modulo
the candidate count. It is an explicit deterministic selection rule, not a
statistical linguistic model or weighted sampler. It never selects from an
unexplained truncated enumeration. No generated candidate is saved automatically.

Current spike limits: 2 MiB definition; 32 JSON nesting levels; 128 segments;
64 natural classes; 32 sound rules; 16 pattern positions; 1000 lexemes; 64
paradigms with 64 cells each; 10,000 compiled/enumerated forms; 100,000 states
per relation; 100,000 traversal steps; 256 segments per realized form; 8 MiB
compiled form/trace payload and 8 MiB generated candidate/trace payload; 256
displayed candidates. Compilation diagnostics cap at 256 and rejection details
at 64, with explicit omission counts. Context cancellation aborts computation.

## Authored reference and goldens

`testdata/reference.json` defines six segments (five phonemes plus one phonetic
realization), feature-based V/C classes, word patterns, a forbidden sequence,
one ordered context rule, four lexemes, a one/many paradigm, one suppletive cell
and a transcription containing a combining mark. These are authored test data,
never fabricated production content.

The regular derivation is `/p a t/ + /a/ → /p a t a/ → [p a d a] → pad̪a`.
The one form remains `pat`, showing that t only voices between vowels. The
suppletive many form of person is explicitly `/n i n/`, without the regular
suffix. Stone and rock intentionally share forms to test ambiguous analysis.
The template VCV has 12 underlying candidates: phonetic d is not a generated
phoneme; `ipi` is rejected; 11 candidates remain. The fixed seed chooses
`/i t a/ → [i d a] → id̪a` under the versioned selection rule.

`regular-many.golden.json` and `seeded.golden.json` freeze the full expected
request/result metadata, intermediate sequences, applied rule IDs and rejection
diagnostic. Intentional changes require review of the linguistic derivation,
canonical/engine compatibility and golden diff; do not auto-regenerate expected
routes merely to make a test pass.

## Reproduce

From the repository root, using the supported Go executable:

```powershell
.\.tools\go\bin\go.exe test ./internal/languages/spike/...
.\.tools\go\bin\go.exe vet ./internal/languages/spike/...
.\.tools\go\bin\go.exe run ./internal/languages/spike/cmd/linguistic-spike `
  -definition internal/languages/spike/testdata/reference.json `
  -operation inflect -lexeme stone -feature number=many
.\.tools\go\bin\go.exe run ./internal/languages/spike/cmd/linguistic-spike `
  -definition internal/languages/spike/testdata/reference.json `
  -operation generate -template vowel-consonant-vowel `
  -policy seeded -seed fixed-author-seed
docker build -f docs/languages/Dockerfile.spike `
  -t kriemhild-languages-spike:phase0 .
docker run --rm kriemhild-languages-spike:phase0
```

The dedicated Docker ignore file exposes only this experiment and module
manifests. The image downloads the pinned existing `golang.org/x/text` module;
this repository has no Go vendor tree. Production Dockerfile/ignore behavior is
unchanged. `go build` can also cross-compile the CLI without CGO.

## Recorded evidence

- Windows Go 1.27.1: unit tests, both full-trace goldens, vet and Linux cross-build
  passed. Tests cover stale identity/engine requests, missing modules, ambiguity,
  rule ordering, boundaries, Unicode, unsupported operations, strict JSON,
  immutable inputs/results, cancellation and resource exhaustion.
- Docker Engine 29.8.2, Go 1.27.3 linux/amd64: the isolated image built and ran
  its tests/vet/build. Regular and seeded CLI JSON outputs were byte-for-byte
  identical to Windows, including full traces (2523 and 2167 bytes respectively).
- Initial Windows benchmark, Intel Core i5-6300HQ, four logical CPUs: reference
  compile ~1.44 ms / 86.8 KB allocated; inflection with trace ~55 µs / 5.6 KB;
  1000 synthetic fixture lexemes, 2000 forms ~197 ms / 15.8 MB allocated. These
  are total benchmark allocations, not measured retained/peak memory. The large
  fixture duplicates authored forms under test IDs; it is not a realistic
  language corpus or proof of unbounded scalability.
- Existing frontend: 127 Node tests and production Vite build passed. New Go
  code has no imports from the production server and cannot change live Worlds.

The external `.cache/languages` logs record commands, benchmark output and
platform-comparison artifacts. It is an ignored working directory, not required
source. The existing retired terrain-suite timeout is separate baseline evidence.

## Explicitly not yet implemented

World/Age persistence, Languages navigation/editors, active/draft version lifecycle,
module-local incremental compilation, independently saved scripts, general
productive FST composition/minimization, a native HFST/foma binding, author
weights, infixes/circumfixes/reduplication/interdigitation, morpheme-boundary
strata, recursive typed grammar, sentence parsing, historical batch application,
custom fonts/audio and large-lexicon indexes are not provided by this spike.
The whole-definition spike compiler rejects invalid modules together; the
production compiler must expose readiness per valid operation/dependency.
Phase 1 must add the real persisted lifecycle and tests before advertising the
subsystem as a usable Languages product.
