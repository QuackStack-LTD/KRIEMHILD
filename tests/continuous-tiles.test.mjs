import test from "node:test";
import assert from "node:assert/strict";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { transformWithEsbuild } from "vite";
import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { visibleTiles } from "../src/continuous/camera.js";
mkdirSync(new URL("../.cache", import.meta.url), { recursive: true });
const compiled = new URL("../.cache/tile-surface-test.mjs", import.meta.url);
const sourceURL = new URL("../src/continuous/TileSurface.jsx", import.meta.url);
const source = readFileSync(sourceURL, "utf8").replace(
  /from "(\.\/[^"]+)"/g,
  (_, path) => `from "${new URL(path, sourceURL).href}"`,
);
const compiledSource = await transformWithEsbuild(source, "TileSurface.jsx", {
  jsx: "automatic",
});
writeFileSync(compiled, compiledSource.code);
const { default: TileSurface } = await import(compiled.href + "?" + Date.now());
const dom = new JSDOM("<div id='root'></div>");
Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  devicePixelRatio: 1,
  IS_REACT_ACT_ENVIRONMENT: true,
});
const camera = { x: 0.6, y: 0.3, w: 0.2, h: 0.1 },
  invalidations = [];
let serial = 0;
const pause = (ms) => new Promise((r) => setTimeout(r, ms));
async function until(predicate) {
  for (let i = 0; i < 150; i++) {
    if (predicate()) return;
    await act(() => pause(20));
  }
  assert.ok(predicate(), "tile work did not settle");
}
async function fixture(fn) {
  const root = createRoot(document.getElementById("root"));
  const props = {
    session: "test",
    identity: "fixture-" + serial++,
    revision: 1,
    invalidations,
    style: "natural",
    camera,
    width: 256,
  };
  const render = async (changes) => {
    Object.assign(props, changes);
    await act(async () =>
      root.render(
        React.createElement(
          "svg",
          null,
          React.createElement(TileSurface, props),
        ),
      ),
    );
  };
  try {
    await fn({
      props,
      render,
      images: () => [...document.querySelectorAll("image")],
    });
  } finally {
    await act(async () => root.unmount());
  }
}
const response = (revision = 1) =>
  new Response(new Blob(["png"]), {
    headers: {
      "X-Terrain-Revision": String(revision),
      "Content-Type": "image/png",
    },
  });

test("cache eviction cannot remove visible terrain on a high-DPI display", async () =>
  fixture(async ({ render, images }) => {
    globalThis.devicePixelRatio = 3;
    globalThis.fetch = async () => response();
    let loading;
    try {
      await render({
        camera: { x: 0, y: 0, w: 2, h: 1 },
        width: 4096,
        height: 2048,
        onLoading: (n) => (loading = n),
      });
      await until(() => loading === 0);
      const children = images().filter(
        (e) => +e.getAttribute("width") === 1 / 16,
      );
      assert.equal(
        children.length,
        512,
        "every visible child must survive cache eviction",
      );
    } finally {
      globalThis.devicePixelRatio = 1;
    }
  }));

test("a local dirty rectangle reuses unaffected tiles and replaces affected parents", async () =>
  fixture(async ({ render, images }) => {
    const calls = [];
    globalThis.fetch = async (url) => {
      calls.push(url);
      return response(
        new URL(url, "http://local").searchParams.get("revision"),
      );
    };
    let loading;
    await render({ onLoading: (n) => (loading = n) });
    await until(() => loading === 0);
    const before = calls.length;
    await render({
      revision: 2,
      invalidations: [{ x: 0.61, y: 0.31, w: 0.01, h: 0.01, revision: 2 }],
    });
    await until(() => loading === 0);
    assert.ok(calls.length > before && calls.length < 2 * before);
    assert.ok(images().some((e) => e.dataset.revision === "1"));
    assert.ok(images().some((e) => e.dataset.revision === "2"));
  }));

