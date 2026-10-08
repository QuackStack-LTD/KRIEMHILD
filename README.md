# KRIEMHILD

React frontend and native Go backend reproducing the local **TerrainGenOnSteroids** application. This project is self-contained; the neighboring source directory is not needed to build or run it.

## Run

Install Node.js 22.12+ and Go 1.26+, then run from this folder:

```sh
npm install
npm start
```

Open **http://127.0.0.1:8124**. On Windows, `start.bat` performs the same steps. The launcher also recognizes a portable Go installation at `.tools/go`.

For development, run these in separate terminals:

```sh
npm run build
go run ./cmd/kriemhild
npm run dev
```

Open http://127.0.0.1:5173. Vite forwards `/api` requests to Go on port 8124.

For a production build:

```sh
npm run build
go build -o bin/kriemhild ./cmd/kriemhild
./bin/kriemhild -addr 127.0.0.1:8124 -dist dist
```

On Windows, use `bin/kriemhild.exe`. Ship the binary with `dist/`; Node.js is only needed for building the frontend. Three.js and Delaunator are bundled locally, so 3D viewing and SVG export work without a CDN.

## Preserved behavior

- Original dark layout, map canvas, brush toolbar, and World / Terrain / Generator / View tabs.
- Seeded WFC with random/entropy cell selection, neighborhood radii, weights, neighbor boosts, propagation, backtracking, local repairs, stability, and cleanup.
- Continental kinds, latitude climate bands, editable terrain palettes, presets, browser persistence, JSON import/export.
- Physical environment: continuous terrain, tectonics, temperature, wind, precipitation, rain shadows, priority-flood drainage, geological and biome suitability, and numerical overlays.
- Brush painting with environmental/adjacency constraints, right-click eyedropper, 30-stroke undo, animation, pause/resume, and single stepping.
- Voronoi/square rendering, terrain textures, landscape/globe 3D, PNG and SVG export.
- Shortcuts: **G** generate, **Space** pause/resume, **S** step, **C** cleanup, **B** brush, **Ctrl/Cmd+Z** undo.

The app starts with the original terrain-rule engine. Presets compose with both Physical environment and Real-world geology & climate: selecting a preset keeps the switches enabled, and enabling physics keeps the preset selected. Each preset supplies land coverage, latitude, temperature, rainfall, plate activity and relief appropriate to its theme. Terrain weights, neighbor boosts, radius, stability and point count remain effective within environmental constraints. Turning physics off restores the named preset's original palette. Physical mode uses environmental masks instead of legacy adjacency/climate-band weights. Its Points control distributes land/ocean regions; higher counts generally produce smaller landmasses. High land coverage may still join them into a supercontinent.

Physical elevation builds broad continental shelves, coastal lowlands, highlands and mountain systems before adding valleys, ridges and individual peaks. Mountain belts vary in width and height, with foothills and supporting terrain around their summits. Active margins and volcanic edifices explicitly permit steeper slopes. Reef development is restricted to coherent suitable regions rather than covering every warm coastline; the same seed reproduces these structures. Extreme elevations use a gradual height limit to avoid identical flat summits. The map shades elevation slopes; 3D uses the numerical elevations directly without the smoothing applied to discrete WFC tiles. **Relief %** adjusts mountain prominence.

Physical lowlands, plateaus and valleys also have continuous elevation. Water is a separate layer: ocean connectivity and basin water balance determine coverage, allowing enclosed dry land below sea level. Lakes have local surface levels and varied bed depths; oceans have shelves, slopes, trenches and submerged peaks. Reef habitat produces fringing, barrier, patch and atoll patterns. The 3D water mesh covers only wet cells. See [continuous terrain and water](docs/CONTINUOUS_TERRAIN.md) for fields, generation and validation.

Physical worlds have a 24,576-cell limit; rules-only worlds allow dimensions up to 256 × 256. All four borders retain the source's ocean constraint, including sphere mode. These are procedural approximations, not a scientific climate simulator. A custom physical palette must retain the required biomes; renaming one preserves its `environmentType` mapping.

## Architecture

Enable **World → Map → Real-world geology & climate** for derived climate zones, explicit rivers, sediment-driven dunes, tectonic volcanoes, and water/fertility-dependent farms and villages. View exposes every underlying field, including wind arrows and climate categories. The switch is saved; switching it off keeps the earlier physical model available. See [the guide audit and implementation](docs/GEOSPATIAL_REALISM.md) for the causal rules, calibration tests and model limits.

`src/App.jsx` renders the interface in React. `src/terrain/controller.js` mounts and disposes the original canvas/editor interactions using a React effect. Rendering stays in the browser; generation, stepping, cleanup, painting, and undo execute in Go through `src/terrain/api.js`. Client solver helpers only compile palette/display data and calculate hover probabilities; there is no browser generation fallback.

`internal/terrain/` contains the native Go algorithms and embedded default palette. `internal/httpapi/` owns isolated, serialized map sessions and validates API inputs. `internal/storage/` stores complete world projects using embedded SQLite or PostgreSQL. `cmd/kriemhild/` serves the API and built frontend. The backend uses pure-Go database drivers and needs no JavaScript runtime.

## Docker and project database

Copy `.env.example` to `.env` and set a strong URL-safe PostgreSQL password. The single `docker-compose.yml` starts the app and PostgreSQL:

```sh
docker compose up -d
```

Open http://127.0.0.1:8124. The container serves both React and the Go API. Worlds, generation progress, edits and explored detail are saved automatically; camera and display changes save after a brief batching interval. **View → Save → Saved worlds** reopens earlier worlds after a restart, including unfinished generation. The autosave indicator reports pending writes or failures. ZIP export remains available for portable backups.

Running the binary or image without `DATABASE_URL` still uses embedded SQLite. With PostgreSQL configured, it requires that database to be available. See [container setup and releases](docs/CONTAINERS.md) and [autosave storage design](docs/AUTOSAVE.md).

Use **View → Save → Save world ZIP** for a portable backup of a completed world, explored detail and edits. **Open world ZIP** or **Open world folder** restores it without regenerating geography or depending on old server caches. See [Portable world projects](docs/WORLD_PROJECTS.md) for the manifest and validation. Active sessions expire after 30 minutes of inactivity, but their autosaved worlds remain in the database. The visible undo history resets when reopening. This is a local application; it starts on the loopback interface.

## Validation

```sh
go test ./...
go vet ./...
npm test
npm run build
```

`npm test` builds a separate Go test server. WFC retains exact domain/statistic comparisons against the JavaScript reference; physical generation is checked for determinism, consistent water surfaces and preset composition. Go tests check landmass distribution, mountains, lowland/depth variation, dry/wet basins, drainage and reef habitats. Browser audits exercise modes, presets, overlays, editing and persistence. See [the migration analysis](docs/TERRAINGEN_ANALYSIS.md) for compatibility details.


Physical maps now support hierarchical exploration in 2D and 3D: scroll toward the cursor to reveal deterministic detail, drag to navigate, and use World view to return. Eight refinement bands preserve parent elevations, water bodies and drainage, with World through Maximum detail labels. See [Hierarchical terrain exploration](docs/HIERARCHICAL_DETAIL.md) for the model, controls, API and current limits.
