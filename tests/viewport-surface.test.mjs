import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { transformWithEsbuild } from "vite";
import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
const src = new URL("../src/continuous/ViewportSurface.jsx", import.meta.url),
  compiled = new URL("../.cache/viewport-surface-test.mjs", import.meta.url);
mkdirSync(new URL("../.cache", import.meta.url), { recursive: true });
const source = readFileSync(src, "utf8").replace(
  /from "(\.\/[^"]+)"/g,
  (_, path) => `from "${new URL(path, src).href}"`,
);
writeFileSync(
  compiled,
  (
    await transformWithEsbuild(source, "ViewportSurface.jsx", {
      jsx: "automatic",
    })
  ).code,
);
const { default: Surface, ViewportLayer } = await import(compiled.href);
const dom = new JSDOM('<div id="root"></div>');
let decode = async () => {},
  live = new Set(),
  serial = 0;
Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  devicePixelRatio: 1,
  IS_REACT_ACT_ENVIRONMENT: true,
  Image: class {
    decode() {
      return decode(this.src);
    }
  },
  createImageBitmap: async () => ({ close() {} }),
  OffscreenCanvas: class {
    constructor(w, h) {
      this.width = w;
      this.height = h;
    }
    getContext() {
      return { drawImage() {}, fillRect() {} };
    }
    async convertToBlob() {
      return new Blob(["image"]);
    }
  },
});

test("preview composition keeps one image and an export-layer placeholder; Illustrator retains separate layers", async () => {
  const root = createRoot(document.getElementById("root"));
  let loading;
  globalThis.fetch = async () =>
    new Response(new Blob(["png"]), { headers: { "X-Terrain-Revision": "1" } });
  decode = async () => {};
  const props = {
    identity: "composition",
    session: "s",
    revision: 1,
    styles: ["natural", "hillshade"],
    camera: { x: 0, y: 0, w: 2, h: 1 },
    width: 256,
    height: 128,
    previewQuality: { preset: "balanced" },
    previewHillshade: 0.55,
    onLoading: (n) => (loading = n),
  };
  const render = async (changes) => {
    Object.assign(props, changes);
    await act(() =>
      root.render(
        React.createElement(
          "svg",
          null,
          React.createElement(
            Surface,
            props,
            React.createElement(ViewportLayer, {
              style: "natural",
              primary: true,
            }),
            React.createElement(ViewportLayer, { style: "hillshade" }),
          ),
        ),
      ),
    );
  };
  try {
    await render({});
    await until(() => loading === 0);
    assert.equal(document.querySelectorAll("image").length, 1);
    assert.equal(
      document.querySelector('[data-terrain-style="hillshade"]').children
        .length,
      0,
    );
    const generation = +document.querySelector("image").dataset.generation;
    await render({ previewHillshade: 0.25 });
    await until(() => loading === 0);
    assert.ok(
      +document.querySelector("image").dataset.generation > generation,
      "shade strength was omitted from render identity",
    );
    await render({ previewQuality: undefined, previewHillshade: undefined });
    await until(() => loading === 0);
    assert.equal(document.querySelectorAll("image").length, 2);
  } finally {
    await act(() => root.unmount());
  }
  assert.equal(live.size, 0);
});
URL.createObjectURL = () => {
  const id = `blob:${serial++}`;
  live.add(id);
  return id;
};
URL.revokeObjectURL = (id) => live.delete(id);
const pause = (ms) => new Promise((r) => setTimeout(r, ms));
async function until(fn) {
  for (let i = 0; i < 200 && !fn(); i++) await act(() => pause(10));
  assert.ok(fn());
}
test("decoded layers commit together; obsolete decode cannot replace the latest camera; URLs release", async () => {
  const root = createRoot(document.getElementById("root"));
  let loading;
  globalThis.fetch = async () =>
    new Response(new Blob(["png"]), { headers: { "X-Terrain-Revision": "1" } });
  const props = {
    identity: "a",
    session: "s",
    revision: 1,
    styles: ["natural", "hillshade"],
    camera: { x: 0, y: 0, w: 2, h: 1 },
    width: 256,
    height: 128,
    onLoading: (n) => (loading = n),
  };
  const render = async (changes) => {
    Object.assign(props, changes);
    await act(() =>
      root.render(
        React.createElement(
          "svg",
          null,
          React.createElement(
            Surface,
            props,
            React.createElement(ViewportLayer, {
              style: "natural",
              primary: true,
            }),
            React.createElement(ViewportLayer, { style: "hillshade" }),
          ),
        ),
      ),
    );
  };
  const images = () => [...document.querySelectorAll("image")];
  try {
    await render({});
    await until(() => loading === 0);
    assert.equal(images().length, 2);
    const original = images()[0].getAttribute("href");
    let release;
    const held = new Promise((r) => (release = r));
    let heldCount = 0;
    decode = async () => {
      heldCount++;
      await held;
    };
    await render({ previewQuality: { preset: "performance" } });
    await until(() => heldCount === 2);
    assert.equal(images()[0].getAttribute("href"), original);
    decode = async () => {};
    await render({
      previewQuality: { preset: "high" },
      camera: { x: 0.4, y: 0, w: 2, h: 1 },
    });
    await until(() => loading === 0);
    const latest = images()[0].dataset.generation;
    await act(async () => {
      release();
      await pause(40);
    });
    assert.ok(images().every((i) => i.dataset.generation === latest));
    assert.equal(images()[0].getAttribute("x"), ".4".replace(/^\./, "0."));
    assert.equal(
      live.size,
      2,
      "obsolete and previous decoded frame URLs leaked",
    );
  } finally {
    await act(() => root.unmount());
  }
  assert.equal(live.size, 0);
});

