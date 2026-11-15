import test from "node:test";
import assert from "node:assert/strict";
import { JSDOM } from "jsdom";
import { build } from "esbuild";
import { mkdtemp, rm } from "node:fs/promises";
import { resolve, join } from "node:path";
import { pathToFileURL } from "node:url";
import React, { act } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import {
  illustrationLayout,
  illustrationPageBounds,
} from "../src/continuous/illustration-layout.js";
import { newIllustration } from "../src/continuous/illustration.js";
import { mapProjection } from "../src/continuous/projection.js";

const dom = new JSDOM('<div id="host"></div>', { url: "http://localhost/" });
for (const name of [
  "window",
  "document",
  "HTMLElement",
  "Event",
  "MouseEvent",
  "CustomEvent",
  "location",
  "history",
])
  globalThis[name] = dom.window[name];
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
globalThis.requestAnimationFrame = (fn) => setTimeout(fn, 0);
globalThis.cancelAnimationFrame = clearTimeout;
globalThis.WebSocket = class {
  close() {}
};
const { createRoot } = await import("react-dom/client");
const temp = await mkdtemp(resolve(".tools/terrain-workflow-"));
const output = join(temp, "components.mjs");
await build({
  stdin: {
    contents: `export {default as App} from './src/App.jsx'; export {default as UIProvider} from './src/worldbuilding/ui/UIProvider.jsx';
      export {default as TerrainRouter} from './src/continuous/TerrainRouter.jsx';
      export {default as IllustrationScene} from './src/continuous/IllustrationScene.jsx';
      export {ProjectionContext} from './src/continuous/MapProjection.js';`,
    resolveDir: process.cwd(),
    loader: "jsx",
  },
  outfile: output,
  bundle: true,
  platform: "node",
  format: "esm",
  packages: "external",
  jsx: "automatic",
  loader: { ".css": "empty", ".ttf": "file" },
  plugins: [
    {
      name: "terrain-editors",
      setup(builder) {
        builder.onLoad(
          { filter: /[\\/]continuous[\\/](Studio|ArchivedTerrain)\.jsx$/ },
          () => ({
            contents:
              'export default function Editor() { return <div data-terrain-editor="true"/> }',
            loader: "jsx",
          }),
        );
      },
    },
  ],
});
const { App, TerrainRouter, IllustrationScene, ProjectionContext, UIProvider } =
  await import(pathToFileURL(output));
const h = React.createElement;
let root;
let calls;
function responses(routes) {
  calls = [];
  globalThis.fetch = async (url, options) => {
    calls.push([url, options?.method || "GET"]);
    assert.ok(Object.hasOwn(routes, url), `Unexpected request: ${url}`);
    return {
      ok: true,
      status: 200,
      json: async () =>
        typeof routes[url] === "function" ? routes[url]() : routes[url],
    };
  };
}
async function mount(component, path = "/") {
  history.replaceState({}, "", path);
  root = createRoot(document.querySelector("#host"));
  await act(async () => {
    root.render(component);
  });
  await act(async () => {
    await new Promise((r) => setTimeout(r, 10));
  });
}
test.afterEach(async () => {
  if (root) {
    await act(async () => root.unmount());
    root = null;
  }
});
test.after(async () => {
  await rm(temp, { recursive: true, force: true });
  dom.window.close();
});

test("welcome lists Worlds without fetching or displaying standalone terrain", async () => {
  responses({
    "/api/invitations": [],
    "/api/worlds": [{ id: "world-a", name: "My World" }],
  });
  await mount(h(App));
  assert.match(document.body.textContent, /My World/);
  assert.doesNotMatch(
    document.body.textContent,
    /Existing terrain projects|Terrain archives/,
  );
  assert.deepEqual(calls, [
    ["/api/invitations", "GET"],
    ["/api/worlds", "GET"],
  ]);
});

test("terrain generation requires an existing World and Age", async () => {
  responses({});
  await mount(h(TerrainRouter), "/world/new");
  assert.match(
    document.querySelector('[role="alert"]').textContent,
    /Open a World/,
  );
  assert.equal(document.querySelector("[data-terrain-editor]"), null);
  assert.deepEqual(calls, []);
});

