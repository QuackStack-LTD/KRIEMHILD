import { chromium } from "@playwright/test";
import { readFile, writeFile } from "node:fs/promises";
import assert from "node:assert/strict";
const fixture = JSON.parse(
  await readFile(
    process.env.CLIMATE_UI_FIXTURE ||
      "docs/deepzoom-investigation/climate-ui/results.json",
    "utf8",
  ),
);
const base = new URL(fixture.url).origin;
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
  await page.goto(fixture.url);
  await page.waitForSelector('[data-phase="final"]', { timeout: 180000 });
  const session = await page.evaluate(
    () =>
      window.__terrainViewportLog.find((e) => e.event === "request").session,
  );
  const state = await (
    await page.request.get(`${base}/api/sessions/${session}/world/state`)
  ).json();
  const sample = await (
    await page.request.get(
      `${base}/api/sessions/${session}/terrain/sample?lon=54&lat=50.4`,
    )
  ).json();
  assert.equal(
    state.continuous.spec.climateVersion,
    fixture.spec.climateVersion,
  );
  assert.equal(jobs, 0);
  assert.deepEqual(sample, fixture.after);
  await writeFile(
    process.env.CLIMATE_UI_COLD_OUT ||
      "docs/deepzoom-investigation/climate-ui/cold-restart.json",
    JSON.stringify(
      { world: state.worldId, spec: state.continuous.spec, jobs, sample },
      null,
      2,
    ),
  );
} finally {
  await browser.close();
}
