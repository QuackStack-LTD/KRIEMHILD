import { chromium } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import assert from "node:assert/strict";
const out =
  process.env.CLIMATE_UI_OUT || "docs/deepzoom-investigation/climate-ui";
const base = process.env.KRIEMHILD_AUDIT_URL || "http://127.0.0.1:8125";
const climateVersion = +(process.env.CLIMATE_UI_VERSION || 2);
await mkdir(out, { recursive: true });
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.KRIEMHILD_BROWSER_EXECUTABLE,
});
try {
  const page = await browser.newPage({
    viewport: { width: 1280, height: 800 },
  });
  let jobs = 0;
  page.on("request", (r) => {
    if (r.method() === "POST" && r.url().endsWith("/terrain/generation-jobs"))
      jobs++;
  });
  await page.goto(`${base}/world/new?tileDebug=1`);
  await page.getByText("Climate & scale", { exact: true }).click();
  await page.getByLabel("Climate model").selectOption(String(climateVersion));
  await page
    .getByLabel("Optional generation seed")
    .fill("d1feae98-6510-4a72-951b-44e93a01ae86");
  const request = page.waitForRequest(
    (r) =>
      r.method() === "POST" && r.url().endsWith("/terrain/generation-jobs"),
  );
  await page
    .getByRole("button", { name: "Generate preview", exact: true })
    .first()
    .click();
  assert.equal(
    (await request).postDataJSON().spec.climateVersion,
    climateVersion,
  );
  await page
    .getByRole("button", { name: "Accept terrain", exact: true })
    .waitFor({ timeout: 180000 });
  await page.waitForSelector('[data-phase="final"]', { timeout: 180000 });
  const session = await page.evaluate(
    () =>
      window.__terrainViewportLog.find((e) => e.event === "request").session,
  );
  const before = await (
    await page.request.get(
      `${base}/api/sessions/${session}/terrain/sample?lon=54&lat=50.4`,
    )
  ).json();
  await writeFile(`${out}/pending-accept.json`,JSON.stringify({session,before,climateVersion},null,2));
  const acceptance=page.waitForResponse(r=>r.request().method()==='POST'&&r.url().endsWith('/accept'),{timeout:180000});
  await page
    .getByRole("button", { name: "Accept terrain", exact: true })
    .click();
  const accepted=await acceptance;
  if(!accepted.ok()){
    const message=await accepted.text();
    await writeFile(`${out}/accept-error.json`,JSON.stringify({status:accepted.status(),message},null,2));
    await page.screenshot({path:`${out}/accept-error.png`});
    throw Error(`Accept terrain HTTP ${accepted.status()}: ${message}`);
  }
  await page.waitForURL(/world\/[a-f0-9]+\/build/, { timeout: 120000 });
  const url = page.url();
  await page.reload();
  await page.waitForSelector('[data-phase="final"]', { timeout: 180000 });
  const restored = await page.evaluate(
    () =>
      window.__terrainViewportLog.find((e) => e.event === "request").session,
  );
  const state = await (
    await page.request.get(`${base}/api/sessions/${restored}/world/state`)
  ).json();
  const after = await (
    await page.request.get(
      `${base}/api/sessions/${restored}/terrain/sample?lon=54&lat=50.4`,
    )
  ).json();
  assert.equal(state.continuous.spec.climateVersion, climateVersion);
  assert.equal(jobs, 1);
  assert.deepEqual(after, before);
  await page
    .locator("select")
    .filter({ hasText: "watersheds" })
    .selectOption("biome");
  await page.waitForFunction(() =>
    document.querySelector("footer")?.innerText.endsWith("Ready"),
  );
  await page.screenshot({ path: `${out}/reopened-biomes.png` });
  await writeFile(
    `${out}/results.json`,
    JSON.stringify(
      {
        url,
        world: state.worldId,
        spec: state.continuous.spec,
        jobs,
        before,
        after,
      },
      null,
      2,
    ),
  );
  console.log(url);
} finally {
  await browser.close();
}
