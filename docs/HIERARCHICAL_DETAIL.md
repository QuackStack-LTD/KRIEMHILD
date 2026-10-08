# Hierarchical terrain exploration

Physical environment and Real-world geology & climate maps now share a deterministic detail model served by Go. Scrolling explores the generated world; it does not create a replacement generation session. The original preset, world seed, continent geometry, physical fields, water bodies and drainage remain authoritative. Rules-only WFC maps retain their discrete editor.

## Scale responsibilities

| View stage | Detail bands | Responsibility |
|---|---|---|
| World | 0 | Original continents, ocean basins, mountain systems, climate, major lakes and rivers. Original elevation samples are immutable anchors. |
| Continental | 1–2 | Interpolate the regional structures into a continuous surface; resolve shelves, banks, coastlines, regional slopes and major drainage. |
| Regional | 3–4 | Connected lowland hills, basins, foothills, river valleys, coastal margins and submerged ridges. |
| Local | 5–6 | Incised channels and floodplains, wind-aligned dunes, wetland hummocks, geological ridges and finer coves. |
| Maximum detail | 7–8 | Nested gullies, rock relief, dune trains, banks and seabed relief; material shading follows the physical surface. |

Stages are descriptive labels over a continuous scale. Eight dyadic bands provide up to 256 subdivisions per original cell, without allocating a world-sized grid at that resolution. The current finite maximum is explicit; zoom does not invent unlimited new terrain.

## Invariants

- All levels sample one immutable, coherent physical surface on nested grids. Its residual is anchored to the original samples, so every deeper grid reproduces parent node elevations exactly. Geology, moisture, wetlands, dunes, slope and a broad mountain apron govern the forms. Original mountain generation is unchanged.
- Three small, seeded coordinate-flow steps refine banks and coves while preserving cell edges and original samples. It cannot reconnect the parent water footprint. Shore profiles meet the local water surface continuously rather than clamping an above-water bed abruptly downward.
- Water-body identity and surface levels come from the parent. Detail changes the bed below those surfaces. Dry below-sea-level depressions remain dry. A level-independent envelope prevents detail from crossing an established shore.
- Existing drainage junctions and source anchors are protected; the intervening channel bed is incised. Major channels acquire wider valleys and alluvial floodplains, while small streams stay surface lines. Discharge controls width, low gradients permit stronger meandering, and river ribbons stop at the continuous shoreline. Visible channels require sufficient catchment area and accumulated water supply, with stricter thresholds on mountain slopes. Synthetic per-slope tributaries are removed; established channels continue to receiving waters. Climate inherits parent fields, with a lapse-rate correction for finer land elevation.
- Samples and feature IDs use global coordinates and the world seed. Tile generation never consumes a camera-dependent random sequence or mutates the parent environment. Request order, cache eviction and repeat visits cannot change results.
- Adjacent tiles share both border samples and halo-derived gradients. Rendering uses scale-driven blending and retains parents while children load. The 3D covering quadtree removes only parent quadrants with available children and stitches mixed-resolution edges.

## Navigation and rendering

In 2D, scroll over a location to zoom, drag to pan, use `+`/`−`, and use **World view** or `0` to fit the map. The camera computes the geographic point before changing scale and adjusts translation so that point stays under the cursor. The hover reports refined coordinates and elevation.

The flat 3D and globe viewers consume the same Go tiles. Their water triangles clip to the refined terrain and local water levels. Children morph from the actual parent triangles; globe positions also morph from the parent chords to avoid subdivision jumps. Wheel zoom raycasts the terrain/water and moves camera and target about the hit point. Globe requests split at the longitude seam rather than requesting an entire strip at high resolution. Pole vertices and the displayed seam are stitched.

Features fade in when the scale can represent them. Hiding a feature at a smaller scale does not delete it. River paths keep their world coordinates and IDs. Terrain placeholder dots have been removed entirely; terrain detail is elevation geometry and surface material, not point symbols. Rivers use variable-width ribbons in 2D and clipped, graded water surfaces over incised beds in 3D. Environmental overlays remain available; the detail elevation and bathymetry overlays show the refined fields.

## API and lifetime

`GET /api/sessions/{id}/detail/{level}/{x}/{y}` returns a 33 × 33 sample tile, its parent elevations, water information, gradients and anchored features. Each tile covers `32 / 2^level` original grid intervals. Levels outside 0–8 and coordinates outside the world are rejected. The endpoint requires a physical session and respects session expiry.

The client requests visible children and their ancestors, keeps root samples immediately available, aborts obsolete requests and discards responses from previous worlds. Detail has a separate three-request server concurrency limit so exploration does not block new world generation. Retry backoff preserves parent display during temporary failures. The cache evicts offscreen detail around a 192-tile target while retaining required visible ancestors and roots.

## Validation and current model boundaries

Go tests cover exact parent preservation, repeat visits, rebuilding without the cache, matching adjacent tile samples/gradients, continuous shore profiles, protected drainage junctions, water identity/depth and monotonic feature profiles. Node tests cover cursor anchoring, scale transitions, visible tile selection and the real detail API. The shared-browser audit checks 2D zoom/return and raycast-based 3D cursor anchoring.

This is a constrained procedural refinement model, not a new geological or hydrological simulation at every zoom. It inherits the existing lakes, catchments and large landforms; it does not independently discover new regional lakes or islands when the camera moves. The engine does not model explorable caves, individual trees, tidal dynamics, sediment transport through time or waterfall fluid simulation. Forest cover remains an environmental material over physical ground relief. New islands and lakes are not introduced across fixed parent water topology. Horizontal distances still use world-grid units rather than a calibrated planetary radius. Existing PNG/SVG exports remain whole-world exports at the original map resolution.

Explored detail is persisted in portable world ZIP projects, including tiles evicted from the display cache. See [World projects](WORLD_PROJECTS.md).
