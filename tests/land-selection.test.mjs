import test from "node:test";
import assert from "node:assert/strict";
import {
  landPaths,
  toggleLand,
  rectangleLand,
  selectedLand,
  landContains,
} from "../src/continuous/land-selection.js";
import { ExportTiles } from "../src/continuous/illustration-export.js";
const rect = (x, y, w, h) => [
  [x, y],
  [x + w, y],
  [x + w, y + h],
  [x, y + h],
  [x, y],
];
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