test("parent rectangles precede correctly positioned child rectangles", async () =>
  fixture(async ({ render, images }) => {
    globalThis.fetch = async () => response();
    await render();
    await until(() => images().length >= 6);
    const spans = images().map((e) => +e.getAttribute("width"));
    assert.deepEqual(
      spans,
      [...spans].sort((a, b) => b - a),
    );
    for (const e of images()) {
      const span = +e.getAttribute("width");
      assert.equal(+e.getAttribute("height"), span);
      assert.equal((+e.getAttribute("x") / span) % 1, 0);
      assert.equal((+e.getAttribute("y") / span) % 1, 0);
    }
  }));

test("a revision without a dirty rectangle cannot reuse old terrain", async () =>
  fixture(async ({ render, images }) => {
    const calls = [];
    globalThis.fetch = async (url) => {
      calls.push(url);
      return response(
        new URL(url, "http://local").searchParams.get("revision"),
      );
    };
    await render();
    await until(() => images().length >= 6);
    const old = new Set(images().map((e) => e.getAttribute("href")));
    await render({ revision: 2 });
    await until(() => calls.some((u) => u.includes("revision=2")));
    await until(
      () =>
        images().length >= 6 &&
        images().every((e) => !old.has(e.getAttribute("href"))),
    );
  }));

test("transient child failures retry without camera movement", async () =>
  fixture(async ({ render, images }) => {
    const attempts = new Map();
    globalThis.fetch = async (url) => {
      const n = (attempts.get(url) || 0) + 1;
      attempts.set(url, n);
      return url.includes("/3/") && n === 1
        ? new Response("", { status: 503 })
        : response();
    };
    let loading;
    await render({ onLoading: (n) => (loading = n) });
    await until(() => [...attempts.values()].some((n) => n > 1));
    await until(() => loading === 0);
    assert.ok(images().length >= 6);
  }));

test("a delayed child retains its parent while neighboring children load", async () =>
  fixture(async ({ render, images, props }) => {
    const target = visibleTiles(props.camera, props.width)[0];
    const address = `${target.z}/${target.x}/${target.y}`;
    let release, loading;
    globalThis.fetch = (url) =>
      url.includes(`/tiles/${address}?`)
        ? new Promise((resolve) => {
            release = () => resolve(response());
          })
        : Promise.resolve(response());
    await render({ onLoading: (n) => (loading = n) });
    await until(
      () =>
        release &&
        images().some((e) => e.dataset.tile.startsWith(`${target.z}/`)),
    );
    assert.ok(loading > 0);
    assert.ok(!images().some((e) => e.dataset.tile === address));
    const parent = images().find(
      (e) =>
        e.dataset.tile ===
        `${target.z - 1}/${Math.floor(target.x / 2)}/${Math.floor(target.y / 2)}`,
    );
    assert.ok(parent, "the missing child must remain covered by its parent");
    assert.equal(+parent.getAttribute("width"), target.span * 2);
    await act(async () => release());
    await until(() => loading === 0);
    assert.ok(images().some((e) => e.dataset.tile === address));
  }));

test("camera cancellation discards obsolete results and loads the latest viewport", async () =>
  fixture(async ({ render, images }) => {
    let aborted = 0,
      calls = 0;
    globalThis.fetch = (url, { signal }) => {
      calls++;
      return new Promise((resolve, reject) => {
        const timer = setTimeout(() => resolve(response()), 160);
        signal.addEventListener(
          "abort",
          () => {
            clearTimeout(timer);
            aborted++;
            reject(new DOMException("aborted", "AbortError"));
          },
          { once: true },
        );
      });
    };
    await render();
    await until(() => calls > 0);
    await render({ camera: { ...camera, x: 1.4 } });
    await until(() => images().length >= 6);
    assert.ok(aborted > 0);
    assert.ok(images().every((e) => +e.getAttribute("x") >= 1));
  }));
