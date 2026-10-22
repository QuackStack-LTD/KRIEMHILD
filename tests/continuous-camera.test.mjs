import test from "node:test";
import assert from "node:assert/strict";
import { normalizeCamera, visibleTiles } from "../src/continuous/camera.js";
import { screenToMap } from "../src/continuous/geometry.js";

test("every visible SVG pixel is covered, including letterboxed margins", () => {
  const camera = {
    x: 0.7620343104171656,
    y: 0.3127600712541041,
    w: 0.18252779440611827,
    h: 0.09126389720305914,
  };
  for (const [width, height] of [
    [892, 695],
    [1600, 500],
  ]) {
    const tiles = visibleTiles(camera, width, 1, height);
    for (let y = 1; y < height; y += 17)
      for (let x = 1; x < width; x += 17) {
        const p = screenToMap(x, y, { left: 0, top: 0, width, height }, camera);
        assert.ok(
          tiles.some(
            (t) =>
              p[0] >= t.x * t.span &&
              p[0] < (t.x + 1) * t.span &&
              p[1] >= t.y * t.span &&
              p[1] < (t.y + 1) * t.span,
          ),
          `unrequested visible geography at ${p}`,
        );
      }
  }
});

test("viewport right and bottom edges are exclusive", () => {
  const tiles = visibleTiles({ x: 0.5, y: 0.25, w: 0.5, h: 0.25 }, 512);
  assert.equal(tiles.length, 2);
  assert.ok(tiles.every((t) => t.x * t.span < 1 && t.y * t.span < 0.5));
});

test("visible tiles stay inside the finite world at all four map boundaries", () => {
  for (const [x, y] of [
    [-0.05, -0.05],
    [1.95, 0.95],
  ]) {
    const tiles = visibleTiles({ x, y, w: 0.1, h: 0.05 }, 600, 1, 600);
    assert.ok(tiles.length > 0);
    assert.ok(
      tiles.every(
        (t) => t.x >= 0 && t.x < 2 ** (t.z + 1) && t.y >= 0 && t.y < 2 ** t.z,
      ),
    );
  }
});

test("invalid imported camera recovers a finite world viewport", () => {
  for (const value of [
    null,
    {},
    { x: 0, y: 0, w: 0, h: 1 },
    { x: Infinity, y: 0, w: 2, h: 1 },
  ])
    assert.deepEqual(normalizeCamera(value), { x: 0, y: 0, w: 2, h: 1 });
});
test("tile density follows viewport and DPR at world and settlement scales", () => {
  for (const w of [2, 0.5, 0.02, 0.00008])
    for (const dpr of [1, 2]) {
      const camera = { x: 0.5, y: 0.3, w, h: w / 2 },
        tiles = visibleTiles(camera, 1200, dpr);
      assert.ok(tiles.length > 0 && tiles.length <= 100);
      for (const tile of tiles) {
        assert.ok((tile.span / w) * 1200 * dpr <= tile.size + 0.00001);
        assert.ok(tile.x >= 0 && tile.x < 2 ** (tile.z + 1));
        assert.ok(tile.y >= 0 && tile.y < 2 ** tile.z);
      }
    }
});
