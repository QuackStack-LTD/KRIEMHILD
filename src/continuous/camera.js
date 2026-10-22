export function normalizeCamera(value) {
  if (
    !value ||
    !["x", "y", "w", "h"].every((k) => Number.isFinite(value[k])) ||
    value.w < 0.00008 ||
    value.w > 3 ||
    value.h <= 0 ||
    value.h > 3
  )
    return { x: 0, y: 0, w: 2, h: 1 };
  return {
    x: Math.max(-1, Math.min(2, value.x)),
    y: Math.max(-1, Math.min(1, value.y)),
    w: value.w,
    h: value.h,
  };
}
// SVG xMidYMid meet paints beyond the viewBox into its letterboxed margins.
// Request those pixels too, using exactly the scale used for entity placement.
export function visibleViewport(
  camera,
  width,
  height = (width * camera.h) / camera.w,
) {
  const scale = Math.min(width / camera.w, height / camera.h);
  const w = width / scale,
    h = height / scale;
  return {
    x: camera.x - (w - camera.w) / 2,
    y: camera.y - (h - camera.h) / 2,
    w,
    h,
  };
}
export function visibleTiles(camera, width, dpr = 1, height) {
  camera = visibleViewport(camera, width, height);
  const size = width * dpr > 1536 ? 512 : 256;
  const z = Math.max(
    0,
    Math.min(
      18,
      Math.ceil(Math.log2(Math.max(1, (width * dpr) / camera.w / size))),
    ),
  );
  const span = 1 / 2 ** z,
    tiles = [];
  for (
    let y = Math.max(0, Math.floor(camera.y / span));
    y <= Math.min(2 ** z - 1, Math.ceil((camera.y + camera.h) / span) - 1);
    y++
  )
    for (
      let x = Math.max(0, Math.floor(camera.x / span));
      x <=
      Math.min(2 ** (z + 1) - 1, Math.ceil((camera.x + camera.w) / span) - 1);
      x++
    )
      tiles.push({ z, x, y, span, size });
  return tiles;
}
