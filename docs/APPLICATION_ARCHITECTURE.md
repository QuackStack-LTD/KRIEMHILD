# Project hub and World Builder

This implementation follows `KRIEMHILD_Application_Architecture.md`: the existing procedural engine remains the natural-world foundation, while an independent authored model supplies incremental worldbuilding.

## Using the application

- `/` opens the project hub without generating a world. Create a named world, reopen a recent project in either editor, or import a ZIP/extracted project folder.
- `/world/new` creates a world in World Generation. Existing presets, physical geology/climate, solver controls, exploration and 3D viewing remain available there.
- `/world/<world-id>/generate` and `/world/<world-id>/build` refer to the same saved project and live server session. Switching editors flushes saves, suspends the hidden generator and retains world identity. It never invokes generation.
- World Builder uses cursor-centered pan/zoom and the existing hierarchical terrain. Finish an unfinished solve in World Generation before building. Physical environment is necessary for numerical terrain sculpting; entities also work on rules-only maps.

Choose a civilization layer and type. Settlements use polygons; roads and infrastructure use lines; buildings use points or polygons; factories and landmarks use points; custom structures support all three. New objects default to black. Existing saved colors and legacy geometry remain intact on import. Click vertices and press Enter or double-click to complete a line/polygon. Escape cancels it. Select objects on the map or viewport list; drag to move them. Selected lines and polygons expose draggable vertices. The inspector edits names, colors, parent relationships, notes, visibility detail and optional creation/destruction years. Children must be removed or reparented before deleting a parent. Countries, settlements, buildings, infrastructure and annotations use stable identities in the same hierarchy.

Landscape tools stamp local raise/lower, flatten, smooth, crater, valley, island, water, drain and vegetation operations. River uses a drawn path to incise a channel. Radius is measured in base world cells; heights and water levels are metres. Flatten samples the clicked surface; smoothing stores its target samples when applied. A crater excavates terrain, raises its rim, clears vegetation and exposes rock. Increasing submerged elevation can expose an island; water depth follows the edited bed.

Layer visibility, locks and ordering are saved. Reordering controls entity draw order; natural operations retain their recorded chronological order. Undo/redo covers placed objects only (creation, moves, properties and deletion), with the last 100 object commands retained across restarts and ZIP export. Terrain edits, layer settings and camera changes never enter that history or discard its redo branch. Locked object layers cannot be changed through undo. Older mixed histories are filtered on load without changing the current landscape. Editing names/metadata and camera preferences is staged until Save and does not enter the geometric undo stack.

## Ownership and update path

`internal/world` owns project metadata, layers, authored entities, terrain operations, history and Builder view state. `internal/terrain` continues to own generated base geography and deterministic detail. Renderers consume these models; images and meshes never become save data.

`src/App.jsx` owns navigation and shared React context. `GeneratorView` hosts the existing generator controller; `BuilderView` hosts the separate Builder renderer. The Go session is authoritative for both. Builder commands carry an expected revision; a stale writer receives a conflict instead of silently overwriting newer content.

An entity edit updates its quadtree entry and independently stored entity record. It does not touch terrain. A landscape operation updates the operation index and returns its affected bounds. Only intersecting composed tiles, plus a two-cell derivative/parent interpolation halo, are invalidated. The immutable generated tile store remains intact. Composed child tiles inherit edited parent morph targets. Cached unrelated tiles are reused, and no continent, climate or mountain generation runs during Builder edits.

Terrain is fetched only for visible LOD tiles. Tile images are cached by tile identity and morph amount. Normals/gradients are recalculated within affected tiles. The Builder supports both flat editing and a 3D terrain view, using the same composed detail endpoint as Generation. Lines and closed polygon outlines are subdivided at terrain triangle boundaries and interpolated against the actual displayed mesh, including its LOD morph and edited elevations. They follow ridges and depressions instead of bridging between endpoint heights. Switching views saves both cameras. Selection and point/path placement are available in 3D; use the flat view to drag objects and vertices.

Entities use a server-side quadtree and a viewport/detail query capped at 2,000 results. The browser renders only that result. Small buildings default to local detail; settlement polygon footprints and authored roads/districts/buildings appear at their assigned detail. Wind and analytical overlays stay in Generation; Builder offers terrain, elevation and water-depth views. Continuous camera changes are coalesced; entity moves commit on release rather than on every pointer event.

