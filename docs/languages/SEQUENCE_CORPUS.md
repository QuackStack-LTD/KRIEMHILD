# Sequence corpus /1 and /2

`sequence-corpus/1` is standalone verification data, not language source or a
mutable language store. No language, Script, canonicalizer or archive version
changes. Existing candidate-only Definition.Tests remains unchanged.

Each case declares a stable ID, description, tags, review provenance, exact
language identity/hash and schema/compiler/adapter/engine/canonicalizer pins,
Unicode segmentation/normalizer pins and exact Script/renderer pins. Requests
retain the production typed input and boundaries. The resolver must load that
exact immutable body in its explicit World/Age binding; active heads are never
consulted. A different imported World can supply a new explicit binding without
changing the linguistic pins. CLI resolution uses explicitly supplied source
files, never paths embedded in corpus JSON.

Expectations contain the complete typed production Result. Comparison is exact
structured JSON (canonical typed bytes, including null versus empty), including
all stages, provenance, offsets, Unicode, sign readings/roles, normalization,
SVG and A2 analyses/boundaries. Recursive field diffs identify the first bounded
set of mismatches. Optional unordered-preimages comparison sorts complete typed
preimages, preserving distinct lexical analyses and duplicate multiplicity;
deterministic display order is checked separately by exact comparison. No fuzzy
comparison or executable expressions. Result hashes identify complete rendered
and traced results; operational timings never participate.

Outcomes normalize explicit engine statuses into success, no_result, ambiguous,
unsupported, cancelled, limit_reached or rejected, while retaining raw statuses.
No mapping is no_result; a capped search is limit_reached, never exhaustive.
Missing/hash-mismatched/incompatible dependencies fail preflight before any case
executes. Validation-only resolves and verifies source and policies without
compilation/execution or writes. Execution compiles through the production
compiler and calls its Execute method; corpus code never implements spelling.

Review status is pending or reviewed. Reviewed requires a named reviewer and
an auditable note identifying the expectation's independent provenance. Pending
cases can execute for comparison, but cannot yield a verified passing corpus.
Scaffolding copies prior expectations; it never learns a golden from execution.
No automated claim of human review is made.
Review provenance is authored verification metadata; source-control review is
the approval mechanism, not a cryptographic reviewer identity service.

Hard bounds: 2 MiB corpus, 32 cases, 8 distinct language pins, 16 KiB request,
256 KiB expectation per case, 1 MiB total expectations, 64 KiB strings within
expectations, 512 input units, 4 KiB input text, 128 results. Runner budgets may
only reduce these: case count, production calls, deterministic charged work and
report bytes. A raw call charges 100,000 work units, composed forward adds
100,000, and inverse+A2 adds 128 × 100,000. These are conservative deterministic
execution reservations, not a CPU measurement or a bound on compilation,
segmentation or vector-rendering instructions. Those have separate production
limits. Observed spelling steps are reported separately, never called peak
heap. Production execution retains its existing inner cancellation and work,
trace, inverse-path, output and renderer caps. Complete validation/preflight
precedes execution. Cancelled or exhausted runs retain completed case reports
and mark remaining cases explicitly, never passing them as empty results.
Per-run artifact retention reuses the production 16 MiB LRU reservation budget;
oversized artifacts execute within production limits and are not cached. This
reservation is not a measured heap bound. Authored bodies are independently
bounded by 2 MiB/source, eight language pins and 32 preflight cases.

Exhaustiveness is only a bounded cross-check: an explicit unique alphabet and
maximum length (up to 3, at most 64 input streams) are enumerated in deterministic
order. Forward output is inverted with production unspell, and the exact source
unit sequence must occur among preimages. This property is not an authored
golden or a claim of general reversibility; normalization/deletion and A2 are
excluded. Any cap/failure invalidates the exhaustiveness claim.

CLI/API commands, exact final bounds and observed verification are recorded with
the implementation in BRIEF_5_AUDIT.md. Portability evidence is reported per
platform/database; unavailable checks remain unverified.

## Serialized contract and commands

The complete example is
`internal/languages/testdata/sequence.corpus.json`. The strict Go contract is
`sequence_corpus.go`. Required fields are `version`, `id`, `bounds`, `cases`;
each case requires `id`, `description`, `tags`, `review`, `pin`, `inputLayer`,
`request`, `outcome`, `comparison`, `expected`. `domain` is an optional property
check attached to a successful reviewed forward case. Its raw streams replace
that case's lexical input only for the property check; the reviewed primary
expectation/request remain unchanged. Limits are `calls` (1–128), `work`
(1–100,000,000), and `reportBytes` (at least 1024 + 1024 per case, max 2 MiB).
No source paths, executable callbacks or expressions can be authored in JSON.

`pin.revision` is both the exact language revision and canonical source hash.
The rule source is inside that same revision. Pin schema, canonicalizer, engine,
adapter, compiler, Unicode segmentation and normalization policy, Script ID/hash
and renderer/adapter/compiler. World/Age context is an explicit external resolver
binding, not a machine-specific field in a portable expected linguistic result.

`comparison=exact` compares all typed result bytes, arrays, trace and embedded SVG.
`unordered-preimages` only canonical-sorts complete outer preimages for comparison;
it preserves duplicate multiplicity and exact ordered A2 analyses inside each.
Use `exact` separately to assert display ordering. Actual complete results remain
in their production order. Render identity is checked through its complete typed
SVG result; the canonical report SHA-256 can additionally identify the whole run.
No separate contradictory authored inverse rules or partial field assertions.

