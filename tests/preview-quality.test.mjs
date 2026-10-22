import test from "node:test";
import assert from "node:assert/strict";
import {
  previewRequest,
  normalizePreviewQuality,
  readPreviewQuality,
  savePreviewQuality,
} from "../src/continuous/preview-quality.js";
import {
  viewportRequest,
  pyramidNodes,
  TerrainPyramid,
} from "../src/continuous/terrain-pyramid.js";
import { screenToMap } from "../src/continuous/geometry.js";

const options = {
  camera: { x: 1.3, y: 0.22, w: 0.2, h: 0.1 },
  width: 1200,
  height: 800,
};
test("preview presets change output and sampling demand, independently of screen DPR and camera", () => {
  const camera = structuredClone(options.camera);
  const rect = { left: 0, top: 0, width: 1200, height: 800 };
  const anchor = screenToMap(500, 300, rect, camera);
  for (const dpr of [1, 2, 3]) {
    for (const [preset, scale] of [
      ["performance", 0.5],
      ["balanced", 0.75],
      ["high", 1],
    ]) {
      const r = previewRequest({ ...options, dpr }, { preset });
      assert.deepEqual(r.pixels, { width: 1200 * scale, height: 800 * scale });
      assert.deepEqual(r.bounds, viewportRequest(options).bounds);
      assert.deepEqual(screenToMap(500, 300, rect, camera), anchor);
      assert.ok(r.z <= viewportRequest({ ...options, dpr }).z);
    }
  }
  assert.deepEqual(options.camera, camera);
});
test("all zooms and large custom previews respect output and atlas safety budgets", () => {
  for (const w of [2, 0.6, 0.05, 0.003, 0.00008]) {
    const r = previewRequest(
      {
        ...options,
        camera: { x: 1.1, y: 0.2, w, h: w / 2 },
        width: 16000,
        height: 10000,
      },
      { preset: "custom", customScale: 1.5 },
    );
    assert.ok(r.pixels.width * r.pixels.height <= 32 * 1024 * 1024);
    assert.ok(pyramidNodes(r).length * r.size ** 2 <= 64 * 1024 * 1024);
    assert.equal(r.preview.clamped, true);
  }
});
test("preference is user local, validated and never implicitly affects generic/Illustrator/export requests", () => {
  let value;
  const storage = {
    getItem: () => value,
    setItem: (_, v) => {
      value = v;
    },
  };
  const before = viewportRequest({ ...options, dpr: 3 });
  assert.equal(readPreviewQuality(storage).preset, "balanced");
  savePreviewQuality({ preset: "performance", customScale: 99 }, storage);
  assert.deepEqual(readPreviewQuality(storage), {
    preset: "performance",
    customScale: 1.5,
  });
  assert.deepEqual(viewportRequest({ ...options, dpr: 3 }), before);
  assert.equal(normalizePreviewQuality({ preset: "bogus" }).preset, "balanced");
  assert.equal(
    savePreviewQuality(
      {},
      {
        setItem() {
          throw Error("denied");
        },
      },
    ),
    false,
  );
  value = "not JSON";
  assert.equal(readPreviewQuality(storage).preset, "balanced");
});
test("quality does not invalidate reusable authoritative raster nodes", () => {
  const pyramid = new TerrainPyramid();
  const source = { ...options, identity: "w/s", revision: 4 };
  const a = pyramid.prepare(previewRequest(source, { preset: "balanced" }));
  const b = pyramid.prepare(previewRequest(source, { preset: "high" }));
  const node = pyramidNodes(a)[0];
  assert.equal(
    pyramid.key(a, node, "natural"),
    pyramid.key(b, node, "natural"),
  );
});
