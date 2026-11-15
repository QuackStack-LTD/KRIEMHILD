# Containers, storage and releases

Run commands from the KRIEMHILD repository root. Docker Engine with Compose v2 is required. The image contains the React production bundle and a static Go binary; there is one application container, with optional PostgreSQL alongside it.

## PostgreSQL: Docker Compose default

Copy `.env.example` to `.env` and replace the password with a strong URL-safe value (letters, digits, `_` and `-` need no URI escaping). The sole `docker-compose.yml` contains both services:

```sh
docker compose up -d
docker compose ps
docker compose logs -f kriemhild
docker compose down
```

Compose pulls `ghcr.io/quackstack-ltd/kriemhild:latest` on startup; it does not build the application locally. The builder is at http://127.0.0.1:8124. `down` preserves both named data volumes. `down --volumes` deletes them, including the world library. Compose waits for PostgreSQL's health check before starting the app. PostgreSQL has no published host port.

## SQLite: standalone fallback

To build/run the image directly without an external database:

```sh
docker build -t kriemhild:local .
docker run --rm --name kriemhild -p 127.0.0.1:8124:8124 -v kriemhild-data:/data kriemhild:local
```

With no database URL, SQLite runs inside Go with WAL and a busy timeout; H2 is not used. Keep `/data` when upgrading. Switching database modes selects a separate library; it does not copy data. Use self-contained world ZIPs for portable transfer, and PostgreSQL's backup tools for whole-database backups.

For an existing database, set `DATABASE_URL=postgres://user:password@database-host:5432/kriemhild?sslmode=require` on the app. URI-encode special characters in credentials. Compose uses `sslmode=disable` for its private local connection; select appropriate TLS verification for a remote database. The database/user must already exist and permit creating the application's tables. Versioned schema initialization runs transactionally at startup.

When no database URL is provided, SQLite is automatic. An explicitly configured PostgreSQL connection that fails causes startup to fail; a later outage makes readiness return 503. It never silently saves worlds to another database. For secret mounts, set `DATABASE_URL_FILE` to a readable file containing the URL; do not set both variables.

## Configuration and persistence

| Setting | Native default | Container default |
| --- | --- | --- |
| `KRIEMHILD_ADDR` / `-addr` | `127.0.0.1:8124` | `0.0.0.0:8124` |
| `KRIEMHILD_DIST` / `-dist` | `dist` | `/app/dist` |
| `KRIEMHILD_DATA_DIR` / `-data` | `data` | `/data` |
| `DATABASE_URL` | unset: SQLite | unset: SQLite |
| `DATABASE_URL_FILE` | unset | unset |

Explicit flags override environment defaults. `/data` must be writable by UID/GID 10001; named volumes receive the image directory's ownership. For host bind mounts, create the directory with that ownership first. The app works with a read-only root filesystem and writable `/data` and `/tmp`, as configured in Compose.

Accept generated terrain into a World and Age with **Save terrain**. Attached terrain edits and view changes then save automatically. Replacement generation previews and imported worlds retain their explicit acceptance controls. **Export ZIP** creates a separate portable backup without changing the database. API endpoints include `POST /api/sessions/{id}/save`, `GET /api/projects`, `POST /api/projects/{worldID}/open` and `DELETE /api/projects/{worldID}`. See [saving and deletion](AUTOSAVE.md).

Check the save indicator if a persistence error appears; unsaved previews do not survive a server restart. Old working cache directories may remain after a crash; they are disposable once the app is stopped. Keep the SQLite database and its WAL together when backing up a running database, or stop the app first. Portable ZIP limits remain those in [WORLD_PROJECTS.md](WORLD_PROJECTS.md).

## Container readiness audit

