# Brief 5: sequence corpus audit

2026-10-28, Windows amd64, branch `rebuild/continuous-terrain`, clean starting
HEAD `3a82adf190ff0931db7d8b7ac7ee1ad238e210a3`. Recent commits: `3a82adf`
delivery record, `54e8e47` artifact ownership, `8a0772e` retained NFC replay,
`35d1151` normalization trace, `0b8b0fe` workbench. No AGENTS.md was found in
the repository or ancestor chain. Go 1.27.1, Node 24.21.0; CGO disabled.

| Capability | Status | Current evidence / next increment |
| --- | --- | --- |
| Language schemas 1–4, A1/A2, Script 1/2 | working | Fresh `go test -count=1 ./internal/languages/...`; complete old goldens retained |
| Sequence forward, inverse/A2, Unicode, renderer | working | `sequence_*test.go`, `graphemes_test.go`, production tests rerun |
| Immutable revisions, dependency ownership, archive 4/5 | working | Fresh history/storage tests, including `languages_sequence_test.go` |
| Sequence API and final authorization | working | Fresh `go test -count=1 ./internal/httpapi -run '^TestLanguage'` |
| Candidate-only corpus | working | `corpus.go`; sequence requests explicitly rejected with `unsupported_case` |
| Structured sequence corpus | unsupported | No typed expectation/runner; add standalone verification format, without changing Definition.Tests |
| Human review of new corpus expectations | unverified | Existing full-result goldens can be wrapped without regeneration; author review requested explicitly |
| Fresh browser/platform/database parity | unverified | Existing Brief 4 evidence is historical until rerun below |
| Prosody, recursive grammar, general inversion, font export | unsupported | Outside Brief 5; no new controls or semantics |

Baseline executions before editing application logic:

- `CGO_ENABLED=0 .tools/go/bin/go.exe test -count=1 ./internal/languages/... ./internal/history ./internal/storage`: pass; languages 11.018s, spike 1.884s, history 1.327s, storage 7.546s.
- `.tools/go/bin/go.exe test -count=1 ./internal/httpapi -run '^TestLanguage'`: pass, 6.608s.
- `npm.cmd run build`: pass, 26.02s.
- `npm.cmd test`: running; result appended when complete.

The author process/database on port 8124 is excluded from all work. Temporary
Worlds, test directories, isolated servers and disposable containers are used.
The retired legacyterrain timeout, generic container smoke-route mismatch and
Biosphere formatting failures remain unrelated boundaries; no tests are weakened.

## First increment and version implications

Implement `sequence-corpus/1` as external verification data referencing immutable
language/Script hashes, with exact typed results, strict whole-file validation,
explicit review provenance and production-engine execution. Keep old candidate
corpus behavior, language/compiler/adapter/canonicalizer/renderer/archive versions
and old golden bytes unchanged. See [SEQUENCE_CORPUS.md](SEQUENCE_CORPUS.md).
