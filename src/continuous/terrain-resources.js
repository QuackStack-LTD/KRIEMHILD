const tau = 2 * Math.PI;
const wrap = (x) => x - 2 * Math.floor((x + 1) / 2);
export function resourceDistance(a, b, planetRadius) {
  const latA = (0.5 - a[1]) * Math.PI,
    latB = (0.5 - b[1]) * Math.PI;
  const h =
    Math.sin((latB - latA) / 2) ** 2 +
    Math.cos(latA) *
      Math.cos(latB) *
      Math.sin((wrap(b[0] - a[0]) * Math.PI) / 2) ** 2;
  return 2 * planetRadius * Math.asin(Math.sqrt(Math.max(0, Math.min(1, h))));
}
// Return an unwrapped ring, continuous across the antimeridian. Callers draw
// shifted copies and clip in map coordinates, just like the terrain itself.
export function resourceRing(center, radiusKm, planetRadius, count = 128) {
  const lat = (0.5 - center[1]) * Math.PI,
    lon = (center[0] - 1) * Math.PI,
    d = radiusKm / planetRadius;
  let previous = center[0];
  return Array.from({ length: count + 1 }, (_, i) => {
    const bearing = (i / count) * tau;
    const p = Math.asin(
      Math.max(
        -1,
        Math.min(
          1,
          Math.sin(lat) * Math.cos(d) +
            Math.cos(lat) * Math.sin(d) * Math.cos(bearing),
        ),
      ),
    );
    const l =
      lon +
      Math.atan2(
        Math.sin(bearing) * Math.sin(d) * Math.cos(lat),
        Math.cos(d) - Math.sin(lat) * Math.sin(p),
      );
    let x = l / Math.PI + 1;
    x += 2 * Math.round((previous - x) / 2);
    previous = x;
    return [x, 0.5 - p / Math.PI];
  });
}
export const resourceQuery = (center, radiusKm) =>
  new URLSearchParams({
    longitude: ((((center[0] * 180 - 180 + 180) % 360) + 360) % 360) - 180,
    latitude: Math.max(-90, Math.min(90, 90 - center[1] * 180)),
    radiusKm,
  }).toString();
