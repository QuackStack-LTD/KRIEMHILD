# Current development build

KRIEMHILD remains a Next.js static frontend served by one Go process. There is no AI provider, model runtime, external creative service or separate collaboration service. Installed dependencies and local project files are sufficient for ordinary authoring.

## Run and inspect

From the repository, run `npm.cmd run setup`, `npm.cmd run build`, then `npm.cmd start`. Use `npm` outside Windows PowerShell. Open `http://127.0.0.1:4784`. The default paths are `bin/kriemhild-dev.exe`, `apps/web/out-dev` and `worlds-dev`. The older P1/P2 binaries, exports and libraries remain separate.

`npm.cmd run demo` creates a new, disposable three-Age reference world. It includes connected examples for all 32 domain templates, a historical scene, an adoptive relationship, a coast, a biome region and a gatehouse plan. Running the command again creates another world; it never overwrites one. This small fixture is a workflow example, not the large performance fixture in PLATFORM_PLAN §25.

## Societies and writing

**Societies & systems** offers optional templates for settlements, populations, cultures, polities, institutions, law, classes, beliefs, languages, dictionary entries, names, rulebooks, technology, resources, recipes, routes, currencies, people, dynasties, warfare, climate, biomes, species and writing structures. Authors can continue using their own entity types. Quantities explicitly distinguish unknown, qualitative, exact decimal and range values. Text fields do not secretly run simulations.

Choose an entity to connect it with another using an authored role. **Families & connections** traces incoming, outgoing or both directions, optionally filtered by role and historical tick. Adoption, disputed claims, multiple parents and cycles remain explicit links. Graphs stop at eight steps, 200 identities and 500 displayed links; an equivalent named-link list accompanies the drawing.

**Story architecture** arranges scenes independently of world chronology and connects them to works, chapters and arcs. Goal/conflict/outcome, foreshadowing, payoff, knowledge and research remain planning fields. POV and location resolve against the scene's pinned historical setting, not the currently edited Age. Prose is never automatically rewritten.

**Storyboards** arranges image panels and author-written shot/framing/action/dialogue/caption fields. Upload PNG/JPEG/GIF images, save panels and reorder them. Publication exports include selected panel images and their public description; other planning properties remain excluded.

Unsaved panel changes disable workspace switching and panel reordering. Browser drafts survive failed saves and are offered for explicit review after a reload. If the saved panel changed meanwhile, the recovery notice says so. Discarding a draft is explicit; recovery does not silently overwrite canon.

Domain dossiers and scene plans have the same navigation guards and explicit browser recovery. To recover an unfinished new dossier, choose its template and **New domain entity** again. It is saved as a new entity after review. Scene-plan recovery applies planning fields while preserving the saved manuscript text.

Generic entities, unfinished custom properties, relationships, research notes and custom type definitions also retain browser drafts and guard navigation. Reopen the same record or new-record form after a reload to review its draft. Recovery slots are scoped to the browser tab's session, including ordinary manuscript drafts, so a save in one independent tab does not erase another tab's recovery copy. Browser storage is a recovery aid, not a backup; copying/duplicating a browser tab's entire session and recovery after clearing browser storage are not guaranteed.

Renaming a custom type shows the number of affected entities and saves their type names with the definition as one undoable operation. Existing values must validate under the new fields before any canonical change is committed. Other Ages retain their definitions and entity types. This does not perform automatic field-value conversions.

Search starts in the selected Age. **Search all Ages** groups matching records on the current page by identity and labels each result with its Age and derivation. Opening a result first loads that Age. Pages return at most 100 matches and inspect at most three Ages; continue through an empty page if more Ages remain. Search cursors reject changed world revisions. This searches Age overviews, not each dated state within an Age.

## Maps and optional experiments

Atlas retains seeded terrain, direct editing proposals, drainage, authored rivers, routes, lakes and overlapping political boundaries. Climate and biome polygons link to existing domain entities. Authored regions are not a computed climate model.

**Local maps & tactics** uses an existing blank/image map. Set a parent map and anchor, physical map width/unit, square/hex/no grid, and floor. Add room polygons, walls or paths using clicks or numeric point controls. Arrange multiple tokens linked to existing entities, with a floor and facing. Save the drawing explicitly. A token move never automatically resolves combat, changes a character or creates history. Age copying preserves drawings and tokens independently.

**Experiments** pins its input revision. Preview results, inspect warnings and accept selected record proposals as one undoable change, optionally with an exact dated event. Changed input revisions invalidate acceptance.

