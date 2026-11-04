import { chromium } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import assert from "node:assert/strict";
const base = process.env.KRIEMHILD_AUDIT_URL || "http://127.0.0.1:8125",
  out = "docs/terrain-investigation/browser";
await mkdir(out, { recursive: true });
const browser = await chromium.launch({
  headless: true,
  ...(process.env.KRIEMHILD_BROWSER_EXECUTABLE
    ? { executablePath: process.env.KRIEMHILD_BROWSER_EXECUTABLE }
    : {}),
});
const page = await browser.newPage({ viewport: { width: 1280, height: 800 } }),
  errors = [];
page.on("pageerror", (e) => errors.push(e.message));
page.on("dialog", (d) => d.accept());
try {
  await page.goto(base + "/world/new?tileDebug=1");
  await page.getByLabel("Optional generation seed").fill("Kriemhild");
  const start = Date.now();
  await page
    .getByRole("button", { name: "Generate preview", exact: true })
    .first()
    .click();
  await page
    .getByRole("button", { name: "Accept terrain", exact: true })
    .waitFor({ timeout: 180000 });
  await page.waitForFunction(
    () => document.querySelector("footer")?.innerText.endsWith("Ready"),
    null,
    { timeout: 90000 },
  );
  const session = await page.evaluate(
    () => window.__terrainTileLog.find((e) => e.url).url.split("/")[3],
  );
  let state = await (
    await page.request.get(`${base}/api/sessions/${session}/world/state`)
  ).json();
  assert.equal(state.continuous.spec.climateVersion, 1);
  assert.equal(state.continuous.spec.surfaceWidth, 4096);
  const accepted = page.waitForResponse((r) => r.url().endsWith("/accept"), {
    timeout: 120000,
  });
  await page
    .getByRole("button", { name: "Accept terrain", exact: true })
    .click();
  assert.equal((await accepted).status(), 201);
  await page.getByLabel("Style").selectOption("biome", { timeout: 90000 });
  state = await (
    await page.request.get(`${base}/api/sessions/${session}/world/state`)
  ).json();
  await page.waitForFunction(
    () => document.querySelector("footer")?.innerText.endsWith("Ready"),
    null,
    { timeout: 90000 },
  );
  await page.waitForLoadState("networkidle");
  await page.screenshot({ path: out + "/16-new-climate-world.png" });
  const saved = page.waitForResponse((r) => r.url().endsWith("/save"));
  await page.getByRole("button", { name: "Save", exact: true }).click();
  assert.equal((await saved).status(), 200);
  await page.goto(`${base}/world/${state.worldId}/build?tileDebug=1`);
  await page.waitForSelector("image", { timeout: 90000 });
  await page.waitForLoadState("networkidle");
  assert.deepEqual(errors, []);
  await writeFile(
    out + "/new-climate-world.json",
    JSON.stringify(
      {
        session,
        world: state.worldId,
        spec: state.continuous.spec,
        elapsedMs: Date.now() - start,
        errors,
        reopened: true,
      },
      null,
      2,
    ),
  );
  console.log(
    "Generated, saved and reopened full-resolution climate version 1 world",
    state.worldId,
  );
} finally {
  await browser.close();
}
