# Phase implementation and acceptance status

This file distinguishes working implementation slices from the exit gates in `PLATFORM_PLAN.md` §26. Adding a workspace or an API does not by itself complete a phase. The existing plan and specification coverage remain the intended scope; this status does not silently narrow them.

| Phase | Present implementation | Acceptance status / remaining work |
|---|---|---|
| P1 | Local projects, custom entities/relations, image maps and pins, rich manuscript editing/recovery, search, independent Ages, comparisons, undo/redo | Existing storage/API/browser regression suite retained. See P1 guide. |
| P2 | Seeded regional terrain, authored geography, layers, calendars, dated change sets, historical settings and alternate Ages | Existing flood/isolation browser journey retained. Full planetary/geographic depth is not implied by the bounded regional implementation. |
| P3 | 32 connected domain templates, numerical/qualitative quantities, relationship dossiers, role/direction-filtered family graphs, historical context, work/chapter/arc/scene planning | Connected-domain and historical isolation tests are present; a three-Age reference demo is reproducible. Domain depth beyond basic authored fields remains in the plan. |
| P4 | Local Git/HTTPS remote operations, staged clone, semantic merge review, native revision review/restoration, native backup/import/migration, CSV/JSON/Markdown preview into a new Age, consistency exceptions, selected text/map/codex exports, package and Docker build definitions | **Release gate open.** Large-world performance, full accessibility audit, all storage-failure/merge cases, streaming large archives, arbitrary Git branch/revision management and production packaging/deployment verification remain. |
| P5 | Contextual sound rules, phonotactic checks, inflection previews, authored production networks with exact inventories, bounded succession/trait/rate/route/climate/terrain experiments, parented local drawings, tactical tokens/grids, storyboards, EPUB/DOCX/Fountain export | **Depth gate open.** Implemented calculators are bounded approximations with explicit limitations. Rich morphology/script tools, constrained trade/logistics, advanced terrain/ecology and high-fidelity document interchange are not complete. |
| P6 | Offline searchable allowlisted editions and maps, explicit hosted accounts/world roles, password/session management, presence/comments, durable Yjs manuscript drafts, compacted checkpoints and interrupted-journal recovery | **Operational gate open.** World-level permissions and two-author editing are tested; broader multi-account/long-running offline journeys, independent validation of Yjs payloads, deployment hardening/quotas and more complex sync histories remain. |

## Evidence checked into the repository

- P1/P2 regression and acceptance browser journeys.
- Connected domain/quantity/scene planning and native archive import browser tests.
- Two simultaneous browser authors merging text and saving a canonical shared revision.
- Keyboard local-map authoring, Age-copy preservation, standalone publication search with external networking unavailable, and CSV preview/acceptance browser tests.
- Go tests for domain/history isolation, exact calculations, cyclic/multiple-role genealogy, local-map validation, privacy allowlists, hosted permissions, durable shared drafts, external-edit conflicts and real fetched Git merges.
- A genuine format-1 archive fixture with old snapshot versions, source revision/scene pins and raster assets, migrated only in a copy.
- Reproducible `npm run demo`, `npm run benchmark`, dependency inventory and main/PR GitHub Actions definitions.

Tests report their own current outcomes. Configuring a GitHub Actions job does not establish that it has run remotely. The Docker daemon being unavailable locally is not a successful container test.

## Performance work still required

The current snapshot index is monolithic, state responses materialize an Age, and validation can revisit substantial historical data. A bounded verified-byte cache now avoids rereading unchanged small objects while checking file identity/size/mtime and decoding independent values. This is an optimization, not a substitute for the planned lazy shards, paginated APIs, viewport tiling, job cancellation and derived indexes.

The initial Windows baseline with **1,000 identities and five Ages**, under concurrent development/test load, measured state-read p95 **303 ms**, search p95 **398 ms**, and Age-copy samples **1.245–2.062 s** before the cache optimization. It contained no relations, events, terrain or media. These numbers do **not** satisfy or represent the planned 50,000-identity/200,000-relation/2-GB-media reference world. The benchmark script writes each current run to `.tools/benchmark-latest.json`; retain workload and environment information when comparing runs.

