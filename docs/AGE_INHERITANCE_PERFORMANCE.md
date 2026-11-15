# Large Biosphere Age inheritance

Creating an Age after accepting a large Biosphere exercised several independent
costs. A CPU profile of 8,000 lifeforms and 24,000 populations attributed about
74% of sampled time to linear `Document.Entity` lookups, chiefly population
pruning and taxonomic validation. Inheritance also scanned the growing
destination for each record. Quadrupling the catalogue from 2,000 to 8,000
lifeforms increased inheritance plus validation from 0.48 to 8.70 seconds.

The browser mounted every inheritance checkbox and name editor inside a closed
`details` element, repeating full-list membership checks during rendering and
typing. Database reads and writes built intermediate JSON arrays and record
maps. Every history command reloaded the full World separately for each open
map. Revision polling could download another complete World while the Age
transaction was in flight. Multi-Age responses exceeded 30 MB uncompressed.

## Changes

- Index destination identities once per inherited collection. Filter by source
  Age and merge policy before cloning; keep independent mutable state.
- Use operation-scoped entity indexes during validation and final inheritance
  reconciliation. Discard them on success and error, before further edits.
- Deep-copy biological profiles, habitat cells and profiles, occurrences, rules
  and settings directly rather than through JSON.
- Decode database rows directly into typed slices, serialize each record once,
  and reuse prepared insert statements. History commands retain compact row
  fingerprints during the read and upsert only changed/new rows, removing only
  vanished identities. They do not rewrite older Ages during inheritance.
  Transactions, revision checks, complete
  revision snapshots, restart and export behavior remain intact.
- Reuse the committed document for open-map projections. Creating an Age only
  advances existing maps' history token; their map revision and pending edits
  remain unchanged.
- Mount the inheritance review when opened and paginate/search it in groups of
  100. Selection and renaming cover the whole catalogue. Memoize merge review
  and index representations rather than scanning all geometry per entity.
- Pause name polling during local commands, immediately remember a successful
  command's revision, and ignore older in-flight responses.
- Negotiate streaming gzip for full World/history and accepted Biosphere
  responses. Plain JSON remains available to clients without gzip support.

## Measurements

Measured on Windows / Intel i5-6300HQ with fixed synthetic Biosphere fixtures;
these are diagnostics, not timing assertions in CI:

| Operation | Before | After |
| --- | ---: | ---: |
| Inherit + validate, 2,000 lifeforms | 476 ms | 27–37 ms |
| Inherit + validate, 8,000 lifeforms | 8,700 ms | 109–139 ms |
| Store update, 8,000 lifeforms, after lookup fix | 2,264 ms | 977–1,049 ms |
| Allocations during that store update | 528 MB | 207 MB |

Allocation figures are total allocated bytes per operation, not peak resident
memory. Store benchmarks include read, inheritance, validation and persistence;
they exclude authenticated audit snapshot serialization, HTTP transfer and UI
rendering. Worlds still load complete history, and distinct Age states and
audit revisions still occupy storage; this change does not introduce lazy
history loading or a new persistence schema.

The native browser check used the same 8,000-lifeform / 24,000-population World
across successive Ages. Its review mounted zero entity editors while closed and
100 rows while open. A 32,347,738-byte World response transferred as 714,175 bytes
with gzip. A subsequent Age response transferred 43,220,925 decoded bytes as
958,259 bytes; post-command revision checks were 19 bytes, with no duplicate
World download. These transfer figures are separate from the store benchmark.
The final incremental-save browser check retained five Ages with 8,000
lifeforms and 24,000 populations in each. Creating the fifth Age from the original
Age completed in about 5.8 seconds including the authenticated revision snapshot,
transfer, parsing and rendering; its 54,094,106 decoded bytes transferred as
1,202,420 bytes. Earlier Ages retained their original entity/population counts.

Reproduce the performance checks using the repository Go toolchain:

```powershell
.tools/go/bin/go.exe test ./internal/history -run '^$' -bench BenchmarkLargeBiosphereAge -benchtime=1x -benchmem
.tools/go/bin/go.exe test ./internal/storage -run '^$' -bench BenchmarkStoredBiosphereAge -benchtime=1x -benchmem
```

Regression coverage includes large-catalogue exclusions and renamed descendants,
taxon reconciliation, isolation of all mutable biological fields, lookup cleanup
after failed validation, database reopen, authenticated revision snapshots,
stale-write rejection, unchanged open maps, gzip negotiation, bounded browser
editors, filtered bulk selections, merge policies and polling races. The existing
generated-terrain Biosphere integration test also covers acceptance, inheritance,
restart and ZIP portability.

A database regression test installs triggers that reject any update or deletion
of the source Age's entity and Biosphere rows. Creating an Age and editing the
new records must succeed without touching those rows; subsequent Age deletion
must still remove the appropriate identities.
