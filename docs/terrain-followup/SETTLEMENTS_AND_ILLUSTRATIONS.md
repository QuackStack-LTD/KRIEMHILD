# Mountains, map drafts, connected land and illustrated profiles

Implementation started from clean commit
`0a19bf3e25dc921d6b38921ba2da6afcca22e352` on
`rebuild/continuous-terrain`. The original source ZIP/SDK investigation remains
in `../atlas-redesign/BASELINE.md`; it is independent of these changes.

## Terrain generation and existing worlds

New generation uses relief version 2. Each major continental nucleus contributes
a convergent pair of spherical plates in addition to the existing random plate
field. Compression passes through the existing curved, spatially warped plate
boundaries and variable crest field before climate and drainage are computed.
It is not a preview noise layer. Small island nuclei do not add plate pairs.

Three controlled base-grid seeds increased the share above 2500 m from
0.30–0.84% to 12.32–16.76%, with maximum elevations of 7305–9273 m versus
3385–5330 m. These are synthetic generation checks, not a claim that every
world has Earth's mountain distribution. Existing saved relief versions 0/1
retain their original algorithms and terrain. Generate a new terrain to use
the new mountain field; opening an old world does not regenerate it.

The Number of continents control sets 1–12 major landmass nuclei. Version 2
honors it for archipelago and supercontinent presets too. Coast erosion,
overlap and islands can make the number of connected dry pieces differ from
the requested number of major continents. 3D vertical exaggeration now allows
60, twice the previous maximum of 30; this is a display setting only.

## Drawing, naming and publication

Map Builder offers State, with country/state/kingdom/empire types. Settlement
Builder defaults to manually drawing a Settlement, with capital/city/town/
village/hamlet types in the selection panel. Its vertices are exactly the
clicked boundary. The former Site action generated a complete procedural town
layout and its irregular boundary. That separate optional action is now named
Generate layout; placing a manual Settlement does not invoke it.

New map shapes and accepted procedural streets/buildings are terrain drafts.
Save stores their geometry without adding records to Atlas. The frozen initial
placeholder name, empty names and generic generated labels cannot publish a
record. Changing the name alone also does not publish. A meaningful name plus
Apply changes explicitly publishes, preserving identity and geographic points.
The record button then uses the place's name. Subsequent shape edits update the
same record; existing published records remain published. Undoing first
publication returns the shape to a draft. Local drafts survive a cold reopen
and representation refresh. Fully submerged new settlement footprints are
rejected against authoritative terrain; coastal footprints may cross water.

## Illustrator selection and exports

Opening Medieval Illustrator first asks for a map name. Existing saved designs
can be resumed. A focused published place can produce a linked place map.
Otherwise Map Builder enters a temporary connected-land selection mode: click
a piece to select/toggle it, or drag a rectangle to add multiple islands.
Creating or cancelling the map exits that mode. Normal editing never toggles
land selection. The exporter fits the complete selected extent with padding,
keeps relative geographic positions, and masks unselected land. It does not
resize or move individual continents.

Connectivity follows saved surface triangles, including their diagonal and
longitude seam, rather than generator mass IDs. Lake holes remain holes.
Finite outlines close at map edges, including polar caps and all-land worlds.
Authored terrain operations participate in the selection surface. Revision
checks reject an obsolete selection, and a saved operation fingerprint requires
reselection if terrain changes before a selected-land illustration is saved.
Source data, queries, terrain coordinates and preview quality are independent.

Selection runs outside the session lock, is cancellable, and uses at most
64 MiB for flood labels/queue plus the source copy and bounded coastline maps.
More than 10,000 connected pieces, 300,000 coastline vertices, or an 80,000
vertex final map selection produces a diagnostic instead of unbounded work.
The existing bounded terrain cache holds derived selection responses.

The decorative map border has been removed. Saving produces two PNGs in one
transaction: a square map and a separate 3:1 banner with the decoration layer
(including compass, scale and title ornament) removed. The square map retains
the complete geographic extent; the banner uses a centered wide crop to fill
its image rather than letterboxing a square map in empty paper. Export width is bounded at 4096 for this
two-image operation; individual stored images retain the existing 16 MP limit,
and the combined upload is limited to 64 MiB. Both blobs travel in World export/
import and survive database restart. Legacy illustrations without a banner
fall back to their primary image until resaved.

Atlas settlement tiles display the saved map. Place profiles display its banner
at the full content width beside the navigation, with square corners and a
responsive 3:1 image. Profiles use the available content width for their fields.
Large maps remain in the Library's Illustrated maps inbox; focused maps link to
the place and its map folder. Each new scope/focus gets its own illustration
identity, while repeat saves of the same design preserve versions and filing.

## Why saving took time

Saving previously rebuilt export terrain across four independent style passes.
Those passes could evict and recompute the same physical samples. It also
recommitted unchanged design/source state. This was rendering work before the
PNG database write, not simply uploading an already available PNG.

Exports now visit every style of a terrain node together, keep three bounded
workers, reuse immutable export rasters in a separate 48 MiB cache, and reuse
the completed pair of PNGs when their dependencies are unchanged. Source,
design, geography and export settings participate in cache identity. Preview
quality never does. Progress names the render and save phases separately.
The debug log `window.__illustrationSaveLog` retains the last 20 timing samples;
the API also returns Server-Timing for the save request.

One browser run on an expendable copy of jjj measured 8.24 s for the first save
(6.31 s rasterization, 0.28 s save request) and 0.19 s for an unchanged repeat
(cached PNGs, 0.12 s save request). These are measured examples, not a fixed
latency guarantee. A new design or changed terrain still needs export rendering.

## Reproduction and validation

Mutating browser regressions use a consistent, expendable database copy on
port 8152. They cover manual State/Settlement typing, placeholder drafts,
explicit publication, house naming, four-vertex footprints, linked illustration
images, desktop/mobile banner geometry, connected land rectangle selection,
label painting, identical PNG hashes across preview qualities, cached repeat
save, cold reopen, deep zoom inspection/alignment, coherent frame loading, 3D
overlays and the new exaggeration limit. Set the fixture IDs described in the
browser tests; do not run mutating fixtures against an author's database.

Deterministic tests cover connected seam fragments, separate islands, polar/
all-land areas, cancellation, selection holes, immutability, cache budget,
requested generation counts, relief versions, draft save/reopen/undo,
procedural batch publication, banner storage and portable export/import.

The release's clean commit archive is verified by `.cache/verify-active-source.mjs`:
every extracted file must match its Git blob, then npm ci, frontend build,
JavaScript tests, Go build and every active Go package run from the extraction.
The external verification.json records exact commit, archive size/hash, commands
and outcomes. The unchanged retired legacyterrain suite retains its documented
baseline timeout; active-package verification is not a passing full legacy suite.
No local SDK, database, cache, credentials or test-copy maps belong in the ZIP.
