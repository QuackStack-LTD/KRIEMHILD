import {climateSettings,climateSettingIds,climateInputs,geographicSettings,riverSettings,erosionSettings} from './climate-ui.js';
import {bindInspectionClick,createInspectionPin} from './inspection-pin.js';
import { createNaturalHover } from './natural-hover.js';
// UI, rendering and config handling. The algorithm lives in wfc.js, the type editor in editor.js.
import './helpers.js';
import './patterns.js';
import './presets.js';
import './editor.js';
import { RemoteSolver } from './api.js';
import { AutosaveQueue } from './autosave.js';
import { createExplorer } from './explorer.js';
import { reliefShades, shadePixel } from './relief.js';
import { installEnvironmentOverlays, fieldColor, overlayLegend, drawPhysicalEntities } from './environment-view.js';

export function mountTerrain({onReady=()=>{},onWorld=()=>{}}={}) {
  installEnvironmentOverlays(document.getElementById('environmentOverlay'));
  const lifecycle = new AbortController();
  const naturalHover=createNaturalHover(document.getElementById("naturalHover"),point=>{explorer.setInspectionPin(point);view?.setInspectionPin(point);updateBasePin();});
  let disposed = false;
  let initializing=true,suspended=false,suspended3DCamera=null;
  let generation = 0;
  let inFlight = false;
  let paintQueue = Promise.resolve();
  function listen(target, event, handler) {
    target.addEventListener(event, (...args) => {
      if(suspended)return;
      try { Promise.resolve(handler(...args)).catch(error => { if (!disposed) showError(error.message); }).finally(()=>{if(['click','change','input','keydown','mouseup'].includes(event))queueAutosave();}); }
      catch (error) { if (!disposed) showError(error.message); }
    }, { signal: lifecycle.signal });
  }
  'use strict';

  const { compileRules, seedFromString, makeRng, neighborSteps } = window.TerrainWFC;
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
  const STORAGE_KEY = 'kriemhild.types.v1';
  const BG = [17, 20, 24]; // cells still in superposition fade toward the page background
  const MAX_VORONOI_PIXELS = 8e6; // above this the Voronoi buffer uses fewer pixels per cell

  // Editable model: { types: [{ id, name, color: '#rrggbb', weight, neighbors: [id], weightNear: { id: w } }] }
  let config = null;
  let rules = null; // compiled from config by wfc.js
  let typeRgb = [];
  let maskColors = new Map();
  let solver = null;
  let previousPhysicalMode = null;
  let paused = false;
  let rafId = 0;
  let regenTimer = 0;
  let solveMs = 0;
  let seedUsed = 0;
  let hoverCell = -1;
  let hoverDetail = null;
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
  const explorer = createExplorer(ui.canvasWrap,ui.canvas,()=>ui.brushOn.checked,queueAutosave);

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
    if (disposed) return;
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
        ...(raw.types[i].environmentType ? { environmentType: String(raw.types[i].environmentType) } : {}),
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
      types: config.types.map(({ id, name, color, weight, height, continent, pattern, environmentType, neighbors, weightNear }) => ({
        id,
        name,
        color,
        weight,
        height,
        ...(continent ? { continent } : {}),
        ...(pattern ? { pattern } : {}),
        ...(environmentType ? { environmentType } : {}),
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
    if (disposed) return false;
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
    if(!initializing)void generate();
  }

  async function fetchDefaults() {
    const res = await fetch('/api/defaults', { cache: 'no-store', signal: lifecycle.signal });
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
        `Couldn't load the default terrain palette (${err.message}). ` +
        'Start KRIEMHILD with npm start and open http://127.0.0.1:8124.'
      );
    }
  }

  function updateAddButton() {
    ui.addType.disabled = !editor.canAdd();
    ui.addType.title = editor.canAdd() ? 'Add a new terrain type' : `At most ${window.TerrainWFC.MAX_TYPES} types`;
  }

  const editor = window.TerrainEditor.createTypeEditor(ui.types, {
    getConfig: () => config,
    isPhysical: () => $('environment').checked,
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

  async function generate() {
    if (disposed || projectBusy || suspended) return;
    await flushAutosave();
    if(solver?.unsaved&&!window.confirm('This world has unsaved changes. Discard them and generate a new world?'))return;
    suspended3DCamera=null;
    updateModeControls();
    persistSettings();
    const ticket = ++generation;
    if (!rules) return;
    stop();
    if (solverError) showError('');
    brush.painting = false;
    const sphere = ui.shape.value === 'sphere';
    const width = readInt(ui.width, 16, 256, 160);
    if (sphere) ui.height.value = Math.max(5, Math.round(width / 2)); // a globe's map is 2:1
    const height = readInt(ui.height, 16, 256, 100);
    const seedText = ui.seed.value.trim();
    seedUsed = seedText ? seedFromString(seedText) : (Math.random() * 2 ** 32) >>> 0;

    const t0 = performance.now();
    ui.statusLine.textContent = "Preparing map…";
    const oldSolver = solver;
    solver = null;
    await oldSolver?.dispose();
    let nextSolver;
    try {
      if ($('environment').checked) {
        const physicalValues=Object.fromEntries(climateSettingIds.map(id=>[id,$(id).value]));
        nextSolver = await RemoteSolver.create({ environment: {geography:geographicSettings(physicalValues,width,height,readInt(ui.cellSize,1,40,6)),riverOptions:riverSettings(physicalValues),erosion:erosionSettings(physicalValues),seasonalClimate:climateSettings(Object.fromEntries(climateSettingIds.map(id=>[id,$(id).value])),width,height,readInt(ui.cellSize,1,40,6)),realism:$('realism').checked,columns:width,rows:height,seed:String(seedUsed),landPercent:Number($('landCoverage').value),temperatureOffset:Number($('temperatureOffset').value),rainfall:Number($('rainfall').value),plateCount:Number($('plateCount').value),continentCount:Math.max(2,Number(ui.contPoints.value)),selection:ui.selection.value,stability:Math.min(10,ui.stability.checked?stabilityStrength():0),latitudeNorth:Number($('latitudeNorth').value),latitudeSouth:Number($('latitudeSouth').value),ruggedness:Number($('ruggedness').value),volcanism:Number($('volcanism').value),islandFrequency:Number($('islandFrequency').value),islandCoastalShare:Number($('islandCoastalShare').value),radius2:RADIUS_STEPS[Number(ui.radius.value)].radius2}, config, wrapX: sphere });
      } else {
    nextSolver = await RemoteSolver.create({ config, options: {
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
    }});
      }
    } catch(error) { if (ticket === generation && !disposed) { showError(error.message); ui.statusLine.textContent = 'Generation failed'; } return; }
    if (ticket !== generation || disposed) { await nextSolver.dispose(); return; }
    solver = nextSolver;
    rules = solver.rules;
    showError('');
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
    queueAutosave();
    onWorld(solver);
    schedule();
    return solver;
  }

  // (Re)creates the offscreen image the map is painted into: one pixel per cell for plain
  // squares, or a finer Voronoi image. Called on new maps and when drawing settings change.
  function setupSurface() {
    if (!solver) return;
    terrainShades=reliefShades(solver.environment,solver.W,solver.H);
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
    if (view) view.setMap(solver.W, solver.H, ui.canvas, { sphere: solver.wrapX,waterFields:solver.environment?.fields,solver });
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

    return { w: vw, h: vh, scale, owner, start, list, ptX, ptY };
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

  async function frame() {
    rafId = 0;
    if (!solver || disposed) return;
    if (inFlight) { schedule(); return; }
    const active = solver;
    inFlight = true;
    try {
      if (!paused && active.status === 'running') {
        const t0 = performance.now();
        await active.step(ui.instant.checked ? 10000 : stepsPerFrame());
        solveMs += performance.now() - t0;
      }
      if (active !== solver || disposed) return;
      await autoCleanup(active);
      if (active !== solver || disposed) return;
      draw();
      if (!paused && solver.status === 'running') schedule();
      if(solver.status==='done')onWorld(solver);
    } catch (error) { if (active === solver) { paused = true; showError(error.message); updatePanel(); } }
    finally { inFlight = false; }
  }

  async function stepOnce() {
    if (!solver || solver.status !== 'running') return;
    const active = solver;
    paused = true;
    stop();
    const t0 = performance.now();
    await active.step();
    if (active !== solver || disposed) return;
    solveMs += performance.now() - t0;
    await autoCleanup(active);
    if (active === solver && !disposed) draw();
  }

  async function autoCleanup(active = solver) {
    if (autoCleaned || active.status !== 'done') return;
    autoCleaned = true;
    const passes = readInt(ui.cleanupPasses, 0, 50, 2);
    if (passes) await active.cleanup(passes);
  }

  async function cleanupOnce() {
    if (!solver || solver.status !== 'done') return;
    const active = solver;
    await active.cleanup();
    if (active === solver && !disposed) draw();
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
    const color=fieldColor(solver.environment,$('environmentOverlay').value,c);
    if(!color)return shadePixel(maskColor(m),$('environmentOverlay').value==='terrain'?(terrainShades?.[c]??1):1);
    // Cache CSS colors to avoid parsing them once for every Voronoi pixel.
    let value=environmentColorCache.get(color);
    if(value===undefined){colorCtx.fillStyle=color;colorCtx.fillRect(0,0,1,1);const p=colorCtx.getImageData(0,0,1,1).data;value=pack(p[0],p[1],p[2]);environmentColorCache.set(color,value);}
    return value;
  }
  const colorCtx=document.createElement('canvas').getContext('2d',{willReadFrequently:true});
  const environmentColorCache=new Map();
  let terrainShades=null;
  function drawEnvironmentEntities(){
    const layer=$('environmentOverlay').value;
    drawPhysicalEntities(ctx,solver,vor,layer);
    $('environmentLegend').textContent=overlayLegend(solver.environment,layer);
  }
  listen($('environmentOverlay'), 'change',()=>{if(solver){solver.markAllDirty();draw();}});
  for(const id of ['landCoverage','temperatureOffset','rainfall','plateCount','latitudeNorth','latitudeSouth','ruggedness','volcanism','islandFrequency','islandCoastalShare']) listen($(id), 'change',()=>generate());
  async function updateGenerationMode(){
    const request=++presetRequest;
    updateModeControls();
    const defaults=await fetchDefaults();
    if(disposed||request!==presetRequest)return;
    const preset=PRESETS.find(p=>p.id===presetId);
    // Rebuild named presets so temporary environmental biome additions disappear
    // when physical mode is disabled. Custom palettes keep the user's edits.
    let next=preset?window.TerrainPresets.build(defaults,preset):config;
    if($('environment').checked)next=window.TerrainPresets.withEnvironmentTypes(next,defaults);
    if(useConfig(next))save();
  }
  listen($('environment'), 'change', () => {
    if (!$('environment').checked) $('realism').checked = false;
    return updateGenerationMode();
  });
  listen($('realism'), 'change', () => {
    if ($('realism').checked) $('environment').checked = true;
    return updateGenerationMode();
  });
  const basePin=createInspectionPin(ui.canvasWrap);
  function updateBasePin(){const p=naturalHover.pinned();if(!p||view||solver?.environment){basePin.hide();return;}
    const r=ui.canvas.getBoundingClientRect(),host=ui.canvasWrap.getBoundingClientRect();
    basePin.show(r.left-host.left+(p.x+.5)/solver.W*r.width,r.top-host.top+(p.y+.5)/solver.H*r.height);
  }
  const pinResize=new ResizeObserver(updateBasePin);pinResize.observe(ui.canvasWrap);
  bindInspectionClick(ui.canvasWrap,()=>!ui.brushOn.checked||!!view,e=>{
    if(!solver||!e.target.matches('canvas'))return null;
    if(view)return view.pick(e);
    const p=explorer.point(e),cell=cellAt(e);if(cell<0)return null;
    return p||{x:cell%solver.W,y:Math.floor(cell/solver.W)};
  },p=>naturalHover.pin(solver,p),lifecycle.signal);
  function draw() {
    if (!solver) return;
    if (vor) {
      const { start, list, w: vw } = vor;
      const S = tileSize;
      solver.consumeDirty((c, m) => {
        // Settled cells with a texture copy their pixels from the type's tile; the tile is
        // anchored to the image, so the pattern runs on seamlessly across neighbouring cells.
        const type=rules.types[31-Math.clz32(m)];
        const physicalDune=solver.environment?.options.realism&&(type?.environmentType||type?.id)==='dunes';
        const tile = !physicalDune && $('environmentOverlay').value === 'terrain' && tiles && m !== 0 && (m & (m - 1)) === 0 ? tiles[31 - Math.clz32(m)] : null;
        if (tile) {
          for (let i = start[c], end = start[c + 1]; i < end; i++) {
            const p = list[i];
            const x = p % vw;
            pixels[p] = shadePixel(tile[(((p - x) / vw) % S) * S + (x % S)],terrainShades?.[c]??1);
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
    drawEnvironmentEntities();
    if (ui.contShow.checked) drawContinentPoints();
    updatePanel();
    updateHover();
    update3d();
    explorer.setWorld(solver,$('environmentOverlay').value,!view);
    updateBasePin();
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
      heightBuf[c] = solver.environment ? h : h + 0.15 * Math.abs(h) * roughness[c];
    }
    // Two 3×3 box blurs turn type steps into slopes.
    for (let pass = 0; pass < (solver.environment ? 0 : 2); pass++) {
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
    const scale = heightScale();
    for (let c = 0; c < N; c++) heightBuf[c] *= scale;
    return heightBuf;
  }

  function update3d() {
    if (!view || !solver) return;
    view.setDetailLayer($('environmentOverlay').value);
    view.textureChanged();
    view.setHeights(computeHeights(),heightScale());
  }
  function heightScale(){return Number(ui.height3d.value)*(solver.wrapX?(solver.W/(2*Math.PI))*.03:Math.max(solver.W,solver.H)/50)}

  async function setView3d(on) {
    if (on && !view) {
      try {
        viewModule = viewModule || (await import('./view3d.js'));
      } catch (err) {
        ui.view3d.checked = false;
        showError(
          `Couldn't load the 3D viewer (${err.message}). ` +
          'Reload the page to retry loading the bundled viewer.'
        );
        return;
      }
      if (disposed || !ui.view3d.checked || view) return; // switched off again while loading
      view = viewModule.createView(ui.canvasWrap,queueAutosave);
      view.setInspectionPin(naturalHover.pinned());
      let lastNaturalPick=0;
      listen(view.renderer.domElement,'pointermove',event=>{if(performance.now()-lastNaturalPick<120)return;lastNaturalPick=performance.now();const p=view?.pick(event);if(p)naturalHover.update(solver,p);});
      ui.canvasWrap.classList.add('is-3d');
      if (solver) {
        view.setMap(solver.W, solver.H, ui.canvas, { sphere: solver.wrapX,waterFields:solver.environment?.fields,solver });
        update3d();
      }
    } else if (!on && view) {
      view.dispose();
      view = null;
      ui.canvasWrap.classList.remove('is-3d');
    }
    explorer.setWorld(solver,$('environmentOverlay').value,!view);
    updateBasePin();
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
    naturalHover.update(solver,solver&&hoverCell>=0?{x:hoverDetail?.x??hoverCell%solver.W,y:hoverDetail?.y??Math.floor(hoverCell/solver.W)}:null);
    if (view) {
      el.textContent = '3D view: drag to rotate, right-drag to pan, scroll to zoom.';
      return;
    }
    if (!solver || hoverCell < 0 || hoverCell >= solver.N) {
      el.textContent = ui.brushOn.checked
        ? 'Brush: drag on the map to paint, right-click a cell to pick up its type.'
        : HOVER_HINT;
      return;
    }
    const x = hoverCell % solver.W;
    const y = (hoverCell / solver.W) | 0;
    const zone = solver.zoneAt(hoverCell);
    el.textContent = `${hoverDetail?`(${hoverDetail.x.toFixed(3)}, ${hoverDetail.y.toFixed(3)})`:`(${x}, ${y})`}${zone >= 0 ? ` ${rules.climate.zones[zone].name} zone` : ''}  `;
    const env=solver.environment;
    if(env){
      const f=env.fields,c=hoverCell;
      const climate=env.climateZones?.[f.climate?.[c]];
      el.append(`${climate?climate+' · ':''}${Math.round(hoverDetail?.elevation??f.elevation[c])} m · ${f.temperature[c].toFixed(1)} °C · ${Math.round(f.precipitation[c])} mm/yr · `);
      if(f.waterDepth?.[c]>0)el.append(`Depth ${f.waterDepth[c].toFixed(1)} m · surface ${f.waterLevel[c].toFixed(1)} m · `);
      else if(f.elevation[c]<0)el.append('Dry depression below sea level · ');
      if(f.reefType?.[c]>0)el.append(`${['','Fringing reef','Barrier reef','Patch reef','Atoll'][f.reefType[c]]} · `);
      const layer=$('environmentOverlay').value;
      if(f[layer])el.append(`${layer}: ${f[layer][c].toFixed(2)} · `);
    }
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
    const point=explorer.point(e);
    if(point&&solver){if(point.x<0||point.y<0||point.x>solver.W-1||point.y>solver.H-1)return -1;return Math.round(point.y)*solver.W+Math.round(point.x);}
    const r = ui.canvas.getBoundingClientRect();
    const fx = (e.clientX - r.left) / r.width;
    const fy = (e.clientY - r.top) / r.height;
    if (fx < 0 || fy < 0 || fx >= 1 || fy >= 1) return -1;
    if (!solver) return -1;
    if (vor) return vor.owner[Math.floor(fy * vor.h) * vor.w + Math.floor(fx * vor.w)];
    return Math.floor(fy * solver.H) * solver.W + Math.floor(fx * solver.W);
  }

  listen(ui.canvas, 'mousemove', (e) => {
    if (!solver) return;
    hoverCell = cellAt(e);
    hoverDetail = explorer.sample(e);
    moveBrushCursor(e);
    if (brush.painting && hoverCell >= 0) paintStroke(hoverCell);
    updateHover();
  });
  listen(ui.canvas, 'mouseleave', () => {
    hoverCell = -1;
    hoverDetail = null;
    ui.brushCursor.hidden = true;
    updateHover();
  });

  // ---- brush ------------------------------------------------------------------
  // Paints terrain onto a finished map. Painted cells are pinned; the solver reopens a margin
  // around them and the generator refills it so the surroundings adjust to fit.

  const brush = { painting: false, last: -1, stroke: null, changed: false };

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
    const size = (explorer.pixelScale() ?? r.width / solver.W) * (2 * Number(ui.brushSize.value) + 1);
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
    if (!solver || brush.painting || brushType() < 0) return;
    if (solver.status !== 'done') {
      showError('Let the map finish generating before painting.');
      return;
    }
    if (solverError) showError('');
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
    for (const c of fresh) brush.stroke.add(c);
    const active = solver;
    paintQueue = paintQueue.then(async () => {
      if (active !== solver || disposed) return;
      if (!await active.paint(fresh, t)) { showError(active.message); return; }
      if (active === solver && !disposed) { showError(''); draw(); }
    }).catch(error => { if (active === solver && !disposed) showError(error.message); });
  }

  function endStroke() {
    if (!brush.painting) return;
    brush.painting = false;
    paintQueue = paintQueue.then(() => {
      if (!disposed) updateBrushControls();
    });
  }

  listen(ui.canvas, 'mousedown', (e) => {
    if (!ui.brushOn.checked || view || e.button !== 0) return;
    e.preventDefault();
    const cell = cellAt(e);
    if (cell >= 0) startStroke(cell);
  });
  listen(window, 'mouseup', endStroke);
  // Right-click picks up the type under the cursor.
  listen(ui.canvas, 'contextmenu', (e) => {
    if (!ui.brushOn.checked || !solver) return;
    e.preventDefault();
    const t = solver.typeAt(cellAt(e));
    if (t >= 0) {
      ui.brushType.value = rules.types[t].id;
      updateBrushSwatch();
      updateHover();
    }
  });
  listen(ui.brushOn, 'change', () => {
    updateBrushControls();
    updateHover();
  });
  listen(ui.brushType, 'change', () => {
    updateBrushSwatch();
    updateHover();
  });
  listen(ui.brushSize, 'input', updateBrushControls);

  // ---- controls ------------------------------------------------------------------

  function updateSpeedLabel() {
    ui.speed.disabled = ui.instant.checked;
    ui.speedOut.textContent = ui.instant.checked ? 'instant' : `${fmt(stepsPerFrame())} cells / frame`;
  }

  listen(ui.generate, 'click', generate);
  listen(ui.pause, 'click', togglePause);
  listen(ui.step, 'click', stepOnce);
  listen(ui.cleanup, 'click', cleanupOnce);
  listen(ui.speed, 'input', updateSpeedLabel);
  listen(ui.instant, 'change', () => {
    updateSpeedLabel();
    if (solver && !paused && solver.status === 'running') schedule();
  });
  listen(ui.cellSize, 'change', setupSurface);

  // Sphere worlds need a 2:1 map, so the height follows the width; picking Sphere also opens the
  // 3D view, since that's where the globe shows.
  function updateShapeControls() {
    const sphere = ui.shape.value === 'sphere';
    ui.height.disabled = sphere;
    ui.height.title = sphere ? 'Set automatically to width ÷ 2 for a sphere' : '';
  }
  listen(ui.shape, 'change', () => {
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
  listen(ui.voronoi, 'change', () => {
    updateJitterLabel();
    setupSurface();
  });

  function updateTextureLabel() {
    ui.iconSize.disabled = !ui.textures.checked;
    ui.iconSizeOut.textContent = `${ui.iconSize.value} px`;
  }
  listen(ui.textures, 'change', () => {
    updateTextureLabel();
    setupSurface();
  });

  function updateHeightLabel() {
    ui.height3dOut.textContent = `×${Number(ui.height3d.value).toFixed(1)}`;
  }
  listen(ui.view3d, 'change', () => setView3d(ui.view3d.checked));
  listen(ui.resetCamera, 'click', () => view && view.resetCamera());
  listen(ui.height3d, 'input', () => {
    updateHeightLabel();
    update3d();
  });
  listen(ui.iconSize, 'input', () => {
    updateTextureLabel();
    buildTiles();
    if (solver) {
      solver.markAllDirty();
      draw();
    }
  });
  listen(ui.jitter, 'input', () => {
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
    listen(el, 'change', generate);
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
  listen(ui.stabilityStrength, 'input', updateStabilityLabel);

  function updateContinentLabels() {
    const physical = $('environment').checked;
    const on = ui.continents.checked || physical;
    ui.continents.disabled = physical;
    ui.contPoints.disabled = !on;
    ui.contPoints.min = physical ? 2 : 1;
    ui.contStrength.disabled = physical || !on;
    ui.contShow.disabled = !on;
    ui.contPointsOut.textContent = `${ui.contPoints.value} points`;
    const s = Number(ui.contStrength.value);
    ui.contStrengthOut.textContent = `${s} · up to ×${Math.round(Math.exp(s)).toLocaleString('en-US')}`;
  }
  for (const el of [ui.contPoints, ui.contStrength]) {
    listen(el, 'input', updateContinentLabels);
    listen(el, 'change', generate);
  }
  listen(ui.continents, 'change', () => {
    updateContinentLabels();
    generate();
  });
  function updateClimateLabels() {
    const physical = $('environment').checked;
    const on = ui.climate.checked;
    ui.climate.disabled = physical;
    ui.climLayout.disabled = physical || !on;
    ui.climStrength.disabled = physical || !on;
    ui.climStrengthOut.textContent = `strength ${Number(ui.climStrength.value).toFixed(1)}`;
  }
  listen(ui.climStrength, 'input', updateClimateLabels);
  for (const el of [ui.climStrength, ui.climLayout]) listen(el, 'change', generate);
  listen(ui.climate, 'change', () => {
    updateClimateLabels();
    generate();
  });

  listen(ui.contShow, 'change', () => {
    if (solver) {
      solver.markAllDirty();
      draw();
    }
  });
  listen(ui.stability, 'change', () => {
    updateStabilityLabel();
    generate();
  });

  function updateRadiusLabel() {
    ui.radius.disabled = false;
    const { radius2, count } = RADIUS_STEPS[Number(ui.radius.value)];
    const r = Math.sqrt(radius2);
    ui.radiusOut.textContent = `${Number.isInteger(r) ? r : r.toFixed(2)} · ${count} cells`;
  }
  ui.radius.max = RADIUS_STEPS.length - 1;
  listen(ui.radius, 'input', updateRadiusLabel);

  listen(ui.addType, 'click', () => editor.addType());
  listen(ui.addKind, 'click', () => kindEditor.addKind());

  listen(ui.exportJson, 'click', () => {
    if (!config) return;
    const blob = new Blob([JSON.stringify(exportable(), null, 2) + '\n'], { type: 'application/json' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = 'tiles.json';
    a.click();
    setTimeout(() => URL.revokeObjectURL(a.href), 1000);
  });

  listen(ui.importJson, 'change', async () => {
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

  listen(ui.resetTypes, 'click', () => {
    if (!confirm('Throw away your type edits and load the defaults from tiles.json?')) return;
    try {
      localStorage.removeItem(STORAGE_KEY);
    } catch (_) {
      // nothing saved to remove
    }
    loadDefaults();
  });

  listen(ui.savePng, 'click', () => {
    if (!solver) return;
    ui.canvas.toBlob((blob) => {
      const a = document.createElement('a');
      a.href = URL.createObjectURL(blob);
      a.download = `kriemhild-${seedUsed}.png`;
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

  listen(ui.saveSvg, 'click', async () => {
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
        `Couldn’t build the SVG (${err.message}).`
      );
      return;
    }
    const a = document.createElement('a');
    a.href = URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml' }));
    a.download = `kriemhild-${seedUsed}.svg`;
    a.click();
    setTimeout(() => URL.revokeObjectURL(a.href), 1000);
  });

  listen(document, 'keydown', (e) => {
    const t = e.target;
    // Leave keys alone while typing in a text or number field.
    if (t.matches && t.matches('input[type="text"], input[type="number"], textarea')) return;
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
  const PRESET_KEY = 'kriemhild.preset';
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
  function applyEnvironmentProfile(profile){
    for(const [key,value] of Object.entries(profile||{})) $(key==='landPercent'?'landCoverage':key).value=value;
  }
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
    updateModeControls();
  }

  let presetRequest = 0;
  async function applyPreset(id) {
    const request = ++presetRequest;
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
    if (disposed || request !== presetRequest) return;
    applySettings(p.settings);
    applyEnvironmentProfile(p.environment);
    let next=window.TerrainPresets.build(defaults,p);
    if($('environment').checked)next=window.TerrainPresets.withEnvironmentTypes(next,defaults);
    setPreset(p.id);
    if (useConfig(next)) {
      save();
    }
  }

  listen(ui.preset, 'change', () => {
    if (ui.preset.value) applyPreset(ui.preset.value);
  });

  // ---- tabs ------------------------------------------------------------------

  const TAB_KEY = 'kriemhild.tab';

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
    listen(tab, 'click', () => selectTab(tab));
    listen(tab, 'keydown', (e) => {
      const d = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0;
      if (!d) return;
      e.preventDefault();
      selectTab(ui.tabs[(i + d + ui.tabs.length) % ui.tabs.length], true);
    });
  });

  // ---- start ------------------------------------------------------------------

  const SETTINGS_KEY = 'kriemhild.settings.v1';
  const settingIds = [...climateSettingIds,'environment','realism','landCoverage','plateCount','temperatureOffset','rainfall','latitudeNorth','latitudeSouth','ruggedness','volcanism','islandFrequency','islandCoastalShare',
    'width','height','cellSize','shape','seed','continents','contPoints','contStrength','climate',
    'climLayout','climStrength','selection','radius','stability','stabilityStrength','cleanupPasses',
    'speed','instant','voronoi','jitter','textures','iconSize','contShow','height3d','environmentOverlay'];
  listen($('seasonMode'),'change',()=>{$('seasonCount').disabled=$('seasonMode').value!=='custom';});
  let projectBusy=false;
  const viewClient=crypto.randomUUID();let viewSequence=0;
  function captureProjectUI(compact=false){
    const palette=exportable();
    const compatible=palette.types.length===solver.T&&palette.types.every((t,i)=>t.id===solver.config.types[i].id);
    return {seedUsed,presetId,palette:compatible?palette:solver.config,settings:Object.fromEntries(settingIds.map(id=>{const el=$(id);return [id,el.type==='checkbox'?el.checked:el.value];})),camera2d:explorer.cameraState(),view3d:!!view||!!suspended3DCamera,camera3d:view?.cameraState()||suspended3DCamera,paused,activeTab:document.querySelector('[role="tab"][aria-selected="true"]')?.id,brush:{on:ui.brushOn.checked,type:ui.brushType.value,size:ui.brushSize.value},...(!compact?{offsets:Array.from(offsets),roughness:Array.from(roughness)}:{})};
  }
  const autosaver=new AutosaveQueue(async({active,payload})=>{await active.stageView(payload);},(state,error)=>{
    if(disposed)return;
    $('autosaveStatus').textContent=state==='error'?`Session update failed: ${error.message}`:'Click Save in the top bar to store the world and explored detail.';
  });
  function queueAutosave(){if(disposed||suspended||projectBusy||!solver)return;autosaver.schedule({active:solver,payload:{ui:captureProjectUI(),client:viewClient,sequence:++viewSequence}});}
  async function flushAutosave(){if(!solver)return;await paintQueue;queueAutosave();try{await autosaver.flush();}catch(error){if(error.status!==404)throw error;autosaver.discard();}}
  // Capture controls managed by child editors as well as map gestures.
  for(const event of ['input','change','click','pointerup','pointercancel','wheel','keyup'])document.addEventListener(event,e=>{if(e.target.closest?.('.generator-host'))setTimeout(queueAutosave,0);},{signal:lifecycle.signal,passive:true});
  function projectControls(busy){projectBusy=busy;for(const id of ['saveProject','openProject','openProjectFolder','openStoredProject','generate'])$(id).disabled=busy;}
  async function refreshProjects(){
    try{const data=await RemoteSolver.savedProjects(),select=$('storedProjects'),selected=select.value;select.replaceChildren(new Option(data.projects.length?'Select a saved world':'No saved worlds',''));
      for(const p of data.projects)select.add(new Option(`${p.width}×${p.height} · ${p.seed||p.id.slice(0,8)} · ${p.detailTiles} tiles · ${new Date(p.updatedAt).toLocaleString()}`,p.id));
      if([...select.options].some(o=>o.value===selected))select.value=selected;
    }catch(error){$('projectStatus').textContent='Saved worlds unavailable: '+error.message;}
  }
  listen($('refreshProjects'),'click',refreshProjects);
  listen($('openStoredProject'),'click',()=>{const id=$('storedProjects').value;if(id)void openWorld([],false,id);});
  void refreshProjects();
  listen($('saveProject'),'click',async()=>{
    if(projectBusy)return;
    if(!solver||solver.status!=='done'){showError('Finish generating the world before saving its project.');return;}
    projectControls(true);$('projectStatus').textContent='Saving world fields, explored tiles and edits…';
    try{
      await paintQueue;const active=solver;
      const uiState=captureProjectUI();
      const blob=await active.saveProject(uiState);
      const url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download=`KRIEMHILD-${seedUsed}.world.zip`;a.click();setTimeout(()=>URL.revokeObjectURL(url),60000);
      $('projectStatus').textContent='World ZIP saved, including all generated detail. It can be reopened after the server is restarted.';
      void refreshProjects();
    }catch(error){showError(error.message);$('projectStatus').textContent='Project was not saved.';}finally{projectControls(false);}
  });
  async function openWorld(files,folder,storedID=null){
    if(projectBusy||(!files.length&&!storedID))return;
    await flushAutosave();
    if(solver?.unsaved&&storedID!==solver.worldId&&!window.confirm('This world has unsaved changes. Discard them and open another world?'))return;
    suspended3DCamera=null;
    projectControls(true);clearTimeout(regenTimer);stop();const ticket=++generation;
    $('projectStatus').textContent='Validating and loading stored world data…';let next;
    try{
      next=storedID?await RemoteSolver.openSavedProject(storedID):await RemoteSolver.importProject(files,folder);
      if(disposed||ticket!==generation){await next.dispose();return;}
      const saved=next.projectUI||{},palette=normalize(saved.palette||next.config);
      if(palette.types.length!==next.T||palette.types.some((t,i)=>t.id!==next.config.types[i].id))throw new Error('Saved display palette does not match the world.');
      await paintQueue;const previous=solver;await setView3d(false);
      config=palette;solver=next;rules=next.rules;solver.config=palette;solver.rules=compileRules(palette);rules=solver.rules;
      for(const id of settingIds){const el=$(id),value=saved.settings?.[id];if(value===undefined)continue;
        if(el.type==='checkbox')el.checked=value===true;
        else if(el.tagName==='SELECT'){if([...el.options].some(o=>o.value===String(value)))el.value=String(value);}
        else if(el.type==='number'||el.type==='range'){if(value===''&&climateSettingIds.includes(id)){el.value='';continue;}const n=Number(value);if(Number.isFinite(n)&&n>=Number(el.min)&&n<=Number(el.max))el.value=String(n);}
        else el.value=String(value);
      }
      $('environment').checked=!!solver.environment;$('realism').checked=!!solver.environment?.options.realism;
      for(const id of ['islandFrequency','islandCoastalShare'])if(saved.settings?.[id]===undefined&&solver.environment?.options[id]!==undefined)$(id).value=solver.environment.options[id];
      const climate=solver.environment?.options.seasonalClimate;
      if(climate){for(const [control,key] of Object.entries(climateInputs))if(saved.settings?.[control]===undefined)$(control).value=climate[key]??'';if(saved.settings?.seasonMode===undefined)$('seasonMode').value=climate.mode||'automatic';if(saved.settings?.seasonCount===undefined)$('seasonCount').value=climate.count||4;if(saved.settings?.seasonNames===undefined)$('seasonNames').value=(climate.names||[]).join(', ');if(saved.settings?.climateCoverage===undefined)$('climateCoverage').value=climate.coverage||'';}
      const geography=solver.environment?.options.geography,riverOptions=solver.environment?.options.riverOptions,erosion=solver.environment?.options.erosion;
      for(const [id,key] of Object.entries({worldScale:'world_scale',planetRadius:'planetary_radius',longitudeWest:'longitude_west',longitudeEast:'longitude_east',climateCoverage:'map_coverage'}))if(saved.settings?.[id]===undefined&&geography?.[key]!==undefined)$(id).value=geography[key];
      for(const [id,key] of Object.entries({riverMinArea:'minimum_area_km2',riverMinDischarge:'minimum_discharge_m3s',riverVisibleOrder:'visible_stream_order',riverMajorArea:'major_area_km2',riverRegionalArea:'regional_area_km2',ephemeralDensity:'ephemeral_density'}))if(saved.settings?.[id]===undefined&&riverOptions?.[key]!==undefined)$(id).value=riverOptions[key];
      for(const [id,key] of Object.entries({erosionIterations:'iterations',erosionDuration:'duration_ma',erosionStrength:'strength'}))if(saved.settings?.[id]===undefined&&erosion?.[key]!==undefined)$(id).value=erosion[key];
      ui.width.value=solver.W;ui.height.value=solver.H;ui.shape.value=solver.wrapX?'sphere':'flat';
      seedUsed=Number(saved.seedUsed??solver.environment?.options.seed??0)>>>0;
      const rng=makeRng(seedUsed^0x9e3779b9),restoreArray=(value,n)=>Array.isArray(value)&&value.length===n&&value.every(Number.isFinite)?Float32Array.from(value):Float32Array.from({length:n},()=>rng()*2-1);
      offsets=restoreArray(saved.offsets,solver.N*2);roughness=restoreArray(saved.roughness,solver.N);
      typeRgb=config.types.map(t=>parseColor(t.color,t.id));maskColors=new Map();maskHeights.clear();
      brush.painting=false;paused=solver.status==='running';autoCleaned=solver.status==='done';solveMs=0;
      setPreset(PRESETS.some(p=>p.id===saved.presetId)?saved.presetId:null);updateModeControls();editor.render();kindEditor.render();climateEditor.render();updateAddButton();updateBrushTypes();
      setupSurface();draw();explorer.restoreCamera(saved.camera2d);
      ui.view3d.checked=saved.view3d===true;await setView3d(ui.view3d.checked);view?.restoreCamera(saved.camera3d);
      if(previous?.id!==next.id)await previous?.dispose();save();persistSettings();showError('');
      if(saved.activeTab&&$(saved.activeTab))selectTab($(saved.activeTab));
      if(saved.brush){ui.brushOn.checked=!!saved.brush.on;ui.brushType.value=saved.brush.type;ui.brushSize.value=saved.brush.size;updateBrushControls();}
      $('projectStatus').textContent=`World loaded from project · ${next.storedDetailCount} stored detail tiles. Missing detail will be generated only when explored.`;
      void refreshProjects();onWorld(solver);return solver;
    }catch(error){if(next&&next!==solver)await next.dispose();showError(error.message);$('projectStatus').textContent='Project could not be opened.';schedule();throw error;}finally{projectControls(false);queueAutosave();}
  }
  for(const [id,folder] of [['openProject',false],['openProjectFolder',true]])listen($(id),'change',()=>{const files=[...$(id).files];$(id).value='';void openWorld(files,folder);});
  function persistSettings() {
    try {
      localStorage.setItem(SETTINGS_KEY, JSON.stringify(Object.fromEntries(settingIds.map(id => {
        const el = $(id); return [id, el.type === 'checkbox' ? el.checked : el.value];
      }))));
    } catch (_) { /* Storage may be disabled. */ }
  }
  function restoreSettings() {
    try {
      const saved = JSON.parse(localStorage.getItem(SETTINGS_KEY));
      if (!saved || typeof saved !== 'object') return false;
      for (const id of settingIds) {
        if (!(id in saved)) continue;
        const el = $(id);
        if (el.type === 'checkbox') el.checked = saved[id] === true;
        else if (el.tagName === 'SELECT') {
          if ([...el.options].some(option => option.value === String(saved[id]))) el.value = saved[id];
        } else if (el.type === 'number' || el.type === 'range') {
          if(saved[id]===''&&climateSettingIds.includes(id)){el.value='';continue;}
          const value = Number(saved[id]);
          if (Number.isFinite(value) && value >= Number(el.min) && value <= Number(el.max)) el.value = value;
        } else el.value = String(saved[id]);
      }
      if(!('latitudeNorth' in saved)) {
        const preset=PRESETS.find(p=>p.id===presetId);
        if(preset)applyEnvironmentProfile(preset.environment);
        else if(ui.climLayout.value==='north')$('latitudeSouth').value=0;
      }
      return true;
    } catch (_) { return false; }
  }
  function updateModeControls() {
    $('seasonCount').disabled=$('seasonMode').value!=='custom';
    if ($('realism').checked) $('environment').checked = true;
    const physical = $('environment').checked;
    if (physical !== previousPhysicalMode) {
      previousPhysicalMode = physical;
      if (config) editor.render();
    }
    for (const id of ['landCoverage','plateCount','temperatureOffset','rainfall','environmentOverlay','latitudeNorth','latitudeSouth','ruggedness','volcanism','islandFrequency','islandCoastalShare']) $(id).disabled = !physical;
    $('modeHint').textContent = physical
      ? 'Physical environment layers onto the preset: its land coverage, climate, relief, points and terrain preferences shape the result. Environmental constraints replace legacy adjacency and climate-band weights.'
      : 'Terrain rules: continental kinds, climate weights, neighbor radius and terrain adjacency all apply. Physical settings and overlays are unavailable.';
    if ($('realism').checked) $('modeHint').textContent = 'Real-world rules enabled: climate, geology and water constrain terrain. Rivers, dunes, volcanoes and settlements follow the environmental fields. Inspect them in View.';
    for (const id of ['kindDetails', 'climateDetails']) {
      const details = $(id);
      if (physical) details.open = false;
      details.hidden = physical;
    }
    ui.stabilityStrength.max = physical ? 55 : 300;
    updateStabilityLabel();
    updateContinentLabels();
    updateClimateLabels();
    updateRadiusLabel();
  }

  const restoredSettings = restoreSettings();
  if (!restoredSettings) {
    const preset = PRESETS.find(p => p.id === presetId) || PRESETS[0];
    applySettings(preset.settings);
    applyEnvironmentProfile(preset.environment);
  }
  for (const id of settingIds) listen($(id), 'change', persistSettings);

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
  updateModeControls();

  // Saved edits made against older defaults: bring in what tiles.json has added since (continental
  // kinds, types, a type's kind if it had none) without touching anything the user already has.
  // Returns true if anything changed.
  async function mergeNewDefaults(saved) {
    let defaults;
    try {
      const res = await fetch('/api/defaults', { cache: 'no-store' });
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
    if (disposed) return;
    if (!saved || !useConfig(saved)) await loadDefaults();
    else if (merged) save();
    initializing=false;
    onReady({
      current:()=>solver,
      generate:async()=>{suspended=false;return generate();},
      open:async id=>{suspended=false;return openWorld([],false,id);},
      import:async(files,folder)=>{suspended=false;return openWorld(files,folder);},
      flush:flushAutosave,
      save:async()=>{if(!solver)throw Error('Open a world first.');paused=true;stop();updatePanel();await solver.pending;await flushAutosave();await solver.save();await refreshProjects();},
      suspend:async({discard=false}={})=>{if(discard)autosaver.discard();else await flushAutosave();if(view)suspended3DCamera=view.cameraState();suspended=true;paused=true;stop();clearTimeout(regenTimer);for(const store of solver?.detailStores||[])store.pause();if(view)await setView3d(false);},
      resume:async()=>{suspended=false;if(!solver)return;const state=await solver.refreshWorld();solver.apply(state);for(const store of solver.detailStores||[])store.invalidate();setupSurface();draw();if(suspended3DCamera){const camera=suspended3DCamera;suspended3DCamera=null;ui.view3d.checked=true;await setView3d(true);view?.restoreCamera(camera);}},
      invalidate:bounds=>{for(const store of solver?.detailStores||[])store.invalidate(bounds);},
      export:async()=>{await flushAutosave();return solver?.saveProject(captureProjectUI());}
    });
  })();
  return () => {
    naturalHover.dispose();basePin.dispose();pinResize.disconnect();
    autosaver.dispose();
    disposed = true;
    generation++;
    lifecycle.abort();
    stop();
    clearTimeout(regenTimer);
    clearTimeout(vorRebuild);
    view?.dispose();
    explorer.dispose();
    solver?.dispose();
  };
}
