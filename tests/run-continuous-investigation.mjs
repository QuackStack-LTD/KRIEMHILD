// Use only a disposable investigation server. Set KRIEMHILD_AUDIT_URL and
// KRIEMHILD_AUDIT_WORLD to an already generated/saved fixed-seed world.
import { chromium } from "@playwright/test";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import assert from "node:assert/strict";
const base = process.env.KRIEMHILD_AUDIT_URL || "http://127.0.0.1:8125";
const world =
  process.env.KRIEMHILD_AUDIT_WORLD || "47d7f3ae6edf78298c34a8b8f334ae6f";
const out = "docs/terrain-investigation/browser";
await mkdir(out, { recursive: true });
const browser = await chromium.launch({
  headless: true,
  ...(process.env.KRIEMHILD_BROWSER_EXECUTABLE
    ? { executablePath: process.env.KRIEMHILD_BROWSER_EXECUTABLE }
    : {}),
});
const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
const requests = [],
  errors = [],
  stages = [];
page.on("pageerror", (e) => errors.push(e.message));
page.on("dialog", (d) => d.accept());
const started = new WeakMap();
page.on("request", (r) => {
  if (r.url().includes("/terrain/tiles/")) started.set(r, Date.now());
});
page.on("requestfailed", (r) => {
  if (started.has(r))
    requests.push({
      url: r.url(),
      start: started.get(r),
      end: Date.now(),
      error: r.failure()?.errorText,
    });
});
page.on("response", async (r) => {
  if (!started.has(r.request())) return;
  const e = {
    url: r.url(),
    start: started.get(r.request()),
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
});
const helper = await readFile("tests/continuous-investigation.js", "utf8");
async function settled() {
  await page.waitForFunction(
    () => document.querySelector("footer")?.innerText.endsWith("Ready"),
    null,
    { timeout: 90000 },
  );
  await page.waitForLoadState("networkidle", { timeout: 90000 });
}
async function stage(name) {
  await settled();
  const state = await page.evaluate(() => {
    const s = terrainAudit.state();
    delete s.log;
    return s;
  });
  assert.equal(state.missing, 0, `${name}: uncovered visible terrain`);
  stages.push({ name, ...state });
  await page.screenshot({ path: `${out}/${name}.png` });
  console.log(name, state.view, "covered");
  return state;
}
async function load() {
  await page.goto(`${base}/world/${world}/build?tileDebug=1`);
  await page.waitForSelector('g[data-terrain-style="natural"] image', {
    timeout: 90000,
  });
  await page.evaluate(helper);
  await settled();
}
async function zoom(n, delay = 700, delta = -266) {
  const r = await page.locator(".studio-stage svg").boundingBox();
  await page.mouse.move(r.x + r.width * 0.42, r.y + r.height * 0.4);
  for (let i = 0; i < n; i++) {
    await page.mouse.wheel(0, delta);
    await page.waitForTimeout(delay);
  }
}
async function pan(dx, dy) {
  await page.getByRole("button", { name: /^inspect$/i }).click();
  const r = await page.locator(".studio-stage svg").boundingBox();
  await page.mouse.move(r.x + 50, r.y + 50);
  await page.mouse.down();
  await page.mouse.move(r.x + 50 + dx, r.y + 50 + dy, { steps: 5 });
  await page.mouse.up();
}
let session, placement, edit, settlement;
try {
  await load();
  session = await page.evaluate(
    () => window.__terrainTileLog.find((e) => e.url).url.split("/")[3],
  );
  await page.getByRole("button", { name: "Fit world", exact: true }).click();
  await stage("01-fit");
  await zoom(6);
  await stage("02-gradual");
  let failure;
  await page.route("**/terrain/tiles/**", async (route) => {
    const url = route.request().url();
    if (!failure && url.includes("style=natural") && /tiles\/7\//.test(url)) {
      failure = url;
      await route.fulfill({ status: 503, body: "temporary fixture failure" });
      return;
    }
    await route.continue();
  });
  await zoom(4, 30);
  await stage("03-rapid-retry");
  assert.ok(failure, "failure fixture never activated");
  assert.ok(
    requests.some((r) => r.url === failure && r.status === 200),
    "failed tile did not recover",
  );
  await page.unroute("**/terrain/tiles/**");
  await pan(160, 0);
  await stage("04-pan-horizontal");
  await pan(0, 160);
  await stage("05-pan-vertical");
  await zoom(4, 30, 266);
  await stage("06-zoom-out");
  await zoom(4, 30);
  await stage("07-return");
  await page.getByRole("button", { name: "Map Builder", exact: true }).click();
  await page.getByRole("button", { name: /^landmark$/i }).click();
  let r = await page.locator(".studio-stage svg").boundingBox();
  const click = [r.x + r.width * 0.58, r.y + r.height * 0.55];
  const state = await (
    await page.request.get(`${base}/api/sessions/${session}/world/state`)
  ).json();
  const expected = await page.evaluate(([x, y]) => {
    const p = new DOMPoint(x, y).matrixTransform(
      document.querySelector(".studio-stage svg").getScreenCTM().inverse(),
    );
    return [p.x, p.y];
  }, click);
  await page.mouse.click(...click);
  await settled();
  const entities = await (
    await page.request.get(
      `${base}/api/sessions/${session}/world/entities?all=1&detail=8`,
    )
  ).json();
  const entity = entities.entities.find(
    (e) =>
      e.kind === "landmark" &&
      Math.abs((e.points[0][0] * 2) / (state.W - 1) - expected[0]) < 1e-5,
  );
  assert.ok(entity, "placed landmark not at pointer geography");
  placement = { expected, entity, projectSize: [state.W, state.H] };
  await zoom(2, 40);
  await pan(80, 60);
  await stage("08-map-builder-placement");
  const entitiesAfter = await (
    await page.request.get(
      `${base}/api/sessions/${session}/world/entities?all=1&detail=8`,
    )
  ).json();
  assert.deepEqual(
    entitiesAfter.entities.find((e) => e.id === entity.id).points,
    entity.points,
  );
  await page.getByRole("button", { name: /^raise$/i }).click();
  r = await page.locator(".studio-stage svg").boundingBox();
  const centre = [r.x + r.width * 0.5, r.y + r.height * 0.5];
  const map = await page.evaluate(([x, y]) => {
    const p = new DOMPoint(x, y).matrixTransform(
      document.querySelector(".studio-stage svg").getScreenCTM().inverse(),
    );
    return [p.x, p.y];
  }, centre);
  const sampleURL = `${base}/api/sessions/${session}/terrain/sample?lon=${map[0] * 180 - 180}&lat=${90 - map[1] * 180}`;
  const before = await (await page.request.get(sampleURL)).json();
  await page.mouse.click(...centre);
  await settled();
  const after = await (await page.request.get(sampleURL)).json();
  assert.ok(
    after.bed > before.bed + 250,
    "raise did not affect selected geography",
  );
  edit = { map, before, after };
  await stage("09-terrain-edit");
  await page.getByRole("button", { name: "Undo", exact: true }).click();
  await settled();
  const undone = await (await page.request.get(sampleURL)).json();
  assert.ok(Math.abs(undone.bed - before.bed) < 1e-6);
  edit.undone = undone;
  await stage("10-undo");
  await page
    .getByRole("button", { name: "Settlement Builder", exact: true })
    .click();
  await stage("11-settlement-builder");
  // Selecting a site invokes the actual preview and centres its local camera.
  const site = page.getByRole("button", { name: "Choose site", exact: true });
  if (await site.count()) await site.click();
  await writeFile(
    `${out}/settlement-controls.txt`,
    await page.locator("body").innerText(),
  );
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await settled();
  await page.reload();
  await page.waitForSelector('g[data-terrain-style="natural"] image', {
    timeout: 90000,
  });
  await page.evaluate(helper);
  await stage("12-reload");
  await page.getByRole("button", { name: "Fit world", exact: true }).click();
  await settled();
  await zoom(6);
  await stage("13-repeat-after-reload");
  assert.deepEqual(errors, []);
} finally {
  await writeFile(
    `${out}/audit.json`,
    JSON.stringify(
      {
        base,
        world,
        session,
        stages,
        placement,
        edit,
        settlement,
        errors,
        requests,
      },
      null,
      2,
    ),
  );
  await browser.close();
}
