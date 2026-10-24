import { ShapeUtils, Vector2 } from "three";
// Clip in geographic space before evaluating height. A tile is a rendering
// boundary only; it never becomes a stored endpoint or moves an authored vertex.
export function clipSegment(a, b, bounds) {
  let lo = 0,
    hi = 1;
  const dx = b[0] - a[0],
    dy = b[1] - a[1];
  for (const [p, q] of [
    [-dx, a[0] - bounds.x],
    [dx, bounds.x + bounds.w - a[0]],
    [-dy, a[1] - bounds.y],
    [dy, bounds.y + bounds.h - a[1]],
  ]) {
    if (p === 0) {
      if (q < 0) return null;
      continue;
    }
    const t = q / p;
    if (p < 0) lo = Math.max(lo, t);
    else hi = Math.min(hi, t);
    if (lo > hi) return null;
  }
  return [
    [a[0] + dx * lo, a[1] + dy * lo],
    [a[0] + dx * hi, a[1] + dy * hi],
  ];
}

export function meshBounds(m) {
  const [w, s, e, n] = m.bounds;
  return {
    x: w / Math.PI + 1,
    y: 0.5 - n / Math.PI,
    w: (e - w) / Math.PI,
    h: (n - s) / Math.PI,
  };
}
export function meshHeight(m, p) {
  const b = meshBounds(m),
    x = Math.max(
      0,
      Math.min(m.columns - 1, ((p[0] - b.x) / b.w) * (m.columns - 1)),
    ),
    y = Math.max(0, Math.min(m.rows - 1, ((p[1] - b.y) / b.h) * (m.rows - 1))),
    i = Math.min(m.columns - 2, Math.floor(x)),
    j = Math.min(m.rows - 2, Math.floor(y)),
    u = x - i,
    v = y - j;
  const h = (ix, iy) => {
    const k = iy * m.columns + ix;
    return m.wet[k] ? Math.max(m.bed[k], m.surface[k]) : m.bed[k];
  };
  return u + v <= 1
    ? h(i, j) * (1 - u - v) + h(i + 1, j) * u + h(i, j + 1) * v
    : h(i + 1, j + 1) * (u + v - 1) +
        h(i, j + 1) * (1 - u) +
        h(i + 1, j) * (1 - v);
}

export function meshBulkWater(m, p) {
  if (!m.waterKinds) return false;
  const b = meshBounds(m),
    x = Math.max(
      0,
      Math.min(m.columns - 1, ((p[0] - b.x) / b.w) * (m.columns - 1)),
    ),
    y = Math.max(0, Math.min(m.rows - 1, ((p[1] - b.y) / b.h) * (m.rows - 1)));
  const i = Math.min(m.columns - 2, Math.floor(x)),
    j = Math.min(m.rows - 2, Math.floor(y)),
    u = x - i,
    v = y - j;
  const weights =
    u + v <= 1
      ? [
          [i, j, 1 - u - v],
          [i + 1, j, u],
          [i, j + 1, v],
        ]
      : [
          [i + 1, j + 1, u + v - 1],
          [i, j + 1, 1 - u],
          [i + 1, j, 1 - v],
        ];
  return (
    weights.reduce(
      (sum, [a, b, w]) =>
        sum + ([1, 2].includes(m.waterKinds[b * m.columns + a]) ? w : 0),
      0,
    ) >= 0.5
  );
}

// Break at every mesh edge (including diagonals). Each resulting line segment
// lies in exactly one terrain triangle, so its side view follows the terrain.
export function drapeSegment(m, a, b) {
  const bounds = meshBounds(m),
    clipped = clipSegment(a, b, bounds);
  if (!clipped) return [];
  [a, b] = clipped;
  const x = ((a[0] - bounds.x) / bounds.w) * (m.columns - 1),
    y = ((a[1] - bounds.y) / bounds.h) * (m.rows - 1),
    dx = ((b[0] - a[0]) / bounds.w) * (m.columns - 1),
    dy = ((b[1] - a[1]) / bounds.h) * (m.rows - 1),
    ts = [0, 1];
  for (const [start, delta] of [
    [x, dx],
    [y, dy],
    [x + y, dx + dy],
  ]) {
    if (Math.abs(delta) < 1e-12) continue;
    for (
      let k = Math.ceil(Math.min(start, start + delta));
      k <= Math.floor(Math.max(start, start + delta));
      k++
    ) {
      const t = (k - start) / delta;
      if (t > 0 && t < 1) ts.push(t);
    }
  }
  return [...new Set(ts)]
    .sort((a, b) => a - b)
    .map((t) => {
      const p = [a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t];
      return [...p, meshHeight(m, p)];
    });
}

function cutPolygon(points, a, b, limit) {
  const result = [];
  for (let i = 0, j = points.length - 1; i < points.length; j = i++) {
    const p = points[j],
      q = points[i],
      d = p[0] * a + p[1] * b - limit,
      e = q[0] * a + q[1] * b - limit;
    if (d <= 0 !== e <= 0) {
      const t = d / (d - e);
      result.push([p[0] + (q[0] - p[0]) * t, p[1] + (q[1] - p[1]) * t]);
    }
    if (e <= 0) result.push(q);
  }
  return result;
}

// Intersect the actual polygon with each terrain triangle. A flat polygon mesh
// would float above valleys; sampling only its corners would cut through ridges.
export function drapePolygon(m, points) {
  const b = meshBounds(m),
    cols = m.columns - 1,
    rows = m.rows - 1;
  const path = points.map(
    (p) =>
      new Vector2(((p[0] - b.x) / b.w) * cols, ((p[1] - b.y) / b.h) * rows),
  );
  const triangles = ShapeUtils.triangulateShape(path, []),
    result = [];
  for (const ids of triangles) {
    const triangle = ids.map((i) => [path[i].x, path[i].y]);
    const x0 = Math.max(0, Math.floor(Math.min(...triangle.map((p) => p[0])))),
      x1 = Math.min(
        cols - 1,
        Math.floor(Math.max(...triangle.map((p) => p[0]))),
      );
    const y0 = Math.max(0, Math.floor(Math.min(...triangle.map((p) => p[1])))),
      y1 = Math.min(
        rows - 1,
        Math.floor(Math.max(...triangle.map((p) => p[1]))),
      );
    for (let y = y0; y <= y1; y++)
      for (let x = x0; x <= x1; x++) {
        let cell = triangle;
        for (const [a, c, d] of [
          [-1, 0, -x],
          [1, 0, x + 1],
          [0, -1, -y],
          [0, 1, y + 1],
        ])
          cell = cutPolygon(cell, a, c, d);
        if (cell.length < 3) continue;
        for (const poly of [
          cutPolygon(cell, 1, 1, x + y + 1),
          cutPolygon(cell, -1, -1, -x - y - 1),
        ]) {
          const vertices = poly.map((p) => {
            const q = [b.x + (p[0] / cols) * b.w, b.y + (p[1] / rows) * b.h];
            return [...q, meshHeight(m, q)];
          });
          for (let i = 1; i + 1 < vertices.length; i++)
            result.push(vertices[0], vertices[i], vertices[i + 1]);
        }
      }
  }
  return result;
}
