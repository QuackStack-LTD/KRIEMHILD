// Disposable local server only. Used when the collaborative preview explicitly
// reports that its automation host is unavailable.
import { chromium } from "@playwright/test";
import fs from "node:fs/promises";
import path from "node:path";
import crypto from "node:crypto";
import assert from "node:assert/strict";

const origin = process.env.KRIEMHILD_ATLAS_URL || "http://127.0.0.1:8149";
const output = path.resolve(".cache/atlas-browser");
await fs.mkdir(output, { recursive: true });
const browser = await chromium.launch({ channel: "chrome", headless: true });
const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
page.setDefaultTimeout(30000);
const report = { origin, errors: [], stages: [] };
page.on("pageerror", (error) => report.errors.push(error.message));
const ready = async () => {
  await page.waitForFunction(
    () => {
      const request = window.__terrainViewportLog?.findLast(
        (e) => e.event === "request",
      );
      const frame = document.querySelector(
        '[aria-label="Committed terrain viewport"]',
      );
      return (
        frame?.dataset.phase === "final" &&
        +frame.dataset.generation === request?.generation
      );
    },
    undefined,
    { timeout: 180000 },
  );
};
const camera = async (value) => {
  await page.evaluate(
    (value) =>
      window.dispatchEvent(
        new CustomEvent("terrain-diagnostic-camera", { detail: value }),
      ),
    value,
  );
  await page.waitForTimeout(150);
  await ready();
};
const capture = async (name) => {
  report.stages.push(
    await page.evaluate(
      (name) => ({
        name,
        camera: document
          .querySelector(".studio-stage svg")
          .getAttribute("viewBox"),
        pixels: {
          ...document.querySelector('[aria-label="Committed terrain viewport"]')
            .dataset,
        },
        frames: window.__terrainViewportLog
          .filter((e) => e.event === "frame")
          .slice(-2),
        rivers: document.querySelectorAll("path[data-river-id]").length,
        resolution: document.querySelector(
          '[aria-label="Effective preview resolution"]',
        )?.textContent,
      }),
      name,
    ),
  );
  await page.screenshot({
    path: path.join(output, name + ".png"),
    fullPage: false,
  });
};
const exportSVG = async (name) => {
  const download = page.waitForEvent("download");
  await page.getByRole("button", { name: "SVG", exact: true }).click();
  const file = path.join(output, name + ".svg");
  await (await download).saveAs(file);
  return crypto
    .createHash("sha256")
    .update(await fs.readFile(file))
    .digest("hex");
};
try {
  await page.goto(origin + "/world/new?tileDebug=1");
  await page.getByLabel("Optional generation seed").fill("Kriemhild");
  await page
    .locator("label")
    .filter({ hasText: "Terrain detail" })
    .locator("select")
    .selectOption("1024");
  await page
    .getByRole("button", { name: "Generate preview", exact: true })
    .first()
    .click();
  await ready();
  await page.waitForSelector("path[data-river-id]");
  const session = await page.evaluate(
    () =>
      window.__terrainViewportLog.findLast((e) => e.event === "request")
        .session,
  );
  report.session = session;
  const getState = () =>
    page.request
      .get(origin + "/api/sessions/" + session + "/world/state")
      .then((r) => r.json());
  const before = await getState();
  const quality = page.getByLabel("Preview Quality", { exact: true });
  await page.getByRole("button", { name: "Atlas", exact: true }).click();
  await capture("world");
  for (const [name, w] of [
    ["region", 0.4],
    ["local", 0.05],
    ["editing", 0.005],
  ]) {
    await camera({ x: 1.3 - w / 2, y: 0.22 - w / 4, w, h: w / 2 });
    await capture(name);
  }
  await camera({ x: 0, y: 0, w: 2, h: 1 });
  await page.getByRole("button", { name: "Map Builder", exact: true }).click();
  const autosave = page.getByLabel(
    "Autosave accepted terrain after 30 seconds",
  );
  if (await autosave.isChecked()) await autosave.uncheck();
  await page
    .locator("label")
    .filter({ hasText: /^Export width/ })
    .locator("select")
    .selectOption("1024");
  await quality.selectOption("balanced");
  await page.waitForTimeout(150);
  await ready();
  const balancedSVG = await exportSVG("builder-balanced");
  await quality.selectOption("performance");
  await page.waitForTimeout(150);
  await ready();
  const fastSVG = await exportSVG("builder-fast");
  assert.equal(
    fastSVG,
    balancedSVG,
    "builder exports depend on preview quality",
  );
  report.builderExportSHA256 = balancedSVG;
  const after = await getState();
  assert.deepEqual(after.continuous, before.continuous);
  assert.equal(after.project.revision, before.project.revision);
  report.previewPreservedSource = true;

  let held = 0;
  await page.route("**/terrain/tiles/**", async (route) => {
    held++;
    await new Promise((r) => setTimeout(r, 600));
    await route.continue();
  });
  await page.evaluate(() =>
    window.dispatchEvent(
      new CustomEvent("terrain-diagnostic-camera", {
        detail: { x: 1.25, y: 0.19, w: 0.12, h: 0.06 },
      }),
    ),
  );
  const start = Date.now();
  const svg = page.locator(".studio-stage svg");
  const rect = await svg.boundingBox();
  await page.mouse.click(
    rect.x + rect.width * 0.55,
    rect.y + rect.height * 0.5,
  );
  await page
    .getByRole("heading", { name: "Geographic inspection", exact: true })
    .waitFor();
  report.queryDuringRenderMs = Date.now() - start;
  report.deferredRasterRequests = held;
  await ready();
  await page.unroute("**/terrain/tiles/**");
  await capture("selection");

  // Rapid input must create one latest camera, not dozens of render jobs.
  const requestsBefore = await page.evaluate(
    () =>
      window.__terrainViewportLog.filter((e) => e.event === "request").length,
  );
  await page.evaluate(() => {
    const svg = document.querySelector(".studio-stage svg");
    const b = svg.getBoundingClientRect();
    for (let i = 0; i < 50; i++)
      svg.dispatchEvent(
        new WheelEvent("wheel", {
          clientX: b.x + b.width / 2,
          clientY: b.y + b.height / 2,
          deltaY: -3,
          bubbles: true,
          cancelable: true,
        }),
      );
  });
  await page.waitForTimeout(180);
  await ready();
  const requestsAfter = await page.evaluate(
    () =>
      window.__terrainViewportLog.filter((e) => e.event === "request").length,
  );
  assert.ok(
    requestsAfter - requestsBefore >= 1 && requestsAfter - requestsBefore <= 2,
    "obsolete wheel requests accumulated",
  );
  report.burstRenderRequests = requestsAfter - requestsBefore;

  await camera({ x: 0, y: 0, w: 2, h: 1 });
  await page
    .getByRole("button", { name: "Medieval Illustrator", exact: true })
    .click();
  await page.waitForTimeout(150);
  await ready();
  const artFast = await exportSVG("illustrator-fast");
  await page
    .getByRole("button", { name: "Construction mode", exact: true })
    .click();
  await quality.selectOption("high");
  await page.waitForTimeout(150);
  await ready();
  await page
    .getByRole("button", { name: "Medieval Illustrator", exact: true })
    .click();
  await page.waitForTimeout(150);
  await ready();
  const artHigh = await exportSVG("illustrator-detailed");
  assert.equal(
    artFast,
    artHigh,
    "Illustrator export depends on preview quality",
  );
  report.illustratorExportSHA256 = artFast;
  await capture("illustrator");
  assert.deepEqual(report.errors, []);
  report.passed = true;
} catch (error) {
  report.passed = false;
  report.failure = error.stack;
  await page
    .screenshot({ path: path.join(output, "failure.png") })
    .catch(() => {});
  process.exitCode = 1;
} finally {
  await fs.writeFile(
    path.join(output, "results.json"),
    JSON.stringify(report, null, 2),
  );
  console.log(JSON.stringify(report, null, 2));
  await browser.close();
}
