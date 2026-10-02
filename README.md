# KRIEMHILD
KRIEMHILD - Knowledge Record of Intertwined Eras, Maps, Histories, &amp; Invented Legendary Domains

A local-first worldbuilding and writing platform built around independent historical Ages, with a Next.js frontend and primarily Go backend. Creative decisions belong to the author; AI and LLM features are excluded.

The development build now includes connected society/domain dossiers, story planning, family graphs, local and tactical maps, storyboards, author-controlled language/economy/terrain experiments, portable backups and text imports, Git conflict review, publication exports, and optional hosted/shared writing. These extend the P1/P2 Age, terrain, time and manuscript foundations. See the [current development guide](docs/DEVELOPMENT_GUIDE.md) for workflows and the [phase status](docs/IMPLEMENTATION_STATUS.md) for implemented scope and open acceptance gates. This is a development build, not a declaration that every roadmap phase is finished.

## Run

Prerequisites: Node.js 22 or newer, npm, and Go 1.25 or newer. On this development machine a verified Go toolchain is already available in the ignored `.tools/go` folder; the scripts find it automatically. Other machines can install Go normally or set `KRIEMHILD_GO` to the Go executable.

From the `KRIEMHILD` directory:

```powershell
npm.cmd run setup
npm.cmd run build
npm.cmd start
```

Open **http://127.0.0.1:4784** for the current development build. In shells other than Windows PowerShell, use `npm` in place of `npm.cmd`. Setup downloads dependencies; the built application works without an internet connection. No Node server is required at runtime.

The preserved P1 preview on this development machine uses **http://127.0.0.1:4780**, `worlds/`, `bin/kriemhild.exe` and `apps/web/out/`. The preserved P2 preview uses **http://127.0.0.1:4782**, `worlds-p2/`, `bin/kriemhild-p2.exe` and `apps/web/out-p2/`. Current builds use separate paths and port-scoped sessions. Native import can migrate a format-1 backup into a new format-2 world without changing its source. Open projects with new optional features using the current build.

For this machine, dependencies and the compiled application have already been prepared, so `npm.cmd start` is sufficient when the server is not already running. Stop it with Ctrl+C. Stop the server before rebuilding its executable on Windows.

Current projects are stored in `worlds-dev/<world UUID>/`, independently of Git. Choose another library or port with:

```powershell
npm.cmd start -- -data "C:\My Worlds" -addr 127.0.0.1:4784
```

You can also run `bin/kriemhild-dev.exe` directly (or `bin/kriemhild-dev` on Linux/macOS). Its default frontend directory is `apps/web/out-dev`. Pass `-web` to choose another directory. Local mode binds to loopback. Hosted mode requires an explicit origin and accounts; see the development guide before deploying it.

`npm.cmd run demo` creates a **new** three-Age reference world, printing its URL. `npm.cmd run package` creates a self-contained local package under `dist/`; no Go or Node runtime is needed to use that package. An existing package directory is never overwritten. Set `KRIEMHILD_PACKAGE_DIR` to a fresh output directory to rebuild a package.

## Test

```powershell
npm.cmd test
npm.cmd --prefix apps/web exec -- playwright install chromium
npm.cmd run test:e2e
```

`npm test` runs Go storage, API, domain, migration, Git, map, experiment and permission tests, Go vet and TypeScript checks. Browser tests require a current `npm run build`; they start isolated local and hosted servers on ports 4781 and 4783, using only the ignored `.test-worlds-dev` and `.test-hosted-dev` libraries. The browser installation is needed once per Playwright browser version.

GitHub Actions is configured to run the build and validation suite on **every push to `main` and every pull request targeting `main`**, on Ubuntu and Windows. The [verification workflow](.github/workflows/verification.yml) also checks Go races on Linux, builds the Docker image and produces Windows/Linux/macOS package artifacts. Browser reports and failure traces are retained for seven days; packages for fourteen days. These configured jobs are not evidence of a remote CI run until the changes are pushed. Focused browser tests (`test.only`) fail CI.

The validation tests are included in the repository:

- [Storage, Age isolation and crash recovery](internal/project/store_test.go)
- [Rich-text document validation](internal/project/model_test.go)
- [HTTP API, image upload and access boundaries](internal/httpapi/server_test.go)
- [Browser acceptance and recovery workflows](apps/web/tests/p1.spec.ts)
- [Terrain, dated history, calendars and format safety](internal/project/p2_test.go)
- [P2 flood and historical writing journey](apps/web/tests/p2.spec.ts)
- [Domains, migration, semantic Git merging and shared drafts](internal/project/later_phases_test.go)
- [Local maps, conlang, text imports, family paths and cache integrity](internal/project/deep_authoring_test.go)
- [True format-1 archive migration](internal/project/legacy_import_test.go)
- [Hosted authentication and role enforcement](internal/httpapi/access_test.go)
- [Connected writing and concurrent browser editing](apps/web/tests/later-phases.spec.ts)
- [Keyboard map editing, offline editions and text import](apps/web/tests/deep-authoring.spec.ts)

`npm.cmd run benchmark` measures a small, reproducible storage/API workload on isolated port 4786. It stores its report in `.tools/benchmark-latest.json`. This baseline does not satisfy the roadmap's much larger reference-world, rendering or accessibility gates.

See [the P1 guide](docs/P1_GUIDE.md) for a manual acceptance walkthrough, recovery instructions, API details and the current boundaries. See [the storage format](docs/P1_STORAGE.md) for the save protocol and historical invariants.

## Planning and research

- [Comprehensive platform plan](docs/PLATFORM_PLAN.md): product workflows, domain features, historical semantics, architecture, persistence, implementation phases and acceptance gates.
- [Assessment of all 80 reference repositories](docs/REPOSITORY_ASSESSMENT.md): useful ideas, reuse candidates, exclusions and optional service decisions for every checkout.
- [Specification coverage](docs/SPECIFICATION_COVERAGE.md): all 65 specification sections, first-release requirements and resolved ambiguities.
- [Research inventory](docs/research/repository-inventory.json): local origins, revisions and evidence paths. Research scripts expect the original sibling repositories in the surrounding worldbuilding directory.
