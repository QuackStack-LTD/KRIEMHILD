# P1: the first creative workspace

P1 connects local worlds, independent Ages, structured entities, image maps and writing. It implements the first usable slice in the platform plan. It does not claim that P2 terrain generation or the P4 serious release is complete.

## What is implemented

| P1 capability | Available behavior |
|---|---|
| Create/open local world | Create a world and first Age; reopen from the library or a bookmarked URL |
| Contextual Next interface | World/Age navigation, overview, entities, atlas, writing, history, notes and custom types |
| Stable entities | UUID identity; name, freeform type, lifecycle, notes and custom properties |
| Custom entity types | Text, number, yes/no and entity-reference field definitions, validated on save |
| Relationships | Named directed links between entities; reverse display in either dossier |
| Image map | Import PNG/JPEG/GIF, or create a blank map; zoom and scroll |
| Entity-linked pins | Place/move by click or numeric percentages; list view and links to dossiers |
| Scene writing | Rich text, basic formatting, typing undo, story grouping, order, word count and autosave |
| Scene settings | Pin an Age snapshot; reference its entities; explicitly retarget to the current Age |
| Research notes | Named notes stored in each Age |
| Search | Case-insensitive search of saved record content in the current Age, including manuscript text |
| Copy Age | Complete independent copy, including schema definitions, maps, pins, scenes and notes |
| Comparison | Added/removed/changed records with before/after content and full saved fields |
| Source corrections | Three-way record comparison against the original source; explicit selected replacement with conflict warnings |
| Undo/redo | Persistent per-Age content history; undo/redo are new saved project revisions |
| Recovery | Immutable files, flushed journal and atomic head replacement; interrupted-save recovery and browser scene drafts |
| Concurrent access | One writer process per project; stale browser edits rejected by revision checks |

## Manual acceptance walkthrough

1. The preserved P1 preview is at `http://127.0.0.1:4780`. Current source builds launch P2 at `http://127.0.0.1:4782`; the P1 workflows below remain available there. See [P2 guide](P2_GUIDE.md) for separate libraries and format compatibility.
2. Create **The Glass Coast**, with **Age of Rivers** as its first Age.
3. Open **Entity types**. Create type **Port**, with a numeric **Population** field.
4. Open **Entities**. Create **Valer**, type **Port**, population **1200**, with a short description.
5. Create **Mira**, type **Person**. In her relationships, enter **lives in**, choose Valer and click **Link**.
6. Open **Atlas**. Name a map **Northern coast** and optionally upload a PNG/JPEG/GIF. Choose Valer in **Entity to place** and click the map, or enter X=30 and Y=40 and use **Place / move pin**.
7. Open **Writing**. Create **Arrival** and write a paragraph. Wait for **Saved on this device**. Add Valer to the scene's referenced entities. Its contextual notes come from the scene's pinned setting.
8. Use **Begin a new Age**, naming it **Age of Ash**. All saved records initially match Age of Rivers. The manuscript keeps its original setting binding.
9. In Age of Ash, rename Valer to **Velar**, mark it destroyed and change its population to zero. Move its map pin and rewrite the scene. Wait for saving to finish.
10. Open **History → Compare Ages**. Compare Age of Rivers with Age of Ash. The entity, map and scene changes are visible.
11. Use **Undo** and **Redo** to verify the last saved change. These affect the selected Age, not its source.
12. Switch back to Age of Rivers. The original name, location, population and manuscript are unchanged.
13. Search for a phrase from the original manuscript. Search is scoped to the selected Age.
14. Stop the server, restart it, and reopen the world. Both Ages, their data and undo/redo state remain available. Refresh a bookmarked URL containing `world`, `age` and `view` to verify deep linking.

For source-correction review: edit a record in the source Age after copying, return to the descendant and open **History → Review source corrections**. A conflict is marked if both Ages changed the same record. Selecting a correction replaces the whole descendant record with the source record; no record is preselected. Applying a correction is undoable. This does not change the recorded original inheritance baseline.

## Saving and recovery

Entity/type/note forms have explicit Save buttons. Manuscripts save automatically after an 800 ms pause. The editor first writes a browser recovery draft and removes it only after Go confirms persistence. While a scene has pending changes, Age copying and general navigation are held until it saves; the browser also warns before closing an unsaved scene.

If a scene save fails, its status states that it was not saved. On reopening the same scene in the same browser/origin, choose **Restore draft for review** or **Discard recovery draft**. Restoring requires an explicit **Save scene now**. If a stale-write error occurs, use **Reload saved state** in the error banner, inspect the preserved draft, and decide whether to save it over the newly loaded state. Nothing automatically merges or overwrites another tab's newer data.

Browser storage may be unavailable or full. The editor reports that condition and must stay open until the backend confirms saving. Recovery drafts are not backups and do not travel with the project folder.

The Go process owns a project lock until it exits. A second process trying to write that project is rejected. Multiple tabs in the same process can read; a save from an outdated project revision receives HTTP 409. The local service does not support active projects on network shares or simultaneously edited cloud-sync folders.

An interrupted multi-file commit leaves either the previous complete state or the next complete state. If the head was damaged during a journaled commit, the service can restore the validated previous root. Unexplained corrupt object hashes or unsupported project formats block opening rather than silently discarding data. Retain the original folder when investigating corruption.

