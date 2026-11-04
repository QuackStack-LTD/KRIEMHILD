# A2 indexed inverse composition

Explicit opt-in: language schema 3, `languages/3`, `finite-compose/2`,
`languages-compiler/3`, derived `survivor-trie/1`. Schema 1 and A1 schema 2 retain
their canonical bodies, versions, goldens and full enumeration behavior. No
automatic migration occurs. Forward productive semantics remain `productive/1`.

## Index and correctness argument

Compile retains compatible lexical/cell references in authored order, including
suppletion. For each source form, omit any segment that matches any consuming
rule input (including natural classes). Every remaining segment must survive
in the same order: affixes surround the form, snapshot sound rules never reorder
unconsumed tokens, and boundary erasure removes only typed boundaries. A trie of
these necessary subsequences therefore excludes only impossible forms. Its first
matching prefix position dominates later positions, so query traversal activates
each trie node at most once. Terminal references preserve lexical and cell
identity; root references preserve epsilon signatures. This is conservative
pruning, not a general inverse transducer or pre-expanded surface dictionary.

All eligible cells still execute the ordinary ordered composer, conditions,
boundary stratum, phonotactics and pinned writing. Actual allomorph derivations
remain distinct. Written search uses NFD nonmark code points from explicit pinned
mappings, keeping a conservative necessary subsequence under normalization across
mapping boundaries. It does not greedily segment written strings or equate code
points with graphemes. Exact authored written output is compared afterward.

In contrast to A1, inverse rejection diagnostics concern eligible composed cells,
not every unrelated failed form. This observable difference is why the profile
is separately versioned. All A1 goldens remain unchanged. The new full golden
reviews `pada`: rock/many and stone/many, 9 examined cells, 2 rejected forms,
3 pruned cells, actual typed steps and pinned versions.

Index identity includes source revision, dependency hashes, schema, compiler,
engine, adapter and index version. The source hash covers lexical and rule data.
Request identity includes the complete canonical request, including policy/seed.
Cold/warm/rebuilt results contain no timings/cache-hit flags. HTTP runtime metadata
stays outside deterministic linguistic output. Indexes live in bounded disposable
artifacts; authored source remains in immutable database/ZIP bodies.

Limits: 32,768 combined trie nodes, 64,000 compatible lexical cells, 200,000
construction steps, 8 MiB accounted index storage, 8,192 eligible request cells;
ordinary 100,000 execution steps, 8 MiB intermediate/result payload, 256 live
branches, typed-token/segment limits, two-slot/five-second HTTP gate remain.
Cancellation is checked during build/traversal/composition. Index build failure
disables inverse readiness independently of forward execution. Work-limit results
clear partial candidates. Display caps remain explicit `candidate_limit` with
known-valid counts. Accounted storage is not a measured peak heap.

## Current measured evidence

Windows amd64 / Intel i5-6300HQ / four CPUs / Go 1.27.1. Existing varied
`productiveInventory`: 16/128/1,000 entries (1,000 is the current authored-source
lexeme limit), >900 distinct long stems at 1,000,
deliberate homophones, polysemy, 53 suppletions, suffix/circumfix/zero/conditions.
Five samples of three operations per benchmark. Cold means compile + request
with source already loaded, not OS/process cold. Concurrent desktop activity
affected timings; these are observations, not stable confidence intervals.

```powershell
.tools/go/bin/go.exe test ./internal/languages -run '^$' -bench '^BenchmarkProductiveA1$' -benchtime=3x -count=5 -benchmem
.tools/go/bin/go.exe test ./internal/languages -run '^$' -bench '^BenchmarkProductiveA2$' -benchtime=3x -count=5 -benchmem
```

| Entries | Trial | Min / median / max ms | Median total allocated KiB/op | Outcome |
| --- | --- | --- | --- | --- |
| 16 | A2 compile cold | 1.066 / 1.148 / 1.223 | 279.6 | complete |
| 16 | A2 inverse warm | 0.764 / 0.781 / 0.796 | 173.8 | complete |
| 16 | A2 inverse cold | 1.830 / 1.902 / 2.135 | 455.3 | complete |
| 128 | A1 inverse warm | 55.415 / 69.393 / 121.603 | 10855.5 | complete |
| 128 | A2 compile cold | 3.768 / 3.864 / 4.526 | 1243.7 | complete |
| 128 | A2 inverse warm | 1.035 / 1.160 / 1.385 | 259.7 | complete |
| 128 | A2 inverse cold | 5.205 / 5.309 / 5.440 | 1464.0 | complete |
| 1,000 | A1 inverse warm | 212.164 / 224.965 / 230.029 | 39316.3 | execution_limit, 1,384 examined, no partial output |
| 1,000 | A2 compile cold | 28.321 / 32.145 / 37.016 | 8841.3 | complete |
| 1,000 | A2 inverse warm | 2.287 / 2.377 / 2.576 | 496.0 | complete, 17 examined, 1 valid, 1,164 steps |
| 1,000 | A2 inverse cold | 28.076 / 29.340 / 29.794 | 9731.2 | complete |
| 1,000 | A2 suppletion warm | 4.696 / 4.767 / 4.796 | 1069.3 | complete, all 53 analyses, 1,032 steps |

Peak heap/RSS and representative production-language scaling remain unverified;
allocation totals/reservations are not presented as peak measurements. Separate
process replay and SQL-runtime/restart/ZIP gates are recorded in BRIEF_3_AUDIT
when rerun. No native dependency was evaluated or adopted. The owned adapter is
adequate for this measured bounded domain; a general backend comparison remains
open if workloads beyond these limits require one.

Correctness tests compare the index with exhaustive forward-domain results for
allomorph epsilon alternatives, deletion, insertion and ordinary morphology;
test normalization, explicit limits/cancellation, cold rebuilds and policy/source
invalidation. Storage tests retain A1 alongside A2, compile/activate, restart,
export/import and compare canonical results. New bodies require World archive 4,
including retained/pinned bodies; ordinary/A1 World archive versions stay intact.

Remaining limitations: no general inversion of arbitrary phonology; rules that
can consume all distinguishing segments can still reach an explicit work cap.
The existing phonotactic validator does not accept empty whole words. Zero
morphology and distinct epsilon derivations are supported; that is not a claim
of valid empty-word generation. Prosody, recursive grammar and historical
execution on productive profiles remain unsupported.
