# Automatic world persistence

Every created world receives a stable ID and a database checkpoint. Generation steps, painting, cleanup, undo state and newly explored terrain are persisted automatically. No ZIP download or manual save is needed to keep a world in the server library. View → Save → Saved worlds lists previous worlds and opens their stored state.

The browser automatically saves display settings, palette, seed, active tab, brush settings, 2D/3D camera and rendering offsets. Continuous input is coalesced over 300 ms and requests are serialized. A fixed deadline keeps a continuous gesture from postponing saving indefinitely. Later changes made during a request are saved next; failed writes remain queued and retry after two seconds. Switching or generating worlds flushes pending view changes first. A compact page-hide beacon helps preserve the final camera position; browsers cannot guarantee delivery after abrupt termination. The UI warns before leaving while known writes are pending or failed. The saved indicator refers to acknowledged view writes; terrain requests commit their own data independently.

Unfinished worlds retain solver domains, RNG, queues, buckets, repair/backtracking state and counters. They reopen paused; use Pause/Resume or Step to continue. Loading a world never reruns world generation. Explored tile payloads and their ancestors load from storage, with missing detail generated only when first requested. The editor resets its visible undo button history when reopening, while the checkpoint retains engine snapshots.

## Storage decision

Keep PostgreSQL externally and SQLite for standalone use. Use relational metadata plus independently compressed structured payloads, rather than a separate database engine or an entire ZIP rewrite after each action.

| Option | Fit for this engine |
| --- | --- |
| MongoDB | Flexible documents, but large terrain worlds still require splitting: BSON documents are limited to 16 MiB, with GridFS for larger files. Adds another deployment/driver without solving incremental world consistency by itself. |
| Fully normalized geography | Useful for cross-world spatial queries, but unnecessary for the current viewer, which consumes numerical fields and deterministic tiles as units. Turning every elevation sample into a row would complicate reconstruction. |
| Whole ZIP after every action | Portable but repeatedly compresses and rewrites all previously explored geography, even for a camera move. |
| Relational catalog + structured parts (implemented) | Atomic commits, reusable SQLite/PostgreSQL infrastructure, exact solver/tile fidelity, and writes limited to changed parts. ZIP remains the portable exchange format. |

Source references: [MongoDB document limits](https://www.mongodb.com/docs/manual/core/document/), [GridFS](https://www.mongodb.com/docs/manual/core/gridfs/), [PostgreSQL binary data](https://www.postgresql.org/docs/17/datatype-binary.html). The choice above is an engineering judgment based on KRIEMHILD's existing access patterns.

## Database schema 2

`kriemhild_projects` stores world ID, seed, dimensions, tile count, stored bytes and last update time. `kriemhild_world_parts` has a composite primary key `(world_id, path)`, a foreign key to the world and binary payloads containing gzip-compressed JSON:

- `checkpoint`: versioned complete running solver, config, original generation request, base snapshot, edits and undo snapshots. Updated after engine mutations.
- `environment`: immutable base geographical fields, entities, hydrology and climate. Written when the world is first saved or a legacy ZIP is converted.
- `detail-model`: immutable hierarchical terrain context, written when first created.
- `tile/<level>/<x>/<y>`: actual explored tile samples/features. Each tile and its ancestors are persisted before its successful response; previously saved tiles are not rewritten.
- `view`: browser controls and camera state. View changes do not rewrite terrain, solver or tile payloads.

Metadata and changed parts commit in one transaction. A failed commit leaves the earlier durable snapshot intact and retains dirty session state for retry. View requests carry a browser identity and sequence to reject delayed older requests, including unload beacons. Opening a world already active in this server reuses that session so it cannot create a second stale solver copy.

Schema 1 ZIP records remain readable. They migrate to parts on their next automatic write. Manual ZIP saves/imports remain supported and replace the stored snapshot atomically. Export reconstructs the full portable archive from the loaded world and all stored explored tiles; it has no dependency on server caches. Internal checkpoint version 1 and the detail-engine version are checked on load. Future incompatible engine changes require explicit migration.

Use one application instance: live sessions and mutation locks are process-local. The current platform has a shared library, not authenticated user accounts; adding a login system and ownership filtering is a separate feature. SQLite remains the default when no external connection is configured. `docker-compose.yml` explicitly configures PostgreSQL and preserves its data in a named volume.

## Validation

Tests cover exact unfinished solver/RNG recovery and continued generation, stored terrain and edits after deleting the old session/cache, camera-only updates leaving geographic payloads unchanged, stale camera request rejection, compact view merge, ZIP export of autosaved tiles, and client write serialization/retry. The container smoke script exercises automatic save/restart/open with SQLite and PostgreSQL, without a manual ZIP save before restart.
