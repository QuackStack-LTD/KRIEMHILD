# Terrain alignment, overhead rendering and builder records

Baseline: `1bc76e840a286e3e222861b085e611bd27c2ce5f`, branch
`rebuild/continuous-terrain`. Repository and source ZIP verification from the
previous change remains in `docs/atlas-redesign/BASELINE.md`.

## Findings and changes

- The visible SVG measured 892 × 695 while the first renderer request used its
  1000 × 500 defaults. Measure the actual SVG synchronously before paint and
  observe subsequent resizes. Feature culling now includes letterboxed margins.
- A single retained close-up cannot cover a later zoom-out. Interactive previews
  retain one 512 × 256 world overview beneath the last coherent local frame.
  Both frames keep their geographic bounds; neither is repositioned to fit a
  new camera. Decoded publications are atomic, obsolete results are rejected,
  and superseded, uncommitted object URLs are released too.
- The default preview is `terrain`, replacing saved atlas preview styles. Its
  natural material, fixed physical normal stencil and world-space relief light
  are shared with the 3D terrain mesh. Analysis layers remain available. The
  Illustrator keeps its own style, export sizing and rendering requests.
- The former 3D scheduler estimated a rectangular region and discarded every
  tile after the nearest 24. The new scheduler refines a complete frustum cover,
  within 64 leaves, keeping parent coverage when the budget prevents refinement.
  It supersedes pending work, retains a coherent cover during loading and checks
  source revisions. The regional origin includes the authoritative focus height,
  so deep views start above the selected terrain rather than below it.
  Tile planning ignores sub-pixel orbit round-off; rendering occurs when visible
  content or the camera changes, rather than redrawing an idle view continuously.
- Authored roads, polygons, settlements, establishments and geographic features
  share their stored coordinates in 3D. Lines split at mesh triangle edges;
  polygon fills intersect the same triangles, preserving their overhead footprint
  while following relief and globe curvature. There is a “Look from above” view.
  Mesh caching is bounded to 132 entries during replacement; fill geometry has
  an 8 MiB budget, line geometry 24 MiB, and label textures 16 MiB. Dense-overlay
  limits are reported in the 3D status.
- Previewing a terrain and saving its standalone project never attached it to
  the surrounding World/Age. Entering a geographic builder, creating an entity,
  or saving in a World context now performs that attachment. Entity publication
  covers individual changes, settlement batches, family moves and undo/redo.
  Publication failure restores the local model. Only changed entities are synced,
  preserving unrelated profile edits. Every accepted drawable receives a record,
  including unnamed generated buildings; generated default names use stable IDs.
- Settlement clones now retain pending persistence flags. Previously a second
  accepted batch could save a history entry without saving the first batch's
  entity parts. Missing parts from affected files are recovered from their
  validated saved edit history at its cursor; existing parts and newer shared
  representations take precedence. Tombstones retain undo/redo across restart,
  including roots attached by older versions. Recovery counts are exposed in the
  session response, and recovered parts are persisted on the next save.
- The inspector opens the corresponding entity record. Atlas adds towns and all
  mapped places/infrastructure; cities and capitals appear in their collections.
  Capital, border and map-label kinds have editable record definitions.
- Small village previews could produce a camera narrower than the supported
  minimum and reset to the whole world on reopening. Their framing now respects
  the same minimum zoom as navigation and saved camera validation.

## Mountains and saved-world compatibility

New worlds explicitly select `reliefVersion: 1`. Missing/zero retains the old
model. The new model bends spherical plate boundaries, widens uplift belts,
varies crests and subsidiary ridges, and removes the climate-grid envelope that
imprinted regular troughs into materialized mountains. Terrain is materialized
before climate and drainage. Rendering never generates additional mountain noise.

At surface width 1024, paired runs of `Kriemhild`, `gallery-001`, and `gallery-002`
increased area-weighted land above 1500 m from 8.4% to 9.7%. This measures the
result of the structural model; no biome or elevation quotas are enforced.
Old surfaces round-trip byte-for-byte through the terrain codec. Existing saved
worlds are not regenerated, and zoom does not increase their finite source detail.

## Measured rendering repair

Profiling four saved-world tiles (z=9, x=700–703, y=262, size=256) attributed
92.9% of CPU time to channel sampling. Their original complete render times were
5.90–6.04 s. A bed-only query avoids unnecessary climate work for relief normals;
conservative channel envelopes reject distant segments before projection/hypot.
Refining the existing index cover removes irrelevant halo candidates without
broadening any saved world's candidate set or changing candidate order.
The same four renders then took 0.36–0.42 s, including sampling and PNG encoding.
Tests compare the optimized results with the former unfiltered bin cover across
variable widths, seams, poles and influence boundaries.

## Saved-world investigation

`jkj` initially had no attached maps, terrains or logical entities. Its separate
saved project contained the country Zenon and three house markers. Attaching the
project creates their records while preserving every coordinate and existing
terrain. No saved city was present in that project at inspection time.

The authoritative sample at normalized map coordinate (1.3436, 0.5145), inside
Zenon's stored footprint, is ocean: bed approximately -2883 m and coast distance
approximately -255 km. Correcting the display cannot recover an intended earlier
placement from those coordinates. No automatic relocation is performed.

A consistent pre-change SQLite backup is stored outside the repository at
`C:\Users\tomyi\worldbuilding\jkj-before-terrain-repair.sqlite`.
Browser experiments use a separate database copy and disposable entities.
The real `jkj` has now been attached: one terrain and four records. The shape
coordinate SHA-256 before and after was
`514717c33efce228deb05c6be125d005e82df1cb2b854a3828cc779677484ba5`;
the sampled terrain responses were unchanged.

## Verification

The release verification report records final browser, build, test and source
archive results. Regression coverage includes geographic transforms, pointer
anchoring, coherent zoom-out fallback and object-URL release, planar footprints
and terrain-following polygons/lines, bounded frustum coverage, material equality,
all-member batch publication, undo/redo, source compatibility and Illustrator
independence.

The existing retired `internal/legacyterrain` suite timed out in the verified
baseline. It is unchanged; active package results are recorded separately and
must not be described as a passing full legacy suite.
