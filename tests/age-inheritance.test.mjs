import test from "node:test";
import assert from "node:assert/strict";
import { JSDOM } from "jsdom";
import { build } from "esbuild";
import { mkdtemp, rm } from "node:fs/promises";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import React, { act } from "react";

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
  "CustomEvent",
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
const temp = await mkdtemp(resolve(".tools/age-inheritance-")),
  output = join(temp, "components.mjs");
await build({
  stdin: {
    contents: `export {default as AgeDialog} from './src/worldbuilding/AgeDialog.jsx'; export {default as useWorldNames, announceWorldChange} from './src/worldbuilding/useWorldNames.js'; export {default as UIProvider} from './src/worldbuilding/ui/UIProvider.jsx';`,
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
const { AgeDialog, useWorldNames, announceWorldChange, UIProvider } =
  await import(pathToFileURL(output));
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

const button = (text) =>
  [...document.querySelectorAll("button")].find(
    (b) => b.textContent.trim() === text,
  );
async function openReview() {
  const details = [...document.querySelectorAll("details")].find((d) =>
    d.querySelector("summary").textContent.includes("Review inherited"),
  );
  await act(async () => {
    details.open = true;
    details.dispatchEvent(new Event("toggle"));
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
});

function largeWorld(count = 8000) {
  return {
    world: { id: "world", currentAge: "first", revision: 1 },
    ages: [
      { id: "first", name: "First" },
      { id: "other", name: "Other" },
    ],
    maps: [],
    representations: [],
    entities: Array.from({ length: count }, (_, i) => ({
      id: `species-${i}`,
      ageId: "first",
      kind: "lifeform",
      name: `Species ${String(i).padStart(5, "0")}`,
      properties: {},
      biology: { group: "plant" },
    })),
  };
}

test("a large Biosphere keeps the Age dialog bounded and preserves edits across search and pages", async () => {
  const doc = largeWorld();
  let command;
  await render(
    h(AgeDialog, {
      doc,
      sourceAge: "first",
      onClose() {},
      onSubmit(c) {
        command = c;
      },
    }),
  );
  assert.equal(
    document.querySelectorAll(".inheritance-review input").length,
    0,
    "closed review mounts no entity editors",
  );
  await input(document.querySelector("input[type=text]"), "Later");
  await openReview();
  assert.equal(
    document.querySelectorAll(".inheritance-review input[type=checkbox]")
      .length,
    100,
  );
  await click(
    document.querySelector(".inheritance-review input[type=checkbox]"),
  );
  await click(button("Next records"));
  assert.match(
    document.querySelector(".inheritance-review").textContent,
    /Species 00100/,
  );
  await input(document.querySelector("input[type=search]"), "Species 07999");
  assert.equal(
    document.querySelectorAll(".inheritance-review input[type=checkbox]")
      .length,
    1,
  );
  await input(
    document.querySelector(".inheritance-review input[type=text]"),
    "Later tree",
  );
  await input(document.querySelector("input[type=search]"), "");
  assert.equal(
    document.querySelector(".inheritance-review input[type=checkbox]").checked,
    false,
    "exclusion retained across pages",
  );
  await click(button("Apply"));
  assert.equal(command.name, "Later");
  assert.deepEqual(command.excludeIds, ["species-0"]);
  assert.deepEqual(command.entityNames, { "species-7999": "Later tree" });
  assert.equal(command.ageId, "first");
});

test("include and exclude all cover the whole catalogue while filtered and merging follows the selected source", async () => {
  const doc = largeWorld(300);
  doc.entities.push({
    ...doc.entities[0],
    ageId: "other",
    name: "Other tree",
    biology: { group: "animal" },
  });
  let command;
  await render(
    h(AgeDialog, {
      doc,
      sourceAge: "first",
      onClose() {},
      onSubmit(c) {
        command = c;
      },
    }),
  );
  await input(document.querySelector("input[type=text]"), "Merged");
  const operation = document.querySelector("select");
  await act(async () => {
    operation.value = "merge";
    operation.dispatchEvent(new Event("change", { bubbles: true }));
  });
  const policy = [...document.querySelectorAll("select")].find((s) =>
    s.querySelector("option[value=secondary]"),
  );
  await act(async () => {
    policy.value = "primary";
    policy.dispatchEvent(new Event("change", { bubbles: true }));
  });
  await openReview();
  assert.equal(
    document.querySelector(".inheritance-review input[type=text]").value,
    "Species 00000",
  );
  await act(async () => {
    policy.value = "secondary";
    policy.dispatchEvent(new Event("change", { bubbles: true }));
  });
  assert.equal(
    document.querySelector(".inheritance-review input[type=text]").value,
    "Other tree",
  );
  await input(document.querySelector("input[type=search]"), "Other tree");
  await click(button("Exclude all"));
  await click(
    document.querySelector(".inheritance-review input[type=checkbox]"),
  );
  await click(button("Apply"));
  assert.equal(
    command.excludeIds.length,
    299,
    "bulk exclusion includes off-page and filtered records",
  );
  assert.ok(!command.excludeIds.includes("species-0"));
  await click(button("Include all"));
  await click(button("Apply"));
  assert.deepEqual(command.excludeIds, []);
});

test("an old in-flight name poll cannot overwrite a newly created Age or download it twice", async () => {
  const previousFetch = globalThis.fetch,
    requests = [],
    documents = [];
  let remember;
  globalThis.fetch = (url) =>
    new Promise((resolve) => requests.push({ url, resolve }));
  function Probe() {
    remember = useWorldNames("world", 1, (d) => documents.push(d));
    return null;
  }
  try {
    await render(h(Probe));
    await act(async () => announceWorldChange("world"));
    assert.match(requests[0].url, /revision=1/);
    remember(2);
    await act(async () =>
      requests[0].resolve({
        ok: true,
        status: 200,
        json: async () => ({ document: { world: { revision: 1 } } }),
      }),
    );
    assert.equal(documents.length, 0, "stale poll response ignored");
    await act(async () => announceWorldChange("world"));
    assert.match(
      requests[1].url,
      /revision=2/,
      "broadcast uses locally committed revision",
    );
    await act(async () =>
      requests[1].resolve({
        ok: true,
        status: 200,
        json: async () => ({ unchanged: true }),
      }),
    );
    assert.equal(documents.length, 0);
  } finally {
    globalThis.fetch = previousFetch;
  }
});

test("name polling pauses during an Age transaction instead of queuing another World download", async () => {
  const previousFetch = globalThis.fetch;
  let requests = 0;
  globalThis.fetch = async () => {
    requests++;
    return { ok: true, status: 200, json: async () => ({ unchanged: true }) };
  };
  function Probe({ paused }) {
    useWorldNames("world", 1, () => {}, paused);
    return null;
  }
  try {
    await render(h(Probe, { paused: true }));
    await act(async () => announceWorldChange("world"));
    assert.equal(
      requests,
      0,
      "no revision request queued behind a pending write",
    );
    await render(h(Probe, { paused: false }));
    await act(async () => announceWorldChange("world"));
    assert.equal(requests, 1, "polling resumes after the transaction");
  } finally {
    globalThis.fetch = previousFetch;
  }
});

test("name polling applies compact deltas and ignores a superseded rename response", async () => {
  const previousFetch = globalThis.fetch,
    requests = [],
    changes = [];
  let remember;
  globalThis.fetch = (url) =>
    new Promise((resolve) => requests.push({ url, resolve }));
  function Probe() {
    remember = useWorldNames(
      "world",
      1,
      () => assert.fail("full World reload"),
      false,
      "workspace",
      (c) => changes.push(c),
    );
    return null;
  }
  try {
    await render(h(Probe));
    await act(async () => announceWorldChange("world"));
    assert.match(requests[0].url, /delta=names/);
    await act(async () =>
      requests[0].resolve({
        ok: true,
        status: 200,
        json: async () => ({ nameChange: { base: 1, world: { revision: 2 } } }),
      }),
    );
    assert.equal(changes.length, 1);
    remember(3);
    await act(async () => announceWorldChange("world"));
    await act(async () =>
      requests[1].resolve({
        ok: true,
        status: 200,
        json: async () => ({ nameChange: { base: 1, world: { revision: 2 } } }),
      }),
    );
    assert.equal(changes.length, 1, "stale rename delta ignored");
  } finally {
    globalThis.fetch = previousFetch;
  }
});