- Sound changes use ordered token rules such as `p > f / # _ V`, with classes such as `V = a e i o u`; blank replacement deletes a token. Context is one token/class on either side. `#` denotes a word boundary.
- Phonotactics compares a whole token sequence with explicitly listed patterns. Inflection previews concatenate author-supplied prefixes/stems/suffixes and can save a labelled paradigm. Neither invents vocabulary or grammatical rules.
- Production computes a constant-rate balance with exact rational arithmetic. Transport finds a directed, nonnegative, same-unit least-cost path with a 1000-node budget.
- **Economy & production** authors recipes from resource dossiers: consumed/produced quantities, units, whole batches per period and priority. Choose **Recipe network inventories** in Experiments, select saved recipes and preview 1–100 periods. Stocks must be exact and nonnegative, and units must match. The calculation runs in priority/name order, exposes shortages and accepts every linked stock change together. Earlier outputs can supply later recipes; cycles wait for the following period. Recipe drafts have explicit browser recovery. No labour, unit conversion, storage or transport constraints are inferred.
- Succession lists candidates reachable through selected parent roles. A custom two-allele inheritance calculator reports probabilities without choosing a character's traits.
- Climate offsets update exact temperatures in matching units; terrain smoothing is a bounded geometric approximation. Neither predicts societies, habitat migration or ecological consequences.

## Backups, import and export

**Project & export → Download native backup** contains the entire world, saved revisions, private records and media. It is not a public edition. Shared drafts must first be checkpointed with **Save shared revision** to enter portable canonical history.

Native import validates and previews a ZIP before making a world visible. Acceptance creates a new world ID while retaining Age/entity identities and rewriting necessary historical hashes. Format-1 snapshots, scene pins, source revisions and undo/redo links migrate only in the copy. Discard removes the preview's staging directory. HTTP uploads, expanded entries and native backup output stream through private temporary files; a download is served only after hash verification succeeds. The CLI streams directly to its exclusively created output file and removes it on failure. Current ZIP expansion/input budget remains 256 MiB/100,000 entries; larger archive validation remains an open release gate.

To back up an older project folder without opening it in the current writer:

```powershell
.\.tools\go\bin\go.exe run ./cmd/kriemhild-tool -source "C:\My Worlds\<world-id>" -output "C:\Backups\world.zip"
```

Or use the packaged `kriemhild-tool` binary. The output must not already exist. The tool captures the immutable head and copies its object files; it does not rewrite the source or take over its writer lock.

Text import supports up to 500 entries and 2 MiB of UTF-8 source per preview. CSV requires `name`; `type`, `kind`, `notes` and `id` are recognized, with other columns becoming text properties. JSON accepts an array with `name`, `type`, `kind`, `notes`, `id` and `properties`; unknown top-level entry fields are rejected rather than discarded. Entities and notes get new identities, and source IDs/provenance are retained. Entity references must use existing IDs; matching names are never silently merged. Markdown/plain text becomes one scene with its syntax preserved literally. Acceptance creates a new Age atomically.

Selected exports include HTML codex/story bible, an offline searchable publication, Markdown, EPUB, text DOCX/Fountain, native planar-map JSON and SVG map images in a ZIP. Native archive is the lossless format. Text exports omit comments and planning properties; DOCX/Fountain currently carry text rather than full editorial formatting. Fictional XY map data is not labelled WGS84 GeoJSON.

Public editions only contain checked entries. Linked private dossiers and old Ages are not traversed. Map pins and linked features require their entity to be selected. Uploaded background images/panel images are indivisible: review any annotations already baked into the pixels. Publication search runs entirely inside the exported HTML; a fixed script is permitted by its CSP hash. Public map floors can be filtered without a server.

## Git and consistency

Git is optional. Initialize a repository, commit, configure an HTTPS origin, fetch `main`, inspect semantic changes/conflicts, explicitly apply the reviewed merge, then push if desired. Credentials belong to Git's credential helper, never world files or the remote URL. These buttons perform the named remote actions; development tests use only temporary local repositories.

From the library's import card, **Clone an existing world from Git** downloads `main` into an isolated preview without checking out remote files. Only the native project files are materialized after validation. Acceptance preserves the world ID and Git history; duplicate world IDs in the same library are rejected. Hosted cloning is restricted to the administrator. A Git operation has a 45-second timeout, and the expanded working-file archive is limited to 256 MiB/100,000 files; a total download/disk quota is not yet enforced.

Fetch does not move the active project head. Merge preview reads an isolated temporary copy and compares logical fields against common project ancestry. Disjoint fields combine; conflicting names, arrays, prose or geometry need an explicit mine/fetched choice. Validation must succeed before a project revision is written. The Git merge commit records both parents. No network push happens as a side effect of merging. More complex merge/recovery cases remain tracked in the phase status.

New merges also retain both parents in the native project revision graph. This ancestry survives native archive import, and subsequent synchronization uses the latest unique common revision. Fetching an already included revision makes no additional save. Ambiguous criss-cross histories are rejected for explicit reconciliation; they are not flattened automatically.

**Project & export → Saved revisions** lists native saves, including history carried by a cloned repository. Review the Age/record counts before restoring a complete saved world. Restoration creates a new revision with the prior active head as its parent, so later work remains restorable. It does not move or push Git HEAD: review and commit the restored working files separately. Native history traversal currently has a 10,000-revision safety budget. This is world-content restoration, not arbitrary Git branch/reset management.

**Consistency** reports explicit facts: duplicate names, submerged pins, unresolved/renamed/destroyed scene references, incomplete routes/recipes, and orphaned comments. Exceptions need a reason and are tied to the finding's input fingerprint. Warnings never repair fictional canon automatically.

