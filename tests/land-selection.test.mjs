import test from "node:test";
import assert from "node:assert/strict";
import {
  landPaths,
  toggleLand,
  rectangleLand,
  selectedLand,
  landContains,
  illustrationFootprint,
  illustrationPoints,
  illustrationMeshBounds,
} from "../src/continuous/land-selection.js";
import { ExportTiles } from "../src/continuous/illustration-export.js";
const rect = (x, y, w, h) => [
  [x, y],
  [x + w, y],
  [x + w, y + h],
  [x, y + h],
  [x, y],
];
const coverage = (rings, point) =>
  rings.some((ring) => landContains({ paths: [ring] }, point));

test("illustration relief samples both canonical sides so spatially indexed edits remain visible", () => {
  const config = {
    bounds: [1.9, 0.3, 0.2, 0.2],
    regions: [rect(1.95, 0.35, 0.1, 0.1)],
  };
  const parts = illustrationMeshBounds(config);
  assert.equal(parts.length, 2);
  assert.ok(Math.abs(parts[0][0] - 162) < 1e-10);
  assert.equal(parts[0][2], 180);
  assert.equal(parts[1][0], -180);
  assert.ok(Math.abs(parts[1][2] + 162) < 1e-10);
  assert.ok(parts.every((b) => b[0] >= -180 && b[2] <= 180 && b[2] > b[0]));
  assert.deepEqual(
    illustrationMeshBounds({ ...config, bounds: [0, 0, 2.14, 1] }),
    [[-180, -90, 180, 90]],
  );
  assert.equal(
    illustrationMeshBounds({ ...config, bounds: [-0.05, 0.3, 0.1, 0.2] })
      .length,
    2,
  );
});
test("selection toggles one connected land component, including a longitude seam", () => {
  const c = [
    { id: "a", paths: landPaths([rect(0.2, 0.2, 0.2, 0.2)]) },
    { id: "b", paths: landPaths([rect(0.6, 0.2, 0.1, 0.1)]) },
    {
      id: "seam",
      paths: landPaths([
        [
          [1.95, 0.5],
          [0.05, 0.5],
          [0.05, 0.6],
          [1.95, 0.6],
          [1.95, 0.5],
        ],
      ]),
    },
  ];
  assert.deepEqual(toggleLand(c, [], [0.3, 0.3]), ["a"]);
  assert.deepEqual(toggleLand(c, ["a"], [0.3, 0.3]), []);
  assert.equal(landContains(c[2], [1.99, 0.55]), true);
  assert.equal(landContains(c[2], [0.01, 0.55]), true);
  assert.equal(landContains(c[2], [1, 0.55]), false);
  assert.deepEqual(rectangleLand(c, [], { x: 0.15, y: 0.15, w: 0.6, h: 0.3 }), [
    "a",
    "b",
  ]);
  const fit = selectedLand(c, ["a", "b"]);
  assert.ok(fit.bounds.x < 0.2 && fit.bounds.x + fit.bounds.w > 0.7);
  assert.equal(fit.regions.length, 2);
});

test("a seam-crossing continent is fitted once with inland water and nearby islands", () => {
  // The server closes each fragment at the finite preview's edges.
  const components = [
    {
      id: "seam",
      paths: landPaths([
        rect(1.85, 0.3, 0.15, 0.3),
        rect(0, 0.3, 0.15, 0.3),
        rect(0.04, 0.4, 0.04, 0.04),
      ]),
    },
    { id: "island", paths: landPaths([rect(0.2, 0.4, 0.03, 0.03)]) },
  ];
  const before = structuredClone(components);
  const fit = selectedLand(components, ["seam", "island"]);
  assert.ok(fit.bounds.w < 0.42);
  for (const p of [
    [1.9, 0.35],
    [2.1, 0.35],
    [2.21, 0.41],
  ]) {
    assert.equal(landContains({ paths: fit.regions }, p), true);
    assert.ok(p[0] > fit.bounds.x && p[0] < fit.bounds.x + fit.bounds.w);
  }
  const lake = [2.06, 0.42];
  assert.equal(landContains({ paths: fit.regions }, lake), false);
  assert.equal(coverage(illustrationFootprint(fit.regions), lake), true);
  assert.equal(
    coverage(illustrationFootprint(fit.regions), [2.18, 0.42]),
    false,
  );
  const config = { regions: fit.regions, bounds: Object.values(fit.bounds) };
  assert.deepEqual(
    illustrationPoints(
      [
        [1.99, 0.5],
        [0.01, 0.5],
        [0.04, 0.51],
      ],
      config,
    ),
    [
      [1.99, 0.5],
      [2.01, 0.5],
      [2.04, 0.51],
    ],
  );
  assert.deepEqual(components, before);
});

test("inland water coverage includes an enclosed island without punching a second hole", () => {
  const regions = [
    rect(0.2, 0.2, 0.6, 0.6),
    rect(0.3, 0.3, 0.3, 0.3),
    rect(0.4, 0.4, 0.05, 0.05),
  ];
  const footprint = illustrationFootprint(regions);
  assert.equal(coverage(footprint, [0.42, 0.42]), true);
  assert.equal(coverage(footprint, [0.5, 0.5]), true);
  assert.equal(coverage(footprint, [0.1, 0.5]), false);
});

test("illustration coverage unions reversed and overlapping seam rings without losing thin strips", () => {
  const regions = [
    rect(1.9, 0.3, 0.1, 0.3),
    rect(1.999, 0.3, 0.101, 0.3).reverse(),
    rect(1.9995, 0.3, 0.001, 0.3),
  ];
  const before = structuredClone(regions);
  const footprint = illustrationFootprint(regions);
  for (const ring of footprint) {
    const area = ring.reduce((sum, p, i) => {
      const next = ring[(i + 1) % ring.length];
      return sum + p[0] * next[1] - next[0] * p[1];
    }, 0);
    assert.ok(
      area > 0,
      "nonzero fill must union every ring, independent of source winding",
    );
  }
  assert.equal(coverage(footprint, [1.99975, 0.4]), true);
  assert.deepEqual(regions, before);
});
test("a lake hole remains unselected and output geometry does not move land", () => {
  const raw = [rect(0.2, 0.2, 0.5, 0.5), rect(0.3, 0.3, 0.1, 0.1)],
    before = structuredClone(raw),
    c = { paths: landPaths(raw) };
  assert.equal(landContains(c, [0.35, 0.35]), false);
  assert.equal(landContains(c, [0.25, 0.25]), true);
  assert.deepEqual(raw, before);
});
test("export cache reuses immutable rasters and stays within its own memory budget", async () => {
  const cache = new ExportTiles(20);
  let calls = 0;
  const load = async () => {
    calls++;
    return "12345";
  };
  await cache.get("terrain/a", load);
  await cache.get("terrain/a", load);
  assert.equal(calls, 1);
  await cache.get("terrain/b", load);
  await cache.get("terrain/c", load);
  assert.ok(cache.bytes <= 20);
  assert.ok(!cache.cache.has("terrain/a"));
});
