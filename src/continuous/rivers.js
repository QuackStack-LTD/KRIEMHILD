// The only source is the persisted canonical channel network, in radians / SI.
import { geographicParts } from "./geographic-path.js";
export function riverFeatures(rivers) {
  return rivers.flatMap((r) => {
    return geographicParts(r.path).map((points, i) => ({
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
  if (r.class)
    return scope === "world"
      ? ["major", "distributary"].includes(r.class)
      : scope === "continent"
        ? ["major", "regional", "distributary"].includes(r.class)
        : true;
  const q = r.discharge?.[0] || 0;
  return scope === "world"
    ? q >= 5000 && (!r.order || r.order >= 3)
    : scope === "continent"
      ? q >= 800 && (!r.order || r.order >= 2)
      : true;
}
