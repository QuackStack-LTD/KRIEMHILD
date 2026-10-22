import test from "node:test";
import assert from "node:assert/strict";
import {
  TerrainPyramid,
  viewportRequest,
  pyramidNodes,
  renderViewport,
} from "../src/continuous/terrain-pyramid.js";
import { screenToMap } from "../src/continuous/geometry.js";
import { reliefResolution, visibleTiles } from "../src/continuous/camera.js";
const camera = {
  x: 0.7620343104171656,
  y: 0.3127600712541041,
  w: 0.18252779440611827,
  h: 0.09126389720305914,
};
const request = (changes) =>
  viewportRequest({
    identity: "world/session",
    session: "session",
    revision: 0,
    invalidations: [],
    camera,
    width: 892,
    height: 695,
    styles: ["natural", "hillshade"],
    generation: 1,
    ...changes,
  });
const signal = () => new AbortController().signal;
function drawing() {
  const calls = [];
  return {
    calls,
    decode: async (blob) => ({ blob, close() {} }),
    createCanvas: (width, height) => ({
      width,
      height,
      getContext: () => ({ drawImage: (...args) => calls.push(args.slice(1)) }),
      convertToBlob: async () => new Blob(["complete"]),
    }),
  };
}

test("manual retry fetches fresh bytes after a corrupt PNG decode", async () => {
  let requests = 0;
  const p = new TerrainPyramid({
    fetchTile: async () => new Blob([++requests === 1 ? "truncated" : "valid"]),
  });
  const r = request({
    camera: { x: 0, y: 0, w: 1, h: 1 },
    width: 256,
    height: 256,
    styles: ["natural"],
  });
  const options = {
    ...drawing(),
    onFrame: async () => {},
    decode: async (blob) => {
      if ((await blob.text()) === "truncated") throw Error("Invalid PNG");
      return { close() {} };
    },
  };
  await assert.rejects(renderViewport(p, r, signal(), options), /Invalid PNG/);
  await renderViewport(p, r, signal(), options);
  assert.equal(requests, 2);
});

test("a fully cached fine viewport bypasses coarse work; unknown edits restore fallback", async () => {
  const p = new TerrainPyramid({ fetchTile: async () => new Blob(["valid"]) });
  const phases = [];
  const options = { ...drawing(), onFrame: async (f) => phases.push(f.phase) };
  await renderViewport(p, request(), signal(), options);
  assert.deepEqual(phases, ["coarse", "final"]);
  phases.length = 0;
  await renderViewport(p, request(), signal(), options);
  assert.deepEqual(phases, ["final"]);
  phases.length = 0;
  await renderViewport(p, request({ revision: 2 }), signal(), options);
  assert.deepEqual(phases, ["coarse", "final"]);
});

test("related styles are fetched together within the shared worker bound", async () => {
  const addresses = [];
  let active = 0,
    max = 0;
  const p = new TerrainPyramid({
    fetchTile: async (url) => {
      active++;
      max = Math.max(max, active);
      addresses.push(url);
      await new Promise((r) => setTimeout(r, 1));
      active--;
      return new Blob(["valid"]);
    },
  });
  await renderViewport(p, request(), signal(), {
    ...drawing(),
    onFrame: async () => {},
  });
  const firstShade = addresses.findIndex((u) => u.includes("style=hillshade"));
  // At most three first-style requests can precede the first related style.
  assert.ok(
    firstShade > 0 && firstShade <= 3,
    "physical samples evicted before related style request",
  );
  assert.ok(max <= 3);
});

