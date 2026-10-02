# P1 storage contract (format 1)

The P1 implementation uses immutable JSON records, complete snapshot manifests and a single project head. This is the first implementation of the platform plan's storage architecture. It intentionally omits sharding, SQLite indexes, compaction and migrations until subsequent phases.

```text
worlds/<world UUID>/
  kriemhild.json             # version, world ID, name
  project.head.json          # {"revision": "<SHA-256>"}
  revisions/<SHA-256>        # world metadata, Age heads, parent revision, save time
  snapshots/<SHA-256>        # version and full {record UUID: record SHA-256} manifest
  objects/<SHA-256>          # structured entity/relation/schema/map/scene/note JSON
  assets/<SHA-256>           # original image bytes
  .kriemhild/
    writer.lock             # operating-system process lock
    transaction.json        # old/new revision pointers during a commit
```

Hash-addressed JSON files have no extension in this first format; their directory determines the representation. The hash covers the exact UTF-8 JSON bytes or original image bytes. Maps store dimensions, an optional image hash, and pins in normalized 0–1 coordinates. Scenes store schema-constrained rich-text document JSON with stable block IDs, a setting Age/snapshot, entity references, a work label and an order.

## Invariants

1. UUIDs identify records independently of names and coordinates. A record's kind cannot be changed by a normal edit.
2. Every Age has its own complete manifest. Resolving it does not walk mutable parent Ages.
3. Copying an Age initially shares the exact snapshot hash. Editing creates new objects and a new manifest only for the selected Age.
4. Source Age, source snapshot and source project revision are retained as inheritance provenance.
5. Scenes preserve setting references across copying. Retargeting is an explicit edit; a scene may still refer to an entity absent from its current owning Age if it exists in the pinned setting.
6. Undo/redo select complete previous/next snapshots in the current Age and commit a new root. They never remove previously saved revisions. Copying/renaming Age metadata is not part of the content undo stack.
7. One writer process owns the project. Every command supplies an expected project revision; outdated callers cannot overwrite newer work.
8. Validation blocks malformed records, missing live relationship/pin targets, invalid typed custom fields and missing pinned scene references.
9. Immutable object and image hashes are checked on reads. Project opening validates the snapshot closure, including scene settings, inheritance baselines and undo/redo snapshots.
10. No retention or garbage collection is performed in P1. Unreferenced objects from interrupted or rejected writes are harmless and are kept.

## Commit protocol

1. Read the current revision and reject a mismatched expected revision.
2. Construct the changed record(s), write/flush immutable objects and snapshot manifests, then validate the proposed root and reachable state.
3. Write/flush the immutable project revision, containing the previous revision as parent.
4. Atomically write/flush a recovery journal recording old/new revision hashes.
5. Recheck that the active head has not changed externally.
6. Write/flush a temporary head and atomically replace `project.head.json` on the same volume.
7. Acknowledge the save; remove the completed journal. Leftover valid journals are checked on reopening.

Windows replacement uses `MoveFileEx` with `REPLACE_EXISTING | WRITE_THROUGH`; the Unix implementation renames and syncs the parent directory. The write helper also syncs file contents before replacement. These protections assume a local filesystem with normal atomic replacement semantics; they are not a claim about every network filesystem or hardware failure mode.

On startup, a valid current root is retained. If a journaled commit has an invalid head, its old root must pass validation before recovery can replace the head. Without that evidence, opening fails and leaves the content for investigation. Future format versions are refused without automatically rolling back to an older schema.

## Review semantics

Age comparison resolves both manifests and compares record hashes, returning added, removed and changed records with their actual contents.

Source-correction review compares the descendant's pinned source with the source Age's current state, then checks the descendant's current state. A conflict means both sides differ from the baseline and from each other. P1 allows explicit whole-record replacement/removal; it does not claim field-level or manuscript-block merging. The command checks both the project revision and the exact incoming snapshot. Integrity validation runs before committing the combined result.

## Recovery tests and boundaries

The Go suite tests normal reopen, read-time corruption, stale revisions, locking, simulated save failures and subprocess exits at the object, journal and head stages. Browser tests separately exercise the scene recovery queue. Windows behavior is exercised on the development machine; Unix code is provided and can be tested in CI but should not be described as locally exercised on Windows.

P1 does not provide a project archive exporter or automated rolling backups. A stopped-server copy of the complete world directory is the supported manual backup. Never delete old snapshots or assets based only on the selected Age: historical settings and undo states can still reference them.
