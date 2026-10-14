# Continuous terrain rebuild

Status: implementation in progress. The complete acceptance checklist is the supplied
implementation brief; a successful build alone does not complete this rebuild.

## Rollback and baseline

Branch: `rebuild/continuous-terrain`. Baseline tag: `terrain-rebuild-baseline-20261014`,
commit `2880921883e7b2e6f6fc62b28cbbfd575b415d50`.
Rollback directory: `C:/Users/tomyi/worldbuilding-backups/terrain-rebuild-20261014`.
It contains a Git bundle, binary working-tree patch, source tree including untracked
work and project data, both original Downloads ZIPs, and the exact implementation brief.
The repository was already dirty; those changes belong to the pre-rebuild baseline.

`dependency-inventory.txt` records file/line callers before any retirement. In particular:

* `httpapi/server.go` owns solver sessions, generation, legacy mutations and detail routes.
* `httpapi/project.go`, `database.go`, `autosave.go`, and storage project/archive code
  encode, restore and snapshot the solver, environment, detail tiles and authored state.
* `httpapi/history.go` attaches those archives to World/Age and forks snapshots.
* `world/model.go`, `surface.go`, `rivers.go` hold reusable authoring, IDs, spatial indexes,
  layer locks, undo, and landscape composition, but accept legacy terrain structures.
* `httpapi/geography.go`, `natural.go`, `climate.go`, `biosphere*.go` and `atlas.go`
  consume physical fields; they must remain available during migration.
* `TerrainApp.jsx` coordinates acceptance, project saves, routes and World/Age ownership.
  `terrain/controller.js`, `views/GeneratorView.jsx`, `builder/map.js` and
  `views/BuilderView.jsx` couple display to the old solver/detail geometry.

Preserve: World/Age identity and isolation, authored entities/relationships, seed determinism,
inspection, separate bed/water surfaces, rivers/lakes, climate/ecology access, portable
archives, autosave/manual-save semantics, exports, incremental edits and undo/redo.
Legacy archive restoration must read stored data, never call generation.

The new model must not import the legacy solver. A transitional adapter may expose sampled
fields to shared platform consumers while their contracts are migrated. Legacy removal is
gated on replacement coverage and migration tests, not on the age of a file.

Baseline commands: `.tools/go/bin/go.exe test ./...`, `npm.cmd test`, `npm.cmd run build`.
The `.cmd` entrypoint is necessary because this machine disallows unsigned PowerShell scripts.
Baseline browser: the existing server on 8124 opens Project Hub and lists the existing world
without console errors. Development/browser tests use a separate database and port.
