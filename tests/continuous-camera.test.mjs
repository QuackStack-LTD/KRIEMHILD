import test from "node:test";
import assert from "node:assert/strict";
import { normalizeCamera, visibleTiles } from "../src/continuous/camera.js";

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
