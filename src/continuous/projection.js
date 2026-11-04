import { visibleViewport } from "./camera.js";

// Subtract the geographic origin in double precision before SVG receives any
// coordinates. SVG stores viewBox/path numbers as floats, which lose metres at
// editing zoom when absolute world coordinates are magnified.
export function mapProjection(camera, width, height) {
  const bounds = visibleViewport(camera, width, height);
  const scale = width / bounds.w;
  const point = (p) => [(p[0] - bounds.x) * scale, (p[1] - bounds.y) * scale];
  const box = (b) => {
    const [x, y] = point([b.x, b.y]);
    return { x, y, w: b.w * scale, h: b.h * scale };
  };
  return {
    bounds,
    width,
    height,
    scale,
    point,
    box,
    inverse: (p) => [bounds.x + p[0] / scale, bounds.y + p[1] / scale],
    points: (values) =>
      values
        .map(point)
        .map((p) => p.join(","))
        .join(" "),
  };
}

export function rasterCrop(frame, projection) {
  const a = frame.bounds,
    b = projection.bounds;
  const x = Math.max(a.x, b.x),
    y = Math.max(a.y, b.y);
  const w = Math.min(a.x + a.w, b.x + b.w) - x,
    h = Math.min(a.y + a.h, b.y + b.h) - y;
  if (w <= 0 || h <= 0) return null;
  const source = {
    x: ((x - a.x) / a.w) * frame.pixels.width,
    y: ((y - a.y) / a.h) * frame.pixels.height,
    w: (w / a.w) * frame.pixels.width,
    h: (h / a.h) * frame.pixels.height,
  };
  return { source, destination: projection.box({ x, y, w, h }) };
}
