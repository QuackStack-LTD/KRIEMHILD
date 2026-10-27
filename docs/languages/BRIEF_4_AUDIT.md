# Brief 4: checkout audit and delivery record

2026-10-27, `rebuild/continuous-terrain`, clean
`4ce394d10a61dbfdc4fdd50ef2350e20cb96a9b8`. Recent commits: `4ce394d`
verification, `02611b3` Script 2, `f4fe6e0` A2, `a563840` audit. Historical
Brief 3 evidence was read, not treated as a fresh execution. No AGENTS.md
was found in the workspace or its parent chain.

| Capability | Status | Evidence at this checkout / next increment |
| --- | --- | --- |
| Schema 1/2/3, old bytes and deterministic A1/A2 | working | Full product/spike tests rerun; `schema.go`, `productive_*`, `inverse_index.go`, old full goldens |
| Immutable state, World/Age, activation/CAS, retention | working | Full history/storage and focused HTTP tests rerun; `storage/languages.go`, `history/languages.go` |
| Script 1 Unicode mapping | working | Old goldens and pinned dependency tests rerun |
| Script 2 vectors, roles/readings and LTR/RTL | working | `vector_*`, full product/storage/API tests rerun; real editor browser baseline scheduled below |
| Passive SVG import | working | Negative/normalization/golden/API tests rerun; no original markup execution |
| Sequence spelling language-to-sign bridge | unsupported | No typed rule source or executor; add explicit schema 4 |
| Extended grapheme segmentation / cluster selection | unsupported | Strings retained only; add pinned tested Go UAX29 library |
| Script 2 Unicode/sign associations | persisted-only | Author annotations are not spelling rules or inferred glyph mappings |
| Sequence inverse | unsupported | Add bounded inverse derived from eligible forward rules; lossy stages explicit |
| Recursive feature metadata | persisted-only | No general recursive executor |
| Vertical/font/prosody/recursive grammar | unsupported | Preserve honest capabilities; out of this slice |
| Current browser/platform/database parity | unverified | Fresh checks launched against unchanged source; results appended after completion |

Fresh baseline commands (Windows amd64 Go 1.27.1, Node 24.21.0):

- `CGO_ENABLED=0 .tools/go/bin/go.exe test -count=1 ./internal/languages/... ./internal/history ./internal/storage`: pass, product 7.154s, spike 1.408s, history 1.257s, storage 4.792s.
- `.tools/go/bin/go.exe test -count=1 ./internal/httpapi -run '^TestLanguage'`: pass, 4.980s.
- `node --test tests/languages.test.mjs tests/vector-script.test.mjs`: 7 pass.
- `npm.cmd run check:ui`: existing Biosphere.jsx/BiosphereEditors.jsx formatting failures; unchanged.
- Go build, production UI build, 13 browser scenarios, 14-case Windows/Linux replay and SQLite/PostgreSQL restart/import checks launched; actual results will be appended.

The baseline replay/runtime images are Brief 3 images with the same implementation
source as this HEAD (the HEAD changes documentation only). The Windows CLI is
rebuilt now. Final checks will build new images from the delivered source.
Retired legacyterrain timeout and old generic smoke-route mismatch remain
historical unresolved failures; no test/source-grid changes are authorized here.
The collaborative browser explicitly reported unavailable after status/open;
repository Playwright tests provide browser evidence on disposable Worlds only.

The reviewed-before-coding semantics are in [SEQUENCE_ORTHOGRAPHY.md](SEQUENCE_ORTHOGRAPHY.md).
New behavior is opt-in language schema 4, never a reinterpretation of A2/schema 3.
The first complete scope is finite literal sequence rewriting, explicit whole
grapheme normalization, pinned Script 2 rendering and bounded eligible inverse
followed optionally by A2. No automatic migration or independent data store.
