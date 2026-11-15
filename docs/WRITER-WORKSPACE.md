# Writer's Archive

A World has two spaces: **Ages** for worldbuilding, and **Writer's Archive** for
notes, research, images, manuscripts, and finished documents. The archive remains
inside its World, at the existing `/worlds/{id}/writing` route.

## File browser and editor

The file browser provides nested folders, list/grid views, name search, type and
modified-date filters, recent files, rename, move, duplicate, and Trash/Restore.
Deleting a folder atomically trashes its contents. Restoring the folder restores
only that deletion group; older separately deleted files stay deleted.

Create DOCX, PDF, TXT, or Markdown documents and open them in the new document
editor. The editor provides headings, formatting, lists, links, images, an outline,
find/replace, undo/redo, autosave, local recovery, and version history. Documents
can be downloaded in any of the four document formats. Reading-font and zoom
controls affect the reading surface, rather than publication typography.

Import multiple files or drop them into the archive (64 MiB per file). Text,
Markdown, DOCX paragraphs/basic formatting, and text-bearing PDFs become editable
documents. Originals remain downloadable. Complex DOCX/PDF layouts are preserved
in the original file; edited exports use the archive's document layout. Scanned
PDFs need OCR for editable text and otherwise remain previews. Other formats are
stored unchanged, with image/audio/video/PDF previews where supported.

## Inworld Archive

The permanent linked folder is a live projection, not a second copy:

    World / Writer's Archive / Inworld Archive / Age / Archive / Document

All Ages appear, including empty Ages. Library subarchives retain their hierarchy.
File contents edited here save a new version in the same Library record; custody,
archive configuration, and other historical-record management remain in Library.
External file changes are detected before saving a linked document. Lost or
removed documents cannot silently be overwritten. Changes in the World refresh
this projection. Opening Library from a link preserves the selected Age.

Each inworld archive has a Resources folder with a map/statistics review and
resource documents grouped by source Age and year. Reports retain their terrain
circle links and use only that archive's retained observations for charts. See
[terrain resource archives](TERRAIN_RESOURCE_ARCHIVES.md) for overlap handling,
custody, sampling, and separate Age timelines.

## Persistence and access

Existing projects appear as folders and their existing contents remain available.
Author files use revisioned `writer_projects`, `writer_nodes`, and `writer_versions`.
Original binary data lives in World-scoped `writer_files`, addressed by checksum.
World ZIP and project backups include those originals and historical file versions.
Moving between folders/projects retains node IDs, shared text updates, comments,
revision history, and attachments; cross-World moves are rejected.

Author archive operations do not increment fictional World revisions or create
Age/Year records. Archive listings omit document bodies. Opening this page reads
World metadata and record-only Library projections, without loading biospheres.
Existing World roles and CSRF checks apply to all endpoints.

- `GET/POST /api/worlds/{world}/writer-archive`: list / initialize the archive.
- `GET /api/worlds/{world}/writer-archive/inworld`: linked Age/Library projection.
- `POST /api/worlds/{world}/writer-archive/actions`: atomic trash/restore/move.
- `POST /api/writer/projects/{project}/files`: multipart metadata and original file.
- `GET /api/writer/projects/{project}/files/{node}?version={revision}`: exact file version.

The existing writer node, shared-text, revision, and backup endpoints remain the
storage protocol. Browser tests use an isolated server with
`KRIEMHILD_ARCHIVE_AUDIT=1` and `KRIEMHILD_UI_URL` pointing to that server.
