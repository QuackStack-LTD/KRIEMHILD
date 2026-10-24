# A1: finite productive composition semantics

This is an additive adapter, not a replacement for the verified kernel. An author
opts in by saving a new schema-2 revision. Old revisions remain exact replay inputs.

1. A productive module declares flat feature domains and ordered paradigms/cells.
   A lexical entry selects its paradigm. Requests unify lexical, cell and requested
   features. No missing value becomes a default. Invalid feature diagnostics sort
   keys. A lexical cell override is a complete suppletive plan, bypassing affixes
   but still passing the boundary-rule and surface-validation stages.
2. Each cell contains an authored ordered chain of operations. Kinds are `zero`,
   `prefix`, `suffix`, `circumfix`. Every operation has an explicit ordered list of
   allomorphs and policy `first` or `all`. Each allomorph has a stable ID, prefix/
   suffix segment arrays and a conjunction of feature, category, previous-operation
   and supported segment/class context predicates. Empty conditions explicitly mean
   unconditional. No matching allomorph rejects that path; there is no default.
   `first` chooses the first matching authored alternative; `all` preserves distinct
   derivations. Nonmatching and precedence-skipped alternatives are traced.
3. Prefix/suffix attach to the whole current form. A circumfix adds its nonempty
   prefix AND suffix in one operation, anchored to the two edges of that same form,
   with an explicit morpheme boundary on either side. This is not infixation,
   reduplication or interdigitation. Zero has empty affix arrays; epsilon stages
   remain in the trace. Underlying lexeme tokens are never overwritten.
4. Composition is lazy: compile validates/indexes authored plans; it does not
   enumerate surface forms into a trie. At execution, the lexical relation feeds
   affix relations in authored order, then a single named boundary-aware rule
   stratum. Rules use one consuming left-to-right snapshot pass, including bounded
   insertion/deletion. Input consumes segment selectors only; context may explicitly
   match segment/class, word edge or morpheme boundary. Boundaries are retained by
   every rule, then erased in one separately traced final stage. They are typed
   tokens, never segment IDs. No rule recursively rewrites its own output.
5. Surface phonotactics use the existing frozen phonology validator. In A1, schema
   2 phonology word rules must be empty: author changes in the explicit productive
   stratum instead of silently applying two overlapping rule pipelines. Writing
   uses the existing pinned script and whole-output normalization.
6. Forward inflection executes compatible authored cells at request time. Inverse
   analysis traverses the same finite authored lexeme/cell domain with the same
   forward composition and compares surface tokens or written strings. This is a
   complete bounded inverse, **not** a general optimized inverse transducer or a
   large-language scalability claim. Homophones, cell identity and distinct
   allomorph derivations remain distinct. Canonical ordering precedes display caps
   or seeded selection; a work limit clears partial candidates and never chooses
   from a truncated search prefix.
7. Limits: existing JSON/lexicon/output limits remain; at most 64 paradigms, 64
   cells each, 16 operations/cell, 16 allomorphs/operation, 32 rules, 16 selectors/
   context, 256 live candidate branches, 256 surface segments, 512 typed tokens,
   100,000 execution steps, 8 MiB intermediate/result trace payload. Check context
   cancellation inside enumeration, rule and branch loops. The existing server
   five-second context and two-slot admission remain unchanged.
8. Traces record actual lexical/cell/allomorph/rule IDs and typed before/after
   states, including boundaries, rejected alternatives and stage order. The result
   identifies source hash, schema, compiler, engine, adapter and pinned script
   dependencies. World/Age authorization context is supplied by the HTTP envelope,
   not used as a hidden linguistic input. Definitions remain authoritative; compile
   indexes/artifacts and search results are disposable.

A1 does not claim all Milestone A exit criteria: general finite-state composition,
optimized inversion, richer morphological context and later non-concatenative
mechanisms still require their own increments. Benchmark results decide only
whether this bounded adapter is adequate for A1, not whether foma/HFST is inferior.

Compatibility review: schema-2 requests and compile references explicitly pin
`languages-compiler/2` in addition to engine/adapter. Schema-1 requests retain the
old omitted compiler field and byte-compatible goldens. A schema-1 body rejects
the new `productive` field even when null. New nested inventory limits are checked
before persistence, rather than leaving oversized source arrays for execution.
An archive containing a retained/pinned schema-2 body requires World schema 3;
reference/body schemas must agree. Ordinary Library JSON is not inspected as a
linguistic definition.

Corpus policy: seeded choice includes the exact source revision. Adding expected
output changes that revision, making a seeded corpus case self-referential. A1
reports `revision_sensitive_case` and does not advertise those cases as executable.
All-candidates corpus expectations include typed accepted/rejected derivations;
external seeded requests remain replayable against their exact retained revision.
