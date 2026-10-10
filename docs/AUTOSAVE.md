# Explicit world saving and deletion

Worlds are stored in the database **only when the user clicks Save** in the application header. The same Save and Delete world controls are available from the welcome page, World Generation and World Builder. Recent Worlds also offers deletion on each saved project.

Generation, editing, camera changes and exploration update a temporary server session. The interface shows Unsaved changes. Returning to Projects retains the active working world; opening another world warns before discarding changes. Closing the page warns about unsaved work. Inactive sessions can expire after 30 minutes; unsaved work is not durable across a server restart.

Save flushes pending editor/view updates, waits for requested explored tiles, and atomically commits the world: solver state, seed/configuration, elevation/bathymetry, hydrology/climate, authored objects and operations, undo history, cameras, detail model and all explored tiles and ancestors. Saving pauses active generation so the checkpoint is consistent. Unfinished worlds reopen paused and resume from their saved solver/RNG state without regeneration.

ZIP export is a separate portable backup action and never writes to the database. Import opens an unsaved working world, preserving its structured data and explored tiles. Importing the same world ID stages a replacement; the existing database copy remains intact until Save. Saving that replacement atomically removes obsolete stored parts. Saved detail remains authoritative.

Delete world asks for confirmation and removes the project and all database parts, including explored terrain. Matching live sessions are retired and their temporary caches removed. Delayed requests cannot save the deleted world again. Exported ZIP files are unaffected. Cancelling the dialog changes nothing.

## Storage

PostgreSQL remains supported externally; SQLite is the embedded default. No additional database is needed. Schema 3 uses relational catalog metadata and independently compressed JSON records in `kriemhild_world_parts`:

- `checkpoint`: full running solver, configuration, generation request, base snapshot and engine edits/history.
- `environment`: geographical, geological, climatic and hydrological fields and entities.
- `detail-model` and `tile/<level>/<x>/<y>`: deterministic hierarchy and actual explored detail.
- `view`: generator controls and camera state.
- `builder/header`, `builder/entity/<id>`, `builder/operation/<id>`: authored project, layer state, objects and terrain modifications.

Only dirty records and newly explored tiles are written on subsequent Save operations. A failed transaction preserves the previous saved world and retains unsaved session data for retry. Edits and ZIP export continue to work without a database connection; Save reports a failure instead of claiming success.

`POST /api/sessions/{id}/save` is the explicit commit endpoint. `POST /api/sessions/{id}/view` stages display state; the legacy `/autosave` endpoint is a staging-only compatibility alias. Builder `{kind:"save"}` is also an explicit commit. `DELETE /api/projects/{id}` deletes a complete world, while `DELETE /api/sessions/{id}` closes only a temporary session.

The application currently has one shared project library and process-local live sessions. Opening an already active world reuses its working session; after a server restart, opening reads the last committed version.

## Validation

Tests cover draft-only generation/editing/exploration/import/export; explicit checkpoint recovery across restart; explored tile fidelity; view sequencing; database failure/retry; deletion with cascading parts and retired writers; and imported replacement without stale data. The container smoke script explicitly saves before its restart/recovery check.