A subsequent run with verified-byte caching measured state-read p95 **109 ms**, search p95 **138 ms**, and Age-copy samples **0.572–0.953 s** on that small workload. Background load differed, so this is preliminary evidence rather than a controlled performance guarantee. Per-validation reuse of shared snapshots is being checked separately.

With shared-snapshot validation reuse and indexed search pagination, the same small fixture measured state-read p95 **124 ms**, a cold indexed search **119 ms**, warm first-page search p95 **13 ms**, and Age copies **0.382–0.388 s**. Paged search returns 100 records; the earlier search returned all matching records, so the search figures are different workloads and should not be presented as a like-for-like speedup. Indexes are derived and bounded; ordinary Age state still materializes all records.

The browser suite now includes search pagination and two hosted browser sessions exercising password rotation, viewer enforcement and session revocation. Backend tests also cover account-store writer exclusion, failed password-save rollback, repeated Git synchronization through both native revision parents, and archive migration of that merge ancestry. Ambiguous criss-cross merge bases stop for reconciliation rather than selecting a base silently.

At **10,000 identities and five Ages**, the first run measured state-read p95 **1,089 ms**, cold search **1,072 ms**, warm search-page p95 **19 ms**, and Age copies **12.966–13.437 s**. Reusing unchanged decoded objects within each validation pass reduced copies to **3.556–3.656 s** in a subsequent run; state-read p95 was **1,127 ms**, cold search **1,126 ms**, and warm search-page p95 **19 ms**. This fixture still has no relations, events, terrain or media. Validation caches are cleared between commands; a regression test modifies an older historical object and verifies that a later save rejects it. Shared assets are streamed through their integrity check once per validation pass.

Browser journeys cover native restoration without losing intervening saves, entity/property/relationship/note/type/dossier/scene-plan/storyboard/recipe draft recovery, Unicode and right-to-left manuscript persistence, hosted password rotation, offline catch-up after shared-log compaction and comments on newly created shared paragraphs. Separate ordinary browser tabs retain independent entity/manuscript recovery copies, including when a stale write fails after another tab saves. The offline administrator recovery CLI preserves unrelated accounts/roles and refuses to run against a live hosted writer.

Production recipes specify consumed and produced resources, exact per-batch amounts, matching units, planned whole batches and priority. Experiments run at most 100 periods and 500 selected recipes/resources. Earlier outputs can supply later recipes within a period; a cycle returns to an earlier recipe in the following period. Unknown inventories and unit mismatches stop calculation. Preview does not change canon; the UI accepts linked balances together. Exact fraction/cycle, invalid input, stale preview, recovery and source-Age isolation tests accompany the feature. This is an ordered inventory calculator, not an equilibrium solver or an inferred economic simulation.

Native HTTP backups and the backup CLI now stream verified objects into ZIP output; HTTP downloads are staged privately until verification succeeds. Uploads and expanded ZIP entries are streamed to staging files. Error paths discard partial temporary data. The 256 MiB/100,000-entry quotas and migration graph limits remain; larger archive support is still an open gate. The small in-memory helper remains for tests and existing internal callers.

Current native archive/import/merge memory quotas are 256 MiB, terrain generation is 64×48, experiments accept at most 500 selected inputs, and relationships display bounded subgraphs. These are visible development limits to lift through measured architecture work, not hidden claims of massive-world support.

## Invariants retained

Creative decisions stay with the author. No AI/LLM provider, inference runtime, autonomous plot/world agent or model download was added. Calculators require explicit inputs and acceptance. Later-Age edits cannot rewrite their source; scenes retain historical pins; import/migration preserves source files; public editions require explicit selection. The preserved P1/P2 previews use separate binaries, frontend exports, ports and libraries.
