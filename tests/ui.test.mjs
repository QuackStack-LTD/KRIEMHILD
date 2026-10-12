import test from "node:test";
import assert from "node:assert/strict";
import { JSDOM } from "jsdom";
import { build } from "esbuild";
import { mkdtemp, rm } from "node:fs/promises";
import { pathToFileURL } from "node:url";
import { join, resolve } from "node:path";
import React, { act } from "react";

const dom = new JSDOM(
  '<div id="root"><button id="opener">Open</button><div id="host"></div></div>',
  { url: "http://localhost" },
);
for (const key of [
  "window",
  "document",
  "HTMLElement",
  "KeyboardEvent",
  "Event",
  "MouseEvent",
])
  globalThis[key] = dom.window[key];
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
globalThis.requestAnimationFrame = (fn) => setTimeout(fn, 0);
globalThis.cancelAnimationFrame = clearTimeout;
dom.window.HTMLElement.prototype.getClientRects = function () {
  return this.hidden ? [] : [{ width: 100, height: 40 }];
};
const { createRoot } = await import("react-dom/client");
const temp = await mkdtemp(resolve(".tools/ui-test-"));
const output = join(temp, "components.mjs");
await build({
  stdin: {
    contents: `export {default as Dialog} from './src/worldbuilding/ui/Dialog.jsx'; export {default as UIProvider} from './src/worldbuilding/ui/UIProvider.jsx'; export * from './src/worldbuilding/ui/feedback.js'; export {LibraryDialog} from './src/worldbuilding/Library.jsx'; export {Preview} from './src/worldbuilding/Biosphere.jsx';`,
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
  Dialog,
  UIProvider,
  LibraryDialog,
  Preview,
  confirmAction,
  requestText,
  allowNavigation,
  registerNavigationGuard,
} = await import(pathToFileURL(output));
const h = React.createElement;
let root;
async function mount(
  props = {},
  child = h(
    "label",
    null,
    "Name",
    h("input", { type: "text", required: true }),
  ),
) {
  document.querySelector("#opener").focus();
  root = createRoot(document.querySelector("#host"));
  await act(async () => {
    root.render(
      h(
        UIProvider,
        null,
        h(
          Dialog,
          {
            title: "Edit record",
            onClose: () => {},
            actions: h("button", null, "Save"),
            ...props,
          },
          child,
        ),
      ),
    );
  });
  await act(() => new Promise((r) => setTimeout(r, 5)));
}
async function click(el) {
  await act(async () => el.click());
}
test.afterEach(async () => {
  if (root) {
    await act(() => root.unmount());
    root = null;
  }
});
test.after(async () => {
  await rm(temp, { recursive: true, force: true });
  dom.window.close();
});

test("dialog has a named heading, scroll body and independent action footer; locks and restores the page", async () => {
  await mount();
  const dialog = document.querySelector("[role=dialog]");
  assert.equal(
    document.getElementById(dialog.getAttribute("aria-labelledby")).textContent,
    "Edit record",
  );
  assert.ok(dialog.querySelector(".ui-dialog-body .ui-dialog-fields input"));
  assert.ok(dialog.querySelector(":scope > .dialog-actions button"));
  assert.equal(document.querySelector("#root").inert, true);
  assert.equal(document.body.style.overflow, "hidden");
  assert.equal(document.activeElement.tagName, "INPUT");
  await act(() => root.unmount());
  root = null;
  assert.equal(document.querySelector("#root").inert, false);
  assert.equal(document.body.style.overflow, "");
  assert.equal(document.activeElement.id, "opener");
});
test("Tab wraps inside the dialog and Escape invokes its close action", async () => {
  let closed = 0;
  await mount({ onClose: () => closed++ });
  const first = document.querySelector(".ui-icon-button"),
    last = document.querySelector(".dialog-actions button");
  last.focus();
  await act(() =>
    document.dispatchEvent(
      new KeyboardEvent("keydown", {
        key: "Tab",
        bubbles: true,
        cancelable: true,
      }),
    ),
  );
  assert.equal(document.activeElement, first);
  await act(() =>
    document.dispatchEvent(
      new KeyboardEvent("keydown", {
        key: "Tab",
        shiftKey: true,
        bubbles: true,
        cancelable: true,
      }),
    ),
  );
  assert.equal(document.activeElement, last);
  await act(() =>
    document.dispatchEvent(
      new KeyboardEvent("keydown", { key: "Escape", bubbles: true }),
    ),
  );
  assert.equal(closed, 1);
});
test("edited dialogs confirm dismissal, cancel preserves edits, and confirmation restores the underlying modal", async () => {
  let closed = 0;
  await mount({ onClose: () => closed++ });
  await act(() =>
    document
      .querySelector("input")
      .dispatchEvent(new Event("input", { bubbles: true })),
  );
  await click(document.querySelector(".ui-icon-button"));
  assert.equal(document.querySelectorAll("[aria-modal=true]").length, 2);
  const confirmation = document.querySelector("[role=alertdialog]");
  assert.match(confirmation.textContent, /Discard the unsaved changes/);
  await click(confirmation.querySelector(".dialog-actions button"));
  assert.equal(closed, 0);
  assert.equal(document.querySelector("[role=dialog]").inert, false);
  await click(document.querySelector(".ui-icon-button"));
  await click(document.querySelector("[role=alertdialog] .danger"));
  assert.equal(closed, 1);
});
test("submissions are serialized and errors remain inside the dialog for retry", async () => {
  let count = 0,
    finish;
  await mount({
    onSubmit: async () => {
      count++;
      await new Promise((r) => (finish = r));
      throw Error("Save unavailable");
    },
  });
  const form = document.querySelector("form");
  await act(() => {
    form.dispatchEvent(
      new Event("submit", { bubbles: true, cancelable: true }),
    );
    form.dispatchEvent(
      new Event("submit", { bubbles: true, cancelable: true }),
    );
  });
  assert.equal(count, 1);
  assert.equal(form.getAttribute("aria-busy"), "true");
  await act(async () => {
    finish();
    await Promise.resolve();
  });
  assert.equal(
    form.querySelector("[role=alert]").textContent,
    "Save unavailable",
  );
  assert.equal(form.getAttribute("aria-busy"), "false");
});
test("feedback requests queue instead of opening overlapping confirmations", async () => {
  await mount();
  let first, second;
  await act(() => {
    first = confirmAction("Delete archive?");
    second = requestText("World name", "A world");
  });
  assert.equal(document.querySelectorAll("[role=alertdialog]").length, 1);
  await click(document.querySelector("[role=alertdialog] .danger"));
  assert.equal(await first, true);
  const prompt = [...document.querySelectorAll("[role=dialog]")].at(-1);
  assert.equal(prompt.querySelector("input").value, "A world");
  await click(prompt.querySelector(".dialog-actions button"));
  assert.equal(await second, null);
});
test("navigation guards await decisions and can be removed without leaking across pages", async () => {
  let calls = 0;
  const unregister = registerNavigationGuard(async () => {
    calls++;
    return false;
  });
  assert.equal(await allowNavigation(), false);
  assert.equal(calls, 1);
  unregister();
  assert.equal(await allowNavigation(), true);
});

test("Library archive creation uses the shared visible modal with labeled fields and explicit actions", async () => {
  root = createRoot(document.querySelector("#host"));
  await act(() =>
    root.render(
      h(
        UIProvider,
        null,
        h(LibraryDialog, {
          dialog: { type: "archive" },
          error: "",
          all: [{ id: "city", name: "Harbor", kind: "city" }],
          archives: [],
          locations: () => [],
          age: { name: "First Age" },
          busy: false,
          close: () => {},
          save: async () => {},
          onAtlas: () => {},
          onOrganizations: () => {},
        }),
      ),
    ),
  );
  const panel = document.querySelector("[role=dialog]");
  assert.ok(panel.closest(".ui-dialog-backdrop"));
  assert.equal(panel.querySelector("h2").textContent, "Create archive folder");
  assert.equal(
    panel.querySelector("select[multiple]").options[0].textContent,
    "Harbor — city",
  );
  assert.ok(panel.querySelector(".dialog-actions button[data-dialog-dismiss]"));
  assert.equal(
    panel.querySelector(".dialog-actions button:last-child").disabled,
    true,
    "Archive owner selection is required before saving",
  );
});
test("Biosphere preview remains in the body with separate habitat groups, not in the action footer", async () => {
  const species = {
    id: "plant",
    name: "Forest oak",
    biology: {
      group: "plant",
      origin: "reference",
      roles: ["producer"],
      traits: {},
    },
  };
  const preview = {
    species: [species],
    state: {
      habitats: [
        { id: "wood", name: "Woodland", description: "Damp woodland" },
      ],
      occurrences: [
        {
          id: "range",
          speciesId: "plant",
          habitatId: "wood",
          abundance: 0.7,
          reason: "Suitable woodland",
        },
      ],
    },
  };
  await mount(
    { title: "Community preview", size: "wizard" },
    h(Preview, {
      preview,
      accepted: ["range"],
      locked: [],
      names: {},
      setAccepted: () => {},
      setLocked: () => {},
      setNames: () => {},
      onRegenerate: () => {},
    }),
  );
  const panel = document.querySelector("[role=dialog]");
  assert.ok(panel.classList.contains("ui-dialog--wizard"));
  assert.equal(
    panel.querySelectorAll(".ui-dialog-body .bio-preview-group").length,
    1,
  );
  assert.equal(
    panel.querySelectorAll(".dialog-actions .bio-preview-group").length,
    0,
  );
  assert.equal(
    panel.querySelector('input[aria-label="Include Forest oak"]').checked,
    true,
  );
});
