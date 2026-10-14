// CSS pixels only. The SVG viewport may be letterboxed; DPR belongs to sampling.
export function viewport(rect, camera) {
  const scale = Math.min(rect.width / camera.w, rect.height / camera.h);
  return {
    scale,
    left: rect.left + (rect.width - camera.w * scale) / 2,
    top: rect.top + (rect.height - camera.h * scale) / 2,
  };
}
export function screenToMap(x, y, rect, camera) {
  const v = viewport(rect, camera);
  return [camera.x + (x - v.left) / v.scale, camera.y + (y - v.top) / v.scale];
}
export function mapToScreen(p, rect, camera) {
  const v = viewport(rect, camera);
  return [
    v.left + (p[0] - camera.x) * v.scale,
    v.top + (p[1] - camera.y) * v.scale,
  ];
}
export const projectToMap = (p, state) => [
  (p[0] * 2) / (state.W - 1),
  p[1] / (state.H - 1),
];
export const mapToProject = (p, state) => [
  (p[0] * (state.W - 1)) / 2,
  p[1] * (state.H - 1),
];
export function contains(p, points) {
  let hit = false;
  for (let i = 0, j = points.length - 1; i < points.length; j = i++) {
    const a = points[i],
      b = points[j];
    if (
      a[1] > p[1] !== b[1] > p[1] &&
      p[0] < ((b[0] - a[0]) * (p[1] - a[1])) / (b[1] - a[1]) + a[0]
    )
      hit = !hit;
  }
  return hit;
}
export function segmentDistance(p, a, b) {
  const dx = b[0] - a[0],
    dy = b[1] - a[1],
    t = Math.max(
      0,
      Math.min(
        1,
        ((p[0] - a[0]) * dx + (p[1] - a[1]) * dy) / (dx * dx + dy * dy || 1),
      ),
    );
  return Math.hypot(p[0] - a[0] - t * dx, p[1] - a[1] - t * dy);
}
export function bounds(points) {
  const xs = points.map((p) => p[0]),
    ys = points.map((p) => p[1]);
  return {
    x: Math.min(...xs),
    y: Math.min(...ys),
    w: Math.max(...xs) - Math.min(...xs),
    h: Math.max(...ys) - Math.min(...ys),
  };
}
export function anchor(points, geometry) {
  if (points.length === 1) return points[0];
  if (geometry === "line") {
    const lengths = points
      .slice(1)
      .map((p, i) => Math.hypot(p[0] - points[i][0], p[1] - points[i][1]));
    let half = lengths.reduce((a, b) => a + b, 0) / 2;
    for (let i = 0; i < lengths.length; i++) {
      if (half <= lengths[i])
        return points[i].map(
          (v, k) => v + ((points[i + 1][k] - v) * half) / (lengths[i] || 1),
        );
      half -= lengths[i];
    }
  }
  // Recovered scanline interior-anchor strategy from the former illustrator.
  const b = bounds(points);
  let best = null;
  for (let row = 1; row < 24; row++) {
    const y = b.y + (b.h * row) / 24,
      xs = [];
    for (let i = 0, j = points.length - 1; i < points.length; j = i++) {
      const a = points[j],
        z = points[i];
      if (a[1] > y !== z[1] > y)
        xs.push(a[0] + ((y - a[1]) * (z[0] - a[0])) / (z[1] - a[1]));
    }
    xs.sort((a, b) => a - b);
    for (let i = 1; i < xs.length; i += 2) {
      const w = xs[i] - xs[i - 1];
      const score = w - Math.abs(y - b.y - b.h / 2) * 0.1;
      if (!best || score > best.score)
        best = { score, p: [(xs[i] + xs[i - 1]) / 2, y] };
    }
  }
  return best?.p || points[0];
}
export class FeatureIndex {
  constructor(features) {
    this.bins = new Map();
    this.features = features;
    for (const f of features) {
      if (!f.points?.length) continue;
      const b = bounds(f.points);
      for (let y = Math.floor(b.y * 32); y <= Math.floor((b.y + b.h) * 32); y++)
        for (
          let x = Math.floor(b.x * 32);
          x <= Math.floor((b.x + b.w) * 32);
          x++
        ) {
          const key = x + "," + y;
          if (!this.bins.has(key)) this.bins.set(key, []);
          this.bins.get(key).push(f);
        }
    }
  }
  pick(p, tolerance) {
    const candidates = new Set();
    for (
      let y = Math.floor((p[1] - tolerance) * 32);
      y <= Math.floor((p[1] + tolerance) * 32);
      y++
    )
      for (
        let x = Math.floor((p[0] - tolerance) * 32);
        x <= Math.floor((p[0] + tolerance) * 32);
        x++
      )
        for (const f of this.bins.get(x + "," + y) || []) candidates.add(f);
    return [...candidates]
      .filter((f) =>
        f.geometry === "point"
          ? Math.hypot(p[0] - f.points[0][0], p[1] - f.points[0][1]) <=
            tolerance
          : (f.geometry === "polygon" && contains(p, f.points)) ||
            f.points
              .slice(1)
              .some((q, i) => segmentDistance(p, f.points[i], q) <= tolerance),
      )
      .sort(
        (a, b) =>
          (a.priority ?? priority(a)) - (b.priority ?? priority(b)) ||
          a.id.localeCompare(b.id),
      );
  }
}
function priority(f) {
  return f.geometry === "point" || ["house", "building"].includes(f.kind)
    ? 0
    : f.geometry === "line"
      ? 1
      : ["district", "city", "town", "village", "capital"].includes(f.kind)
        ? 2
        : 3;
}
