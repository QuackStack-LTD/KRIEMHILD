import test from "node:test";
import assert from "node:assert/strict";
import {
  CartographicGeometry,
  cartographicLevel,
  simplifyPath,
} from "../src/continuous/cartography.js";
import { geographicParts } from "../src/continuous/geographic-path.js";
import { FrameNavigator } from "../src/continuous/frame-navigator.js";
import {
  buildIllustration,
  newIllustration,
} from "../src/continuous/illustration.js";
import {
  savePreviewQuality,
  previewRequest,
} from "../src/continuous/preview-quality.js";

test("presentation levels are independent of preview pixels, with stable transitions", () => {
  assert.deepEqual(
    [2, 0.4, 0.05, 0.005].map((w) => cartographicLevel(w)),
    [0, 1, 2, 3],
  );
  assert.equal(cartographicLevel(0.79, 0), 0);
  assert.equal(cartographicLevel(0.69, 0), 1);
  assert.equal(cartographicLevel(0.81, 1), 1);
  assert.equal(cartographicLevel(0.91, 1), 0);
});
test("seam crossing edges include both interpolated boundary vertices at every zoom", () => {
  const parts = geographicParts([
    [Math.PI - 0.01, 0.2],
    [-Math.PI + 0.01, 0.1],
  ]);
  assert.equal(parts.length, 2);
  assert.equal(parts[0].at(-1)[0], 2);
  assert.equal(parts[1][0][0], 0);
  assert.equal(parts[0].at(-1)[1], parts[1][0][1]);
  assert.ok(parts.every((p) => p.length === 2));
  assert.ok(
    geographicParts([
      [Math.PI, 0],
      [-Math.PI, 0.1],
      [-3, 0.2],
    ])
      .flat(2)
      .every(Number.isFinite),
  );
});
test("all detail levels retain confluences, mouths and downstream visibility without tile-local changes", () => {
  const main = {
    id: "main",
    class: "ephemeral",
    destination: "ocean",
    path: [
      [0, 0.4],
      [0.1, 0.3],
      [0.2, 0.2],
      [0.3, 0.1],
    ],
  };
  const tributary = {
    id: "tributary",
    class: "major",
    destination: "main",
    path: [
      [-0.1, 0.4],
      [0, 0.35],
      [0.1, 0.3],
    ],
  };
  const rivers = [main, tributary],
    before = structuredClone(rivers);
  const cache = new CartographicGeometry({ budget: 4000, limit: 2 });
  const empty = cache.derive({
    identity: "w",
    revision: 1,
    rivers: [],
    level: 0,
  });
  assert.equal(empty.rivers.length, 0);
  for (let level = 0; level < 4; level++) {
    const r = cache.derive({ identity: "w", revision: 1, rivers, level });
    const trunk = r.rivers.find((f) => f.sourceId === "main");
    const end = r.rivers.find((f) => f.sourceId === "tributary").points.at(-1);
    assert.ok(trunk.points.some((p) => p[0] === end[0] && p[1] === end[1]));
    assert.equal(r.rivers.length, 2);
    assert.deepEqual(trunk.points.at(-1), geographicParts(main.path)[0].at(-1));
  }
  assert.ok(cache.bytes <= 4000 && cache.cache.size <= 2);
  assert.deepEqual(rivers, before);
  const a = cache.derive({ identity: "w", revision: 2, rivers, level: 3 });
  assert.equal(
    a,
    cache.derive({ identity: "w", revision: 2, rivers, level: 3 }),
  );
  assert.notEqual(
    a,
    cache.derive({ identity: "w", revision: 3, rivers, level: 3 }),
  );
});
test("simplification preserves major bends and camera events retain one latest transform", () => {
  assert.deepEqual(
    simplifyPath(
      [
        [0, 0],
        [0.1, 0],
        [0.2, 0],
        [0.2, 1],
      ],
      0.01,
    ),
    [
      [0, 0],
      [0.2, 0],
      [0.2, 1],
    ],
  );
  let scheduled = 0,
    camera = { w: 2 },
    callback;
  const nav = new FrameNavigator(
    () => camera,
    (c) => (camera = c),
    (fn) => {
      scheduled++;
      callback = fn;
      return 1;
    },
    () => {},
  );
  for (let n = 0; n < 100; n++) nav.transform((c) => ({ w: c.w * 0.99 }));
  assert.equal(scheduled, 1);
  assert.equal(camera.w, 2);
  callback();
  assert.ok(Math.abs(camera.w - 2 * 0.99 ** 100) < 1e-12);
  nav.transform(() => ({ w: 10 }));
  nav.cancel();
  assert.equal(nav.pending, null);
});

test("default frame scheduling preserves the native Window receiver", () => {
  const prior = globalThis.requestAnimationFrame;
  globalThis.requestAnimationFrame = function () {
    assert.equal(this, globalThis);
    return 1;
  };
  try {
    const nav = new FrameNavigator(
      () => ({ w: 1 }),
      () => {},
    );
    nav.transform((c) => c);
  } finally {
    if (prior) globalThis.requestAnimationFrame = prior;
    else delete globalThis.requestAnimationFrame;
  }
});
test("preview changes preserve exact authored placement, source data and Illustrator export objects", () => {
  const state = { W: 257, H: 129, continuous: { spec: { radiusKm: 6371 } } };
  const entity = {
    id: "town",
    name: "Town",
    kind: "town",
    geometry: "point",
    points: [[123.456, 65.432]],
    minDetail: 0,
  };
  const config = newIllustration("world", { x: 0, y: 0, w: 2, h: 1 });
  const original = structuredClone({ state, entity, config });
  const output = JSON.stringify(
    buildIllustration(config, [entity], [], null, state),
  );
  const storage = { setItem() {} };
  for (const preset of ["performance", "balanced", "high", "custom"]) {
    savePreviewQuality({ preset, customScale: 1.5 }, storage);
    previewRequest(
      { camera: { x: 0, y: 0, w: 2, h: 1 }, width: 1000, height: 500 },
      { preset, customScale: 1.5 },
    );
    assert.equal(
      JSON.stringify(buildIllustration(config, [entity], [], null, state)),
      output,
    );
    assert.deepEqual({ state, entity, config }, original);
  }
});
