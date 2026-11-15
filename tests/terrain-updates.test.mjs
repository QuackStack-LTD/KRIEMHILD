import test from "node:test";
import assert from "node:assert/strict";
import {
  mergeTerrainEntities,
  terrainRenderRevision,
  patchRiverNames,
} from "../src/continuous/terrain-updates.js";
import {
  resourceArea,
  resourceRadiusForArea,
} from "../src/continuous/terrain-resources.js";
import { TerrainPyramid } from "../src/continuous/terrain-pyramid.js";

test("terrain responses patch drawings, deletions and batches without replacing unchanged features", () => {
  const city = { id: "city" },
    road = { id: "road" },
    house = { id: "house" };
  const current = [city, road];
  assert.equal(mergeTerrainEntities(current, []), current);
  const next = mergeTerrainEntities(current, [
    { id: "city", name: "New" },
    { id: "road", deleted: true },
    house,
  ]);
  assert.deepEqual(
    next.map((e) => e.id),
    ["city", "house"],
  );
  assert.equal(next[1], house);
  assert.equal(mergeTerrainEntities(current, [house])[0], city);
});

test("canonical river name changes reuse paths and leave unchanged river objects intact", () => {
  const a = {
      id: "a",
      path: [
        [0, 0],
        [1, 1],
      ],
      name: "Old name",
      entityId: "old",
    },
    b = {
      id: "b",
      path: [
        [1, 1],
        [2, 2],
      ],
    };
  const current = [a, b],
    names = [{ sourceId: "a", name: "New name", logicalId: "new" }];
  const changed = patchRiverNames(current, names);
  assert.notEqual(changed, current);
  assert.equal(changed[0].path, a.path);
  assert.equal(changed[0].name, "New name");
  assert.equal(changed[0].entityId, "new");
  assert.equal(changed[1], b);
  assert.equal(
    patchRiverNames(changed, names),
    changed,
    "unchanged metadata must not rebuild geometry",
  );
  assert.equal(patchRiverNames(changed, [])[0].name, "");
  assert.equal(patchRiverNames(current, null), current);
});

test("feature-only revisions keep terrain pixels while terrain edits invalidate only intersecting tiles", () => {
  const pyramid = new TerrainPyramid();
  const initial = pyramid.prepare({
    identity: "world",
    revision: 4,
    invalidations: [],
  });
  const node = { z: 2, x: 1, y: 1, span: 0.25, size: 256 };
  const feature = { revision: 5, raster: false, x: 3, y: 2, w: 0, h: 0 };
  const unchanged = pyramid.prepare({
    identity: "world",
    revision: 5,
    invalidations: [feature],
  });
  assert.equal(
    pyramid.key(initial, node, "terrain"),
    pyramid.key(unchanged, node, "terrain"),
  );
  assert.equal(terrainRenderRevision(5, [feature]), 4);
  const edit = {
    revision: 6,
    raster: true,
    x: 0.26,
    y: 0.26,
    w: 0.01,
    h: 0.01,
  };
  const changed = pyramid.prepare({
    identity: "world",
    revision: 6,
    invalidations: [feature, edit],
  });
  assert.notEqual(
    pyramid.key(changed, node, "terrain"),
    pyramid.key(initial, node, "terrain"),
  );
  const far = { ...node, x: 4 };
  assert.equal(
    pyramid.key(changed, far, "terrain"),
    pyramid.key(initial, far, "terrain"),
  );
  assert.equal(
    terrainRenderRevision(7, [feature, edit, { ...feature, revision: 7 }]),
    6,
  );
  assert.equal(
    terrainRenderRevision(8, [feature, edit]),
    8,
    "unknown revisions must refresh",
  );
});

test("observed area matches spherical circles and is invertible from tiny selections to a planet", () => {
  for (const planet of [100, 6371, 100000])
    for (const radius of [
      0.05,
      5,
      50,
      (planet * Math.PI) / 2,
      planet * Math.PI,
    ]) {
      const area = resourceArea(radius, planet);
      assert.ok(Math.abs(resourceRadiusForArea(area, planet) - radius) < 1e-5);
      assert.ok(area >= 0 && area <= 4 * Math.PI * planet ** 2);
    }
  assert.ok(Math.abs(resourceArea(5, 6371) - Math.PI * 25) < 0.00001);
  assert.ok(resourceArea(10, 6371) > resourceArea(5, 6371));
  assert.ok(Number.isNaN(resourceRadiusForArea(-1, 6371)));
});
