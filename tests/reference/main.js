// UI, rendering and config handling. The algorithm lives in wfc.js, the type editor in editor.js.
(function () {
  'use strict';

  const { compileRules, Solver, seedFromString, makeRng, neighborSteps } = window.TerrainWFC;
  const RADIUS_STEPS = neighborSteps(5); // 4, 8, 12, 20, 24, 28, … neighbours

  const $ = (id) => document.getElementById(id);
  const ui = {
    canvas: $('map'),
    hover: $('hover'),
    generate: $('generate'),
    pause: $('pause'),
    step: $('step'),
    cleanup: $('cleanup'),
    cleanupPasses: $('cleanupPasses'),
    stats: $('stats'),
    width: $('width'),
    height: $('height'),
    cellSize: $('cellSize'),
    shape: $('shape'),
    selection: $('selection'),
    radius: $('radius'),
    radiusOut: $('radiusOut'),
    stability: $('stability'),
    stabilityStrength: $('stabilityStrength'),
    stabilityOut: $('stabilityOut'),
    continents: $('continents'),
    contPoints: $('contPoints'),
    contPointsOut: $('contPointsOut'),
    contStrength: $('contStrength'),
    contStrengthOut: $('contStrengthOut'),
    contShow: $('contShow'),
    seed: $('seed'),
    speed: $('speed'),
    speedOut: $('speedOut'),
    instant: $('instant'),
    voronoi: $('voronoi'),
    textures: $('textures'),
    iconSize: $('iconSize'),
    iconSizeOut: $('iconSizeOut'),
    view3d: $('view3d'),
    resetCamera: $('resetCamera'),
    height3d: $('height3d'),
    height3dOut: $('height3dOut'),
    canvasWrap: document.querySelector('.canvas-wrap'),
    brushOn: $('brushOn'),
    brushType: $('brushType'),
    brushSwatch: $('brushSwatch'),
    brushSize: $('brushSize'),
    brushSizeOut: $('brushSizeOut'),
    brushCursor: $('brushCursor'),
    undo: $('undo'),
    jitter: $('jitter'),
    jitterOut: $('jitterOut'),
    types: $('types'),
    addType: $('addType'),
    kinds: $('kinds'),
    addKind: $('addKind'),
    climate: $('climate'),
    climLayout: $('climLayout'),
    climStrength: $('climStrength'),
    climStrengthOut: $('climStrengthOut'),
    zones: $('zones'),
    exportJson: $('exportJson'),
    importJson: $('importJson'),
    resetTypes: $('resetTypes'),
    error: $('error'),
    savePng: $('savePng'),
    saveSvg: $('saveSvg'),
    preset: $('preset'),
    presetDesc: $('presetDesc'),
    statusLine: $('statusLine'),
    tabs: [...document.querySelectorAll('.tabs [role="tab"]')],
  };
  const ctx = ui.canvas.getContext('2d');
  const HOVER_HINT = 'Hover over a cell to see what it can still become.';
  const STORAGE_KEY = 'terrain-generator.types.v1';
  const BG = [17, 20, 24]; // cells still in superposition fade toward the page background
  const MAX_VORONOI_PIXELS = 8e6; // above this the Voronoi buffer uses fewer pixels per cell

  // Editable model: { types: [{ id, name, color: '#rrggbb', weight, neighbors: [id], weightNear: { id: w } }] }
  let config = null;
  let rules = null; // compiled from config by wfc.js
  let typeRgb = [];
  let maskColors = new Map();
  let solver = null;
  let paused = false;
  let rafId = 0;
  let regenTimer = 0;
  let solveMs = 0;
  let seedUsed = 0;
  let hoverCell = -1;
  let off = null;
  let offCtx = null;
  let image = null;
  let pixels = null;
  let solverError = false;
  let offsets = null; // per cell: random direction in [-1, 1]², scaled by the offset slider
  let vor = null; // Voronoi pixel map, or null when drawing plain squares
  let vorRebuild = 0;
  let autoCleaned = false; // whether this map already got its automatic cleanup passes
  let view = null; // the 3D view while it's switched on
  let viewModule = null;
  const maskHeights = new Map(); // mask -> average 3D height of the types it allows
  let roughness = null; // per cell, -1…1: small bumps so flat areas don't look like plastic
  let heightBuf = null;
  let blurBuf = null;

  // ---- colours --------------------------------------------------------------

  const probe = document.createElement('canvas').getContext('2d');

  // Accepts any CSS colour ("brown", "#8b5a2b", "rgb(139 90 43)") and returns [r, g, b].
  function parseColor(str, typeId) {
    probe.fillStyle = '#010203';
    probe.fillStyle = str;
    const v = probe.fillStyle;
    if (v === '#010203' && !/^#?010203$/i.test(String(str).trim())) {
      throw new Error(`"${typeId}".color: "${str}" isn't a CSS colour.`);
    }
    if (v[0] === '#') return [1, 3, 5].map((i) => parseInt(v.slice(i, i + 2), 16));
    return v.match(/[\d.]+/g).slice(0, 3).map(Number);
  }

  const toHex = (rgb) => '#' + rgb.map((v) => Math.round(v).toString(16).padStart(2, '0')).join('');

  function pack(r, g, b) {
    return (0xff000000 | (b << 16) | (g << 8) | r) >>> 0; // ImageData is RGBA bytes = ABGR little-endian
  }

  function maskColor(mask) {
    let c = maskColors.get(mask);
    if (c !== undefined) return c;
    let r = 0, g = 0, b = 0, n = 0;
    for (let t = 0; t < typeRgb.length; t++) {
      if ((mask >>> t) & 1) {
        r += typeRgb[t][0];
        g += typeRgb[t][1];
        b += typeRgb[t][2];
        n++;
      }
    }
    if (n === 0) {
      c = pack(255, 0, 80); // contradiction; only visible mid-repair
    } else {
      r /= n; g /= n; b /= n;
      if (n > 1) {
        // Average of the possible colours, fainter the more options remain.
        const T = typeRgb.length;
        const f = 0.22 + 0.4 * (1 - (n - 2) / Math.max(1, T - 2));
        r = BG[0] + (r - BG[0]) * f;
        g = BG[1] + (g - BG[1]) * f;
        b = BG[2] + (b - BG[2]) * f;
      }
      c = pack(Math.round(r), Math.round(g), Math.round(b));
    }
    maskColors.set(mask, c);
    return c;
  }

  // ---- config -------------------------------------------------------------------

  function showError(msg, fromSolver = false) {
    ui.error.textContent = msg;
    ui.error.hidden = !msg;
    solverError = fromSolver && !!msg;
  }

  // Validates any loaded JSON and turns it into the editable model: ids everywhere,
  // hex colours, symmetric neighbour lists.
  function normalize(raw) {
    const compiled = compileRules(raw);
    const T = compiled.types.length;
    const K = compiled.kinds.length;
    const { zones, kindMult, typeMult } = compiled.climate;
    return {
      version: Number(raw.version) || 1,
      climates: zones.map((z, i) => ({
        id: z.id,
        name: z.name,
        color: toHex(parseColor(z.color, z.id)),
        kinds: Object.fromEntries(compiled.kinds.map((k, j) => [k.id, kindMult[i * K + j]]).filter(([, v]) => v !== 1)),
        types: Object.fromEntries(compiled.types.map((t, j) => [t.id, typeMult[i * T + j]]).filter(([, v]) => v !== 1)),
      })),
      continents: compiled.kinds.map((k) => ({
        id: k.id,
        name: k.name,
        color: toHex(parseColor(k.color, k.id)),
        odds: k.odds,
      })),
      types: compiled.types.map((t, i) => ({
        id: t.id,
        name: t.name,
        color: toHex(parseColor(t.color, t.id)),
        weight: t.weight,
        height: t.height,
        continent: t.continent,
        pattern: t.pattern,
        neighbors: compiled.types.filter((_, j) => (compiled.allowed[i] >>> j) & 1).map((o) => o.id),
        weightNear: Object.fromEntries(
          compiled.types.map((o, j) => [o.id, compiled.near[i * T + j]]).filter(([, v]) => !Number.isNaN(v))
        ),
      })),
    };
  }

  function exportable() {
    return {
      version: config.version,
      continents: config.continents,
      climates: config.climates,
      types: config.types.map(({ id, name, color, weight, height, continent, pattern, neighbors, weightNear }) => ({
        id,
        name,
        color,
        weight,
        height,
        ...(continent ? { continent } : {}),
        ...(pattern ? { pattern } : {}),
        neighbors,
        ...(Object.keys(weightNear).length ? { weightNear } : {}),
      })),
    };
  }

  function save() {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(exportable()));
    } catch (_) {
      // storage unavailable (private window etc.): edits just won't survive a reload
    }
  }

  function useConfig(raw) {
    try {
      config = normalize(raw);
    } catch (err) {
      showError(err.message);
      return false;
    }
    showError('');
    editor.render();
    kindEditor.render();
    climateEditor.render();
    updateAddButton();
    updateBrushTypes();
    compileAndGenerate();
    return true;
  }

  // Rule changes: regenerate once the user pauses for a moment.
  function scheduleRegen() {
    clearTimeout(regenTimer);
    regenTimer = setTimeout(() => {
      regenTimer = 0;
      compileAndGenerate();
    }, 250);
  }

  function compileAndGenerate() {
    clearTimeout(regenTimer);
    try {
      rules = compileRules(config);
      typeRgb = config.types.map((t) => parseColor(t.color, t.id));
      maskColors = new Map();
      maskHeights.clear();
    } catch (err) {
      showError(err.message);
      return;
    }
    generate();
  }

  async function fetchDefaults() {
    const res = await fetch('tiles.json', { cache: 'no-store' });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
  }

  async function loadDefaults() {
    try {
      if (useConfig(await fetchDefaults())) {
        setPreset('earth');
        save();
      }
    } catch (err) {
      showError(
        `Couldn't load tiles.json (${err.message}).\n` +
        'Browsers block reading files when index.html is opened straight from disk. ' +
        'Run "python -m http.server 8123" in this folder and open http://localhost:8123, ' +
        'or use "Import" to pick tiles.json.'
      );
    }
  }

  function updateAddButton() {
    ui.addType.disabled = !editor.canAdd();
    ui.addType.title = editor.canAdd() ? 'Add a new terrain type' : `At most ${window.TerrainWFC.MAX_TYPES} types`;
  }

  const editor = window.TerrainEditor.createTypeEditor(ui.types, {
    getConfig: () => config,
    onChange(kind) {
      save();
      setPreset(null); // edited by hand: no longer a preset
      if (kind === 'structure') updateAddButton();
      if (kind === 'structure' || kind === 'label') climateEditor.render(); // zones list type names
      updateBrushTypes();
      if (kind === 'label') {
        if (rules && rules.types.length === config.types.length) {
          config.types.forEach((t, i) => (rules.types[i].name = t.name));
        }
        updateHover();
        return;
      }
      if (kind === 'color' && !regenTimer && rules && rules.types.length === config.types.length) {
        // Colours and textures only change the drawing: repaint without a new map.
        typeRgb = config.types.map((t) => parseColor(t.color, t.id));
        config.types.forEach((t, i) => Object.assign(rules.types[i], { pattern: t.pattern, height: t.height }));
        maskColors = new Map();
        maskHeights.clear();
        buildTiles();
        if (solver) {
          solver.markAllDirty();
          draw();
        }
        return;
      }
      scheduleRegen();
    },
  });

  const kindEditor = window.TerrainEditor.createKindEditor(ui.kinds, {
    getConfig: () => config,
    onChange(kind) {
      save();
      setPreset(null); // edited by hand: no longer a preset
      editor.render(); // the types' kind dropdowns list these kinds
      climateEditor.render(); // and so do the zones' multipliers
      if (kind === 'kind-label') {
        // Names and colours only change the markers: update them in place, no new map.
        if (rules && rules.kinds.length === config.continents.length) {
          config.continents.forEach((k, i) => Object.assign(rules.kinds[i], { name: k.name, color: k.color }));
        }
        if (solver) draw();
        return;
      }
      scheduleRegen();
    },
  });

  const climateEditor = window.TerrainEditor.createClimateEditor(ui.zones, {
    getConfig: () => config,
    onChange(kind) {
      save();
      setPreset(null); // edited by hand: no longer a preset
      if (kind === 'label') {
        // Zone names only show up in the hover text: update in place, no new map.
        if (rules && rules.climate.zones.length === config.climates.length) {
          config.climates.forEach((z, i) => Object.assign(rules.climate.zones[i], { name: z.name, color: z.color }));
        }
        updateHover();
        return;
      }
      scheduleRegen();
    },
  });

  // ---- generation loop --------------------------------------------------------------

  function readInt(input, min, max, fallback) {
    let v = parseInt(input.value, 10);
    if (!Number.isFinite(v)) v = fallback;
    v = Math.min(max, Math.max(min, v));
    input.value = v;
    return v;
  }

  function stepsPerFrame() {
    return Math.max(1, Math.round(10 ** (Number(ui.speed.value) / 25))); // 1 … 10 000
  }

  function generate() {
    if (!rules) return;
    stop();
    if (solverError) showError('');
    undoStack.length = 0; // brush strokes belong to the old map
    brush.painting = false;
    const sphere = ui.shape.value === 'sphere';
    const width = readInt(ui.width, 16, 256, 160);
    if (sphere) ui.height.value = Math.max(5, Math.round(width / 2)); // a globe's map is 2:1
    const height = readInt(ui.height, 16, 256, 100);
    const seedText = ui.seed.value.trim();
    seedUsed = seedText ? seedFromString(seedText) : (Math.random() * 2 ** 32) >>> 0;

    const t0 = performance.now();
    try {
      if ($('environment').checked) {
        const result=window.TerrainEnvironment.prepare({columns:width,rows:height,seed:String(seedUsed),landPercent:Number($('landCoverage').value),temperatureOffset:Number($('temperatureOffset').value),rainfall:Number($('rainfall').value),plateCount:Number($('plateCount').value),continentCount:Math.max(2,Number(ui.contPoints.value)),selection:ui.selection.value,stability:Math.min(10,ui.stability.checked?stabilityStrength():0),latitudeSouth:ui.climLayout.value==='north'?0:-90},config);
        solver=result.solver;rules=result.rules;solver.wrapX=sphere;
      } else {
    solver = new Solver(rules, {
      width,
      height,
      radius2: RADIUS_STEPS[Number(ui.radius.value)].radius2,
      wrapX: sphere,
      stability: ui.stability.checked ? stabilityStrength() : 0,
      continents: ui.continents.checked
        ? { count: Number(ui.contPoints.value), strength: Number(ui.contStrength.value) }
        : null,
      climate: ui.climate.checked
        ? { strength: Number(ui.climStrength.value), layout: ui.climLayout.value }
        : null,
      selection: ui.selection.value,
      seed: seedUsed,
    });
      }
    } catch(error) { showError(error.message); return; }
    solveMs = performance.now() - t0;
    paused = false;
    autoCleaned = false;

    // Point offsets come from the seed too, so the same seed gives the same picture.
    const rng = makeRng(seedUsed ^ 0x9e3779b9);
    offsets = new Float32Array(solver.N * 2);
    for (let i = 0; i < offsets.length; i++) offsets[i] = rng() * 2 - 1;
    roughness = new Float32Array(solver.N);
    for (let i = 0; i < roughness.length; i++) roughness[i] = rng() * 2 - 1;

    setupSurface();
    schedule();
  }

  // (Re)creates the offscreen image the map is painted into: one pixel per cell for plain
  // squares, or a finer Voronoi image. Called on new maps and when drawing settings change.
  function setupSurface() {
    if (!solver) return;
    const cs = readInt(ui.cellSize, 1, 40, 6);
    ui.canvas.width = solver.W * cs;
    ui.canvas.height = solver.H * cs;

    let w = solver.W;
    let h = solver.H;
    vor = null;
    // Voronoi shapes and textures both need a full-resolution pixel map; textures on plain squares
    // use the same map with no point offset.
    if (ui.voronoi.checked || ui.textures.checked) {
      const scale = Math.max(1, Math.min(cs, Math.floor(Math.sqrt(MAX_VORONOI_PIXELS / solver.N))));
      vor = buildVoronoi(solver.W, solver.H, scale, ui.voronoi.checked ? Number(ui.jitter.value) : 0, solver.wrapX);
      w = vor.w;
      h = vor.h;
    }
    buildTiles();
    if (view) view.setMap(solver.W, solver.H, ui.canvas, { sphere: solver.wrapX });
    if (!off || off.width !== w || off.height !== h) {
      off = document.createElement('canvas');
      off.width = w;
      off.height = h;
      offCtx = off.getContext('2d');
      image = offCtx.createImageData(w, h);
      pixels = new Uint32Array(image.data.buffer);
    }
    solver.markAllDirty();
    draw();
  }

  // Each cell is a point at its centre, moved by up to `jitter` cells in x and y. Every pixel of
  // the scale×scale-per-cell image belongs to the nearest point. Returns that ownership both ways:
  // owner[pixel] -> cell, and list[start[cell] .. start[cell + 1]) -> the cell's pixels.
  function buildVoronoi(W, H, scale, jitter, wrapX) {
    const N = W * H;
    const vw = W * scale;
    const vh = H * scale;
    const ptX = new Float32Array(N);
    const ptY = new Float32Array(N);
    for (let c = 0; c < N; c++) {
      ptX[c] = (c % W) + 0.5 + jitter * offsets[2 * c];
      ptY[c] = ((c / W) | 0) + 0.5 + jitter * offsets[2 * c + 1];
    }

    // How many cells out to look. The pixel's own point is at most √2·(0.5 + jitter) away, and a
    // point k cells over is at least k − 1 − jitter away, so anything beyond R can't be nearer.
    const R = Math.floor(1 + Math.SQRT1_2 + (1 + Math.SQRT2) * jitter);
    const cand = new Int32Array((2 * R + 1) ** 2);
    const candDx = new Float32Array(cand.length); // ±W for points seen across the sphere's seam
    const owner = new Int32Array(vw * vh);
    for (let cy = 0; cy < H; cy++) {
      for (let cx = 0; cx < W; cx++) {
        let n = 0;
        for (let y = Math.max(0, cy - R); y <= Math.min(H - 1, cy + R); y++) {
          for (let x = cx - R; x <= cx + R; x++) {
            if (x >= 0 && x < W) {
              candDx[n] = 0;
              cand[n++] = y * W + x;
            } else if (wrapX) {
              const wx = x < 0 ? x + W : x - W;
              candDx[n] = x - wx;
              cand[n++] = y * W + wx;
            }
          }
        }
        for (let sy = 0; sy < scale; sy++) {
          const v = cy + (sy + 0.5) / scale;
          const row = (cy * scale + sy) * vw + cx * scale;
          for (let sx = 0; sx < scale; sx++) {
            const u = cx + (sx + 0.5) / scale;
            let best = cand[0];
            let bestD = Infinity;
            for (let k = 0; k < n; k++) {
              const c = cand[k];
              const dx = ptX[c] + candDx[k] - u;
              const dy = ptY[c] - v;
              const d = dx * dx + dy * dy;
              if (d < bestD) {
                bestD = d;
                best = c;
              }
            }
            owner[row + sx] = best;
          }
        }
      }
    }

    const start = new Int32Array(N + 1);
    for (let p = 0; p < owner.length; p++) start[owner[p] + 1]++;
    for (let c = 0; c < N; c++) start[c + 1] += start[c];
    const fill = start.slice(0, N);
    const list = new Int32Array(owner.length);
    for (let p = 0; p < owner.length; p++) list[fill[owner[p]]++] = p;

    return { w: vw, h: vh, scale, owner, start, list };
  }

  // One pre-painted texture tile per terrain type (null = no texture), sized in image pixels.
  let tiles = null;
  let tileSize = 16;

  function buildTiles() {
    tiles = null;
    if (!ui.textures.checked || !vor || !rules) return;
    const cs = readInt(ui.cellSize, 1, 40, 6);
    const S = Math.max(4, Math.round((Number(ui.iconSize.value) * vor.scale) / cs));
    tileSize = S;
    const canvas = document.createElement('canvas');
    canvas.width = S;
    canvas.height = S;
    const tctx = canvas.getContext('2d', { willReadFrequently: true });
    tiles = rules.types.map((t, i) => {
      tctx.clearRect(0, 0, S, S);
      if (!typeRgb[i] || !window.TerrainPatterns.paintTile(tctx, t.pattern, S, typeRgb[i])) return null;
      return new Uint32Array(tctx.getImageData(0, 0, S, S).data.buffer);
    });
  }

  function schedule() {
    if (!rafId) rafId = requestAnimationFrame(frame);
  }

  function stop() {
    if (rafId) cancelAnimationFrame(rafId);
    rafId = 0;
  }

  function frame() {
    rafId = 0;
    if (!solver) return;
    if (!paused && solver.status === 'running') {
      const t0 = performance.now();
      const instant = ui.instant.checked;
      const limit = instant ? Infinity : stepsPerFrame();
      const deadline = t0 + (instant ? 30 : 12); // keep the page responsive on big grids
      for (let i = 1; i <= limit && solver.step() === 'running'; i++) {
        if ((i & 31) === 0 && performance.now() > deadline) break;
      }
      solveMs += performance.now() - t0;
    }
    autoCleanup();
    draw();
    if (!paused && solver.status === 'running') schedule();
  }

  function stepOnce() {
    if (!solver || solver.status !== 'running') return;
    paused = true;
    stop();
    const t0 = performance.now();
    solver.step();
    solveMs += performance.now() - t0;
    autoCleanup();
    draw();
  }

  // Runs the configured number of cleanup passes once, right after the map finishes.
  function autoCleanup() {
    if (autoCleaned || solver.status !== 'done') return;
    autoCleaned = true;
    const passes = readInt(ui.cleanupPasses, 0, 50, 2);
    for (let i = 0; i < passes; i++) {
      if (solver.cleanup() === 0) break; // nothing left to clean
    }
  }

  function cleanupOnce() {
    if (!solver || solver.status !== 'done') return;
    solver.cleanup();
    draw();
  }

  function togglePause() {
    if (!solver || solver.status !== 'running') return;
    paused = !paused;
    if (paused) stop();
    else schedule();
    updatePanel();
  }

  // ---- drawing & panel -----------------------------------------------------------------

  function environmentColor(c,m) {
    const name=$('environmentOverlay').value, f=solver.environment?.fields;
    if(name==='terrain'||!f?.[name])return maskColor(m);
    const v=f[name][c];const scale=name==='temperature'||name==='summer'||name==='winter'?(v+40)/80:name==='elevation'?(v+8000)/16000:name==='bathymetry'?v/8000:name==='precipitation'?v/3000:name==='plate'?v/32:name==='flow'||name==='accumulation'?Math.log1p(Math.max(0,v))/8:v;
    const t=Math.max(0,Math.min(1,scale));return pack(Math.round(40+200*t),Math.round(80+100*(1-Math.abs(t-.5)*2)),Math.round(200-160*t));
  }
  $('environmentOverlay').addEventListener('change',()=>{if(solver){solver.markAllDirty();draw();}});
  for(const id of ['environment','landCoverage','temperatureOffset','rainfall','plateCount']) $(id).addEventListener('change',()=>generate());
  function draw() {
    if (!solver) return;
    if (vor) {
      const { start, list, w: vw } = vor;
      const S = tileSize;
      solver.consumeDirty((c, m) => {
        // Settled cells with a texture copy their pixels from the type's tile; the tile is
        // anchored to the image, so the pattern runs on seamlessly across neighbouring cells.
        const tile = $('environmentOverlay').value === 'terrain' && tiles && m !== 0 && (m & (m - 1)) === 0 ? tiles[31 - Math.clz32(m)] : null;
        if (tile) {
          for (let i = start[c], end = start[c + 1]; i < end; i++) {
            const p = list[i];
            const x = p % vw;
            pixels[p] = tile[(((p - x) / vw) % S) * S + (x % S)];
          }
        } else {
          const col = environmentColor(c,m);
          for (let i = start[c], end = start[c + 1]; i < end; i++) pixels[list[i]] = col;
        }
      });
    } else {
      solver.consumeDirty((c, m) => {
        pixels[c] = environmentColor(c,m);
      });
    }
    offCtx.putImageData(image, 0, 0);
    ctx.imageSmoothingEnabled = false;
    ctx.drawImage(off, 0, 0, ui.canvas.width, ui.canvas.height);
    if (ui.contShow.checked) drawContinentPoints();
    updatePanel();
    updateHover();
    update3d();
  }

  // ---- 3D view ------------------------------------------------------------------

  function maskHeight(m) {
    let v = maskHeights.get(m);
    if (v !== undefined) return v;
    let sum = 0;
    let n = 0;
    for (let t = 0; t < rules.types.length; t++) {
      if ((m >>> t) & 1) {
        sum += rules.types[t].height;
        n++;
      }
    }
    v = n ? sum / n : 0;
    maskHeights.set(m, v);
    return v;
  }

  // Height of every cell in world units: its type's height (or the average of what it can still
  // become), plus a little roughness, smoothed into slopes and scaled with the map size.
  function computeHeights() {
    const { W, H, N, dom } = solver;
    if (!heightBuf || heightBuf.length !== N) {
      heightBuf = new Float32Array(N);
      blurBuf = new Float32Array(N);
    }
    for (let c = 0; c < N; c++) {
      const h = solver.environment ? solver.environment.fields.elevation[c]/2000 : maskHeight(dom[c]);
      heightBuf[c] = h + 0.15 * Math.abs(h) * roughness[c];
    }
    // Two 3×3 box blurs turn type steps into slopes.
    for (let pass = 0; pass < 2; pass++) {
      const src = pass === 0 ? heightBuf : blurBuf;
      const dst = pass === 0 ? blurBuf : heightBuf;
      for (let y = 0; y < H; y++) {
        const y0 = y > 0 ? y - 1 : y;
        const y1 = y < H - 1 ? y + 1 : y;
        for (let x = 0; x < W; x++) {
          const x0 = x > 0 ? x - 1 : x;
          const x1 = x < W - 1 ? x + 1 : x;
          let s = 0;
          let n = 0;
          for (let yy = y0; yy <= y1; yy++) {
            for (let xx = x0; xx <= x1; xx++) {
              s += src[yy * W + xx];
              n++;
            }
          }
          dst[y * W + x] = s / n;
        }
      }
    }
    // On a globe heights are relative to its radius (width / 2π), so mountains stay hills, not spikes.
    const scale = Number(ui.height3d.value) * (solver.wrapX ? (W / (2 * Math.PI)) * 0.03 : Math.max(W, H) / 50);
    for (let c = 0; c < N; c++) heightBuf[c] *= scale;
    return heightBuf;
  }

  function update3d() {
    if (!view || !solver) return;
    view.textureChanged();
    view.setHeights(computeHeights());
  }

  async function setView3d(on) {
    if (on && !view) {
      try {
        viewModule = viewModule || (await import('./view3d.js'));
      } catch (err) {
        ui.view3d.checked = false;
        showError(
          `Couldn't load the 3D viewer (${err.message}). ` +
          'It downloads three.js from cdn.jsdelivr.net the first time, so it needs an internet connection.'
        );
        return;
      }
      if (!ui.view3d.checked || view) return; // switched off again while loading
      view = viewModule.createView(ui.canvasWrap);
      ui.canvasWrap.classList.add('is-3d');
      if (solver) {
        view.setMap(solver.W, solver.H, ui.canvas, { sphere: solver.wrapX });
        update3d();
      }
    } else if (!on && view) {
      view.dispose();
      view = null;
      ui.canvasWrap.classList.remove('is-3d');
    }
    ui.resetCamera.disabled = !view;
    ui.height3d.disabled = !view;
    updateBrushControls(); // the brush only works on the 2D map
    updateHover();
  }

  function drawContinentPoints() {
    const sx = ui.canvas.width / solver.W;
    const sy = ui.canvas.height / solver.H;
    const r = Math.max(7, Math.min(11, sx * 1.6));
    ctx.font = `bold ${Math.round(r * 1.1)}px system-ui, sans-serif`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    for (const p of solver.contPoints) {
      const x = p.x * sx;
      const y = p.y * sy;
      ctx.beginPath();
      ctx.arc(x, y, r, 0, Math.PI * 2);
      ctx.fillStyle = p.kind.color;
      ctx.fill();
      ctx.lineWidth = 4;
      ctx.strokeStyle = 'rgba(0, 0, 0, 0.7)';
      ctx.stroke();
      ctx.lineWidth = 2;
      ctx.strokeStyle = '#fff';
      ctx.stroke();
      ctx.fillStyle = '#fff';
      ctx.fillText(p.kind.name.charAt(0).toUpperCase(), x, y + 1);
    }
  }

  const fmt = (n) => n.toLocaleString('en-US');

  function updatePanel() {
    const s = solver;
    const running = s.status === 'running';
    ui.pause.disabled = !running;
    ui.step.disabled = !running;
    ui.pause.textContent = paused && running ? 'Resume' : 'Pause';
    ui.cleanup.disabled = s.status !== 'done';

    const counts = new Uint32Array(rules.types.length);
    for (let c = 0; c < s.N; c++) {
      const t = s.typeAt(c);
      if (t >= 0) counts[t]++;
    }
    editor.setShares(new Map(rules.types.map((t, i) => [t.id, counts[i] / s.N])));

    const statusText = {
      running: paused ? 'Paused' : 'Generating…',
      done: 'Done',
      failed: 'Failed',
    }[s.status];
    const rows = [
      ['Status', statusText, `status-${s.status}`],
      ['Settled', `${fmt(s.settled)} / ${fmt(s.N)} (${Math.floor((s.settled / s.N) * 100)}%)`],
      ['Random picks', fmt(s.steps)],
      ['Backtracks', s.repairs ? `${fmt(s.backtracks)} (${fmt(s.repairs)} repairs)` : fmt(s.backtracks)],
      ['Cleaned up', `${fmt(s.cleaned)} cells`],
      ['Seed', String(seedUsed)],
      ['Solve time', `${(solveMs / 1000).toFixed(2)} s`],
    ];
    ui.stats.textContent = '';
    for (const [k, v, cls] of rows) {
      const dt = document.createElement('dt');
      dt.textContent = k;
      const dd = document.createElement('dd');
      dd.textContent = v;
      if (cls) dd.className = cls;
      ui.stats.append(dt, dd);
    }

    // One-line summary in the action bar; the full table opens under it.
    const dot = document.createElement('span');
    dot.className = `dot ${paused && running ? 'paused' : s.status}`;
    const pct = Math.floor((s.settled / s.N) * 100);
    ui.statusLine.textContent = '';
    ui.statusLine.append(
      dot,
      `${statusText}${running ? ` ${pct}%` : ''} · ${s.W}×${s.H} · seed ${seedUsed} · ${(solveMs / 1000).toFixed(2)} s`
    );
    if (s.status === 'failed') showError(s.message, true);
  }

  function updateHover() {
    const el = ui.hover;
    if (view) {
      el.textContent = '3D view: drag to rotate, right-drag to pan, scroll to zoom.';
      return;
    }
    if (!solver || hoverCell < 0 || hoverCell >= solver.N) {
      el.textContent = ui.brushOn.checked
        ? 'Brush: drag on the map to paint, right-click a cell to pick up its type, Ctrl+Z to undo.'
        : HOVER_HINT;
      return;
    }
    const x = hoverCell % solver.W;
    const y = (hoverCell / solver.W) | 0;
    const zone = solver.zoneAt(hoverCell);
    el.textContent = `(${x}, ${y})${zone >= 0 ? ` ${rules.climate.zones[zone].name} zone` : ''}  `;
    const t = solver.typeAt(hoverCell);
    if (t >= 0) {
      const b = document.createElement('strong');
      b.textContent = rules.types[t].name;
      el.append(b);
      return;
    }
    const opts = solver.optionsFor(hoverCell);
    if (!opts.length) {
      el.append('contradiction');
      return;
    }
    const total = opts.reduce((a, o) => a + o.weight, 0);
    opts.sort((a, b) => b.weight - a.weight);
    el.append(
      'could be: ' +
      opts
        .map((o) => {
          const p = total > 0 ? (o.weight / total) * 100 : 100 / opts.length;
          return `${rules.types[o.type].name} ${p.toFixed(0)}%`;
        })
        .join(' · ')
    );
  }

  // The cell under the mouse: with Voronoi shapes, the cell whose region it's in.
  function cellAt(e) {
    const r = ui.canvas.getBoundingClientRect();
    const fx = (e.clientX - r.left) / r.width;
    const fy = (e.clientY - r.top) / r.height;
    if (fx < 0 || fy < 0 || fx >= 1 || fy >= 1) return -1;
    if (vor) return vor.owner[Math.floor(fy * vor.h) * vor.w + Math.floor(fx * vor.w)];
    return Math.floor(fy * solver.H) * solver.W + Math.floor(fx * solver.W);
  }

  ui.canvas.addEventListener('mousemove', (e) => {
    if (!solver) return;
    hoverCell = cellAt(e);
    moveBrushCursor(e);
    if (brush.painting && hoverCell >= 0) paintStroke(hoverCell);
    updateHover();
  });
  ui.canvas.addEventListener('mouseleave', () => {
    hoverCell = -1;
    ui.brushCursor.hidden = true;
    updateHover();
  });

  // ---- brush ------------------------------------------------------------------
  // Paints terrain onto a finished map. Painted cells are pinned; the solver reopens a margin
  // around them and the generator refills it so the surroundings adjust to fit.

  const brush = { painting: false, last: -1, stroke: null, changed: false };
  const undoStack = [];
  const MAX_UNDO = 30;

  function brushType() {
    return rules ? rules.types.findIndex((t) => t.id === ui.brushType.value) : -1;
  }

  // Keeps the type picker in step with the type list (names, colours, added / deleted types).
  function updateBrushTypes() {
    if (!config) return;
    const current = ui.brushType.value;
    ui.brushType.textContent = '';
    for (const t of config.types) {
      const o = document.createElement('option');
      o.value = t.id;
      o.textContent = t.name;
      ui.brushType.append(o);
    }
    const keep = config.types.some((t) => t.id === current) ? current : (config.types.find((t) => t.id === 'water') || config.types[0]).id;
    ui.brushType.value = keep;
    updateBrushSwatch();
  }

  function updateBrushSwatch() {
    const t = config && config.types.find((o) => o.id === ui.brushType.value);
    ui.brushSwatch.style.background = t ? t.color : 'transparent';
  }

  function updateBrushControls() {
    const on = ui.brushOn.checked && !view;
    ui.canvasWrap.classList.toggle('brushing', on);
    ui.brushSizeOut.textContent = ui.brushSize.value;
    ui.undo.disabled = undoStack.length === 0 || !!view;
    ui.brushOn.disabled = !!view;
    ui.brushOn.parentElement.title = view ? 'The brush works in the 2D view' : 'Paint terrain onto the map (B)';
    if (!on) ui.brushCursor.hidden = true;
  }

  function moveBrushCursor(e) {
    if (!ui.brushOn.checked || view || !solver) {
      ui.brushCursor.hidden = true;
      return;
    }
    const r = ui.canvas.getBoundingClientRect();
    const size = (r.width / solver.W) * (2 * Number(ui.brushSize.value) + 1);
    Object.assign(ui.brushCursor.style, { left: `${e.clientX}px`, top: `${e.clientY}px`, width: `${size}px`, height: `${size}px` });
    ui.brushCursor.hidden = false;
  }

  // Cells within the brush radius of a centre cell (round brush, wraps on a sphere).
  function brushDisc(center, out) {
    const { W, H } = solver;
    const r = Number(ui.brushSize.value);
    const cx = center % W;
    const cy = (center / W) | 0;
    for (let dy = -r; dy <= r; dy++) {
      const y = cy + dy;
      if (y < 0 || y >= H) continue;
      for (let dx = -r; dx <= r; dx++) {
        if (dx * dx + dy * dy > r * r + r) continue;
        let x = cx + dx;
        if (x < 0 || x >= W) {
          if (!solver.wrapX) continue;
          x = (x + W) % W;
        }
        out.add(y * W + x);
      }
    }
  }

  function startStroke(cell) {
    if (!solver || brushType() < 0) return;
    if (solver.status !== 'done') {
      showError('Let the map finish generating before painting.');
      return;
    }
    if (solverError) showError('');
    undoStack.push(solver.snapshot());
    if (undoStack.length > MAX_UNDO) undoStack.shift();
    brush.painting = true;
    brush.changed = false;
    brush.stroke = new Set();
    brush.last = -1;
    paintStroke(cell);
  }

  // Paints from the last stroke position to `cell`, stamping the brush along the way so fast
  // mouse moves don't leave gaps.
  function paintStroke(cell) {
    const t = brushType();
    if (t < 0) return;
    const { W } = solver;
    const cells = new Set();
    const x1 = cell % W;
    const y1 = (cell / W) | 0;
    if (brush.last < 0) {
      brushDisc(cell, cells);
    } else {
      const x0 = brush.last % W;
      const y0 = (brush.last / W) | 0;
      let dx = x1 - x0;
      if (solver.wrapX && Math.abs(dx) > W / 2) dx -= Math.sign(dx) * W; // the short way round
      const n = Math.max(Math.abs(dx), Math.abs(y1 - y0), 1);
      for (let i = 1; i <= n; i++) {
        const x = (Math.round(x0 + (dx * i) / n) + W) % W;
        const y = Math.round(y0 + ((y1 - y0) * i) / n);
        brushDisc(y * W + x, cells);
      }
    }
    brush.last = cell;
    // Only cells this stroke hasn't painted yet: re-painting would reshuffle their surroundings.
    const fresh = [...cells].filter((c) => !brush.stroke.has(c));
    if (!fresh.length) return;
    if (!solver.paint(fresh, t)) {
      showError(solver.message);
      return;
    }
    for (const c of fresh) brush.stroke.add(c);
    brush.changed = true;
    while (solver.step() === 'running'); // the reopened margin is small: refill it right away
    draw();
  }

  function endStroke() {
    if (!brush.painting) return;
    brush.painting = false;
    if (!brush.changed) undoStack.pop(); // nothing happened: don't keep an empty undo step
    updateBrushControls();
  }

  function undo() {
    if (!solver || !undoStack.length || view) return;
    solver.restore(undoStack.pop());
    draw();
    updateBrushControls();
  }

  ui.canvas.addEventListener('mousedown', (e) => {
    if (!ui.brushOn.checked || view || e.button !== 0) return;
    e.preventDefault();
    const cell = cellAt(e);
    if (cell >= 0) startStroke(cell);
  });
  window.addEventListener('mouseup', endStroke);
  // Right-click picks up the type under the cursor.
  ui.canvas.addEventListener('contextmenu', (e) => {
    if (!ui.brushOn.checked || !solver) return;
    e.preventDefault();
    const t = solver.typeAt(cellAt(e));
    if (t >= 0) {
      ui.brushType.value = rules.types[t].id;
      updateBrushSwatch();
      updateHover();
    }
  });
  ui.brushOn.addEventListener('change', () => {
    updateBrushControls();
    updateHover();
  });
  ui.brushType.addEventListener('change', () => {
    updateBrushSwatch();
    updateHover();
  });
  ui.brushSize.addEventListener('input', updateBrushControls);
  ui.undo.addEventListener('click', undo);

  // ---- controls ------------------------------------------------------------------

  function updateSpeedLabel() {
    ui.speed.disabled = ui.instant.checked;
    ui.speedOut.textContent = ui.instant.checked ? 'instant' : `${fmt(stepsPerFrame())} cells / frame`;
  }

  ui.generate.addEventListener('click', generate);
  ui.pause.addEventListener('click', togglePause);
  ui.step.addEventListener('click', stepOnce);
  ui.cleanup.addEventListener('click', cleanupOnce);
  ui.speed.addEventListener('input', updateSpeedLabel);
  ui.instant.addEventListener('change', () => {
    updateSpeedLabel();
    if (solver && !paused && solver.status === 'running') schedule();
  });
  ui.cellSize.addEventListener('change', setupSurface);

  // Sphere worlds need a 2:1 map, so the height follows the width; picking Sphere also opens the
  // 3D view, since that's where the globe shows.
  function updateShapeControls() {
    const sphere = ui.shape.value === 'sphere';
    ui.height.disabled = sphere;
    ui.height.title = sphere ? 'Set automatically to width ÷ 2 for a sphere' : '';
  }
  ui.shape.addEventListener('change', () => {
    updateShapeControls();
    generate();
    if (ui.shape.value === 'sphere' && !ui.view3d.checked) {
      ui.view3d.checked = true;
      setView3d(true);
    }
  });

  function updateJitterLabel() {
    ui.jitter.disabled = !ui.voronoi.checked;
    ui.jitterOut.textContent = `${Number(ui.jitter.value).toFixed(2)} cells`;
  }
  ui.voronoi.addEventListener('change', () => {
    updateJitterLabel();
    setupSurface();
  });

  function updateTextureLabel() {
    ui.iconSize.disabled = !ui.textures.checked;
    ui.iconSizeOut.textContent = `${ui.iconSize.value} px`;
  }
  ui.textures.addEventListener('change', () => {
    updateTextureLabel();
    setupSurface();
  });

  function updateHeightLabel() {
    ui.height3dOut.textContent = `×${Number(ui.height3d.value).toFixed(1)}`;
  }
  ui.view3d.addEventListener('change', () => setView3d(ui.view3d.checked));
  ui.resetCamera.addEventListener('click', () => view && view.resetCamera());
  ui.height3d.addEventListener('input', () => {
    updateHeightLabel();
    update3d();
  });
  ui.iconSize.addEventListener('input', () => {
    updateTextureLabel();
    buildTiles();
    if (solver) {
      solver.markAllDirty();
      draw();
    }
  });
  ui.jitter.addEventListener('input', () => {
    updateJitterLabel();
    // Rebuilding the pixel map can take tens of ms; do it at most once per frame while dragging.
    if (!vorRebuild) {
      vorRebuild = requestAnimationFrame(() => {
        vorRebuild = 0;
        setupSurface();
      });
    }
  });
  for (const el of [ui.width, ui.height, ui.selection, ui.radius, ui.stabilityStrength, ui.seed]) {
    el.addEventListener('change', generate);
  }

  // The slider runs 0–300 on a square curve, so the low strengths where most of the change
  // happens get most of its length: position 30 → 3, 55 → 10, 300 → 300.
  function stabilityStrength() {
    const p = Number(ui.stabilityStrength.value);
    const s = (p * p) / 300;
    return s < 10 ? Math.round(s * 10) / 10 : Math.round(s);
  }

  function updateStabilityLabel() {
    const s = stabilityStrength();
    ui.stabilityStrength.disabled = !ui.stability.checked;
    const factor = Math.exp(s);
    const shown = factor < 1e6 ? Math.round(factor).toLocaleString('en-US') : `10^${Math.floor(s / Math.LN10)}`;
    ui.stabilityOut.textContent = `${s} · up to ×${shown}`;
    ui.stabilityOut.title = `A type gets up to ${shown}× its weight when every settled neighbour already has it`;
  }
  ui.stabilityStrength.addEventListener('input', updateStabilityLabel);

  function updateContinentLabels() {
    const on = ui.continents.checked;
    ui.contPoints.disabled = !on;
    ui.contStrength.disabled = !on;
    ui.contShow.disabled = !on;
    ui.contPointsOut.textContent = `${ui.contPoints.value} points`;
    const s = Number(ui.contStrength.value);
    ui.contStrengthOut.textContent = `${s} · up to ×${Math.round(Math.exp(s)).toLocaleString('en-US')}`;
  }
  for (const el of [ui.contPoints, ui.contStrength]) {
    el.addEventListener('input', updateContinentLabels);
    el.addEventListener('change', generate);
  }
  ui.continents.addEventListener('change', () => {
    updateContinentLabels();
    generate();
  });
  function updateClimateLabels() {
    const on = ui.climate.checked;
    ui.climLayout.disabled = !on;
    ui.climStrength.disabled = !on;
    ui.climStrengthOut.textContent = `strength ${Number(ui.climStrength.value).toFixed(1)}`;
  }
  ui.climStrength.addEventListener('input', updateClimateLabels);
  for (const el of [ui.climStrength, ui.climLayout]) el.addEventListener('change', generate);
  ui.climate.addEventListener('change', () => {
    updateClimateLabels();
    generate();
  });

  ui.contShow.addEventListener('change', () => {
    if (solver) {
      solver.markAllDirty();
      draw();
    }
  });
  ui.stability.addEventListener('change', () => {
    updateStabilityLabel();
    generate();
  });

  function updateRadiusLabel() {
    const { radius2, count } = RADIUS_STEPS[Number(ui.radius.value)];
    const r = Math.sqrt(radius2);
    ui.radiusOut.textContent = `${Number.isInteger(r) ? r : r.toFixed(2)} · ${count} cells`;
  }
  ui.radius.max = RADIUS_STEPS.length - 1;
  ui.radius.addEventListener('input', updateRadiusLabel);

  ui.addType.addEventListener('click', () => editor.addType());
  ui.addKind.addEventListener('click', () => kindEditor.addKind());

  ui.exportJson.addEventListener('click', () => {
    if (!config) return;
    const blob = new Blob([JSON.stringify(exportable(), null, 2) + '\n'], { type: 'application/json' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = 'tiles.json';
    a.click();
    setTimeout(() => URL.revokeObjectURL(a.href), 1000);
  });

  ui.importJson.addEventListener('change', async () => {
    const file = ui.importJson.files[0];
    ui.importJson.value = '';
    if (!file) return;
    let raw;
    try {
      raw = JSON.parse(await file.text());
    } catch (err) {
      showError(`${file.name} isn't valid JSON: ${err.message}`);
      return;
    }
    if (useConfig(raw)) {
      setPreset(null);
      save();
    }
  });

  ui.resetTypes.addEventListener('click', () => {
    if (!confirm('Throw away your type edits and load the defaults from tiles.json?')) return;
    try {
      localStorage.removeItem(STORAGE_KEY);
    } catch (_) {
      // nothing saved to remove
    }
    loadDefaults();
  });

  ui.savePng.addEventListener('click', () => {
    if (!solver) return;
    ui.canvas.toBlob((blob) => {
      const a = document.createElement('a');
      a.href = URL.createObjectURL(blob);
      a.download = `terrain-${seedUsed}.png`;
      a.click();
      setTimeout(() => URL.revokeObjectURL(a.href), 1000);
    });
  });

  // The finished map as SVG text: the same cell points as the 2D view (Voronoi offsets if that's
  // on, plain squares if not), flat type colours.
  async function buildSvg() {
    const mod = await import('./svgexport.js');
    const { W, H, N } = solver;
    const jitter = ui.voronoi.checked ? Number(ui.jitter.value) : 0;
    const ptX = new Float32Array(N);
    const ptY = new Float32Array(N);
    for (let c = 0; c < N; c++) {
      ptX[c] = (c % W) + 0.5 + jitter * offsets[2 * c];
      ptY[c] = ((c / W) | 0) + 0.5 + jitter * offsets[2 * c + 1];
    }
    return mod.mapToSvg({
      W,
      H,
      cellPx: readInt(ui.cellSize, 1, 40, 6),
      ptX,
      ptY,
      typeAt: (c) => solver.typeAt(c),
      types: rules.types.map((t, i) => ({ id: t.id, name: t.name, color: toHex(typeRgb[i]) })),
      wrapX: solver.wrapX,
    });
  }

  ui.saveSvg.addEventListener('click', async () => {
    if (!solver) return;
    if (solver.status !== 'done') {
      showError('Let the map finish generating before saving it as SVG.');
      return;
    }
    let svg;
    try {
      svg = await buildSvg();
    } catch (err) {
      showError(
        `Couldn't build the SVG (${err.message}). The exporter downloads a small library ` +
        '(Delaunator) from cdn.jsdelivr.net the first time, so it needs an internet connection.'
      );
      return;
    }
    const a = document.createElement('a');
    a.href = URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml' }));
    a.download = `terrain-${seedUsed}.svg`;
    a.click();
    setTimeout(() => URL.revokeObjectURL(a.href), 1000);
  });

  document.addEventListener('keydown', (e) => {
    const t = e.target;
    // Leave keys alone while typing in a text or number field.
    if (t.matches && t.matches('input[type="text"], input[type="number"], textarea')) return;
    if ((e.ctrlKey || e.metaKey) && !e.shiftKey && (e.key === 'z' || e.key === 'Z')) {
      e.preventDefault();
      undo();
      return;
    }
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    if (t.closest && t.closest('select')) return; // letters pick options in a dropdown
    if (e.key === 'b' || e.key === 'B') {
      ui.brushOn.checked = !ui.brushOn.checked && !view;
      updateBrushControls();
      updateHover();
    } else if (e.key === 'g' || e.key === 'G') generate();
    else if (e.key === 's' || e.key === 'S') stepOnce();
    else if (e.key === 'c' || e.key === 'C') cleanupOnce();
    else if (e.key === ' ' && !(t.closest && t.closest('button, summary, label'))) {
      e.preventDefault();
      togglePause();
    }
  });

  // ---- presets ------------------------------------------------------------------
  // A preset rebuilds the terrain types, kinds and zones from tiles.json plus its own changes
  // (presets.js) and sets the generation settings. Editing any of those by hand makes it "Custom".

  const PRESETS = window.TerrainPresets.LIST;
  const PRESET_KEY = 'terrain-generator.preset';
  let presetId = null;
  try {
    presetId = localStorage.getItem(PRESET_KEY);
  } catch (_) {
    // storage unavailable
  }

  function setPreset(id) {
    presetId = id;
    try {
      if (id) localStorage.setItem(PRESET_KEY, id);
      else localStorage.removeItem(PRESET_KEY);
    } catch (_) {
      // storage unavailable
    }
    renderPresetSelect();
  }

  function renderPresetSelect() {
    ui.preset.textContent = '';
    const add = (value, label) => {
      const o = document.createElement('option');
      o.value = value;
      o.textContent = label;
      ui.preset.append(o);
    };
    if (!presetId) add('', 'Custom (your edits)');
    for (const p of PRESETS) add(p.id, p.name);
    ui.preset.value = presetId || '';
    const p = PRESETS.find((o) => o.id === presetId);
    ui.presetDesc.textContent = p
      ? p.description
      : 'Your own terrain types, kinds and zones. Pick a preset to start again from a ready-made world.';
  }

  // Puts a preset's generation settings into the sidebar controls.
  function applySettings(s) {
    const r = RADIUS_STEPS.findIndex((o) => o.radius2 === s.radius2);
    ui.radius.value = r >= 0 ? r : 1;
    ui.stability.checked = s.stability > 0;
    if (s.stability > 0) ui.stabilityStrength.value = Math.round(Math.sqrt(300 * s.stability));
    ui.continents.checked = !!s.continents;
    if (s.continents) {
      ui.contPoints.value = s.continents.points;
      ui.contStrength.value = s.continents.strength;
    }
    ui.climate.checked = !!s.climate;
    if (s.climate) {
      ui.climStrength.value = s.climate.strength;
      ui.climLayout.value = s.climate.layout;
    }
    ui.cleanupPasses.value = s.cleanup;
    updateRadiusLabel();
    updateStabilityLabel();
    updateContinentLabels();
    updateClimateLabels();
  }

  async function applyPreset(id) {
    const p = PRESETS.find((o) => o.id === id);
    if (!p) return;
    if (!presetId && config && !confirm(`Switch to "${p.name}"? It replaces your edited terrain types, continental kinds and climate zones.`)) {
      renderPresetSelect();
      return;
    }
    let defaults;
    try {
      defaults = await fetchDefaults();
    } catch (err) {
      showError(`Couldn't load tiles.json (${err.message}).`);
      renderPresetSelect();
      return;
    }
    applySettings(p.settings);
    if (useConfig(window.TerrainPresets.build(defaults, p))) {
      setPreset(p.id);
      save();
    }
  }

  ui.preset.addEventListener('change', () => {
    if (ui.preset.value) applyPreset(ui.preset.value);
  });

  // ---- tabs ------------------------------------------------------------------

  const TAB_KEY = 'terrain-generator.tab';

  function selectTab(tab, focus) {
    for (const t of ui.tabs) {
      const on = t === tab;
      t.setAttribute('aria-selected', String(on));
      t.tabIndex = on ? 0 : -1;
      $(t.getAttribute('aria-controls')).hidden = !on;
    }
    if (focus) tab.focus();
    try {
      localStorage.setItem(TAB_KEY, tab.id);
    } catch (_) {
      // storage unavailable: the tab just isn't remembered
    }
  }

  ui.tabs.forEach((tab, i) => {
    tab.addEventListener('click', () => selectTab(tab));
    tab.addEventListener('keydown', (e) => {
      const d = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0;
      if (!d) return;
      e.preventDefault();
      selectTab(ui.tabs[(i + d + ui.tabs.length) % ui.tabs.length], true);
    });
  });

  // ---- start ------------------------------------------------------------------

  renderPresetSelect();
  try {
    const remembered = ui.tabs.find((t) => t.id === localStorage.getItem(TAB_KEY));
    if (remembered) selectTab(remembered);
  } catch (_) {
    // storage unavailable
  }

  updateSpeedLabel();
  updateJitterLabel();
  updateTextureLabel();
  updateHeightLabel();
  updateShapeControls();
  updateBrushControls();
  ui.resetCamera.disabled = true;
  ui.height3d.disabled = true;
  updateRadiusLabel();
  updateStabilityLabel();
  updateContinentLabels();
  updateClimateLabels();

  // Saved edits made against older defaults: bring in what tiles.json has added since (continental
  // kinds, types, a type's kind if it had none) without touching anything the user already has.
  // Returns true if anything changed.
  async function mergeNewDefaults(saved) {
    let defaults;
    try {
      const res = await fetch('tiles.json', { cache: 'no-store' });
      defaults = await res.json();
    } catch (_) {
      return false; // defaults unavailable: keep the saved edits as they are
    }
    if ((Number(saved.version) || 1) >= (Number(defaults.version) || 1)) return false;

    const kinds = Array.isArray(saved.continents) ? saved.continents : [];
    for (const k of defaults.continents || []) {
      if (!kinds.some((o) => o.id === k.id)) kinds.push(k);
    }
    saved.continents = kinds;
    for (const d of defaults.types) {
      const t = saved.types.find((o) => o.id === d.id);
      if (!t) {
        saved.types.push(d);
        continue;
      }
      if (!t.continent && d.continent) t.continent = d.continent;
      if (t.pattern === undefined && d.pattern) t.pattern = d.pattern;
      if (t.height === undefined && d.height !== undefined) t.height = d.height;
    }
    if (!Array.isArray(saved.climates) && Array.isArray(defaults.climates)) {
      // Keep only references to kinds and types the user still has.
      const keep = (obj, ids) => Object.fromEntries(Object.entries(obj || {}).filter(([id]) => ids.has(id)));
      const kindIds = new Set(kinds.map((k) => k.id));
      const typeIds = new Set(saved.types.map((t) => t.id));
      saved.climates = defaults.climates.map((z) => ({ ...z, kinds: keep(z.kinds, kindIds), types: keep(z.types, typeIds) }));
    }
    saved.version = defaults.version;
    return true;
  }

  (async () => {
    let saved = null;
    try {
      saved = JSON.parse(localStorage.getItem(STORAGE_KEY));
    } catch (_) {
      // no saved edits
    }
    const merged = saved && Array.isArray(saved.types) && (await mergeNewDefaults(saved));
    if (!saved || !useConfig(saved)) loadDefaults();
    else if (merged) save();
  })();
})();
