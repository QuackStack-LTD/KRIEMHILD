export function intersects(a, b) {
  return (
    a.x <= b.x + b.w && a.x + a.w >= b.x && a.y <= b.y + b.h && a.y + a.h >= b.y
  );
}

// Clip only the displayed representation. Shared endpoints are interpolated on
// the original segment; clipping never creates a source/terminal in the graph.
export function clipLine(points, bounds) {
  const out = [];
  let part = [];
  for (let i = 1; i < points.length; i++) {
    const a = points[i - 1],
      b = points[i],
      dx = b[0] - a[0],
      dy = b[1] - a[1];
    let lo = 0,
      hi = 1,
      valid = true;
    for (const [p, q] of [
      [-dx, a[0] - bounds.x],
      [dx, bounds.x + bounds.w - a[0]],
      [-dy, a[1] - bounds.y],
      [dy, bounds.y + bounds.h - a[1]],
    ]) {
      if (p === 0) {
        if (q < 0) valid = false;
        continue;
      }
      const r = q / p;
      if (p < 0) lo = Math.max(lo, r);
      else hi = Math.min(hi, r);
    }
    if (!valid || lo > hi) {
      if (part.length > 1) out.push(part);
      part = [];
      continue;
    }
    const p = [a[0] + dx * lo, a[1] + dy * lo],
      q = [a[0] + dx * hi, a[1] + dy * hi];
    if (
      part.length &&
      Math.hypot(part.at(-1)[0] - p[0], part.at(-1)[1] - p[1]) > 1e-12
    ) {
      out.push(part);
      part = [];
    }
    if (!part.length) part.push(p);
    part.push(q);
    if (hi < 1) {
      out.push(part);
      part = [];
    }
  }
  if (part.length > 1) out.push(part);
  return out;
}

export function clipPolygon(points, bounds) {
  let result = points;
  for (const [axis, edge, sign] of [
    [0, bounds.x, 1],
    [0, bounds.x + bounds.w, -1],
    [1, bounds.y, 1],
    [1, bounds.y + bounds.h, -1],
  ]) {
    const input = result;
    result = [];
    if (!input.length) break;
    for (let i = 0; i < input.length; i++) {
      const a = input[(i + input.length - 1) % input.length],
        b = input[i],
        insideA = (a[axis] - edge) * sign >= 0,
        insideB = (b[axis] - edge) * sign >= 0;
      if (insideA !== insideB) {
        const t = (edge - a[axis]) / (b[axis] - a[axis]);
        result.push([a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t]);
      }
      if (insideB) result.push(b);
    }
  }
  return result;
}
