# Cleanup and UI audit — 12 October 2026

Implemented against `KRIEMHILD_cleanup_and_UIUX_task_brief.md` and the accompanying
requirement to preserve distinct page experiences.

## Findings and fixes

| Location | Finding | Resolution |
| --- | --- | --- |
| `src/main.jsx`, `src/style.css`, `src/App.jsx` | Generator typography and controls leaked into every workspace. | Terrain styles are scoped to `terrain-app`; shared tokens and controls load deliberately from `worldbuilding/ui.css`. |
| `src/application.css` | `.project-dialog > div` laid out ordinary body content as footer actions. | Removed the rule and legacy modal skin. Dialog header, scrolling body and actions are explicit. |
| `worldbuilding/Library.jsx` | Undeclared backdrop class and incomplete modal surface. | Shared accessible Dialog for archive creation, document creation and metadata editing. |
| `worldbuilding/Biosphere.jsx` | Preview habitat groups collapsed into a horizontal action row. | Stable-width/height wizard with four-step progress, vertical preview groups and fixed actions. Selected scope requires habitats. |
| `worldbuilding/ui/Dialog.jsx` | Modal behavior varied or was absent. | Focus entry/trapping/return, named headings, inert background, scroll lock, nested confirmations, Escape, draft guards, pending states and serialized submissions. |
| App, terrain controllers, profile and document workflows | Browser prompts mixed with styled overlays. | Async in-app confirmations, text prompts and notices. Browser reload/close keeps `beforeunload`, as required by browsers. |
| `worldbuilding/Library.jsx` | Async confirmation could lose the desired file-version selection. | Capture the selected version before awaiting confirmation. |
| Library and System Designer | Draft protection did not consistently cover routes and Age switches. | Shared asynchronous navigation guard; local record switches use the same guard without a second prompt. Writing and entity profiles also participate. |
| `worldbuilding/organizations.css` | Shared tabs were owned by one unrelated page. | One shared tab-control definition in `ui.css`. |
| Organization, lifeform, concept and system profiles | Inputs outside `.workspace` missed the control skin. | Explicit input types and scoped shared form states independent of page layout classes. |
| `src/history.css`, shared UI | Mobile labels were compressed into a 102px rail; Age controls crowded one another. | Readable mobile navigation menu and compact Age-bar controls; content retains footer clearance. |
| Library | Tree and content competed for mobile space. | Explicit Browse/Hide archives control on narrow screens. Native multi-select has keyboard instructions. |
| Library editors | Global control styling risked darkening document content. | Paper, spreadsheet and PDF presentation retained and checked separately. |
| Culture/Biosphere/System editors | Long fields, configuration sections and tables needed usable narrow layouts. | Responsive field pairs, local catalogue scrolling, shared editor-sized dialogs and retained specialist compositions. |
| Generator and hydrology report strings | Corrupted punctuation and squared/cubed units. | Correct north–south/east–west labels, km² catchment areas, m³/s flow and km³ sediment volumes. |

Atlas remains a tile navigator; Characters remains a relationship graph;
Organizations retains its directory, structure and roster workflow; Culture keeps
its conceptual article structure; Biosphere retains habitat/configuration panels;
Magic & Technology retains its outline/editor/consistency workbench; Library
retains archive browsing and format-specific file editors.

## Removal and retention evidence

- Removed `FutureWorkspace` and its App import: no active route used the export.
- Removed `AgeContents`: Atlas had replaced its only intended use; no call sites
  remained. Removed its unused `history-terrain-grid` and `age-entities` styles.
- Removed unused `relationshipClusters` import from Workspace; Characters delegates
  to Networks and no longer reads that import.
- Removed obsolete dialog backdrop/surface/layout selectors and duplicated shared
  tab styling after migrating their consumers, including the illustrated-map dialog.
- Removed CSS byte-order marks, normalized stylesheet/component formatting, and
  corrected stale placeholder documentation in `WORLDBUILDING.md`.
- Preserved Atlas/Characters exports, `flora` compatibility, legacy project routes,
  terrain controllers, ProjectHub, reference/parity fixtures, terrain galleries,
  the vendored spreadsheet package, lockfiles, container configuration and scripts.
- No terrain algorithms, world schemas, ZIP structures or existing world records
  were changed by this cleanup.

## Verification

Baseline was re-established in the provisioned local environment: all 103 existing
frontend tests, the production build, and Go history/storage/HTTP API/biosphere
suites passed. The old network/toolchain failures described in the brief did not
occur here.

The updated frontend suite passes 111 tests, including eight modal/UI regressions.
`npm run check:ui` and the production build pass. All ten browser tests pass,
including temporary archive/document creation, version saving, dirty version
switching, restoration of the latest file, and deletion of the test records.
Browser checks cover direct routes
and refreshes (including `flora`), navigation at 1440/900/390px, Library validation,
focus and dirty cancellation, Biosphere generation through populated preview,
creation dialogs, System Designer draft interruption, nested Culture fields and
variations, profiles, white document surfaces, and legacy generator/builder entry.

Screenshots are generated in `.tools/cleanup-screenshots/`. The shared T3 browser
was used first; its screenshot operation failed and its browser host later
disconnected. The final automated visual/workflow checks use local headless Chrome.
No missing screenshot was treated as visual evidence. Desktop, tablet and mobile
captures preserve the distinct compositions instead of applying a universal layout.

The browser suite requires an expendable populated fixture world; absent that
configuration it explicitly skips rather than reporting those workflows verified.
It is a focused regression suite, not an exhaustive test of all possible user data
or every format-specific document-editing operation.

See `UI-ARCHITECTURE.md` for composition boundaries and commands for future checks.
