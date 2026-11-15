# Terrain inspection and World opening after Biosphere generation

## Causes and changes

Terrain display requests previously read the complete historical document,
including species, populations and habitat suitability profiles from every Age.
Resource inspection also rebuilt the terrain-derived habitat source repeatedly.
These allocations caused garbage-collection pressure; the shared authorization
gate then made expensive rendering delay otherwise unrelated requests.

Storage now has separate read models for World metadata, Atlas navigation, map
projection, one terrain's ecology and Biosphere editing. Map rendering resolves
canonical identities only for represented entities. Atlas navigation skips the
biological catalogue; entity editors and Age inheritance load it on demand.
Workspace responses omit per-cell generation profiles. Full reads, transactions
and exports retain those profiles and preserve saved data.

Ecology display data is shared across inspections in one revision-aware cache,
bounded to 32 MiB for the entire server. Cells share immutable community slices.
New Biospheres record the immutable source snapshot, avoiding habitat derivation
just to establish freshness. Older records still verify against their saved
terrain once per snapshot. Names, Age selection and World revisions invalidate
the appropriate display data.

Continuous editor startup requests compact metadata instead of copying legacy
environmental arrays into the browser. Atlas thumbnails decode only their five
display fields, plus resource geometry when explicitly requested. Immutable base
thumbnails reuse the existing bounded presentation cache; historical resource
and habitat highlights bypass this cache. Resource lists
render 100 species at a time with search over the complete returned list.
Navigation aborts obsolete World loads and releases biological records on return
to the Atlas or terrain editor.

Idle terrain sessions are now actually removed after the documented 30 minutes;
busy sessions are retried at a later sweep. River sampler and hydrology indexes
belong to their planet rather than a global strong-reference cache. Physical tile
keys use weak planet identity and cached sample buffers have a 64 MiB byte limit,
in addition to the eight-entry limit. Tile disk eviction scans are amortized,
and legacy checkpoint restoration no longer decompresses each saved tile twice.
The editor also bounds its retained dirty-region history.

Immutable terrain sampling/rendering releases the shared access gate while it
computes. Authentication and authorization are checked again before the buffered
result is disclosed, including revocation during an in-flight request.

## Measurements

Local Windows Go benchmarks, representative fixed fixtures; allocation numbers
are cumulative allocations per operation, not process resident memory:

| Operation | Before | Revised |
| --- | ---: | ---: |
| Repeated resource inspection with generated Biosphere | 829 ms / 323 MB | 4.5–4.6 ms / 143 KB |
| First resource inspection, empty ecology cache | 829 ms / 323 MB | 144–149 ms / 53.5 MB |
| Historical map display read, 2,000 species across three Ages | 158–199 ms / 44.9 MB | 14–16 ms / 93–97 KB |
| World metadata read, same fixture | Full document required | 0.25–0.33 ms / 13–15 KB |
| Thumbnail environment decoding | 169 ms / 41.3 MB | 89 ms / 22.6 MB |

Native browser verification used an isolated saved continuous terrain with
22,531 lifeforms, 33,170 populations and 9,987 habitats. Opening its World fetched
a 395,328-byte decoded Atlas document (153,921 bytes compressed), without fetching
the biological catalogue. Terrain metadata was 922 bytes. Saved Biosphere editing
still returned all 33,170 populations and 9,987 habitats, with no per-cell profiles.
The terrain snapshot identity remained unchanged. Terrain opening and first
inspection still perform physical terrain I/O and initial index construction;
cached fixture results are not a latency guarantee for every world or machine.
The native resource panel completed a repeated inspection in 97 ms on this large
world while the broader regression suite was running.
Its final Atlas document loaded in 111 ms; repeated server-side thumbnail reads
took 4–5 ms and reported cache hits, with browser caching explicitly bypassed.

## Regression coverage

Entity and terrain renames now use an atomic projected transaction. It reads
the target identity and affected map labels rather than validating and encoding
every Age's generated Biosphere. Audit revisions store a small name-only patch;
restoration still reconstructs the complete historical document. The browser
merges `POST /api/worlds/{world}?view=changes` responses and requests compact
`names?revision=…&delta=names` updates across open pages. Older clients can still
request complete views. Mixed or missing audit chains use the normal read path.

Open maps rebase canonical labels and undo history without replacing pending
geometry, rebuilding spatial indexes, or incrementing the raster revision.
Natural inspection labels invalidate independently of pixels. Species renames
refresh ecological labels; unrelated names reuse the ecological projection.
Profile renames also retain genuine drafts without producing a false unsaved
navigation warning from changed timestamps or canonical-name metadata.

On the disposable Edek copy, a rename with a metadata response originally took
19.5 seconds. Repeated warm requests after the change took 6–16 ms; compact name
polls took 3–4 ms. These are endpoint measurements, not initial page-load times.
The two-page browser test completed a profile rename in 261 ms and a terrain
rename plus synchronization to the other page in 802 ms, with no terrain tile,
river/coast geometry, or full World reload. Busy editors receive the committed
labels on their next request rather than blocking the rename operation.
The stored-world benchmark with 8,000 species went from about 705 ms / 186 MB
allocated per rename to 4 ms / 22 KB. Reproduce it with
`go test ./internal/storage -run '^$' -bench BenchmarkStoredEntityRename -benchmem`.

The [resource archive workflow](TERRAIN_RESOURCE_ARCHIVES.md) adds projected
Library reads/writes and reuses ecology across proven record-only revisions.
Name polling returns a compact unchanged result for those revisions; the map
downloads river/coast geometry again only when its physical source changes.
Resource calculation no longer holds the session lock while projecting ecology
and sampling the circle.

In the saved Edek-copy browser audit, three overlapping report saves and a
subsequent polling interval made only one initial physical-features request.
With warm physical tiles, the final preview committed in about 0.85 seconds;
small-frame PNG encoding took 3–25 ms instead of roughly two seconds per stage
through Chromium's asynchronous idle queue. These are local measurements, not
cold-load guarantees. Large raster outputs keep asynchronous encoding.

Tests cover read-model completeness and saved-state preservation, canonical name
and Age isolation, old and new provenance, compact editor metadata, idle cleanup,
garbage collection of closed planets, physical-cache byte eviction, thumbnail
pixel preservation, concurrent edits and in-flight revocation. UI tests cover
bounded species lists, navigation reuse, canceled loads and on-demand inheritance
records. Existing terrain rendering, save/import/export, history, authorization
and Biosphere tests exercise compatibility.

Reproduce the focused benchmarks with:

```powershell
.tools/go/bin/go.exe test ./internal/httpapi -run '^$' -bench BenchmarkBiosphereTerrainDetails -benchmem
.tools/go/bin/go.exe test ./internal/storage -run '^$' -bench BenchmarkTerrainHistoryReads -benchmem
.tools/go/bin/go.exe test ./internal/httpapi -run '^$' -bench BenchmarkAtlasEnvironmentReads -benchmem
```