## Persistence

Database schema **3** adds project names and migrates existing SQLite/PostgreSQL catalogs automatically. New gzip JSON parts are:

- `builder/header`: metadata, layers, bounded history and Builder camera/tool preferences.
- `builder/entity/<id>`: one authored entity, including hierarchy and optional time fields.
- `builder/operation/<id>`: one ordered sparse landscape modification and any saved smoothing samples.

These parts commit atomically with the project catalog. Local edits leave the generated checkpoint, environmental fields and unrelated tile/entity records unchanged. Deleted or undone records retain tombstones so history can restore their identity. Save failures remain visible and offer Retry saving; navigation flushes pending commands and view changes first.

Portable ZIP schema **2** retains all schema-1 generated geography and explored tiles and adds `world/project.json`, containing authored metadata, layers, entities, operations, history and Builder view state. The manifest checksums that file like every other project entry. Schema-1 archives remain readable and open with an empty authored model. Import validates geometry, coordinates, hierarchy cycles, layer inventory, operation ordering and history before installing the world. Saved detail takes precedence over procedural generation; composed terrain is reconstructed from that stored base plus the saved operations without relying on the old server.

## Scope and extension points

The architecture guide explicitly defers a full historical simulator. Entities have optional `created`/`destroyed` years and operations an optional `year`; there is no timeline playback yet. Generic primitives support the listed object families; the editor does not automatically generate city streets or buildings. Landscape edits are deliberate local changes, not a rerun of global climate or drainage simulation. River incision and local water editing do not calculate a new world-scale drainage network.

Viewport work is bounded, but million-building performance is not claimed: the server currently reconstructs the entity index in memory when opening a project. The existing ZIP entry-size and overall archive limits remain. Very large authored datasets can extend the storage interface with paged spatial loading and chunked archive entities without changing renderer ownership or world coordinates. Worlds still share one unauthenticated library, and deployments use one application process.

## Validation

`internal/world/model_test.go` covers hierarchy, locks, undo/redo, incremental records, 10,000-object viewport/LOD queries, physical craters/water, edited parent anchors, adjacent tile seams and malformed history. `internal/httpapi/world_test.go` covers unchanged generated base data, local cache invalidation, revision conflicts, cold database restart, portable authored ZIPs, names and viewport validation. Frontend tests cover routing and safely restored Builder preferences alongside the existing camera, terrain, autosave and process-level persistence tests.

The shared-browser audit (`tests/builder-browser-audit.js`) also exercises ZIP export, deletes the original session, imports through the file input and checks entity/history identity and identical edited detail without generation. Container checks exercised a PostgreSQL schema-2 catalog migration to schema 3 and preserved authored edits through an application-container restart.

Mountain drainage now permits smaller, supplied catchments in mountain terrain while retaining accumulation, catchment size, aridity and downstream continuity gates. This affects newly generated worlds; stored geographical models and explored detail are not silently regenerated.

The generation brush has no Undo button or terrain-undo shortcut. Undo and redo are reserved for authored objects in World Builder.


Rendering is based on feature type, not authoring origin. Authored rivers store a `DetailFeature` channel inside their landscape operation (curved path, variable widths, discharge, downhill water elevations and incision limit). They are included in the same detail-tile feature list as generated drainage and use the same bank/water/highlight renderer in flat maps and 3D terrain textures. Both use `terrain.SampleChannel` and `terrain.ChannelBed` for their channel/valley cross-section. The radius controls authored channel scale; the height/depth control caps incision. Drawing direction is normalized from the higher endpoint toward the lower endpoint, and water elevations never rise downstream. This does not recompute the world's generated drainage network or relocate its existing rivers.

Saved authored river profiles are authoritative on reopen and ZIP import. Older authored rivers without a profile are upgraded once from their saved path and terrain, with the resulting profile included in subsequent database saves and ZIP exports. Existing generated detail remains untouched. Terrain material shading and zoom transitions no longer depend on the `authored` flag.

`src/terrain/feature-style.js` centralizes water, bank, highlight and authored-object styles. Point entities remain circular dots in both views; line and polygon outlines use the same color, selection color and screen width. Polygon interiors are unfilled in both views, and 3D edges remain draped onto terrain triangles. Flat-view vertex handles and labels are editing aids rather than different object geometry.
