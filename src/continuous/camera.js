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
export function visibleTiles(camera, width, dpr = 1) {
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
    y <= Math.min(2 ** z - 1, Math.floor((camera.y + camera.h) / span));
    y++
  )
    for (
      let x = Math.max(0, Math.floor(camera.x / span));
      x <= Math.min(2 ** (z + 1) - 1, Math.floor((camera.x + camera.w) / span));
      x++
    )
      tiles.push({ z, x, y, span, size });
  return tiles;
}
