import test from "node:test";
import assert from "node:assert/strict";
import { JSDOM } from "jsdom";
import { build } from "esbuild";
import { mkdtemp, rm } from "node:fs/promises";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import React, { act } from "react";
import {
  newIllustration,
  buildIllustration,
} from "../src/continuous/illustration.js";

const dom = new JSDOM('<div id="root"><div id="host"></div></div>', {
  url: "http://localhost/",
  pretendToBeVisual: true,
});
for (const key of [
  "window",
  "document",
  "location",
  "history",
  "HTMLElement",
  "HTMLInputElement",
  "Event",
  "MouseEvent",
  "KeyboardEvent",
])
  globalThis[key] = dom.window[key];
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
globalThis.requestAnimationFrame = (fn) => setTimeout(fn, 0);
globalThis.cancelAnimationFrame = clearTimeout;
dom.window.HTMLElement.prototype.getClientRects = function () {
  return [{ width: 100, height: 30 }];
};
const { createRoot } = await import("react-dom/client");
const temp = await mkdtemp(resolve(".tools/entity-names-")),
  output = join(temp, "components.mjs");
await build({
  stdin: {
    contents: `export {default as ResourceResultGroup} from './src/continuous/ResourceResultGroup.jsx'; export {LifeformResults} from './src/continuous/TerrainResourceReport.jsx'; export {default as WaterBodies} from './src/continuous/WaterBodies.jsx'; export {default as Profile} from './src/worldbuilding/Profile.jsx'; export {default as UIProvider} from './src/worldbuilding/ui/UIProvider.jsx';`,
    resolveDir: process.cwd(),
    loader: "jsx",
  },
  outfile: output,
  bundle: true,
  platform: "node",
  format: "esm",
  packages: "external",
  jsx: "automatic",
  loader: { ".css": "empty" },
});
const {
  WaterBodies,
  Profile,
  UIProvider,
  LifeformResults,
  ResourceResultGroup,
} = await import(pathToFileURL(output));
const h = React.createElement;
let root;
async function render(child) {
  if (!root) root = createRoot(document.querySelector("#host"));
  await act(async () => {
    root.render(h(UIProvider, null, child));
  });
}
async function input(el, value) {
  await act(async () => {
    Object.getOwnPropertyDescriptor(
      HTMLInputElement.prototype,
      "value",
    ).set.call(el, value);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}
async function click(el) {
  assert.ok(el, "button exists");
  await act(async () => {
    el.click();
    await new Promise((r) => setTimeout(r, 10));
  });
}
const button = (text, within = document) =>
  [...within.querySelectorAll("button")].find(
    (b) => b.textContent.trim() === text,
  );
const field = (text) =>
  [...document.querySelectorAll("label")]
    .find((l) => l.textContent.startsWith(text))
    ?.querySelector("input");
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

test("generated water names use source identity and never commit terrain", async () => {
  let body = {
      sourceId: "river-42",
      kind: "river",
      name: "River river-42",
      position: [1, 0.5],
    },
    revision = 4;
  const calls = [];
  globalThis.fetch = async (url, options) => {
    calls.push([url, options]);
    if (options.method === "POST") {
      assert.equal(
        url,
        "/api/sessions/map/terrain/water-bodies/river-42?view=metadata",
      );
      const payload = JSON.parse(options.body);
      assert.equal(payload.revision, 6);
      assert.equal(payload.name, "Silver River");
      body = { ...body, name: payload.name, entityId: "same-logical-river" };
      revision++;
      return {
        ok: true,
        status: 200,
        json: async () => ({ document: { world: { id: "world", revision } } }),
      };
    }
    return {
      ok: true,
      status: 200,
      json: async () => ({ bodies: [body], revision }),
    };
  };
  const props = {
    state: { id: "map", historicalMap: { ageId: "age" } },
    revision,
    selectedSource: "river-42",
    onFocus: () => {},
    onProfile: () => {},
    onCreate: () => {},
    onNamed: async () =>
      root.render(h(UIProvider, null, h(WaterBodies, { ...props, revision }))),
  };
  await render(h(WaterBodies, props));
  // A terrain autosave advances history without reloading the generated water
  // catalogue. Naming must use the new token without discarding typed input.
  await render(
    h(WaterBodies, { ...props, state: { ...props.state, historyRevision: 6 } }),
  );
  await input(field("Water body name"), "Silver River");
  await click(button("Name water body"));
  assert.ok(button("Rename water body"));
  assert.equal(field("Water body name").value, "Silver River");
  assert.equal(
    calls.filter(([url]) => url.endsWith("/save") || url.endsWith("/accept"))
      .length,
    0,
  );
  let opened;
  await render(
    h(WaterBodies, { ...props, revision, onProfile: (id) => (opened = id) }),
  );
  await click(button("Open water body record"));
  assert.equal(opened, "same-logical-river");
});

for (const kind of ["city", "ocean", "country", "character"])
  test(`${kind} profile rename edits the logical entity and follows external updates`, async () => {
    const entity = {
      id: "harbor",
      ageId: "age",
      name: "Old Harbor",
      kind,
      properties: { notes: "Preserved notes" },
    };
    let doc = {
      world: { id: "world", name: "World", currentAge: "age", revision: 1 },
      ages: [{ id: "age", name: "First Age" }],
      entities: [entity],
      relations: [],
      representations: [],
      maps: [],
      terrains: [],
      years: [],
    };
    const commands = [];
    const props = {
      onProfile: () => {},
      onBack: () => {},
      command: async (c) => {
        commands.push(c);
        doc = {
          ...doc,
          entities: [
            {
              ...entity,
              name: c.name,
              updatedAt: 20,
              names: [{ type: "canonical", name: c.name }],
            },
          ],
          world: { ...doc.world, revision: 2 },
        };
        root.render(
          h(
            UIProvider,
            null,
            h(Profile, { ...props, doc, entity: doc.entities[0] }),
          ),
        );
        return doc;
      },
    };
    await render(h(Profile, { ...props, doc, entity }));
    await click(button("Rename"));
    const dialog = document.querySelector('[role="dialog"]');
    await input(dialog.querySelector("input"), "New Harbor");
    await click(button("Save", dialog));
    assert.deepEqual(commands, [
      {
        kind: "rename-entity",
        ageId: "age",
        entityId: "harbor",
        name: "New Harbor",
      },
    ]);
    assert.equal(document.querySelector("h1").textContent, "New Harbor");
    assert.equal(field("Canonical name").value, "New Harbor");
    const unload = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(unload);
    assert.equal(
      unload.defaultPrevented,
      false,
      "a saved rename must leave a clean profile",
    );
    const renamed = { ...doc.entities[0], name: "Current Harbor" };
    doc = { ...doc, entities: [renamed] };
    await render(h(Profile, { ...props, doc, entity: renamed }));
    assert.equal(field("Canonical name").value, "Current Harbor");
    assert.equal(doc.entities[0].properties.notes, "Preserved notes");
  });

test("world illustrations include named oceans, lakes and other water regions", () => {
  const config = newIllustration("world", { x: 0, y: 0, w: 2, h: 1 });
  const state = { W: 101, H: 51 };
  const entities = ["ocean", "sea", "lake", "bay", "gulf", "strait"].map(
    (kind, i) => ({
      id: kind,
      name: `Named ${kind}`,
      kind,
      geometry: "point",
      points: [[10 + i * 12, 20]],
    }),
  );
  const labels = buildIllustration(config, entities, [], null, state).filter(
    (o) => o.kind === "text",
  );
  for (const e of entities)
    assert.ok(
      labels.some((o) => o.source === e.id && o.text === e.name),
      e.kind,
    );
  entities[0].name = "Renamed ocean";
  assert.ok(
    buildIllustration(config, entities, [], null, state).some(
      (o) => o.source === "ocean" && o.text === "Renamed ocean",
    ),
  );
});

test("terrain details keep a dense Biosphere searchable without rendering its entire catalogue", async () => {
  const items = Array.from({ length: 8000 }, (_, i) => ({
    id: `species-${i}`,
    name: `Species ${i}`,
    status: "native",
    roles: ["producer"],
    abundance: 0.5,
  }));
  await render(h(LifeformResults, { group: "plant", items }));
  assert.equal(document.querySelectorAll("details strong").length, 100);
  await click(button("Next lifeforms"));
  assert.equal(
    document.querySelector("details strong").textContent,
    "Species 100",
  );
  await input(document.querySelector("input[type=search]"), "Species 7999");
  assert.equal(document.querySelectorAll("details strong").length, 1);
  assert.equal(
    document.querySelector("details strong").textContent,
    "Species 7999",
  );
});

test("resource groups mount only on opening and page/search large inventories", async () => {
  const items = Array.from({ length: 8000 }, (_, i) => ({
    id: `deposit-${i}`,
    type: "ore",
    variant: `Deposit ${i}`,
  }));
  await render(
    h(ResourceResultGroup, { label: "Ores", items }, (r) =>
      h("article", { key: r.id }, r.variant),
    ),
  );
  const details = document.querySelector("details");
  assert.equal(document.querySelectorAll("article").length, 0);
  await act(async () => {
    details.open = true;
    details.dispatchEvent(new Event("toggle", { bubbles: true }));
  });
  assert.equal(document.querySelectorAll("article").length, 100);
  await click(button("Next resources"));
  assert.equal(document.querySelector("article").textContent, "Deposit 100");
  await input(document.querySelector("input[type=search]"), "Deposit 7999");
  assert.equal(document.querySelectorAll("article").length, 1);
  assert.equal(document.querySelector("article").textContent, "Deposit 7999");
  await act(async () => {
    details.open = false;
    details.dispatchEvent(new Event("toggle", { bubbles: true }));
  });
  assert.equal(document.querySelectorAll("article").length, 0);
});
