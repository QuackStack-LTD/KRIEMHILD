# Sequence corpus /1

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

Hard bounds: 2 MiB corpus, 32 cases, 8 distinct language pins, 16 KiB request,
256 KiB expectation per case, 1 MiB total expectations, 64 KiB strings within
expectations, 512 input units, 4 KiB input text, 128 results. Runner budgets may
only reduce these: case count, production calls, deterministic charged work and
report bytes. Each call reserves the existing engine's maximum work before
execution; observed spelling steps are also reported, not called CPU or peak
heap. Production execution retains its existing inner cancellation and work,
trace, inverse-path, output and renderer caps. Complete validation/preflight
precedes execution. Cancelled or exhausted runs retain completed case reports
and mark remaining cases explicitly, never passing them as empty results.

Exhaustiveness is only a bounded cross-check: an explicit unique alphabet and
maximum length (up to 3, at most 64 input streams) are enumerated in deterministic
order. Forward output is inverted with production unspell, and the exact source
unit sequence must occur among preimages. This property is not an authored
golden or a claim of general reversibility; normalization/deletion and A2 are
excluded. Any cap/failure invalidates the exhaustiveness claim.

CLI/API commands, exact final bounds and observed verification are recorded with
the implementation in BRIEF_5_AUDIT.md. Portability evidence is reported per
platform/database; unavailable checks remain unverified.