test("terrain generation opens only after validating the World Age", async () => {
  responses({
    "/api/worlds/world-a?view=metadata": { ages: [{ id: "age-a" }] },
  });
  await mount(h(TerrainRouter), "/world/new?world=world-a&age=age-a");
  assert.ok(document.querySelector("[data-terrain-editor]"));
  assert.deepEqual(calls, [["/api/worlds/world-a?view=metadata", "GET"]]);
});

test("a saved terrain deep link recovers its World context before opening", async () => {
  const project = "a".repeat(32);
  responses({
    [`/api/map-context/${project}`]: {
      world: { id: "world-a" },
      map: { ageId: "age-a" },
    },
    [`/api/projects/${project}/open?view=terrain`]: {
      id: "session-a",
      continuous: {},
    },
  });
  await mount(h(TerrainRouter), `/world/${project}/build`);
  assert.ok(document.querySelector("[data-terrain-editor]"));
  assert.equal(location.search, "?world=world-a&age=age-a");
  assert.deepEqual(calls, [
    [`/api/map-context/${project}`, "GET"],
    [`/api/projects/${project}/open?view=terrain`, "POST"],
  ]);
});

test("an unlinked terrain project cannot open outside a World", async () => {
  const project = "b".repeat(32);
  responses({ [`/api/map-context/${project}`]: {} });
  await mount(h(TerrainRouter), `/world/${project}/build`);
  assert.match(
    document.querySelector('[role="alert"]').textContent,
    /Open terrain from its World/,
  );
  assert.equal(calls.length, 1);
});

test("title and compass fit in parchment above every geographic scope", () => {
  for (const [scope, bounds] of [
    ["world", [0, 0, 2, 1]],
    ["continent", [1.8, 0.3, 0.5, 0.6]],
    ["country", [0.2, 0.4, 0.1, 0.04]],
    ["city", [0.5, 0.5, 0.0001, 0.0001]],
  ])
    for (const title of [
      "Atlas",
      "An exceptionally long illustrated map title ".repeat(4),
    ]) {
      const config = newIllustration(scope, {
        x: bounds[0],
        y: bounds[1],
        w: bounds[2],
        h: bounds[3],
      });
      config.title = title;
      const before = JSON.stringify(config);
      const layout = illustrationLayout(config);
      assert.deepEqual(illustrationPageBounds(config), layout.page);
      assert.equal(
        JSON.stringify(config),
        before,
        "layout must preserve geographic bounds and saved design",
      );
      assert.ok(layout.title.y + layout.title.size < bounds[1]);
      assert.ok(layout.title.y - layout.title.size > layout.page.y);
      assert.ok(layout.compass.y + layout.compass.size * 1.03 < bounds[1]);
      assert.ok(layout.compass.y - layout.compass.size * 1.65 > layout.page.y);
      const titleRight =
        layout.title.x + (title.length * layout.title.size * 0.65) / 2;
      assert.ok(titleRight < layout.compass.x - layout.compass.size);
      const projection = mapProjection(layout.page, 1200, 800);
      const scene = renderToStaticMarkup(
        h(
          "svg",
          null,
          h(
            ProjectionContext.Provider,
            { value: projection },
            h(IllustrationScene, {
              config,
              objects: [],
              pixel: bounds[2] / 1200,
              state: { continuous: { spec: { radiusKm: 6371 } } },
            }),
          ),
        ),
      );
      const svg = new JSDOM(scene).window.document;
      const header = svg.querySelector(
        '[data-illustration-decoration="page-header"]',
      );
      assert.ok(header);
      const text = header.querySelector("text");
      const northEdge = projection.point([bounds[0], bounds[1]])[1];
      assert.ok(
        +text.getAttribute("y") + +text.getAttribute("font-size") < northEdge,
      );
      assert.equal(text.textContent, title);
      assert.equal(
        header.querySelectorAll("path").length,
        2,
        "compass is rendered with title",
      );
    }
});

test("hiding decoration removes both header furniture and extra page margin", () => {
  const config = newIllustration("world", { x: 0, y: 0, w: 2, h: 1 });
  config.layers.find((l) => l.id === "decoration").visible = false;
  assert.deepEqual(illustrationPageBounds(config), { x: 0, y: 0, w: 2, h: 1 });
  assert.doesNotMatch(
    renderToStaticMarkup(
      h(
        "svg",
        null,
        h(IllustrationScene, { config, objects: [], pixel: 0.001 }),
      ),
    ),
    /page-header/,
  );
});

