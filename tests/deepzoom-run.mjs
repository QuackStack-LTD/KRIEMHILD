import { chromium } from "@playwright/test";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import assert from "node:assert/strict";
const out =
  process.env.DEEPZOOM_OUT || "docs/deepzoom-investigation/browser-measured";
const base = process.env.KRIEMHILD_AUDIT_URL || "http://127.0.0.1:8125";
const world =
  process.env.KRIEMHILD_AUDIT_WORLD || "977d134ee5aadc480e1410ade3c84587";
const repetitions = +(process.env.DEEPZOOM_REPEATS || 3);
await mkdir(out, { recursive: true });
const recorder = await readFile("tests/deepzoom-browser.js", "utf8");
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.KRIEMHILD_BROWSER_EXECUTABLE,
});
const runs = [];
try {
  for (let repeat = 0; repeat < repetitions; repeat++) {
    const page = await browser.newPage({
      viewport: { width: 1280, height: 800 },
      deviceScaleFactor: +(process.env.DEEPZOOM_DPR || 1),
    });
    const errors = [];
    page.on("pageerror", (e) => errors.push(e.message));
    await page.goto(`${base}/world/${world}/build?tileDebug=1`);
    await page.waitForSelector('[data-phase="final"]', { timeout: 180000 });
    await page
      .getByLabel("Autosave accepted terrain after 30 seconds")
      .uncheck();
    await page
      .locator("select")
      .filter({ hasText: "watersheds" })
      .selectOption("natural");
    await page.getByLabel("Hillshade strength").fill("0.55");
    await page.evaluate(recorder);
    await page.evaluate(() => performance.setResourceTimingBufferSize(10000));
    for (const cache of [
      "browser-cold/backend-reused",
      "browser-warm/backend-reused",
    ]) {
      const start = await page.evaluate(() => window.deepzoomResults.length);
      for (const [name, width] of [
        ["global", 2],
        ["continental", 0.25],
        ["regional", 0.03],
        ["close", 0.003],
        ["deep", 0.0003],
        ["extreme", 0.00008],
        ["out", 0.03],
        ["return", 0.00008],
      ]) {
        const state = await page.evaluate(
          ([n, w]) => window.deepzoomStep(n, w),
          [name, width],
        );
        console.log(repeat, cache, name, Math.round(state.duration), state.lod);
        assert.match(state.footer, /Ready|Maximum available/);
        if (repeat === 0 && cache.startsWith("browser-cold"))
          await page.screenshot({ path: `${out}/${name}.png` });
      }
      const stages = await page.evaluate(
        (start) => window.deepzoomResults.slice(start),
        start,
      );
      for (const s of stages) {
        assert.equal(s.images.length, 2);
        assert.deepEqual(s.images[0].bounds, s.images[1].bounds);
        assert.equal(s.images[0].generation, s.images[1].generation);
        assert.equal(s.images[0].revision, s.images[1].revision);
      }
      runs.push({ repeat, cache, world, errors, stages });
      await writeFile(`${out}/results.json`, JSON.stringify(runs, null, 2));
    }
    await page.close();
  }
} finally {
  await browser.close();
}