| Area inspected | Result / implemented change |
| --- | --- |
| Frontend assets and API URLs | Vite produces local assets; relative `/api` calls work on the same origin served by Go. No Vite/Node process or CDN is needed at runtime. |
| Go runtime portability | Pure-Go SQLite and PostgreSQL drivers support `CGO_ENABLED=0`; Linux AMD64 and ARM64 binaries compile. |
| Listen and filesystem paths | Environment/flag configuration replaces fixed deployment assumptions; persistent state and cache are under `/data`. Startup checks the frontend build and storage. |
| Runtime permissions | Non-root UID/GID 10001, dropped capabilities, read-only root and writable volumes. Build context excludes local tools, databases, environment files and world archives. |
| Lifecycle | Exec entrypoint receives SIGTERM; Go drains HTTP requests for up to 30 seconds before closing its database. Compose allows 40 seconds. |
| Health | `/api/health` reports liveness; `/api/ready` also checks the database. The image's `-healthcheck` command needs no curl installation. |
| Persistence | SQLite and PostgreSQL store automatic solver/view/tile checkpoints; saved detail takes precedence over regeneration. Legacy ZIP snapshots remain readable. |
| Deployment boundary | Active sessions are in one Go process. Run one app instance; a shared database alone does not make in-flight sessions work across replicas. The image defaults to OIDC authentication; the smoke test explicitly uses local mode on loopback. Configure authentication before deployment. |

Image stages use Node 24 for frontend builds, Go 1.27 for backend builds, and Alpine 3.23 for runtime. Go source requires 1.26+. The Docker build runs Go tests/vet and focused frontend tests before assembling the runtime image.

## CI and automatic releases

- `pull-request.yml` runs `bash scripts/ci/pull-request.sh` for PRs targeting
  `main`. It builds a local AMD64 image and checks SQLite/PostgreSQL persistence.
  It has read-only repository permissions and never logs into GHCR or publishes.
- `build.yml` runs `bash scripts/ci/build.sh` for pushes to `main`. After the same
  checks pass, it builds and publishes AMD64/ARM64 images to GHCR and creates a
  GitHub release for that commit. The separate release workflow was removed.

Publishing uses `GITHUB_TOKEN` with `packages: write` and `contents: write`;
organization/package policy must allow that repository to publish. No personal
token is required. The image namespace is derived from the lowercase repository
owner/name: `ghcr.io/quackstack-ltd/kriemhild` for this repository. Each successful
main build publishes `sha-<full commit SHA>`, `main`, and `latest`. Its release tag
is `main-<full commit SHA>` and its notes include the image digest. Rerunning the
same commit updates that release instead of attempting a duplicate tag. Package
visibility is controlled in GitHub's package settings.

Actions are pinned to verified commit SHAs. The shared setup action configures
Buildx/QEMU and restores a local BuildKit cache with GitHub Actions cache storage.
The Bash helper in `scripts/ci/image.sh` supplies the same Docker build arguments
to both entry points. Publishing starts only after the candidate passes its
checks, and the main entry point rejects branch/PR invocations. Main release
runs are serialized rather than canceled during publication.

## Verification

```sh
go test -timeout 30m ./...
go vet ./...
npm run build
npm test
docker build -t kriemhild:ci .
bash scripts/container-smoke.sh kriemhild:ci
```

The smoke script requires Docker, Bash, curl and Python 3 (provided by the Ubuntu
CI runner). It uses temporary isolated containers/volumes, checks frontend assets,
non-root runtime and readiness, generates continuous terrain, accepts it into a
World, edits it, inspects resources, and saves its view automatically. It then
restarts the app, compares restored geography/tile bytes and camera settings,
exports ZIP, and verifies graceful shutdown. It runs against both SQLite and a
separate PostgreSQL container. Retired legacy generation endpoints are not used.
`KRIEMHILD_TEST_DATABASE_URL` optionally enables native Go PostgreSQL tests.

The production frontend build completes without exceeding the unchanged 600 kB JavaScript chunk warning threshold. PDF export loads its font engine and compression/unicode dependencies in separate optional chunks. Validation includes the Node suite, multilingual PDF subset tests, Writer's Archive browser tests, workflow actionlint and Bash syntax checks. The native Go run passed all packages except the legacy terrain package, which exceeded the default aggregate 10-minute limit; container builds now allow 30 minutes without excluding tests. Docker Engine was unavailable locally, so image execution and PostgreSQL container smoke tests remain unverified locally; both workflows run these checks before publication.
