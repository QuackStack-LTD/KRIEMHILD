// Split at the map seam with a shared interpolated boundary point on each
// side. Dropping the crossing edge leaves a real gap at close zoom.
export function geographicParts(path) {
  const parts = [[]];
  for (const point of path) {
    const q = [(point[0] + Math.PI) / Math.PI, 0.5 - point[1] / Math.PI];
    const a = parts.at(-1).at(-1);
    if (a && Math.abs(q[0] - a[0]) > 1) {
      const unwrapped = q[0] + (q[0] < a[0] ? 2 : -2);
      if (Math.abs(unwrapped - a[0]) < 1e-14) {
        parts.at(-1).push([a[0], q[1]]);
        parts.push([q]);
        continue;
      }
      const edge = unwrapped > a[0] ? 2 : 0;
      const t = (edge - a[0]) / (unwrapped - a[0]);
      const y = a[1] + (q[1] - a[1]) * t;
      parts.at(-1).push([edge, y]);
      parts.push([[2 - edge, y]]);
    }
    parts.at(-1).push(q);
  }
  return parts.filter((p) => p.length > 1);
}

export const pathText = (points) =>
  points.map((p, i) => `${i ? "L" : "M"}${p[0]},${p[1]}`).join(" ");
