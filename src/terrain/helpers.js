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

  class SolverView {
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


  }
  global.TerrainWFC = { compileRules, SolverView, seedFromString, makeRng, neighborSteps, MAX_TYPES };
})(globalThis);
