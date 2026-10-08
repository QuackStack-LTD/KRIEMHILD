# Containers, storage and releases

Run commands from the KRIEMHILD repository root. Docker Engine with Compose v2 is required. The image contains the React production bundle and a static Go binary; there is one application container, with optional PostgreSQL alongside it.

## SQLite: standalone default

```sh
docker compose up --build -d
docker compose ps
docker compose logs -f kriemhild
docker compose down
```

The builder is at http://127.0.0.1:8124. `down` preserves the named data volume. `down --volumes` deletes it, including the SQLite library. A database container is unnecessary in this mode: SQLite runs inside the Go process, with WAL and a busy timeout. H2 is a Java database and is not used.

To build/run directly:

```sh
docker build -t kriemhild:local .
docker run --rm --name kriemhild -p 127.0.0.1:8124:8124 -v kriemhild-data:/data kriemhild:local
```

## PostgreSQL: separate container

Copy `.env.example` to `.env` and replace the password with a strong URL-safe value (letters, digits, `_` and `-` work without escaping). Then:

```sh
docker compose -f compose.yml -f compose.postgres.yml up --build -d
docker compose -f compose.yml -f compose.postgres.yml ps
docker compose -f compose.yml -f compose.postgres.yml down
```

The overlay starts PostgreSQL 17, waits for its health check, and connects over the Compose network. PostgreSQL has its own named volume and no published host port. The app's `/data` volume holds working terrain caches. Keep both volumes when upgrading. Switching to SQLite selects its separate library; it does not copy PostgreSQL data. Use self-contained world ZIPs for portable transfer, and PostgreSQL's backup tools for whole-database backups.

For an existing database, set `DATABASE_URL=postgres://user:password@database-host:5432/kriemhild?sslmode=require` on the app. URI-encode special characters in credentials. The Compose overlay uses `sslmode=disable` for its private local connection; select appropriate TLS verification for a remote database. The database/user must already exist and permit creating the application's tables. Versioned schema initialization runs transactionally at startup.

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

**Save world ZIP** writes a complete archive to the database and downloads it. Imports also persist a complete snapshot. **Saved worlds → Open saved world** loads stored geography, explored detail, edits and camera state. The archive is the database's canonical project payload, so it has no dependencies on old session caches. Re-saving the same world updates its record atomically. List/open endpoints are `GET /api/projects` and `POST /api/projects/{worldID}/open`.

Exploration and edits after the last save remain session-local; save before stopping. Active sessions and undo history do not survive restarts. Old working cache directories may remain after a process crash; they are disposable once the app is stopped. Keep `kriemhild.sqlite` and its WAL together when backing up a running SQLite database, or stop the app first. Project size limits remain those in [WORLD_PROJECTS.md](WORLD_PROJECTS.md).

## Container readiness audit

| Area inspected | Result / implemented change |
| --- | --- |
| Frontend assets and API URLs | Vite produces local assets; relative `/api` calls work on the same origin served by Go. No Vite/Node process or CDN is needed at runtime. |
| Go runtime portability | Pure-Go SQLite and PostgreSQL drivers support `CGO_ENABLED=0`; Linux AMD64 and ARM64 binaries compile. |
| Listen and filesystem paths | Environment/flag configuration replaces fixed deployment assumptions; persistent state and cache are under `/data`. Startup checks the frontend build and storage. |
| Runtime permissions | Non-root UID/GID 10001, dropped capabilities, read-only root and writable volumes. Build context excludes local tools, databases, environment files and world archives. |
| Lifecycle | Exec entrypoint receives SIGTERM; Go drains HTTP requests for up to 30 seconds before closing its database. Compose allows 40 seconds. |
| Health | `/api/health` reports liveness; `/api/ready` also checks the database. The image's `-healthcheck` command needs no curl installation. |
| Persistence | SQLite and PostgreSQL store full ZIP snapshots; saved detail takes precedence over regeneration. ZIP import remains independent of either database. |
| Deployment boundary | Active sessions are in one Go process. Run one app instance; a shared database alone does not make in-flight sessions work across replicas. The builder has no authentication or per-user authorization; Compose binds localhost. Add an authenticated proxy before exposing a shared/public service. |

Image stages use Node 24 for frontend builds, Go 1.27 for backend builds, and Alpine 3.23 for runtime. Go source requires 1.26+. The Docker build runs Go tests/vet and focused frontend tests before assembling the runtime image.

## CI and manual releases

- `build.yml`: pushes to `main` build and load a local AMD64 image, then run container smoke tests. No registry login or image push.
- `pull_request.yml`: PRs targeting `main` build and smoke-test without registry credentials or publishing.
- `release.yml`: manually run **Release** in GitHub Actions with a new tag such as `v1.0.0` or `v1.1.0-rc.1`. It tests a local candidate, publishes AMD64/ARM64 images, and creates a GitHub release/tag for the selected commit with generated notes and the image digest.

The shared composite action defaults to `push: false`. Release publishing uses `GITHUB_TOKEN` with `packages: write` and `contents: write`; organization/package policy must allow that repository to publish. No personal token is required. The image namespace is derived from the lowercase repository owner/name: for this repository, `ghcr.io/quackstack-ltd/kriemhild`. Stable releases also update `latest`; prereleases only publish their version tag. Package visibility is controlled in GitHub's package settings. Existing tags are rejected. Workflows must be on the default branch before manual dispatch is available.

Actions are pinned to verified commit SHAs. BuildKit uses GitHub Actions cache storage; branch and PR builds never push images to a container registry.

## Verification

```sh
go test ./...
go vet ./...
npm run build
npm test
docker build -t kriemhild:ci .
bash scripts/container-smoke.sh kriemhild:ci
```

The smoke script requires Docker, Bash, curl and Python 3 (provided by the Ubuntu CI runner). It uses temporary isolated containers/volumes, checks frontend assets, non-root runtime and readiness, saves explored terrain, restarts the app, compares restored geography/tile bytes, and verifies graceful shutdown. It runs against both SQLite and a separate PostgreSQL container. `KRIEMHILD_TEST_DATABASE_URL` optionally enables the native Go PostgreSQL integration test against a test database.

Local validation on this change passed the production frontend build, all 24 Node integration tests, Go tests/vet, module checksum verification, Linux AMD64/ARM64 cross-compilation, both Compose configuration checks, Bash syntax checks and actionlint. The browser also reopened a database-backed world with its three stored detail tiles through the saved-world picker. Database outage tests confirm that readiness and saving return 503 while liveness remains available. Docker Desktop's engine was unresponsive and its service could not be started with the available Windows permissions, so the actual image build and PostgreSQL container smoke run could not be executed locally. Both are configured in CI; their successful completion is required before treating container runtime validation as complete.