For a P1 backup, stop the server and copy the complete world folder to another location. Restore it into the chosen library with its original UUID folder name. Automated backup/export/import interfaces are P4 work. Do not edit hash-addressed files directly.

## Automated checks

`npm test` runs the Go suite, vet and the TypeScript checker. The tests cover:

- Full-copy equality and isolation after edits to entities, schemas, relationships, maps, notes and scenes.
- Source edits that do not propagate to descendants.
- Reopening both Ages, retained assets and persistent undo/redo.
- Rejection of stale writes, invalid field values, dangling links, corrupt hashes and competing writer processes.
- Pinned scene references surviving removal of an entity from the current Age.
- Source-correction conflict detection, explicit application and undo.
- Simulated write failures and abrupt subprocess termination before/after journal and head updates.
- API sessions, hostile origins/hosts, invalid paths, image upload restrictions and static deep links.

`npm run test:e2e` runs browser regression tests against the compiled application. Tests create isolated worlds in `.test-worlds` and cover the complete author journey, failed-autosave draft recovery, custom types/source conflicts and two-tab concurrency. No internet request is needed by the running app; dependency and browser downloads belong to setup.

The same checks run automatically through [`.github/workflows/verification.yml`](../.github/workflows/verification.yml) for pushes to `main` and pull requests whose target branch is `main`. Independent Ubuntu and Windows jobs install locked dependencies, build the application, run Go tests/vet and TypeScript checking, install Chromium, and run the P1 and P2 browser acceptance tests. Jobs have a 20-minute timeout, and failure on one operating system does not cancel the other.

In GitHub's **Actions → P1 verification** run, download `p1-browser-results-ubuntu-latest` or `p1-browser-results-windows-latest` for the browser report, JUnit results, and failure screenshots/traces. Artifacts are retained for seven days. `test.only` is forbidden in CI. These workflow files take effect after the changes are pushed to GitHub; local verification does not establish that a hosted run has already passed.

## Development and API

The default workflow is edit source, stop the running Windows executable, run `npm run build`, then restart. The production frontend is a static Next export served by Go. The standalone Next development server is not configured as an API proxy, so use the Go-served build for integrated testing.

Main code locations:

- `internal/project`: record validation, immutable snapshots, revisions, commands and recovery.
- `internal/httpapi`: same-origin local sessions, project library, query/command routes and images.
- `cmd/kriemhild`: local HTTP server and command-line configuration.
- `apps/web/app`: Next shell and styling.
- `apps/web/components`: entities/types/notes, atlas, writing and comparison.
- `apps/web/tests`: browser acceptance tests.

API base: `/api/v1`. Start a local cookie session with `GET /session`. The cookie is HttpOnly and SameSite=Strict; cross-site requests and non-loopback hostnames are rejected. All other API calls require the cookie.

| Route | Purpose |
|---|---|
| `GET /projects` | List worlds in the configured library |
| `POST /projects` | Create `{name, age}` |
| `GET /projects/{id}/state?age={age}` | Current root, selected Age and resolved records |
| `POST /projects/{id}/commands` | Mutation with `expected` project revision and `age` |
| `GET /projects/{id}/search?age={age}&q={query}` | Search saved records |
| `GET /projects/{id}/compare?from={snapshot}&to={snapshot}` | Read-only record comparison |
| `GET /projects/{id}/corrections?age={age}` | Three-way source correction candidates |
| `GET /projects/{id}/snapshots/{hash}` | Resolve a pinned setting |
| `POST /projects/{id}/assets` | Raw PNG/JPEG/GIF upload; returns hash and dimensions |
| `GET /projects/{id}/assets/{hash}` | Read a validated image asset |

Commands: `put`, `delete`, `copy-age`, `rename-age`, `undo`, `redo`, `apply-corrections`. Invalid mutations do not move the project head. Images are limited to 15 MB, 40 million pixels, and 20,000 pixels per dimension. Manuscript/API JSON requests are limited to 8 MB.

## Current boundaries

P1 is an editable overview snapshot per Age. Date-specific state reconstruction, calendars, events, generated terrain and the twelve specialized domain workspaces follow in P2/P3. Current map comparison shows saved record/pin differences; geographic overlay comparison belongs to P2.

Custom fields are a deliberately small first schema system. Relationships are directed pairs; richer multi-participant relations and historical validity arrive later. A scene has a work name and numeric order; full works/chapters/arcs, rich export and publication are later phases. Rich-text block IDs are retained; comment anchoring is not implemented yet.

Search scans the current snapshot, and snapshots use full record manifests. SQLite indexes, sharded manifests, background jobs and large-world benchmarks are not implemented in P1. Integrity checks prioritize correctness over large-world throughput. All saved revisions are retained; there is no garbage collection yet. Storage usage grows as scenes are revised.

Git synchronization, Docker packaging, hosted permissions, collaboration, automatic backups, format migrations and broad import/export are not part of P1. Unknown format versions are refused without migration. Only custom-property payloads, not arbitrary future core schema extensions, are supported for editing. No AI/LLM dependency or creative provider integration is included.