test("zooming out retains a bounded world overview underneath the last close-up until replacement commits", async () => {
  const root = createRoot(document.getElementById("root"));
  let loading;
  globalThis.fetch = async () =>
    new Response(new Blob(["png"]), { headers: { "X-Terrain-Revision": "1" } });
  decode = async () => {};
  const props = {
    identity: "overview-regression",
    session: "s",
    revision: 1,
    styles: ["natural"],
    camera: { x: 1, y: 0.4, w: 0.001, h: 0.0005 },
    width: 256,
    height: 128,
    previewQuality: { preset: "balanced" },
    onLoading: (n) => (loading = n),
  };
  const render = async (camera) => {
    props.camera = camera;
    await act(() =>
      root.render(
        React.createElement(
          "svg",
          null,
          React.createElement(
            Surface,
            props,
            React.createElement(ViewportLayer, {
              style: "natural",
              primary: true,
            }),
          ),
        ),
      ),
    );
  };
  let release;
  try {
    await render(props.camera);
    await until(() => loading === 0);
    const closeup = document.querySelector("image").getAttribute("href");
    assert.equal(
      live.size,
      2,
      "one overview plus one close-up should be retained",
    );
    const held = new Promise((r) => (release = r));
    let heldCount = 0;
    decode = async () => {
      heldCount++;
      await held;
    };
    await render({ x: 0.8, y: 0.3, w: 0.4, h: 0.2 });
    await until(() => heldCount > 0);
    const overview = document.querySelector('image[data-overview="true"]');
    assert.ok(overview, "newly exposed terrain is black while detail loads");
    assert.equal(overview.getAttribute("x"), "0");
    assert.equal(overview.getAttribute("width"), "2");
    assert.equal(overview.getAttribute("height"), "1");
    assert.ok(
      [...document.querySelectorAll("image")].some(
        (i) => i.getAttribute("href") === closeup,
      ),
      "last coherent close-up disappeared",
    );
    decode = async () => {};
    await act(async () => {
      release();
      await pause(20);
    });
    await until(() => loading === 0);
    assert.equal(document.querySelectorAll("image").length, 1);
    assert.equal(document.querySelector("image").getAttribute("width"), "0.4");
    assert.equal(live.size, 2, "obsolete overview/close-up URLs leaked");
  } finally {
    release?.();
    decode = async () => {};
    await act(() => root.unmount());
  }
  assert.equal(live.size, 0);
});
