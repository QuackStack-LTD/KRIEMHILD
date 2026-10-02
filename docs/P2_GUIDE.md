# P2 — first atlas and temporal slice

This implements the roadmap's connected flood/history acceptance journey. P2 remains in progress: this is a bounded regional atlas and temporal foundation, not the full geography system described in the platform plan.

## Running alongside P1

The existing P1 process remains on `http://127.0.0.1:4780`, serving `apps/web/out`, using `bin/kriemhild.exe` and `worlds/`. None of those files are replaced by a P2 build.

P2 uses `http://127.0.0.1:4782`, `apps/web/out-p2`, `bin/kriemhild-p2.exe` (without `.exe` on Unix), and `worlds-p2/`. Session cookies are scoped by application generation and port. Both instances can be used in one browser.

From the repository root:

```powershell
npm.cmd run setup
npm.cmd run build
npm.cmd start
```

Use `npm` outside Windows PowerShell. Build requires Node and Go; runtime needs only the Go executable and exported frontend. If P2 is already running, open its address directly. Stop only P2 before rebuilding its executable on Windows. Do not stop P1 to build P2.

New P2 projects use storage format 2. P2 refuses format-1 projects rather than changing their history or silently dropping fields. P1 refuses format-2 projects. A reviewed copy/migration tool is outstanding; continue testing existing P1 worlds in P1.

## Available tools

### Atlas

- Import a map image or create a blank map, then generate terrain from a user-selected seed. The generator runs in Go, stores the accepted elevation samples, and identifies its algorithm as `regional-grid-v1`.
- Generate a 64 × 48 regional grid with optional east/west wrapping. Relative elevation units make no assumption about Earth's scale.
- Raise, lower, smooth, flatten, flood or drain a circular region; change the sea level. Enter a brush position or click the terrain preview map. Preview generation and every edit before accepting. Discard changes without saving.
- Recompute water connectivity and strictly downhill drainage for the entire proposed grid. Boundary-connected water is ocean; enclosed water is distinguished as lake water. Drainage directions are an optional overlay, **not a complete generated river network**.
- Review settlement/entity pins that become submerged or exposed. Pin effects are advisory: settlement status, population and prose remain authored decisions.
- Author and edit rivers, routes, lakes and overlapping borders using percentage-coordinate vertices. Each geographic feature has a stable UUID, a name, an optional entity owner and independent geometry. Map regeneration preserves these authored features and pins. It does not force authored rivers to follow computed flow.
- Toggle terrain, water, drainage, authored rivers, routes, borders, lakes and pins separately.
- Accept an undated overview edit, or provide an event title, exact tick, optional validity end and explanation to save the terrain and event atomically.

### Timeline and chronology

- Explicitly pin the current snapshot as an Age's opening baseline. A start tick asserts that baseline from the start of the Age. No start means undated baseline records remain unresolved in historical queries.
- Set an optional exclusive end tick. Dates outside the configured Age interval produce an unresolved result, not interpolated history.
- Label branches canon, alternate, experiment or abandoned. The ancestry list shows the source Age and pinned source snapshot. Copying an Age preserves its saved map/domain state and creates a new chronology with the selected source checkpoint as baseline; enter the new Age's own dates. Inherited events retain their original Age ownership and do not replay as new events.
- Create narrative events on named tracks, filter tracks, and assert causes. Cyclic cause relationships are rejected. Exact, approximate, before, after, range, unknown and relative date descriptions are supported. Only exact events can assert state changes; uncertain events remain narrative and are explicitly identified as unresolved during inspection.
- Use arbitrary-precision signed decimal integer ticks, stored as strings and compared with Go `math/big` and JavaScript `BigInt`. Dates are never converted to JavaScript `Number` or Go `time.Time`.
- Define simple calendars with an epoch, named months, week length, year-zero policy and periodic leap days added to the final month. A tick is one day in this implementation. Negative years use Euclidean division. Leap cycles start at the epoch; every Nth year in that cycle receives leap days. Calendar definitions live in immutable snapshots.
- Inspect an exact date to see asserted entities and maps, with unresolved information shown explicitly.

### History and writing

