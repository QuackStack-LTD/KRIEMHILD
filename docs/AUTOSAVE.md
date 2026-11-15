# Terrain autosaving and deletion

Open a World and choose an Age first. **Save terrain** in the Terrain Generator accepts a new generation preview into that World and Age. Once attached, terrain edits, completed drawings, deletions, undo/redo, accepted settlement layouts and illustration design changes save automatically. The welcome page lists Worlds; terrain is accessed through its World.

Automatic saving applies in both local and authenticated hosted mode. Generating a replacement remains a preview until accepted. Name edits, publishing named records, finishing geometry, accepting a generated settlement layout and saving illustration images keep their explicit controls. Save illustration stores its image, banner, and design in the World's Library.

Naming or renaming an entity is an explicit record transaction, separate from terrain persistence. Names belong to stable logical IDs in the selected Age. Profiles, map representations, generated river labels, terrain titles, and ID-based references resolve the same canonical name. Active editors refresh on focus, cross-tab notifications, and lightweight revision checks. Pending geometry is retained, and undoing older geometry does not restore a name superseded by another editor. Earlier Ages keep their historical names.

The Water bodies panel identifies generated oceans, lakes and rivers from their immutable source IDs. Naming does not mutate the generated planet or save a terrain snapshot. Seas, bays and other subdivisions of connected water can have their own authored name anchors. Exported PNGs/PDFs and existing Library image versions are snapshots: open the illustration and save a new image to update baked-in labels. Linked interactive labels use the current records; deliberately customized label text remains an illustration override.

Medieval illustration titles and compass roses occupy a reserved parchment header above the geographic frame. Preview fitting and full-map illustration exports include this header. The banner derivative omits decoration and crops only the geographic frame. Geographic bounds and saved source geometry remain unchanged.

Terrain commands commit before reporting success. Camera and display settings save after 700 ms of inactivity; leaving the workspace flushes pending view changes, and hiding/closing the page sends a keepalive request. Normal navigation does not ask for confirmation. A real persistence failure remains visible and retains the unsaved warning for recovery. Inactive sessions can expire after 30 minutes; successfully saved edits survive reopening and server restarts.

Save terrain atomically commits the completed terrain, seed/configuration, elevation/bathymetry, hydrology/climate, authored objects and operations, undo history, cameras, and stored detail. Legacy archive checkpoint support can also restore an unfinished historical solver without regeneration.

ZIP export is a separate portable backup action and never writes to the database. Import opens an unsaved working world, preserving its structured data and explored tiles. Importing the same world ID stages a replacement; the existing database copy remains intact until Save. Saving that replacement atomically removes obsolete stored parts. Saved detail remains authoritative.

Delete world asks for confirmation and removes the project and all database parts, including explored terrain. Matching live sessions are retired and their temporary caches removed. Delayed requests cannot save the deleted world again. Exported ZIP files are unaffected. Cancelling the dialog changes nothing.

## Storage

PostgreSQL remains supported externally; SQLite is the embedded default. No additional database is needed. Schema 3 uses relational catalog metadata and independently compressed JSON records in `kriemhild_world_parts`:

- `checkpoint`: full running solver, configuration, generation request, base snapshot and engine edits/history.
- `environment`: geographical, geological, climatic and hydrological fields and entities.
- `detail-model` and `tile/<level>/<x>/<y>`: deterministic hierarchy and actual explored detail.
- `view`: generator controls and camera state.
- `builder/header`, `builder/entity/<id>`, `builder/operation/<id>`: authored project, layer state, objects and terrain modifications.

Only dirty records and newly explored tiles are written on subsequent saves. Map writes read geographic records from the selected Age, bypassing generated species and biosphere tables. Audit revisions store geographic patches and expand them when inspected/restored. A failed transaction preserves the previous saved world and reports an error instead of claiming success.

Edit responses contain changed drawings and deletion tombstones. The client merges these without reloading every entity or generated feature. Raster changes invalidate only intersecting cached tiles; drawing-only changes retain the existing terrain frame. The current inspection and resource report refresh automatically. Resource circles show their spherical observed area in km² immediately while resizing.

`POST /api/sessions/{id}/save` remains an explicit retry/commit endpoint. `POST /api/sessions/{id}/world`, `/view` (also `/autosave`) and settlement acceptance automatically commit attached continuous terrain. Unattached generation previews and legacy archives retain staging semantics. `DELETE /api/projects/{id}` deletes a complete world, while `DELETE /api/sessions/{id}` closes only a temporary session.

The application currently has one shared project library and process-local live sessions. Opening an already active world reuses its working session; after a server restart, opening reads the last committed version.

## Validation

Tests cover automatic drawing/deletion/undo/redo and view persistence, draft publication boundaries, biological-data isolation, geographic audit recovery, local raster reuse, spherical circle areas, legacy checkpoint recovery, view sequencing, database failure/retry, deletion with retired writers and portable imports. `BenchmarkTerrainMapAutosave` compares complete World updates against geographic snapshot and entity transactions.
