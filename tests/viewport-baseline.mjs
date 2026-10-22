import { chromium } from "@playwright/test";
import { mkdir, writeFile, readFile } from "node:fs/promises";
import assert from "node:assert/strict";
const out = "docs/viewport-investigation/baseline";
await mkdir(out, { recursive: true });
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.KRIEMHILD_BROWSER_EXECUTABLE,
});
const page = await browser.newPage({ viewport: { width: 1280, height: 800 } }),
  requests = [],
  errors = [];
page.on("pageerror", (e) => errors.push(e.message));
page.on("dialog", (d) => d.accept());
const starts = new WeakMap();
page.on("request", (r) => {
  if (r.url().includes("/terrain/tiles/")) starts.set(r, Date.now());
});
page.on("requestfailed", (r) => {
  if (starts.has(r))
    requests.push({
      url: r.url(),
      start: starts.get(r),
      end: Date.now(),
      error: r.failure()?.errorText,
    });
});
page.on("response", async (r) => {
  if (starts.has(r.request())) {
    const e = {
      url: r.url(),
      start: starts.get(r.request()),
      end: Date.now(),
      status: r.status(),
      headers: r.headers(),
    };
    requests.push(e);
    try {
      e.bytes = (await r.body()).length;
    } catch {
      e.abortedBody = true;
    }
  }
});
const settle = async () => {
  await page.waitForFunction(
    () => document.querySelector("footer")?.innerText.endsWith("Ready"),
    null,
    { timeout: 180000 },
  );
  await page.waitForLoadState("networkidle", { timeout: 180000 });
};
const states = [];
async function capture(name, wait = true) {
  if (wait) await settle();
  const state = await page.evaluate(() => {
    const s = document.querySelector(".studio-stage svg"),
      r = s.getBoundingClientRect(),
      inv = s.getScreenCTM().inverse();
    const groups = [...s.querySelectorAll("[data-terrain-style]")].map((g) => {
      const tiles = [...g.querySelectorAll("image")].map((e) => ({
        tile: e.dataset.tile,
        revision: e.dataset.revision,
        x: +e.getAttribute("x"),
        y: +e.getAttribute("y"),
        span: +e.getAttribute("width"),
      }));
      const z = Math.max(...tiles.map((t) => +t.tile?.split("/")[0]));
      let missing = 0;
      for (let y = 1; y < r.height; y += 17)
        for (let x = 1; x < r.width; x += 17) {
          const p = new DOMPoint(r.x + x, r.y + y).matrixTransform(inv);
          if (p.x < 0 || p.x >= 2 || p.y < 0 || p.y >= 1) continue;
          if (
            !tiles.some(
              (t) =>
                +t.tile?.split("/")[0] === z &&
                p.x >= t.x &&
                p.x < t.x + t.span &&
                p.y >= t.y &&
                p.y < t.y + t.span,
            )
          )
            missing++;
        }
      return { style: g.dataset.terrainStyle, z, missing, tiles };
    });
    return {
      camera: s.getAttribute("viewBox"),
      rect: r.toJSON(),
      dpr: devicePixelRatio,
      groups,
      footer: document.querySelector("footer").innerText,
      log: window.__terrainTileLog || [],
    };
  });
  states.push({ name, ...state });
  await page.screenshot({ path: `${out}/${name}.png` });
  console.log(
    name,
    state.groups.map((g) => [g.style, g.z, g.missing]),
    state.footer.split("\n").at(-1),
  );
  return state;
}
async function zoom(n, delay = 300, delta = -266) {
  const r = await page.locator(".studio-stage svg").boundingBox();
  await page.mouse.move(r.x + r.width * 0.42, r.y + r.height * 0.4);
  for (let i = 0; i < n; i++) {
    await page.mouse.wheel(0, delta);
    await page.waitForTimeout(delay);
  }
}
let spec, world, session;
try {
  const old = process.env.AUDIT_OLD === "1";
  await page.goto(
    old
      ? "http://127.0.0.1:8126/world/ae7e539f0fba71dad0417df2186d0ac8/build?tileDebug=1&terrainRenderer=tiles"
      : "http://127.0.0.1:8125/world/new?tileDebug=1&terrainRenderer=tiles",
  );
  if (!old) {
    await page.getByLabel("Optional generation seed").fill("Kriemhild");
    await page
      .getByRole("button", { name: "Generate preview", exact: true })
      .first()
      .click();
    await page
      .getByRole("button", { name: "Accept terrain", exact: true })
      .waitFor({ timeout: 180000 });
  }
  await page.waitForSelector("image", { timeout: 180000 });
  await settle();
  session = await page.evaluate(
    () => window.__terrainTileLog.find((e) => e.url).url.split("/")[3],
  );
  let state = await (
    await page.request.get(
      new URL(`/api/sessions/${session}/world/state`, page.url()).href,
    )
  ).json();
  spec = state.continuous.spec;
  if (!old) {
    const accepted = page.waitForResponse((r) => r.url().endsWith("/accept"), {
      timeout: 120000,
    });
    await page
      .getByRole("button", { name: "Accept terrain", exact: true })
      .click();
    assert.equal((await accepted).status(), 201);
    await page
      .getByRole("button", { name: "Map Builder", exact: true })
      .waitFor();
    await settle();
    state = await (
      await page.request.get(
        new URL(`/api/sessions/${session}/world/state`, page.url()).href,
      )
    ).json();
  }
  world = state.worldId;
  const prefix = old ? "old" : "new";
  await page.getByRole("button", { name: "Fit world", exact: true }).click();
  await capture(prefix + "-00-global");
  for (let i = 1; i <= 3; i++) {
    await zoom(2);
    await capture(prefix + `-0${i}-zoom`);
  }
  // Hold an independently loaded shading child after the base terrain finishes.
  let release;
  const held = new Promise((r) => (release = r));
  let heldURL;
  await page.route("**/terrain/tiles/**", async (route) => {
    const u = route.request().url();
    if (!heldURL && u.includes("style=hillshade") && /tiles\/[6-9]\//.test(u)) {
      heldURL = u;
      await held;
    }
    await route.continue();
  });
  await zoom(2, 30);
  await page.waitForFunction(
    () => document.querySelector("footer").innerText.endsWith("Ready"),
    null,
    { timeout: 180000 },
  );
  await capture(prefix + "-04-delayed-shading", false);
  release();
  await page.unrouteAll({ behavior: "wait" });
  await capture(prefix + "-05-complete");
  await zoom(3, 30);
  await capture(prefix + "-06-close");
  const r = await page.locator(".studio-stage svg").boundingBox();
  await page.getByRole("button", { name: /^inspect$/i }).click();
  await page.mouse.move(r.x + 40, r.y + 40);
  await page.mouse.down();
  await page.mouse.move(r.x + 160, r.y + 150, { steps: 5 });
  await page.mouse.up();
  await capture(prefix + "-07-pan");
  await zoom(3, 30, 266);
  await capture(prefix + "-08-out");
  await zoom(3, 30);
  await capture(prefix + "-09-return");
  if (!old) {
    const saved = page.waitForResponse((r) => r.url().endsWith("/save"));
    await page.getByRole("button", { name: "Save", exact: true }).click();
    assert.equal((await saved).status(), 200);
  }
  await page.reload();
  await page.waitForSelector("image", { timeout: 120000 });
  await capture(prefix + "-10-reload");
  await writeFile(
    `${out}/${prefix}.json`,
    JSON.stringify(
      {
        commit: "2e4f5e4",
        spec,
        world,
        session,
        states,
        requests,
        errors,
        heldURL,
      },
      null,
      2,
    ),
  );
  assert.deepEqual(errors, []);
} finally {
  await writeFile(
    `${out}/${process.env.AUDIT_OLD === "1" ? "old" : "new"}-partial.json`,
    JSON.stringify({ spec, world, session, states, requests, errors }, null, 2),
  );
  await browser.close();
}