test("deep display LOD increases beyond finite relief without claiming new source samples", () => {
  const spec = { version: "continuous-3", surfaceWidth: 4096, radiusKm: 6371 };
  let previous = -1;
  for (const w of [2, 0.25, 0.03, 0.003, 0.0003, 0.00008]) {
    const camera = { x: 1.1, y: 0.2, w, h: w / 2 };
    const z = visibleTiles(camera, 892, 1, 695)[0].z;
    assert.ok(z > previous);
    previous = z;
    const r = reliefResolution(spec, camera, 892, 695);
    assert.equal(r.sourceWidth, 4096);
    assert.ok(r.spacingKM > 9 && r.spacingKM < 10);
    assert.equal(r.resolved, w < 0.5);
  }
  assert.equal(previous, 16);
  assert.equal(
    reliefResolution({ version: "continuous-1" }, camera, 892, 695),
    null,
  );
});
test("viewport request covers the actual SVG, with the original sample density", () => {
  const r = request();
  assert.equal(r.z, 5);
  const b = r.bounds;
  for (const [x, y] of [
    [0, 0],
    [892, 695],
  ]) {
    const p = screenToMap(
      x,
      y,
      { left: 0, top: 0, width: 892, height: 695 },
      camera,
    );
    assert.ok(Math.abs(p[0] - (x ? b.x + b.w : b.x)) < 1e-12);
    assert.ok(Math.abs(p[1] - (y ? b.y + b.h : b.y)) < 1e-12);
  }
  assert.equal(
    pyramidNodes({ ...r, bounds: { x: 0, y: 0, w: 2, h: 1 } }, 0).length,
    2,
  );
});
test("every child has the same bounds as its quadrant of the parent", () => {
  const r = request();
  for (const n of pyramidNodes(r)) {
    const parent = {
      x: Math.floor(n.x / 2),
      y: Math.floor(n.y / 2),
      span: n.span * 2,
    };
    assert.equal(n.x * n.span, parent.x * parent.span + (n.x % 2) * n.span);
    assert.equal(n.y * n.span, parent.y * parent.span + (n.y % 2) * n.span);
  }
});
test("pyramid reuse is local, while missing revision history invalidates every level", async () => {
  let fetches = 0;
  const p = new TerrainPyramid({
      fetchTile: async () => {
        fetches++;
        return new Blob(["pixels"]);
      },
    }),
    r = p.prepare(request()),
    node = pyramidNodes(r)[0];
  await p.get(r, node, "natural", signal());
  await p.get(r, node, "natural", signal());
  assert.equal(fetches, 1);
  const clean = p.prepare(
    request({
      revision: 1,
      invalidations: [{ revision: 1, x: 1.8, y: 0.8, w: 0.01, h: 0.01 }],
    }),
  );
  await p.get(clean, node, "natural", signal());
  assert.equal(fetches, 1);
  const unknown = p.prepare(request({ revision: 3 }));
  await p.get(unknown, node, "natural", signal());
  assert.equal(fetches, 2);
  await p.get(unknown, node, "biome", signal());
  assert.equal(fetches, 3);
});
test("bounded blob cache eviction cannot remove pixels already assembled into a frame", async () => {
  const p = new TerrainPyramid({
      budget: 8,
      fetchTile: async () => new Blob(["12345678"]),
    }),
    frames = [],
    draw = drawing();
  await renderViewport(p, request(), signal(), {
    ...draw,
    onFrame: (f) => frames.push(f),
  });
  assert.ok(p.bytes <= 8);
  assert.ok(p.stats.evictions > 0);
  assert.deepEqual(
    frames.map((f) => f.phase),
    ["coarse", "final"],
  );
  assert.ok(frames.every((f) => f.images.length === 2));
});
test("a delayed layer never publishes a partially completed transaction", async () => {
  let release,
    held = false;
  const wait = new Promise((r) => (release = r)),
    frames = [];
  const p = new TerrainPyramid({
    fetchTile: async (url) => {
      if (url.includes("style=hillshade") && !held) {
        held = true;
        await wait;
      }
      return new Blob(["pixel"]);
    },
  });
  const done = renderViewport(
    p,
    request({ camera: { x: 0, y: 0, w: 2, h: 1 }, width: 256, height: 128 }),
    signal(),
    { ...drawing(), onFrame: (f) => frames.push(f) },
  );
  for (let i = 0; i < 20 && !held; i++)
    await new Promise((r) => setTimeout(r, 5));
  assert.ok(held);
  assert.equal(frames.length, 0);
  release();
  await done;
  assert.equal(frames.length, 1);
  assert.equal(frames[0].images.length, 2);
});
test("obsolete renders do not publish and use at most three requests across styles", async () => {
  let active = 0,
    max = 0;
  const abort = new AbortController(),
    frames = [];
  const p = new TerrainPyramid({
    fetchTile: async (url, revision, signal) => {
      active++;
      max = Math.max(max, active);
      await new Promise((r) => setTimeout(r, 10));
      active--;
      signal.throwIfAborted();
      return new Blob(["pixel"]);
    },
  });
  const done = renderViewport(p, request(), abort.signal, {
    ...drawing(),
    onFrame: (f) => frames.push(f),
  });
  setTimeout(() => abort.abort(), 2);
  await assert.rejects(done, { name: "AbortError" });
  assert.equal(frames.length, 0);
  assert.ok(max <= 3);
  assert.equal(active, 0);
});
test("coarse failure does not prevent a complete fine replacement", async () => {
  const p = new TerrainPyramid({
      fetchTile: async (url) => {
        if (url.includes("/tiles/3/")) throw Error("coarse failed");
        return new Blob(["fine"]);
      },
    }),
    frames = [];
  await renderViewport(p, request(), signal(), {
    ...drawing(),
    onFrame: (f) => frames.push(f),
  });
  assert.deepEqual(
    frames.map((f) => f.phase),
    ["final"],
  );
});
test("large/high-DPI requests keep density, with an explicit resource limit", async () => {
  const r = request({ width: 1920, height: 1080, dpr: 2 });
  assert.deepEqual(r.pixels, { width: 3840, height: 2160 });
  assert.ok(pyramidNodes(r).length > 0);
  await assert.rejects(
    renderViewport(
      new TerrainPyramid(),
      request({ width: 4096, height: 2160, dpr: 3 }),
      signal(),
      { ...drawing(), onFrame: () => {} },
    ),
    /32-megapixel/,
  );
});

test("coarse previews avoid full-DPI storage while final detail retains every requested pixel", async () => {
  const r = request({ width: 1920, height: 1080, dpr: 3 }),
    frames = [];
  const p = new TerrainPyramid({ fetchTile: async () => new Blob(["sample"]) });
  await renderViewport(p, r, signal(), {
    ...drawing(),
    onFrame: (f) => frames.push(f),
  });
  assert.equal(frames[0].phase, "coarse");
  assert.deepEqual(frames[0].pixels, { width: 1440, height: 810 });
  assert.deepEqual(frames.at(-1).pixels, r.pixels);
  assert.deepEqual(frames[0].bounds, frames.at(-1).bounds);
});

test("Balanced's global level 1 publishes a complete level 0 before fine work", async () => {
  const r = request({
    camera: { x: 0, y: 0, w: 2, h: 1 },
    width: 1200,
    height: 800,
    dpr: 0.75,
  });
  assert.equal(r.z, 1);
  const frames = [],
    p = new TerrainPyramid({ fetchTile: async () => new Blob(["terrain"]) });
  await renderViewport(p, r, signal(), {
    ...drawing(),
    onFrame: (f) => frames.push(f),
  });
  assert.deepEqual(
    frames.map((f) => f.phase),
    ["coarse", "final"],
  );
  assert.deepEqual(frames[0].pixels, { width: 450, height: 300 });
  assert.deepEqual(frames[0].bounds, frames[1].bounds);
});
