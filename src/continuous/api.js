export async function request(
  path,
  body,
  method = body === undefined ? "GET" : "POST",
  signal,
) {
  const response = await fetch(`/api/${path}`, {
    method,
    signal,
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (response.status === 204) return;
  const result = await response.json();
  if (!response.ok) {
    const error = new Error(
      result.error || `Request failed (${response.status})`,
    );
    error.status = response.status;
    throw error;
  }
  return result;
}
export const defaults = {
  version: "continuous-3",
  reliefVersion: 2,
  climateVersion: 1,
  seed: "",
  preset: "earthlike",
  radiusKm: 6371,
  landCoverage: 0.32,
  continents: 5,
  complexity: 1,
  islands: 1,
  relief: 0.8,
  rainfall: 1.65,
  tilt: 23.4,
  layoutVariant: 0,
  tileSize: 256,
  surfaceWidth: 4096,
};
export const presets = {
  earthlike: {
    landCoverage: 0.32,
    continents: 5,
    islands: 1,
    relief: 0.8,
    rainfall: 1.65,
  },
  archipelago: {
    landCoverage: 0.18,
    continents: 6,
    islands: 2,
    relief: 0.8,
    rainfall: 1.2,
  },
  rugged: {
    landCoverage: 0.35,
    continents: 5,
    islands: 1.2,
    relief: 1.6,
    rainfall: 1,
  },
  supercontinent: {
    landCoverage: 0.4,
    continents: 1,
    islands: 0.7,
    relief: 1.2,
    rainfall: 1,
  },
  ocean: {
    landCoverage: 0.12,
    continents: 4,
    islands: 1.7,
    relief: 0.8,
    rainfall: 1,
  },
  arid: {
    landCoverage: 0.35,
    continents: 5,
    islands: 0.5,
    relief: 1,
    rainfall: 0.4,
  },
};