## Hosted access and shared writing

Local mode needs no account. Hosted mode is explicit and uses the same project format. For a local authentication trial, choose a fresh library and port:

```powershell
$env:KRIEMHILD_ADMIN_PASSWORD = '<choose a password of at least 12 characters>'
npm.cmd start -- -data worlds-hosted -addr 127.0.0.1:4787 -origin http://127.0.0.1:4787
```

For remote hosting, use an HTTPS origin behind a TLS reverse proxy that preserves the configured Host. Non-loopback HTTP origins are rejected. Bind the backend as needed, e.g. `-addr 0.0.0.0:4784 -origin https://worlds.example.org`. The application itself currently terminates HTTP, not TLS. Configure the public hostname explicitly; forwarded headers are not trusted to change it.

The first hosted start creates `admin` from `KRIEMHILD_ADMIN_PASSWORD`. Accounts and world roles are stored outside world archives in the library's `.access.json`. Passwords use salted PBKDF2-SHA256; sessions are HttpOnly/SameSite cookies, Secure for HTTPS. Owners manage world membership and Git; editors change world content; viewers read/export. The global administrator can access all worlds. Use one server writer per library; multi-instance shared hosting and quotas are not complete.

Create accounts and assign owner/editor/viewer roles in **Workspace access**. In **Shared writing**, authors edit a Yjs document through the Next frontend, with the Go server durably retaining update logs over HTTP polling. Presence and scene/paragraph comments are available. **Save shared revision** checkpoints merged prose into canonical Age history. Concurrent structured edits continue to use optimistic revisions.

**Password and sessions** lets each account change its own password after entering the current password. Administrators can reset another account's password after reauthenticating. A successful change signs the affected account out everywhere; a failed save preserves its credentials and sessions. Changing the bootstrap environment variable does not reset an existing administrator. The hosted account store holds an exclusive process lock.

For a forgotten administrator password, stop the hosted server, set `KRIEMHILD_RESET_PASSWORD` in the operator's environment, and run the packaged `kriemhild-admin -data <existing library> -user admin` (use `.exe` on Windows). From source, use `go run ./cmd/kriemhild-admin` with the same arguments. Remove the temporary environment variable afterward and restart the server. The tool refuses to run against a live hosted writer, does not create missing accounts, and preserves other accounts, memberships and worlds. Passwords are not accepted as command-line arguments or printed. The tool requires filesystem access to the hosted library; it is not an unauthenticated HTTP recovery endpoint.

Age search returns up to 100 matches per page with **Previous results** and **Next results**. A changed query or snapshot restarts pagination. Its derived text index excludes internal collaboration metadata and stays within a bounded memory cache. Search pagination does not yet make the main Age state response lazy.

An ordinary editor changing the canonical scene causes an explicit shared-draft conflict instead of overwriting it. Browser recovery state is retained, and a shared draft can be saved as a separate scene for manual reconciliation. Native exports carry saved checkpoints, not uncheckpointed live logs. Read-only viewers cannot write updates or checkpoints. Each checkpoint compacts its update log to a full state while retaining sequence continuity for other browsers. A saved checkpoint whose journal write was interrupted can recover through an exact previous-document/sequence marker. The Go server still transports opaque Yjs bytes; it does not independently verify that those bytes encode the supplied canonical document. Advanced conflict resolution and broader hosted operations remain open gates.

## Packages, Docker and verification

`npm.cmd run package` creates a runtime folder under `dist/` containing Go executables, exported frontend, notices and `RUN.txt`. Set `KRIEMHILD_PACKAGE_DIR` to a new directory for another build. Run the packaged binary with `-web web -data worlds -addr 127.0.0.1:4784`. Linux/macOS downloads may need executable permission restored after extraction.

Docker is configured for a single Go runtime, bundled static frontend and Git, running as UID 10001 with `/data` persisted. A loopback-only hosted trial can use:

```text
docker build -t kriemhild:dev .
docker run --rm -p 127.0.0.1:4784:4784 -e KRIEMHILD_ADMIN_PASSWORD -v kriemhild-data:/data kriemhild:dev -addr 0.0.0.0:4784 -origin http://127.0.0.1:4784
```

Set the password environment variable before running. For a remote deployment, replace the loopback origin with the actual HTTPS origin and configure TLS at the reverse proxy. The local Docker daemon was unavailable during the initial implementation; the CI image-build job is configured separately.

Run `npm.cmd test` for Go tests/vet and TypeScript; build first, install Playwright Chromium, then run `npm.cmd run test:e2e`. Browser fixtures use isolated ports 4781 (local, `.test-worlds-dev`) and 4783 (hosted, `.test-hosted-dev`). Hosted test credentials are fixtures for the isolated loopback server only. `npm.cmd run notices` refreshes dependency inventory and license texts. GitHub Actions runs on main pushes and PRs targeting main, with Ubuntu/Windows browser checks, a Linux Go race job, Docker build and cross-platform package builds. No deployment or release publication is automatic.
