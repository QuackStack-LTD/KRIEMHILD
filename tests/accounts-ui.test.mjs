import test from "node:test";
import assert from "node:assert/strict";
import { JSDOM } from "jsdom";
import { build } from "esbuild";
import { mkdtemp, rm } from "node:fs/promises";
import { resolve, join } from "node:path";
import { pathToFileURL } from "node:url";
import React, { act } from "react";

const dom = new JSDOM('<div id="host"></div>', { url: "http://localhost" });
for (const key of [
  "window",
  "document",
  "HTMLElement",
  "Event",
  "KeyboardEvent",
  "MouseEvent",
])
  globalThis[key] = dom.window[key];
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
globalThis.requestAnimationFrame = (fn) => setTimeout(fn, 0);
globalThis.cancelAnimationFrame = clearTimeout;
dom.window.HTMLElement.prototype.getClientRects = () => [
  { width: 100, height: 30 },
];
const { createRoot } = await import("react-dom/client");
const temp = await mkdtemp(resolve(".tools/account-ui-")),
  output = join(temp, "components.mjs");
await build({
  stdin: {
    contents: `export {default as AccountSecurity} from './src/auth/AccountSecurity.jsx'; export {default as UIProvider} from './src/worldbuilding/ui/UIProvider.jsx';`,
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
const { AccountSecurity, UIProvider } = await import(pathToFileURL(output));
test.after(async () => {
  dom.window.close();
  await rm(temp, { recursive: true, force: true });
});
const h = React.createElement;
test("account closure requires ownership transfer and does not send a destructive request", async () => {
  const requests = [];
  globalThis.fetch = async (path, init) => {
    requests.push([path, init.method]);
    return new Response(
      JSON.stringify({
        ownedWorlds: [{ id: "world", name: "My shared world" }],
        activeSessions: 2,
      }),
      { status: 200 },
    );
  };
  const root = createRoot(document.getElementById("host"));
  try {
    await act(async () => {
      root.render(h(UIProvider, null, h(AccountSecurity, { mode: "oidc" })));
    });
    assert.match(document.body.textContent, /My shared world/);
    const close = [...document.querySelectorAll("button")].find(
      (b) => b.textContent === "Close account",
    );
    assert.equal(close.disabled, true);
    assert.deepEqual(requests, [["/api/account", "GET"]]);
  } finally {
    await act(async () => root.unmount());
  }
});
test("sign out everywhere waits for confirmation and expires the application session", async () => {
  const requests = [];
  let expired = false;
  const onExpired = () => {
    expired = true;
  };
  window.addEventListener("session-expired", onExpired);
  globalThis.fetch = async (path, init) => {
    requests.push([path, init.method]);
    return path === "/api/account"
      ? new Response(JSON.stringify({ ownedWorlds: [], activeSessions: 2 }), {
          status: 200,
        })
      : new Response(null, { status: 204 });
  };
  const root = createRoot(document.getElementById("host"));
  try {
    await act(async () => {
      root.render(h(UIProvider, null, h(AccountSecurity, { mode: "oidc" })));
    });
    await act(async () => {
      [...document.querySelectorAll("button")]
        .find((b) => b.textContent === "Sign out everywhere")
        .click();
    });
    assert.equal(requests.length, 1);
    assert.equal(expired, false);
    const dialog = document.querySelector('[role="alertdialog"]');
    assert.ok(dialog);
    await act(async () => {
      [...dialog.querySelectorAll("button")]
        .find((b) => b.textContent === "Confirm")
        .click();
    });
    assert.deepEqual(requests.at(-1), ["/api/account/sessions/revoke", "POST"]);
    assert.equal(expired, true);
  } finally {
    window.removeEventListener("session-expired", onExpired);
    await act(async () => root.unmount());
  }
});
