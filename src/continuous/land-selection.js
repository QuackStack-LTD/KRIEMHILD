import { contains, bounds } from "./geometry.js";
import { clipPolygon, clipLine } from "./clip.js";
import { simplifyPath } from "./cartography.js";

export function landPaths(paths, tolerance = 0) {
  return paths.flatMap((ring) => {
    const unwrapped = [];
    for (const p of ring) {
      let x = p[0];
      if (unwrapped.length) {
        const last = unwrapped.at(-1)[0];
        while (x - last > 1) x -= 2;
        while (x - last < -1) x += 2;
      }
      unwrapped.push([x, p[1]]);
    }
    return [-2, 0, 2].flatMap((shift) => {
      const part = clipPolygon(
        unwrapped.map((p) => [p[0] + shift, p[1]]),
        { x: 0, y: 0, w: 2, h: 1 },
      );
      return part.length < 3
        ? []
        : [simplifyPath([...part, part[0]], tolerance)];
    });
  });
}
export function landContains(component, point) {
  return component.paths.reduce(
    (inside, ring) => (contains(point, ring) ? !inside : inside),
    false,
  );
}
export function toggleLand(components, selected, point) {
  const hit = components.find((c) => landContains(c, point));
  if (!hit) return selected;
  return selected.includes(hit.id)
    ? selected.filter((id) => id !== hit.id)
    : [...selected, hit.id];
}
export function rectangleLand(components, selected, rect) {
  const corners = [
    [rect.x, rect.y],
    [rect.x + rect.w, rect.y],
    [rect.x, rect.y + rect.h],
    [rect.x + rect.w, rect.y + rect.h],
  ];
  return [
    ...new Set([
      ...selected,
      ...components
        .filter(
          (c) =>
            corners.some((p) => landContains(c, p)) ||
            c.paths.some((r) => clipLine(r, rect).length > 0),
        )
        .map((c) => c.id),
    ]),
  ];
}
export function selectedLand(components, ids) {
  const paths = components
    .filter((c) => ids.includes(c.id))
    .flatMap((c) => c.paths);
  if (!paths.length) return null;
  const b = bounds(paths.flat()),
    pad = Math.max(b.w, b.h) * 0.035;
  return {
    regions: paths,
    bounds: {
      x: b.x - pad,
      y: b.y - pad,
      w: Math.max(0.0001, b.w + 2 * pad),
      h: Math.max(0.0001, b.h + 2 * pad),
    },
  };
}
