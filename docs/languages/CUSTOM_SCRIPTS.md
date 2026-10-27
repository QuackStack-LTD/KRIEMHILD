# Custom vector Scripts — first executable slice

Brief 4 now connects languages to this renderer through opt-in language schema 4:
[sequence orthography](SEQUENCE_ORTHOGRAPHY.md). Script schema 2 itself is
unchanged; the rules live in immutable language source, with explicit Script pins.
The independent glyph editor and its old revision behavior remain intact.

Explicit opt-in **Script schema 2**, `vector-script/1`, `sign-layout/1`,
`vector-script-compiler/1`. Create a Script, choose **Custom vector glyphs**,
and open its **Scripts** tab. Existing Script schema 1 remains the independent
Unicode mapping model with exactly its old canonical bodies and language pins.
No authored revision or active Age state is migrated automatically.

## Authoritative representation

`ScriptDefinition.writing` contains stable glyph IDs, normalized vector paths,
metrics, signs, readings, author associations and structured sequences. Artwork
is embedded in the immutable canonical body, not a transient browser blob or a
remote asset. The glyph ID survives renaming, reordering and artwork replacement.
Signs independently reference glyphs. A glyph needs no Unicode character.

Profiles are alphabetic, abjad, abugida, syllabic, logographic and mixed. These
describe author intent; they do not restrict hybrid roles or imply automatic
vowel shaping, ligatures or a particular historical script's conventions.
Signs may have phonetic, syllabic, logographic, morphemic, semantic and classifier
roles simultaneously. Linguistic/semantic associations are explicit authored
annotations, not inferred etymologies, translations or automatically resolved
language/lexeme references.

The optional `grapheme` association is a complete authored Unicode string,
preserved without normalization. `e` plus combining acute is retained as two code
points in one association; this is independent of vector artwork and reading.
The editor does not split/count/cursor-edit user-perceived grapheme clusters,
and this slice does not claim a grapheme segmentation implementation.

A sequence contains `sign`, `group` and explicit top-level `break` nodes.
Each sign use chooses its roles, context atoms and reading ID. Reading contexts
are exact authored key/value constraints. Audible roles require an explicit
compatible reading, even if only one alternative is eligible. Semantic/classifier
only uses may remain silent. Missing or incompatible readings return
`choice_required` or `unresolved_reading` with bounded eligible reading IDs and
no SVG. Missing glyphs/signs never produce substitutes. Groups are ordered
horizontal runs, not two-dimensional hieroglyphic quadrats.

## Layout and deterministic output

Path commands are absolute `M`, `L`, `Q`, `C`, `Z` with exact numeric arities.
Paint is `#RRGGBB` or `none`. Glyph viewBox, baseline, advance and bearing use
glyph-local units. Scale is `unitsPerEm / viewBox.height * glyph.scale`.
Advance is `glyph.advance * scale + spacing`; bearing also scales. The initial
baseline is `unitsPerEm`; explicit breaks add `lineHeight`. LTR increments the
pen; RTL decrements before placing the next sign. Authored order is preserved.
There is no implicit bidi algorithm, automatic wrapping, kerning or shaping.

The result retains the exact canonical request, Script ID/hash, compiler,
engine, adapter, structured sequence, individual sign/glyph/reading identities,
roles, Unicode associations, positions and ordered group/sign/break trace.
SVG uses only generated whitelisted geometry and escaped authored text/IDs.
Its bounds include path control points and stroke; the renderer does not embed
fonts, imported markup, network resources or executable content. SVG and browser
blob URLs are disposable output. They are never the only stored representation.

`render-script` explicitly previews a captured draft; `run` requires an exact
saved compiled revision. Both require engine/adapter/compiler pins. Save,
compile, activation and revision selection reuse the existing World/Age lifecycle,
CAS and retained revision store. Changing artwork, readings, mappings or layout
creates N+1; N remains reproducible. Rendering never activates or saves candidates.

