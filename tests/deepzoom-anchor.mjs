import { chromium } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import assert from "node:assert/strict";
const out =
  process.env.KRIEMHILD_AUDIT_OUT ||
  "docs/deepzoom-investigation/anchor-before";
await mkdir(out, { recursive: true });
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.KRIEMHILD_BROWSER_EXECUTABLE,
});
try {
  const page = await browser.newPage({
    viewport: { width: 1280, height: 800 },
  });
  await page.goto(
    "http://127.0.0.1:8125/world/977d134ee5aadc480e1410ade3c84587/build?tileDebug=1",
  );
  await page.waitForSelector('[data-phase="final"]', { timeout: 180000 });
  await page.getByLabel("Autosave accepted terrain after 30 seconds").uncheck();
  await page.getByRole("button", { name: "Fit world", exact: true }).click();
  await page.waitForTimeout(300);
  const before = await page.evaluate(() => {
    const svg = document.querySelector(".studio-stage svg"),
      r = svg.getBoundingClientRect();
    const x = Math.round(r.x + r.width * 0.37),
      y = Math.round(r.y + r.height * 0.42);
    const p = new DOMPoint(x, y).matrixTransform(svg.getScreenCTM().inverse());
    for (let i = 0; i < 4; i++)
      svg.dispatchEvent(
        new WheelEvent("wheel", {
          bubbles: true,
          cancelable: true,
          clientX: x,
          clientY: y,
          deltaY: -200,
        }),
      );
    return { x, y, point: [p.x, p.y] };
  });
  await page.waitForTimeout(500);
  const after = await page.evaluate(({ x, y }) => {
    const svg = document.querySelector(".studio-stage svg");
    const p = new DOMPoint(x, y).matrixTransform(svg.getScreenCTM().inverse());
    return { point: [p.x, p.y], camera: svg.getAttribute("viewBox") };
  }, before);
  const error = Math.hypot(...before.point.map((v, i) => v - after.point[i]));
  await writeFile(
    `${out}/results.json`,
    JSON.stringify({ before, after, error }, null, 2),
  );
  console.log({ error });
  // Chromium's SVG screen matrix rounds coefficients; camera-state comparisons
  // below remain at 1e-12. This checks displayed pointer agreement at CTM precision.
  assert.ok(error < 1e-7, "batched wheel events moved the geographic anchor");
  await page.getByRole("button", { name: "Fit world", exact: true }).click();
  await page.waitForTimeout(300);
  const moving = await page.evaluate(() => {
    const svg = document.querySelector(".studio-stage svg"),
      r = svg.getBoundingClientRect();
    const [x, y, w, h] = svg.getAttribute("viewBox").split(" ").map(Number);
    const original = { x, y, w, h };
    let expected = { ...original },
      legacy = { ...original };
    const pointer = (c, x, y) => {
      const scale = Math.min(r.width / c.w, r.height / c.h);
      return [
        c.x + (x - r.left - (r.width - c.w * scale) / 2) / scale,
        c.y + (y - r.top - (r.height - c.h * scale) / 2) / scale,
      ];
    };
    const advance = (c, p) => {
      const f = Math.exp(-0.3);
      return {
        x: p[0] - (p[0] - c.x) * f,
        y: p[1] - (p[1] - c.y) * f,
        w: c.w * f,
        h: c.h * f,
      };
    };
    const events = [];
    for (const [u, v] of [
      [0.37, 0.42],
      [0.6, 0.7],
      [0.4, 0.5],
      [0.7, 0.2],
    ]) {
      const e = new WheelEvent("wheel", {
        bubbles: true,
        cancelable: true,
        clientX: Math.round(r.x + r.width * u),
        clientY: Math.round(r.y + r.height * v),
        deltaY: -200,
      });
      expected = advance(expected, pointer(expected, e.clientX, e.clientY));
      legacy = advance(legacy, pointer(original, e.clientX, e.clientY));
      events.push({ x: e.clientX, y: e.clientY });
      svg.dispatchEvent(e);
    }
    return { original, expected, legacy, events };
  });
  await page.waitForTimeout(500);
  moving.actual = await page.evaluate(() =>
    document
      .querySelector(".studio-stage svg")
      .getAttribute("viewBox")
      .split(" ")
      .map(Number),
  );
  moving.error = Math.max(
    ...["x", "y", "w", "h"].map((k, i) =>
      Math.abs(moving.expected[k] - moving.actual[i]),
    ),
  );
  moving.legacyError = Math.max(
    ...["x", "y", "w", "h"].map((k, i) =>
      Math.abs(moving.legacy[k] - moving.actual[i]),
    ),
  );
  await writeFile(`${out}/moving.json`, JSON.stringify(moving, null, 2));
  console.log({ movingError: moving.error, legacyError: moving.legacyError });
  assert.ok(moving.error < 1e-12);
  assert.ok(moving.legacyError > 1e-3);
} finally {
  await browser.close();
}
