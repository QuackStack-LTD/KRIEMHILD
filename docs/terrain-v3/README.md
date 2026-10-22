# Terrain generation v3 validation

New generation jobs use `continuous-3`. The complete surface, channel paths and
shorelines are saved before the world becomes ready. Zooming interpolates this
data; it does not add procedural terrain. Older saved maps remain readable, and
Library, Characters and the other worldbuilding pages are preserved.

Earthlike defaults use lower relief and more rainfall. Drainage follows entire
watersheds, with major trunks, regional rivers, streams, ephemeral channels,
tributaries, lowland meanders and suitable coastal distributaries. Lakes,
wetland complexes and springs contribute to the network. Diagnostic overlays
show watersheds, flow accumulation, river networks and wetlands.

Map Builder, Settlement Builder and Medieval Illustrator share geographic
coordinates. Linked borders and settlements follow their source geometry.
Saved shorelines use the same surface interpolation as terrain rendering.
After terrain edits, illustrator shoreline ink follows the edited water mask;
edit and undo/redo responses update that behavior without reloading.

## Recorded results

The [default-world browser audit](browser-validation.json) used seed `Kriemhild`
at 4096 × 2048. Generation and initial rendering took 37.7 seconds on the test
machine. It recorded:

| Metric | Result |
| --- | ---: |
| Major river reaches | 122 |
| Regional river reaches | 569 |
| Streams | 1,480 |
| Longest saved river | 6,200 km |
| Lowland headwaters | 1,661 |
| Lakes | 961 |
| Wetland complexes | 568 |
| Springs | 1,253 |
| Land in network-bearing drainage cells | 24.0% |
| Invalid river terminations | 0 |

Reach counts include segments separated by distributaries; they are not counts
of independent continental river systems. Coverage measures drainage cells,
not the physical area occupied by water.

The [three-seed audit](ensemble-validation.json), at 1024 × 512, recorded
11.3–13.1% steppe, 2.8–6.8% desert, longest rivers of 6,272–9,327 km, and no
invalid river terminations. These are sample results, not a guarantee for every
seed, preset or climate setting.

The [full-resolution archive check](archive-validation.json) exported and
reimported a 151,411,567-byte World ZIP. All five representative samples, all
river geometry and all saved shorelines matched exactly. Terrain fields were
stored in 24 checksummed chunks of at most 8 MiB.

## Checks performed

- 66 JavaScript tests passed, including a real server restart and portable
  archive round trip.
- Active Go terrain packages and the world model passed. Follow-up integration
  tests covered completed surfaces, climate, tile seams, meshes, archives,
  historical isolation and cold restarts.
- Final regression tests cover unsupported generator versions, immediate
  terrain-edit status through undo/redo, and shoreline ink matching an edited
  water boundary without retaining the original coast.
- Production frontend build passed. Browser checks covered zoom stability,
  linked country coordinates, illustration and 3D loading, with no page errors
  or failed API requests in the recorded audit.
- The final [live edit/undo/redo check](edited-shoreline-validation.json) used
  the T3 collaborative preview and confirmed that coastline rendering switches
  immediately between edited tiles and saved shorelines without reloading.

The entire legacy Go solver suite was not completed; an earlier full-suite run
timed out in that unchanged package. The active generation and integration
checks above passed.

## Resolution and model limits

Default terrain resolution is 4096 × 2048, approximately 9.8 km per sample at an
Earth-sized equator. Saved vector channel profiles represent narrower rivers
and oxbows. Further zoom reveals interpolation, not new sub-grid landforms.
Climate and drainage use coarser grids; lakes and geological classifications
are approximations. This is a procedural geographic model, not a full erosion
or hydrodynamic simulation.

Restart the application to load the backend changes and generate a new world
to use v3. Existing worlds retain their saved geography.

Screenshots: [world](world.png), [river region](river-region.png),
[close inspection](close-inspection.png), [illustration](illustration.png),
[3D terrain](terrain-3d.png).
