import test from "node:test";
import assert from "node:assert/strict";
import { mapProjection, rasterCrop } from "../src/continuous/projection.js";
import { clipLine, clipPolygon } from "../src/continuous/clip.js";
import { CartographicGeometry } from "../src/continuous/cartography.js";
import { meshBulkWater } from "../src/continuous/terrain-drape.js";

test("raster, geographic features and inspection share a stable deep-zoom transform", () => {
  const p = [1.1797963305036703, 0.5818945745133671];
  for (const w of [2, 0.1, 0.01, 0.001, 0.00008]) {
    const projection = mapProjection(
      { x: p[0] - w * 0.61, y: p[1] - w * 0.19, w, h: w / 2 },
      1200,
      600,
    );
    const pixel = projection.point(p);
    assert.ok(Math.abs(pixel[0] - 732) < 1e-7);
    assert.ok(Math.abs(pixel[1] - 228) < 1e-7);
    assert.deepEqual(projection.inverse(pixel), p);
    // An overview held during replacement never becomes an enormous SVG image.
    const crop = rasterCrop(
      {
        bounds: { x: 0, y: 0, w: 2, h: 1 },
        pixels: { width: 1024, height: 512 },
      },
      projection,
    );
    assert.ok(crop.destination.w <= 1200 + 1e-7);
    assert.ok(crop.destination.h <= 600 + 1e-7);
    const imagePixel = [
      crop.destination.x +
        (((p[0] / 2) * 1024 - crop.source.x) / crop.source.w) *
          crop.destination.w,
      crop.destination.y +
        ((p[1] * 512 - crop.source.y) / crop.source.h) * crop.destination.h,
    ];
    assert.ok(Math.hypot(...pixel.map((v, i) => v - imagePixel[i])) < 1e-7);
  }
});

test("offscreen geography is clipped without moving stored vertices or inventing polygon borders", () => {
  const b = { x: 1, y: 0.5, w: 0.00008, h: 0.00004 };
  const path = [
      [0, 0.50002],
      [2, 0.50002],
    ],
    original = structuredClone(path);
  assert.deepEqual(clipLine(path, b), [
    [
      [1, 0.50002],
      [1.00008, 0.50002],
    ],
  ]);
  const polygon = [
    [0, 0],
    [2, 0],
    [2, 1],
    [0, 1],
  ];
  assert.equal(clipPolygon(polygon, b).length, 4);
  assert.deepEqual(clipLine([...polygon, polygon[0]], b), []);
  assert.deepEqual(path, original);
  assert.equal(
    rasterCrop(
      {
        bounds: { x: 0, y: 0, w: 0.1, h: 0.1 },
        pixels: { width: 256, height: 256 },
      },
      mapProjection(b, 1000, 500),
    ),
    null,
  );
});

test("river simplification cannot move a bend by a visible pixel when zoomed in", () => {
  const river = {
    id: "plain",
    class: "major",
    destination: "ocean",
    path: [
      [0.4, 0.1],
      [0.401, 0.1001],
      [0.402, 0.1],
    ],
  };
  const cache = new CartographicGeometry();
  for (let level = 0; level < 4; level++) {
    const result = cache.derive({
      identity: "test",
      revision: 1,
      rivers: [river],
      level,
      pixel: 1e-7,
    });
    assert.equal(result.rivers[0].points.length, 3);
  }
});

test("3D river visibility distinguishes lakes and ocean from river channels", () => {
  const mesh = {
    bounds: [-Math.PI, -Math.PI / 2, Math.PI, Math.PI / 2],
    columns: 2,
    rows: 2,
    waterKinds: [1, 2, 3, 0],
  };
  assert.equal(meshBulkWater(mesh, [0, 0]), true);
  assert.equal(meshBulkWater(mesh, [2, 0]), true);
  assert.equal(meshBulkWater(mesh, [0, 1]), false);
  assert.equal(meshBulkWater(mesh, [2, 1]), false);
});
