// Wave-function-collapse terrain solver. Pure logic, no DOM: main.js drives it.
//
// Every cell holds a bitmask of the types it can still become (bit t set = type t possible).
// A cell is "in superposition" while more than one bit is set. Collapsing a cell locks it to one
// type, then propagation strips incompatible types from neighbours, their neighbours, and so on.
//
// If propagation ever empties a cell (a contradiction), we first undo the last choice and forbid
// it. If that still contradicts, the area around the conflict is put back into superposition and
// refilled, widening the area each time it fails again.
(function (global) {
  'use strict';

  const MAX_TYPES = 32; // one bit per type in a Uint32
  // Used when a config has no "continents" list (saved before kinds became configurable).
  const DEFAULT_CONTINENTS = [
    { id: 'water', name: 'Water', color: '#3a7bd5', odds: 0.4 },
    { id: 'land', name: 'Land', color: '#7cb342', odds: 0.4 },
    { id: 'mountain', name: 'Mountain', color: '#8a8580', odds: 0.2 },
  ];

  function compileContinents(list) {
    if (list === undefined) list = DEFAULT_CONTINENTS;
    if (!Array.isArray(list)) throw new Error('"continents" must be a list.');
    const seen = new Set();
    return list.map((k, i) => {
      if (!k || typeof k !== 'object' || k.id === undefined) throw new Error(`continents[${i}] needs an "id".`);
      const id = String(k.id);
      if (seen.has(id)) throw new Error(`Duplicate continent id "${id}".`);
      seen.add(id);
      const odds = k.odds === undefined ? 1 : Number(k.odds);
      if (!(odds >= 0)) throw new Error(`Continent "${id}".odds must be a number >= 0.`);
      return { id, name: k.name !== undefined ? String(k.name) : id, color: k.color !== undefined ? String(k.color) : '#888888', odds };
    });
  }

  // Every [dx, dy] with 0 < dx² + dy² <= radius2.
  function neighborOffsets(radius2) {
    const r2 = Math.max(1, Math.floor(radius2) || 1);
    const r = Math.floor(Math.sqrt(r2));
    const out = [];
    for (let dy = -r; dy <= r; dy++) {
      for (let dx = -r; dx <= r; dx++) {
        if ((dx || dy) && dx * dx + dy * dy <= r2) out.push([dx, dy]);
      }
    }
    return out;
  }

  // The distinct neighbourhood sizes up to a radius: [{ radius2, count }], e.g. 1→4, 2→8, 4→12, 5→20.
  function neighborSteps(maxRadius) {
    const steps = [];
    for (let r2 = 1; r2 <= maxRadius * maxRadius; r2++) {
      const count = neighborOffsets(r2).length;
      if (!steps.length || count > steps[steps.length - 1].count) steps.push({ radius2: r2, count });
    }
    return steps;
  }

  function popcount(m) {
    m = m - ((m >>> 1) & 0x55555555);
    m = (m & 0x33333333) + ((m >>> 2) & 0x33333333);
    return Math.imul((m + (m >>> 4)) & 0x0f0f0f0f, 0x01010101) >>> 24;
  }

  function bit(t) {
    return (1 << t) >>> 0;
  }

  // mulberry32: small, fast, seedable
  function makeRng(seed) {
    let a = seed >>> 0;
    return function () {
      a = (a + 0x6d2b79f5) >>> 0;
      let t = a;
      t = Math.imul(t ^ (t >>> 15), t | 1);
      t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }

  // Numeric strings are used as-is, anything else is hashed (FNV-1a) so "hello" is a valid seed.
  function seedFromString(str) {
    const s = String(str).trim();
    if (/^\d+$/.test(s)) return Number(s) >>> 0;
    let h = 0x811c9dc5;
    for (let i = 0; i < s.length; i++) {
      h ^= s.charCodeAt(i);
      h = Math.imul(h, 0x01000193);
    }
    return h >>> 0;
  }

  // Climate zones, listed from the equator to the poles. Each zone multiplies the spawn odds of
  // continental kinds (kindMult[zone*K + kind]) and the weights of terrain types
  // (typeMult[zone*T + type]); anything not listed stays ×1.
  function compileClimates(list, kinds, T, resolveType) {
    if (list === undefined) list = [];
    if (!Array.isArray(list)) throw new Error('"climates" must be a list.');
    const K = kinds.length;
    const Z = list.length;
    const kindMult = new Float64Array(Z * K).fill(1);
    const typeMult = new Float64Array(Z * T).fill(1);
    const seen = new Set();
    const zones = list.map((z, i) => {
      if (!z || typeof z !== 'object' || z.id === undefined) throw new Error(`climates[${i}] needs an "id".`);
      const id = String(z.id);
      if (seen.has(id)) throw new Error(`Duplicate climate id "${id}".`);
      seen.add(id);
      const mult = (v, where) => {
        const n = Number(v);
        if (!(n >= 0)) throw new Error(`${where} must be a number >= 0.`);
        return n;
      };
      for (const [ref, v] of Object.entries(z.kinds || {})) {
        const k = kinds.findIndex((o) => o.id === ref);
        if (k < 0) throw new Error(`Climate "${id}".kinds: unknown continental kind "${ref}".`);
        kindMult[i * K + k] = mult(v, `Climate "${id}".kinds["${ref}"]`);
      }
      for (const [ref, v] of Object.entries(z.types || {})) {
        typeMult[i * T + resolveType(ref, `Climate "${id}".types`)] = mult(v, `Climate "${id}".types["${ref}"]`);
      }
      return { id, name: z.name !== undefined ? String(z.name) : id, color: z.color !== undefined ? String(z.color) : '#888888' };
    });
    return { zones, kindMult, typeMult };
  }

  // Turns the JSON config into lookup tables. Throws with a readable message on bad input.
  function compileRules(config) {
    if (!config || !Array.isArray(config.types) || config.types.length === 0) {
      throw new Error('The config needs a non-empty "types" array.');
    }
    const defs = config.types;
    const T = defs.length;
    if (T > MAX_TYPES) throw new Error(`At most ${MAX_TYPES} types are supported (found ${T}).`);

    const ids = defs.map((d, i) => {
      if (!d || typeof d !== 'object') throw new Error(`types[${i}] must be an object.`);
      return d.id !== undefined ? String(d.id) : String(i);
    });
    const index = new Map();
    ids.forEach((id, i) => {
      if (index.has(id)) throw new Error(`Duplicate type id "${id}".`);
      index.set(id, i);
    });
    // A reference can be a type id ("sand") or a position in the list (2).
    const resolve = (ref, where) => {
      if (index.has(String(ref))) return index.get(String(ref));
      if (Number.isInteger(ref) && ref >= 0 && ref < T) return ref;
      throw new Error(`${where}: unknown type "${ref}".`);
    };

    const allowed = new Uint32Array(T); // allowed[t] = mask of types that may sit next to t
    const weight = new Float64Array(T);
    const kinds = compileContinents(config.continents);
    const continent = new Int16Array(T).fill(-1); // index into kinds, -1 = none
    const near = new Float64Array(T * T).fill(NaN); // near[t*T + n] = weight of t when next to n

    const types = defs.map((d, i) => {
      const id = ids[i];
      const w = d.weight === undefined ? 1 : Number(d.weight);
      if (!(w >= 0)) throw new Error(`"${id}".weight must be a number >= 0.`);
      weight[i] = w;

      if (!Array.isArray(d.neighbors)) throw new Error(`"${id}" needs a "neighbors" array.`);
      for (const ref of d.neighbors) {
        const j = resolve(ref, `"${id}".neighbors`);
        // One side listing the pair is enough: it's allowed in both directions.
        allowed[i] |= bit(j);
        allowed[j] |= bit(i);
      }

      if (d.weightNear !== undefined) {
        if (!d.weightNear || typeof d.weightNear !== 'object' || Array.isArray(d.weightNear)) {
          throw new Error(`"${id}".weightNear must be an object like { "village": 0.7 }.`);
        }
        for (const [ref, v] of Object.entries(d.weightNear)) {
          const j = resolve(ref, `"${id}".weightNear`);
          const nv = Number(v);
          if (!(nv >= 0)) throw new Error(`"${id}".weightNear["${ref}"] must be a number >= 0.`);
          near[i * T + j] = nv;
        }
      }

      if (d.continent !== undefined && d.continent !== null && d.continent !== '' && d.continent !== 'none') {
        continent[i] = kinds.findIndex((k) => k.id === String(d.continent));
        if (continent[i] < 0) {
          throw new Error(`"${id}".continent must be one of ${kinds.map((k) => k.id).join(', ')} (or left out).`);
        }
      }

      const height = d.height === undefined ? 0.5 : Number(d.height);
      if (!Number.isFinite(height)) throw new Error(`"${id}".height must be a number.`);

      return {
        id,
        name: d.name !== undefined ? String(d.name) : id,
        color: d.color !== undefined ? String(d.color) : '#ff00ff',
        weight: w,
        height, // 3D view only; 0 = sea level
        continent: continent[i] >= 0 ? kinds[continent[i]].id : '',
        pattern: d.pattern ? String(d.pattern) : '', // texture, only used for drawing
      };
    });

    const climate = compileClimates(config.climates, kinds, T, resolve);

    // typeDist[a*T + b]: how many neighbour steps apart two types are (mountain → forest → grass
    // → sand = 3). The brush uses it to change cells as little as possible.
    const typeDist = new Uint8Array(T * T).fill(255);
    for (let a = 0; a < T; a++) {
      typeDist[a * T + a] = 0;
      const queue = [a];
      for (let qi = 0; qi < queue.length; qi++) {
        const u = queue[qi];
        for (let v = 0; v < T; v++) {
          if ((allowed[u] >>> v) & 1 && typeDist[a * T + v] === 255) {
            typeDist[a * T + v] = typeDist[a * T + u] + 1;
            queue.push(v);
          }
        }
      }
    }

    return { types, allowed, weight, near, continent, kinds, climate, typeDist };
  }

  class Solver {
    // opts: { width, height, radius2, selection: 'random'|'entropy', stability, continents, seed }
    // radius2: neighbours are the cells with dx² + dy² <= radius2 (1 → 4 cells, 2 → 8, 4 → 12, …)
    // stability: how strongly a cell copies its settled neighbours (0 = off), see optionsFor
    // continents: { count, strength } for the continental layer, or null for off
    // climate: { strength, layout: 'both'|'north' } for climate zones, or null for off
    // wrapX: true for a sphere world, where the left and right edges are neighbours
    constructor(rules, opts) {
      this.rules = rules;
      this.T = rules.types.length;
      this.W = opts.width;
      this.H = opts.height;
      this.N = this.W * this.H;
      this.environmentMasks = opts.environmentMasks || null;
      this.selection = opts.selection === 'entropy' ? 'entropy' : 'random';
      this.wrapX = !!opts.wrapX;
      this.rng = makeRng(opts.seed);
      this.full = this.T === 32 ? 0xffffffff : 2 ** this.T - 1;
      // Even original rules-only worlds keep land away from the map edges.
      if (!this.environmentMasks) {
        let water = 0;
        rules.types.forEach((t,i)=>{if(t.id==='water'||t.id==='deep_water')water|=1<<i;});
        if (water) {
          this.environmentMasks = new Uint32Array(this.N).fill(this.full);
          for(let i=0;i<this.N;i++) if(Math.min(i%this.W,Math.floor(i/this.W),this.W-1-i%this.W,this.H-1-Math.floor(i/this.W))<2)this.environmentMasks[i]=water>>>0;
        }
      }

      this.radius2 = Math.max(1, Math.floor(opts.radius2) || 1);
      const offsets = neighborOffsets(opts.radius2);
      this.DX = Int32Array.from(offsets, (o) => o[0]);
      this.DY = Int32Array.from(offsets, (o) => o[1]);
      this.D = offsets.length;
      this.stability = Math.max(0, Number(opts.stability) || 0);
      this.nearCounts = new Int32Array(this.T);
      this._buildClimate(opts.climate, opts.seed);
      this._buildContinents(opts.continents, opts.seed);

      this.dom = new Uint32Array(this.N);
      this.locked = new Uint8Array(this.N); // 1 = type was picked at random (not forced by neighbours)
      this.pinned = new Uint8Array(this.N); // 1 = painted with the brush: never changed by the solver
      this.pos = new Int32Array(this.N);
      this.queue = new Int32Array(this.N);
      this.inQueue = new Uint8Array(this.N);
      this.dirty = new Uint8Array(this.N);
      this.dirtyList = [];
      this.compatCache = new Map();
      this.trailCell = []; // changes made by the current pick, so it can be undone
      this.trailMask = [];
      this.recording = false;
      this.conflict = -1; // cell that ran out of options in the last failed propagation
      this.qLen = 0;
      this.open = 0; // cells still in superposition
      this.buckets = [];
      for (let k = 0; k <= this.T; k++) this.buckets.push([]);

      this.steps = 0; // random picks (cells settled by their neighbours don't count)
      this.backtracks = 0;
      this.repairs = 0;
      this.cleaned = 0; // cells changed by cleanup passes
      this.maxRepairs = Math.max(2000, this.N);
      this.lastRepair = -1;
      this.lastRadius = 2;
      this.seen = new Uint32Array(this.N);
      this.stack = new Int32Array(this.N);
      this.stamp = 0;
      this.message = '';

      this._reset();
      if (this.status === 'failed') {
        this.message = 'These rules can’t fill the grid at all: some cell ends up with no possible type.';
      }
    }

    // ---- public API -------------------------------------------------------

    // Locks one random cell that's still in superposition. Returns 'running', 'done' or 'failed'.
    step() {
      if (this.status !== 'running') return this.status;
      const c = this._selectCell();
      if (c < 0) return (this.status = 'done');

      const t = this._chooseType(c);
      this.steps++;
      this.trailCell.length = 0;
      this.trailMask.length = 0;
      this.recording = true;
      this._set(c, bit(t));
      this.locked[c] = 1;
      this._enqueue(c);
      const ok = this._propagate();
      this.recording = false;

      if (!ok) {
        // Undo this pick and forbid it here. Usually that's enough.
        this.backtracks++;
        this._undoTrail();
        this.locked[c] = 0;
        this._set(c, (this.dom[c] & ~bit(t)) >>> 0);
        this._enqueue(c);
        if (!this._propagate()) this._repair(this.conflict);
      }

      if (this.status === 'running' && this.open === 0) this.status = 'done';
      return this.status;
    }

    // Type index if the cell is settled, otherwise -1.
    typeAt(c) {
      const m = this.dom[c];
      return m !== 0 && (m & (m - 1)) === 0 ? 31 - Math.clz32(m) : -1;
    }

    // The types a cell can still become, with the weights it would roll with right now.
    optionsFor(c) {
      const { T, dom, D, DX, DY, W, H, stability, nearCounts, contShare, contStrength, contNeutral } = this;
      const { climShare, Z } = this;
      const { weight, near } = this.rules;
      const x = c % W;
      const y = (c / W) | 0;

      let nearMask = 0; // types of the settled neighbours
      let settled = 0;
      nearCounts.fill(0); // how many settled neighbours have each type
      for (let k = 0; k < D; k++) {
        let nx = x + DX[k];
        const ny = y + DY[k];
        if (ny < 0 || ny >= H) continue;
        if (nx < 0 || nx >= W) {
          if (!this.wrapX) continue;
          nx += nx < 0 ? W : -W; // sphere: the left and right edges touch
        }
        const nm = dom[ny * W + nx];
        if (nm !== 0 && (nm & (nm - 1)) === 0) {
          nearMask |= nm;
          nearCounts[31 - Math.clz32(nm)]++;
          settled++;
        }
      }

      const m = dom[c];
      const out = [];
      for (let t = 0; t < T; t++) {
        if (!((m >>> t) & 1)) continue;
        // A weightNear entry overrides the base weight when a matching neighbour exists.
        // If several match, the highest one wins.
        let boosted = -1;
        if (nearMask !== 0) {
          for (let j = 0; j < T; j++) {
            if ((nearMask >>> j) & 1) {
              const v = near[t * T + j];
              if (v > boosted) boosted = v;
            }
          }
        }
        let w = boosted >= 0 ? boosted : weight[t];
        // Continental layer: favour the types that belong to the kind of land this cell is in.
        if (contShare) {
          const k = this.rules.continent[t];
          w *= k >= 0 ? Math.exp(contStrength * contShare[c * this.K + k]) : contNeutral;
        }
        // Climate: blend the zones' multipliers for this type by how much of each zone the cell is in.
        if (climShare) {
          const typeMult = this.rules.climate.typeMult;
          let f = 0;
          for (let z = 0; z < Z; z++) f += climShare[c * Z + z] * typeMult[z * T + t];
          w *= this.climStrength === 1 ? f : Math.pow(f, this.climStrength);
        }
        // Brush: a cell reopened around new paint keeps its old type wherever the rules allow, and
        // otherwise takes the type nearest to it (×1000 per step closer), so the change around
        // the paint is as small as the rules allow.
        if (this.prefer && this.prefer[c] >= 0) {
          const steps = Math.min(6, this.rules.typeDist[this.prefer[c] * T + t]);
          w *= Math.pow(1000, 2 - steps);
        }
        // Stability: the more settled neighbours already have this type, the likelier it gets.
        // The weight is multiplied by e^(stability × share of settled neighbours with this type),
        // so at full agreement strength 3 gives ×20 and strength 10 gives ×22 000.
        if (stability > 0 && settled > 0) w *= Math.exp((stability * nearCounts[t]) / settled);
        out.push({ type: t, weight: w });
      }
      return out;
    }

    // Calls fn(cell, mask) for every cell that changed since the last call.
    consumeDirty(fn) {
      const list = this.dirtyList;
      for (let i = 0; i < list.length; i++) {
        const c = list[i];
        this.dirty[c] = 0;
        fn(c, this.dom[c]);
      }
      list.length = 0;
    }

    // One cleanup pass over a finished map. A cell whose type is rare around it (at most a quarter
    // of its neighbours share it) switches to the type that makes up at least half of its
    // neighbours, but only if that type is allowed next to all of them, so the rules still hold.
    // Cells are visited in random order and see earlier changes. Returns how many cells changed.
    cleanup() {
      if (this.status !== 'done') return 0;
      const { N, W, H, D, DX, DY, T, dom, nearCounts, rng } = this;
      const allowed = this.rules.allowed;

      const order = new Int32Array(N);
      for (let i = 0; i < N; i++) order[i] = i;
      for (let i = N - 1; i > 0; i--) {
        const j = (rng() * (i + 1)) | 0;
        const tmp = order[i];
        order[i] = order[j];
        order[j] = tmp;
      }

      let changed = 0;
      for (let i = 0; i < N; i++) {
        const c = order[i];
        if (this.pinned[c]) continue; // never clean up what the user painted
        const t = this.typeAt(c);
        const x = c % W;
        const y = (c / W) | 0;
        nearCounts.fill(0);
        let total = 0;
        let around = 0; // mask of every type next to this cell
        for (let k = 0; k < D; k++) {
          let nx = x + DX[k];
          const ny = y + DY[k];
          if (ny < 0 || ny >= H) continue;
          if (nx < 0 || nx >= W) {
            if (!this.wrapX) continue;
            nx += nx < 0 ? W : -W; // sphere: the left and right edges touch
          }
          const m = dom[ny * W + nx];
          around = (around | m) >>> 0;
          nearCounts[31 - Math.clz32(m)]++;
          total++;
        }
        if (total === 0 || nearCounts[t] * 4 > total) continue;

        let best = -1;
        for (let u = 0; u < T; u++) {
          if (this.environmentMasks && !((this.environmentMasks[c] >>> u) & 1)) continue;
          if (u === t || (best >= 0 && nearCounts[u] <= nearCounts[best])) continue;
          if (((allowed[u] & around) >>> 0) === around) best = u;
        }
        if (best >= 0 && nearCounts[best] * 2 >= total) {
          this._write(c, bit(best));
          changed++;
        }
      }
      this.cleaned += changed;
      return changed;
    }

    // ---- brush ----------------------------------------------------------------

    // Paints `cells` with type t and pins them, then reopens the smallest margin around them that
    // lets the rules fit, so only what has to change changes: water painted into a mountain gets
    // rings of sand, grass and forest, and the rest of the mountain stays. Reopened cells strongly
    // prefer their old type when refilled. As a last resort earlier paint in the margin may change
    // too. Afterwards status is 'running' until step() has refilled the margin.
    // Returns false (map unchanged, reason in this.message) if the type can't fit there at all.
    paint(cells, t) {
      if (this.status !== 'done' || !cells.length) return false;
      if (this.environmentMasks && cells.some(c => !((this.environmentMasks[c] >>> t) & 1))) {
        this.message = 'This terrain is incompatible with the environment here. Change elevation or climate first.';
        return false;
      }
      const maxMargin = 4 * (2 * Math.ceil(Math.sqrt(this.radius2)) + 2);
      const { dist, queue } = this._paintDistances(cells, maxMargin + 1);
      if (!this.prefer) this.prefer = new Int16Array(this.N).fill(-1);
      for (const c of queue) this.prefer[c] = this.typeAt(c);

      const margins = [];
      for (let m = 0; m <= maxMargin; m = m < 4 ? m + 1 : Math.ceil(m * 1.35)) margins.push(m);
      const oldPaintNearby = queue.some((c) => dist[c] > 0 && this.pinned[c]);
      const snap = this.snapshot();
      for (const freeOldPaint of oldPaintNearby ? [false, true] : [false]) {
        for (const margin of margins) {
          if (this._tryPaint(cells, t, dist, queue, margin, freeOldPaint)) {
            this.status = this.open > 0 ? 'running' : 'done';
            return true;
          }
          this.restore(snap);
        }
      }
      this.message = `${this.rules.types[t].name} can't fit there: its neighbour rules clash with what's around it.`;
      return false;
    }

    // Distance of every cell from the paint in 8-neighbour steps, up to `limit`. `queue` lists the
    // reached cells nearest first.
    _paintDistances(cells, limit) {
      const { W, H, N } = this;
      const dist = this.paintDist || (this.paintDist = new Int16Array(N));
      dist.fill(-1);
      const queue = [];
      for (const c of cells) {
        if (dist[c] < 0) {
          dist[c] = 0;
          queue.push(c);
        }
      }
      for (let qi = 0; qi < queue.length; qi++) {
        const c = queue[qi];
        const d = dist[c];
        if (d >= limit) continue;
        const x = c % W;
        const y = (c / W) | 0;
        for (let dy = -1; dy <= 1; dy++) {
          const ny = y + dy;
          if (ny < 0 || ny >= H) continue;
          for (let dx = -1; dx <= 1; dx++) {
            let nx = x + dx;
            if (nx < 0 || nx >= W) {
              if (!this.wrapX) continue;
              nx += nx < 0 ? W : -W;
            }
            const n = ny * W + nx;
            if (dist[n] < 0) {
              dist[n] = d + 1;
              queue.push(n);
            }
          }
        }
      }
      return { dist, queue };
    }

    // One attempt: paint, reopen everything within `margin`, and check the rules still fit.
    _tryPaint(cells, t, dist, queue, margin, freeOldPaint) {
      const { locked, pinned, dom, full } = this;
      const mask = bit(t);
      for (const c of queue) {
        const d = dist[c];
        if (d > margin + 1) break; // nearest first, so nothing further is involved
        if (d === 0) {
          pinned[c] = 1;
          locked[c] = 1;
          if (dom[c] !== mask) this._write(c, mask);
        } else if (d <= margin && (!pinned[c] || freeOldPaint)) {
          pinned[c] = 0;
          locked[c] = 0;
          if (dom[c] !== full) this._write(c, full);
        }
        this._enqueue(c); // the ring just outside the margin stays as it is and constrains it
      }
      return this._propagate();
    }

    snapshot() {
      return { dom: this.dom.slice(), locked: this.locked.slice(), pinned: this.pinned.slice(), status: this.status };
    }

    restore(s) {
      while (this.qLen > 0) this.inQueue[this.queue[--this.qLen]] = 0;
      for (let c = 0; c < this.N; c++) if (this.dom[c] !== s.dom[c]) this._write(c, s.dom[c]);
      this.locked.set(s.locked);
      this.pinned.set(s.pinned);
      this.status = s.status;
    }

    // Forces a full redraw on the next consumeDirty (e.g. after a colour change).
    markAllDirty() {
      this.dirtyList.length = 0;
      for (let c = 0; c < this.N; c++) {
        this.dirty[c] = 1;
        this.dirtyList.push(c);
      }
    }

    get settled() {
      return this.N - this.open;
    }

    // ---- internals --------------------------------------------------------

    // Climate zones: latitude bands from the equator (zone 0) to the poles (last zone). With layout
    // 'both' the equator runs across the middle and the bands mirror toward the top and bottom; with
    // 'north' it runs along the bottom. A seeded wobble keeps the borders from being straight, and
    // each cell gets a smooth share of the nearest zones so they blend instead of switching.
    _buildClimate(cl, seed) {
      this.climShare = null;
      const Z = (this.Z = this.rules.climate.zones.length);
      const strength = cl ? Number(cl.strength) : 0;
      if (!(strength > 0 && Z > 0)) return;

      const { W, H, N } = this;
      const rng = makeRng((seed ^ 0x2c1b3c6d) >>> 0);
      const TAU = Math.PI * 2;
      // Whole numbers of waves across the width, so the wobble lines up where a sphere wraps around.
      const wave = () => ({ fx: (TAU * Math.floor(1 + rng() * 3)) / W, fy: (TAU * (rng() * 2)) / H, phase: rng() * TAU });
      this.climBoth = cl.layout !== 'north';
      this.climWaves = [wave(), wave(), wave()];
      this.climStrength = strength;

      const share = new Float32Array(N * Z);
      const zones = new Float64Array(Z);
      for (let c = 0; c < N; c++) {
        this._zoneShares((c % W) + 0.5, ((c / W) | 0) + 0.5, zones);
        for (let z = 0; z < Z; z++) share[c * Z + z] = zones[z];
      }
      this.climShare = share;
    }

    // Fills `out` with how much of each climate zone the point (x, y) is in; the shares add up to 1.
    _zoneShares(x, y, out) {
      const Z = this.Z;
      const v = y / this.H;
      let lat = this.climBoth ? Math.abs(2 * v - 1) : 1 - v; // 0 = equator, 1 = pole
      const [a, b, c] = this.climWaves;
      lat +=
        0.07 * (0.5 * Math.sin(x * a.fx + y * a.fy + a.phase) +
          0.3 * Math.sin(x * b.fx * 2 + y * b.fy + b.phase) +
          0.2 * Math.sin(x * c.fx * 3 + y * c.fy * 2 + c.phase));
      lat = Math.min(1, Math.max(0, lat));
      const width = 0.55 / Z;
      let total = 0;
      for (let z = 0; z < Z; z++) {
        const d = (lat - (z + 0.5) / Z) / width;
        out[z] = Math.exp(-d * d);
        total += out[z];
      }
      for (let z = 0; z < Z; z++) out[z] /= total;
    }

    // The climate zone a cell is mostly in, or -1 when climate zones are off.
    zoneAt(c) {
      if (!this.climShare) return -1;
      let best = 0;
      for (let z = 1; z < this.Z; z++) if (this.climShare[c * this.Z + z] > this.climShare[c * this.Z + best]) best = z;
      return best;
    }

    // Continental layer: a few random points, each of one kind (water, land, desert, …). Every cell
    // gets a share of each kind, weighting the points by inverse square distance, so a cell right
    // next to a water point is ~100% water and one halfway between a water and a land point is
    // 50/50. optionsFor multiplies a type's weight by e^(strength × share of its kind); types
    // without a kind get the average boost e^(strength / kinds), so they stay equally likely
    // everywhere.
    _buildContinents(cont, seed) {
      this.contShare = null;
      this.contPoints = [];
      const kinds = this.rules.kinds;
      const K = (this.K = kinds.length);
      const count = cont ? Math.floor(cont.count) : 0;
      const strength = cont ? Number(cont.strength) : 0;
      if (!(count > 0 && strength > 0 && K > 0)) return;

      const { W, H, N } = this;
      const rng = makeRng((seed ^ 0x5bd1e995) >>> 0);
      // The two most likely kinds always get a point, so a map can't end up with no sea or no land.
      const byOdds = kinds.map((k, i) => i).sort((a, b) => kinds[b].odds - kinds[a].odds);
      const odds = new Float64Array(K);
      const zones = new Float64Array(this.Z);
      for (let i = 0; i < count; i++) {
        const x = rng() * W;
        const y = rng() * H;
        let k = byOdds[i];
        if (i >= Math.min(2, K)) {
          // Each kind's odds, times what the climate at this spot thinks of it.
          if (this.climShare) this._zoneShares(x, y, zones);
          let total = 0;
          for (let kk = 0; kk < K; kk++) {
            let o = kinds[kk].odds;
            if (this.climShare) {
              let f = 0;
              for (let z = 0; z < this.Z; z++) f += zones[z] * this.rules.climate.kindMult[z * K + kk];
              o *= Math.pow(f, this.climStrength);
            }
            odds[kk] = o;
            total += o;
          }
          let r = rng() * total;
          k = total > 0 ? odds.findIndex((o) => (r -= o) < 0) : (rng() * K) | 0;
          if (k < 0) k = K - 1;
        }
        this.contPoints.push({ x, y, k, kind: kinds[k] });
      }

      const share = new Float32Array(N * K);
      const sums = new Float64Array(K);
      for (let c = 0; c < N; c++) {
        const x = (c % W) + 0.5;
        const y = ((c / W) | 0) + 0.5;
        sums.fill(0);
        let total = 0;
        for (const p of this.contPoints) {
          let dx = p.x - x;
          if (this.wrapX) dx -= W * Math.round(dx / W); // shortest way round the sphere
          const dy = p.y - y;
          const w = 1 / (dx * dx + dy * dy + 1);
          sums[p.k] += w;
          total += w;
        }
        for (let k = 0; k < K; k++) share[c * K + k] = sums[k] / total;
      }
      this.contShare = share;
      this.contStrength = strength;
      this.contNeutral = Math.exp(strength / K);
    }

    // Everything back into full superposition, except painted cells.
    _reset() {
      this.locked.set(this.pinned);
      this._relaxBox(0, 0, this.W - 1, this.H - 1);
      this.status = this._propagate() ? 'running' : 'failed';
      if (this.status === 'running' && this.open === 0) this.status = 'done';
    }

    // Puts every cell in the box that isn't locked back into full superposition and queues the
    // box plus a one-cell ring around it, so constraints from outside flow back in.
    _relaxBox(x0, y0, x1, y1) {
      for (let y = y0; y <= y1; y++) {
        for (let x = x0; x <= x1; x++) {
          const c = y * this.W + x;
          if (!this.locked[c] && this.dom[c] !== this.full) this._write(c, this.full);
        }
      }
      const ex0 = Math.max(0, x0 - 1);
      const ey0 = Math.max(0, y0 - 1);
      const ex1 = Math.min(this.W - 1, x1 + 1);
      const ey1 = Math.min(this.H - 1, y1 + 1);
      for (let y = ey0; y <= ey1; y++) {
        for (let x = ex0; x <= ex1; x++) this._enqueue(y * this.W + x);
      }
    }

    // Unlocks the cells within radius r of the conflict, then rebuilds every unlocked cell's
    // options from the locked cells that remain. If that still contradicts, the radius doubles;
    // past the map size everything is wiped. Repeated failures in the same spot start bigger.
    _repair(center) {
      const cx0 = center % this.W;
      const cy0 = (center / this.W) | 0;
      const nearLast =
        this.lastRepair >= 0 &&
        Math.max(Math.abs(cx0 - (this.lastRepair % this.W)), Math.abs(cy0 - ((this.lastRepair / this.W) | 0))) <=
          2 * this.lastRadius;
      let r = nearLast ? this.lastRadius * 2 : 2;

      for (; ; r *= 2) {
        if (++this.repairs > this.maxRepairs) {
          this.status = 'failed';
          this.message =
            `Gave up after ${this.maxRepairs.toLocaleString('en-US')} repairs: the rules contradict each ` +
            'other too often. "Fewest options first" copes better with strict rules than random order.';
          return;
        }
        if (center < 0 || r >= Math.max(this.W, this.H)) {
          this.lastRepair = -1;
          this._reset();
          if (this.status === 'failed') {
            this.message = 'These rules can’t fill the grid at all: some cell ends up with no possible type.';
          }
          return;
        }
        const cx = center % this.W;
        const cy = (center / this.W) | 0;
        const x0 = Math.max(0, cx - r);
        const y0 = Math.max(0, cy - r);
        const x1 = Math.min(this.W - 1, cx + r);
        const y1 = Math.min(this.H - 1, cy + r);
        for (let y = y0; y <= y1; y++) {
          for (let x = x0; x <= x1; x++) {
            const c = y * this.W + x;
            if (!this.pinned[c]) this.locked[c] = 0; // painted cells stay
          }
        }
        this.lastRepair = center;
        this.lastRadius = r;
        this._relaxConnected(x0, y0, x1, y1);
        if (this._propagate()) return;
        center = this.conflict;
      }
    }

    // Puts the box back into superposition, along with every unlocked cell connected to it.
    // A removed lock can only have restricted cells through chains of unlocked cells (locked
    // cells never change), so nothing beyond that region needs recomputing. Locked cells on its
    // border are queued so their constraints flow back in.
    _relaxConnected(x0, y0, x1, y1) {
      const { seen, stack, D, DX, DY, W, H, locked, dom, full } = this;
      const stamp = ++this.stamp;
      let sp = 0;
      for (let y = y0; y <= y1; y++) {
        for (let x = x0; x <= x1; x++) {
          const c = y * this.W + x;
          seen[c] = stamp;
          stack[sp++] = c;
        }
      }
      while (sp > 0) {
        const c = stack[--sp];
        this._enqueue(c);
        if (locked[c]) continue;
        if (dom[c] !== full) this._write(c, full);
        const x = c % W;
        const y = (c / W) | 0;
        for (let k = 0; k < D; k++) {
          let nx = x + DX[k];
          const ny = y + DY[k];
          if (ny < 0 || ny >= H) continue;
          if (nx < 0 || nx >= W) {
            if (!this.wrapX) continue;
            nx += nx < 0 ? W : -W; // sphere: the left and right edges touch
          }
          const n = ny * W + nx;
          if (seen[n] !== stamp) {
            seen[n] = stamp;
            stack[sp++] = n;
          }
        }
      }
    }

    _set(c, m) {
      if (this.recording) {
        this.trailCell.push(c);
        this.trailMask.push(this.dom[c]);
      }
      this._write(c, m);
    }

    _undoTrail() {
      const { trailCell, trailMask } = this;
      while (trailCell.length) this._write(trailCell.pop(), trailMask.pop());
    }

    _write(c, m) {
      if (this.environmentMasks) m = (m & this.environmentMasks[c]) >>> 0;
      const k0 = popcount(this.dom[c]);
      const k1 = popcount(m);
      this.dom[c] = m;
      if (k0 !== k1) {
        if (k0 >= 2) this._bucketRemove(k0, c);
        if (k1 >= 2) this._bucketAdd(k1, c);
      }
      if (!this.dirty[c]) {
        this.dirty[c] = 1;
        this.dirtyList.push(c);
      }
    }

    // buckets[k] holds the cells with exactly k options left (k >= 2), for O(1) random picks.
    _bucketAdd(k, c) {
      const b = this.buckets[k];
      this.pos[c] = b.length;
      b.push(c);
      this.open++;
    }

    _bucketRemove(k, c) {
      const b = this.buckets[k];
      const i = this.pos[c];
      const last = b.pop();
      if (last !== c) {
        b[i] = last;
        this.pos[last] = i;
      }
      this.open--;
    }

    _enqueue(c) {
      if (!this.inQueue[c]) {
        this.inQueue[c] = 1;
        this.queue[this.qLen++] = c;
      }
    }

    // Union of everything the types in `mask` are allowed to touch.
    _compat(mask) {
      let v = this.compatCache.get(mask);
      if (v === undefined) {
        v = 0;
        for (let t = 0; t < this.T; t++) if ((mask >>> t) & 1) v |= this.rules.allowed[t];
        v >>>= 0;
        this.compatCache.set(mask, v);
      }
      return v;
    }

    // Spreads restrictions outward until nothing changes. Returns false on contradiction.
    _propagate() {
      const { dom, D, DX, DY, W, H, queue, inQueue } = this;
      while (this.qLen > 0) {
        const c = queue[--this.qLen];
        inQueue[c] = 0;
        const compat = this._compat(dom[c]);
        const x = c % W;
        const y = (c / W) | 0;
        for (let k = 0; k < D; k++) {
          let nx = x + DX[k];
          const ny = y + DY[k];
          if (ny < 0 || ny >= H) continue;
          if (nx < 0 || nx >= W) {
            if (!this.wrapX) continue;
            nx += nx < 0 ? W : -W; // sphere: the left and right edges touch
          }
          const n = ny * W + nx;
          const m = dom[n];
          const m2 = (m & compat) >>> 0;
          if (m2 === m) continue;
          this._set(n, m2);
          if (m2 === 0) {
            this.conflict = n;
            while (this.qLen > 0) inQueue[queue[--this.qLen]] = 0;
            return false;
          }
          this._enqueue(n);
        }
      }
      return true;
    }

    _selectCell() {
      if (this.open === 0) return -1;
      const { buckets, T, rng } = this;
      if (this.selection === 'entropy') {
        // Fewest options first; this grows the map outward from what's already settled.
        for (let k = 2; k <= T; k++) {
          const b = buckets[k];
          if (b.length) return b[(rng() * b.length) | 0];
        }
        return -1;
      }
      // Uniformly random among every cell still in superposition.
      let r = (rng() * this.open) | 0;
      for (let k = 2; k <= T; k++) {
        const b = buckets[k];
        if (r < b.length) return b[r];
        r -= b.length;
      }
      return -1;
    }

    _chooseType(c) {
      const opts = this.optionsFor(c);
      let total = 0;
      for (const o of opts) total += o.weight;
      if (total <= 0) return opts[(this.rng() * opts.length) | 0].type;
      let r = this.rng() * total;
      for (const o of opts) {
        r -= o.weight;
        if (r < 0) return o.type;
      }
      return opts[opts.length - 1].type;
    }
  }

  global.TerrainWFC = { compileRules, Solver, seedFromString, makeRng, neighborSteps, MAX_TYPES };
})(globalThis);
