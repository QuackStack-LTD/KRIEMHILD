// The only source is the persisted canonical channel network, in radians / SI.
export function riverFeatures(rivers) {
  return rivers.flatMap((r) => {
    const parts = [[]];
    for (const p of r.path) {
      const q = [(p[0] + Math.PI) / Math.PI, 0.5 - p[1] / Math.PI];
      if (parts.at(-1).length && Math.abs(q[0] - parts.at(-1).at(-1)[0]) > 1)
        parts.push([]);
      parts.at(-1).push(q);
    }
    return parts
      .filter((p) => p.length > 1)
      .map((points, i) => ({
        id: r.id + ":" + i,
        sourceId: r.id,
        name: r.name || r.id,
        kind: "river",
        geometry: "line",
        points,
        river: r,
      }));
  });
}
export function riverWidth(r, pixel, radius) {
  const metres = r.width?.at(-1) || 1,
    q = r.discharge?.at(-1) || 0;
  return Math.max(
    pixel * (0.35 + 0.07 * Math.log1p(q)),
    Math.min(pixel * 7, metres / (Math.PI * radius * 1000)),
  );
}
export function riverVisible(r, scope) {
  const q = r.discharge?.[0] || 0;
  return scope === "world"
    ? q >= 5000 && (!r.order || r.order >= 3)
    : scope === "continent"
      ? q >= 800 && (!r.order || r.order >= 2)
      : true;
}