```powershell
.tools/go/bin/go.exe build -o .cache/languages/corpus-cli.exe ./cmd/languages
.cache/languages/corpus-cli.exe -operation corpus-validate `
  -corpus internal/languages/testdata/sequence.corpus.json `
  -corpus-definition internal/languages/testdata/sequence.json `
  -corpus-definition internal/languages/testdata/sequence-normalized.json `
  -script internal/languages/testdata/sequence-script.json
```

Use `-operation corpus-run` with the same files to execute. JSON goes to stdout;
case summaries and actionable errors go to stderr. Exit 0: successful validation
or all reviewed expectations match; 1: mismatch/unreviewed expectation; 2: invalid
format/pin/source or input I/O error; 3: cancellation or runner budget exhaustion.
Underlying engine caps retain raw `actual.status` plus `actualOutcome=limit_reached`;
an unexpected cap is a comparison failure, never an exhaustive passing result.
Validation-only reports zero production calls and no case execution.

The read-only API is
`POST /api/worlds/{world}/languages/{language}/corpus` with
`{ageId, mode: "validate"|"execute", corpus}`. All case identities must match the
route. It uses existing authentication, CSRF, view permission, two-slot/five-second
compute gate and final session/membership/retained ownership recheck. No compile
record or active head is written. Capabilities expose `corpora` separately from
old artifact capabilities so old compiled/golden bytes remain unchanged.

Verification files stay in source control; they are not automatically added to
World ZIP. Archive 5 retains the immutable definitions and Script bodies needed
by the external corpus. Explicitly bind the same corpus to the imported World/Age
to reproduce it. Archive 4 rejection and older readers are unchanged.

The initial four retained Brief 4 expectations were approved by the author on
2026-10-28 and remain byte-for-byte intact. Brief 6 adds focused fixtures in
`internal/languages/testdata/sequence-corpus/`. The author explicitly authorized
independent derivation with **Codex review provenance**, followed by source-control
review; that metadata does not claim human approval. Expectations were authored
from retained goldens, production tests and documented rule semantics before
executing them. No engine-output capture or golden regeneration is provided.

## Additive version 2

`sequence-corpus/1` remains readable with identical canonical result bytes, bounds,
pins and comparisons. `/2` is explicitly required for three additions:

- Early `unsupported_inverse` results have the exact initialized-but-unconfigured
  production spelling object: version/Unicode policies, one work step, empty
  arrays and empty pipeline/Script/renderer fields. The diagnostic is exact.
  Preflight still resolves every immutable dependency and verifies that source
  explicitly disables inverse or contains deletion/normalization. Successful
  results cannot use this exception. Invented or partly filled rejection shapes
  fail validation.
- The renderer's actual `missing_sign` and `unresolved_reading` rejection statuses
  are accepted and compared as `rejected`, preserving the raw statuses. They are
  not renamed `missing_reading`, `unsupported`, or `no_result`.
- An optional `coordinates` table on successful `graphemes` cases records four
  ordered, distinctly named half-open span arrays: `utf8-byte`,
  `unicode-code-point`, `utf16-code-unit`, `grapheme-cluster`. The whole table is
  checked against the complete expected clusters and code-point arrays; exact
  Result comparison verifies those same clusters against execution. Code-point
  positions are cumulative counts, not aliases for bytes or UTF-16 offsets.

Version 2 also supports `phase="definition-validation"` with
`request.operation="validate-definition"`, `outcome="rejected"`, `expected=null`
and a `validation` expectation. This calls the existing production compiler and
**never Execute**. The exact semantic validation record contains `status`, the
complete `ready` map, all ordered `diagnostics`, and `diagnosticsOmitted=0`.
At least one error and disabled spell/unspell are required. The containing
definition can remain `partial` because independent morphology is ready; its
corpus outcome is still rejection. Compile counters, size metadata and adapter
capability descriptions are outside this validation record; all identity/version
checks remain in the exact pins. Execution cases still compare complete Results.
Validate-only mode performs source/pin preflight with zero compiler/execution
calls even for this phase. A rejected definition is never an execution success.

`phase`, `validation` and `coordinates` fields are rejected in `/1`, including
explicit null/empty fields. `/1` readers cannot read `/2`; no language, Script,
compiler, adapter, renderer, normalizer or archive schema is changed. Reports
retain the selected corpus version after successful decoding/preflight. CLI
pre-decode errors retain the historical `/1` error envelope. API capability discovery retains `version`
as `/1` and adds a `versions` list. CLI exit 0/1/2/3 and review gates are unchanged.

For example, with the previously built CLI:

```powershell
.cache/languages/corpus-cli.exe -operation corpus-run `
  -corpus internal/languages/testdata/sequence-corpus/topology-exact-inverse-a2.corpus.json `
  -corpus-definition internal/languages/testdata/sequence-corpus/topology.definition.json `
  -script internal/languages/testdata/sequence-script.json
```

The two topology-negative fixtures preserve the same surface segments while
shifting or omitting the pre-erasure morpheme boundary. Their explicit raw streams
spell, but inverse followed by A2 returns `no_result`. Positive expectations retain
the complete reviewed rock/stone candidates, derivations, allomorph alternatives
and pre/post-erasure boundary provenance. Adjacent, leading and trailing morpheme
boundaries are rejected as malformed topology. Word edges are virtual contexts;
there is no stored word-boundary token or word-boundary erasure stage.

NFC and NFD compare exact original/normalized strings, stage spans, provenance and
all clusters. There is no normalization-insensitive comparison. Unicode evidence
is pinned to the implementation's Unicode 15.0 tables and the authored cluster
examples; it makes no claim about later Unicode, fonts or arbitrary editors.

See [BRIEF_6_AUDIT.md](BRIEF_6_AUDIT.md) for the final evidence matrix, commands,
review record, compatibility and resource limitations. This closes the current
sequence-orthography corpus workstream.
