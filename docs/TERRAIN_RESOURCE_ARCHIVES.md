# Terrain resource archives

Observe a circle in Terrain Resources by its area in km². **Copy information**
copies the complete result as one paragraph, independently of which result
groups are expanded. Large inventories are searched and paged rather than
mounting every resource or lifeform in the map sidebar.

**Archive information** lists ordinary Library archives in the selected Age,
ordered by distance from the circle to their recorded settlement locations.
Archives without a usable location appear last. Choose an archive, an optional
integer year, and Markdown or plain text. Confirm creates a normal downloadable
Library document containing the complete report and a hidden structured
observation linked to its terrain, source snapshot, circle, and source Age.
The same actions are available in saved settlement resource profiles. Documents
can also be opened and exported through Writer's Archive, including PDF export.

## Reading recorded knowledge

Every ordinary archive exposes **Resources**, containing two choices:

- **Review Terrain Resources** opens the circles known to that archive, a
  geographic coverage map, current recorded statistics, pie charts, and history.
- **Resource documents** opens the reports under source Age, then Year or Undated.

These choices are available in Library and the linked Inworld Archive in
Writer's Archive. Opening a report also provides a link back to its terrain
statistics. Identical circles share a map entry; different overlapping circles
remain individually selectable. Multiple terrains have separate map selections.

Review reads retained observations, never a fresh terrain resource query. Loss,
destruction, deletion, recovery, transfer, copying, and Age inheritance follow
Library custody. A branch from a known year excludes reports dated after that
year. Transfers retain the source Age and year folders. Editing the readable
document does not rewrite its captured measurements.

Each Age has its own timeline and year axis. Year zero is a date; a blank year
is undated. Within an Age, later observation years replace older overlapping
knowledge; equal years use recording order. Undated reports contribute to the
latest recorded view after dated reports, but never to a dated historical view.
The review explicitly distinguishes missing observations from recorded zeroes.

## Overlap, statistics, and storage

Reports sample a deterministic hierarchical cube-sphere grid. Cell identities
are independent of a report's circle and work across the longitude seam and
poles. A subsequent overlapping observation retains older knowledge outside its
circle and replaces knowledge inside it. Comparisons report new, changed, and
unchanged samples. Backdated observations do not replace later dated knowledge.

Geographic boundaries and statistics are sampled approximations, not exact
polygon clipping. Normal Earth-sized observations use level 13 (about 1.27 km²
per cell on average); very small circles refine the grid and large observations
coarsen it to bound work. Review displays its sample resolution and known
coverage. Physical field means and biome fractions use spherical cell areas.
Different resolutions are resolved by location on the common grid hierarchy.

Resource occurrence identities deduplicate deposits across samples and reports.
Quantities retain their resource type and unit. Deposit quantities describe the
whole recorded deposit, not reserves clipped to the circle; river reach flow is
not summed as inventory. Biology is stored as shared community observations;
relative abundance is averaged over known biological observations, not treated
as a population estimate. Detailed lifeforms and the full original report load
only when opened.

Immutable sample values, resource occurrences, biological communities, and
report bodies use content hashes in the existing World blob store. Unchanged
overlaps reference existing values. Each report retains its bounded coverage
manifest and full readable document; saving the same area still intentionally
creates a new historical report. This is change detection with immutable
snapshots, not a second visible file collection. World ZIP schema 6 includes all
referenced observations and original terrain snapshots and validates their hashes.

## Performance and endpoints

Record-only archive reads and writes avoid projecting generated lifeform
catalogues. Resource calculations release the session lock before expensive
work. Proven archive-only revisions reuse ecological and terrain projections,
and name polling does not re-download river/coast geometry after those saves.
Small preview rasters use bounded immediate encoding to avoid Chromium's idle
PNG queue; large previews and exports retain asynchronous encoding.

- `GET /api/sessions/{id}/terrain/resource-archives`: nearby archive choices.
- `POST /api/sessions/{id}/terrain/resource-reports`: capture MD/TXT and samples.
- Saved settlement profiles expose the same suffixes under
  `/api/worlds/{world}/settlements/{entity}`, with an `age` query.
- `GET /api/worlds/{world}/resource-surveys?age=…&archive=…`: retained areas.
- Add `area={fileId}`, optional `sourceAge` and `year`, for saved statistics.
- Add `original=1` for the original captured report, or `lifeforms=1` for detailed
  recorded biology at the selected Age/year. Both are lazy reads.

Captures verify the preview fingerprint against the resource result before
committing. Access roles, CSRF checks, atomic Library writes, audit restoration,
and portable backup validation apply to the new endpoints.
