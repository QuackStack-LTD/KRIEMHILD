import { chromium } from "@playwright/test";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import assert from "node:assert/strict";
import { build } from "esbuild";
const out =
  process.env.PREVIEW_AUDIT_OUT ||
  "docs/preview-climate-investigation/preview-headless";
const base = process.env.KRIEMHILD_AUDIT_URL || "http://127.0.0.1:8127";
await mkdir(out, { recursive: true });
const auditScript = (
  await build({
    entryPoints: ["tests/preview-quality-benchmark.js"],
    bundle: true,
    format: "iife",
    write: false,
  })
).outputFiles[0].text;
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.KRIEMHILD_BROWSER_EXECUTABLE,
});
try {
  for (const dpr of [1, 2, 3]) {
    const page = await browser.newPage({
      viewport: { width: 1280, height: 800 },
      deviceScaleFactor: dpr,
    });
    await page.route("**/quality-benchmark-empty", (route) =>
      route.fulfill({
        contentType: "text/html",
        body: "<!doctype html><title>Isolated preview benchmark</title>",
      }),
    );
    await page.goto(base + "/quality-benchmark-empty");
    await page.addScriptTag({ content: auditScript });
    await page.waitForFunction(() => window.runQualityAudit);
    await page.evaluate(
      async ({ dpr, repeats, presets, styles }) => {
        const s = await (
          await fetch("/api/projects/977d134ee5aadc480e1410ade3c84587/open", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: "{}",
          })
        ).json();
        window.runQualityAudit({
          session: s.id,
          revision: s.project.revision,
          dprs: [dpr],
          repeats,
          presets,
          styles,
        });
      },
      {
        dpr,
        repeats: +(process.env.PREVIEW_REPEATS || 2),
        presets: (
          process.env.PREVIEW_PRESETS || "reference,performance,balanced,high"
        ).split(","),
        styles: (process.env.PREVIEW_STYLES || "natural,hillshade").split(","),
      },
    );
    while (await page.evaluate(() => qualityAudit.status === "running")) {
      await page.waitForTimeout(1000);
      const progress = await page.evaluate(() => ({
        status: qualityAudit.status,
        count: qualityAudit.runs.length,
        latest: qualityAudit.latest,
      }));
      if (progress.count % 10 === 0) console.log(dpr, progress);
    }
    const result = await page.evaluate(() => qualityAudit);
    await writeFile(`${out}/dpr-${dpr}.json`, JSON.stringify(result, null, 2));
    assert.equal(result.status, "complete", result.error);
    assert.ok(result.runs.every((r) => r.actualDeviceDPR === dpr));
    console.log(`DPR ${dpr}: ${result.runs.length} renders complete`);
    await page.close();
  }
  const page = await browser.newPage({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 3,
  });
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto(
    (process.env.PREVIEW_UI_URL || base) +
      "/world/977d134ee5aadc480e1410ade3c84587/build?tileDebug=1",
  );
  await page.waitForSelector('[data-phase="final"]', { timeout: 180000 });
  await page.evaluate(await readFile("tests/preview-quality-ui.js", "utf8"));
  await page.evaluate(() => {
    runPreviewUIAudit();
  });
  await page.waitForFunction(() => previewUIAudit.status !== "running", null, {
    timeout: 240000,
  });
  const result = await page.evaluate(() => previewUIAudit);
  await writeFile(
    `${out}/ui.json`,
    JSON.stringify({ result, errors }, null, 2),
  );
  await page.screenshot({ path: `${out}/illustrator.png` });
  assert.equal(result.status, "complete", result.error);
  assert.equal(errors.length, 0, errors.join("\n"));
  console.log("UI preset/camera/revision/Illustrator checks complete");
} finally {
  await browser.close();
}
