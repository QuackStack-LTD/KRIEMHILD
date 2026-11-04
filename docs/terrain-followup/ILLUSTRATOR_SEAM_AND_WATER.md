# Illustrator seam, inland water and profile layout

Started from clean `a7475f8c5a4a9f18ab5b1ce8663a6935f214a925` on
`rebuild/continuous-terrain`. This follow-up changes geographic presentation,
navigation and layout. It does not change terrain generation, hydrology,
Languages semantics or the saved-world schema.

## Causes and changes

- Languages now sits immediately after Culture & Religion and before Characters.
- The settlement banner gap was the inherited 32 px page padding. Pages with
  banners now have zero top padding at desktop and narrow widths. Breadcrumbs
  remain inside the existing short banner.
- The Illustrator reused the dry-land even-odd selection mask. Lake interiors
  therefore exposed paper instead of the water raster. Illustration coverage
  now unions consistently wound rings with a nonzero fill. This also prevents
  overlapping finite seam strips from cancelling each other. Dry-land selection
  still distinguishes lakes; source rings and terrain are not edited.
- Selected-land fitting now puts the longitude cut in the largest unoccupied
  interval. Opposite-edge fragments move by whole 360-degree turns, keeping
  geographic distances and relative placement. The complete selection receives
  padding. Countries, labels, coast paths, rivers and relief symbols use the
  same longitude branch.
- The atomic viewport renderer, tile fallback and export place wrapped tiles in
  that fitted interval while requesting/cache-keying canonical source tiles.
  Dirty-region dependencies use canonical coordinates too. Existing resource
  limits and render cancellation remain in force; wrapping is enabled only for
  selected-land illustrations.
- Decorative relief meshes sample both canonical sides separately, preserving
  the server spatial index's inclusion of authored terrain edits. Both meshes
  are accepted together before symbols update. No unwrapped mesh request skips
  edits on the other side of the original seam.
- Illustrator navigation no longer overwrites the builder's saved camera.
  Construction mode restores the pre-illustration view. Illustration bounds
  remain stored with the illustration; exports use those displayed coordinates.
- The latitude-dependent distance caption and its scale line are removed from
  the continuous Illustrator scene and its exports.

## Verification

Windows/amd64, Go 1.27.1, Node 24.21.0, npm 11.19.0. Tests use an isolated server
at `127.0.0.1:8137` with `.cache/illustrator-seam/data`, never the author's DB.

- `npm test`: 141 passed, zero failures or skips (final run, 43.2 s).
- `npm run build`: passed.
- `node --test tests/land-selection.test.mjs tests/terrain-pyramid.test.mjs tests/illustration-labels.test.mjs tests/export-geography.test.mjs tests/continuous-tiles.test.mjs`:
  passed, including all 37 tests in these files in the final full run. Covers complete padded fitting, lake/overlap coverage, source
  immutability, relief mesh bounds, country/river continuity, canonical tile
  invalidation, fallback rendering and embedded export rasters.
- `.tools/go/bin/go.exe test ./internal/terrain/landmass ./internal/terrain/tiles`:
  passed.
- `.tools/go/bin/go.exe test ./internal/httpapi -run 'Test(Illustration|Artwork|Illustrated)' -count=1`:
  passed, including persistence and archive regressions.
- `KRIEMHILD_LANGUAGES_TEST=1 KRIEMHILD_UI_URL=http://127.0.0.1:8137 npx playwright test tests/languages.browser.spec.mjs --grep 'short settlement banner'`:
  passed at 1440 and 390 px, checking header/banner adjacency, contained
  breadcrumbs, navigation order and absence of horizontal overflow.
- `KRIEMHILD_UI_URL=http://127.0.0.1:8137 KRIEMHILD_ILLUSTRATOR_TEST_PROJECT=<disposable-project> npx playwright test tests/illustrator.browser.spec.mjs`:
  verifies the actual selection/create/export workflow on generated terrain.
  Fixture seed `illustrator-seam-regression`, source width 1024, other settings
  from `src/continuous/api.js` defaults. The first seam continent fits in about
  0.679 normalized longitude units instead of 2. The test confirms both relief
  mesh branches, queries a real lake, checks mask coverage, decodes an embedded
  SVG export and verifies painted lake pixels, checks source data invariance,
  and verifies the restored/staged builder camera.
- Changed-file Prettier checks and `node scripts/check-ui.mjs`: passed.
- `npm run check:ui`: retains the existing formatting failures in
  `Biosphere.jsx` and `BiosphereEditors.jsx`. Neither file is changed. The known
  retired-terrain Go timeout and old generic smoke-route mismatch remain
  unrelated baseline failures; neither was suppressed or reclassified.

The final combined browser run passed both tests in 15.2 s. In PowerShell:

```powershell
$env:KRIEMHILD_UI_URL = 'http://127.0.0.1:8137'
$env:KRIEMHILD_ILLUSTRATOR_TEST_PROJECT = '<disposable-project>'
$env:KRIEMHILD_LANGUAGES_TEST = '1'
npx.cmd playwright test tests/illustrator.browser.spec.mjs tests/languages.browser.spec.mjs --grep 'selected seam-crossing|short settlement banner'
```

Activation replaces only the active frontend's index after copying the new
hashed assets. Previous assets and an index backup remain available. The
existing Go process, database and in-memory terrain sessions stay running.

Existing stored PNG/SVG maps are snapshots. To update their appearance, recreate
the selected-land illustration and save it explicitly. Existing edited designs
are not silently refitted, and saved images are not rewritten automatically.
