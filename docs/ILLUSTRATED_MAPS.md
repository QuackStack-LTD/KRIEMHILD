# Illustrated maps

In Map Builder or Settlement Builder, select **Generate Illustrated Map**. The action flushes pending edits and explicitly saves the complete source map. It does not use the editor canvas, camera, overlays, selection state, layer visibility, or viewport object limit.

The preview uses one medieval style on white paper, with a border, compass, title and a centre-latitude scale bar when geographic metadata exists. Change the artwork title, then choose:

- **Save to World** to persist a named, immutable SVG snapshot with map/Age provenance.
- **Export SVG** for an editable, scalable vector document.
- **Export PNG** for a raster image with a long edge of 3,200, 4,800, 6,400 or 8,192 pixels.

The Saved artwork selector reopens previous snapshots. Regenerate from map produces a new preview; changing the geography never alters a previously saved illustration. If the source changes while a preview is open, regenerate it before saving. Regular Save continues to save editable world data, independently of artwork.

## Renderer and geographic scale

`src/builder/medieval-map-export.js` is shared by both builders. The full Map Builder extent or the child's persisted bounds are fitted, without stretching, onto a 1,600 by 1,100 SVG page. Geometry is clipped to that extent. Coastlines and lake shores come from the terrain's water footprint. Sub-page-unit contour rounding removes raster stair steps without substituting an invented coastline. Mountain and forest symbols follow elevation and vegetation, including authored geography. Settlements retain their polygon outlines; building density and castle symbols distinguish villages, towns and capitals. Political geometry uses separate dashed ink. Labels are escaped, prioritized and placed around symbols and rivers.

River paths come from the persisted detail model and authored river profiles, including actual distributaries and braided channels. Variable `Widths` are retained. SI widths, discharge, local geographic scale and latitude-dependent projection metrics supply physical and cartographic widths. Small streams are suppressed at distant scales; nonlinear optical exaggeration keeps visible rivers distinct without a common minimum stroke. River class and system length determine visibility/label priority, not physical width. Shared junctions use matching endpoint widths and round joins. Modeled/estimated channel depth selects muted ink and bank outlines; valley incision and morphology independently gate canyon hatching.

The export API samples saved terrain detail at page-appropriate resolution, creating missing detail through the existing terrain engine. It composes authored changes independently of editor layer toggles. It never creates a second hydrology model. The white-paper SVG has no scripts, external assets, embedded screenshots or dependency on a font download; serif fallbacks keep text portable.

Continental illustrations aggregate cities and villages into their enclosing named
country or larger state. Independent settlements remain visible. Saved political and
spatial relationships, plus actual polygon containment, determine this aggregation;
it does not change ownership in the world model. Country and other detailed place maps
show cities and villages, including footprints saved in child maps. State names use
current Age records, interior anchors and priority placement. Country names and detailed
settlement names remain visible even where collision avoidance finds no empty position.
Regenerate an illustration to apply renderer `medieval-3`; saved artwork is unchanged.

## Persistence

Artwork is an optional collection in authored world state. Each record has its own ID, name, origin World/Map/Age, optional parent map, source revision, bounds, SVG, page dimensions, style ID, renderer version and creation time. Origin fields remain snapshot provenance when inherited by a later Age; the containing map snapshot supplies its historical ownership. A child map does not claim its parent's artwork.

Database checkpoints store independently compressed `builder/artwork/{id}` parts. Historical maps reuse the existing immutable, content-addressed snapshot storage, so unchanged artwork is shared safely across Ages. Terrain ZIPs include the optional `artworks` collection in `world/project.json`; historical World ZIPs include the artwork parts among referenced snapshot blobs. Both paths validate and restore the SVG byte-for-byte, without regeneration. Existing projects without artwork remain valid. Render version `medieval-1` and style `medieval-white` identify this initial renderer. Saved SVGs are validated as inert, self-contained vector documents before database writes and on import.

## Validation

`tests/medieval-map.test.mjs` exercises width hierarchy, length versus physical size, local visibility, depth versus incision, deterministic SVGs, escaping, water boundaries, child extents, complete entity inventories and branching geometry. `internal/httpapi/illustration_test.go` verifies uncapped export input, stale-preview rejection, inert SVG validation, database/terrain ZIP/historical World ZIP restoration, and child-map artwork isolation.

## Geological channel width

New terrain generation stores a `channelWidthFactor` field and each reach's `bankWidthFactor`. These combine seeded regional bank fabric, resistant versus erodible lithology, valley confinement, and a coherent seed-derived factor for each river system. Discharge and slope remain the primary hydraulic inputs. Factors affect physical `WidthMetres`/`Width`, erosion's lateral extent, detailed riverbed cross-sections, and all river renderers. Widths interpolate between shared drainage nodes instead of rerolling at each zoom or tile.

The terrain renderer uses the physical width profile for optical scaling instead of a common capped stroke. Different channels can therefore appear near 2 and 20 pixels at the same regional zoom; pixel size still varies with geographic scale and zoom. Contrast increases gradually toward regional zoom while overview strokes remain restrained. River-specific physical variation spans roughly 0.25 to 4 times the geological baseline. Existing saved worlds retain their stored physical dimensions. Generate a new terrain for the new geological width variation; saved illustrations remain immutable snapshots.

The Generator View tab and both builder sidebars offer a River display thickness slider (0.25x to 4x, Reset to 1x). It updates generated and authored channel ribbons immediately in 2D and 3D, preserving their relative hierarchy. This is a browser-local viewing preference, stored under `kriemhild.riverThickness`; it does not edit terrain, hydrology, world ZIP data, or saved illustrated artwork.
