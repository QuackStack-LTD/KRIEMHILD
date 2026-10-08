# Portable world projects

Use **View → Save → Save world ZIP** after generation finishes. **Open world ZIP** imports that archive; **Open world folder** imports its extracted `KRIEMHILD` folder. PNG/SVG buttons remain visual exports and are not project files.

The ZIP is self-contained. It includes the original world, the actual generated detail tiles, their parent hierarchy, the stored detail model, and the current edits. The original Go process, session ID, browser cache and server tile cache are not needed to reopen it. Saving waits for visible detail requests and pending brush operations. Previously visited tiles remain in the server's project store even when the browser evicts them from its rendering cache.

## Schema 1 layout

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
  view/
    builder.json
```

- **Manifest:** format/schema and detail-engine versions, stable world identity, original generation request/seed, dimensions, wrapping and coordinate conventions, field encoding/order, geographic region IDs and parent relationships, and each file's path, byte length and SHA-256 checksum.
- **Base:** the actual generated solver domains and constraints, palette/rules, environment settings, land/water mask, continents' control points, tectonic plates, named climate classifications, water-body memberships, reef regions and geographic entities. All physical fields are preserved, including elevations, bathymetry, water levels, drainage/catchments, climate, geology and biomes. Their stored values are loaded directly; generation is not rerun.
- **Physical chunks:** 32 × 32 parent-grid cells, clipped at world edges. Values are float64 little-endian, ordered first by the manifest's field inventory and then row-major within each field. This preserves the original numerical values without lossy quantization.
- **Detail model:** stored geological apron and complete selected river geometry, identities, parent links, widths and grades. This supplies the immutable context for generating only missing regions.
- **Detail tiles:** actual 33 × 33 elevation samples, parent elevations, gradients, hydrological/environmental properties and anchored features. Every generated tile and its ancestors are included. Import restores their bytes; runtime requests load the stored tile first. New descendants inherit the stored parent surface, including differences from the procedural model.
- **Edits:** the current explicit biome assignments and pins, plus operation records, separate from the pre-edit generated base. The current snapshot is authoritative, including the effects of undo and cleanup. The editor currently paints biome assignments; it does not expose an elevation sculpting tool. Undo history itself is session-local.
- **Builder view:** display palette/settings, original display seed, rendering offsets, active 2D/3D mode and camera state. Returning to an explored area uses its stored data.

## Loading and lifetime

Import locates the unique manifest and resolves paths relative to it. It validates checksums, array dimensions, physical fields, drainage references, parent-region links and shared parent elevation anchors before installing the project. It rejects missing files, duplicate identities, invalid paths and unsupported schema/detail-engine versions; it never silently regenerates corrupted data.

Physical base fields are loaded for the world overview. Detail is validated at import and restored into a disposable disk store, then read by geographic tile on demand. The display cache remains bounded. Missing detail is generated from the saved model and retained for the next save. Server storage under `KRIEMHILD_DATA_DIR/cache/` (default `data/cache/`, `/data/cache/` in Docker) is a working cache, not an external dependency of the ZIP. Session deletion removes that session's cache. A saved ZIP can be imported after the entire cache is deleted or the server restarts.

Saving a ZIP or importing a project also persists the complete archive, including explored tiles, in the selected SQLite/PostgreSQL database. The **Saved worlds** picker restores that stored snapshot without regeneration. Repeated saves update the same world ID. Further exploration and edits require another save; there is no background autosave. SQLite and PostgreSQL contain independent libraries: export/import ZIPs to transfer worlds between them. See [database configuration](CONTAINERS.md).

Completed physical and rules-only maps can be saved. Unfinished WFC solves must finish first. Current import/export limits are 256 MiB per upload/archive, 1 GiB expanded, 16 MiB per entry and 65,536 entries. The exporter refuses projects that would exceed the import limits instead of producing an unusable archive. Very large projects may require a future streaming format revision. Reopening older schema/engine versions requires an explicit compatible interpreter or migration, not a seed-only fallback.

## Verification

Go tests delete the original cache, create a fresh server instance, restore identical physical fields and edits, and serve stored tile bytes with generation unavailable. A deliberately distinct stored sample proves saved detail wins over procedural output; a newly generated descendant must inherit that sample. Folder and ZIP imports use the same hierarchy. Tests reject damaged checksums, missing tiles, invalid parents, duplicate regions and traversal paths.

The process-level integration test saves a world with explored detail and a brush pin, deletes its session, terminates the Go process, starts a new process and imports the ZIP. World fields, biome assignments, pins and explored tile bytes must match. The shared-browser audit exercises the actual save button and ZIP file input and verifies camera restoration without creating another generated world.
