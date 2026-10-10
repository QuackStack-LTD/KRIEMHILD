# Historical Worlds

KRIEMHILD now separates fictional Worlds from terrain projects. A World owns a directed graph of Ages. Each Age owns a historical state made of terrains, maps, logical entities, representations and typed spatial relationships.

## Working with a World

Create a World from the welcome page; its initial Age is created immediately. Generate as many terrain previews as needed, then choose **Accept terrain & Build**. Acceptance saves the complete terrain and creates its Terrain and root Map records. Multiple terrains may coexist in one Age.

Map edits remain staged until **Save**. Historical metadata actions (creating/selecting/renaming Ages, timeline connections, and creating child maps) commit immediately. New Ages copy the source Age's **saved** state. Opening a detailed map explicitly saves the parent first.

The bottom bar displays a single timeline, centered on the current Age, with at most two neighbors in each direction. Arrow buttons and horizontal trackpad scrolling move through longer timelines. Branch stubs are clipped to the bar. The Time Graph shows all lanes and directed relationships with scrolling and zoom controls.

After/before operations insert into a timeline's ordered Age list and splice its linear edges. Branches and parallel histories get new lanes; parallel histories inherit content but have no chronological edge. Merge creates a new Age with two incoming edges. The user must choose which source wins conflicting map snapshots, representations and entity properties; unique content is retained from both. Connecting existing Ages creates a chronological link only and does not silently rewrite either historical state. Directed chronological cycles and self-connections are allowed, including a branch reconnecting to one of its own ancestors. Graph layout condenses strongly connected components before assigning lanes and draws return connections as curves. Entity containment and water drainage still reject their own invalid cycles.

## Logical objects and detailed maps

Objects placed in the builder become logical entities when the map is saved. Their map geometry is stored separately as a representation. Stable entity, terrain and map IDs survive Age inheritance; each Age has its own values and independent editor-session identity.

Pin a spot and use **Name geography at this pin** to name an existing river, lake, mountain region, landmass or environmental region. Generated feature identities are retained; naming the same feature again renames it. Users can also draw named geographical regions through the Natural features layer. Containment and river source/destination relationships are recomputed from geometry rather than creation order. Natural region cell sets preserve holes such as islands within an ocean. The geological classification is a procedural region approximation, not a gazetteer of every possible individual peak or political region.

Select a settlement or zoom into an area and choose **Explore ... in detail**. This opens a separately persisted Settlement Map with a bounded initial extent in the parent's unchanged world coordinates. The selected settlement's polygon is copied exactly. Internal buildings, roads and other city objects are local to this map. Editing a shared footprint updates its representations in the same Age; new named geographical entities are represented in intersecting maps of the same terrain. Earlier Ages remain unchanged. The initial Settlement Builder uses the existing 2D layout tools; root maps retain both 2D and 3D views.

## Storage

SQLite and PostgreSQL use the same additional tables:

- `history_worlds`, `history_ages`, `history_timelines`, `history_edges`
- `history_terrains`, `history_maps`, `history_entities`
- `history_representations`, `history_relations`
- `history_snapshots`, `history_blobs`

Each logical record has an explicit owning World, and stateful records also have an Age. Edge rows include source, destination and type. Map editor IDs are unique per Age/map, preventing reuse of a mutable session across Ages. World transactions use revision checks to reject stale graph changes.

Timeline lanes retain their saved order. PostgreSQL history transactions use a consistent snapshot across the logical tables. Entity deletion tombstones let merge conflict choices distinguish deleted objects from objects that never existed on a branch.

A map snapshot is a content-addressed manifest of the existing compressed solver, environment, authored data, detail model, and explored tiles. Unchanged parts are shared by hash. Creating an Age or child map initially shares its source snapshot; saving edits creates a new manifest and only stores changed blobs. Database and ZIP restoration load saved detail before generating missing tiles. Logical representations are authoritative when materializing a map, so an inherited snapshot cannot overwrite a newer shared footprint.

World ZIP schema 1 uses `manifest.json` for the complete historical hierarchy and snapshot manifests, with `blobs/<sha256>.bin` for each unique terrain/data part. All checksums, references, graph topology, terrain checkpoints and explored tiles are validated before import commits. Import into an empty database preserves logical identities. Import alongside an existing World creates an independent copy and remaps colliding editor IDs; it does not overwrite the existing World.

Earlier terrain-only database projects remain available on the welcome page. Open one and choose **Move into a World**. Terrain-only ZIPs and folders can still be imported using the generator's legacy import controls, then accepted into a World. This leaves the existing generation engine and original terrain archive reader intact.

## Verification

History tests exercise graph insertion, branching, parallel histories, merge choices, chronological cycle persistence, identity preservation, spatial naming order, and city/parent isolation. HTTP tests accept real generated terrain, fork an Age, edit child footprints, restart the database, then export and import into an unrelated empty database and compare previously explored tile bytes. Frontend tests cover the five-Age window and graph layout.

Additional checks cover deletion conflicts, stale-command rollback, stable lane ordering, generated geography naming, ocean region holes, and rejecting archives with missing terrain blobs before any database records are created. Database round-trip tests use SQLite; a live PostgreSQL integration run is not included in this verification.
