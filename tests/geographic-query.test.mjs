import test from "node:test";
import assert from "node:assert/strict";
import {
  GeographicQuery,
  mapGeography,
} from "../src/continuous/geographic-query.js";
import { screenToMap, mapToScreen } from "../src/continuous/geometry.js";

test("geographic queries wrap longitudes, include poles and reject polar margins", () => {
  for (const x of [-4, 0, 2, 8])
    assert.equal(mapGeography([x, 0]).longitude, -180);
  assert.deepEqual(mapGeography([1, 1]), { longitude: 0, latitude: -90 });
  assert.equal(mapGeography([0, -0.001]), null);
  assert.equal(mapGeography([NaN, 0]), null);
  assert.equal(mapGeography([]), null);
  assert.equal(mapGeography("hi"), null);
  for (const camera of [
    { x: 0, y: 0, w: 2, h: 1 },
    { x: 1.3, y: 0.4, w: 0.001, h: 0.0005 },
  ]) {
    for (const rect of [
      { left: 40, top: 30, width: 1000, height: 900 },
      { left: 0, top: 0, width: 1200, height: 400 },
    ]) {
      const point = [camera.x + camera.w * 0.25, camera.y + camera.h * 0.75];
      const screen = mapToScreen(point, rect, camera);
      const result = screenToMap(...screen, rect, camera);
      assert.ok(Math.hypot(result[0] - point[0], result[1] - point[1]) < 1e-12);
    }
  }
});
test("new clicks and source revisions supersede responses even if transport ignores abort", async () => {
  const pending = [];
  const service = new GeographicQuery({
    fetchSample: (...args) =>
      new Promise((resolve) => pending.push({ resolve, args })),
  });
  const results = [];
  const options = {
    session: "s",
    identity: "world",
    revision: 1,
    point: [1, 0.5],
  };
  const first = service.select(options, (r) => results.push(r));
  const second = service.select({ ...options, point: [1.1, 0.5] }, (r) =>
    results.push(r),
  );
  pending[1].resolve({ biome: "forest", revision: 1 });
  await second;
  pending[0].resolve({ biome: "desert", revision: 1 });
  await first;
  assert.deepEqual(
    results.map((r) => r.biome),
    ["forest"],
  );
  const third = service.select({ ...options, point: [1.2, 0.5] }, (r) =>
    results.push(r),
  );
  service.source("world", 2);
  pending[2].resolve({ biome: "ice", revision: 1 });
  await third;
  assert.equal(results.length, 1);
  assert.equal(pending[0].args[3].aborted, true);
});
test("query cache is revision keyed and bounded; cached selections still supersede in-flight work", async () => {
  let calls = 0;
  const service = new GeographicQuery({
    limit: 2,
    budget: 1000,
    fetchSample: async () => {
      calls++;
      return { biome: "forest" };
    },
  });
  const options = { session: "s", identity: "w", revision: 1, point: [1, 0.5] };
  await service.select(options, () => {});
  const cached = await service.select(options, () => {});
  assert.equal(cached.query.cache, "hit");
  assert.equal(calls, 1);
  await service.select({ ...options, revision: 2 }, () => {});
  assert.equal(calls, 2);
  for (let i = 0; i < 10; i++)
    await service.select({ ...options, point: [i / 10, 0.5] }, () => {});
  assert.ok(service.cache.size <= 2 && service.bytes <= 1000);
});