- Compare saved maps side by side in History. Each side uses its own snapshot's entity labels. The existing record comparison still exposes all before/after fields and source-conflict review.
- Set a scene's date against its preserved setting snapshot. The context panel renders historical maps and offers entity references from that resolved setting. Changing the date clears the selected references for explicit reselection. Copying an Age preserves the scene's original setting Age, snapshot and date.
- The current-Age dossier remains a separately labelled action. Editing the overview, a calendar or another Age cannot change a scene's pinned historical setting.
- Undo/redo treats the terrain and its event as one operation. The existing immutable-object, journal, revision conflict and crash recovery protocol applies to P2.

## Temporal semantics

An opening baseline is an explicit assertion, not a guess about an earlier world. Undated edits after that baseline change the overview only. Date inspection starts from the asserted baseline, then overlays exact event changes whose half-open validity interval contains the query tick. At an assertion's exclusive end, that assertion no longer supplies a value; the baseline applies if it exists. Without a baseline or an active assertion, the value remains unknown.

Events store the actual before/after content hashes, explanation and affected identities. Recorded values cannot be rewritten through an event metadata edit. A new exact assertion for a target automatically closes a prior single-target open assertion at the new tick. Other overlapping intervals are rejected. End a compound event's validity explicitly before superseding only some of its records; per-field/per-target interval editing is future work.

All event changes in this slice are complete record states. A terrain event records the entire map, including its geographic features and pins. Deletion is an overview operation; use explicit lifecycle values when asserting a dated destroyed/abandoned entity. No automatic historical deletion or inferred consequences are performed.

## Acceptance walkthrough

1. Create a new P2 world and a settlement entity. Create a map and place the settlement at 50%, 50%.
2. Generate terrain from a seed and accept it. For an easily repeatable dry baseline, preview **Change sea level** to `-1000` and accept. The generated central region lies above that level.
3. In Timeline, set start `0`, end `100`, and pin the opening baseline. Optionally create a calendar and select it.
4. Begin a new Age. In its Timeline, set start `100`, end `200`; keep its alternate branch label.
5. In Atlas, preview **Flood region** at 50%, 50%, radius 25%, strength 300. The impact list identifies the settlement as newly submerged. Its dossier is unchanged.
6. Enter event title `The Great Flood`, tick `120`, and your explanation; accept terrain and record the event.
7. Compare the Ages in History. Inspect ticks `119` and `120` in Timeline: the map changes at the event, with the original Age unaffected.
8. Create a scene in the new Age. Apply scene setting ticks `119` and `120` and inspect the different context maps. Reload to verify persistence.
9. Undo a terrain/event operation immediately after saving it, then redo it. Both the map and event disappear/reappear together.

## Validation and CI

```powershell
npm.cmd test
npm.cmd --prefix apps/web exec -- playwright install chromium
npm.cmd run test:e2e
```

Build first when application code changed. Tests start a separate server at port 4781 and use `.test-worlds-p2/`; they do not use either preview library.

- [P2 Go tests](../internal/project/p2_test.go): deterministic generation, wrapping, drainage, preview non-mutation, flood impacts, stale proposals, copied-Age chronology, date boundaries, immutable source Ages, scene pins, atomic undo/redo, restart, huge ticks, invalid/overlapping intervals, uncertainty, causal cycles, negative/leap calendars, calendar pins and format-1 refusal.
- [P2 browser test](../apps/web/tests/p2.spec.ts): the flood/alternate-Age/history/scene journey using the user interface and checking the resulting saved data.
- All P1 storage, API and browser regression tests remain included.
- [GitHub Actions](../.github/workflows/verification.yml) runs the complete suite for pushes to `main` and PRs targeting `main`, on Windows and Ubuntu, retaining browser reports and failure artifacts. Local validation does not imply that the hosted workflow has run before these changes are pushed.

## Remaining P2 work

- Explicit P1-to-P2 project copy/migration with recovery and verification.
- Chunked terrain storage, larger-map rendering, measured scale limits, incremental water recalculation, river-network extraction, advanced terrain brushes and climate/biome generation.
- Dragging geographic vertices, richer layer styles and layer persistence, region/route impact analysis, map overlay/change-only comparison, and field-level difference summaries.
- A graphical ancestry board and causal graph, richer branch/date metadata presentation, dated entity-edit forms and per-target validity editing for compound events. The API already accepts a batch of map/entity/relation/note records in one event.
- Calendar definition editing/conversion workflows, irregular calendars, sub-day time units, named weekdays and more complex intercalation rules.

These limitations are deliberate boundaries of this first P2 slice; no AI/LLM participates in world generation, event explanations, historical resolution or writing.
