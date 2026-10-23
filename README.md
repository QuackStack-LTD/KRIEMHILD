# KRIEMHILD

React frontend and native Go backend for a shared worldbuilding project. New worlds use the versioned continuous terrain workspace. Older projects open in a read-only archive viewer; the previous editors and generator endpoints have been retired. This project is self-contained; the neighboring source directory is not needed to build or run it.

The continuous terrain rebuild is an **in-progress replacement**, with working generation, tiled maps, local authoring and settlement previews. See [implementation and acceptance status](docs/terrain-rebuild/STATUS.md), the [seed gallery](docs/terrain-rebuild/gallery/index.html), and [coast/detail views](docs/terrain-rebuild/gallery/detail.html). The older engine documentation below describes compatibility behavior; it does not imply that every legacy feature has migrated to the new workspace.

The opening page is a **Project Hub** with named recent worlds and ZIP/folder import. Fresh terrain opens the new Generator, Map Builder and Settlement Builder workspace. Existing continuous maps open in the new editor; older maps open in the archive viewer. Switching editors never regenerates the map. See [retirement scope](docs/terrain-rebuild/RETIREMENT.md). Older terrain documentation describes historical formats, not current UI controls.

## Run

Legacy physical worlds use terrain- and climate-driven drainage, lake water budgets,
groundwater springs and narrow, blended shore materials. See the
[hydrology model, validation and compatibility notes](docs/HYDROLOGY.md).
The legacy engine also stores layered lithology, geological history and natural resources. These records remain available when opening legacy projects; their full continuous-terrain replacement is outstanding. See [natural resources and hover queries](docs/NATURAL_RESOURCES.md) for the legacy contracts.

Install Node.js 22.12+ and Go 1.26+, then run from this folder:

```sh
npm install
npm start
```

Open **http://127.0.0.1:8124**. On Windows, `start.bat` performs the same steps. The launcher also recognizes a portable Go installation at `.tools/go`.

For source distribution, package a commit with `node scripts/verify-source.mjs HEAD`.
This verifies archive contents and builds a clean extraction; it does not include
the optional local Go SDK, credentials, databases or dependency installations.
See [archive investigation and baseline](docs/atlas-redesign/BASELINE.md).

For development, run these in separate terminals:

```sh
npm run build
go run ./cmd/kriemhild
npm run dev
```

Open http://127.0.0.1:5173. Vite forwards `/api` requests to Go on port 8124.

For a production build:

```sh
npm run build
go build -o bin/kriemhild ./cmd/kriemhild
./bin/kriemhild -addr 127.0.0.1:8124 -dist dist
```

On Windows, use `bin/kriemhild.exe`. Ship the binary with `dist/`; Node.js is only needed for building the frontend. Three.js and Delaunator are bundled locally, so 3D viewing and SVG export work without a CDN.

## Active implementation

Terrain Generator, Map Builder and Settlement Builder use `src/continuous/` and the new
`internal/terrain` subpackages. The old frontend editors have been removed. The old backend
package is isolated in `internal/legacyterrain` for saved archive types, validation, queries
and historical fixture tests. Production terrain and detail generation use only the new pipeline.

Library, Characters, Writing, Biosphere, Magic & Technology, histories, relationships,
authentication, collaboration and storage retain their existing implementations.

## Docker and project database

For a complete Keycloak, HTTPS, PostgreSQL and scheduled-backup stack, follow [self-hosted deployment](deploy/README.md). If you already operate an identity provider and reverse proxy, copy `.env.example` to `.env` and use the root Compose configuration for the app and PostgreSQL. See [accounts, collaboration, migration and backups](docs/USER-COLLABORATION.md).

```sh
docker compose up -d
```

Open your configured HTTPS origin. The container serves React and the Go API through the reverse proxy. Hosted mode persists accepted map changes and requires authentication; loopback local mode retains the explicit **Save** workflow. **Projects / Recent Worlds** shows only accessible worlds. Owners can delete worlds. ZIP export remains a separate portable content backup.

Running the binary or image without `DATABASE_URL` still uses embedded SQLite. With PostgreSQL configured, it requires that database to be available. See [container setup and releases](docs/CONTAINERS.md) and [world saving and deletion](docs/AUTOSAVE.md).

Use **View → Save → Export world ZIP** for a portable backup of a completed world, explored detail and edits. **Open world ZIP** or **Open world folder** restores it without regenerating geography or depending on old server caches. See [Portable world projects](docs/WORLD_PROJECTS.md) for the manifest and validation. Active sessions expire after 30 minutes of inactivity, and only explicitly saved versions remain in the database. Terrain painting is available through the Builders, not the Generator. Builder undo/redo persists. This is a local application; it starts on the loopback interface.

## Validation

```sh
go test ./...
go vet ./...
npm test
npm run build
```

`npm test` includes a real Go process restart and continuous ZIP round trip. Go integration
tests cover both archive formats, World/Age persistence, artwork, API retirement and the
continuous terrain/settlement pipeline. The historical `internal/legacyterrain` test suite
is separate; its baseline full run timed out, as recorded in the rebuild status.

Terrain archives imports ZIP files or folders. Continuous maps open in the new editor;
older maps show saved cells and authored objects. Original terrain fields, explored tiles
and artwork remain in exported archives. Old WFC controls and the previous illustrated-map
dialog have been removed.

See [Historical Worlds](docs/HISTORICAL_WORLDS.md) for the retained World/Age hierarchy.

Biosphere provides habitat-based ecological generation and editing, with guided previews,
reference and fictional lifeforms, biological rules, Age-independent histories and World
ZIP persistence. See [Biosphere architecture and workflow](docs/WORLDBUILDING.md#biosphere-version-1).

### Magic & Technology

The System Designer provides a structured workbench for principles, mechanics, capabilities, resources, methods, implementations, and applications. Review draft rule changes to trace dependencies, advisory conflicts, affected people and places, and recorded supply. Records remain independent across Ages and travel with World ZIP exports. See [System Designer](docs/SYSTEM_DESIGNER.md) for the data model, source queries, and validation limits.
