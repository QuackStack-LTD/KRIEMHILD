# Backend decision for A1

Decision: retain the KRIEMHILD-owned CGO-free adapter for this **opt-in bounded
increment**. Do not adopt a native backend yet. This is not a declaration that
Milestone B's full comparative study is complete, or that the adapter scales to
arbitrarily large languages. General FST composition/minimization and an indexed
productive inverse remain open.

## Reproducible inputs and command

`internal/languages/productive_benchmark_test.go:productiveInventory` constructs
16/128/1,000-entry inventories with varied 11-segment stems (more than 900 unique
stems at 1,000 entries), deliberately retained homophones, polysemy, suppletion,
three cells, conditioned allomorphs, circumfixing and boundary rules. An independent
test checks executable results and rejects a malformed inventory. No benchmark
counts an invalid compile as useful work. The small hand-authored fixture and
full forward/inverse goldens separately establish linguistic correctness.

```powershell
.tools/go/bin/go.exe test ./internal/languages -run '^$' -bench '^BenchmarkProductiveA1$' -benchtime=3x -count=5 -benchmem
.tools/go/bin/go.exe test ./internal/languages -run TestProductiveRetained -v -count=1
```

Windows amd64, Go 1.27.1, Intel i5-6300HQ, four logical CPUs. Five benchmark
samples, three operations per sample; no confidence interval is inferred from
this small desktop sample. Cold means a newly compiled artifact with input already
loaded, **not** OS/process cold start. Warm reuses an immutable artifact. Revision
invalidation includes cloning/editing a definition, a cache miss, recompilation
and bounded cache insertion. Wall-clock measurements can vary with other desktop
work. Total allocation and GC snapshots below are not peak/RSS measurements.

| Entries | Operation | Min / median / max ms | Median allocated KiB/op | Outcome |
| --- | --- | --- | --- | --- |
| 16 | Compile cold | 0.93 / 1.06 / 1.24 | 227.2 | Executable plans; no expanded forms |
| 16 | Forward warm | 0.13 / 0.15 / 0.15 | 34.5 | Complete |
| 16 | Inverse warm | 5.41 / 5.73 / 6.30 | 1358.1 | Complete, 48 cells examined |
| 16 | Ambiguity warm | 6.14 / 6.40 / 6.83 | 1366.3 | Complete finite ambiguity |
| 16 | Forward cold | 1.15 / 1.27 / 1.38 | 258.0 | Complete |
| 16 | Revision invalidation | 1.31 / 1.35 / 1.42 | 297.1 | Stale artifact not reused |
| 128 | Compile cold | 2.89 / 3.02 / 3.24 | 799.6 | Executable plans |
| 128 | Forward warm | 0.13 / 0.14 / 0.18 | 34.5 | Complete |
| 128 | Inverse warm | 46.20 / 47.39 / 50.65 | 10864.4 | Complete, 384 cells examined |
| 128 | Ambiguity warm | 44.85 / 46.12 / 49.68 | 10864.2 | Complete finite ambiguity |
| 128 | Forward cold | 3.27 / 3.30 / 3.39 | 834.6 | Complete |
| 128 | Revision invalidation | 4.44 / 4.47 / 5.80 | 1138.5 | Stale artifact not reused |
| 1,000 | Compile cold | 19.87 / 20.43 / 22.26 | 6169.7 | Executable plans |
| 1,000 | Forward warm | 0.15 / 0.16 / 0.31 | 34.5 | Complete |
| 1,000 | Inverse warm | 185.88 / 192.54 / 199.39 | 39330.8 | **execution_limit**, 1,384 cells examined; partial candidates cleared |
| 1,000 | Ambiguity warm | 190.66 / 196.52 / 204.28 | 39284.5 | **execution_limit**, no partial choice |
| 1,000 | Forward cold | 21.48 / 21.95 / 22.71 | 6590.6 | Complete |
| 1,000 | Revision invalidation | 29.56 / 31.97 / 32.79 | 8587.6 | Stale artifact not reused |

GC snapshots gave approximately 125,304 retained additional bytes at 128 entries
and 844,872 at 1,000, versus cache reservations 460,092 and 2,819,964 bytes. These
are noisy retained-heap deltas with live input/artifact held through the snapshot,
not maximum heap or a promise that a compute job occupies only that much memory.
The 8 MiB serialized-payload budget likewise differs from total allocation.

## Correctness, deployment and alternatives

The adapter composes authored finite relations lazily. Forward/inverse fixtures
preserve ambiguity, ordering, epsilon stages, boundary states and trace provenance.
An inverse limit is a product limitation, not a successful empty analysis.
Cancellation is tested after work begins; obsolete work is not queued by the API.
Existing version-1 results remain covered by their original full-result goldens.
Windows/Linux raw-byte replay and non-root SQLite/PostgreSQL restart/ZIP/replay
commands are recorded in VERIFICATION.

No new dependency, native library, CGO requirement or third-party source was added;
`go.mod`, `go.sum` and package lockfiles are unchanged. A native candidate was not
installed, benchmarked or licensed for distribution in this increment. Therefore
there is no evidence here that foma/HFST is faster/slower or preserves this trace
contract. Their packaging/license review remains a prerequisite to any prototype
that would be distributed or integrated. No native backend has been approved.

Next: **A2**, typed/indexed inverse composition that prunes impossible analyses
without pre-expanding the entire surface lexicon or losing derivations. Repeat
these workloads and add process-cold latency and peak-memory instrumentation.
Only then use the same correctness/trace corpus for an isolated candidate backend
comparison if the owned adapter remains inadequate. Do not raise safety limits
to turn the 1,000-entry limit outcome into an apparent performance success.
