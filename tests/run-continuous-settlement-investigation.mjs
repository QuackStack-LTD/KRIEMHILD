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
const errors = [],
  result = {};
page.on("pageerror", (e) => errors.push(e.message));
page.on("dialog", (d) => d.accept());
const settle = async () => {
  await page.waitForFunction(
    () => document.querySelector("footer")?.innerText.endsWith("Ready"),
    null,
    { timeout: 90000 },
  );
  await page.waitForLoadState("networkidle", { timeout: 90000 });
};
try {
  await page.goto(`${base}/world/${world}/build?tileDebug=1`);
  await page.waitForSelector("image", { timeout: 90000 });
  await settle();
  await page.evaluate(
    await readFile("tests/continuous-investigation.js", "utf8"),
  );
  await page
    .getByRole("button", { name: "Settlement Builder", exact: true })
    .click();
  await page.getByRole("button", { name: /^site$/i }).click();
  const rect = await page.locator(".studio-stage svg").boundingBox(),
    click = [rect.x + rect.width / 2, rect.y + rect.height / 2];
  const point = await page.evaluate(([x, y]) => {
    const p = new DOMPoint(x, y).matrixTransform(
      document.querySelector(".studio-stage svg").getScreenCTM().inverse(),
    );
    return [p.x, p.y];
  }, click);
  const previewPromise = page.waitForResponse((r) =>
    r.url().endsWith("/settlements/preview"),
  );
  await page.mouse.click(...click);
  const response = await previewPromise,
    draft = await response.json();
  assert.equal(response.status(), 200, JSON.stringify(draft));
  const spec = response.request().postDataJSON();
  assert.ok(Math.abs(spec.longitude - (point[0] * 180 - 180)) < 1e-4);
  assert.ok(Math.abs(spec.latitude - (90 - point[1] * 180)) < 1e-4);
  const session = response.url().split("/")[5];
  await settle();
  const camera = await page
    .locator(".studio-stage svg")
    .getAttribute("viewBox");
  const [x, y, w, h] = camera.split(" ").map(Number);
  assert.ok(
    Math.abs(x + w / 2 - point[0]) < 1e-6 &&
      Math.abs(y + h / 2 - point[1]) < 1e-6,
    "preview camera drifted from selected site",
  );
  await page.screenshot({ path: `${out}/14-settlement-site.png` });
  await page
    .getByRole("button", { name: "Accept settlement", exact: true })
    .click();
  await settle();
  const accepted = await (
    await page.request.get(
      `${base}/api/sessions/${session}/world/entities?all=1&detail=8`,
    )
  ).json();
  for (const e of draft.entities)
    assert.deepEqual(
      accepted.entities.find((v) => v.id === e.id)?.points,
      e.points,
    );
  await page.getByRole("button", { name: /^select$/i }).click();
  const r = await page.locator(".studio-stage svg").boundingBox();
  await page.mouse.move(r.x + r.width * 0.42, r.y + r.height * 0.4);
  await page.mouse.wheel(0, -266);
  await settle();
  await page.mouse.move(r.x + 30, r.y + 30);
  await page.mouse.down();
  await page.mouse.move(r.x + 140, r.y + 120, { steps: 4 });
  await page.mouse.up();
  await settle();
  const coverage = await page.evaluate(() => {
    const s = terrainAudit.state();
    delete s.log;
    return s;
  });
  assert.equal(coverage.missing, 0);
  await page.screenshot({ path: `${out}/15-settlement-zoom-pan.png` });
  const after = await (
    await page.request.get(
      `${base}/api/sessions/${session}/world/entities?all=1&detail=8`,
    )
  ).json();
  for (const e of draft.entities)
    assert.deepEqual(
      after.entities.find((v) => v.id === e.id)?.points,
      e.points,
    );
  result.settlement = {
    point,
    spec,
    camera,
    featureCount: draft.entities.length,
    score: draft.preview.score,
    coverage,
    geographicCoordinatesUnchanged: true,
  };
  await page.getByLabel("Export width").selectOption("1024");
  for (const format of ["SVG", "PNG"]) {
    const promise = page.waitForEvent("download", { timeout: 90000 });
    await page.getByRole("button", { name: format, exact: true }).click();
    const download = await promise;
    const path = `${out}/settlement-export.${format.toLowerCase()}`;
    await download.saveAs(path);
    result[format] = { path, bytes: (await readFile(path)).length };
  }
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await settle();
  assert.deepEqual(errors, []);
  console.log(JSON.stringify(result.settlement, null, 2));
} finally {
  result.errors = errors;
  await writeFile(
    `${out}/settlement-audit.json`,
    JSON.stringify(result, null, 2),
  );
  await browser.close();
}