test("navigating inside a World reuses its loaded records", async () => {
  let completeWorkspace;
  const world = "c".repeat(32),
    age = "first";
  const doc = {
    world: { id: world, name: "Living world", currentAge: age, revision: 1 },
    ages: [{ id: age, name: "First Age", timelineId: "timeline" }],
    timelines: [{ id: "timeline", name: "Timeline", ages: [age], order: 0 }],
    entities: [],
    maps: [],
    terrains: [],
    representations: [],
    relations: [],
    years: [],
    edges: [],
  };
  responses({
    "/api/invitations": [],
    [`/api/worlds/${world}?view=atlas`]: doc,
    [`/api/worlds/${world}?view=workspace`]: () =>
      new Promise((resolve) => {
        completeWorkspace = resolve;
      }),
    [`/api/worlds/${world}/members`]: { members: [], role: "viewer" },
  });
  await mount(h(UIProvider, null, h(App)), `/worlds/${world}`);
  const atlas = [...document.querySelectorAll("button")].find((b) =>
    b.textContent.trim().startsWith("Time Graph"),
  );
  assert.ok(atlas);
  await act(async () => {
    atlas.click();
    await new Promise((r) => setTimeout(r, 20));
  });
  assert.equal(location.pathname, `/worlds/${world}/graph`);
  assert.equal(
    calls.filter(([url]) => url === `/api/worlds/${world}?view=atlas`).length,
    1,
    "a page switch must not reread the complete World",
  );
  const navigateSection = async (name) => {
    await act(async () => {
      [...document.querySelectorAll(".sidebar-section")]
        .find((b) => b.textContent.trim() === name)
        .click();
      await new Promise((resolve) => setTimeout(resolve, 10));
    });
  };
  await navigateSection("Characters");
  assert.ok(completeWorkspace, "entity pages load their records on demand");
  assert.match(document.querySelector("header").textContent, /Working/);
  await navigateSection("Atlas");
  assert.doesNotMatch(
    document.querySelector("header").textContent,
    /Working/,
    "canceling an entity load must not leave the World permanently busy",
  );
  await act(async () => {
    completeWorkspace(doc);
    await new Promise((resolve) => setTimeout(resolve, 10));
  });
  assert.equal(location.pathname, `/worlds/${world}`);
  assert.equal(calls.filter(([url]) => url.includes("view=atlas")).length, 1);
});

test("Age inheritance loads biological records on demand and keeps the selected older Age", async () => {
  const world = "d".repeat(32);
  const atlas = {
    world: {
      id: world,
      name: "Living world",
      currentAge: "later",
      revision: 1,
    },
    ages: [
      { id: "first", name: "First Age", timelineId: "timeline" },
      { id: "later", name: "Later Age", timelineId: "timeline" },
    ],
    timelines: [
      { id: "timeline", name: "Timeline", ages: ["first", "later"], order: 0 },
    ],
    entities: [],
    maps: [],
    terrains: [],
    representations: [],
    relations: [],
    years: [],
    edges: [],
  };
  const workspace = {
    ...atlas,
    entities: [
      {
        id: "tree",
        ageId: "first",
        name: "Tree",
        kind: "lifeform",
        properties: {},
        biology: { group: "plant" },
      },
      {
        id: "tree",
        ageId: "later",
        name: "Later tree",
        kind: "lifeform",
        properties: {},
        biology: { group: "plant" },
      },
    ],
  };
  responses({
    [`/api/worlds/${world}?view=atlas`]: atlas,
    [`/api/worlds/${world}?view=workspace`]: workspace,
    [`/api/worlds/${world}/members`]: { members: [], role: "viewer" },
  });
  await mount(h(UIProvider, null, h(App)), `/worlds/${world}`);
  assert.equal(
    calls.filter(([url]) => url.includes("view=workspace")).length,
    0,
  );
  await act(async () => {
    document.querySelector(".age-node:not(.current)").click();
    await new Promise((r) => setTimeout(r, 10));
  });
  await act(async () => {
    document.querySelector(".age-new").click();
    await new Promise((r) => setTimeout(r, 20));
  });
  assert.equal(
    calls.filter(([url]) => url.includes("view=workspace")).length,
    1,
  );
  assert.match(
    document.querySelector(".age-dialog").textContent,
    /From First Age/,
  );
  assert.match(
    document.querySelector(".age-dialog summary").textContent,
    /1 of 1 included/,
  );
});
