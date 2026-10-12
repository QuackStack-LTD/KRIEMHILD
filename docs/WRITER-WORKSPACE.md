# Writer workspace

Writing is the creator's manuscript workspace. It is independent of fictional
Library archives and the global Age timeline. The project belongs to a World;
each scene can choose a different story Age and optional year.

## Implemented workflow

- Multiple World-associated projects plus unassigned **Loose Drafts**. Legacy
  `writing_documents` are migrated without deleting their original content or
  resetting revision numbers. Legacy API reads and writes resolve migrated drafts
  through the new revision store.
- Nested books, parts, chapters, scenes, front/back matter, notes, research and
  outline nodes. Drag to reorder before a sibling; Scene options provide keyboard
  alternatives, parent moves, rename, duplicate, type, target and recoverable delete.
  A container with live children cannot be deleted or converted into a leaf.
- Write, outline, corkboard and chronology views share the same nodes. Story dates
  never change manuscript order. Chronology groups by Age rather than pretending a
  branching/cyclic World timeline has one universal chronological order.
- Tiptap/ProseMirror semantic JSON (schema version 1), browser spellcheck, formatting,
  lists, links, explicit scene breaks, undo/redo, find and scene replacement.
  Editor preferences affect only the writing surface. The editor is lazy loaded.
- 1.1-second debounced autosave, blur save, navigation flush, visible errors,
  optimistic concurrency, local recovery and conflict comparison/copy recovery.
  Edits made during an outstanding save are flushed before navigation completes.
  Local recovery is best effort; server confirmation is the authoritative Saved state.
- Every saved node revision is immutable. Manual named snapshots, revision previews,
  restores and deleted-node recovery retain earlier history. Restore creates a new
  revision. Imports create new nodes and preserve the original text file.
- Unicode word counts, character counts, chapter/book/project totals, optional
  targets and net session words. Imported text does not count as session writing.
- World entity search, metadata pins and inline references retain World, entity and
  Age identifiers. Visible prose is never rewritten when a canonical name changes.
  Missing entities remain visible as missing references. Entity profiles open in a
  separate tab. The scene continuity check reports recorded death, lifetime and
  missing-reference issues with links; it does not claim to understand plot logic.
- TXT/Markdown import, TXT/Markdown and basic semantic DOCX export. The selected
  node and its descendants or the whole manuscript export in canonical order.
  Notes/research/outline subtrees are excluded unless explicitly requested. Private
  scene metadata is only in the full backup, not injected into prose.
- Focus mode, collapsible desktop panels/mobile drawers, keyboard shortcuts,
  accessible control labels, shared dialogs and design tokens.

Tiptap was selected to reuse mature selection, history, keyboard and schema behavior
instead of maintaining a custom contenteditable engine. See the
[official React integration](https://tiptap.dev/docs/editor/getting-started/install/react)
and [StarterKit](https://tiptap.dev/docs/editor/extensions/functionality/starterkit).
Dependencies are pinned in package-lock.json. Markdown conversion uses Tiptap's
schema parser; imported markup is never injected as trusted page HTML. Server-side
validation restricts node/mark kinds and URL protocols, derives searchable plain
text, validates hierarchy/world associations and rejects stale revisions.

## Persistence and backup v1

The existing SQLite/PostgreSQL database contains `writer_projects`, `writer_nodes`
and `writer_versions`. Each project/node has its own revision. Structural writes
lock the project row so concurrent moves cannot create a cycle. Node content saves
do not replace the entire workspace or increment unrelated nodes.

**World ZIPs deliberately do not include manuscripts.** Choose Export → Project
backup JSON and retain it alongside the World ZIP. To restore on another server,
import the World ZIP first (retaining World/entity IDs), then restore the writing
backup. Unassigned drafts restore without a World. A restore creates a new project
and new manuscript node IDs, remaps parent/version references, and preserves World
entity IDs, content, snapshots, metadata, source text and settings. It never
silently relocates links into a different World or overwrites an existing project.

```json
{
  "format": "kriemhild-writing",
  "version": 1,
  "project": { "id": "…", "worldId": "…", "revision": 1, "settings": {} },
  "nodes": [{ "id": "…", "projectId": "…", "parentId": "…", "contentSchema": 1,
              "content": { "type": "doc", "content": [] }, "revision": 1 }],
  "versions": [{ "node": { "id": "…", "revision": 1 }, "label": "Autosave" }]
}
```

Full records contain the remaining title, type, position, notes, reference,
story-time and original-import fields defined in `internal/storage/writer.go`.
Backups are transactional; invalid hierarchies or schema versions leave no partial
project. The import request limit is 64 MB; individual rich documents are limited
to 2 MB of JSON. Very large projects should be split into volumes/projects before
exceeding this initial backup limit.

## API

- `GET /api/writer/projects?world={id}` — this World's projects and Loose Drafts.
- `POST /api/writer/projects` — create/update project with expected revision.
- `GET /api/writer/projects/{id}` — project and current nodes.
- `POST /api/writer/projects/{id}/nodes` — `{node, label?}`, expected node revision.
- `GET /api/writer/projects/{id}/versions/{node}` — immutable revision history.
- `GET /api/writer/projects/{id}?backup=1` — complete portable backup.
- `POST /api/writer/import?world={id}` — atomic restore into an independent project.

The current application has no user/ACL subsystem. These endpoints follow its
existing access model; project/world association validation is not authentication.

## Verification and deliberate limits

Storage/API tests cover legacy migration and restart, stale writes, hierarchy cycles,
invalid cross-project/world links, snapshots, soft deletion, safe content, atomic
backup restoration and isolation from Library/world history. Frontend tests cover
save failures, concurrent typing, conflicts, editor/planning/focus UI, semantic
exports, ordering, counts and historical references. Browser checks exercised mobile
creation, drafting/autosave, named snapshots and world reference insertion.

The guide's P2 tools remain deferred: AI authoring, collaboration, tracked changes,
sprint/streak analytics, screenplay editing and publishing-grade PDF/EPUB. DOCX
import is also deferred; TXT/Markdown imports retain their original source. DOCX
export is an editable basic manuscript, not a print-layout engine. It uses a simple
list representation, basic headings/emphasis/hyperlinks and scene breaks; Markdown
or the project backup preserves the full semantic source. No inactive controls are
shown for these future features.
