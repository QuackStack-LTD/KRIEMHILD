# Zoom alignment, loading feedback, and saved illustrations

Investigated from `d9e86bbbe6197ff90f3e3f4755c808f4f1f99820` on
`rebuild/continuous-terrain`. The repository/archive investigation and known
retired test timeout remain documented in `../atlas-redesign/BASELINE.md`.
Neither a ZIP error nor generation was used to explain this rendering defect.

## Reproduction and repair

The latest saved test World was **jjj**, ID
`0d2648a7662a160a3a53a23023d90e8c`, containing Zenon and its settlement map.
Reproduction and all mutating browser tests used an independent SQLite copy on
port 8152. A consistent pre-investigation backup is in the parent directory as
`jjj-before-coordinate-repair.sqlite`. Test illustrations are not installed in
the original database.

The old SVG magnified absolute normalized coordinates through its `viewBox`.
Its float precision differed from the double precision screen-to-world query.
At Zenon's first boundary vertex, camera widths 0.1, 0.01, 0.001 and 0.00008
produced measured SVG/query discrepancies of approximately 0.00047, 0.00905,
0.06990 and **0.85061 display pixels**. A near-shore click could consequently
query the opposite side of the rendered boundary.

The common projection now subtracts the geographic camera origin in JavaScript
before sending viewport pixel coordinates to SVG. Raster bounds, geographic
features, editing handles, labels and selection feedback share that projection.
Inspection continues to query authoritative data independently. Saved geometry
and coordinates are unchanged. The browser regression compares actual mouse
event coordinates with the query URL, accounting for device pointer rounding.

Retained raster frames are cropped into bounded nested SVG viewports. A world
overview no longer becomes an enormous displayed image at editing zoom. The
root SVG CSS rule is scoped to the root, so it cannot stretch those nested
viewports. Replacement frames retain their own bounds and publish atomically.
Offscreen vector geometry is clipped only for display; a clipped polygon's
original outline is drawn separately so viewport edges do not become borders.

River presentation previously used fixed geographic simplification tolerances,
which could move bends substantially when a detail level changed. Its tolerance
is now also bounded by 0.2 display pixels. Source topology, confluences, lake
connections and stored reach paths are not rewritten by presentation.

The new `river-mask` samples the same authoritative water and authored edits as
the terrain raster, in the same frame transaction. River strokes are hidden
over lakes and oceans while channel strokes remain visible. Illustrator and
normal preview use the mask; 3D uses numeric mesh water classifications to
suppress bulk-water sections. Raster/mesh cache revisions changed accordingly.
The source network can still pass through a lake without drawing a line across
its surface. Source-resolution limits remain visible; zoom adds no new relief.

Detail loading has a visible status/progress indicator. Progress updates are
coalesced, and obsolete requests cannot update it or replace a newer frame.
The previous coherent frame remains visible while replacement tiles load.

## Save illustration and Library

**Save illustration** first persists the source design and terrain, renders the
complete illustration extent at its independent export dimensions, and saves
a PNG to the database. It checks the current revision after rendering. A browser
test exposed a stale React closure that requested pre-save terrain revisions;
export requests now use the captured current session/revision.

- World/continent maps go into Library's **Illustrated maps** inbox.
- Focused maps go into a folder named for their place, with a link to its record.
- The focused record displays the saved map as its banner and links back to it.
- Existing Library filing controls can move maps later; subsequent saves respect
  the chosen folder.
- Repeat saves append immutable image versions; identical images are deduplicated.
- The file, folder, record link, banner and PNG commit in one transaction.
- Stale source/world revisions, missing places and invalid/truncated PNGs reject
  the save without partial records. Dimensions and file size are bounded.
- Backups, restarts and portable World bundles preserve the images and versions.

The editable design remains in the source map and image metadata; existing SVG,
PNG and PDF exports remain available. Preview quality does not set illustration
dimensions or output quality. A browser regression changes preview quality and
verifies the saved illustration's PNG hash is unchanged.

User-facing built-in wording uses names and records/places instead of
"entity/entities". The Map Builder and Library use **Open Zenon record**, for
example. Internal API identifiers and user-authored names are preserved.

## Verification

- All 23 active Go packages passed, including HTTP, storage, world, terrain,
  hydrology and mesh tests. New tests cover atomic illustration persistence,
  invalid/stale requests, versions, manual filing, banners, restart and portable
  bundles, safe image delivery, lake masking and numeric mesh water values.
- Four new JavaScript regressions cover deep-zoom projection, bounded raster
  crops, clipping without invented borders, subpixel simplification and bulk
  water classification. Existing tests cover cancellation, stale selections,
  atomic publications, overview retention, coordinates and export isolation.
- Both browser regressions passed against the expendable jjj copy. They check
  painted country labels, database illustration saving, quality-independent PNG
  hashes, Library/record/banner navigation, cold reopen, unchanged coordinates,
  boundary alignment and actual inspection coordinates at four zooms, bounded
  raster display sizes and loading feedback while retaining the previous frame.
- Production frontend builds and `node scripts/check-ui.mjs` passed.

Run the mutating browser tests using the environment variables documented in
`SAVE_AND_ILLUSTRATOR.md`. Never point them at the user's original database.
Clean archive verification records the final commit, every extracted blob hash,
runtime versions, installation, build and test results in `verification.json`
beside the generated archive. The unchanged retired `internal/legacyterrain`
suite retains its documented ten-minute timeout; active-package success does
not mean that retired suite passed.
