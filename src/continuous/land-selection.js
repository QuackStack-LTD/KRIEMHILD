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
  let paths = components
    .filter((c) => ids.includes(c.id))
    .flatMap((c) => c.paths);
  if (!paths.length) return null;
  // Put the map seam in the largest unoccupied longitude interval. Move whole
  // rings by a full turn only: relative geography and source coordinates stay
  // intact, while the two fragments of a seam-crossing continent meet again.
  const intervals = paths
    .map((p) => {
      const b = bounds(p);
      return [b.x, b.x + b.w];
    })
    .sort((a, b) => a[0] - b[0]);
  const merged = [];
  for (const interval of intervals) {
    if (merged.length && interval[0] <= merged.at(-1)[1])
      merged.at(-1)[1] = Math.max(merged.at(-1)[1], interval[1]);
    else merged.push([...interval]);
  }
  let gap = merged[0][0] + 2 - merged.at(-1)[1],
    origin = 0;
  for (let i = 1; i < merged.length; i++) {
    const width = merged[i][0] - merged[i - 1][1];
    if (width > gap) {
      gap = width;
      origin = merged[i][0];
    }
  }
  if (origin)
    paths = paths.map((ring) =>
      ring.map(([x, y]) => [x < origin - 1e-10 ? x + 2 : x, y]),
    );
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

// The selection still excludes lakes for geographic hit-testing. Illustration
// coverage includes enclosed water, so the terrain sampler can paint the actual
// lake and its shore instead of revealing blank paper through the land mask.
export function illustrationFootprint(regions) {
  // A nonzero fill of equally wound rings is their union. This fills enclosed
  // lakes and avoids XOR holes where closed seam fragments slightly overlap.
  // Keep every vertex: thin seam strips can contain genuine shoreline detail.
  return regions.map((ring) => {
    let area = 0;
    for (let i = 0, j = ring.length - 1; i < ring.length; j = i++)
      area += ring[j][0] * ring[i][1] - ring[i][0] * ring[j][1];
    return area < 0 ? [...ring].reverse() : ring;
  });
}

// Every feature uses the same longitude branch as the fitted illustration.
// Unwrap a path continuously before translating the whole path, never each
// vertex independently (which would stretch a polygon across the map).
export function illustrationPoints(points, config) {
  if (!config.regions?.length || !points.length) return points;
  const unwrapped = [];
  for (const [longitude, y] of points) {
    let x = longitude;
    if (unwrapped.length) x += 2 * Math.round((unwrapped.at(-1)[0] - x) / 2);
    unwrapped.push([x, y]);
  }
  const b = bounds(unwrapped),
    center = config.bounds[0] + config.bounds[2] / 2,
    shift = 2 * Math.round((center - b.x - b.w / 2) / 2);
  return unwrapped.map(([x, y]) => [x + shift, y]);
}

// Sample each side through the canonical API, so local authored terrain edits
// are included by the server's spatial index as well as the base sampler.
export function illustrationMeshBounds(config) {
  const [x, y, w, h] = config.bounds;
  const south = Math.max(-90, 90 - (y + h) * 180),
    north = Math.min(90, 90 - y * 180);
  if (w >= 2) return [[-180, south, 180, north]];
  if (!config.regions?.length)
    return [
      [
        Math.max(-180, x * 180 - 180),
        south,
        Math.min(180, (x + w) * 180 - 180),
        north,
      ],
    ];
  const result = [];
  for (let turn = Math.floor(x / 2); turn <= Math.floor((x + w) / 2); turn++) {
    const left = Math.max(x, turn * 2),
      right = Math.min(x + w, (turn + 1) * 2);
    if (right > left)
      result.push([
        (left - turn * 2) * 180 - 180,
        south,
        (right - turn * 2) * 180 - 180,
        north,
      ]);
  }
  return result;
}
