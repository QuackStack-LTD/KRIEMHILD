# Source and archive investigation

Investigated on 2026-10-23, before application changes. Checkout was clean.
Branch: `rebuild/continuous-terrain`. Commit:
`9d3e88f8e56144600ea591544d113f4ba984ceb6`.
Origin: `https://github.com/QuackStack-LTD/KRIEMHILD`.

## Exact reported ZIP

The user supplied `../KRIEMHILD-rebuild-continuous-terrain.zip`.
Size: **316,449,990 bytes**. SHA-256:
`79c12fcf37b588099fb58c5d9328b31f2cf44fe2e396c4af02369f477564d6e7`.
Its ZIP comment identifies the investigated commit above. Its top directory is
`KRIEMHILD-rebuild-continuous-terrain/`; it has 3,631 directory/file entries.
`tar -tf` and extraction to a unique empty temporary directory both returned **0**,
with empty stderr. Every one of the **3,500** extracted files matches its Git blob
hash at that commit. No tracked file is missing or changed. There are no submodules,
LFS pointers, or case-insensitive file collisions in this revision.

The original inspection error message/tool was not supplied. It could not be
reproduced using these checks; this does not establish what failed in that tool.

## Local compression warning

`git ls-files .tools/go` is empty; `git check-ignore -v .tools/go` reports
`.gitignore:4:.tools/`. README and `scripts/start.mjs` explicitly recognize a
portable Go installation there. `.tools/go/VERSION` identifies **go1.27.1**.
This is an optional local SDK, not KRIEMHILD source. Nothing was removed or untracked.

The literal reported `foo.go` path does not exist. The directory contains
**Þfoo.go** and **Þmain.go** (leading character **U+00DE**, Latin capital thorn).
They contain Go Authors copyright/license headers and Unicode package/import
identifiers; the adjacent `issue27836.go` is an upstream `compiledir` fixture.
Both files have ordinary `Archive` attributes. Their layout must remain intact.

A controlled `tar -a -cf` ZIP of this fixture succeeded but **changed the leading
thorn to `_` in both filenames**, verified by reading ZIP entries with .NET.
`.NET ZipFile.CreateFromDirectory` with explicit UTF-8 preserved U+00DE. Thus a
local archive utility/encoding issue is demonstrated; the original Windows
compression warning remains tool-specific. Use a commit source archive, not a
ZIP of the entire checkout or a renamed SDK fixture.

## Clean commit archive and builds

`git archive --format=zip HEAD` produced **316,289,082 bytes**, SHA-256
`ec94a4b5f13767bc84020903816260fc2a3e1380ae5e0d55b20fa7364141e80a`.
Listing/extraction succeeded; the manifest contained all 3,500 tracked files.
Builds were run from the clean extracted tree, with the external portable Go SDK
on PATH. Local credentials, databases, cached terrain, dist and node_modules
were not copied. Runtime: **Go 1.27.1 windows/amd64**, **Node v24.21.0**,
**npm 11.19.0**. This frontend is **Vite/React**, not Next.js.

| Check | Baseline result |
| --- | --- |
| `go env GOMOD` | Extracted tree's go.mod |
| `go build ./...` | PASS (checkout and extracted source) |
| `go test ./... -timeout 3m` | FAIL: HTTP and legacy packages time out; extracted generation passes in 165.495s |
| `npm.cmd ci` | PASS: 179 packages, zero audit vulnerabilities reported |
| `npm.cmd run build` | PASS (17.90s) |
| `npm.cmd test` | FAIL: 85 pass, 3 test files fail because `.tools` does not exist |

The checkout Go run also timed out in generation under concurrent baseline work;
the complete outputs are recorded alongside this document. A three-minute package
limit is a bounded baseline, not evidence that all suites would fail at a longer
timeout. Existing documentation also records legacy full-suite timeouts.

Two reproducible source/setup defects are fixed before renderer work:
the Dockerfile copied three deleted test paths, and three JS UI test modules
assumed a pre-existing ignored `.tools` parent directory. Tests now create it.
PowerShell's policy blocks `npm.ps1` here; `npm.cmd` works without policy changes.

## Reproduce

Install the README's supported Node and Go versions. Run:

```powershell
node scripts/verify-source.mjs HEAD
```

This creates a unique external output directory, archives the selected commit,
checks Windows filenames/submodules/LFS, lists and extracts the ZIP, verifies
every file against its Git blob, then independently records backend build/tests,
frozen npm installation, JS tests and frontend build. It saves complete output,
exit codes, timings, commit, size and SHA-256 in `verification.json` and log files.
The default Go package test limit is 10m; `KRIEMHILD_TEST_TIMEOUT` overrides it.
It never archives uncommitted work or deletes an existing destination.
