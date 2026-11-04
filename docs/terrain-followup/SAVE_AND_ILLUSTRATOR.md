# Save, Illustrator labels, and the running version

Investigated from source revision `0e9c9ae70956f6f4a7f77149efc02f48b6b3723a`.
The existing archive/build baseline is documented in `IMPLEMENTATION.md` and
`../atlas-redesign/BASELINE.md`. No toolchain files or saved terrain fields were
removed or regenerated for this follow-up.

## Reproduction and causes

The server at port 8124 was still the earlier Go executable, last written on
2026-10-23 at 23:50:42 local time, serving the earlier frontend bundle
`index-LxyVYYpz.js`. Its editor still offered `physical-atlas` as the default
preview. Updating source files does not update an already running Go process.
The launcher previously rebuilt the shared frontend directory before attempting
to bind its port, allowing a second launch to mix a new frontend and old server.
It now checks the address first and exits without rebuilding when occupied.

The new test World, `inn` (`4c2385a9d9435ea6c9c7d8b9a079ad8e`), had no maps,
terrains or entity records. Its standalone terrain project,
`88000b5ac4dd4197848d4616f9f7f1f7`, was present, saved, and contained the named
country Zenon. Thus its source was persisted, but was not attached to its World.
Save now attaches a standalone terrain to a World/Age even when the URL lacks a
World parameter. Attachment requests for the same session are coalesced. All
existing authored entities acquire their corresponding World records.

Save commits the current view and its persisted UI copy under the same session
lock as the terrain/feature transaction. Previously these were separate
requests, and an older UI copy could override the saved camera on reopening.
Sequence numbers reject delayed staging requests; the frontend also ignores
stale staging acknowledgements and retains the dirty indicator for navigation
that happens after Save begins. A failed commit keeps changes in the session.

Illustrator had two independent label failures:

1. It automatically hid a country name whenever the whole text box could not
   fit within the polygon. It now tries readable interior sizes, then places an
   external label with a leader. Explicit manual hiding and placement remain
   respected. Major cities are included at world scale, and towns at continent
   scale. Focusing a selected country opens a country illustration regardless
   of the builder that launched Illustrator.
2. At close zoom, the SVG multiply layer could visually cover later labels,
   symbols and boundaries while those elements remained present, visible and
   correctly positioned in the DOM. Removing the blend mode reproduced their
   appearance without changing geometry. The replacement `relief-shadow` raster
   represents grayscale multiply as black alpha (`alpha = 1 - shade`). Ordinary
   alpha compositing preserves the illumination and SVG layer order without
   the problematic blend surface. Its cache revision is `raster-5`.

Illustrator's coast ink now always samples the same authoritative water mask as
the land raster. Both are published in the same viewport transaction and use
the same bounds, revision and pixel addresses. Saved shoreline geometry and
authoritative elevation/hydrology are unchanged. Export dimensions, layer
visibility and Illustrator settings remain independent of preview quality.

## Data preservation

Consistent SQLite backups were written outside the repository before repair:
`inn-before-save-repair.sqlite` and `inn-before-activation.sqlite` in the parent
worldbuilding directory. Development and browser reproduction used a database
copy on port 8152. Test entities and artwork from that copy were not copied into
the user's database.

The existing `inn` terrain was attached to its existing World and First Age.
This changed zero maps/records into one terrain map and one Zenon record. Its
entity-coordinate SHA-256 remained
`e6f595c426a119e9ee5d6affe67b39aade937ce7158dea3e98ea7f963f0c3d54`.
Three authoritative terrain samples were identical before and after attachment.
The existing `jkj` session was also checked for pending changes before activation.
Neither session had unsaved changes at that check.

## Verification

- `npm test`: 120 passing JavaScript tests, including small-country labels,
  edge/crowded labels, manual label overrides and duplicate-launch protection.
- `go build ./...`: passed.
- Backend suites for HTTP API, storage, history and authored world state passed.
  The new Save test reopens persisted parts and verifies both camera copies and
  rejection of delayed staging requests.
- Terrain tile tests passed. Synthetic shores stay at the same water boundary
  at zoom levels 0, 4, 12 and 18. Alpha relief is byte-equivalent to the existing
  grayscale multiplier, including its opacity behavior.
- Frontend production build and UI boundary checks passed.
- The browser Save/Illustrator regression passed on an expendable copy of `inn`.
  It checks World records, exact coordinates after closing/reopening the session,
  saved artwork and cameras, and actual painted label pixels at close zoom.
  A DOM-only visibility check would miss the SVG blending defect.
- Additional browser checks covered world/regional/local camera widths 2, 0.08,
  0.015 and 0.001, identical raster/ink bounds, and unchanged source samples.

Run the mutating browser regression only against an expendable database:

```powershell
$env:KRIEMHILD_UI_URL = 'http://127.0.0.1:8152'
$env:KRIEMHILD_SAVE_TEST_PROJECT = '<terrain-project-id>'
$env:KRIEMHILD_SAVE_TEST_WORLD = '<world-id>'
npm.cmd exec playwright -- test terrain-save.browser.spec.mjs
```

The unchanged retired `internal/legacyterrain` suite has the previously recorded
ten-minute baseline timeout. A passing active-package verification must not be
reported as a passing full legacy suite. Clean archive verification records its
commit, hashes, extraction and build/test results alongside the generated ZIP.

The saved source is finite; zooming does not create additional relief. Remaining
source-resolution limitations continue to be shown in the editor footer.
