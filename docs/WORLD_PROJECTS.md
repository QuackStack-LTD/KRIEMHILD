# Portable world projects

Use **View → Save → Save world ZIP** after generation finishes. **Open world ZIP** imports that archive; **Open world folder** imports its extracted `KRIEMHILD` folder. PNG/SVG buttons remain visual exports and are not project files.

The ZIP is self-contained. It includes the original world, the actual generated detail tiles, their parent hierarchy, the stored detail model, and the current edits. The original Go process, session ID, browser cache and server tile cache are not needed to reopen it. Saving waits for visible detail requests and pending brush operations. Previously visited tiles remain in the server's project store even when the browser evicts them from its rendering cache.

## Schema 2 layout (schema 1 remains readable)

```text
KRIEMHILD/
  manifest.json
  base/
    config.json
    solver.json
    environment.json
    fields/<chunk-x>/<chunk-y>.f64
  detail/
    model.json
    tiles/<level>/<tile-x>/<tile-y>.json
  edits/
    world.json
  world/
    project.json
  view/
    builder.json
```

- **Manifest:** format/schema and detail-engine versions, stable world identity, original generation request/seed, dimensions, wrapping and coordinate conventions, field encoding/order, geographic region IDs and parent relationships, and each file's path, byte length and SHA-256 checksum.
- **Base:** the actual generated solver domains and constraints, palette/rules, environment settings, land/water mask, continents' control points, tectonic plates, named climate classifications, water-body memberships, reef regions and geographic entities. All physical fields are preserved, including elevations, bathymetry, water levels, drainage/catchments, climate, geology and biomes. Their stored values are loaded directly; generation is not rerun.
- **Physical chunks:** 32 × 32 parent-grid cells, clipped at world edges. Values are float64 little-endian, ordered first by the manifest's field inventory and then row-major within each field. This preserves the original numerical values without lossy quantization.
- **Detail model:** stored geological apron and complete selected river geometry, identities, parent links, widths and grades. This supplies the immutable context for generating only missing regions.
- **Detail tiles:** actual 33 × 33 elevation samples, parent elevations, gradients, hydrological/environmental properties and anchored features. Every generated tile and its ancestors are included. Import restores their bytes; runtime requests load the stored tile first. New descendants inherit the stored parent surface, including differences from the procedural model.
- **Generator edits:** the current explicit biome assignments and pins, plus operation records, separate from the pre-edit generated base. The current snapshot is authoritative, including undo and cleanup. The original generator's visible brush undo stack is session-local.
- **Authored project:** `world/project.json` stores project metadata, layers, hierarchy, human entities, non-destructive terrain/water/vegetation operations, persistent Builder undo/redo and its camera/tool state. Schema 1 opens with an empty authored project. Composed detail uses the saved generated tile and these ordered operations; edits never overwrite the stored base.
- **Generator view:** the existing `view/builder.json` filename is retained for compatibility. It stores the generator palette/settings, display seed, rendering offsets, active 2D/3D mode and camera. Returning to an explored area uses its stored data.

## Loading and lifetime

Import locates the unique manifest and resolves paths relative to it. It validates checksums, array dimensions, physical fields, drainage references, parent-region links and shared parent elevation anchors before installing the project. It rejects missing files, duplicate identities, invalid paths and unsupported schema/detail-engine versions; it never silently regenerates corrupted data.

Physical base fields are loaded for the world overview. Detail is validated at import and restored into a disposable disk store, then read by geographic tile on demand. The display cache remains bounded. Missing detail is generated from the saved model and retained for the next save. Server storage under `KRIEMHILD_DATA_DIR/cache/` (default `data/cache/`, `/data/cache/` in Docker) is a working cache, not an external dependency of the ZIP. Session deletion removes that session's cache. A saved ZIP can be imported after the entire cache is deleted or the server restarts.

Click **Save** in the application header to commit a world to SQLite/PostgreSQL. Generation, edits, exploration and imports remain session-only until that action. Recent Worlds restores saved projects; Delete world removes the full project and explored detail after confirmation. ZIP export does not write to the database and import does not overwrite a saved version until Save. Repeated saves update the same world ID atomically. See [saving and deletion](AUTOSAVE.md) and [database configuration](CONTAINERS.md).

Completed physical and rules-only maps can be saved. Unfinished WFC solves must finish first. Current import/export limits are 256 MiB per upload/archive, 1 GiB expanded, 16 MiB per entry and 65,536 entries. The exporter refuses projects that would exceed the import limits instead of producing an unusable archive. Very large projects may require a future streaming format revision. Reopening older schema/engine versions requires an explicit compatible interpreter or migration, not a seed-only fallback.

## Verification

Go tests delete the original cache, create a fresh server instance, restore identical physical fields and edits, and serve stored tile bytes with generation unavailable. A deliberately distinct stored sample proves saved detail wins over procedural output; a newly generated descendant must inherit that sample. Folder and ZIP imports use the same hierarchy. Tests reject damaged checksums, missing tiles, invalid parents, duplicate regions and traversal paths.

The process-level integration test saves a world with explored detail and a brush pin, deletes its session, terminates the Go process, starts a new process and imports the ZIP. World fields, biome assignments, pins and explored tile bytes must match. The shared-browser audit exercises the actual save button and ZIP file input and verifies camera restoration without creating another generated world.
