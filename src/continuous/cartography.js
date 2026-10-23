import { geographicParts } from "./geographic-path.js";
import { riverFeatures, riverVisible } from "./rivers.js";

export const cartographicLevels = [
  { label: "World & continents", tolerance: 0.0012 },
  { label: "Regional geography", tolerance: 0.00025 },
  { label: "Local terrain", tolerance: 0.00003 },
  { label: "Editing detail", tolerance: 0 },
];
const boundaries = [0.8, 0.15, 0.015];
const noCoasts = Object.freeze([]);
export function cartographicLevel(width, previous) {
  const level = boundaries.filter((b) => width < b).length;
  if (previous === undefined || previous === level) return level;
  // A 12% dead band prevents wheel/pinch jitter around a presentation boundary.
  if (level > previous && width > boundaries[previous] * 0.88) return previous;
  if (level < previous && width < boundaries[previous - 1] * 1.12)
    return previous;
  return level;
}
const pointKey = (p) => `${p[0].toFixed(12)},${p[1].toFixed(12)}`;
function distanceSquared(p, a, b) {
  const dx = b[0] - a[0],
    dy = b[1] - a[1];
  const t = Math.max(
    0,
    Math.min(
      1,
      ((p[0] - a[0]) * dx + (p[1] - a[1]) * dy) / (dx * dx + dy * dy || 1),
    ),
  );
  return (p[0] - a[0] - dx * t) ** 2 + (p[1] - a[1] - dy * t) ** 2;
}
export function simplifyPath(points, tolerance, anchors = new Set()) {
  if (!tolerance || points.length < 3) return points;
  const keep = new Set([0, points.length - 1]);
  points.forEach((p, i) => {
    if (anchors.has(pointKey(p))) keep.add(i);
  });
  const locked = [...keep].sort((a, b) => a - b);
  const stack = locked.slice(1).map((end, i) => [locked[i], end]);
  while (stack.length) {
    const [a, b] = stack.pop();
    let far = -1,
      max = tolerance * tolerance;
    for (let i = a + 1; i < b; i++) {
      const d = distanceSquared(points[i], points[a], points[b]);
      if (d > max) {
        max = d;
        far = i;
      }
    }
    if (far >= 0) {
      keep.add(far);
      stack.push([a, far], [far, b]);
    }
  }
  return [...keep].sort((a, b) => a - b).map((i) => points[i]);
}

// Source features and their presentation cache are separate from raster/query
// caches. Never simplify clipped tiles: adjacent views use identical geometry.
export class CartographicGeometry {
  constructor({ budget = 16 * 1024 * 1024, limit = 24 } = {}) {
    this.cache = new Map();
    this.bytes = 0;
    this.budget = budget;
    this.limit = limit;
    this.stats = { hits: 0, misses: 0 };
    this.sources = new WeakMap();
    this.sourceSerial = 0;
  }
  derive({
    identity,
    revision,
    rivers,
    coasts = noCoasts,
    level,
    fast = false,
  }) {
    const sourceKey = (source) => {
      if (!source) return 0;
      if (!this.sources.has(source))
        this.sources.set(source, ++this.sourceSerial);
      return this.sources.get(source);
    };
    const key = JSON.stringify([
      identity,
      revision,
      sourceKey(rivers),
      sourceKey(coasts),
      level,
      fast,
    ]);
    const hit = this.cache.get(key);
    if (hit) {
      this.cache.delete(key);
      this.cache.set(key, hit);
      this.stats.hits++;
      return hit.value;
    }
    this.stats.misses++;
    const started = performance.now();
    const all = riverFeatures(rivers);
    const anchors = new Set(
      all.flatMap((f) => [pointKey(f.points[0]), pointKey(f.points.at(-1))]),
    );
    const byId = new Map(rivers.map((r) => [r.id, r]));
    const selected = new Set();
    const include = (r) => {
      if (!r || selected.has(r.id)) return;
      selected.add(r.id);
      include(byId.get(r.destination));
      include(byId.get(r.parent));
    };
    rivers
      .filter((r) =>
        riverVisible(
          r,
          level === 0 ? "world" : level === 1 ? "continent" : "local",
        ),
      )
      .forEach(include);
    const tolerance = cartographicLevels[level].tolerance * (fast ? 1.6 : 1);
    const value = {
      rivers: all
        .filter((f) => selected.has(f.sourceId))
        .map((f) => ({
          ...f,
          points: simplifyPath(f.points, tolerance, anchors),
        })),
      coasts: (coasts || [])
        .flatMap(geographicParts)
        .map((p) => simplifyPath(p, tolerance)),
      level,
    };
    value.durationMs = performance.now() - started;
    const bytes =
      256 +
      value.rivers.reduce((n, f) => n + f.river.path.length * 128 + 512, 0) +
      [...value.rivers.map((f) => f.points), ...value.coasts].reduce(
        (n, p) => n + p.length * 96 + 256,
        0,
      );
    if (bytes <= this.budget) {
      this.cache.set(key, { value, bytes });
      this.bytes += bytes;
      while (this.bytes > this.budget || this.cache.size > this.limit) {
        const oldest = this.cache.keys().next().value;
        this.bytes -= this.cache.get(oldest).bytes;
        this.cache.delete(oldest);
      }
    }
    return value;
  }
}