The workbench provides drawing, rectangle/ellipse paths, point/control-handle
dragging and keyboard arrows, numeric M/L/Q/C coordinates, path commands,
metrics, multiple-size previews, sign/reading/context editing, explicit sequence
composition, SVG review and exact saved/draft rendering. Undo/redo covers writing
edits, bounded to 32 snapshots and 8 MiB accounted history; writing direction
uses the ordinary draft field and is outside glyph undo. Libraries and numeric
command lists paginate at 20. Edits/selection supersede pending computation;
rendering does not disable editing or allow an old preview to replace a new one.

## Passive SVG import

Import parses XML on the server into normalized geometry before the user chooses
**Use normalized artwork**. The current glyph identity is retained. Original SVG
is not persisted or mounted in the DOM. No network/file/entity resolution occurs.

Supported: passive groups, paths, rect, circle, ellipse, line, polygon and polyline;
unitless viewBox/size, inherited explicit hex paint/stroke. Relative M/L/H/V/Q/C/Z
normalize to absolute M/L/Q/C/Z. Shapes become paths; normalization warnings are
shown alongside the inert preview and normalized data. Curves for circles/ellipses
use the fixed documented-in-code cubic approximation.

Rejected: scripts, event handlers, foreign namespaces/HTML/foreignObject, images,
use/href/resources, URLs, styles, transforms, filters, DOCTYPE/entities, duplicate
IDs/attributes, malformed XML, unsupported path commands (including arcs and
shorthand), extreme numbers and excessive geometry. Unsupported constructs are
rejected explicitly rather than silently discarded.

Limits are 64 KiB import, depth 16, 256 XML elements, 128 paths and 4,096 commands.
Coordinates are finite within ±8192; viewBox dimensions are 1–4096. Import is
cancellable inside XML/path loops. Rejection is `svg_import_rejected` with a
reason; cancellation is explicit.

## Bounds, API and portability

Canonical source remains within the existing 2 MiB body limit: at most 256 glyphs,
512 signs, 128 sequences, 128 paths/glyph, 4,096 total commands and 4,096 stored
sequence nodes, with group depth four. Readings, role lists, context atoms and
annotations also have explicit per-item limits. Rendering permits at most 512
sequence nodes, 100,000 execution/geometry steps, 2 MiB SVG, 1 MiB trace and
8 MiB result. Every limit returns an explicit failure without partial SVG/signs.
Cancellation is checked inside validation, reading resolution and geometry loops.
These are budgets, not measured peak-heap claims.

Routes reuse `/api/worlds/{world}/languages/{script}`: `validate`, `compile`,
`activate`, `run`, `render-script` and `import-svg`. The capability endpoint now
returns independent `scripts` profiles alongside language `profiles`. Compute
uses the existing two-slot/five-second gate, captured World permissions and final
session/membership/revision authorization. Draft preview/import do not persist.

World ZIP schema **4** is required for any retained Script-2 or language-3 body,
including hidden retained dependencies. Geometry is in the ordinary hashed source
blobs with existing ownership/reference validation; missing or corrupt bodies
are rejected. Old revisions and compatible archive versions are retained. No
new storage service, native library, font dependency or CGO requirement is added.

## Executed evidence and remaining scope

`vector_test.go` includes full reviewed SVG/layout/import goldens, N/N+1
immutability, role/reading distinctions, combining Unicode, RTL/breaks, rejection,
limits and cancellation. Storage tests restart and import retained Script 1/2
revisions. API tests cover owner/viewer access and revocation outside the gate.
`tests/scripts.browser.spec.mjs` exercises the real editor, publication, reload,
old revision replay, unsafe/safe SVG, mixed roles, ambiguity, ZIP import, narrow
layout, pagination, keyboard access and stale previews. Current cross-platform
and SQLite/PostgreSQL results are in [BRIEF_3_AUDIT.md](BRIEF_3_AUDIT.md).

Script 2 by itself has no sequence-level spelling rules. Language schema 4 adds
authored finite phonology-to-grapheme-to-sign conversion and eligible bounded
inverse spelling; this is explicit mapping, not automatic inferred orthography.
Existing Script-1 Unicode
mapping/normalization remains available to languages. Vertical layout, automatic
bidi/shaping, installable fonts, general two-dimensional sign packing, prosody
and recursive grammar execution remain **unsupported**. The next slice should
specify a separately versioned sequence rule model and inverse policy before
adding execution or controls.
