# Previous terrain implementation retirement

Completed on 2026-10-14 following the request to keep only the new Terrain Generator,
Map Builder and Settlement Builder while preserving other worldbuilding pages.

## Scope

- Removed `src/TerrainApp.jsx`, `src/terrain`, `src/builder`, `src/views` and their
  old scoped stylesheet. Removed their obsolete tests and terrain profiling/gallery CLIs.
- Removed pre-existing uncommitted terrain repair changes, tests and repair documents
  after backing them up outside the repository. The new continuous implementation remains.
- All active terrain/editor routes use `src/continuous`. Older saved maps use a new
  read-only archive viewer with ZIP/folder import, overview, saving and ZIP export.
- Retired the old defaults, session-creation and solver-action endpoints with HTTP 410.
  Legacy detail endpoints return saved tiles only; missing tiles are never generated.
  Artwork export falls back to saved macro fields instead of synthesizing missing detail.
- Moved the old backend root package to `internal/legacyterrain`. It remains a
  compatibility dependency for archive DTOs, validation and queries, and contains
  historical algorithms used by test fixtures. It is not a parallel production editor
  or terrain generator. Historical fixture handlers exist only in `_test.go` files.
- Preserved Library, Characters, Writing, Biosphere, Magic & Technology, histories,
  relationships, authentication, collaboration and storage. No files under
  `src/worldbuilding` were changed. Shared backend changes outside HTTP are import-path
  updates to the isolated compatibility package.

Older archive overviews show saved categorical cells and authored objects; their full
continuous fields, original explored tiles and saved artwork remain in exported ZIPs.
They cannot be edited by the removed editors. The new workspace still has the broader
implementation limits recorded in [STATUS.md](STATUS.md).

## Recovery

Git tag: `terrain-before-retirement-20261014` (commit `b1be3ce`).

External backup: `C:\Users\tomyi\worldbuilding-backups\terrain-retirement-20261014`.
It contains the pre-existing binary patch and untracked repair tests/documents. The
earlier full working-tree backup and Git bundle remain in the sibling
`terrain-rebuild-20261014` directory. User databases and saved project archives were
not removed.

## Verification

- JavaScript: **58 passed**, including a new continuous ZIP/server-process restart
  test, archive rendering, retired source checks, Library, characters, writing and systems.
- Go: HTTP API, world, history, storage, biosphere, auth, all new terrain subpackages
  and settlements passed. HTTP regression run completed in 110 seconds.
- `go vet` passed for HTTP, world, new terrain and settlements.
- Production frontend and Go executable builds passed.
- UI boundary checks passed. Full UI formatting check still reports pre-existing
  formatting in `Biosphere.jsx` and `BiosphereEditors.jsx`; these unrelated files were
  left unchanged.
- The old algorithm suite was not rerun: its baseline full run had timed out.

See `retirement-go-tests.txt`, `retirement-js-tests.txt` and `retirement-build.txt`
for captured results. Previous evidence in STATUS.md predates this retirement and
includes tests for implementations that have now been removed.
